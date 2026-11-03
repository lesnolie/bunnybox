// Package smoke runs an explicitly requested, direct-only core lifecycle check.
// It never installs a service, takes ownership from another user, or enables a
// TUN, proxy listener, system proxy, or external API.
package smoke

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"singboxwin.local/client/internal/rpc/client"
	"singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
	"singboxwin.local/client/internal/rpc/worker"
)

// No inbound, DNS server, experimental API, or route/interface override exists.
const minimalConfig = `{"log":{"level":"warn"},"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`

type Stage struct {
	Name   string `json:"name"`
	Result string `json:"result"`
	Detail string `json:"detail,omitempty"`
}

type Report struct {
	Time             string  `json:"time"`
	Executable       string  `json:"daemon_executable,omitempty"`
	Ownership        string  `json:"ownership,omitempty"`
	InitialState     string  `json:"initial_state,omitempty"`
	StartedAt        int64   `json:"started_at_unix_ms,omitempty"`
	Claimed          bool    `json:"claimed"`
	CoreStarted      bool    `json:"core_started"`
	CoreStopped      bool    `json:"core_stopped"`
	CleanupAttempted bool    `json:"cleanup_attempted"`
	CleanupError     string  `json:"cleanup_error,omitempty"`
	Stages           []Stage `json:"stages"`
	FailedStage      string  `json:"failed_stage,omitempty"`
	Error            string  `json:"error,omitempty"`
}

func (r *Report) record(name string, err error) bool {
	stage := Stage{Name: name, Result: "ok"}
	if err != nil {
		stage.Result, stage.Detail = "failed", err.Error()
		if r.Error == "" {
			r.FailedStage, r.Error = name, err.Error()
		}
	}
	r.Stages = append(r.Stages, stage)
	return err == nil
}

// Run is opt-in. It uses the installed official worker and its authenticated
// daemon relay. The caller must ensure no other same-user client changes the
// core during this check: upstream has no atomic start-if-idle/stop-instance RPC.
// Claim/start persist ownership and the minimal configuration in daemon state.
func Run(ctx context.Context) (report Report) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	report.Time = time.Now().UTC().Format(time.RFC3339)
	path, err := worker.InstalledExecutable()
	if !report.record("locate", err) {
		return
	}
	report.Executable = path
	manager, err := worker.New(worker.Config{Executable: path})
	if !report.record("worker_setup", err) {
		return
	}
	defer manager.Close()
	rpc := client.New(manager, client.Config{Language: func() string { return "zh-Hans" }})
	defer rpc.Close()
	conn, err := rpc.Daemon(ctx)
	if !report.record("daemon_transport", err) {
		return
	}
	lifecycle(ctx, conn, &report)
	return
}

func lifecycle(ctx context.Context, conn grpc.ClientConnInterface, report *Report) {
	desktopClient := desktop.NewDesktopServiceClient(conn)
	started := daemon.NewStartedServiceClient(conn)
	managed := daemon.NewManagedServiceClient(conn)
	empty := &emptypb.Empty{}
	info, err := desktopClient.GetDaemonInfo(ctx, empty)
	if !report.record("get_daemon_info", err) {
		return
	}
	report.Ownership = info.GetOwnership().String()
	if info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE && info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER {
		report.record("ownership_guard", fmt.Errorf("refusing daemon ownership %s; no takeover", report.Ownership))
		return
	}
	// An unowned daemon denies StartedService RPCs until ClaimService. An
	// already owned daemon can and must be inspected before making any claim.
	if info.GetOwnership() == desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER {
		state, err := snapshot(ctx, started)
		if err == nil {
			report.InitialState = state.GetStatus().String()
			err = requireIdle(state)
		}
		if !report.record("initial_idle_guard", err) {
			return
		}
	}
	_, err = desktopClient.ClaimService(ctx, empty)
	if !report.record("claim_service", err) {
		return
	}
	report.Claimed = true
	state, err := snapshot(ctx, started)
	if err == nil {
		report.InitialState = state.GetStatus().String()
		err = requireIdle(state)
	}
	if !report.record("pre_start_idle_guard", err) {
		return
	}
	// No StopService is permitted unless this exact StartService returned
	// success. In particular, a failed or deadline-ambiguous start is not ours
	// to stop: upstream itself cleans a failed start.
	_, err = desktopClient.StartService(ctx, &desktop.StartServiceRequest{
		ConfigContent: minimalConfig,
		Options:       &desktop.StartOptions{OomKillerDisabled: true},
	})
	if !report.record("start_service", err) {
		return
	}
	report.CoreStarted = true
	defer func() {
		if report.CoreStopped {
			return
		}
		report.CleanupAttempted = true
		// Main deadline/cancellation must not strand a successfully started core.
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		err := stopOwnCore(cleanupCtx, desktopClient, started, managed, report.StartedAt)
		if err == nil {
			report.CoreStopped = true
		} else {
			report.CleanupError = err.Error()
		}
		report.record("cleanup_own_core", err)
	}()
	stamp, err := started.GetStartedAt(ctx, empty)
	if err == nil && stamp.GetStartedAt() <= 0 {
		err = errors.New("started core has no positive start timestamp")
	}
	if !report.record("get_started_at", err) {
		return
	}
	report.StartedAt = stamp.GetStartedAt()
	err = waitState(ctx, started, daemon.ServiceStatus_STARTED)
	if !report.record("wait_started", err) {
		return
	}
	err = stopOwnCore(ctx, desktopClient, started, managed, report.StartedAt)
	if !report.record("stop_and_confirm_idle", err) {
		return
	}
	report.CoreStopped = true
}

func snapshot(ctx context.Context, started daemon.StartedServiceClient) (*daemon.ServiceStatus, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := started.SubscribeServiceStatus(streamCtx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	// Official SubscribeServiceStatus sends the current state as its first item.
	return stream.Recv()
}

func requireIdle(state *daemon.ServiceStatus) error {
	if state == nil || state.GetStatus() != daemon.ServiceStatus_IDLE {
		return fmt.Errorf("refusing non-idle core state %s; existing core will not be replaced or stopped", state.GetStatus())
	}
	return nil
}

func waitState(ctx context.Context, started daemon.StartedServiceClient, expected daemon.ServiceStatus_Type) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := started.SubscribeServiceStatus(streamCtx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	for {
		state, err := stream.Recv()
		if err != nil {
			return err
		}
		if state.GetStatus() == expected {
			return nil
		}
		if state.GetStatus() == daemon.ServiceStatus_FATAL {
			return fmt.Errorf("core FATAL: %s", state.GetErrorMessage())
		}
		if expected == daemon.ServiceStatus_STARTED && state.GetStatus() != daemon.ServiceStatus_STARTING {
			return fmt.Errorf("core did not start: %s", state.GetStatus())
		}
	}
}

func stopOwnCore(ctx context.Context, desktopClient desktop.DesktopServiceClient, started daemon.StartedServiceClient, managed daemon.ManagedServiceClient, expectedStamp int64) error {
	empty := &emptypb.Empty{}
	info, err := desktopClient.GetDaemonInfo(ctx, empty)
	if err != nil {
		return fmt.Errorf("recheck owner before stop: %w", err)
	}
	if info.GetOwnership() != desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER {
		return errors.New("ownership changed; refusing stop")
	}
	state, err := snapshot(ctx, started)
	if err != nil {
		return err
	}
	if state.GetStatus() == daemon.ServiceStatus_IDLE {
		return nil
	}
	if expectedStamp <= 0 {
		return errors.New("own core start timestamp unavailable; refusing potentially unrelated core stop")
	}
	stamp, err := started.GetStartedAt(ctx, empty)
	if err != nil {
		return fmt.Errorf("recheck start timestamp before stop: %w", err)
	}
	if expectedStamp > 0 && stamp.GetStartedAt() != expectedStamp {
		return errors.New("core start timestamp changed; refusing stop of a replacement core")
	}
	_, err = managed.StopService(ctx, empty)
	if err != nil {
		return err
	}
	return waitState(ctx, started, daemon.ServiceStatus_IDLE)
}

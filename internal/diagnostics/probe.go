// Package diagnostics performs read-only checks of the real worker and daemon.
package diagnostics

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"singboxwin.local/client/internal/daemon"
	"singboxwin.local/client/internal/rpc/client"
	"singboxwin.local/client/internal/rpc/gen/desktop"
	"singboxwin.local/client/internal/rpc/worker"
)

type Report struct {
	Time           string       `json:"time"`
	Executable     string       `json:"daemon_executable,omitempty"`
	Version        string       `json:"binary_version,omitempty"`
	WorkerReady    bool         `json:"worker_ready"`
	ApplicationRPC bool         `json:"application_rpc"`
	DaemonRPC      bool         `json:"daemon_rpc"`
	DaemonVersion  string       `json:"daemon_version,omitempty"`
	Ownership      string       `json:"ownership,omitempty"`
	Service        *daemon.Info `json:"service,omitempty"`
	VersionMatch   *bool        `json:"version_match,omitempty"`
	FailedStage    string       `json:"failed_stage,omitempty"`
	Error          string       `json:"error,omitempty"`
}

// Run never installs a service, claims ownership or starts/stops the core.
// Official authentication still enforces the signed, protected install layout.
func Run(ctx context.Context) (report Report) {
	report.Time = time.Now().UTC().Format(time.RFC3339)
	fail := func(stage string, err error) Report {
		report.FailedStage, report.Error = stage, err.Error()
		return report
	}
	path, err := worker.InstalledExecutable()
	if err != nil {
		return fail("locate", err)
	}
	report.Executable = path
	version, err := daemon.BinaryVersion(ctx, path)
	if err != nil {
		return fail("version", err)
	}
	report.Version = version
	manager, err := worker.New(worker.Config{Executable: path})
	if err != nil {
		return fail("worker", err)
	}
	defer manager.Close()
	if _, err = manager.Ensure(ctx); err != nil {
		return fail("worker", err)
	}
	report.WorkerReady = true
	rpc := client.New(manager, client.Config{Language: func() string { return "zh-Hans" }})
	defer rpc.Close()
	app, err := rpc.Application(ctx)
	if err != nil {
		return fail("application_transport", err)
	}
	if _, err = desktop.NewApplicationServiceClient(app).FormatConfig(ctx, &desktop.ConfigContent{Content: "{}"}); err != nil {
		return fail("application_rpc", err)
	}
	report.ApplicationRPC = true
	if err = inspectService(ctx, &report, daemon.Inspect); err != nil {
		return fail(daemon.FailedStage(err), err)
	}
	daemonConn, err := rpc.Daemon(ctx)
	if err != nil {
		return fail("daemon_transport", err)
	}
	info, err := desktop.NewDesktopServiceClient(daemonConn).GetDaemonInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return fail("daemon_rpc", err)
	}
	if err := recordDaemonInfo(&report, info.GetVersion(), info.GetOwnership().String()); err != nil {
		return fail(daemon.FailedStage(err), err)
	}
	return report
}

func recordDaemonInfo(report *Report, version, ownership string) error {
	report.DaemonRPC, report.DaemonVersion, report.Ownership = true, version, ownership
	err := daemon.CheckVersion(report.Version, version)
	match := err == nil
	report.VersionMatch = &match
	return err
}

func inspectService(ctx context.Context, report *Report, inspect func(context.Context, string) (daemon.Info, error)) error {
	info, err := inspect(ctx, report.Executable)
	report.Service = &info
	return err
}

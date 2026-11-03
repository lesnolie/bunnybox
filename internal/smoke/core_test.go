package smoke

import (
	"context"
	"encoding/json"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	"singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
)

// In-memory gRPC exercises the generated wire clients, unary and status stream
// RPCs, cancellation, and the no-stop safety guards without a real service.
type fakeDaemon struct {
	desktop.UnimplementedDesktopServiceServer
	daemon.UnimplementedStartedServiceServer
	daemon.UnimplementedManagedServiceServer
	mu              sync.Mutex
	owner           desktop.DaemonOwnership
	state           daemon.ServiceStatus_Type
	calls           []string
	config          string
	startedAt       int64
	stampReads      int
	startError      bool
	claimError      bool
	ownerChanges    bool
	replacement     bool
	hangStartStatus bool
	stampError      bool
	stopFailureOnce bool
	fatalAfterStart bool
	stopCalls       int
	stateAfterClaim *daemon.ServiceStatus_Type
}

func (f *fakeDaemon) GetDaemonInfo(context.Context, *emptypb.Empty) (*desktop.DaemonInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "info")
	owner := f.owner
	if f.ownerChanges && f.startedAt > 0 {
		owner = desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER
	}
	return &desktop.DaemonInfo{Ownership: owner}, nil
}
func (f *fakeDaemon) ClaimService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "claim")
	if f.claimError {
		return nil, status.Error(codes.Aborted, "claimed concurrently")
	}
	f.owner = desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER
	if f.stateAfterClaim != nil {
		f.state = *f.stateAfterClaim
	}
	return &emptypb.Empty{}, nil
}
func (f *fakeDaemon) StartService(_ context.Context, r *desktop.StartServiceRequest) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "start")
	f.config = r.GetConfigContent()
	if f.startError {
		return nil, status.Error(codes.Unavailable, "start failed or reply lost")
	}
	f.state = daemon.ServiceStatus_STARTED
	if f.fatalAfterStart {
		f.state = daemon.ServiceStatus_FATAL
	}
	f.startedAt = time.Now().UnixMilli()
	return &emptypb.Empty{}, nil
}
func (f *fakeDaemon) GetStartedAt(context.Context, *emptypb.Empty) (*daemon.StartedAt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "stamp")
	f.stampReads++
	if f.stampError {
		return nil, status.Error(codes.Unavailable, "stamp unavailable")
	}
	stamp := f.startedAt
	if f.replacement && f.stampReads > 1 {
		stamp++
	}
	return &daemon.StartedAt{StartedAt: stamp}, nil
}
func (f *fakeDaemon) StopService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "stop")
	f.stopCalls++
	if f.stopFailureOnce && f.stopCalls == 1 {
		return nil, status.Error(codes.Unavailable, "stop failed")
	}
	f.state = daemon.ServiceStatus_IDLE
	f.startedAt = 0
	return &emptypb.Empty{}, nil
}
func (f *fakeDaemon) SubscribeServiceStatus(_ *emptypb.Empty, s grpc.ServerStreamingServer[daemon.ServiceStatus]) error {
	f.mu.Lock()
	f.calls = append(f.calls, "status")
	state := f.state
	owner := f.owner
	if f.hangStartStatus && f.startedAt > 0 {
		state = daemon.ServiceStatus_STARTING
	}
	f.mu.Unlock()
	if owner == desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE {
		return status.Error(codes.PermissionDenied, "service has no owner")
	}
	if err := s.Send(&daemon.ServiceStatus{Status: state}); err != nil {
		return err
	}
	<-s.Context().Done()
	return s.Context().Err()
}

func exercise(t *testing.T, f *fakeDaemon, timeout time.Duration) Report {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	desktop.RegisterDesktopServiceServer(server, f)
	daemon.RegisterStartedServiceServer(server, f)
	daemon.RegisterManagedServiceServer(server, f)
	go server.Serve(lis)
	t.Cleanup(func() { server.Stop(); lis.Close() })
	conn, err := grpc.NewClient("passthrough:///smoke-fake", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var report Report
	lifecycle(ctx, conn, &report)
	return report
}

func countCall(f *fakeDaemon, name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if call == name {
			count++
		}
	}
	return count
}

func TestLifecycleStartsDirectOnlyAndConfirmsIdle(t *testing.T) {
	for _, owner := range []desktop.DaemonOwnership{desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER} {
		t.Run(owner.String(), func(t *testing.T) {
			f := &fakeDaemon{owner: owner}
			r := exercise(t, f, 2*time.Second)
			if r.Error != "" || !r.Claimed || !r.CoreStarted || !r.CoreStopped || r.CleanupAttempted || r.StartedAt <= 0 {
				t.Fatalf("unexpected report: %+v", r)
			}
			if countCall(f, "start") != 1 || countCall(f, "stop") != 1 {
				t.Fatalf("calls: %v", f.calls)
			}
			if f.calls[len(f.calls)-1] != "status" {
				t.Fatalf("missing post-stop state confirmation: %v", f.calls)
			}
			var config map[string]any
			if err := json.Unmarshal([]byte(f.config), &config); err != nil {
				t.Fatal(err)
			}
			if len(config) != 3 || config["inbounds"] != nil || config["dns"] != nil || config["experimental"] != nil {
				t.Fatalf("unsafe config: %s", f.config)
			}
			outbounds := config["outbounds"].([]any)
			if len(outbounds) != 1 || outbounds[0].(map[string]any)["type"] != "direct" {
				t.Fatalf("not direct-only: %s", f.config)
			}
		})
	}
}

func TestRefusesOtherOrUnknownOwnerWithoutMutation(t *testing.T) {
	for _, owner := range []desktop.DaemonOwnership{desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER, desktop.DaemonOwnership_DAEMON_OWNERSHIP_UNSPECIFIED, 99} {
		f := &fakeDaemon{owner: owner}
		r := exercise(t, f, time.Second)
		if r.FailedStage != "ownership_guard" || !reflect.DeepEqual(f.calls, []string{"info"}) {
			t.Fatalf("report=%+v calls=%v", r, f.calls)
		}
	}
}

func TestNeverReplacesOrStopsPreexistingCore(t *testing.T) {
	for _, state := range []daemon.ServiceStatus_Type{daemon.ServiceStatus_STARTING, daemon.ServiceStatus_STARTED, daemon.ServiceStatus_STOPPING, daemon.ServiceStatus_FATAL, 99} {
		f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, state: state}
		r := exercise(t, f, time.Second)
		if r.FailedStage != "initial_idle_guard" || countCall(f, "claim") != 0 || countCall(f, "start") != 0 || countCall(f, "stop") != 0 {
			t.Fatalf("report=%+v calls=%v", r, f.calls)
		}
	}
}

func TestRechecksIdleAfterClaim(t *testing.T) {
	state := daemon.ServiceStatus_STARTED
	f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE, stateAfterClaim: &state}
	r := exercise(t, f, time.Second)
	if r.FailedStage != "pre_start_idle_guard" || countCall(f, "start") != 0 || countCall(f, "stop") != 0 {
		t.Fatalf("report=%+v calls=%v", r, f.calls)
	}
}

func TestFailureBeforeSuccessfulStartNeverStops(t *testing.T) {
	for _, claimFail := range []bool{false, true} {
		f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, claimError: claimFail, startError: !claimFail}
		r := exercise(t, f, time.Second)
		if r.Error == "" || r.CoreStarted || r.CleanupAttempted || countCall(f, "stop") != 0 {
			t.Fatalf("report=%+v calls=%v", r, f.calls)
		}
	}
}

func TestDeadlineCleansOnlySuccessfullyStartedCoreWithFreshContext(t *testing.T) {
	f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, hangStartStatus: true}
	r := exercise(t, f, 100*time.Millisecond)
	if r.FailedStage != "wait_started" || !r.CleanupAttempted || !r.CoreStopped || r.CleanupError != "" || countCall(f, "stop") != 1 {
		t.Fatalf("report=%+v calls=%v", r, f.calls)
	}
}

func TestCleanupRefusesChangedOwnershipOrReplacementCore(t *testing.T) {
	for _, test := range []struct {
		name    string
		f       *fakeDaemon
		message string
	}{
		{"owner", &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, ownerChanges: true}, "ownership changed"},
		{"replacement", &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, replacement: true}, "timestamp changed"},
		{"unknown_identity", &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, stampError: true}, "timestamp unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := exercise(t, test.f, time.Second)
			if r.Error == "" || !r.CleanupAttempted || r.CoreStopped || !strings.Contains(r.CleanupError, test.message) || countCall(test.f, "stop") != 0 {
				t.Fatalf("report=%+v calls=%v", r, test.f.calls)
			}
		})
	}
}

func TestCleanupRetriesOwnFailedStopAndKeepsOriginalFailure(t *testing.T) {
	f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, stopFailureOnce: true}
	r := exercise(t, f, time.Second)
	if r.FailedStage != "stop_and_confirm_idle" || !r.CleanupAttempted || !r.CoreStopped || r.CleanupError != "" || countCall(f, "stop") != 2 {
		t.Fatalf("report=%+v calls=%v", r, f.calls)
	}
}

func TestFatalStartedCoreIsFailureAndOwnInstanceIsCleaned(t *testing.T) {
	f := &fakeDaemon{owner: desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, fatalAfterStart: true}
	r := exercise(t, f, time.Second)
	if r.FailedStage != "wait_started" || !strings.Contains(r.Error, "FATAL") || !r.CoreStopped || countCall(f, "stop") != 1 {
		t.Fatalf("report=%+v calls=%v", r, f.calls)
	}
}

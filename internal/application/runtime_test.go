package application

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
)

type fakeRuntimeDesktop struct {
	desktop.UnimplementedDesktopServiceServer
	owner desktop.DaemonOwnership
}

func (f *fakeRuntimeDesktop) GetDaemonInfo(context.Context, *emptypb.Empty) (*desktop.DaemonInfo, error) {
	return &desktop.DaemonInfo{Ownership: f.owner}, nil
}

type fakeRuntimeStarted struct {
	d.UnimplementedStartedServiceServer
	state            d.ServiceStatus_Type
	metrics          *d.Status
	groups           *d.Groups
	mode             *d.ClashModeStatus
	modeError        error
	frames           []*d.Status
	actions          chan string
	proxyUnavailable bool
	stallMetrics     bool
	statusError      error
	canceled         chan string
	interval         chan int64
}

func (f *fakeRuntimeStarted) SubscribeServiceStatus(_ *emptypb.Empty, s grpc.ServerStreamingServer[d.ServiceStatus]) error {
	if f.statusError != nil {
		return f.statusError
	}
	if e := s.Send(&d.ServiceStatus{Status: f.state}); e != nil {
		return e
	}
	<-s.Context().Done()
	f.canceled <- "state"
	return s.Context().Err()
}
func (f *fakeRuntimeStarted) SubscribeStatus(req *d.SubscribeStatusRequest, s grpc.ServerStreamingServer[d.Status]) error {
	f.interval <- req.Interval
	if !f.stallMetrics {
		if e := s.Send(f.metrics); e != nil {
			return e
		}
	}
	for _, frame := range f.frames {
		if e := s.Send(frame); e != nil {
			return e
		}
	}
	<-s.Context().Done()
	f.canceled <- "metrics"
	return s.Context().Err()
}
func (f *fakeRuntimeStarted) SubscribeGroups(_ *emptypb.Empty, s grpc.ServerStreamingServer[d.Groups]) error {
	if e := s.Send(f.groups); e != nil {
		return e
	}
	<-s.Context().Done()
	f.canceled <- "groups"
	return s.Context().Err()
}
func (f *fakeRuntimeStarted) GetClashModeStatus(context.Context, *emptypb.Empty) (*d.ClashModeStatus, error) {
	return f.mode, f.modeError
}

type runtimeCalls struct {
	mu      sync.Mutex
	methods []string
}

func (c *runtimeCalls) add(method string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.methods = append(c.methods, method)
}
func (c *runtimeCalls) list() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string{}, c.methods...)
}
func runtimeFixture(t *testing.T, owner desktop.DaemonOwnership, configure func(*fakeRuntimeStarted)) (runtimeReader, *fakeRuntimeStarted, *runtimeCalls) {
	t.Helper()
	listener := bufconn.Listen(256 * 1024)
	f := &fakeRuntimeStarted{actions: make(chan string, 16), state: d.ServiceStatus_STARTED, metrics: &d.Status{Memory: 1234567, Goroutines: 23, ConnectionsIn: 4, ConnectionsOut: 5, TrafficAvailable: true, UplinkTotal: 123, DownlinkTotal: 456}, groups: &d.Groups{Group: []*d.Group{{Tag: "auto", Type: "urltest", Selected: "node-a", IsExpand: true, Items: []*d.GroupItem{{Tag: "node-a", Type: "shadowsocks", UrlTestTime: 1234, UrlTestDelay: 81}}}, {Tag: "select", Type: "selector", Selectable: true, Selected: "direct", Items: []*d.GroupItem{{Tag: "direct", Type: "direct"}}}}}, mode: &d.ClashModeStatus{CurrentMode: "rule", ModeList: []string{"rule", "global", "direct"}}, canceled: make(chan string, 64), interval: make(chan int64, 16)}
	if configure != nil {
		configure(f)
	}
	calls := new(runtimeCalls)
	server := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		calls.add(info.FullMethod)
		return handler(ctx, req)
	}), grpc.StreamInterceptor(func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		calls.add(info.FullMethod)
		return handler(srv, stream)
	}))
	desktop.RegisterDesktopServiceServer(server, &fakeRuntimeDesktop{owner: owner})
	d.RegisterStartedServiceServer(server, f)
	d.RegisterManagedServiceServer(server, &fakeDashboardManaged{f: f})
	go server.Serve(listener)
	t.Cleanup(func() { server.Stop(); listener.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, e := grpc.DialContext(ctx, "passthrough:///runtime-test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithBlock())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { conn.Close() })
	return runtimeReader{conn: conn, timeout: 150 * time.Millisecond}, f, calls
}
func waitCanceled(t *testing.T, f *fakeRuntimeStarted, want int) {
	t.Helper()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for range want {
		select {
		case <-f.canceled:
		case <-timeout.C:
			t.Fatal("snapshot left a stream running after first frame / timeout")
		}
	}
}
func assertOnlyReadCalls(t *testing.T, calls *runtimeCalls) {
	t.Helper()
	allowed := map[string]bool{desktop.DesktopService_GetDaemonInfo_FullMethodName: true, d.StartedService_SubscribeServiceStatus_FullMethodName: true, d.StartedService_SubscribeStatus_FullMethodName: true, d.StartedService_SubscribeGroups_FullMethodName: true, d.StartedService_GetClashModeStatus_FullMethodName: true}
	for _, method := range calls.list() {
		if !allowed[method] {
			t.Errorf("unexpected mutating / probing RPC: %s", method)
		}
	}
}

func TestRuntimeOwnershipDoesNotSubscribeOrClaim(t *testing.T) {
	for _, owner := range []desktop.DaemonOwnership{desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE, desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER} {
		t.Run(owner.String(), func(t *testing.T) {
			r, _, calls := runtimeFixture(t, owner, nil)
			metrics, e := r.metrics(context.Background())
			if e != nil || metrics.Available || metrics.MemoryBytes != 0 || metrics.TrafficAvailable {
				t.Fatalf("metrics=%+v err=%v", metrics, e)
			}
			groups, e := r.groups(context.Background())
			if e != nil || groups.Available || len(groups.Groups) != 0 {
				t.Fatalf("groups=%+v err=%v", groups, e)
			}
			mode, e := r.mode(context.Background())
			if e != nil || mode.Available || mode.Current != "" {
				t.Fatalf("mode=%+v err=%v", mode, e)
			}
			reason := "unclaimed"
			if owner == desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER {
				reason = "other_owner"
			}
			if metrics.Reason != reason || groups.Reason != reason || mode.Reason != reason {
				t.Fatal("missing explicit availability reason")
			}
			for _, method := range calls.list() {
				if method != desktop.DesktopService_GetDaemonInfo_FullMethodName {
					t.Fatalf("unowned request performed %s", method)
				}
			}
		})
	}
}
func TestRuntimeSnapshotsKeepRealValuesAndCancelStreams(t *testing.T) {
	r, f, calls := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, nil)
	metrics, e := r.metrics(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if !metrics.Available || metrics.MemoryBytes != 1234567 || metrics.Goroutines != 23 || metrics.ConnectionsIn != 4 || metrics.ConnectionsOut != 5 || !metrics.TrafficAvailable || metrics.UplinkTotal != 123 || metrics.DownlinkTotal != 456 {
		t.Fatalf("wire metrics lost: %+v", metrics)
	}
	if metrics.RateAvailable {
		t.Fatal("first status frame is not a measured rate interval")
	}
	if interval := <-f.interval; interval != int64(time.Second) {
		t.Fatalf("interval=%d", interval)
	}
	groups, e := r.groups(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if !groups.Available || len(groups.Groups) != 2 || groups.Groups[0].Tag != "auto" || groups.Groups[0].Type != "urltest" || groups.Groups[0].Selected != "node-a" || !groups.Groups[0].Expanded || groups.Groups[1].Selectable != true {
		t.Fatalf("wire groups lost: %+v", groups)
	}
	item := groups.Groups[0].Items[0]
	if item.Tag != "node-a" || item.Type != "shadowsocks" || item.DelayMS != 81 || item.URLTestTime != 1234 {
		t.Fatalf("wire node lost: %+v", item)
	}
	if groups.Groups[1].Items[0].DelayMS != 0 || groups.Groups[1].Items[0].URLTestTime != 0 {
		t.Fatal("unmeasured node delay fabricated")
	}
	mode, e := r.mode(context.Background())
	if e != nil || !mode.Available || mode.Current != "rule" || len(mode.Options) != 3 {
		t.Fatalf("wire mode lost: %+v %v", mode, e)
	}
	waitCanceled(t, f, 5)
	assertOnlyReadCalls(t, calls)
}
func TestRuntimeIdleDoesNotWaitForGroupsOrMode(t *testing.T) {
	r, f, calls := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) { f.state = d.ServiceStatus_IDLE })
	groups, e := r.groups(context.Background())
	if e != nil || groups.Available || groups.Reason != "core_idle" {
		t.Fatalf("%+v %v", groups, e)
	}
	mode, e := r.mode(context.Background())
	if e != nil || mode.Available || mode.Reason != "core_idle" {
		t.Fatalf("%+v %v", mode, e)
	}
	for _, method := range calls.list() {
		if method != desktop.DesktopService_GetDaemonInfo_FullMethodName && method != d.StartedService_SubscribeServiceStatus_FullMethodName {
			t.Fatalf("idle core invoked %s", method)
		}
	}
	waitCanceled(t, f, 2)
}
func TestRuntimeDeadlineReleasesStreamAndDoesNotBlockOtherReads(t *testing.T) {
	r, f, calls := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) { f.stallMetrics = true })
	done := make(chan error, 1)
	go func() {
		snapshot, e := r.metrics(context.Background())
		if snapshot.Available || snapshot.Reason != "timeout" {
			t.Errorf("timeout snapshot %+v", snapshot)
		}
		done <- e
	}()
	// A stalled metric stream does not lock the independent group reader.
	groups, e := r.groups(context.Background())
	if e != nil || !groups.Available {
		t.Fatalf("independent groups blocked: %+v %v", groups, e)
	}
	select {
	case e := <-done:
		if status.Code(e) != codes.DeadlineExceeded {
			t.Fatalf("want deadline, got %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded metrics read")
	}
	waitCanceled(t, f, 4)
	assertOnlyReadCalls(t, calls)
}
func TestRuntimeOwnershipRaceAndMissingModeReturnNoFakeData(t *testing.T) {
	t.Run("ownership race", func(t *testing.T) {
		r, _, _ := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) { f.statusError = status.Error(codes.PermissionDenied, "owner changed") })
		s, e := r.metrics(context.Background())
		if status.Code(e) != codes.PermissionDenied || s.Available || s.Reason != "permission_denied" {
			t.Fatalf("%+v %v", s, e)
		}
	})
	t.Run("no mode support", func(t *testing.T) {
		r, f, _ := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) { f.modeError = status.Error(codes.NotFound, "clash mode not available") })
		s, e := r.mode(context.Background())
		if e != nil || s.Available || s.Reason != "mode_unavailable" || s.Current != "" {
			t.Fatalf("%+v %v", s, e)
		}
		waitCanceled(t, f, 1)
	})
}
func TestBackendRuntimeMissingTransportReturnsExplicitUnavailable(t *testing.T) {
	b := new(Backend)
	s, e := b.Runtime(context.Background())
	if e == nil || s.Metrics.Available || s.Groups.Available || s.Mode.Available || s.Metrics.Reason != "read_failed" {
		t.Fatalf("%+v %v", s, e)
	}
}

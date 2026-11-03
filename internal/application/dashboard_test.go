package application

import (
	"context"
	"fmt"
	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/rpc/gen/desktop"
	"testing"
	"time"
)

type fakeDashboardManaged struct {
	d.UnimplementedManagedServiceServer
	f *fakeRuntimeStarted
}

func (f *fakeDashboardManaged) GetSystemProxyStatus(context.Context, *emptypb.Empty) (*d.SystemProxyStatus, error) {
	return &d.SystemProxyStatus{Available: !f.f.proxyUnavailable}, nil
}
func (f *fakeDashboardManaged) SetSystemProxyEnabled(_ context.Context, r *d.SetSystemProxyEnabledRequest) (*emptypb.Empty, error) {
	f.f.actions <- fmt.Sprint("proxy:", r.Enabled)
	return &emptypb.Empty{}, nil
}
func (f *fakeRuntimeStarted) SetClashMode(_ context.Context, r *d.ClashMode) (*emptypb.Empty, error) {
	f.actions <- "mode:" + r.Mode
	return &emptypb.Empty{}, nil
}

func TestDashboardStreamSkipsFirstRateAndCancels(t *testing.T) {
	r, f, calls := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) {
		f.frames = []*d.Status{{TrafficAvailable: true, Uplink: 500, Downlink: 2000, UplinkTotal: 1000}}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var frames []MetricsSnapshot
	err := r.watchMetrics(ctx, func(m MetricsSnapshot) {
		frames = append(frames, m)
		if len(frames) == 2 {
			cancel()
		}
	})
	if err == nil || len(frames) != 2 || frames[0].RateAvailable || !frames[1].RateAvailable || frames[1].Uplink != 500 {
		t.Fatalf("frames=%+v err=%v", frames, err)
	}
	waitCanceled(t, f, 2)
	assertOnlyReadCalls(t, calls)
}
func TestDashboardControlsRequireOwnershipAndCapability(t *testing.T) {
	for _, owner := range []desktop.DaemonOwnership{desktop.DaemonOwnership_DAEMON_OWNERSHIP_OTHER, desktop.DaemonOwnership_DAEMON_OWNERSHIP_AVAILABLE} {
		r, f, _ := runtimeFixture(t, owner, nil)
		if r.setHTTPProxy(t.Context(), true) == nil || r.setMode(t.Context(), "global") == nil {
			t.Fatal("control accepted without ownership")
		}
		if len(f.actions) != 0 {
			t.Fatal("unauthorized mutation")
		}
	}
	r, f, _ := runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, nil)
	if err := r.setMode(t.Context(), "invalid"); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if err := r.setMode(t.Context(), "global"); err != nil {
		t.Fatal(err)
	}
	if got := <-f.actions; got != "mode:global" {
		t.Fatal(got)
	}
	if err := r.setHTTPProxy(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	if got := <-f.actions; got != "proxy:true" {
		t.Fatal(got)
	}
	r, f, _ = runtimeFixture(t, desktop.DaemonOwnership_DAEMON_OWNERSHIP_CALLER, func(f *fakeRuntimeStarted) { f.proxyUnavailable = true })
	if r.setHTTPProxy(t.Context(), true) == nil || len(f.actions) != 0 {
		t.Fatal("unsupported proxy mutated")
	}
}

package shell

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestDashboardVisibilityAndExplicitControls(t *testing.T) {
	calls := 0
	enabled := false
	s := New(Config{Callbacks: Callbacks{
		SetHTTPProxy: func(_ context.Context, v bool) error { calls++; enabled = v; return nil },
		Status: func(context.Context) (ServiceState, error) {
			return ServiceState{Running: true, Owned: true, HTTPProxy: &enabled}, nil
		},
	}})
	s.loaded = true
	tt := ui.NewTester(s.Render, 1000, 760)
	if tt.HasText("上传") || tt.HasText("状态") || tt.HasText("HTTP Proxy") {
		t.Fatal("disconnected dashboard showed runtime controls")
	}
	s.service = ServiceState{Running: true, Owned: true, HTTPProxy: &enabled}
	tt.Frame()
	if calls != 0 || !tt.HasText("上传") || !tt.HasText("下载") || !tt.HasText("HTTP Proxy") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("HTTP Proxy"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if calls != 1 || !enabled {
		t.Fatal("explicit toggle was not applied")
	}
	s.service.HTTPProxy = nil
	tt.Frame()
	if tt.HasText("HTTP Proxy") {
		t.Fatal("unsupported HTTP proxy remained visible")
	}
}
func TestDashboardMetricsHistoryAndShutdown(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	s := New(Config{Callbacks: Callbacks{WatchMetrics: func(ctx context.Context, emit func(Metrics)) error {
		close(started)
		<-ctx.Done()
		close(canceled)
		return ctx.Err()
	}}})
	s.loaded = true
	s.service.Running = true
	tt := ui.NewTester(s.Render, 1000, 760)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("subscription did not start")
	}
	for i := range 65 {
		s.metrics.events <- streamEvent[Metrics]{value: Metrics{RateAvailable: true, UploadBytes: float64(i), DownloadBytes: float64(i * 2)}}
		tt.Frame()
	}
	if len(s.uploadHistory) != 60 || s.uploadHistory[0] != 5 || s.downloadHistory[59] != 128 {
		t.Fatal("invalid history window")
	}
	s.service.Running = false
	tt.Frame()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("subscription not canceled")
	}
	if len(s.uploadHistory) != 0 || len(s.downloadHistory) != 0 {
		t.Fatal("stopped core retained stale chart")
	}
}

func TestStatusRefreshKeepsHealthySubscriptionAndRebuildsFailedOne(t *testing.T) {
	subscriptions := make(chan struct{}, 4)
	s := New(Config{Callbacks: Callbacks{WatchMetrics: func(ctx context.Context, emit func(Metrics)) error {
		subscriptions <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}}})
	s.loaded = true
	s.service.Running = true
	tt := ui.NewTester(s.Render, 1000, 760)
	<-subscriptions
	live := Metrics{RateAvailable: true, Upload: "1.0 KB/s", UploadBytes: 1024}
	for range 3 {
		s.metrics.events <- streamEvent[Metrics]{value: live}
		tt.Frame()
	}
	// A status refresh (e.g. after an explicit control click) carries a
	// one-frame snapshot without a measured rate; it must not reset the chart.
	s.results <- result{service: &ServiceState{Running: true, Owned: true, Metrics: &Metrics{}}}
	tt.Frame()
	if len(s.uploadHistory) != 3 || s.service.Metrics == nil || *s.service.Metrics != live || len(subscriptions) != 0 {
		t.Fatalf("refresh disturbed healthy stream: history=%d", len(s.uploadHistory))
	}
	s.metrics.events <- streamEvent[Metrics]{epoch: s.metrics.epoch, err: context.DeadlineExceeded}
	tt.Frame()
	s.results <- result{service: &ServiceState{Running: true, Owned: true}}
	tt.Frame()
	select {
	case <-subscriptions:
	case <-time.After(time.Second):
		t.Fatal("failed subscription was not rebuilt by a status refresh")
	}
	s.Close()
}

func TestModeCardNeedsAChoice(t *testing.T) {
	var chosen string
	s := New(Config{Callbacks: Callbacks{
		SetMode: func(_ context.Context, mode string) error { chosen = mode; return nil },
		Status: func(context.Context) (ServiceState, error) {
			return ServiceState{Running: true, Owned: true, ClashMode: chosen, ModeOptions: []string{"Rule", "Global"}}, nil
		},
	}})
	s.loaded = true
	s.service = ServiceState{Running: true, Owned: true, ClashMode: "Rule", ModeOptions: []string{"Rule"}}
	tt := ui.NewTester(s.Render, 1000, 760)
	if tt.HasText("模式") || tt.HasText("Rule") {
		t.Fatal("a single mode must not show a picker")
	}
	s.service.ModeOptions = []string{"Rule", "Global"}
	tt.Frame()
	if !tt.HasText("模式") || !tt.HasText("Global") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Global"); err != nil {
		t.Fatal(err)
	}
	complete(t, s, tt)
	if chosen != "Global" || s.service.ClashMode != "Global" {
		t.Fatalf("mode not applied: %q", chosen)
	}
	s.Close()
}

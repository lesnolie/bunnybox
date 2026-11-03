package application

import "testing"

func TestFirstFrameDoesNotInventRate(t *testing.T) {
	m := uiMetrics(MetricsSnapshot{MemoryBytes: 1024, Goroutines: 4, TrafficAvailable: true, UplinkTotal: 1234, DownlinkTotal: 5678})
	if m.Upload != "" || m.Download != "" {
		t.Fatal("first frame has no valid measurement interval")
	}
	if m.UploadTotal == "" || m.DownloadTotal == "" || m.Memory != "1.0 KB" {
		t.Fatal("valid current values must remain visible")
	}
}

func TestUnavailableTrafficDoesNotShowZeroTotals(t *testing.T) {
	m := uiMetrics(MetricsSnapshot{TrafficAvailable: false})
	if m.UploadTotal != "" || m.DownloadTotal != "" {
		t.Fatal("missing traffic stats must not be labelled as observed zero")
	}
}

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.14.5", "1.14.4", true},
		{"1.14.4", "1.14.4", false},
		{"1.14.4", "1.14.5", false},
		{"1.15.0", "1.14.9", true},
		{"1.15.0", "1.15.0-beta.3", true},
		{"1.15.0-beta.3", "1.15.0", false},
		{"1.15.0-beta.10", "1.15.0-beta.9", true},
		{"1.15.0-rc.1", "1.15.0-beta.9", true},
		{"v1.14.5", "1.14.4", true},
		{"garbage", "1.14.4", false},
	} {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

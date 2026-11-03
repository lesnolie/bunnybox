package shell

import (
	"slices"
	"testing"
)

func TestCardDefaultsFollowApple(t *testing.T) {
	cfg := newCardConfig(nil, nil)
	if !slices.Equal(cfg.ordered(), defaultCardOrder) {
		t.Fatalf("ordered = %v", cfg.ordered())
	}
	rows := cardRows(cfg.ordered())
	want := [][]string{{cardUpload, cardDownload}, {cardStatus, cardConnections}, {cardHTTPProxy}, {cardClashMode}, {cardProfile}}
	if len(rows) != len(want) {
		t.Fatalf("rows = %v", rows)
	}
	for i := range want {
		if !slices.Equal(rows[i], want[i]) {
			t.Fatalf("row %d = %v, want %v", i, rows[i], want[i])
		}
	}
}

func TestCardConfigNormalizesSaved(t *testing.T) {
	// Old names migrate, the profile card is always on, unknown cards drop
	// and cards missing from the order are appended.
	cfg := newCardConfig([]string{"traffic", "status", "bogus"}, []string{"profile", "status", "traffic"})
	if !slices.Equal(cfg.enabled, []string{cardUpload, cardStatus, cardProfile}) {
		t.Fatalf("enabled = %v", cfg.enabled)
	}
	if !slices.Equal(cfg.order[:3], []string{cardProfile, cardStatus, cardUpload}) || len(cfg.order) != len(allCards) {
		t.Fatalf("order = %v", cfg.order)
	}
	if !slices.Equal(cfg.ordered(), []string{cardProfile, cardStatus, cardUpload}) {
		t.Fatalf("ordered = %v", cfg.ordered())
	}
}

func TestCardToggleAndMove(t *testing.T) {
	cfg := newCardConfig(nil, nil)
	cfg.toggle(cardProfile)
	if !slices.Contains(cfg.enabled, cardProfile) {
		t.Fatal("profile card turned off")
	}
	cfg.toggle(cardDownload)
	// Upload alone takes a row of its own.
	if rows := cardRows(cfg.ordered()); len(rows[0]) != 1 || rows[0][0] != cardUpload {
		t.Fatalf("rows = %v", rows)
	}
	cfg.move(cardUpload, -1) // already first
	cfg.move(cardProfile, 1) // already last
	cfg.move(cardStatus, -1)
	if cfg.order[1] != cardStatus || cfg.order[2] != cardDownload {
		t.Fatalf("order = %v", cfg.order)
	}
}

func TestCardVisibility(t *testing.T) {
	s := New(Config{})
	if !s.cardVisible(cardProfile) || s.cardVisible(cardUpload) {
		t.Fatal("stopped core: only the profile card shows")
	}
	s.service.Running = true
	if !s.cardVisible(cardUpload) || s.cardVisible(cardHTTPProxy) || s.cardVisible(cardClashMode) {
		t.Fatal("running core without proxy or modes")
	}
	on := false
	s.service.HTTPProxy, s.service.ModeOptions = &on, []string{"Rule", "Global"}
	if !s.cardVisible(cardHTTPProxy) || !s.cardVisible(cardClashMode) {
		t.Fatal("running core with proxy and modes")
	}
}

func TestBitrateMatchesCore(t *testing.T) {
	for in, want := range map[int64]string{0: "", 999: "999 bps", 1500: "1.5 Kbps", 25_300_000: "25.3 Mbps", 1_200_000_000: "1.2 Gbps"} {
		if got := bitrate(in); got != want {
			t.Errorf("bitrate(%d) = %q, want %q", in, got, want)
		}
	}
}

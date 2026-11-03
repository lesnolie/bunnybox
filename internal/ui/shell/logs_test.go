package shell

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestLogsFollowAppleLogView(t *testing.T) {
	emits := make(chan func(LogFrame), 1)
	canceled := make(chan struct{})
	clears := 0
	s := New(Config{Callbacks: Callbacks{
		WatchLogs: func(ctx context.Context, emit func(LogFrame)) error {
			emit(LogFrame{Reset: true, DefaultLevel: 4, Lines: []LogLine{
				{2, "\x1b[31mERROR\x1b[0m boom"}, {4, "info line"}, {5, "debug line"},
			}})
			emits <- emit
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		},
		ClearLogs: func(context.Context) error { clears++; return nil },
	}})
	s.loaded, s.page, s.service.Running = true, "logs", true
	tt := ui.NewTester(s.Render, 1000, 760)
	var emit func(LogFrame)
	select {
	case emit = <-emits:
	case <-time.After(time.Second):
		t.Fatal("log subscription did not start")
	}
	tt.Frame()
	if !tt.HasText("info line") || tt.HasText("debug line") {
		t.Fatal("the core's default level must filter the view", tt.Texts())
	}
	menu := func(path ...string) {
		t.Helper()
		if err := tt.Click("日志选项"); err != nil {
			t.Fatal(err)
		}
		for _, label := range path {
			if err := tt.Click(label); err != nil {
				t.Fatal(err, tt.Texts())
			}
		}
		tt.Frame()
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
	menu("日志级别", "Debug")
	if !tt.HasText("debug line") {
		t.Fatal("level filter not applied", tt.Texts())
	}
	click("搜索")
	click("搜索日志")
	tt.Type("boom")
	tt.Frame()
	if tt.HasText("info line") {
		t.Fatal("search did not filter", tt.Texts())
	}
	click("搜索") // closing search clears it
	click("暂停")
	emit(LogFrame{DefaultLevel: 4, Lines: []LogLine{{4, "late line"}}})
	tt.Frame()
	if tt.HasText("late line") {
		t.Fatal("paused view must keep its snapshot")
	}
	click("继续")
	if !tt.HasText("late line") {
		t.Fatal("resume did not show new lines", tt.Texts())
	}
	menu("保存", "到剪贴板")
	if got, want := tt.Clipboard(), "ERROR boom\ninfo line\ndebug line\nlate line"; got != want {
		t.Fatalf("clipboard=%q want %q", got, want)
	}
	menu("清除日志")
	complete(t, s, tt)
	if clears != 1 || !tt.HasText("日志为空") {
		t.Fatal("clear did not empty the view and the service", clears, tt.Texts())
	}
	click("概览")
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("leaving the page did not cancel the log subscription")
	}
}

func TestLogsWithoutServiceKeepLastLines(t *testing.T) {
	s := New(Config{})
	s.loaded, s.page = true, "logs"
	tt := ui.NewTester(s.Render, 1000, 760)
	if !tt.HasText("服务未启动") {
		t.Fatal(tt.Texts())
	}
	s.logs.apply(LogFrame{Reset: true, DefaultLevel: 4, Lines: []LogLine{{2, "start failed"}}})
	tt.Frame()
	if !tt.HasText("start failed") {
		t.Fatal("the last lines must stay after the core stops", tt.Texts())
	}
}

func TestLogBufferAndANSI(t *testing.T) {
	var v logView
	for i := 0; i < logMaxLines+5; i++ {
		v.apply(LogFrame{DefaultLevel: 6, Lines: []LogLine{{4, "x"}}})
	}
	if len(v.lines) != logMaxLines || v.lines[0].id != 6 {
		t.Fatal("buffer must keep the newest lines", len(v.lines), v.lines[0].id)
	}
	spans := ansiSpans("\x1b[36mINFO\x1b[0m [123] router: \x1b[1;31mfail\x1b[0m", logPalette(ui.Hex("#FFFFFF")))
	if len(spans) != 3 || spans[0].Text != "INFO" || spans[1].Color != (ui.Color{}) || spans[2].Weight != 700 || spans[2].Text != "fail" {
		t.Fatalf("spans=%+v", spans)
	}
	white, black := [3]float64{1, 1, 1}, [3]float64{0, 0, 0}
	yellow := [3]float64{0.9, 0.9, 0}
	if contrast(adjustContrast(yellow, white), white) < 4.5 || adjustContrast(yellow, black) != yellow {
		t.Fatal("contrast adjustment")
	}
	hl := highlightSpans([]ui.Span{{Text: "a boom b boom"}}, "boom", ui.Hex("#FFCC00"))
	if len(hl) != 4 || hl[1].Text != "boom" || hl[1].Background == (ui.Color{}) || hl[2].Text != " b " {
		t.Fatalf("highlight=%+v", hl)
	}
}

func TestLogResumeReturnsToTheEnd(t *testing.T) {
	s := New(Config{})
	s.loaded, s.page = true, "logs"
	s.logs.apply(LogFrame{Reset: true, DefaultLevel: 4, Lines: []LogLine{{4, "first"}}})
	tt := ui.NewTester(s.Render, 1000, 760)
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	lines := make([]LogLine, 200)
	for i := range lines {
		lines[i] = LogLine{4, fmt.Sprintf("late line %d", i)}
	}
	s.logs.apply(LogFrame{DefaultLevel: 4, Lines: lines})
	tt.Frame()
	if err := tt.Click("继续"); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		tt.Frame()
	}
	if !tt.HasText("late line 199") {
		t.Fatal("resume must show the newest line", tt.Texts())
	}
}

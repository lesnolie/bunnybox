package shell

import (
	"context"
	"math"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Limits of the Apple client's CommandClient and LogDataModel.
const (
	logMaxLines   = 3000
	logMaxVisible = 1000
)

var logLevels = []struct {
	level int
	name  string
}{{2, "Error"}, {3, "Warn"}, {4, "Info"}, {5, "Debug"}, {6, "Trace"}}

type logEntry struct {
	LogLine
	id uint64 // keeps rows in place as the buffer drops its oldest lines
}

// logView mirrors Apple's LogViewModel: a level filter (0 follows the core's
// default), a search filter, and pause, which freezes a snapshot and stops
// following the end.
type logView struct {
	lines        []logEntry
	nextID       uint64
	defaultLevel int
	level        int
	received     bool
	paused       bool
	snapshot     []logEntry
	searching    bool
	query        string
	list         ui.ListState

	rev      uint64
	key      logFilterKey
	filtered []logEntry
}

type logFilterKey struct {
	rev    uint64
	paused bool
	level  int
	query  string
	valid  bool
}

func (v *logView) apply(f LogFrame) {
	v.received = true
	v.defaultLevel = f.DefaultLevel
	if f.Reset {
		v.lines = nil
	}
	for _, line := range f.Lines {
		v.nextID++
		v.lines = append(v.lines, logEntry{line, v.nextID})
	}
	if over := len(v.lines) - logMaxLines; over > 0 {
		v.lines = append([]logEntry(nil), v.lines[over:]...)
	}
	v.rev++
}

func (v *logView) effectiveLevel() int {
	if v.level != 0 {
		return v.level
	}
	return v.defaultLevel
}

func (v *logView) setPaused(paused bool) {
	v.paused, v.snapshot = paused, nil
	if paused {
		v.snapshot = v.lines // apply never writes into a published slice
	} else {
		// FollowEnd only follows an end in view; resuming jumps to it, as
		// Apple's auto-scroll does.
		v.list.ScrollToEnd()
	}
}

// filter returns the lines that pass the level and search filters.
func (v *logView) filter() []logEntry {
	key := logFilterKey{v.rev, v.paused, v.effectiveLevel(), v.query, true}
	if key == v.key {
		return v.filtered
	}
	source := v.lines
	if v.paused {
		source = v.snapshot
	}
	v.filtered = v.filtered[:0]
	for _, e := range source {
		if e.Level <= key.level && (key.query == "" || strings.Contains(e.Message, key.query)) {
			v.filtered = append(v.filtered, e)
		}
	}
	v.key = key
	return v.filtered
}

func (v *logView) text() string {
	var b strings.Builder
	for i, e := range v.filter() {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ansiCode.ReplaceAllString(e.Message, ""))
	}
	return b.String()
}

func (s *Shell) logPage(c *ui.Context, t *ui.Theme) {
	v := &s.logs
	cb := s.config.Callbacks
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if v.searching {
			ui.TextInput(c, &v.query).Label(s.text("搜索日志", "Search logs")).Placeholder(s.text("搜索", "Search")).Grow(1)
		} else {
			ui.Column(c).Grow(1)
		}
		if iconButton(c, icons["search"], s.text("搜索", "Search")).Clicked() {
			v.searching = !v.searching
			if !v.searching {
				v.query = ""
			}
		}
		pause, label := icons["pause"], s.text("暂停", "Pause")
		if v.paused {
			pause, label = icons["play"], s.text("继续", "Resume")
		}
		if iconButton(c, pause, label).Clicked() {
			v.setPaused(!v.paused)
		}
		ui.Button(c, "").Label(s.text("日志选项", "Log options")).Width(32).Height(32).Background(ui.Transparent).Border(0, ui.Transparent).Children(func() { ui.Icon(c, icons["more"]).FontSize(18) }).PopoverMenuButton(func(m *ui.Menu) {
			m.Submenu(s.text("日志级别", "Log Level"), func(m *ui.Menu) {
				if m.Item(s.text("默认", "Default")).Radio(v.level == 0).Chosen() {
					v.level = 0
				}
				for _, l := range logLevels {
					if m.Item(l.name).Radio(v.level == l.level).Chosen() {
						v.level = l.level
					}
				}
			})
			m.Submenu(s.text("保存", "Save"), func(m *ui.Menu) {
				if m.Item(s.text("到剪贴板", "To Clipboard")).Chosen() {
					c.WriteClipboard(v.text())
				}
				if m.Item(s.text("到文件", "To File")).Disabled(s.busy || cb.SaveFile == nil).Chosen() {
					text := v.text()
					s.groupAction(c, func(ctx context.Context) error { return cb.SaveFile(ctx, "logs.txt", []byte(text)) })
				}
			})
			if m.Item(s.text("清除日志", "Clear Logs")).Disabled(s.busy).Chosen() {
				s.clearLogs(c)
			}
		})
	})
	lines := v.filter()
	if len(lines) == 0 {
		label := s.text("服务未启动", "Service not started")
		switch {
		case s.logStream.started && !s.logStream.failed && !v.received:
			label = s.text("加载中…", "Loading…")
		case v.received || s.service.Running:
			label = s.text("日志为空", "Empty logs")
		}
		ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, label).TextColor(t.TextMuted)
		})
		return
	}
	if len(lines) > logMaxVisible {
		lines = lines[len(lines)-logMaxVisible:]
	}
	v.list.FollowEnd = !v.paused
	v.list.Key = func(i int) any { return lines[i].id }
	palette := logPalette(t.Surface)
	highlight := pick(t, "#FFCC00", "#FFD60A")
	ui.List(c, &v.list, len(lines), func(i int) {
		spans := highlightSpans(ansiSpans(lines[i].Message, palette), v.query, highlight)
		ui.RichText(c, spans...).Font("Consolas").FontSize(12).TextColor(t.Text).Padding(1, 12)
	}).Label(s.text("日志", "Logs")).Grow(1).Radius(12).Background(t.Surface).Padding(8, 0)
}

// clearLogs follows Apple's LogDataModel.clearLogs: resume, empty the view
// and ask the service to drop its saved lines.
func (s *Shell) clearLogs(c *ui.Context) {
	v := &s.logs
	v.setPaused(false)
	v.lines = nil
	v.rev++
	if s.config.Callbacks.ClearLogs != nil && s.service.Running {
		s.groupAction(c, s.config.Callbacks.ClearLogs)
	}
}

var ansiCode = regexp.MustCompile("\x1b\\[[;\\d]*m")

// logPalette is Apple's ANSIColors palette, adjusted for contrast against the
// log background as its LogTextView does.
type ansiPalette map[string]ui.Color

func logPalette(background ui.Color) ansiPalette {
	base := map[string][3]float64{
		"30": {0, 0, 0}, "31": {1.0, 0.13, 0.35}, "32": {0.18, 0.8, 0.44}, "33": {0.9, 0.9, 0.0},
		"34": {0.2, 0.6, 0.86}, "35": {0.61, 0.35, 0.71}, "36": {0.36, 0.68, 0.89}, "37": {0.93, 0.94, 0.95},
	}
	bg := [3]float64{float64(background.R) / 255, float64(background.G) / 255, float64(background.B) / 255}
	p := ansiPalette{}
	for code, rgb := range base {
		p[code] = toColor(adjustContrast(rgb, bg))
	}
	return p
}

// ansiSpans splits a message into spans styled by its SGR codes, following
// Apple's ANSIColors.parseAnsiString: a code without a known attribute, or
// 0, ends the current style.
func ansiSpans(message string, p ansiPalette) []ui.Span {
	matches := ansiCode.FindAllStringIndex(message, -1)
	if len(matches) == 0 {
		return []ui.Span{{Text: message}}
	}
	var spans []ui.Span
	style := ui.Span{}
	last := 0
	for _, m := range matches {
		if m[0] > last {
			span := style
			span.Text = message[last:m[0]]
			spans = append(spans, span)
		}
		style = parseSGR(message[m[0]+2:m[1]-1], p)
		last = m[1]
	}
	if last < len(message) {
		span := style
		span.Text = message[last:]
		spans = append(spans, span)
	}
	return spans
}

func parseSGR(codes string, p ansiPalette) ui.Span {
	var span ui.Span
	for _, code := range strings.Split(codes, ";") {
		switch code {
		case "0":
			return ui.Span{}
		case "1":
			span.Weight = 700
		case "3":
			span.Italic = true
		case "4":
			span.Underline = true
		default:
			if c, ok := p[code]; ok {
				span.Color = c
			}
		}
	}
	return span
}

// highlightSpans marks occurrences of query, as Apple's search does.
func highlightSpans(spans []ui.Span, query string, highlight ui.Color) []ui.Span {
	if query == "" {
		return spans
	}
	var out []ui.Span
	for _, span := range spans {
		text := span.Text
		for {
			i := strings.Index(text, query)
			if i < 0 {
				break
			}
			if i > 0 {
				before := span
				before.Text = text[:i]
				out = append(out, before)
			}
			hit := span
			hit.Text, hit.Background = text[i:i+len(query)], highlight
			out = append(out, hit)
			text = text[i+len(query):]
		}
		if text != "" {
			rest := span
			rest.Text = text
			out = append(out, rest)
		}
	}
	return out
}

// adjustContrast mirrors Apple's adjustedForContrast (minimum ratio 4.5).
func adjustContrast(c, bg [3]float64) [3]float64 {
	const minRatio = 4.5
	if contrast(c, bg) >= minRatio {
		return c
	}
	darken := luminance(bg) > 0.5
	low, high, best := 0.0, 1.0, c
	for range 10 {
		mid := (low + high) / 2
		var a [3]float64
		for i, v := range c {
			if darken {
				a[i] = v * (1 - mid)
			} else {
				a[i] = v + (1-v)*mid
			}
		}
		if contrast(a, bg) >= minRatio {
			best, high = a, mid
		} else {
			low = mid
		}
	}
	return best
}

func luminance(c [3]float64) float64 {
	var l [3]float64
	for i, v := range c {
		if v <= 0.03928 {
			l[i] = v / 12.92
		} else {
			l[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*l[0] + 0.7152*l[1] + 0.0722*l[2]
}

func contrast(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

func toColor(c [3]float64) ui.Color {
	return ui.RGB(uint8(math.Round(c[0]*255)), uint8(math.Round(c[1]*255)), uint8(math.Round(c[2]*255)))
}

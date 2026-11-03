package shell

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
)

// dashboard is Apple's OverviewView: the enabled cards in the user's order,
// traffic and statistics cards paired in rows. Runtime cards show only while
// the core is connected; the profile card always shows.
func (s *Shell) dashboard(c *ui.Context, t *ui.Theme) {
	var visible []string
	for _, id := range s.cards.ordered() {
		if s.cardVisible(id) {
			visible = append(visible, id)
		}
	}
	for _, row := range cardRows(visible) {
		if len(row) == 2 {
			ui.Row(c).Gap(20).Children(func() {
				s.dashboardCard(c, t, row[0])
				s.dashboardCard(c, t, row[1])
			})
			continue
		}
		s.dashboardCard(c, t, row[0])
	}
	if m := s.serviceMessage(); m != "" {
		ui.Text(c, m).FontSize(12).TextColor(t.TextMuted)
	}
}

func (s *Shell) cardVisible(id string) bool {
	switch id {
	case cardProfile:
		return true
	case cardHTTPProxy:
		return s.service.Running && s.service.HTTPProxy != nil
	case cardClashMode:
		// Apple's ClashModeCard shows a picker only when there is a choice.
		return s.service.Running && len(s.service.ModeOptions) > 1
	default:
		return s.service.Running
	}
}

func (s *Shell) dashboardCard(c *ui.Context, t *ui.Theme, id string) {
	m := s.service.Metrics
	if m == nil {
		m = &Metrics{}
	}
	switch id {
	case cardUpload:
		s.trafficCard(c, t, s.text("上传", "Upload"), "upload", m.Upload, m.UploadTotal, s.uploadHistory)
	case cardDownload:
		s.trafficCard(c, t, s.text("下载", "Download"), "download", m.Download, m.DownloadTotal, s.downloadHistory)
	case cardStatus:
		s.metric(c, t, s.text("状态", "Status"), "info", s.text("内存", "Memory"), m.Memory, s.text("协程", "Goroutines"), m.Goroutines)
	case cardConnections:
		s.metric(c, t, s.text("连接", "Connections"), "connections", s.text("入站", "Inbound"), m.Inbound, s.text("出站", "Outbound"), m.Outbound)
	case cardHTTPProxy:
		ui.Row(c).Padding(16, 20).Radius(16).Background(t.Surface).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.text("系统 HTTP 代理", "System HTTP Proxy")).FontSize(16).FontWeight(600).Grow(1)
			enabled := *s.service.HTTPProxy
			if ui.Switch(c, &enabled).Label("HTTP Proxy").Disabled(s.busy || !s.service.Owned || s.config.Callbacks.SetHTTPProxy == nil).Changed() {
				s.controlAction(c, func(ctx context.Context) error { return s.config.Callbacks.SetHTTPProxy(ctx, enabled) })
			}
		})
	case cardClashMode:
		ui.Column(c).Padding(20).Gap(12).Radius(18).Background(t.Surface).Children(func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons["mode"]).FontSize(18)
				ui.Text(c, s.text("模式", "Mode")).FontSize(17).FontWeight(600)
			})
			ui.Row(c).Padding(3).Radius(9).Background(t.Background).Children(func() {
				for _, mode := range s.service.ModeOptions {
					fill := ui.Transparent
					if mode == s.service.ClashMode {
						fill = t.Surface
					}
					if hoverFill(ui.Button(c, mode).Grow(1).Radius(7), fill).Border(0, ui.Transparent).Disabled(s.busy || !s.service.Owned || s.config.Callbacks.SetMode == nil).Clicked() && mode != s.service.ClashMode {
						selected := mode
						s.controlAction(c, func(ctx context.Context) error { return s.config.Callbacks.SetMode(ctx, selected) })
					}
				}
			})
		})
	case cardProfile:
		s.dashboardProfile(c, t)
	}
}

// serviceMessages translates the state codes ServiceState.Message carries;
// any other text is shown as it is.
var serviceMessages = map[string][2]string{
	"available":          {"服务可用，尚未连接", "Service available, not connected"},
	"other_user":         {"服务正被其他用户使用", "Service is in use by another user"},
	"core_running":       {"核心运行中", "Core running"},
	"core_idle":          {"服务可用，核心未启动", "Service available, core not started"},
	"core_starting":      {"核心正在启动", "Core starting"},
	"core_stopping":      {"核心正在停止", "Core stopping"},
	"core_failed":        {"核心启动失败", "Core failed to start"},
	"unknown_core_state": {"核心状态未知", "Core state unknown"},
}

func (s *Shell) serviceMessage() string {
	if m, ok := serviceMessages[s.service.Message]; ok {
		return s.text(m[0], m[1])
	}
	return s.service.Message
}

func (s *Shell) trafficCard(c *ui.Context, t *ui.Theme, title, icon, rate, total string, history []float32) {
	ui.Column(c).Grow(1).Height(208).Padding(20).Gap(10).Radius(18).Background(t.Surface).Children(func() {
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() { ui.Icon(c, icons[icon]).FontSize(18); ui.Text(c, title).FontSize(17).FontWeight(600) })
		ui.Text(c, value(rate)).FontSize(26).FontWeight(600)
		ui.Text(c, value(total)).FontSize(14).TextColor(t.TextMuted)
		ui.Box(c).Grow(1).MinHeight(58).Draw(func(p *ui.Painter, r ui.Rect) {
			var grid ui.Path
			grid.Dash(0, 5, 5)
			for i := 0; i <= 3; i++ {
				y := r.Y + r.H*float32(i)/3
				grid.MoveTo(r.X, y)
				grid.LineTo(r.X+r.W, y)
			}
			p.StrokePath(&grid, 1, t.TextMuted.Alpha(.25))
			if len(history) < 2 {
				return
			}
			peak := float32(1)
			for _, v := range history {
				peak = max(peak, v*1.2)
			}
			xs, ys := make([]float32, len(history)), make([]float32, len(history))
			for i, v := range history {
				xs[i] = float32(i) / float32(len(history)-1)
				ys[i] = v / peak
			}
			p.FillPath(chart.Area(r, xs, nil, ys, chart.CurveLinear), t.Text.Alpha(.08))
			p.StrokePath(chart.Line(r, xs, ys, chart.CurveLinear), 2, t.Text)
		})
	})
}

func dashboardAction(c *ui.Context, t *ui.Theme, icon *ui.SVG, label string) ui.Element {
	return ui.Button(c, "").Label(label).Width(46).Height(36).Padding(9, 13).Radius(9).Background(t.Background).Border(0, ui.Transparent).Children(func() { ui.Icon(c, icon).FontSize(19) })
}
func (s *Shell) controlAction(c *ui.Context, action func(context.Context) error) {
	s.run(c, func(ctx context.Context) result {
		if err := action(ctx); err != nil {
			return result{err: err}
		}
		if s.config.Callbacks.Status == nil {
			return result{err: errors.New("无法读取更新后的服务状态 / Cannot read updated service state")}
		}
		state, err := s.config.Callbacks.Status(ctx)
		return result{service: &state, err: err}
	})
}

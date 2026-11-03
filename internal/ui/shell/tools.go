package shell

import (
	"context"
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Network tools follow the Apple client's ToolsView, Network section. The
// tests run in the started core through its outbound, as on macOS while
// connected. Apple's Endpoints (Tailscale, OpenConnect, OpenVPN) and USB/IP
// sections appear only for configurations that have them, and its Debug
// section reads Apple crash reports; neither has a counterpart here yet.

// Defaults of the locked core (common/networkquality, common/stun).
const (
	defaultNetworkQualityURL = "https://mensura.cdn-apple.com/api/v1/gm/config"
	defaultSTUNServer        = "stun.voipgate.com:3478"
)

// Phases and values of the locked core's tests.
const (
	nqPhaseIdle, nqPhaseDownload, nqPhaseUpload, nqPhaseDone = 0, 1, 2, 3
	stunPhaseBinding, stunPhaseMapping, stunPhaseFiltering   = 0, 1, 2
	stunPhaseDone                                            = 3
)

type NetworkQualityRequest struct {
	ConfigURL, Outbound string
	Serial, HTTP3       bool
	MaxRuntimeSeconds   int
}

type NetworkQualityProgress struct {
	Phase, IdleLatencyMS                   int
	Download, Upload                       int64 // bits per second
	DownloadRPM, UploadRPM                 int
	DownloadAccuracy, UploadAccuracy       int // 0 low, 1 medium, 2 high
	DownloadRPMAccuracy, UploadRPMAccuracy int
	Final                                  bool
	Error                                  string
}

type STUNRequest struct{ Server, Outbound string }

type STUNProgress struct {
	Phase                    int
	ExternalAddr             string
	LatencyMS                int
	NATMapping, NATFiltering int // 0 unknown, 1 endpoint independent, 2 address, 3 address and port dependent
	NATTypeSupported, Final  bool
	Error                    string
}

type toolsState struct {
	page      string // "", "nq" or "stun"
	outbounds []string
	nqURL     string
	nqSerial  bool
	nqHTTP3   bool
	nqRuntime int
	nqOut     string
	nq        *NetworkQualityProgress
	nqStream  *stream[NetworkQualityProgress]
	stunHost  string
	stunOut   string
	stun      *STUNProgress
	stunRun   *stream[STUNProgress]
}

func newToolsState() toolsState {
	return toolsState{nqURL: defaultNetworkQualityURL, nqRuntime: 30, stunHost: defaultSTUNServer,
		nqStream: newStream[NetworkQualityProgress](), stunRun: newStream[STUNProgress]()}
}

// updateTools applies test progress and cancels a test when its page closes
// or the core stops, as Apple's tools do.
func (s *Shell) updateTools() {
	tl := &s.tools
	if !s.service.Running || s.page != "tools" || tl.page != "nq" {
		tl.nqStream.stop()
	}
	if !s.service.Running || s.page != "tools" || tl.page != "stun" {
		tl.stunRun.stop()
	}
	if !s.service.Running {
		tl.outbounds, tl.nqOut, tl.stunOut = nil, "", ""
	}
	tl.nqStream.drain(func(p NetworkQualityProgress) {
		tl.nq = &p
		if p.Error != "" {
			s.errorText = p.Error
		}
		if p.Final {
			tl.nqStream.stop()
		}
	}, func(err error) {
		s.errorText = err.Error()
		tl.nqStream.stop()
	})
	tl.stunRun.drain(func(p STUNProgress) {
		tl.stun = &p
		if p.Error != "" {
			s.errorText = p.Error
		}
		if p.Final {
			tl.stunRun.stop()
		}
	}, func(err error) {
		s.errorText = err.Error()
		tl.stunRun.stop()
	})
}

func (s *Shell) toolsPage(c *ui.Context, t *ui.Theme) {
	switch s.tools.page {
	case "nq":
		s.networkQualityPage(c, t)
		return
	case "stun":
		s.stunPage(c, t)
		return
	}
	s.card(c, t, func() {
		ui.Text(c, s.text("网络", "Network")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		s.toolLink(c, t, "nq", "download", s.text("网络质量", "Network Quality"))
		s.toolLink(c, t, "stun", "connections", s.text("STUN 测试", "STUN Test"))
	})
}

func (s *Shell) toolLink(c *ui.Context, t *ui.Theme, page, icon, label string) {
	if hoverFill(ui.Button(c, "").Label(label).Height(40).Padding(8, 6).Justify(ui.Start), ui.Transparent).Border(0, ui.Transparent).Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(t.Accent)
			ui.Text(c, label).FontSize(14).Grow(1)
			ui.Text(c, "›").FontSize(18).TextColor(t.TextMuted)
		})
	}).Clicked() {
		s.tools.page, s.errorText = page, ""
		s.loadOutbounds(c)
	}
}

func (s *Shell) toolHeader(c *ui.Context, t *ui.Theme, title string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if hoverFill(ui.Button(c, "‹ "+s.text("工具", "Tools")).Label(s.text("返回工具", "Back to Tools")), ui.Transparent).Border(0, ui.Transparent).TextColor(t.Accent).Clicked() {
			s.tools.page, s.errorText = "", ""
		}
		ui.Text(c, title).FontSize(17).FontWeight(600)
	})
}

// loadOutbounds fills the outbound picker while the core runs.
func (s *Shell) loadOutbounds(c *ui.Context) {
	get := s.config.Callbacks.Outbounds
	if get == nil || !s.service.Running {
		return
	}
	s.run(c, func(ctx context.Context) result {
		list, err := get(ctx)
		return result{outbounds: &list, err: err, ack: true}
	})
}

// toolsNotice says why a test cannot start.
func (s *Shell) toolsNotice(c *ui.Context, t *ui.Theme) {
	if !s.service.Running {
		ui.Text(c, s.text("测试经由核心的出站进行，请先启动核心。", "Tests run through the core's outbound; start the core first.")).FontSize(12).TextColor(t.TextMuted)
	}
}

func (s *Shell) outboundPicker(c *ui.Context, t *ui.Theme, selected *string, disabled bool) {
	if !s.service.Running {
		return
	}
	formRow(c, t, s.text("出站", "Outbound"), func() {
		label := *selected
		if label == "" {
			label = s.text("默认", "Default")
		}
		ui.PopoverMenuButton(c, label, func(m *ui.Menu) {
			if m.Item(s.text("默认", "Default")).Radio(*selected == "").Chosen() {
				*selected = ""
			}
			for _, o := range s.tools.outbounds {
				if m.Item(o).Radio(*selected == o).Chosen() {
					*selected = o
				}
			}
		}).Label(s.text("出站", "Outbound")).Disabled(disabled)
	})
}

func (s *Shell) networkQualityPage(c *ui.Context, t *ui.Theme) {
	tl := &s.tools
	running := tl.nqStream.started
	s.toolHeader(c, t, s.text("网络质量", "Network Quality"))
	s.card(c, t, func() {
		ui.Text(c, s.text("配置", "Configuration")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		formRow(c, t, "URL", func() {
			ui.TextInput(c, &tl.nqURL).Label("URL").Grow(1).Disabled(running)
		})
		formRow(c, t, s.text("串行", "Serial"), func() {
			ui.Switch(c, &tl.nqSerial).Label(s.text("串行", "Serial")).Disabled(running)
		})
		formRow(c, t, "HTTP/3", func() {
			ui.Switch(c, &tl.nqHTTP3).Label("HTTP/3").Disabled(running)
		})
		formRow(c, t, s.text("最长时间", "Max Runtime"), func() {
			ui.PopoverMenuButton(c, fmt.Sprintf("%ds", tl.nqRuntime), func(m *ui.Menu) {
				for _, v := range []int{30, 60} {
					if m.Item(fmt.Sprintf("%ds", v)).Radio(tl.nqRuntime == v).Chosen() {
						tl.nqRuntime = v
					}
				}
			}).Label(s.text("最长时间", "Max Runtime")).Disabled(running)
		})
		s.outboundPicker(c, t, &tl.nqOut, running)
	})
	s.card(c, t, func() {
		ui.Text(c, s.text("操作", "Action")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		if running {
			if ui.Button(c, s.text("取消测试", "Cancel Test")).Clicked() {
				tl.nqStream.stop()
			}
		} else {
			start := s.config.Callbacks.NetworkQuality
			if ui.Button(c, s.text("开始测试", "Start Test")).Disabled(!s.service.Running || start == nil).Clicked() {
				req := NetworkQualityRequest{ConfigURL: tl.nqURL, Outbound: tl.nqOut, Serial: tl.nqSerial, HTTP3: tl.nqHTTP3, MaxRuntimeSeconds: tl.nqRuntime}
				tl.nq, s.errorText = &NetworkQualityProgress{Phase: nqPhaseIdle}, ""
				tl.nqStream.start(s.invalidator(c), func(ctx context.Context, emit func(NetworkQualityProgress)) error {
					return start(ctx, req, emit)
				})
			}
		}
		s.toolsNotice(c, t)
	})
	p := tl.nq
	if p == nil {
		return
	}
	done := p.Phase == nqPhaseDone
	// Downloads and uploads run together unless the test is serial.
	parallel := running && !tl.nqSerial && p.Phase >= nqPhaseDownload && p.Phase < nqPhaseDone
	downloadActive := parallel || p.Phase == nqPhaseDownload
	uploadActive := parallel || p.Phase == nqPhaseUpload
	s.card(c, t, func() {
		ui.Text(c, s.text("结果", "Results")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		s.resultRow(c, t, s.text("空闲延迟", "Idle Latency"), positive(p.IdleLatencyMS, func(v int) string { return fmt.Sprintf("%d ms", v) }), running && p.Phase == nqPhaseIdle, -1, false)
		s.resultRow(c, t, s.text("下载", "Download"), bitrate(p.Download), running && downloadActive, p.DownloadAccuracy, done)
		s.resultRow(c, t, s.text("下载 RPM", "Download RPM"), positive(p.DownloadRPM, strconv.Itoa), running && downloadActive, p.DownloadRPMAccuracy, done)
		s.resultRow(c, t, s.text("上传", "Upload"), bitrate(p.Upload), running && uploadActive, p.UploadAccuracy, done)
		s.resultRow(c, t, s.text("上传 RPM", "Upload RPM"), positive(p.UploadRPM, strconv.Itoa), running && uploadActive, p.UploadRPMAccuracy, done)
	})
}

func (s *Shell) stunPage(c *ui.Context, t *ui.Theme) {
	tl := &s.tools
	running := tl.stunRun.started
	s.toolHeader(c, t, s.text("STUN 测试", "STUN Test"))
	s.card(c, t, func() {
		ui.Text(c, s.text("配置", "Configuration")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		formRow(c, t, s.text("服务器", "Server"), func() {
			ui.TextInput(c, &tl.stunHost).Label(s.text("服务器", "Server")).Grow(1).Disabled(running)
		})
		s.outboundPicker(c, t, &tl.stunOut, running)
	})
	s.card(c, t, func() {
		ui.Text(c, s.text("操作", "Action")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		if running {
			if ui.Button(c, s.text("取消测试", "Cancel Test")).Clicked() {
				tl.stunRun.stop()
			}
		} else {
			start := s.config.Callbacks.STUNTest
			if ui.Button(c, s.text("开始测试", "Start Test")).Disabled(!s.service.Running || start == nil).Clicked() {
				req := STUNRequest{Server: tl.stunHost, Outbound: tl.stunOut}
				tl.stun, s.errorText = &STUNProgress{Phase: stunPhaseBinding}, ""
				tl.stunRun.start(s.invalidator(c), func(ctx context.Context, emit func(STUNProgress)) error {
					return start(ctx, req, emit)
				})
			}
		}
		s.toolsNotice(c, t)
	})
	p := tl.stun
	if p == nil {
		return
	}
	s.card(c, t, func() {
		ui.Text(c, s.text("结果", "Results")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		binding := running && p.Phase == stunPhaseBinding
		s.resultRow(c, t, s.text("外部地址", "External Address"), p.ExternalAddr, binding, -1, false)
		s.resultRow(c, t, s.text("延迟", "Latency"), positive(p.LatencyMS, func(v int) string { return fmt.Sprintf("%d ms", v) }), binding, -1, false)
		if p.Phase == stunPhaseDone && !p.NATTypeSupported {
			s.resultRow(c, t, s.text("NAT 类型检测", "NAT Type Detection"), s.text("服务器不支持", "Not supported by server"), false, -1, false)
			return
		}
		s.natRow(c, t, s.text("NAT 映射", "NAT Mapping"), p.NATMapping, running && p.Phase == stunPhaseMapping)
		s.natRow(c, t, s.text("NAT 过滤", "NAT Filtering"), p.NATFiltering, running && p.Phase == stunPhaseFiltering)
	})
}

// resultRow is Apple's resultValue: the value, "…" while its phase runs,
// and the confidence of a finished measurement.
func (s *Shell) resultRow(c *ui.Context, t *ui.Theme, label, val string, active bool, accuracy int, done bool) {
	formRow(c, t, label, func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			switch {
			case val != "" && active:
				ui.Text(c, val+" …")
			case val != "":
				ui.Text(c, val)
			case active:
				ui.Text(c, "…").TextColor(t.TextMuted)
			default:
				ui.Text(c, "-").TextColor(t.TextMuted)
			}
			if val != "" && done && accuracy >= 0 {
				text, color := s.accuracy(accuracy)
				ui.Text(c, text).FontSize(11).TextColor(color)
			}
		})
	})
}

func (s *Shell) accuracy(v int) (string, ui.Color) {
	switch v {
	case 2:
		return s.text("可信度高", "Confidence High"), ui.Hex("#34C759")
	case 1:
		return s.text("可信度中", "Confidence Medium"), ui.Hex("#D4A000")
	default:
		return s.text("可信度低", "Confidence Low"), ui.Hex("#FF3B30")
	}
}

func (s *Shell) natRow(c *ui.Context, t *ui.Theme, label string, v int, active bool) {
	names := [][2]string{{"未知", "Unknown"}, {"端点无关", "Endpoint Independent"}, {"地址相关", "Address Dependent"}, {"地址和端口相关", "Address and Port Dependent"}}
	colors := []ui.Color{t.Text, ui.Hex("#34C759"), ui.Hex("#D4A000"), ui.Hex("#FF3B30")}
	formRow(c, t, label, func() {
		switch {
		case v > 0 && v < len(names):
			ui.Text(c, s.text(names[v][0], names[v][1])).TextColor(colors[v])
		case active:
			ui.Text(c, "…").TextColor(t.TextMuted)
		default:
			ui.Text(c, "-").TextColor(t.TextMuted)
		}
	})
}

func positive(v int, format func(int) string) string {
	if v <= 0 {
		return ""
	}
	return format(v)
}

// bitrate is the locked core's networkquality.FormatBitrate.
func bitrate(bps int64) string {
	switch {
	case bps <= 0:
		return ""
	case bps >= 1_000_000_000:
		return fmt.Sprintf("%.1f Gbps", float64(bps)/1_000_000_000)
	case bps >= 1_000_000:
		return fmt.Sprintf("%.1f Mbps", float64(bps)/1_000_000)
	case bps >= 1_000:
		return fmt.Sprintf("%.1f Kbps", float64(bps)/1_000)
	default:
		return fmt.Sprintf("%d bps", bps)
	}
}

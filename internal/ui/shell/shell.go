// Package shell is the native MyGo UI. It follows the official Apple client's
// layout; service and network operations require explicit user actions.
package shell

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

type Callbacks struct {
	List          func(context.Context) ([]Profile, error)
	Create        func(ctx context.Context, name, content string) error
	ReadContent   func(ctx context.Context, id string) (string, error)
	UpdateContent func(ctx context.Context, id, content string) error
	Select        func(ctx context.Context, id string) error
	Delete        func(ctx context.Context, id string) error
	// Remote profiles and import/export follow the Apple client.
	CreateRemote  func(ctx context.Context, name, url string, autoUpdate bool, intervalMinutes int) error
	UpdateRemote  func(ctx context.Context, id string) error
	SaveProfile   func(ctx context.Context, id string, settings ProfileSettings) error
	EncodeProfile func(ctx context.Context, id string) ([]byte, error)
	ProfileName   func(ctx context.Context, data []byte) (string, error)
	ImportProfile func(ctx context.Context, data []byte) error
	// OpenFile asks for a profile file (.json or .bpf) and reads it; an empty
	// name is a canceled dialog. SaveFile asks where to save name and writes
	// data. Both are nil without a desktop host.
	OpenFile func(ctx context.Context) (name string, data []byte, err error)
	SaveFile func(ctx context.Context, name string, data []byte) error
	// Settings follow the Apple client's settings pages. OpenURL and
	// OpenPath use the desktop's default applications.
	SaveSettings    func(context.Context, AppSettings) error
	StartAtLogin    func(context.Context) (bool, error)
	SetStartAtLogin func(context.Context, bool) error
	CoreInfo        func(context.Context) (CoreInfo, error)
	OpenURL         func(url string) error
	OpenPath        func(path string) error
	Status          func(context.Context) (ServiceState, error)
	Start           func(context.Context) error
	Stop            func(context.Context) error
	WatchMetrics    func(context.Context, func(Metrics)) error
	SetHTTPProxy    func(context.Context, bool) error
	SetMode         func(context.Context, string) error
	// Groups follow the Apple client: a live subscription plus explicit
	// selection, URL test (group or node tag) and expansion requests.
	WatchGroups    func(context.Context, func([]Group)) error
	SelectOutbound func(ctx context.Context, group, outbound string) error
	URLTest        func(ctx context.Context, tag string) error
	SetGroupExpand func(ctx context.Context, group string, expand bool) error
	// Logs follow the Apple client's log view.
	WatchLogs func(context.Context, func(LogFrame)) error
	ClearLogs func(context.Context) error
	// Connections follow the Apple client's connection list.
	WatchConnections    func(context.Context, func(ConnectionEvents)) error
	CloseConnection     func(ctx context.Context, id string) error
	CloseAllConnections func(context.Context) error
	// Tools follow the Apple client's Network tools; they run in the
	// started core.
	Outbounds      func(context.Context) ([]string, error)
	NetworkQuality func(context.Context, NetworkQualityRequest, func(NetworkQualityProgress)) error
	STUNTest       func(context.Context, STUNRequest, func(STUNProgress)) error
	// CheckUpdate reads this application's latest release; InstallUpdate
	// downloads it and hands it to the core, reporting download progress.
	CheckUpdate   func(context.Context) (UpdateInfo, error)
	InstallUpdate func(context.Context, UpdateInfo, func(float64)) error
}
type ServiceState struct {
	Available, Running, Owned bool
	Version                   string
	// Message is a state code (see serviceMessages) or text to show.
	Message     string
	Metrics     *Metrics
	ClashMode   string
	HTTPProxy   *bool
	ModeOptions []string
}
type Metrics struct {
	Memory, Goroutines, Inbound, Outbound, Upload, Download, UploadTotal, DownloadTotal string
	RateAvailable                                                                       bool
	UploadBytes, DownloadBytes                                                          float64
}
type Node struct {
	Name, Type string
	DelayMS    int // 0: not tested
	Selected   bool
}
type Group struct {
	Name, Type           string
	Selectable, Expanded bool
	Nodes                []Node
}

// LogLine levels are the upstream ones: 2 Error, 3 Warn, 4 Info, 5 Debug, 6 Trace.
type LogLine struct {
	Level   int
	Message string
}
type LogFrame struct {
	Reset        bool // replace the buffer (backlog replay or a clear)
	DefaultLevel int
	Lines        []LogLine
}
type Config struct {
	Callbacks  Callbacks
	Settings   *AppSettings // saved settings; nil keeps the defaults
	Language   string       // "en" selects English; default: simplified Chinese.
	Invalidate func()       // Optional: Window.Invalidate; Context.Invalidate is used otherwise.
	// SetAppearance gives the host's windows, title bars included, the
	// chosen appearance: "system", "light" or "dark". Optional.
	SetAppearance func(string)
	// SettingsChanged tells the host about saved settings it applies itself,
	// as the tray. Optional; called on the UI goroutine.
	SettingsChanged func(AppSettings)
	// AppName is shown at the top of the sidebar, Logo beside it (PNG).
	AppName string
	Logo    []byte
	// Quit ends the application, as after handing an update to the core.
	Quit func()
}
type result struct {
	profiles    []Profile
	content     string
	loadContent bool
	opened      *openedFile // Import from File
	chosen      *openedFile // New Profile's file chooser
	coreInfo    *CoreInfo
	loginItem   *bool
	outbounds   *[]string
	err         error
	service     *ServiceState
	ack         bool // an action whose effect arrives through a stream
	startup     bool // first load: profiles plus a read-only service check
}
type Shell struct {
	config                         Config
	page                           string
	profiles                       []Profile
	loaded, busy                   bool
	results                        chan result
	errorText                      string
	dialog                         bool
	form, pendingForm              profileForm
	settingsPage                   string // "", "app" or "core"
	coreInfo                       *CoreInfo
	loginItem, loginItemKnown      bool
	profilesChanged                atomic.Bool
	transparency                   bool
	appearance                     string
	sidebar                        bool
	service                        ServiceState
	groups                         []Group
	groupsReceived                 bool
	metrics                        *stream[Metrics]
	groupStream                    *stream[[]Group]
	logStream                      *stream[LogFrame]
	logs                           logView
	connStream                     *stream[ConnectionEvents]
	conns                          connectionView
	uploadHistory, downloadHistory []float32
	tools                          toolsState
	cards                          cardConfig
	cardsOpen                      bool
	moreOpen                       bool
	showTray, trayBackground       bool
	serviceChanged                 atomic.Bool
	logo                           ui.ImageSource
	// serviceRetries counts the re-reads of a running core whose mode or
	// system proxy status was not ready yet; see followService.
	serviceRetries int
	retryPending   bool
	update         updateState
	autoUpdate     atomic.Bool
}

func New(config Config) *Shell {
	s := &Shell{config: config, page: "dashboard", results: make(chan result, 1), metrics: newStream[Metrics](), groupStream: newStream[[]Group](), logStream: newStream[LogFrame](), connStream: newStream[ConnectionEvents](), conns: newConnectionView(), transparency: true, appearance: "system", sidebar: true, tools: newToolsState(), showTray: true}
	if saved := config.Settings; saved != nil {
		s.config.Language, s.appearance, s.transparency = saved.Language, saved.Appearance, saved.Transparency
		s.showTray, s.trayBackground = saved.ShowTray, saved.TrayBackground
		s.autoUpdate.Store(saved.AutoUpdate)
		s.cards = newCardConfig(saved.Cards, saved.CardOrder)
	} else {
		s.cards = newCardConfig(nil, nil)
		s.autoUpdate.Store(true)
	}
	if s.config.AppName == "" {
		s.config.AppName = "sing-box"
	}
	if len(config.Logo) > 0 {
		if img, err := ui.DecodeImage(config.Logo); err == nil {
			s.logo = img
		}
	}
	return s
}
func (s *Shell) View() *ui.Content { return ui.View(s.Render) }
func (s *Shell) text(zh, en string) string {
	if s.config.Language == "en" {
		return en
	}
	return zh
}

func (s *Shell) Render(c *ui.Context) {
	select {
	case r := <-s.results:
		s.apply(c, r)
	default:
	}
	if !s.loaded {
		s.loaded = true
		s.startup(c)
	} else if !s.busy && s.profilesChanged.Swap(false) {
		s.reload(c)
	} else if !s.busy && s.serviceChanged.Swap(false) {
		s.retryPending = false
		s.refreshService(c)
	}
	s.updateStreams(c)
	t := s.theme(c)
	// Set the content language explicitly for text shaping and accessibility.
	ui.Themed(c, t).Font(t.Font).Lang(s.contentLanguage()).Grow(1).Children(func() {
		// The window's material (Mica) shows only under a translucent sidebar.
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if s.sidebar {
				s.navigation(c, t)
			}
			ui.Column(c).Grow(1).Background(t.Background).Children(func() {
				s.toolbar(c, t)
				// Logs and connections are virtual lists that scroll themselves.
				if s.page == "logs" || s.page == "connections" {
					ui.Column(c).Grow(1).Padding(16, 24, 16, 24).Gap(12).Children(func() {
						s.pageError(c, t)
						if s.page == "logs" {
							s.logPage(c, t)
						} else {
							s.connectionsPage(c, t)
						}
					})
					return
				}
				ui.Scroll(c).Grow(1).Padding(24).Gap(20).Children(func() {
					s.pageError(c, t)
					switch s.page {
					case "dashboard":
						s.dashboard(c, t)
					case "profiles":
						s.profileSection(c, t)
					case "groups":
						s.groupPage(c, t)
					case "settings":
						s.settings(c, t)
					case "tools":
						s.toolsPage(c, t)
					default:
						s.empty(c, t, s.text("暂无数据", "No data available"))
					}
				})
			})
		})
		s.editor(c, t)
		s.cardManagement(c, t)
	})
}

// pageError shows the last error; an open dialog shows it itself.
func (s *Shell) pageError(c *ui.Context, t *ui.Theme) {
	if s.errorText != "" && !s.dialog {
		ui.Text(c, s.errorText).TextColor(t.Danger).FontSize(12)
	}
}

func (s *Shell) toolbar(c *ui.Context, t *ui.Theme) {
	bg := ui.Hex("#F8F8F8")
	if t.Dark {
		bg = ui.Hex("#343638")
	}
	// The toolbar is the window's title bar, as Apple's unified toolbar: it
	// moves the window and leaves room for the native window controls.
	bar := c.TitleBar()
	ui.Row(c).Height(62).Padding(12, 22+bar.Right, 12, 22).Gap(12).AlignItems(ui.Center).Background(bg).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).DragWindow().Children(func() {
		if iconButton(c, icons["sidebar"], s.text("切换侧边栏", "Toggle sidebar")).Clicked() {
			s.sidebar = !s.sidebar
		}
		label := s.text("启动", "Start")
		can := s.service.Available && len(s.profiles) > 0 && s.config.Callbacks.Start != nil
		ico := icons["play"]
		if s.service.Running {
			label = s.text("停止", "Stop")
			can = s.service.Owned && s.config.Callbacks.Stop != nil
			ico = icons["stop"]
		}
		if iconButton(c, ico, label).Disabled(s.busy || !can).Clicked() {
			s.serviceAction(c)
		}
		ui.Text(c, s.pageTitle()).FontSize(22).FontWeight(600)
		ui.Column(c).Grow(1)
		if iconButton(c, icons["refresh"], s.text("检测服务", "Check service")).Disabled(s.busy).Clicked() {
			s.refreshService(c)
		}
		more := iconButton(c, icons["more"], s.text("其他", "More"))
		if more.Clicked() {
			s.moreOpen = !s.moreOpen
		}
		s.moreMenu(c, t, more)
	})
}

// moreMenu is the toolbar's More menu. It hangs from the button's right
// edge, as Windows 11's More menus do, and not over the window buttons.
func (s *Shell) moreMenu(c *ui.Context, t *ui.Theme, anchor ui.Element) {
	ui.PopoverBase(c, anchor, &s.moreOpen, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorBottomEnd, ui.AnchorTopEnd).Margin(t.Space(1), 0, 0, 0).
			MinWidth(t.Space(45)).Padding(t.Space(1)).Radius(t.Radius+2).Background(t.Background).Border(1, t.Border).
			Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.18))
		item := func(label string, checked bool) bool {
			row := hoverFill(button(c.Key("more:"+label), "").Label(label).Height(32).Padding(0, 12, 0, 8).Gap(6).Justify(ui.Start).Radius(t.Radius).Border(0, ui.Transparent), ui.Transparent)
			row.Children(func() {
				mark := ""
				if checked {
					mark = "✓"
				}
				ui.Text(c, mark).Width(16)
				ui.Text(c, label)
			})
			if row.Clicked() {
				s.moreOpen = false
				return true
			}
			return false
		}
		if item(s.text("概览", "Overview"), s.page == "dashboard") {
			s.page = "dashboard"
		}
		if item(s.text("设置", "Settings"), s.page == "settings") {
			s.page, s.settingsPage = "settings", ""
		}
		if item(s.text("管理配置", "Manage profiles"), s.page == "profiles") {
			s.page = "profiles"
		}
		if item(s.text("仪表盘项目", "Dashboard Items"), false) {
			s.cardsOpen = true
		}
	})
}

func (s *Shell) navigation(c *ui.Context, t *ui.Theme) {
	bg := ui.Hex("#E5E5E8")
	if t.Dark {
		bg = ui.Hex("#242A34")
	}
	if c.Preferences().HighContrast {
		bg = t.Background
	}
	// Translucent, the sidebar shows the window's Mica, as Windows 11's own
	// apps do, with no edge between it and the content.
	box := ui.Column(c).Width(216).Padding(16, 12).Gap(5)
	if !s.transparency || !c.Vibrancy() || c.Preferences().HighContrast {
		box.Background(bg)
	}
	box.Children(func() {
		ui.Row(c).Height(44).AlignItems(ui.Center).Padding(0, 10).Gap(10).DragWindow().Children(func() {
			if s.logo != nil {
				ui.Image(c, s.logo).Width(26).Height(26)
			}
			ui.Text(c, s.config.AppName).FontWeight(600).FontSize(16)
		})
		ui.Text(c, s.text("仪表盘", "Dashboard")).FontSize(12).FontWeight(600).TextColor(t.TextMuted).Padding(16, 10, 7, 10)
		for _, item := range []struct{ id, zh, en string }{{"dashboard", "概览", "Overview"}, {"groups", "分组", "Groups"}, {"connections", "连接", "Connections"}, {"logs", "日志", "Logs"}, {"tools", "工具", "Tools"}, {"settings", "设置", "Settings"}} {
			label := s.text(item.zh, item.en)
			color := t.Text
			fill := ui.Transparent
			if s.page == item.id {
				fill = t.Accent
				color = ui.Hex("#FFFFFF")
			}
			if hoverFill(button(c, "").Label(label).Height(42).Padding(9, 12).Gap(12).Justify(ui.Start).Radius(8).Border(0, ui.Transparent), fill).TextColor(color).Children(func() { ui.Icon(c, icons[item.id]).FontSize(21); ui.Text(c, label).FontSize(16).FontWeight(500) }).Clicked() {
				s.page, s.settingsPage, s.tools.page = item.id, "", ""
			}
		}
		ui.Column(c).Grow(1)
		s.updateBadge(c, t)
		ui.Row(c).Gap(7).Padding(10).Children(func() {
			ui.Icon(c, icons["info"]).FontSize(14).TextColor(t.TextMuted)
			ui.Text(c, s.text("本地工作空间", "Local workspace")).FontSize(12).TextColor(t.TextMuted)
		})
	})
}
func (s *Shell) pageTitle() string {
	switch s.page {
	case "groups":
		return s.text("分组", "Groups")
	case "connections":
		return s.text("连接", "Connections")
	case "profiles":
		return s.text("配置", "Profiles")
	case "tools":
		return s.text("工具", "Tools")
	case "logs":
		return s.text("日志", "Logs")
	case "settings":
		return s.text("设置", "Settings")
	default:
		return s.text("仪表盘", "Dashboard")
	}
}
func (s *Shell) card(c *ui.Context, t *ui.Theme, build func()) {
	ui.Column(c).Padding(18).Gap(14).Radius(18).Background(t.Surface).Children(build)
}
func (s *Shell) empty(c *ui.Context, t *ui.Theme, label string) {
	ui.Column(c).Padding(35).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons[s.page]).FontSize(32).TextColor(t.TextMuted)
		ui.Text(c, label).TextColor(t.TextMuted)
	})
}
func value(v string) string {
	if v == "" {
		return "—"
	}
	return v
}

func (s *Shell) metric(c *ui.Context, t *ui.Theme, title, icon, a, av, b, bv string) {
	ui.Column(c).Grow(1).Height(132).Padding(20).Gap(20).Radius(14).Background(t.Surface).Border(1, t.Border.Alpha(.35)).Children(func() {
		ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(t.TextMuted)
			ui.Text(c, title).FontSize(16).FontWeight(600)
		})
		ui.Column(c).Gap(7).Children(func() {
			for _, r := range [][2]string{{a, av}, {b, bv}} {
				ui.Row(c).Children(func() {
					ui.Text(c, r[0]).FontSize(14).TextColor(t.TextMuted).Grow(1)
					ui.Text(c, value(r[1])).FontSize(14).FontWeight(500)
				})
			}
		})
	})
}
func (s *Shell) refreshService(c *ui.Context) {
	s.run(c, func(ctx context.Context) result {
		if s.config.Callbacks.Status == nil {
			return result{err: errors.New("服务状态检测尚未接入 / Service status backend unavailable")}
		}
		state, err := s.config.Callbacks.Status(ctx)
		return result{service: &state, err: err}
	})
}
func (s *Shell) serviceAction(c *ui.Context) {
	callback := s.config.Callbacks.Start
	if s.service.Running {
		callback = s.config.Callbacks.Stop
	}
	s.run(c, func(ctx context.Context) result {
		if callback == nil {
			return result{err: errUnavailable}
		}
		if err := callback(ctx); err != nil {
			return result{err: err}
		}
		if s.config.Callbacks.Status == nil {
			return result{err: errors.New("无法确认服务操作结果 / Cannot verify service operation result")}
		}
		state, err := s.config.Callbacks.Status(ctx)
		return result{service: &state, err: err}
	})
}

func (s *Shell) appearanceLabel() string {
	switch s.appearance {
	case "light":
		return s.text("浅色", "Light")
	case "dark":
		return s.text("深色", "Dark")
	default:
		return s.text("跟随系统", "System")
	}
}

// apply publishes a finished background operation on the UI goroutine.
func (s *Shell) apply(c *ui.Context, r result) {
	s.busy = false
	if r.service != nil {
		s.setService(*r.service)
	}
	if r.startup {
		s.profiles = r.profiles
	}
	if r.coreInfo != nil {
		s.coreInfo = r.coreInfo
	}
	if r.loginItem != nil {
		s.loginItem, s.loginItemKnown = *r.loginItem, true
	}
	if r.outbounds != nil {
		s.tools.outbounds = *r.outbounds
	}
	if r.err != nil {
		s.errorText = r.err.Error()
		return
	}
	s.errorText = ""
	switch {
	case r.opened != nil:
		s.applyOpened(*r.opened)
	case r.chosen != nil:
		s.form.fileName, s.form.content = filepath.Base(r.chosen.name), string(r.chosen.data)
		if strings.TrimSpace(s.form.name) == "" {
			s.form.name = strings.TrimSuffix(s.form.fileName, filepath.Ext(s.form.fileName))
		}
	case r.loadContent:
		s.form, s.dialog = s.pendingForm, true
		s.form.content = r.content
	case r.ack, r.startup, r.service != nil:
		// Neither overwrites the local profile list.
	default:
		s.profiles = r.profiles
		s.dialog = false
	}
}

// ProfilesChanged reloads the profile list, as after a background update.
// It is safe to call from any goroutine.
func (s *Shell) ProfilesChanged() {
	s.profilesChanged.Store(true)
	if s.config.Invalidate != nil {
		s.config.Invalidate()
	}
}

// followService re-reads a just-started core a few times until its Clash
// mode and system proxy status are ready, as Apple's dashboard subscribes to
// them: the first status read after Start can come before the core has them.
func (s *Shell) followService() {
	if !s.service.Running {
		s.serviceRetries = 0
		return
	}
	if (s.service.HTTPProxy != nil && len(s.service.ModeOptions) > 0) || s.serviceRetries >= 5 || s.retryPending || s.busy {
		return
	}
	s.serviceRetries++
	s.retryPending = true
	time.AfterFunc(2*time.Second, func() {
		s.serviceChanged.Store(true)
		if s.config.Invalidate != nil {
			s.config.Invalidate()
		}
	})
}

// ServiceChanged re-reads the service state, as after a start or stop from
// the tray. It is safe to call from any goroutine.
func (s *Shell) ServiceChanged() {
	s.serviceChanged.Store(true)
	if s.config.Invalidate != nil {
		s.config.Invalidate()
	}
}

func (s *Shell) invalidator(c *ui.Context) func() {
	if s.config.Invalidate != nil {
		return s.config.Invalidate
	}
	return c.Invalidate
}

// operationTimeout bounds one background operation. A file dialog's host
// ignores it, so work after a dialog takes a fresh deadline.
const operationTimeout = 15 * time.Second

// run executes work off the UI goroutine; only one operation runs at a time.
func (s *Shell) run(c *ui.Context, work func(context.Context) result) {
	if s.busy {
		return
	}
	s.busy = true
	invalidate := s.invalidator(c)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		r := work(ctx)
		s.results <- r
		invalidate()
	}()
}
func (s *Shell) list(ctx context.Context) ([]Profile, error) {
	if s.config.Callbacks.List == nil {
		return nil, errUnavailable
	}
	return s.config.Callbacks.List(ctx)
}

// startup loads profiles and, like the Apple client on launch, connects to
// the service read-only so Start is usable without a manual check.
func (s *Shell) startup(c *ui.Context) {
	s.run(c, func(ctx context.Context) result {
		p, err := s.list(ctx)
		r := result{profiles: p, err: err, startup: true}
		if err != nil || s.config.Callbacks.Status == nil {
			return r
		}
		state, err := s.config.Callbacks.Status(ctx)
		r.service, r.err = &state, err
		return r
	})
}
func (s *Shell) reload(c *ui.Context) {
	s.run(c, func(ctx context.Context) result { p, err := s.list(ctx); return result{profiles: p, err: err} })
}
func (s *Shell) selectProfile(c *ui.Context, id string) {
	s.run(c, func(ctx context.Context) result {
		if s.config.Callbacks.Select == nil {
			return result{err: errUnavailable}
		}
		if err := s.config.Callbacks.Select(ctx, id); err != nil {
			return result{err: err}
		}
		p, err := s.list(ctx)
		return result{profiles: p, err: err}
	})
}

func (s *Shell) theme(c *ui.Context) *ui.Theme {
	dark := c.Theme().Dark
	if s.appearance != "system" {
		dark = s.appearance == "dark"
	}
	if c.Preferences().HighContrast {
		t := *c.Theme()
		return &t
	}
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
	}
	// This Windows application uses one installed UI family as its base.
	t.Font, t.FontSize, t.Radius = s.fontFamily(), 14, 8
	if dark {
		t.Background = ui.Hex("#252628")
		t.Surface = ui.Hex("#303133")
		t.Text = ui.RGBA(255, 255, 255, .85)
		t.TextMuted = ui.RGBA(255, 255, 255, .60)
		t.Accent = ui.Hex("#0A84FF")
	} else {
		t.Background = ui.Hex("#F5F5F7")
		t.Surface = ui.Hex("#FFFFFF")
		t.Text = ui.RGBA(0, 0, 0, .85)
		t.TextMuted = ui.RGBA(0, 0, 0, .55)
		t.Accent = ui.Hex("#007AFF")
	}
	t.Danger = ui.Hex("#FF3B30")
	return t
}

func (s *Shell) fontFamily() string {
	if s.config.Language == "en" {
		return "Segoe UI"
	}
	return "Microsoft YaHei UI"
}

func (s *Shell) contentLanguage() string {
	if s.config.Language == "en" {
		return "en"
	}
	return "zh-CN"
}

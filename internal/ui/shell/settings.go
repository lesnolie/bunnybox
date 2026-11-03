package shell

import (
	"context"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"
)

// AppSettings are the saved application settings.
type AppSettings struct {
	Language     string // "zh-Hans" or "en"
	Appearance   string // "system", "light" or "dark"
	Transparency bool
	// ShowTray and TrayBackground are Apple's Show in Menu Bar and Keep Menu
	// Bar in Background: closing the window keeps the app in the tray.
	ShowTray, TrayBackground bool
	// AutoUpdate is Apple's check_update_enabled; on by default here.
	AutoUpdate bool
	// Cards and CardOrder are Apple's enabled_dashboard_cards and
	// dashboard_card_order; empty lists are the defaults.
	Cards, CardOrder []string
}

// CoreInfo is Apple's Core settings page.
type CoreInfo struct {
	Version, WorkingDirectory string
	DataSize                  int64
	DataSizeKnown             bool
	// Readable is false when the service's directory is closed to the user,
	// as the official installation secures it.
	Readable bool
}

const (
	docsURL   = "https://sing-box.sagernet.org/"
	sourceURL = "https://github.com/SagerNet/sing-box"
)

func (s *Shell) settingsState() AppSettings {
	language := s.config.Language
	if language != "en" {
		language = "zh-Hans"
	}
	return AppSettings{Language: language, Appearance: s.appearance, Transparency: s.transparency,
		ShowTray: s.showTray, TrayBackground: s.showTray && s.trayBackground, AutoUpdate: s.autoUpdate.Load(),
		Cards: slices.Clone(s.cards.enabled), CardOrder: slices.Clone(s.cards.order)}
}

// changeSettings applies a setting at once and saves all of them.
func (s *Shell) changeSettings(c *ui.Context, change func()) {
	change()
	settings := s.settingsState()
	if notify := s.config.SettingsChanged; notify != nil {
		notify(settings)
	}
	save := s.config.Callbacks.SaveSettings
	if save == nil {
		return
	}
	s.run(c, func(ctx context.Context) result { return result{err: save(ctx, settings), ack: true} })
}

// settings is Apple's SettingView: a list of pages, then About.
func (s *Shell) settings(c *ui.Context, t *ui.Theme) {
	switch s.settingsPage {
	case "app":
		s.appSettings(c, t)
		return
	case "core":
		s.coreSettings(c, t)
		return
	}
	// Packet Tunnel, On Demand Rules and Profile Override configure Apple's
	// Network Extension; the Windows service has no counterpart.
	s.card(c, t, func() {
		s.settingsLink(c, t, "app", "settings", s.text("应用", "App"))
		s.settingsLink(c, t, "core", "info", s.text("核心", "Core"))
	})
	s.card(c, t, func() {
		ui.Text(c, s.text("关于", "About")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		s.externalLink(c, t, "logs", s.text("文档", "Documentation"), docsURL)
		s.externalLink(c, t, "tools", s.text("源代码", "Source Code"), sourceURL)
	})
}

func (s *Shell) settingsLink(c *ui.Context, t *ui.Theme, page, icon, label string) {
	if ui.Button(c, "").Label(label).Height(40).Padding(8, 6).Justify(ui.Start).Background(ui.Transparent).Border(0, ui.Transparent).Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(t.Accent)
			ui.Text(c, label).FontSize(14).Grow(1)
			ui.Text(c, "›").FontSize(18).TextColor(t.TextMuted)
		})
	}).Clicked() {
		s.settingsPage, s.errorText = page, ""
		if page == "core" {
			s.loadCoreInfo(c)
		}
		if page == "app" {
			s.loadLoginItem(c)
		}
	}
}

func (s *Shell) externalLink(c *ui.Context, t *ui.Theme, icon, label, url string) {
	open := s.config.Callbacks.OpenURL
	if ui.Button(c, "").Label(label).Height(40).Padding(8, 6).Justify(ui.Start).Background(ui.Transparent).Border(0, ui.Transparent).Disabled(open == nil).Children(func() {
		ui.Row(c).Grow(1).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons[icon]).FontSize(18).TextColor(t.Accent)
			ui.Text(c, label).FontSize(14).Grow(1)
			ui.Text(c, "↗").FontSize(14).TextColor(t.TextMuted)
		})
	}).Clicked() {
		if err := open(url); err != nil {
			s.errorText = err.Error()
		}
	}
}

// settingsHeader is the back button of a settings page.
func (s *Shell) settingsHeader(c *ui.Context, t *ui.Theme, title string) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, "‹ "+s.text("设置", "Settings")).Label(s.text("返回设置", "Back to Settings")).Background(ui.Transparent).Border(0, ui.Transparent).TextColor(t.Accent).Clicked() {
			s.settingsPage, s.errorText = "", ""
		}
		ui.Text(c, title).FontSize(17).FontWeight(600)
	})
}

// appSettings is Apple's App page; the macOS menu bar settings are the tray
// here, and appearance and transparency are this client's own.
func (s *Shell) appSettings(c *ui.Context, t *ui.Theme) {
	s.settingsHeader(c, t, s.text("应用", "App"))
	s.card(c, t, func() {
		formRow(c, t, s.text("语言", "Language"), func() {
			ui.PopoverMenuButton(c, map[bool]string{true: "English", false: "简体中文"}[s.config.Language == "en"], func(m *ui.Menu) {
				for _, l := range []struct{ id, label string }{{"zh-Hans", "简体中文"}, {"en", "English"}} {
					if m.Item(l.label).Radio(s.settingsState().Language == l.id).Chosen() {
						id := l.id
						s.changeSettings(c, func() { s.config.Language = id })
					}
				}
			}).Label(s.text("语言", "Language"))
		})
		formRow(c, t, s.text("开机启动", "Start At Login"), func() {
			on := s.loginItem
			if ui.Switch(c, &on).Label(s.text("开机启动", "Start At Login")).Disabled(s.busy || !s.loginItemKnown || s.config.Callbacks.SetStartAtLogin == nil).Changed() {
				set := s.config.Callbacks.SetStartAtLogin
				s.run(c, func(ctx context.Context) result {
					if err := set(ctx, on); err != nil {
						return result{err: err, ack: true}
					}
					return result{loginItem: &on, ack: true}
				})
			}
		})
		ui.Text(c, s.text("登录 Windows 时自动打开本应用。若同时开启“显示托盘图标”和“关闭窗口后保留托盘”，登录时不打开主界面。", "Launch the application when you sign in to Windows. With Show in Tray and Keep in Tray also on, the window does not open at sign-in.")).FontSize(11).TextColor(t.TextMuted)
		formRow(c, t, s.text("显示托盘图标", "Show in Tray"), func() {
			on := s.showTray
			if ui.Switch(c, &on).Label(s.text("显示托盘图标", "Show in Tray")).Changed() {
				s.changeSettings(c, func() {
					s.showTray = on
					if !on {
						s.trayBackground = false
					}
				})
			}
		})
		if s.showTray {
			formRow(c, t, s.text("关闭后保留托盘", "Keep in Tray"), func() {
				on := s.trayBackground
				if ui.Switch(c, &on).Label(s.text("关闭后保留托盘", "Keep in Tray")).Changed() {
					s.changeSettings(c, func() { s.trayBackground = on })
				}
			})
			ui.Text(c, s.text("关闭窗口时程序继续在托盘运行，从托盘菜单退出。", "Closing the window keeps the app in the tray; quit from the tray menu.")).FontSize(11).TextColor(t.TextMuted)
		}
	})
	s.card(c, t, func() {
		ui.Text(c, s.text("外观", "Appearance")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		formRow(c, t, s.text("主题", "Theme"), func() {
			ui.PopoverMenuButton(c, s.appearanceLabel(), func(m *ui.Menu) {
				for _, a := range []struct{ id, zh, en string }{{"system", "跟随系统", "System"}, {"light", "浅色", "Light"}, {"dark", "深色", "Dark"}} {
					if m.Item(s.text(a.zh, a.en)).Radio(s.appearance == a.id).Chosen() {
						id := a.id
						s.changeSettings(c, func() {
							s.appearance = id
							if set := s.config.SetAppearance; set != nil {
								set(id)
							}
						})
					}
				}
			}).Label(s.text("主题", "Theme"))
		})
		formRow(c, t, s.text("透明效果", "Transparency"), func() {
			on := s.transparency
			if ui.Switch(c, &on).Label(s.text("透明效果", "Transparency")).Changed() {
				s.changeSettings(c, func() { s.transparency = on })
			}
		})
	})
	s.updateSettings(c, t)
}

// coreSettings is Apple's Core page.
func (s *Shell) coreSettings(c *ui.Context, t *ui.Theme) {
	s.settingsHeader(c, t, s.text("核心", "Core"))
	info := s.coreInfo
	loading := s.text("读取中…", "Loading…")
	s.card(c, t, func() {
		version, size := loading, loading
		if info != nil {
			version, size = info.Version, s.text("不可用", "Unavailable")
			if info.DataSizeKnown {
				size = formatBytes(info.DataSize)
			}
		}
		formRow(c, t, s.text("版本", "Version"), func() { ui.Text(c, version).TextColor(t.TextMuted) })
		formRow(c, t, s.text("数据大小", "Data Size"), func() { ui.Text(c, size).TextColor(t.TextMuted) })
	})
	s.card(c, t, func() {
		ui.Text(c, s.text("工作目录", "Working Directory")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		dir := ""
		if info != nil {
			dir = info.WorkingDirectory
		}
		readable := info != nil && info.Readable
		if dir != "" {
			ui.Text(c, dir).FontSize(12).Font("Consolas").TextColor(t.TextMuted)
		}
		if dir != "" && !readable {
			ui.Text(c, s.text("该目录由系统服务保护，当前用户无法查看。", "The service protects this directory; the current user cannot view it.")).FontSize(11).TextColor(t.TextMuted)
		}
		open := s.config.Callbacks.OpenPath
		if ui.Button(c, s.text("打开", "Open")).Disabled(!readable || open == nil).Clicked() {
			if err := open(dir); err != nil {
				s.errorText = err.Error()
			}
		}
	})
}

func (s *Shell) loadCoreInfo(c *ui.Context) {
	s.coreInfo = nil
	get := s.config.Callbacks.CoreInfo
	s.run(c, func(ctx context.Context) result {
		if get == nil {
			return result{err: errUnavailable, ack: true}
		}
		info, err := get(ctx)
		return result{coreInfo: &info, err: err, ack: true}
	})
}

func (s *Shell) loadLoginItem(c *ui.Context) {
	get := s.config.Callbacks.StartAtLogin
	if get == nil {
		return
	}
	s.run(c, func(ctx context.Context) result {
		on, err := get(ctx)
		if err != nil {
			return result{err: fmt.Errorf("读取开机启动失败 / start at login: %w", err), ack: true}
		}
		return result{loginItem: &on, ack: true}
	})
}

package shell

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// UpdateInfo describes the latest release of this application. Its version
// is the core version it brings; Current is the running core's.
type UpdateInfo struct {
	Current, Latest     string
	Notes, Page         string
	AssetName, AssetURL string
	SHA256              string
	Size                int64
	Available           bool
}

// updateState follows Apple's UpdateManager: a check (automatic once a day
// when enabled, or on demand), then download and install on request. It is
// shared with the automatic check's goroutine.
type updateState struct {
	mu         sync.Mutex
	info       *UpdateInfo
	checking   bool
	installing bool
	progress   float64
	err        string
	checked    time.Time
}

const updateInterval = 24 * time.Hour

func (s *Shell) updateSnapshot() (info *UpdateInfo, checking, installing bool, progress float64, err string, checked time.Time) {
	u := &s.update
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.info, u.checking, u.installing, u.progress, u.err, u.checked
}

// CheckUpdate checks for a new release from any goroutine; the automatic
// check calls it on start and once a day while enabled.
func (s *Shell) CheckUpdate(automatic bool) {
	check := s.config.Callbacks.CheckUpdate
	u := &s.update
	u.mu.Lock()
	if check == nil || u.checking || u.installing || (automatic && !s.autoUpdate.Load()) {
		u.mu.Unlock()
		return
	}
	u.checking, u.err = true, ""
	u.mu.Unlock()
	s.invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		info, err := check(ctx)
		u.mu.Lock()
		u.checking, u.checked = false, time.Now()
		if err != nil {
			// An automatic check stays quiet when offline, as Apple's does.
			if !automatic {
				u.err = err.Error()
			}
		} else {
			u.info = &info
		}
		u.mu.Unlock()
		s.invalidate()
	}()
}

// StartAutoUpdateCheck checks shortly after launch and then once a day.
func (s *Shell) StartAutoUpdateCheck() {
	go func() {
		time.Sleep(10 * time.Second)
		for {
			s.CheckUpdate(true)
			time.Sleep(updateInterval)
		}
	}()
}

func (s *Shell) installUpdate() {
	install := s.config.Callbacks.InstallUpdate
	u := &s.update
	u.mu.Lock()
	if install == nil || u.info == nil || !u.info.Available || u.installing {
		u.mu.Unlock()
		return
	}
	info := *u.info
	u.installing, u.progress, u.err = true, 0, ""
	u.mu.Unlock()
	s.invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		err := install(ctx, info, func(p float64) {
			u.mu.Lock()
			u.progress = p
			u.mu.Unlock()
			s.invalidate()
		})
		u.mu.Lock()
		u.installing = false
		if err != nil {
			u.err = err.Error()
		}
		u.mu.Unlock()
		s.invalidate()
		if err == nil && s.config.Quit != nil {
			// The installer replaces this application; it opens again after.
			s.config.Quit()
		}
	}()
}

func (s *Shell) invalidate() {
	if s.config.Invalidate != nil {
		s.config.Invalidate()
	}
}

// updateBadge is the sidebar notice of an available update.
func (s *Shell) updateBadge(c *ui.Context, t *ui.Theme) {
	info, _, _, _, _, _ := s.updateSnapshot()
	if info == nil || !info.Available {
		return
	}
	label := s.text("有新版本 ", "Update available ") + info.Latest
	if ui.Button(c, "").Label(label).Padding(8, 10).Radius(8).Background(t.Accent.Alpha(.12)).Border(0, ui.Transparent).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons["download"]).FontSize(14).TextColor(t.Accent)
			ui.Text(c, label).FontSize(12).TextColor(t.Accent)
		})
	}).Clicked() {
		s.page, s.settingsPage = "settings", "app"
		s.loadLoginItem(c)
	}
}

// updateSettings is Apple's Updates section of the App page.
func (s *Shell) updateSettings(c *ui.Context, t *ui.Theme) {
	info, checking, installing, progress, errText, checked := s.updateSnapshot()
	s.card(c, t, func() {
		ui.Text(c, s.text("更新", "Updates")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		formRow(c, t, s.text("自动检查更新", "Check Automatically"), func() {
			on := s.autoUpdate.Load()
			if ui.Switch(c, &on).Label(s.text("自动检查更新", "Check Automatically")).Changed() {
				s.changeSettings(c, func() { s.autoUpdate.Store(on) })
				if on {
					s.CheckUpdate(true)
				}
			}
		})
		formRow(c, t, s.text("当前核心版本", "Core Version"), func() {
			current := "—"
			if info != nil && info.Current != "" {
				current = info.Current
			}
			ui.Text(c, current).TextColor(t.TextMuted)
		})
		var status string
		switch {
		case installing && progress < 1:
			status = fmt.Sprintf(s.text("正在下载 %d%%", "Downloading %d%%"), int(progress*100))
		case installing:
			status = s.text("正在交给核心安装…", "Handing over to the core…")
		case checking:
			status = s.text("正在检查…", "Checking…")
		case info != nil && info.Available:
			status = s.text("新版本 ", "New version ") + info.Latest
		case info != nil && !checked.IsZero():
			status = s.text("已是最新版本", "Up to date")
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).MinHeight(34).Children(func() {
			ui.Text(c, status).Grow(1).TextColor(t.TextMuted)
			if info != nil && info.Available {
				if ui.Button(c, s.text("下载并安装", "Download and Install")).Disabled(installing || checking).Clicked() {
					s.installUpdate()
				}
			}
			if ui.Button(c, s.text("检查更新", "Check Now")).Disabled(installing || checking || s.config.Callbacks.CheckUpdate == nil).Clicked() {
				s.CheckUpdate(false)
			}
		})
		if info != nil && info.Available && info.Notes != "" {
			ui.Text(c, info.Notes).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Text(c, s.text("更新随官方 sing-box 核心的新版本发布。安装时核心会短暂重启，程序随后自动重新打开。", "Updates follow new official sing-box core versions. The core restarts briefly while installing, and the application opens again afterwards.")).FontSize(11).TextColor(t.TextMuted)
		if errText != "" {
			ui.Text(c, errText).FontSize(12).TextColor(t.Danger)
		}
	})
}

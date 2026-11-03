package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"singboxwin.local/client/internal/ui/shell"
)

var errUnavailable = errors.New("service status unavailable")

// trayIcon is the Apple client's menu bar extra (StatusBarController) in the
// notification area: a status header with start/stop, the outbound groups
// while connected, the profiles, then Open and Quit. A left click opens the
// window, as Windows tray apps do; a right click shows the menu, built from
// the backend at that moment.
type trayIcon struct {
	mu       sync.Mutex
	tray     *mygo.Tray
	settings shell.AppSettings
	cb       shell.Callbacks
	open     func()
	changed  func(service, profiles bool)
}

func (t *trayIcon) text(zh, en string) string {
	if t.settings.Language == "en" {
		return en
	}
	return zh
}

// apply shows or removes the icon for the settings. Main thread.
func (t *trayIcon) apply(settings shell.AppSettings) {
	t.mu.Lock()
	t.settings = settings
	t.mu.Unlock()
	if !settings.ShowTray {
		if t.tray != nil {
			t.tray.Destroy()
			t.tray = nil
		}
		return
	}
	if t.tray != nil {
		t.tray.SetToolTip(appName)
		return
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{ID: "main", Icon: trayPNG, ToolTip: appName})
	if err != nil {
		return
	}
	t.tray = tray
	tray.OnClick(t.open)
	// The menu is shown after the right click's listeners return, so the
	// one built here is current.
	tray.OnRightClick(func() { tray.SetMenu(t.menu()) })
}

func (t *trayIcon) keepInBackground() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.settings.ShowTray && t.settings.TrayBackground
}

// menu reads the service, groups and profiles with a short deadline.
func (t *trayIcon) menu() *mygo.Menu {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var state shell.ServiceState
	var stateErr error = errUnavailable
	if t.cb.Status != nil {
		state, stateErr = t.cb.Status(ctx)
	}
	var profiles []shell.Profile
	if t.cb.List != nil {
		profiles, _ = t.cb.List(ctx)
	}
	var groups []shell.Group
	if state.Running && t.cb.WatchGroups != nil {
		groups = firstGroups(ctx, t.cb.WatchGroups)
	}

	status := t.text("服务不可用", "Service unavailable")
	switch {
	case stateErr != nil:
	case state.Running:
		status = t.text("已连接", "Connected")
	case state.Available:
		status = t.text("未连接", "Disconnected")
	}
	items := []*mygo.MenuItem{{Label: appName + " — " + status, Disabled: true}}
	if state.Running {
		items = append(items, &mygo.MenuItem{Label: t.text("停止", "Stop"), Disabled: !state.Owned || t.cb.Stop == nil,
			Click: t.action(true, false, t.cb.Stop)})
	} else {
		items = append(items, &mygo.MenuItem{Label: t.text("启动", "Start"), Disabled: !state.Available || len(profiles) == 0 || t.cb.Start == nil,
			Click: t.action(true, false, t.cb.Start)})
	}
	if len(groups) > 0 {
		var sub []*mygo.MenuItem
		for _, g := range groups {
			var nodes []*mygo.MenuItem
			for _, n := range g.Nodes {
				group, node := g.Name, n.Name
				nodes = append(nodes, &mygo.MenuItem{Label: n.Name, Type: mygo.MenuItemRadio, Checked: n.Selected,
					Disabled: !g.Selectable || t.cb.SelectOutbound == nil,
					Click:    t.action(false, false, func(ctx context.Context) error { return t.cb.SelectOutbound(ctx, group, node) })})
			}
			sub = append(sub, &mygo.MenuItem{Label: g.Name, Type: mygo.MenuItemSubmenu, Submenu: nodes})
		}
		items = append(items, &mygo.MenuItem{Label: t.text("分组", "Group"), Type: mygo.MenuItemSubmenu, Submenu: sub})
	}
	var profileItems []*mygo.MenuItem
	for _, p := range profiles {
		id := p.ID
		profileItems = append(profileItems, &mygo.MenuItem{Label: p.Name, Type: mygo.MenuItemRadio, Checked: p.Selected,
			Disabled: t.cb.Select == nil,
			Click:    t.action(true, true, func(ctx context.Context) error { return t.cb.Select(ctx, id) })})
	}
	if len(profileItems) == 0 {
		profileItems = []*mygo.MenuItem{{Label: t.text("暂无配置", "Empty profiles"), Disabled: true}}
	}
	items = append(items,
		&mygo.MenuItem{Label: t.text("配置", "Profile"), Type: mygo.MenuItemSubmenu, Submenu: profileItems},
		mygo.Separator(),
		&mygo.MenuItem{Label: t.text("打开", "Open"), Click: func(*mygo.MenuItem, *mygo.Window) { t.open() }},
		&mygo.MenuItem{Label: t.text("退出", "Quit"), Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	)
	return mygo.NewMenu(items)
}

// action runs a backend call off the main thread, then tells the window.
func (t *trayIcon) action(service, profiles bool, call func(context.Context) error) func(*mygo.MenuItem, *mygo.Window) {
	return func(*mygo.MenuItem, *mygo.Window) {
		if call == nil {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = call(ctx) // the window shows the state the backend reports
			t.changed(service, profiles)
		}()
	}
}

// firstGroups reads one frame of the group subscription.
func firstGroups(ctx context.Context, watch func(context.Context, func([]shell.Group)) error) []shell.Group {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	got := make(chan []shell.Group, 1)
	go func() {
		_ = watch(ctx, func(g []shell.Group) {
			select {
			case got <- g:
				cancel()
			default:
			}
		})
	}()
	select {
	case g := <-got:
		return g
	case <-ctx.Done():
		select {
		case g := <-got:
			return g
		default:
			return nil
		}
	}
}

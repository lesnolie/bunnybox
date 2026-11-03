package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"singboxwin.local/client/internal/application"
	"singboxwin.local/client/internal/diagnostics"
	"singboxwin.local/client/internal/smoke"
	"singboxwin.local/client/internal/ui/shell"
)

// The official sing-box logo (third_party/sing-box-for-apple,
// MacLibrary/AppIcon.icon/Assets/box.png), cropped; see THIRD_PARTY_NOTICES.
var (
	//go:embed assets/icon-64.png
	iconPNG []byte
	//go:embed assets/tray-32.png
	trayPNG []byte
)

const appName = "sing-box for Windows"

func main() {
	diagnose := flag.Bool("diagnose", false, "Read-only official worker/service diagnostics as JSON")
	smokeCore := flag.Bool("smoke-core", false, "Explicitly test an idle core with a direct-only config, then stop it")
	dataDir := flag.String("data-dir", "", "Development profile directory (default: user config/ProxyGlassDev)")
	atLogin := flag.Bool(strings.TrimPrefix(application.LoginFlag, "--"), false, "Launched by Start At Login")
	flag.Parse()
	if *diagnose && *smokeCore {
		log.Fatal("choose either --diagnose or --smoke-core")
	}
	if *smokeCore {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		report := smoke.Run(ctx)
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			log.Fatal(err)
		}
		if report.Error != "" {
			os.Exit(1)
		}
		return
	}
	if *diagnose {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		report := diagnostics.Run(ctx)
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			log.Fatal(err)
		}
		if report.Error != "" {
			os.Exit(1)
		}
		return
	}
	mygo.App.SetName(appName)
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	if *dataDir == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			log.Fatal(err)
		}
		*dataDir = filepath.Join(root, "ProxyGlassDev")
	}
	directory, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	backend, err := application.Open(ctx, directory)
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	var view *shell.Shell
	var window *mygo.Window
	tray := &trayIcon{changed: func(service, profiles bool) {
		if view == nil {
			return
		}
		if service {
			view.ServiceChanged()
		}
		if profiles {
			view.ProfilesChanged()
		}
	}}
	tray.open = func() {
		if window != nil {
			window.Show()
			window.Focus()
		}
	}
	quit := newQuitLog(filepath.Join(directory, "quit.log"))
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { quit.mark("windows closed") })
	// Quit listeners fire in registration order: cancel the view's metrics
	// subscription before the backend closes its transports.
	mygo.App.OnQuit(func() {
		quit.mark("event loop stopped")
		if view != nil {
			view.Close()
			quit.mark("view closed")
		}
		backend.Close()
		quit.mark("backend closed")
	})
	mygo.App.OnSecondInstance(func([]string, string) { tray.open() })
	mygo.App.WhenReady(func() {
		callbacks := backend.Callbacks()
		callbacks.SaveFile = func(_ context.Context, name string, data []byte) error {
			ext := strings.TrimPrefix(filepath.Ext(name), ".")
			filter := map[string]string{"txt": "Text", "json": "JSON", "bpf": "sing-box Profile"}[ext]
			path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: window, DefaultPath: name, Filters: []mygo.FileFilter{{Name: filter, Extensions: []string{ext}}}})
			if err != nil || path == "" { // "" is a canceled dialog
				return err
			}
			return os.WriteFile(path, data, 0o644)
		}
		callbacks.OpenFile = func(context.Context) (string, []byte, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: window, Title: "导入配置", Filters: []mygo.FileFilter{
				{Name: "sing-box Profile", Extensions: []string{"bpf", "json"}}}})
			if err != nil || len(paths) == 0 {
				return "", nil, err
			}
			info, err := os.Stat(paths[0])
			if err != nil {
				return "", nil, err
			}
			if info.Size() > 64<<20 {
				return "", nil, errors.New("文件过大 / file too large")
			}
			data, err := os.ReadFile(paths[0])
			return paths[0], data, err
		}
		callbacks.OpenURL = mygo.Shell.OpenExternal
		callbacks.OpenPath = mygo.Shell.OpenPath
		loadCtx, cancelLoad := context.WithTimeout(context.Background(), 5*time.Second)
		settings := backend.LoadSettings(loadCtx)
		cancelLoad()
		mygo.Theme.SetSource(mygo.ThemeSource(settings.Appearance))
		view = shell.New(shell.Config{Callbacks: callbacks, Language: settings.Language, Settings: &settings, AppName: appName, Logo: iconPNG,
			SetAppearance: func(a string) {
				mygo.Theme.SetSource(mygo.ThemeSource(a))
			},
			SettingsChanged: tray.apply,
			Quit:            func() { mygo.App.Quit() },
			Invalidate: func() {
				if window != nil {
					window.Invalidate()
				}
			}})
		tray.cb = callbacks
		tray.apply(settings)
		// Apple's Start At Login with the menu bar kept in background leaves
		// the window closed.
		hidden := *atLogin && settings.ShowTray && settings.TrayBackground
		window = mygo.NewWindow(mygo.WindowOptions{Title: appName, Width: 1000, Height: 760, MinWidth: 860, MinHeight: 620, Hidden: hidden, TitleBarStyle: mygo.TitleBarHidden, Vibrancy: mygo.VibrancySidebar, Content: view.View()})
		_ = window.SetIcon(iconPNG)
		roundCorners(window.NativeHandle())
		// Quit (the tray's, an update's) closes the window for good; only the
		// window's own close button hides it to the tray.
		quitting := false
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
		window.OnClose(func(e *mygo.CloseEvent) {
			if !quitting && tray.keepInBackground() {
				e.PreventDefault()
				window.Hide()
				return
			}
			quit.start()
		})
		backend.StartAutoUpdate(view.ProfilesChanged)
		view.StartAutoUpdateCheck()
	})
	err = mygo.App.Run()
	quit.mark("run returned")
	quit.write()
	if err != nil {
		log.Fatal(err)
	}
}

package application

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"singboxwin.local/client/internal/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

// Preference names; the Apple client keeps its settings in the same table.
const (
	prefLanguage     = "language"
	prefAppearance   = "appearance"
	prefTransparency = "transparency"
	prefShowTray     = "show_menu_bar_extra"
	prefTrayInBack   = "menu_bar_extra_in_background"
	prefCards        = "enabled_dashboard_cards"
	prefCardOrder    = "dashboard_card_order"
	prefAutoUpdate   = "check_update_enabled"
)

// LoadSettings reads the application settings, with defaults for missing ones.
func (b *Backend) LoadSettings(ctx context.Context) shell.AppSettings {
	return shell.AppSettings{
		Language:     b.profiles.PreferenceString(ctx, prefLanguage, "zh-Hans"),
		Appearance:   b.profiles.PreferenceString(ctx, prefAppearance, "system"),
		Transparency: b.profiles.PreferenceBool(ctx, prefTransparency, true),
		// Apple's defaults: shown in the menu bar, not kept in background.
		ShowTray:       b.profiles.PreferenceBool(ctx, prefShowTray, true),
		TrayBackground: b.profiles.PreferenceBool(ctx, prefTrayInBack, false),
		Cards:          b.profiles.PreferenceStrings(ctx, prefCards),
		CardOrder:      b.profiles.PreferenceStrings(ctx, prefCardOrder),
		AutoUpdate:     b.profiles.PreferenceBool(ctx, prefAutoUpdate, true),
	}
}

func (b *Backend) saveSettings(ctx context.Context, s shell.AppSettings) error {
	switch {
	case s.Language != "zh-Hans" && s.Language != "en":
		return errors.New("unsupported language")
	case s.Appearance != "system" && s.Appearance != "light" && s.Appearance != "dark":
		return errors.New("unsupported appearance")
	}
	return errors.Join(
		b.profiles.SetPreference(ctx, prefLanguage, s.Language),
		b.profiles.SetPreference(ctx, prefAppearance, s.Appearance),
		b.profiles.SetPreference(ctx, prefTransparency, s.Transparency),
		b.profiles.SetPreference(ctx, prefShowTray, s.ShowTray),
		b.profiles.SetPreference(ctx, prefTrayInBack, s.ShowTray && s.TrayBackground),
		b.profiles.SetPreference(ctx, prefCards, nonNil(s.Cards)),
		b.profiles.SetPreference(ctx, prefCardOrder, nonNil(s.CardOrder)),
		b.profiles.SetPreference(ctx, prefAutoUpdate, s.AutoUpdate))
}

// nonNil stores an empty list as [], Apple's default, rather than null.
func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// coreInfo is Apple's Core settings page: version, data size and the
// service's working directory. A part that cannot be read stays empty.
func (b *Backend) coreInfo(ctx context.Context) (shell.CoreInfo, error) {
	var out shell.CoreInfo
	version, err := daemon.BinaryVersion(ctx, b.executable)
	if err != nil {
		return out, err
	}
	out.Version = version
	info, err := daemon.Inspect(ctx, b.executable)
	if err != nil || info.WorkingDirectory == "" {
		return out, nil
	}
	out.WorkingDirectory = info.WorkingDirectory
	if _, err := os.ReadDir(info.WorkingDirectory); err != nil {
		return out, nil // closed to this user: no size, and nothing to open
	}
	out.Readable = true
	var size int64
	err = filepath.WalkDir(info.WorkingDirectory, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Type().IsRegular() {
			i, err := d.Info()
			if err != nil {
				return err
			}
			size += i.Size()
		}
		return nil
	})
	if err == nil {
		out.DataSize, out.DataSizeKnown = size, true
	}
	return out, nil
}

// LoginFlag marks a launch from Start At Login: with the tray kept in the
// background, the window stays closed, as Apple's menu bar app does.
const LoginFlag = "--login"

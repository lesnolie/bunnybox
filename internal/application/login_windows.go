package application

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Start At Login uses the current user's Run key: no administrator rights,
// and Windows' Startup Apps settings can turn it off.
const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "BunnyBox"
	// oldRunValue is the value before the rename to BunnyBox.
	oldRunValue = "sing-box for Windows"
)

func loginCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return "", err
	}
	return `"` + exe + `" ` + LoginFlag, nil
}

// startAtLogin reports whether the Run key starts this executable.
func startAtLogin() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false, err
	}
	defer k.Close()
	value, _, err := k.GetStringValue(runValue)
	if errors.Is(err, registry.ErrNotExist) {
		value, _, err = k.GetStringValue(oldRunValue)
	}
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	want, err := loginCommand()
	return err == nil && strings.EqualFold(value, want), err
}

func setStartAtLogin(on bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	_ = k.DeleteValue(oldRunValue)
	if !on {
		if err = k.DeleteValue(runValue); errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	command, err := loginCommand()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValue, command)
}

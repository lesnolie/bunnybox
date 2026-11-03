//go:build !windows

package daemon

import (
	"errors"
	"os/exec"
	"path/filepath"
)

func hideWindow(*exec.Cmd)      {}
func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }
func readLayout() (Layout, error) {
	return Layout{}, errors.New("Windows installation registry is unavailable on this platform")
}
func queryService(*Info, string) error {
	return errors.New("Windows Service Control Manager is unavailable on this platform")
}

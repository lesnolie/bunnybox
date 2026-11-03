//go:build !windows

package application

import "errors"

func startAtLogin() (bool, error) { return false, errors.New("start at login requires Windows") }
func setStartAtLogin(bool) error  { return errors.New("start at login requires Windows") }

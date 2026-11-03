//go:build !windows

package application

import "errors"

func relaunchAfter(string) error { return errors.New("updates require Windows") }

//go:build !windows

package profile

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

//go:build !windows

package store

import "os"

func isReparse(os.FileInfo) bool { return false }
func secureDirectory(path string) error {
	if e := rejectLinks(path); e != nil {
		return e
	}
	return os.MkdirAll(path, 0700)
}

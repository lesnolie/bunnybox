//go:build windows

package profile

import "golang.org/x/sys/windows"

func replaceFile(source, target string) error {
	s, e := windows.UTF16PtrFromString(source)
	if e != nil {
		return e
	}
	t, e := windows.UTF16PtrFromString(target)
	if e != nil {
		return e
	}
	return windows.MoveFileEx(s, t, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

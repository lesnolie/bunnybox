package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var dwmSetWindowAttribute = windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")

// roundCorners asks Windows 11 to round the window's corners, which it may
// not do on its own for a window that draws its own title bar. Windows 10
// has no rounded windows and refuses the attribute.
func roundCorners(hwnd uintptr) {
	const dwmwaWindowCornerPreference, dwmwcpRound = 33, 2
	preference := int32(dwmwcpRound)
	dwmSetWindowAttribute.Call(hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&preference)), unsafe.Sizeof(preference))
}

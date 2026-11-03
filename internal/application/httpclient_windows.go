package application

import (
	"net/url"

	"golang.org/x/sys/windows/registry"
)

// systemProxy reads the user's manual proxy setting (Settings > Network >
// Proxy) for each request, so it follows changes, including this
// application's own system proxy switch.
func systemProxy(target *url.URL) (*url.URL, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil, false
	}
	defer key.Close()
	if enabled, _, err := key.GetIntegerValue("ProxyEnable"); err != nil || enabled == 0 {
		return nil, false
	}
	server, _, err := key.GetStringValue("ProxyServer")
	if err != nil {
		return nil, false
	}
	bypass, _, _ := key.GetStringValue("ProxyOverride")
	return manualProxy(target, server, bypass)
}

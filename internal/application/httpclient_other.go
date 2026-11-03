//go:build !windows

package application

import "net/url"

func systemProxy(*url.URL) (*url.URL, bool) { return nil, false }

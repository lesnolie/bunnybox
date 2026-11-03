package application

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// downloadHTTP fetches remote profiles and updates through the system proxy,
// as the Apple client's URLSession does: a subscription the browser reaches
// through a proxy is reachable here too.
var downloadHTTP = &http.Client{Transport: proxyTransport()}

// updateHTTP is downloadHTTP with a timeout for the release API.
var updateHTTP = &http.Client{Transport: downloadHTTP.Transport, Timeout: 30 * time.Second}

func proxyTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		if proxy, ok := systemProxy(request.URL); ok {
			return proxy, nil
		}
		return http.ProxyFromEnvironment(request)
	}
	return transport
}

// manualProxy chooses the proxy of a Windows manual proxy setting:
// server is "host:port" or "http=host:port;https=host:port;socks=host:port",
// and bypass the semicolon separated exceptions, with * wildcards and
// <local> for names without a dot. It reports false for a direct request.
func manualProxy(target *url.URL, server, bypass string) (*url.URL, bool) {
	host := strings.ToLower(target.Hostname())
	for _, rule := range strings.Split(bypass, ";") {
		rule = strings.ToLower(strings.TrimSpace(rule))
		switch {
		case rule == "":
		case rule == "<local>":
			if !strings.Contains(host, ".") {
				return nil, false
			}
		default:
			if ok, _ := path.Match(rule, host); ok {
				return nil, false
			}
		}
	}
	entries := map[string]string{}
	for _, entry := range strings.Split(server, ";") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if scheme, address, found := strings.Cut(entry, "="); found {
			entries[strings.ToLower(scheme)] = address
		} else {
			entries[""] = entry
		}
	}
	scheme := "http"
	address := entries[strings.ToLower(target.Scheme)]
	if address == "" {
		address = entries[""]
	}
	if address == "" && entries["socks"] != "" {
		scheme, address = "socks5", entries["socks"]
	}
	if address == "" {
		return nil, false
	}
	if strings.Contains(address, "://") {
		proxy, err := url.Parse(address)
		return proxy, err == nil
	}
	return &url.URL{Scheme: scheme, Host: address}, true
}

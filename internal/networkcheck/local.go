package networkcheck

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"singboxwin.local/client/internal/rpc/client"
	"singboxwin.local/client/internal/rpc/gen/desktop"
)

// BuildLoopbackConfig allows only an IPv4 loopback high-port mixed listener.
// It never enables system proxy integration. Credentials prevent an unrelated
// process that wins the released-port race from passing the negative auth check.
func BuildLoopbackConfig(port int, username, password string) (string, error) {
	if port < 1024 || port > 65535 {
		return "", errors.New("proxy port must be 1024..65535")
	}
	if username == "" || password == "" {
		return "", errors.New("proxy credentials are required")
	}
	config := map[string]any{
		"log": map[string]any{"level": "warn"},
		"inbounds": []any{map[string]any{"type": "mixed", "tag": "loopback-check", "listen": "127.0.0.1", "listen_port": port,
			"set_system_proxy": false, "users": []any{map[string]string{"username": username, "password": password}}}},
		"outbounds": []any{map[string]string{"type": "direct", "tag": "direct"}},
		"route":     map[string]any{"final": "direct"},
	}
	encoded, err := json.Marshal(config)
	return string(encoded), err
}

func token() (string, error) {
	var bytes [24]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}

// fixture responds only to one unguessable local path and does not expose or
// accept an arbitrary destination URL. Its positive token is never logged.
type fixture struct {
	URL      string
	token    string
	hits     atomic.Int64
	server   *http.Server
	listener net.Listener
}

func newFixture() (*fixture, error) {
	secret, err := token()
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	f := &fixture{token: secret, listener: listener}
	f.URL = "http://" + listener.Addr().String() + "/" + secret
	f.server = &http.Server{ReadHeaderTimeout: 2 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/"+secret || r.Host != listener.Addr().String() {
			http.NotFound(w, r)
			return
		}
		f.hits.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, secret)
	})}
	go f.server.Serve(listener)
	return f, nil
}

func (f *fixture) close() { _ = f.server.Close(); _ = f.listener.Close() }

func runLocal(ctx context.Context, rpc *client.Client, conn grpc.ClientConnInterface, report *Report) {
	app, err := rpc.Application(ctx)
	if !report.record("application_transport", err) {
		return
	}
	f, err := newFixture()
	if !report.record("loopback_fixture", err) {
		return
	}
	defer f.close()
	// Reserve first, close only just before StartService. This reduces but
	// cannot eliminate TOCTOU because upstream accepts a port, not a socket.
	reservation, err := net.Listen("tcp4", "127.0.0.1:0")
	if !report.record("reserve_proxy_port", err) {
		return
	}
	defer reservation.Close()
	port := reservation.Addr().(*net.TCPAddr).Port
	username, err := token()
	if !report.record("proxy_username", err) {
		return
	}
	password, err := token()
	if !report.record("proxy_password", err) {
		return
	}
	config, err := BuildLoopbackConfig(port, username, password)
	if !report.record("build_loopback_config", err) {
		return
	}
	_, err = desktop.NewApplicationServiceClient(app).CheckConfig(ctx, &desktop.ConfigContent{Content: config})
	if !report.record("check_loopback_config", err) {
		return
	}
	report.ProxyURL = "http://127.0.0.1:" + strconv.Itoa(port)
	// Hide fixture secret path and proxy credentials in the report.
	report.FixtureURL = "http://" + f.listener.Addr().String()
	if err = reservation.Close(); !report.record("release_proxy_port", err) {
		return
	}
	lifecycle(ctx, conn, report, config, func(ctx context.Context) error { return probeProxy(ctx, report.ProxyURL, username, password, f) })
}

// probeProxy uses a private transport whose only possible dial is the selected
// proxy. It never reads HTTP_PROXY/HTTPS_PROXY/NO_PROXY or follows redirects.
func probeProxy(ctx context.Context, proxyAddress, username, password string, f *fixture) error {
	if err := localHTTPURL(f.URL); err != nil {
		return err
	}
	if err := localHTTPURL(proxyAddress); err != nil {
		return err
	}
	proxy, err := url.Parse(proxyAddress)
	if err != nil {
		return err
	}
	expectedAddress := proxy.Host
	before := f.hits.Load()
	request := func(auth bool) (int, string, error) {
		p := *proxy
		if auth {
			p.User = url.UserPassword(username, password)
		}
		transport := &http.Transport{Proxy: http.ProxyURL(&p), DisableKeepAlives: true, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != expectedAddress {
				return nil, errors.New("refusing dial outside selected loopback proxy")
			}
			return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "tcp4", address)
		}}
		defer transport.CloseIdleConnections()
		c := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
		if err != nil {
			return 0, "", err
		}
		response, err := c.Do(r)
		if err != nil {
			return 0, "", errors.New("local proxy request failed (transport/authentication)")
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 4096))
		return response.StatusCode, string(body), err
	}
	// A listener unrelated to our authenticated mixed inbound must not count
	// as success merely because it forwards HTTP on the same numeric port.
	code, _, err := request(false)
	if err != nil {
		return err
	}
	if code != http.StatusProxyAuthRequired || f.hits.Load() != before {
		return errors.New("proxy did not reject unauthenticated traffic; possible port collision")
	}
	code, body, err := request(true)
	if err != nil {
		return err
	}
	if code != http.StatusOK || body != f.token || f.hits.Load() != before+1 {
		return errors.New("local fixture response or hit count mismatch")
	}
	return nil
}

func localHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return errors.New("only explicit http://127.0.0.1 high-port URLs are allowed")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1024 || port > 65535 || strings.Contains(u.Host, "@") {
		return fmt.Errorf("invalid loopback high port")
	}
	return nil
}

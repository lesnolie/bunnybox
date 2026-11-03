package shell

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func conn(id, dest, domain string, created int64) *Connection {
	return &Connection{ID: id, Network: "tcp", Destination: dest, Domain: domain, Inbound: "mixed-in", InboundType: "mixed",
		Outbound: "direct-a", OutboundType: "direct", Chain: []string{"direct-a", "PROXY"}, CreatedAt: created, IPVersion: 4, Source: "127.0.0.1:5000"}
}

func TestConnectionTablePortsLibbox(t *testing.T) {
	var tab connectionTable
	now := time.Now().UnixMilli()
	tab.apply(ConnectionEvents{Reset: true, Events: []ConnectionEvent{
		{Type: ConnectionNew, ID: "a", Connection: conn("a", "1.1.1.1:443", "one.example", now-3000)},
		{Type: ConnectionNew, ID: "b", Connection: conn("b", "2.2.2.2:80", "", now-2000)},
		{Type: ConnectionNew, ID: "dns", Connection: &Connection{ID: "dns", OutboundType: "dns", CreatedAt: now}},
	}}, now)
	tab.apply(ConnectionEvents{Events: []ConnectionEvent{
		{Type: ConnectionUpdate, ID: "a", UplinkDelta: 100, DownlinkDelta: 900},
		{Type: ConnectionUpdate, ID: "a", UplinkDelta: 10, DownlinkDelta: 20},
		{Type: ConnectionClosed, ID: "b", ClosedAt: now - 1000},
	}}, now)
	a := tab.byID["a"]
	if a.Uplink != 10 || a.Downlink != 20 || a.UplinkTotal != 110 || a.DownlinkTotal != 920 {
		t.Fatalf("update: %+v", a)
	}
	if ids(tab.view(connActive, sortDate, "")) != "a" || ids(tab.view(connClosed, sortDate, "")) != "b" {
		t.Fatal("state filter")
	}
	if got := ids(tab.view(connAll, sortDate, "")); got != "b,a" {
		t.Fatal("newest first and DNS hidden:", got)
	}
	if got := ids(tab.view(connAll, sortTrafficTotal, "")); got != "a,b" {
		t.Fatal("traffic total sort:", got)
	}
	if got := ids(tab.view(connAll, sortDate, "one network:tcp")); got != "a" {
		t.Fatal("search:", got)
	}
	if ids(tab.view(connAll, sortDate, "network:udp")) != "" || ids(tab.view(connAll, sortDate, "bogus:x")) != "" {
		t.Fatal("typed terms")
	}
	tab.apply(ConnectionEvents{}, now+closedConnectionMaxAge+1001)
	if _, ok := tab.byID["b"]; ok {
		t.Fatal("closed connections expire after five minutes")
	}
	if displayDestination(*a) != "one.example:443" || displayDestination(*conn("x", "2.2.2.2:80", "", 0)) != "2.2.2.2:80" {
		t.Fatal("display destination")
	}
	if formatDuration(1500) != "1.50s" && formatDuration(1500) != "1.5s" {
		t.Fatal(formatDuration(1500))
	}
}

func ids(cs []Connection) string {
	out := ""
	for i, c := range cs {
		if i > 0 {
			out += ","
		}
		out += c.ID
	}
	return out
}

func TestConnectionsFollowAppleList(t *testing.T) {
	emits := make(chan func(ConnectionEvents), 1)
	canceled := make(chan struct{})
	var closed []string
	closeAll := 0
	now := time.Now().UnixMilli()
	s := New(Config{Callbacks: Callbacks{
		WatchConnections: func(ctx context.Context, emit func(ConnectionEvents)) error {
			emit(ConnectionEvents{Reset: true, Events: []ConnectionEvent{
				{Type: ConnectionNew, ID: "a", Connection: conn("a", "1.1.1.1:443", "one.example", now-2000)},
				{Type: ConnectionNew, ID: "b", Connection: conn("b", "2.2.2.2:80", "", now-1000)},
			}})
			emits <- emit
			<-ctx.Done()
			close(canceled)
			return ctx.Err()
		},
		CloseConnection:     func(_ context.Context, id string) error { closed = append(closed, id); return nil },
		CloseAllConnections: func(context.Context) error { closeAll++; return nil },
	}})
	s.loaded, s.page, s.service.Running = true, "connections", true
	tt := ui.NewTester(s.Render, 1000, 760)
	var emit func(ConnectionEvents)
	select {
	case emit = <-emits:
	case <-time.After(time.Second):
		t.Fatal("connection subscription did not start")
	}
	tt.Frame()
	if !tt.HasText("TCP one.example:443") || !tt.HasText("TCP 2.2.2.2:80") || !tt.HasText("mixed/mixed-in") {
		t.Fatal("rows", tt.Texts())
	}
	if err := tt.RightClick("TCP one.example:443"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("关闭"); err != nil {
		t.Fatal(err, tt.Menu())
	}
	complete(t, s, tt)
	if len(closed) != 1 || closed[0] != "a" {
		t.Fatal("close connection", closed)
	}
	emit(ConnectionEvents{Events: []ConnectionEvent{{Type: ConnectionClosed, ID: "a", ClosedAt: now}}})
	tt.Frame()
	if tt.HasText("TCP one.example:443") {
		t.Fatal("the active filter must hide closed connections", tt.Texts())
	}
	menu := func(path ...string) {
		t.Helper()
		if err := tt.Click("连接选项"); err != nil {
			t.Fatal(err)
		}
		for _, label := range path {
			if err := tt.Click(label); err != nil {
				t.Fatal(err, tt.Texts())
			}
		}
		tt.Frame()
	}
	menu("状态", "已关闭")
	if !tt.HasText("TCP one.example:443") || !tt.HasText("已关闭") || tt.HasText("TCP 2.2.2.2:80") {
		t.Fatal("closed filter", tt.Texts())
	}
	if err := tt.Click("TCP one.example:443"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("元数据") || !tt.HasText("one.example") || !tt.HasText("PROXY/direct-a") {
		t.Fatal("details", tt.Texts())
	}
	if err := tt.Click("返回连接列表"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	menu("状态", "活动")
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("搜索连接"); err != nil {
		t.Fatal(err)
	}
	tt.Type("nomatch")
	tt.Frame()
	if !tt.HasText("没有匹配的连接") {
		t.Fatal("search empty state", tt.Texts())
	}
	menu("关闭所有连接")
	complete(t, s, tt)
	if closeAll != 1 {
		t.Fatal("close all")
	}
	if err := tt.Click("概览"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("leaving the page did not cancel the subscription")
	}
}

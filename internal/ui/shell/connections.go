package shell

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type ConnectionEventType int

const (
	ConnectionNew ConnectionEventType = iota
	ConnectionUpdate
	ConnectionClosed
)

// Connection mirrors the upstream daemon message. Times are Unix
// milliseconds; ClosedAt is 0 while the connection is active.
type Connection struct {
	ID, Inbound, InboundType             string
	IPVersion                            int
	Network, Source, Destination, Domain string
	Protocol, User, FromOutbound         string
	CreatedAt, ClosedAt                  int64
	Uplink, Downlink                     int64
	UplinkTotal, DownlinkTotal           int64
	Rule, Outbound, OutboundType         string
	Chain                                []string
}
type ConnectionEvent struct {
	Type                       ConnectionEventType
	ID                         string
	Connection                 *Connection
	UplinkDelta, DownlinkDelta int64
	ClosedAt                   int64
}
type ConnectionEvents struct {
	Reset  bool
	Events []ConnectionEvent
}

// Filters and sorts of the Apple client (ConnectionStateFilter, ConnectionSort).
const (
	connAll = iota
	connActive
	connClosed
)
const (
	sortDate = iota
	sortTraffic
	sortTrafficTotal
)

// closedConnectionMaxAge is libbox's eviction age for closed connections.
const closedConnectionMaxAge = int64(5 * time.Minute / time.Millisecond)

// connectionTable ports libbox's Connections: events update a map, closed
// connections expire after five minutes.
type connectionTable struct {
	byID     map[string]*Connection
	received bool
}

func (t *connectionTable) apply(events ConnectionEvents, now int64) {
	t.received = true
	if events.Reset || t.byID == nil {
		t.byID = map[string]*Connection{}
	}
	for _, e := range events.Events {
		switch e.Type {
		case ConnectionNew:
			if e.Connection != nil {
				c := *e.Connection
				t.byID[e.ID] = &c
			}
		case ConnectionUpdate:
			if c, ok := t.byID[e.ID]; ok {
				c.Uplink, c.Downlink = e.UplinkDelta, e.DownlinkDelta
				c.UplinkTotal += e.UplinkDelta
				c.DownlinkTotal += e.DownlinkDelta
			}
		case ConnectionClosed:
			if e.Connection != nil {
				c := *e.Connection
				t.byID[e.ID] = &c
			}
			if c, ok := t.byID[e.ID]; ok {
				c.ClosedAt, c.Uplink, c.Downlink = e.ClosedAt, 0, 0
			}
		}
	}
	for id, c := range t.byID {
		if c.ClosedAt != 0 && now-c.ClosedAt > closedConnectionMaxAge {
			delete(t.byID, id)
		}
	}
}

// view filters by state and search and sorts like libbox and the Apple
// data model, which also hides DNS connections.
func (t *connectionTable) view(state, sort int, search string) []Connection {
	var out []Connection
	for _, c := range t.byID {
		if c.OutboundType == "dns" || state == connActive && c.ClosedAt != 0 || state == connClosed && c.ClosedAt == 0 {
			continue
		}
		if search != "" && !matchConnection(*c, search) {
			continue
		}
		out = append(out, *c)
	}
	key := func(c Connection) int64 {
		switch sort {
		case sortTraffic:
			return c.Uplink + c.Downlink
		case sortTrafficTotal:
			return c.UplinkTotal + c.DownlinkTotal
		}
		return c.CreatedAt
	}
	slices.SortStableFunc(out, func(x, y Connection) int {
		if kx, ky := key(x), key(y); kx != ky {
			if kx < ky {
				return 1
			}
			return -1
		}
		return strings.Compare(y.ID, x.ID)
	})
	return out
}

// matchConnection is Apple's Connection.performSearch: space-separated
// terms that all match; "type:value" terms match one field.
func matchConnection(c Connection, search string) bool {
	for _, item := range strings.Split(search, " ") {
		if parts := strings.Split(item, ":"); len(parts) == 2 {
			value := parts[1]
			var ok bool
			switch parts[0] {
			case "network":
				ok = c.Network == value
			case "inbound":
				ok = strings.Contains(c.Inbound, value)
			case "inbound.type":
				ok = c.InboundType == value
			case "source":
				ok = strings.Contains(c.Source, value)
			case "destination":
				ok = strings.Contains(c.Destination, value)
			}
			if !ok {
				return false
			}
			continue
		}
		if !strings.Contains(c.Destination, item) && !strings.Contains(c.Domain, item) {
			return false
		}
	}
	return true
}

// displayDestination is libbox's Connection.DisplayDestination: an IP
// destination with a sniffed domain shows the domain.
func displayDestination(c Connection) string {
	host, port, err := net.SplitHostPort(c.Destination)
	if err != nil || c.Domain == "" {
		return c.Destination
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return c.Destination
	}
	return c.Domain + ":" + port
}

type connectionView struct {
	table      connectionTable
	state      int
	sort       int
	searching  bool
	query      string
	list       ui.ListState
	detail     *Connection
	detailOpen bool
}

func newConnectionView() connectionView { return connectionView{state: connActive} }

func (s *Shell) connectionsPage(c *ui.Context, t *ui.Theme) {
	v := &s.conns
	if v.detailOpen && v.detail != nil {
		if live, ok := v.table.byID[v.detail.ID]; ok {
			*v.detail = *live
		}
		s.connectionDetails(c, t, *v.detail)
		return
	}
	cb := s.config.Callbacks
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		if v.searching {
			ui.TextInput(c, &v.query).Label(s.text("搜索连接", "Search connections")).Placeholder(s.text("搜索", "Search")).Grow(1)
		} else {
			ui.Column(c).Grow(1)
		}
		if iconButton(c, icons["search"], s.text("搜索", "Search")).Clicked() {
			v.searching = !v.searching
			if !v.searching {
				v.query = ""
			}
		}
		ui.Button(c, "").Label(s.text("连接选项", "Connection options")).Width(32).Height(32).Background(ui.Transparent).Border(0, ui.Transparent).Children(func() { ui.Icon(c, icons["more"]).FontSize(18) }).PopoverMenuButton(func(m *ui.Menu) {
			m.Submenu(s.text("状态", "State"), func(m *ui.Menu) {
				for i, name := range []string{s.text("全部", "All"), s.text("活动", "Active"), s.text("已关闭", "Closed")} {
					if m.Item(name).Radio(v.state == i).Chosen() {
						v.state = i
					}
				}
			})
			m.Submenu(s.text("排序方式", "Sort By"), func(m *ui.Menu) {
				for i, name := range []string{s.text("日期", "Date"), s.text("流量", "Traffic"), s.text("总流量", "Traffic Total")} {
					if m.Item(name).Radio(v.sort == i).Chosen() {
						v.sort = i
					}
				}
			})
			if m.Item(s.text("关闭所有连接", "Close All Connections")).Disabled(s.busy || cb.CloseAllConnections == nil || !s.service.Running).Chosen() {
				s.groupAction(c, cb.CloseAllConnections)
			}
		})
	})
	conns := v.table.view(v.state, v.sort, v.query)
	if len(conns) == 0 {
		var label string
		switch {
		case !s.service.Running && !v.table.received:
			label = s.text("服务未启动", "Service not started")
		case !v.table.received:
			label = s.text("加载中…", "Loading…")
		case v.query != "":
			label = s.text("没有匹配的连接", "No matching connections")
		case v.state == connActive:
			label = s.text("没有活动连接", "No active connections")
		case v.state == connClosed:
			label = s.text("没有已关闭的连接", "No closed connections")
		default:
			label = s.text("没有连接", "No connections")
		}
		ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, label).TextColor(t.TextMuted)
		})
		return
	}
	v.list.Key = func(i int) any { return conns[i].ID }
	ui.List(c, &v.list, len(conns), func(i int) {
		s.connectionRow(c, t, conns[i])
	}).Label(s.text("连接", "Connections")).Grow(1)
}

func (s *Shell) connectionRow(c *ui.Context, t *ui.Theme, conn Connection) {
	cb := s.config.Callbacks
	mono := func(text string) ui.Element { return ui.Text(c, text).Font("Consolas").FontSize(11) }
	ui.Column(c).Padding(0, 0, 10, 0).Children(func() {
		row := ui.Button(c, "").Label(strings.ToUpper(conn.Network)+" "+displayDestination(conn)).Padding(14, 16).Gap(6).Justify(ui.Start).Radius(12).Background(t.Surface).Border(0, ui.Transparent).Children(func() {
			ui.Column(c).Grow(1).Gap(6).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					mono(strings.ToUpper(conn.Network) + " " + displayDestination(conn)).FontWeight(700).SingleLine().Grow(1)
					if conn.ClosedAt == 0 {
						mono(s.text("活动", "Active")).FontWeight(700).TextColor(pick(t, "#248A3D", "#30D158"))
					} else {
						mono(s.text("已关闭", "Closed")).FontWeight(700).TextColor(pick(t, "#D70015", "#FF453A"))
					}
				})
				ui.Row(c).Gap(16).Children(func() {
					var left, middle [2]string
					right := [2]string{conn.InboundType + "/" + conn.Inbound, ""}
					if conn.ClosedAt != 0 {
						left = [2]string{"↑ " + formatBytes(conn.UplinkTotal), "↓ " + formatBytes(conn.DownlinkTotal)}
						middle = [2]string{formatClock(conn.CreatedAt), formatDuration(conn.ClosedAt - conn.CreatedAt)}
						right[1] = strings.Join(reversed(conn.Chain), "/")
					} else {
						left = [2]string{"↑ " + formatBytes(conn.Uplink) + "/s", "↓ " + formatBytes(conn.Downlink) + "/s"}
						middle = [2]string{formatBytes(conn.UplinkTotal), formatBytes(conn.DownlinkTotal)}
						if len(conn.Chain) > 0 {
							right[1] = conn.Chain[0]
						}
					}
					for _, col := range [][2]string{left, middle} {
						ui.Column(c).Gap(2).Children(func() { mono(col[0]); mono(col[1]) })
					}
					ui.Column(c).Grow(1)
					ui.Column(c).Gap(2).AlignItems(ui.End).Children(func() { mono(right[0]); mono(right[1]) })
				})
			})
		}).ContextMenu(func(m *ui.Menu) {
			if conn.ClosedAt == 0 && m.Item(s.text("关闭", "Close")).Disabled(s.busy || cb.CloseConnection == nil).Chosen() {
				id := conn.ID
				s.groupAction(c, func(ctx context.Context) error { return cb.CloseConnection(ctx, id) })
			}
		})
		if row.Clicked() {
			detail := conn
			s.conns.detail, s.conns.detailOpen = &detail, true
		}
	})
}

// connectionDetails follows Apple's ConnectionDetailsView.
func (s *Shell) connectionDetails(c *ui.Context, t *ui.Theme, conn Connection) {
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		if ui.Button(c, s.text("‹ 连接", "‹ Connections")).Label(s.text("返回连接列表", "Back to connections")).Border(0, ui.Transparent).Background(ui.Transparent).Clicked() {
			s.conns.detailOpen, s.conns.detail = false, nil
		}
		ui.Text(c, s.text("连接", "Connection")).FontSize(17).FontWeight(600)
	})
	field := func(name, value string) {
		ui.Row(c).Gap(12).Padding(8, 0).Children(func() {
			ui.Text(c, name).TextColor(t.TextMuted).Grow(1)
			ui.Text(c, value).Selectable()
		})
	}
	ui.Scroll(c).Grow(1).Gap(16).Children(func() {
		s.card(c, t, func() {
			if conn.ClosedAt != 0 {
				field(s.text("状态", "State"), s.text("已关闭", "Closed"))
			} else {
				field(s.text("状态", "State"), s.text("活动", "Active"))
			}
			field(s.text("创建时间", "Created At"), formatDate(conn.CreatedAt))
			if conn.ClosedAt != 0 {
				field(s.text("关闭时间", "Closed At"), formatDate(conn.ClosedAt))
			}
			field(s.text("上传", "Uplink"), formatBytes(conn.UplinkTotal))
			field(s.text("下载", "Downlink"), formatBytes(conn.DownlinkTotal))
		})
		ui.Text(c, s.text("元数据", "Metadata")).FontSize(13).FontWeight(600).TextColor(t.TextMuted)
		s.card(c, t, func() {
			field(s.text("入站", "Inbound"), conn.Inbound)
			field(s.text("入站类型", "Inbound Type"), conn.InboundType)
			field(s.text("IP 版本", "IP Version"), strconv.Itoa(conn.IPVersion))
			field(s.text("网络", "Network"), strings.ToUpper(conn.Network))
			field(s.text("来源", "Source"), conn.Source)
			field(s.text("目标", "Destination"), conn.Destination)
			optional := [][2]string{
				{s.text("域名", "Domain"), conn.Domain}, {s.text("协议", "Protocol"), conn.Protocol},
				{s.text("用户", "User"), conn.User}, {s.text("来自出站", "From Outbound"), conn.FromOutbound},
				{s.text("匹配规则", "Match Rule"), conn.Rule},
			}
			for _, f := range optional {
				if f[1] != "" {
					field(f[0], f[1])
				}
			}
			field(s.text("出站", "Outbound"), conn.Outbound)
			field(s.text("出站类型", "Outbound Type"), conn.OutboundType)
			if len(conn.Chain) > 1 {
				field(s.text("链", "Chain"), strings.Join(reversed(conn.Chain), "/"))
			}
		})
	})
}

func reversed(in []string) []string {
	out := slices.Clone(in)
	slices.Reverse(out)
	return out
}

// formatBytes matches the dashboard's units.
func formatBytes(value int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	n, unit := float64(max(0, value)), 0
	for n >= 1024 && unit < len(units)-1 {
		n /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", int64(n), units[0])
	}
	return fmt.Sprintf("%.1f %s", n, units[unit])
}

// formatDuration is sing-box's log.FormatDuration, which libbox uses.
func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%d.%ds", int64(d.Seconds()), int64(d.Seconds()*100)%100)
	default:
		return fmt.Sprintf("%dm%ds", int64(d.Minutes()), int64(d.Seconds())%60)
	}
}

func formatClock(ms int64) string { return time.UnixMilli(ms).Format("15:04:05") }
func formatDate(ms int64) string  { return time.UnixMilli(ms).Format("2006-01-02 15:04:05") }

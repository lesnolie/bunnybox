package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	"google.golang.org/protobuf/types/known/emptypb"
	d "singboxwin.local/client/internal/rpc/gen/daemon"
	"singboxwin.local/client/internal/ui/shell"
)

// watchGroups streams outbound groups like the Apple client's group list:
// selections, expansion and asynchronous URL test results arrive as frames.
func (r runtimeReader) watchGroups(ctx context.Context, emit func([]Group)) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return fmt.Errorf("groups unavailable: %s", a.Reason)
	}
	stream, err := d.NewStartedServiceClient(r.conn).SubscribeGroups(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}
	for {
		frame, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return errors.New("groups stream ended")
		}
		if err != nil {
			return err
		}
		emit(groupsFromFrame(frame))
	}
}

// requireGroup re-checks ownership, a started core and the group's presence at
// the RPC boundary; it never claims the service or starts a core.
func (r runtimeReader) requireGroup(ctx context.Context, tag string) (Group, error) {
	snapshot, err := r.groups(ctx)
	if err != nil {
		return Group{}, err
	}
	if !snapshot.Available {
		return Group{}, errors.New("core is not owned and running")
	}
	i := slices.IndexFunc(snapshot.Groups, func(g Group) bool { return g.Tag == tag })
	if i < 0 {
		return Group{}, fmt.Errorf("group not found: %s", tag)
	}
	return snapshot.Groups[i], nil
}

func (r runtimeReader) selectOutbound(ctx context.Context, group, outbound string) error {
	g, err := r.requireGroup(ctx, group)
	if err != nil {
		return err
	}
	if !g.Selectable || !slices.ContainsFunc(g.Items, func(item GroupItem) bool { return item.Tag == outbound }) {
		return fmt.Errorf("%s cannot select %s", group, outbound)
	}
	_, err = d.NewStartedServiceClient(r.conn).SelectOutbound(ctx, &d.SelectOutboundRequest{GroupTag: group, OutboundTag: outbound})
	return err
}

// urlTest starts an upstream URL test for a group or a single outbound. The
// upstream call returns immediately; results arrive through watchGroups.
func (r runtimeReader) urlTest(ctx context.Context, tag string) error {
	a, err := r.probe(ctx)
	if err != nil {
		return err
	}
	if !a.Available {
		return errors.New("core is not owned and running")
	}
	_, err = d.NewStartedServiceClient(r.conn).URLTest(ctx, &d.URLTestRequest{OutboundTag: tag})
	return err
}

func (r runtimeReader) setGroupExpand(ctx context.Context, group string, expand bool) error {
	if _, err := r.requireGroup(ctx, group); err != nil {
		return err
	}
	_, err := d.NewStartedServiceClient(r.conn).SetGroupExpand(ctx, &d.SetGroupExpandRequest{GroupTag: group, IsExpand: expand})
	return err
}

func uiGroups(groups []Group) []shell.Group {
	out := make([]shell.Group, 0, len(groups))
	for _, g := range groups {
		group := shell.Group{Name: g.Tag, Type: proxyDisplayName(g.Type), Selectable: g.Selectable, Expanded: g.Expanded}
		for _, n := range g.Items {
			node := shell.Node{Name: n.Tag, Type: proxyDisplayName(n.Type), Selected: g.Selected == n.Tag}
			if n.URLTestTime > 0 && n.DelayMS > 0 {
				node.DelayMS = int(n.DelayMS)
			}
			group.Nodes = append(group.Nodes, node)
		}
		out = append(out, group)
	}
	return out
}

func (b *Backend) withReader(ctx context.Context, fn func(runtimeReader) error) error {
	r, err := b.runtimeReader(ctx)
	if err != nil {
		return err
	}
	return fn(r)
}
func (b *Backend) watchGroups(ctx context.Context, emit func([]shell.Group)) error {
	return b.withReader(ctx, func(r runtimeReader) error {
		return r.watchGroups(ctx, func(g []Group) { emit(uiGroups(g)) })
	})
}
func (b *Backend) selectOutbound(ctx context.Context, group, outbound string) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.selectOutbound(ctx, group, outbound) })
}
func (b *Backend) urlTest(ctx context.Context, tag string) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.urlTest(ctx, tag) })
}
func (b *Backend) setGroupExpand(ctx context.Context, group string, expand bool) error {
	return b.withReader(ctx, func(r runtimeReader) error { return r.setGroupExpand(ctx, group, expand) })
}

// proxyDisplayName mirrors constant.ProxyDisplayName of the locked core
// (v1.14.4), which the Apple client shows via LibboxProxyDisplayType.
func proxyDisplayName(proxyType string) string {
	if name, ok := proxyDisplayNames[proxyType]; ok {
		return name
	}
	return "Unknown"
}

var proxyDisplayNames = map[string]string{
	"tun": "TUN", "redirect": "Redirect", "tproxy": "TProxy", "direct": "Direct",
	"bridge": "Bridge", "block": "Block", "dns": "DNS", "socks": "SOCKS",
	"http": "HTTP", "mixed": "Mixed", "shadowsocks": "Shadowsocks", "snell": "Snell",
	"vmess": "VMess", "trojan": "Trojan", "naive": "Naive", "wireguard": "WireGuard",
	"hysteria": "Hysteria", "tor": "Tor", "ssh": "SSH", "shadowtls": "ShadowTLS",
	"shadowsocksr": "ShadowsocksR", "vless": "VLESS", "tuic": "TUIC", "hysteria2": "Hysteria2",
	"anytls": "AnyTLS", "openconnect": "OpenConnect", "openvpn-client": "OpenVPN Client",
	"openvpn-server": "OpenVPN Server", "tailscale": "Tailscale", "cloudflared": "Cloudflared",
	"selector": "Selector", "urltest": "URLTest",
}

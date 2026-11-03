package shell

import (
	"context"
	"strconv"

	"github.com/egoist/mygo/ui"
)

// Layout of the Apple client's GroupView (macOS values).
const (
	groupDotSize     = 11
	groupDotSpacing  = 4
	groupDotRadius   = 4
	groupSelectedDot = 4
	groupItemMin     = 170
	groupItemSpacing = 10
)

// groupPage follows Apple's GroupListView: one card per group with a header
// (tag, type, count, URL test, expand) and either a node grid or a dot grid.
func (s *Shell) groupPage(c *ui.Context, t *ui.Theme) {
	if !s.service.Running || s.config.Callbacks.WatchGroups == nil {
		s.empty(c, t, s.text("暂无可用分组", "No groups available"))
		return
	}
	if !s.groupsReceived {
		s.empty(c, t, s.text("加载中…", "Loading…"))
		return
	}
	if len(s.groups) == 0 {
		s.empty(c, t, s.text("暂无可用分组", "No groups available"))
		return
	}
	width, _ := c.Size()
	width -= 48 + 32 // page padding, card padding
	if s.sidebar {
		width -= 216
	}
	for i := range s.groups {
		s.groupCard(c, t, i, width)
	}
}

func (s *Shell) groupCard(c *ui.Context, t *ui.Theme, index int, width float32) {
	g := s.groups[index]
	cb := s.config.Callbacks
	ui.Column(c).Key("group:" + g.Name).Radius(16).Background(t.Surface).Children(func() {
		ui.Row(c).Padding(12, 8, 10, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, g.Name).FontSize(15).FontWeight(600)
			ui.Text(c, g.Type).FontSize(13).TextColor(t.TextMuted)
			ui.Column(c).Grow(1)
			ui.Row(c).Padding(2, 8).Radius(10).Background(urlTestNeutral(t)).Children(func() {
				ui.Text(c, strconv.Itoa(len(g.Nodes))).FontSize(11).FontWeight(600).TextColor(t.TextMuted)
			})
			if iconButton(c, icons["bolt"], s.text("测速 ", "URL test ")+g.Name).Disabled(s.busy || cb.URLTest == nil).Clicked() {
				s.groupAction(c, func(ctx context.Context) error { return cb.URLTest(ctx, g.Name) })
			}
			chevron, label := icons["chevronDown"], s.text("展开 ", "Expand ")
			if g.Expanded {
				chevron, label = icons["chevronUp"], s.text("收起 ", "Collapse ")
			}
			if iconButton(c, chevron, label+g.Name).Disabled(s.busy || cb.SetGroupExpand == nil).Clicked() {
				expand := !g.Expanded
				s.groups[index].Expanded = expand // optimistic, as in Apple's toggleExpand
				s.groupAction(c, func(ctx context.Context) error { return cb.SetGroupExpand(ctx, g.Name, expand) })
			}
		})
		ui.Column(c).Padding(2, 16, 14, 16).Children(func() {
			if g.Expanded {
				s.nodeGrid(c, t, g, width)
			} else {
				dotGrid(c, t, g, width)
			}
		})
	})
}

func (s *Shell) nodeGrid(c *ui.Context, t *ui.Theme, g Group, width float32) {
	cb := s.config.Callbacks
	columns := max(1, int((width+groupItemSpacing)/(groupItemMin+groupItemSpacing)))
	ui.Grid(c).Columns(columns).Gap(groupItemSpacing).Children(func() {
		for _, node := range g.Nodes {
			fill, border := t.Background, urlTestNeutral(t)
			if node.Selected {
				fill, border = t.Accent.Alpha(.12), t.Accent
			}
			item := button(c.Key("node:"+node.Name), "").Label(node.Name).Height(54).Padding(10, 12).Justify(ui.Start).Radius(12).Background(fill).Border(1, border).Children(func() {
				ui.Column(c).Grow(1).Gap(4).Children(func() {
					ui.Text(c, node.Name).SingleLine().FontSize(13).FontWeight(600)
					ui.Row(c).Gap(8).Children(func() {
						ui.Text(c, node.Type).FontSize(11).TextColor(t.TextMuted).Grow(1)
						if node.DelayMS > 0 {
							ui.Text(c, strconv.Itoa(node.DelayMS)+"ms").FontSize(11).FontWeight(600).TextColor(urlTestColor(node.DelayMS, t))
						}
					})
				})
			}).ContextMenu(func(m *ui.Menu) {
				if m.Item(s.text("测速", "URL Test")).Disabled(s.busy || cb.URLTest == nil).Chosen() {
					s.groupAction(c, func(ctx context.Context) error { return cb.URLTest(ctx, node.Name) })
				}
			})
			if item.Clicked() && g.Selectable && !node.Selected && cb.SelectOutbound != nil {
				s.groupAction(c, func(ctx context.Context) error { return cb.SelectOutbound(ctx, g.Name, node.Name) })
			}
		}
	})
}

func dotGrid(c *ui.Context, t *ui.Theme, g Group, width float32) {
	columns := max(1, int((width+groupDotSpacing)/(groupDotSize+groupDotSpacing)))
	spacing := float32(0)
	if columns > 1 {
		spacing = float32(int((width - float32(columns)*groupDotSize) / float32(columns-1)))
	}
	rows := (len(g.Nodes) + columns - 1) / columns
	height := float32(rows)*groupDotSize + float32(max(0, rows-1))*groupDotSpacing
	ui.Box(c).Height(height).Draw(func(p *ui.Painter, r ui.Rect) {
		for i, node := range g.Nodes {
			x := r.X + float32(i%columns)*(groupDotSize+spacing)
			y := r.Y + float32(i/columns)*(groupDotSize+groupDotSpacing)
			p.Fill(ui.Rect{X: x, Y: y, W: groupDotSize, H: groupDotSize}, urlTestColor(node.DelayMS, t), groupDotRadius)
			if node.Selected {
				inset := float32(groupDotSize-groupSelectedDot) / 2
				p.Fill(ui.Rect{X: x + inset, Y: y + inset, W: groupSelectedDot, H: groupSelectedDot}, ui.Hex("#FFFFFF"), groupSelectedDot/2)
			}
		}
	})
}

// groupAction runs a group RPC; its effect arrives through the groups stream.
func (s *Shell) groupAction(c *ui.Context, op func(context.Context) error) {
	s.run(c, func(ctx context.Context) result {
		if err := op(ctx); err != nil {
			return result{err: err}
		}
		return result{ack: true}
	})
}

// urlTestColor mirrors Apple's Color.urlTestDelay thresholds and palette.
func urlTestColor(ms int, t *ui.Theme) ui.Color {
	switch {
	case ms <= 0:
		return urlTestNeutral(t)
	case ms < 800:
		return pick(t, "#3D8168", "#7CB89E")
	case ms < 1500:
		return pick(t, "#A8742F", "#D3A45E")
	default:
		return pick(t, "#C25E32", "#DB8A62")
	}
}
func urlTestNeutral(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, .09)
	}
	return ui.RGBA(0, 0, 0, .07)
}
func pick(t *ui.Theme, light, dark string) ui.Color {
	if t.Dark {
		return ui.Hex(dark)
	}
	return ui.Hex(light)
}

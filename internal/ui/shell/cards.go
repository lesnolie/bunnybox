package shell

import (
	"slices"

	"github.com/egoist/mygo/ui"
)

// Dashboard cards follow the Apple client's DashboardCard and
// DashboardCardConfiguration: the same identifiers, defaults, pairing and
// rules (the profile card is always on; cards missing from a saved order are
// appended).
const (
	cardStatus      = "status"
	cardConnections = "connections"
	cardUpload      = "uploadTraffic"
	cardDownload    = "downloadTraffic"
	cardHTTPProxy   = "httpProxy"
	cardClashMode   = "clashMode"
	cardProfile     = "profile"
)

// allCards is Apple's DashboardCard.allCases, the default enabled set.
var allCards = []string{cardStatus, cardConnections, cardUpload, cardDownload, cardHTTPProxy, cardClashMode, cardProfile}

// defaultCardOrder is Apple's DashboardCard.defaultOrder.
var defaultCardOrder = []string{cardUpload, cardDownload, cardStatus, cardConnections, cardHTTPProxy, cardClashMode, cardProfile}

func cardPair(id string) string {
	switch id {
	case cardUpload, cardDownload:
		return "traffic"
	case cardStatus, cardConnections:
		return "statistics"
	}
	return ""
}

// cardRows is Apple's groupIntoRows: two adjacent cards of a pair share a row.
func cardRows(cards []string) [][]string {
	var rows [][]string
	for i := 0; i < len(cards); i++ {
		if p := cardPair(cards[i]); p != "" && i+1 < len(cards) && cardPair(cards[i+1]) == p {
			rows = append(rows, []string{cards[i], cards[i+1]})
			i++
			continue
		}
		rows = append(rows, []string{cards[i]})
	}
	return rows
}

type cardConfig struct {
	enabled, order []string
}

// newCardConfig loads saved cards; empty lists are the defaults.
func newCardConfig(enabled, order []string) cardConfig {
	known := func(list []string) []string {
		var out []string
		for _, id := range list {
			id = migrateCard(id)
			if slices.Contains(allCards, id) && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
		return out
	}
	cfg := cardConfig{enabled: known(enabled), order: known(order)}
	if len(cfg.enabled) == 0 {
		cfg.enabled = slices.Clone(allCards)
	} else if !slices.Contains(cfg.enabled, cardProfile) {
		cfg.enabled = append(cfg.enabled, cardProfile)
	}
	if len(cfg.order) == 0 {
		cfg.order = slices.Clone(defaultCardOrder)
	}
	for _, id := range allCards {
		if !slices.Contains(cfg.order, id) {
			cfg.order = append(cfg.order, id)
		}
	}
	return cfg
}

func migrateCard(id string) string {
	switch id {
	case "traffic":
		return cardUpload
	case "trafficTotal":
		return cardDownload
	}
	return id
}

func (c cardConfig) ordered() []string {
	var out []string
	for _, id := range c.order {
		if slices.Contains(c.enabled, id) {
			out = append(out, id)
		}
	}
	return out
}

func (c *cardConfig) toggle(id string) {
	if id == cardProfile {
		return
	}
	if i := slices.Index(c.enabled, id); i >= 0 {
		c.enabled = slices.Delete(c.enabled, i, i+1)
		return
	}
	c.enabled = append(c.enabled, id)
}

// move shifts a card one place up (-1) or down (+1).
func (c *cardConfig) move(id string, by int) {
	i := slices.Index(c.order, id)
	j := i + by
	if i < 0 || j < 0 || j >= len(c.order) {
		return
	}
	c.order[i], c.order[j] = c.order[j], c.order[i]
}

func (s *Shell) cardTitle(id string) string {
	switch id {
	case cardStatus:
		return s.text("状态", "Status")
	case cardConnections:
		return s.text("连接", "Connections")
	case cardUpload:
		return s.text("上传", "Upload")
	case cardDownload:
		return s.text("下载", "Download")
	case cardHTTPProxy:
		return s.text("系统 HTTP 代理", "System HTTP Proxy")
	case cardClashMode:
		return s.text("Clash 模式", "Clash Mode")
	}
	return s.text("配置", "Profile")
}

var cardIcons = map[string]string{cardStatus: "info", cardConnections: "connections", cardUpload: "upload", cardDownload: "download", cardHTTPProxy: "groups", cardClashMode: "mode", cardProfile: "profiles"}

// cardManagement is Apple's CardManagementSheet: a toggle per card in the
// current order, moved with buttons here instead of dragging, Reset and Done.
// Changes apply and save at once.
func (s *Shell) cardManagement(c *ui.Context, t *ui.Theme) {
	if !s.cardsOpen {
		return
	}
	ui.Modal(c, &s.cardsOpen, func() {
		ui.Column(c).Width(420).Gap(10).Children(func() {
			ui.Text(c, s.text("仪表盘项目", "Dashboard Items")).FontSize(17).Bold()
			for i, id := range s.cards.order {
				title := s.cardTitle(id)
				ui.Row(c).Gap(8).AlignItems(ui.Center).MinHeight(36).Children(func() {
					ui.Icon(c, icons[cardIcons[id]]).FontSize(17).TextColor(t.Accent)
					color := t.Text
					if !slices.Contains(s.cards.enabled, id) {
						color = t.TextMuted
					}
					ui.Text(c, title).Grow(1).TextColor(color)
					if iconButton(c, icons["chevronUp"], s.text("上移 ", "Move up ")+title).Disabled(i == 0).Clicked() {
						s.changeSettings(c, func() { s.cards.move(id, -1) })
					}
					if iconButton(c, icons["chevronDown"], s.text("下移 ", "Move down ")+title).Disabled(i == len(s.cards.order)-1).Clicked() {
						s.changeSettings(c, func() { s.cards.move(id, 1) })
					}
					on := id == cardProfile || slices.Contains(s.cards.enabled, id)
					if ui.Switch(c, &on).Label(title).Disabled(id == cardProfile).Changed() {
						s.changeSettings(c, func() { s.cards.toggle(id) })
					}
				})
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, s.text("重置", "Reset")).TextColor(t.Danger).Clicked() {
					s.changeSettings(c, func() { s.cards = newCardConfig(nil, nil) })
				}
				if ui.Button(c, s.text("完成", "Done")).Clicked() {
					s.cardsOpen = false
				}
			})
		})
	})
}

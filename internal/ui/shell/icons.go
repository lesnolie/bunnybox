package shell

import "github.com/egoist/mygo/ui"

// Original geometric icons, authored for this application. No Apple artwork or
// font glyphs are embedded. Every path shares a 24-DIP optical box.
func svg(path string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
}

var icons = map[string]*ui.SVG{
	"chevrons":    svg(`<path d="m8 8 4-4 4 4m-8 8 4 4 4-4"/>`),
	"tools":       svg(`<rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 4 3-4 3m6 1h4"/>`),
	"edit":        svg(`<path d="m4 16-1 5 5-1L20 8l-4-4zM13 7l4 4"/>`),
	"share":       svg(`<path d="M5 10H3v11h18V10h-2M12 15V2m-5 5 5-5 5 5"/>`),
	"dashboard":   svg(`<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M3 9h18M7 6.5h.1M10 6.5h.1M7 13h4M7 16h4M16 12v5m-2-2h4"/>`),
	"groups":      svg(`<rect x="2" y="4" width="9" height="6" rx="1.5"/><rect x="2" y="14" width="9" height="6" rx="1.5"/><rect x="15" y="4" width="7" height="16" rx="1.5"/>`),
	"connections": svg(`<path d="M9 15l6-6M8 17l-1 1a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0M16 7l1-1a4 4 0 0 1 6 6l-5 5a4 4 0 0 1-6 0" transform="translate(0 0) scale(.95)"/>`),
	"logs":        svg(`<path d="M6 2h8l5 5v15H6zM14 2v6h5M9 12h7M9 16h7M9 19h5"/>`),
	"profiles":    svg(`<rect x="2" y="4" width="20" height="16" rx="2"/><path d="M6 8h.1M6 12h.1M6 16h.1M10 8h8M10 12h8M10 16h5"/>`),
	"settings":    svg(`<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="3"/><path d="M12 3v3m0 12v3M3 12h3m12 0h3M6 6l2 2m8 8 2 2M6 18l2-2M16 8l2-2"/>`),
	"sidebar":     svg(`<rect x="2" y="4" width="20" height="16" rx="2"/><path d="M9 4v16M5 8h1M5 12h1M5 16h1"/>`),
	"play":        svg(`<path d="M7 4l14 8-14 8z" fill="currentColor" stroke="none"/>`),
	"stop":        svg(`<rect x="5" y="5" width="14" height="14" rx="2" fill="currentColor"/>`),
	"refresh":     svg(`<path d="M20 10a8 8 0 1 0-1 8M20 3v7h-7"/>`),
	"more":        svg(`<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`),
	"add":         svg(`<path d="M12 4v16M4 12h16"/>`),
	"circle":      svg(`<circle cx="12" cy="12" r="8"/>`),
	"selected":    svg(`<circle cx="12" cy="12" r="9"/><path d="M8 12l3 3 5-6"/>`),
	"upload":      svg(`<circle cx="12" cy="12" r="9"/><path d="M12 17V7M8 11l4-4 4 4"/>`),
	"download":    svg(`<circle cx="12" cy="12" r="9"/><path d="M12 7v10M8 13l4 4 4-4"/>`),
	"search":      svg(`<circle cx="10.5" cy="10.5" r="6.5"/><path d="m15.5 15.5 5 5"/>`),
	"pause":       svg(`<rect x="6" y="5" width="4" height="14" rx="1" fill="currentColor" stroke="none"/><rect x="14" y="5" width="4" height="14" rx="1" fill="currentColor" stroke="none"/>`),
	"bolt":        svg(`<path d="M13 2 4 14h7l-1 8 9-12h-7z" fill="currentColor" stroke="none"/>`),
	"chevronUp":   svg(`<path d="m6 15 6-6 6 6"/>`),
	"chevronDown": svg(`<path d="m6 9 6 6 6-6"/>`),
	"mode":        svg(`<circle cx="7" cy="7" r="3.2"/><circle cx="17" cy="7" r="3.2"/><circle cx="7" cy="17" r="3.2"/><circle cx="17" cy="17" r="3.2"/>`),
	"info":        svg(`<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.1"/>`),
}

func iconButton(c *ui.Context, icon *ui.SVG, label string) ui.Element {
	return hoverFill(button(c, "").Label(label).Width(32).Height(32).Padding(7).Radius(7).Border(0, ui.Transparent), ui.Transparent).Children(func() { ui.Icon(c, icon).FontSize(18) })
}

// hoverFill gives a button drawn on fill the feedback its own style would:
// a light tint while the pointer is over it and a deeper one while pressed.
// A colored fill, as a selected item's, stays as it is.
func hoverFill(e ui.Element, fill ui.Color) ui.Element {
	pressed := e.Pressed()
	if fill == ui.Transparent {
		switch {
		case pressed:
			fill = ui.Hex("#808080").Alpha(.24)
		case e.Hovered():
			fill = ui.Hex("#808080").Alpha(.14)
		}
	}
	return e.Background(fill)
}

// button is ui.Button that shrinks a little while pressed and springs back,
// as Apple's and Windows 11's buttons do.
func button(c *ui.Context, label string) ui.Element {
	e := ui.Button(c, label)
	pose := ui.Pose{}
	if e.Pressed() {
		pose.Scale = .96
	}
	return e.AnimatePose(pose, ui.Snappy)
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ListPane is a framed list with a bar of small icon buttons beneath it: the
// control a macOS app puts under a table (Add / Remove "gradient" or "square"
// buttons), that a Windows app draws as a toolbar below a list view, and that
// GNOME has as an inline toolbar under a frame. Content is the list (a
// ListBox, SourceList, TreeView, ...); Add, Remove and Extra are the bar's
// buttons. One widget, three native idioms, chosen by [ListBarStyle].
//
// The frame is one Theme.Border hairline round Content AND the bar; the bar
// shares the list's bottom border rather than adding a second one, and is
// filled with Theme.SurfaceAlt so it reads as a distinct strip against the
// Theme.Surface list. The bar's buttons are ordinary *Button values: set
// OnClick / Disabled as usual. Add and Remove are created icon-only, named
// "Add" and "Remove" (their Label, which is their accessible name and the text a
// host shows as a tooltip; [ListPane.ButtonAt] says which one the pointer is
// over). Extra holds optional trailing buttons (an action or export button).
//
// Per style (all metrics in logical pixels, through [scaled] and [TouchTarget]):
//
//   - [ListBarMac]: NSScrollView with a bezel border, buttons of
//     NSButton.BezelStyle.smallSquare joined with a spacing of -1 so neighbours
//     share a 1-px border, the bar overlapping the list's frame by one pixel,
//     and an inert filler bezel of the same face covering the rest of the bar
//     (mac.swift:89-133 in the WireGuard sources). Square buttons at least 26
//     wide, bar about 24 high, icons only (the HIG: square buttons hold symbols,
//     sit beneath the view they act on). Backspace as well as Delete removes.
//   - [ListBarWindows]: the list, then a ToolBar below it with no margin or
//     spacing, flat buttons, a SEPARATOR between the Add/Remove group and the
//     Extra group instead of joins, and "ImageBeforeText" so Add may carry a
//     caption after its icon (see [ListPane.SetAddText]); WireGuard's "Add
//     Tunnel" is a split action, which a host builds by putting a SplitButton in
//     Extra's place (tp.go:53-196, 252-272). Enabling is the caller's: delete
//     when something is selected, export when the list is not empty.
//   - [ListBarGNOME]: Adwaita 3's .inline-toolbar (g3.scss:1525-1533): 3-px
//     padding, borders on three sides, bottom corners rounded 5 px, a darker bar;
//     buttons .linked, rounded only at the ends of a group (g3.scss:1001-1008).
//     GTK4/libadwaita dropped the inline toolbar for a plain .toolbar with 6-px
//     spacing around .linked groups (adw.scss), which the gap between the two
//     groups follows.
//
// Events are routed by position: a click on a button is the button's, on the
// list is the list's, and on the filler, a separator or the padding is nobody's
// (a filler never reacts). Delete, and on Mac Backspace, fires Remove's OnClick
// when Remove is enabled. A disabled pane is inert, drops its children's input,
// and is announced disabled together with every descendant.
//
// The bar is its own accessibility node (a toolbar) whose children are the
// buttons, so a screen reader finds "Add" and "Remove" by name.
type ListPane struct {
	Base

	// Content is the list drawn inside the frame.
	Content Widget
	// Style picks the platform idiom. It has no automatic value: the caller
	// passes [ListBarStyleFor](runtime.GOOS) or whatever it means.
	Style ListBarStyle
	// AccessibleName names the pane for assistive technology ("Tunnels").
	AccessibleName string

	// Add and Remove are the bar's first two buttons, icon-only.
	Add, Remove *Button
	// Extra are optional trailing buttons in the same bar.
	Extra []*Button

	bar *listPaneBar
	lay listPaneLayout
}

// ListBarStyle selects which platform's bar a [ListPane] paints. There is
// deliberately no automatic zero value: pick one, or derive it with
// [ListBarStyleFor].
type ListBarStyle int

const (
	// ListBarMac is the joined smallSquare buttons under a bezel-bordered list.
	ListBarMac ListBarStyle = iota
	// ListBarWindows is a flat toolbar with separators between groups.
	ListBarWindows
	// ListBarGNOME is the Adwaita inline toolbar with linked, end-rounded groups.
	ListBarGNOME
)

// ListBarStyleFor maps a GOOS value to its native bar: "darwin" is Mac,
// "windows" is Windows, anything else is GNOME. The caller passes
// runtime.GOOS, so a test or a preview can ask for another system's.
func ListBarStyleFor(goos string) ListBarStyle {
	switch goos {
	case "darwin":
		return ListBarMac
	case "windows":
		return ListBarWindows
	}
	return ListBarGNOME
}

// ListPane metrics (logical pixels; every use goes through scaled / TouchTarget).
const (
	lpMacBarH        = 24 // smallSquare height, frame included
	lpMacBtnW        = 26 // smallSquare minimum width (mac.swift: width >= 26)
	lpWinBarH        = 28
	lpWinBtnW        = 32
	lpWinSepGap      = 4 // either side of a toolbar separator
	lpGnomePad       = 3 // .inline-toolbar padding
	lpGnomeBtn       = 26
	lpGnomeBtnW      = 32
	lpGnomeRadius    = 5 // .inline-toolbar bottom radius
	lpGnomeBtnRadius = 4
	lpGnomeGroupGap  = 6 // libadwaita .toolbar spacing between linked groups
)

// listPaneLayout is every rectangle a pane computed, in surface coordinates.
type listPaneLayout struct {
	frame, content, bar, filler Rect
	buttons                     []Rect // aligned with barButtons()
	groups                      []int  // group of each button (Add/Remove 0, Extra 1)
	seps                        []Rect // dividers and separators, in paint order
}

// NewListPane builds a pane over content with icon-only Add and Remove buttons.
// Nothing is wired: set Add.OnClick and Remove.OnClick.
func NewListPane(content Widget, style ListBarStyle) *ListPane {
	add := NewButton(Tr("listpane.add", "Add"), nil)
	add.Flat = true
	rm := NewButton(Tr("listpane.remove", "Remove"), nil)
	rm.Flat = true
	p := &ListPane{Content: content, Style: style, Add: add, Remove: rm}
	add.Icon = p.glyph(add, DrawIconPlus)
	rm.Icon = p.glyph(rm, DrawIconMinus)
	return p
}

// barNode is the bar's accessibility node, created on first use so a pane built
// as a literal works too.
func (p *ListPane) barNode() *listPaneBar {
	if p.bar == nil {
		p.bar = &listPaneBar{pane: p}
	}
	return p.bar
}

// glyph centres draw in a square a little over half the button's height, the
// weight of a smallSquare button's symbol. In the Windows style the square is the
// text's glyph height instead: the very box the captioned Add (SetAddText) draws
// its plus in, so the plus and the minus have the same bar beside each other.
func (p *ListPane) glyph(b *Button, draw func(painter.Painter, Rect, RGBA)) func(painter.Painter, Rect, RGBA) {
	return func(pt painter.Painter, r Rect, ink RGBA) {
		s := r.H * 14 / 24
		if p.Style == ListBarWindows {
			s = b.glyphHeight()
		}
		if s < 5 {
			s = 5
		}
		draw(pt, Rect{X: r.X + (r.W-s)/2, Y: r.Y + (r.H-s)/2, W: s, H: s}, ink)
	}
}

// SetAddText gives Add a caption after its icon (Windows' ImageBeforeText): the
// icon becomes a leading icon and text the label, which is also its name.
func (p *ListPane) SetAddText(text string) {
	if p.Add == nil {
		return
	}
	p.Add.Icon = nil
	p.Add.LeadingIcon().Set(DrawIconPlus)
	p.Add.Label().Set(text)
}

// barButtons is the bar's buttons in order, skipping nil slots.
func (p *ListPane) barButtons() []*Button {
	out := make([]*Button, 0, 2+len(p.Extra))
	for _, b := range []*Button{p.Add, p.Remove} {
		if b != nil {
			out = append(out, b)
		}
	}
	for _, b := range p.Extra {
		if b != nil {
			out = append(out, b)
		}
	}
	return out
}

func (p *ListPane) groupOf(b *Button) int {
	if b == p.Add || b == p.Remove {
		return 0
	}
	return 1
}

func (p *ListPane) barHeight() int {
	switch p.Style {
	case ListBarWindows:
		return TouchTarget(scaled(lpWinBarH))
	case ListBarGNOME:
		return TouchTarget(scaled(lpGnomeBtn)) + 2*scaled(lpGnomePad) + 2*strokeWidth()
	}
	return TouchTarget(scaled(lpMacBarH))
}

func (p *ListPane) btnWidth(b *Button) int {
	base := lpMacBtnW
	switch p.Style {
	case ListBarWindows:
		base = lpWinBtnW
	case ListBarGNOME:
		base = lpGnomeBtnW
	}
	w := TouchTarget(scaled(base))
	if b.Icon == nil {
		if pw := b.PreferredWidth(); pw > w {
			w = pw
		}
	}
	return w
}

// SetBounds lays the frame, the list, the bar and every button out.
func (p *ListPane) SetBounds(r Rect) {
	p.Base.SetBounds(r)
	p.layout(r)
	if p.Content != nil {
		p.Content.SetBounds(p.lay.content)
	}
	p.barNode().SetBounds(p.lay.bar)
	for i, b := range p.barButtons() {
		b.SetBounds(p.lay.buttons[i])
	}
}

func (p *ListPane) layout(r Rect) {
	bw := strokeWidth()
	barH := p.barHeight()
	if barH > r.H {
		barH = r.H
	}
	l := listPaneLayout{frame: r}
	l.bar = Rect{X: r.X, Y: r.Y + r.H - barH, W: r.W, H: barH}
	top := r.Y + bw
	l.content = Rect{X: r.X + bw, Y: top, W: r.W - 2*bw, H: l.bar.Y - top}
	if l.content.W < 0 {
		l.content.W = 0
	}
	if l.content.H < 0 {
		l.content.H = 0
	}
	btns := p.barButtons()
	right := r.X + r.W - bw
	x := r.X + bw
	y := l.bar.Y + bw // the bar's top border is the list's bottom border
	h := barH - 2*bw
	if p.Style == ListBarGNOME {
		pad := scaled(lpGnomePad)
		x += pad
		y += pad
		h -= 2 * pad
	}
	if h < 0 {
		h = 0
	}
	for i, b := range btns {
		g := p.groupOf(b)
		w := p.btnWidth(b)
		if i > 0 {
			prev := p.groupOf(btns[i-1])
			switch {
			case p.Style == ListBarMac:
				// shared 1-px border between neighbours
			case g != prev && p.Style == ListBarWindows:
				gap := scaled(lpWinSepGap)
				l.seps = append(l.seps, Rect{X: x + gap, Y: y + gap, W: bw, H: h - 2*gap})
				x += 2*gap + bw
			case g != prev: // GNOME: groups are spaced, not separated
				x += scaled(lpGnomeGroupGap)
			case p.Style == ListBarGNOME:
				l.seps = append(l.seps, Rect{X: x, Y: y, W: bw, H: h})
				x += bw
			}
		}
		l.buttons = append(l.buttons, Rect{X: x, Y: y, W: w, H: h})
		l.groups = append(l.groups, g)
		x += w
		if p.Style == ListBarMac {
			l.seps = append(l.seps, Rect{X: x, Y: y, W: bw, H: h})
			x += bw
		}
	}
	if right > x {
		l.filler = Rect{X: x, Y: y, W: right - x, H: h}
	}
	p.lay = l
}

// FrameRect, ContentRect, BarRect and FillerRect expose the laid-out
// rectangles (surface coordinates) for hosts that overlay or test. FillerRect
// is empty when the buttons fill the bar.
func (p *ListPane) FrameRect() Rect   { return p.lay.frame }
func (p *ListPane) ContentRect() Rect { return p.lay.content }
func (p *ListPane) BarRect() Rect     { return p.lay.bar }
func (p *ListPane) FillerRect() Rect  { return p.lay.filler }

// ButtonAt reports the bar button under a pane-local point, or nil: a host uses
// it to show the button's Label as a tooltip.
func (p *ListPane) ButtonAt(x, y int) *Button {
	r := p.Bounds()
	sx, sy := x+r.X, y+r.Y
	for _, b := range p.barButtons() {
		if b.Bounds().Contains(sx, sy) {
			return b
		}
	}
	return nil
}

// Draw paints the frame, the list, the bar and its buttons, all clipped to the
// pane's bounds.
func (p *ListPane) Draw(p2 painter.Painter, theme *Theme) {
	r := p.Bounds()
	if len(p.lay.buttons) != len(p.barButtons()) {
		p.SetBounds(r) // buttons added after the last layout
	}
	withClip(p2, r, func() { p.draw(p2, theme) })
}

func (p *ListPane) draw(pt painter.Painter, theme *Theme) {
	l := p.lay
	gnome := p.Style == ListBarGNOME
	rad := scaled(lpGnomeRadius)

	// The list's face.
	if gnome {
		fillRoundRect(pt, l.frame.X, l.frame.Y, l.frame.W, l.frame.H, rad, theme.Surface)
	} else {
		fillRect(pt, l.frame.X, l.frame.Y, l.frame.W, l.frame.H, theme.Surface)
	}
	if p.Content != nil {
		withClip(pt, l.content, func() { p.Content.Draw(pt, theme) })
	}

	// The bar: a distinct face, rounded at the bottom only on GNOME.
	if gnome {
		fillRoundRect(pt, l.bar.X, l.bar.Y, l.bar.W, l.bar.H, rad, theme.SurfaceAlt)
		fillRect(pt, l.bar.X, l.bar.Y, l.bar.W, rad, theme.SurfaceAlt)
	} else {
		fillRect(pt, l.bar.X, l.bar.Y, l.bar.W, l.bar.H, theme.SurfaceAlt)
	}

	// The frame, with the bar's top edge doubling as the list's bottom border.
	if gnome {
		above := Rect{X: l.frame.X, Y: l.frame.Y, W: l.frame.W, H: l.bar.Y - l.frame.Y}
		below := Rect{X: l.frame.X, Y: l.bar.Y, W: l.frame.W, H: l.bar.H}
		withClip(pt, above, func() { strokeRect(pt, l.frame.X, l.frame.Y, l.frame.W, l.frame.H, theme.Border) })
		withClip(pt, below, func() { strokeRoundRect(pt, l.frame.X, l.frame.Y, l.frame.W, l.frame.H, rad, theme.Border) })
	} else {
		strokeRect(pt, l.frame.X, l.frame.Y, l.frame.W, l.frame.H, theme.Border)
	}
	fillRect(pt, l.bar.X+strokeWidth(), l.bar.Y, l.bar.W-2*strokeWidth(), strokeWidth(), theme.Border)

	for _, s := range l.seps {
		fillRect(pt, s.X, s.Y, s.W, s.H, theme.Border)
	}
	p.drawButtons(pt, theme)
}

// drawButtons paints each button's face and glyph. The bar owns the chrome, so
// a button's own border is never drawn; hover, press, Selected and disabled
// states follow Button's own.
func (p *ListPane) drawButtons(pt painter.Painter, theme *Theme) {
	off := p.Disabled().Get()
	btns := p.barButtons()
	for i, b := range btns {
		r := p.lay.buttons[i]
		face, ink := theme.SurfaceAlt, theme.OnSurface
		switch {
		case b.Selected().Get():
			face, ink = theme.Accent, accentFg(theme)
		case b.pressed:
			face, ink = theme.Accent, theme.Background
		case b.hovered:
			face = blendRGBA(theme.SurfaceAlt, theme.Surface, 0.5)
		}
		if off || b.Disabled().Get() {
			face, ink = theme.SurfaceAlt, mutedInk(theme)
		}
		if p.Style == ListBarGNOME {
			first := i == 0 || p.lay.groups[i-1] != p.lay.groups[i]
			last := i == len(btns)-1 || p.lay.groups[i+1] != p.lay.groups[i]
			drawLinkedFace(pt, r, scaled(lpGnomeBtnRadius), first, last, face)
		} else {
			fillRect(pt, r.X, r.Y, r.W, r.H, face)
		}
		if b.Icon != nil {
			b.Icon(pt, r, ink)
		} else {
			b.drawContent(pt, theme, r, ink)
		}
		b.drawFocusRing(pt, theme, r)
	}
	if p.Style == ListBarGNOME {
		for _, g := range []int{0, 1} {
			if gr, ok := p.groupRect(g); ok {
				strokeRoundRect(pt, gr.X, gr.Y, gr.W, gr.H, scaled(lpGnomeBtnRadius), theme.Border)
			}
		}
	}
}

// groupRect is the union of one group's buttons.
func (p *ListPane) groupRect(g int) (Rect, bool) {
	var out Rect
	found := false
	for i, r := range p.lay.buttons {
		if p.lay.groups[i] != g {
			continue
		}
		if !found {
			out, found = r, true
			continue
		}
		out.W = r.X + r.W - out.X
	}
	return out, found
}

// drawLinkedFace fills r rounded at the outer corners only: a linked group's
// first button is rounded on the left, its last on the right.
func drawLinkedFace(pt painter.Painter, r Rect, rad int, first, last bool, c RGBA) {
	fillRoundRect(pt, r.X, r.Y, r.W, r.H, rad, c)
	if !last {
		fillRect(pt, r.X+r.W/2, r.Y, r.W-r.W/2, r.H, c)
	}
	if !first {
		fillRect(pt, r.X, r.Y, r.W/2, r.H, c)
	}
}

// OnEvent routes by position. Keys go to the focus system, except Delete (and
// Backspace on Mac), which fires Remove.
func (p *ListPane) OnEvent(ev Event) {
	if p.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventKeyDown:
		if ev.Code == "Delete" || (ev.Code == "Backspace" && p.Style == ListBarMac) {
			if rm := p.Remove; rm != nil && !rm.Disabled().Get() {
				rm.activate()
			}
			return
		}
		routeFocusKey(p, ev)
		return
	case EventChar:
		routeFocusKey(p, ev)
		return
	}
	pr := p.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	btns := p.barButtons()
	toContent := func() {
		if p.Content != nil {
			p.Content.OnEvent(translateEvent(ev, pr, p.Content.Bounds()))
		}
	}
	switch ev.Kind {
	case EventMouseMove, EventMouseUp:
		// Delivered to everyone so a hover or a press clears when the pointer leaves.
		for _, b := range btns {
			b.OnEvent(translateEvent(ev, pr, b.Bounds()))
		}
		toContent()
		return
	case EventMouseDrag:
		toContent()
		return
	case EventClick:
		focusClick(p, sx, sy)
	}
	for _, b := range btns {
		if b.Bounds().Contains(sx, sy) {
			b.OnEvent(translateEvent(ev, pr, b.Bounds()))
			return
		}
	}
	// Filler, separator and padding of the bar are inert: the content's bounds
	// stop above the bar, so its HitTest refuses them.
	if p.Content != nil && p.Content.HitTest(sx, sy) {
		toContent()
	}
}

func (p *ListPane) focusableChildren() []Widget { return p.Children() }

// A11y reports the pane as a group named by AccessibleName.
func (p *ListPane) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: p.AccessibleName, Disabled: p.Disabled().Get()}
}

func (*ListPane) a11yDisablesSubtree() bool { return true }

// listPaneBar is the bar as a node of its own: a toolbar whose children are the
// buttons. It paints and handles nothing; the pane does both.
type listPaneBar struct {
	Base
	pane *ListPane
}

func (b *listPaneBar) Children() []Widget {
	btns := b.pane.barButtons()
	out := make([]Widget, len(btns))
	for i, x := range btns {
		out[i] = x
	}
	return out
}

func (b *listPaneBar) focusableChildren() []Widget { return b.Children() }

func (b *listPaneBar) A11y() A11yInfo {
	return A11yInfo{Role: RoleToolbar, Name: Tr("listpane.bar", "List actions"), Disabled: b.pane.Disabled().Get()}
}

func (*listPaneBar) a11yDisablesSubtree() bool { return true }

// Disabled follows the pane, so the focus walk skips the bar's buttons with it.
func (b *listPaneBar) isDisabled() bool { return b.pane.Disabled().Get() }

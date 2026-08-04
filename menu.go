// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// MenuItem is one row in a Menu. Label is the human text; Action is
// the callback fired on click. A nil Action turns the row into a
// disabled (greyed-out) entry; a non-empty Submenu lets it open a
// nested Menu (popover) on hover or click.
//
// Separator items render as a thin SurfaceAlt line + are not
// clickable. They have empty Label + nil Action.
//
// Shortcut is a hint string ("Ctrl+N", "Cmd+O", …) drawn right-aligned
// on the row in the muted SurfaceAlt tone. Purely visual: the host
// app is responsible for actually wiring the key combo to the
// item's Action (there is no cross-platform "Ctrl vs Cmd" logic in
// the toolkit — different apps route keys through different SDKs).
//
// Checkable marks the row as a toggle: activating it flips Checked
// (in addition to still calling Action, if any) + the row renders a
// ✓ glyph in a left-hand gutter when Checked.
//
// RadioGroup, when non-zero, makes the row a member of a mutually
// exclusive set: every item in the same Menu sharing the same
// RadioGroup value is a sibling. Activating one sets its Checked to
// true + clears Checked on every sibling; the row renders a •
// (bullet) glyph instead of a check mark. RadioGroup implies
// checkable behaviour — Checkable does not need to also be set.
// RadioGroup == 0 means "not part of any radio group".
type MenuItem struct {
	Label      string
	Action     func()
	Submenu    *Menu
	Separator  bool
	Shortcut   string
	Checkable  bool
	Checked    bool
	RadioGroup int
}

// isCheckish reports whether it should reserve/paint a check or
// bullet glyph slot — either explicitly Checkable, or a member of a
// radio group (RadioGroup implies checkable).
func (it *MenuItem) isCheckish() bool { return it.Checkable || it.RadioGroup != 0 }

// Menu is a vertical popover-style list of MenuItems. Used by the
// compositor's right-click root menu, by MenuBar drop-downs and by
// any widget that needs an Openbox-style picker.
type Menu struct {
	Base
	Items   []MenuItem
	Hover   int // index of hovered row, -1 if none
	OnClose func()

	// openSub is the index of the row whose Submenu is currently open, or -1
	// when none is. A click or hover on a submenu-parent row (or ArrowRight on
	// it) opens its child Menu beside the row; ArrowLeft/Escape closes it.
	// While open, pointer + key events that fall on the child route into it.
	openSub int
}

// MenuRowH is the pixel height of a menu row.
const MenuRowH = 22

// MenuSeparatorH is the height of a separator row.
const MenuSeparatorH = 6

// MenuCheckGutterW is the pixel width of the left-hand gutter that
// holds a checkable/radio row's ✓ or • glyph. A Menu only reserves
// this gutter (shifting every row's label right) when at least one of
// its Items is checkable or belongs to a radio group; a Menu with no
// such items lays out exactly as before this feature (label starts at
// the plain 8px inset), so plain menus render unchanged.
const MenuCheckGutterW = 14

// NewMenu builds a Menu with the given items + Hover and openSub at -1.
func NewMenu(items []MenuItem) *Menu { return &Menu{Items: items, Hover: -1, openSub: -1} }

// hasCheckGutter reports whether any item wants a check/bullet glyph,
// i.e. whether Draw must reserve MenuCheckGutterW before the label.
func (m *Menu) hasCheckGutter() bool {
	for i := range m.Items {
		if m.Items[i].isCheckish() {
			return true
		}
	}
	return false
}

// Draw paints the menu's body + every row + a hover highlight on the
// currently-hovered row.
func (m *Menu) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	gutter := 0
	if m.hasCheckGutter() {
		gutter = MenuCheckGutterW
	}
	y := r.Y + 2
	for i, it := range m.Items {
		if it.Separator {
			sep := y + MenuSeparatorH/2
			fillRect(p, r.X+4, sep, r.W-8, 1, theme.SurfaceAlt)
			y += MenuSeparatorH
			continue
		}
		if i == m.Hover && (it.Action != nil || it.Submenu != nil) {
			fillRect(p, r.X+1, y, r.W-2, MenuRowH, theme.Accent)
		}
		ink := theme.OnSurface
		switch {
		case it.Action == nil && it.Submenu == nil:
			ink = theme.SurfaceAlt // disabled = greyed out (no action, no submenu)
		case i == m.Hover:
			ink = theme.Background // hovered row: invert ink
		}
		textY := y + (MenuRowH-m.glyphHeight())/2
		if it.isCheckish() && it.Checked {
			m.drawCheckGlyph(p, r.X+8, y, it.RadioGroup != 0, ink)
		}
		m.drawText(p, r.X+8+gutter, textY, it.Label, ink)
		if it.Submenu != nil {
			// ▶ chevron on the right edge to signal a nested menu.
			// Flat left (tallest column, x = cx-1), point on right
			// (1-pixel tip, x = cx+2).
			cx := r.X + r.W - 8
			cy := y + MenuRowH/2
			for t := 0; t < 4; t++ {
				fillRect(p, cx+2-t, cy-t, 1, 1+2*t, ink)
			}
		} else if it.Shortcut != "" {
			// Right-align the shortcut hint in a muted tone. Skipped when
			// the row has a Submenu (the chevron already occupies the
			// right edge). Muted ink follows the row's active/inactive
			// state so a hovered row's shortcut inverts too.
			sw := m.textWidth(it.Shortcut)
			sx := r.X + r.W - 8 - sw
			shortcutInk := theme.SurfaceAlt
			if i == m.Hover && it.Action != nil {
				shortcutInk = theme.Background
			}
			m.drawText(p, sx, textY, it.Shortcut, shortcutInk)
		}
		y += MenuRowH
	}
	// An open submenu is painted last so it overlays the body: its child Menu
	// is positioned beside the parent row (see subBounds) and drawn recursively,
	// so nested submenus chain naturally.
	if sub, cb, ok := m.openSubmenu(); ok {
		sub.SetBounds(cb)
		sub.Draw(p, theme)
	}
}

// drawCheckGlyph paints a Checked row's indicator in the check gutter:
// a • bullet (filled square dot) for a radio item, or a ✓-ish check
// mark (two pixel strokes, mirroring CheckButton's approximation) for
// a plain checkable item. gx is the gutter's left edge, rowY the
// row's top (widget-local), both in the same frame as Draw's y.
func (m *Menu) drawCheckGlyph(p painter.Painter, gx, rowY int, radio bool, ink RGBA) {
	const glyphBox = 10
	boxY := rowY + (MenuRowH-glyphBox)/2
	if radio {
		fillRect(p, gx+3, boxY+3, glyphBox-6, glyphBox-6, ink)
		return
	}
	for t := 0; t < 3; t++ {
		fillRect(p, gx+t, boxY+5+t, 1, 1, ink)
	}
	for t := 0; t < 5; t++ {
		fillRect(p, gx+2+t, boxY+8-t, 1, 1, ink)
	}
}

// MenuMinW is the floor width a submenu popover sizes to (see preferredSize) so
// a child of very short labels still reads as a panel.
const MenuMinW = 96

// OnEvent: a click on an enabled row fires its Action + closes the menu via
// OnClose (if wired); a click (or hover, or ArrowRight) on a submenu-parent row
// opens its child Menu beside the row. Keyboard: ArrowUp/ArrowDown move the
// Hover highlight to the previous/next navigable row (skipping separators and
// disabled rows, wrapping at both ends); ArrowRight opens the hovered submenu;
// Enter/Space activate the hovered row (opening its submenu, or firing its
// Action); Escape calls OnClose. While a submenu is open, pointer events over
// the child route into it, keys drive the child, and ArrowLeft/Escape close it.
// A disabled Menu ignores keys.
func (m *Menu) OnEvent(ev Event) {
	// An open submenu gets first crack: pointer events on the child route into
	// it; the child owns keys except ArrowLeft/Escape, which close it. Keep the
	// child's Bounds in sync with cb so its own hit-testing (localInBounds) sees
	// the position it was drawn at, even if no Draw ran since it opened.
	if sub, cb, ok := m.openSubmenu(); ok {
		r := m.Bounds()
		sub.SetBounds(cb)
		switch ev.Kind {
		case EventClick, EventMouseMove:
			if cb.Contains(ev.X+r.X, ev.Y+r.Y) {
				sub.OnEvent(translateEvent(ev, r, cb))
				return
			}
		case EventKeyDown:
			if m.Disabled {
				return
			}
			switch ev.Code {
			case "ArrowLeft", "Escape":
				m.openSub = -1
			default:
				sub.OnEvent(ev)
			}
			return
		}
	}
	switch ev.Kind {
	case EventMouseMove:
		// Follow the pointer: highlight the row under it, opening its submenu
		// (or closing any open one) as the hovered parent changes, or clear the
		// highlight when the pointer has moved off the menu body.
		if m.localInBounds(ev.X, ev.Y) {
			idx := m.rowAt(ev.Y)
			m.Hover = idx
			if m.isSubmenuParent(idx) {
				m.openSubAt(idx)
			} else {
				m.openSub = -1
			}
		} else {
			m.Hover = -1
		}
		return
	case EventKeyDown:
		if m.Disabled {
			return
		}
		switch ev.Code {
		case "ArrowDown":
			m.moveHover(1)
		case "ArrowUp":
			m.moveHover(-1)
		case "ArrowRight":
			// Open the hovered row's submenu (if any) and seed its first item so
			// the very next arrow moves within the child.
			if m.isSubmenuParent(m.Hover) {
				m.openSubAt(m.Hover)
				m.Items[m.Hover].Submenu.moveHover(1)
			}
		case "Enter", " ", "Space":
			m.activate(m.Hover)
		case "Escape":
			if m.OnClose != nil {
				m.OnClose()
			}
		}
		return
	case EventClick:
		m.activate(m.rowAt(ev.Y))
	}
}

// activate handles row idx's activation -- the single path a click and an
// Enter/Space keypress share. A submenu-parent row opens its submenu; a plain
// row applies its checkable/radio state change, fires Action, and closes the
// menu via OnClose. An out-of-range index, a separator, or a disabled
// (nil-Action, no-submenu) row is a no-op.
func (m *Menu) activate(idx int) {
	if !m.enabledItem(idx) {
		return
	}
	it := &m.Items[idx]
	if it.Submenu != nil {
		m.openSubAt(idx)
		return
	}
	switch {
	case it.RadioGroup != 0:
		m.selectRadio(idx)
	case it.Checkable:
		it.Checked = !it.Checked
	}
	it.Action()
	if m.OnClose != nil {
		m.OnClose()
	}
}

// enabledItem reports whether row i is navigable: in range, not a separator, and
// carrying either an Action or a Submenu. A submenu-parent row is navigable even
// though its Action is nil (Wave 4) -- activate opens the submenu for it rather
// than firing an Action. A row with neither Action nor Submenu is the disabled
// (greyed) state.
func (m *Menu) enabledItem(i int) bool {
	if i < 0 || i >= len(m.Items) {
		return false
	}
	it := &m.Items[i]
	return !it.Separator && (it.Action != nil || it.Submenu != nil)
}

// isSubmenuParent reports whether row i is a navigable submenu-parent row (in
// range, not a separator, carrying a non-nil Submenu).
func (m *Menu) isSubmenuParent(i int) bool {
	return i >= 0 && i < len(m.Items) && !m.Items[i].Separator && m.Items[i].Submenu != nil
}

// openSubAt marks row idx's submenu open and wires the child's OnClose to this
// menu's OnClose, so activating an item inside the child closes the whole chain.
func (m *Menu) openSubAt(idx int) {
	m.openSub = idx
	if sub := m.Items[idx].Submenu; sub != nil && m.OnClose != nil {
		sub.OnClose = m.OnClose
	}
}

// openSubmenu returns the currently-open child Menu, its bounds (beside the
// parent row), and true -- or ok == false when no submenu is open.
func (m *Menu) openSubmenu() (*Menu, Rect, bool) {
	if m.openSub < 0 || m.openSub >= len(m.Items) {
		return nil, Rect{}, false
	}
	sub := m.Items[m.openSub].Submenu
	if sub == nil {
		return nil, Rect{}, false
	}
	return sub, m.subBounds(m.openSub), true
}

// subBounds places row idx's submenu to the right of the menu, aligned with the
// row's top, sized to the child's preferred footprint. (Edge-flipping / scroll
// on overflow is Wave 6.)
func (m *Menu) subBounds(idx int) Rect {
	r := m.Bounds()
	w, h := m.Items[idx].Submenu.preferredSize()
	return Rect{X: r.X + r.W, Y: r.Y + m.rowTop(idx), W: w, H: h}
}

// rowTop is the widget-local top Y of row idx -- the inverse of rowAt, summing
// the heights (MenuRowH, or MenuSeparatorH for separators) of the rows above it
// past the 2px body inset.
func (m *Menu) rowTop(idx int) int {
	cy := 2
	for k := 0; k < idx && k < len(m.Items); k++ {
		if m.Items[k].Separator {
			cy += MenuSeparatorH
		} else {
			cy += MenuRowH
		}
	}
	return cy
}

// preferredSize measures the menu's popover footprint: width is the widest row
// (label + check gutter + shortcut or submenu chevron) floored at MenuMinW;
// height is the summed row heights plus the 4px body inset. Used to size a
// submenu when it opens.
func (m *Menu) preferredSize() (w, h int) {
	w = MenuMinW
	h = 4
	gutter := 0
	if m.hasCheckGutter() {
		gutter = MenuCheckGutterW
	}
	for i := range m.Items {
		it := &m.Items[i]
		if it.Separator {
			h += MenuSeparatorH
			continue
		}
		rowW := 16 + gutter + m.textWidth(it.Label)
		if it.Submenu != nil {
			rowW += 12
		} else if it.Shortcut != "" {
			rowW += 12 + m.textWidth(it.Shortcut)
		}
		if rowW > w {
			w = rowW
		}
		h += MenuRowH
	}
	return w, h
}

// moveHover advances the Hover highlight by dir (+1 down, -1 up) to the next
// enabled row, skipping separators and disabled/nil-Action rows and wrapping
// at both ends. From no hover (-1) a downward step lands on the first enabled
// row and an upward step on the last. A menu with no enabled row leaves Hover
// untouched.
func (m *Menu) moveHover(dir int) {
	n := len(m.Items)
	if n == 0 {
		return
	}
	cur := m.Hover
	if cur < 0 {
		// Seed so the first step lands on the first (down) or last (up) row.
		if dir < 0 {
			cur = 0
		} else {
			cur = -1
		}
	}
	for k := 0; k < n; k++ {
		cur = (cur + dir + n) % n
		if m.enabledItem(cur) {
			m.Hover = cur
			return
		}
	}
}

// selectRadio sets Items[idx].Checked = true + clears Checked on every
// other item in m.Items that shares Items[idx].RadioGroup (siblings
// live within one Menu's Items — a submenu's items are a separate
// Menu + never touched). Items outside the group (RadioGroup == 0 or
// a different group) are left untouched.
func (m *Menu) selectRadio(idx int) {
	group := m.Items[idx].RadioGroup
	for i := range m.Items {
		if m.Items[i].RadioGroup == group {
			m.Items[i].Checked = i == idx
		}
	}
}

// SetHover updates Hover based on a mouse-Y coordinate (widget-local).
// Useful for keyboard / mouse-move handlers that want to highlight
// the row the user is pointing at.
func (m *Menu) SetHover(y int) { m.Hover = m.rowAt(y) }

// rowAt returns the item index at widget-local y, or -1 if none.
func (m *Menu) rowAt(y int) int {
	if y < 2 {
		return -1
	}
	cy := 2
	for i, it := range m.Items {
		h := MenuRowH
		if it.Separator {
			h = MenuSeparatorH
		}
		if y >= cy && y < cy+h {
			return i
		}
		cy += h
	}
	return -1
}

// MenuBar is a horizontal strip of top-level menu names (File, Edit,
// View, ...). Clicking a name opens its associated Menu as a popover
// just below the strip.
//
// The MenuBar itself doesn't own the open Menu's drawing (the
// containing app composes it with whatever overlay surface it
// has); MenuBar just exposes Active so the app knows which menu
// to render.
type MenuBar struct {
	Base
	Names  []string
	Menus  []*Menu
	Active int // -1 if none open

	// hoverName tracks the top-level name under the pointer as a 1-based index
	// (0 = none), set on EventMouseMove. The +1 offset keeps the zero value
	// (no hover) safe for a literal MenuBar{}, so the resting draw is unchanged.
	hoverName int
}

// MenuBarH is the pixel height of the bar strip.
const MenuBarH = 22

// MenuBarItemW is the DEFAULT (minimum) pixel width allocated per
// top-level name. Names whose TextWidth exceeds this bound scale up
// (with 2×MenuBarItemPadX horizontal padding on each side); shorter
// names take exactly this width so the bar looks stable across
// varying label lengths.
const MenuBarItemW = 60

// MenuBarItemPadX is the horizontal padding around a top-level name
// when its natural width exceeds MenuBarItemW — i.e. the extra
// breathing room beyond the raw glyph run.
const MenuBarItemPadX = 8

// NewMenuBar builds a MenuBar (Active = -1).
func NewMenuBar() *MenuBar { return &MenuBar{Active: -1} }

// AddMenu appends (name, menu) to the bar.
func (b *MenuBar) AddMenu(name string, m *Menu) {
	b.Names = append(b.Names, name)
	b.Menus = append(b.Menus, m)
}

// NameWidth returns the pixel width of the i-th top-level name after
// the auto-size rule: max(MenuBarItemW, TextWidth(name) + 2*pad).
// Exposed so hosts that render their own popover under a clicked
// name know how wide the "click zone" was.
func (b *MenuBar) NameWidth(i int) int {
	if i < 0 || i >= len(b.Names) {
		return MenuBarItemW
	}
	w := b.textWidth(b.Names[i]) + 2*MenuBarItemPadX
	if w < MenuBarItemW {
		return MenuBarItemW
	}
	return w
}

// NameOriginX returns the X offset of the i-th top-level name within
// the bar (cumulative sum of NameWidth up to i, exclusive). Same
// motivation as NameWidth: a host that positions a popover under a
// clicked name reads this to align on the correct column.
func (b *MenuBar) NameOriginX(i int) int {
	x := 0
	for k := 0; k < i && k < len(b.Names); k++ {
		x += b.NameWidth(k)
	}
	return x
}

// Draw paints the bar + every name + a highlight on the Active name.
func (b *MenuBar) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	fillRect(p, r.X, r.Y, r.W, MenuBarH, theme.SurfaceAlt)
	for i, name := range b.Names {
		iw := b.NameWidth(i)
		ix := r.X + b.NameOriginX(i)
		ink := theme.OnSurface
		switch {
		case i == b.Active:
			fillRect(p, ix, r.Y, iw, MenuBarH, theme.Accent)
			ink = theme.Background
		case b.hoverName == i+1:
			// A hovered (but not open) name raises to Surface — a subtle
			// lighter cell against the SurfaceAlt bar. Skipped for the active
			// name, whose Accent highlight already wins.
			fillRect(p, ix, r.Y, iw, MenuBarH, theme.Surface)
		}
		tw := b.textWidth(name)
		textX := ix + (iw-tw)/2
		textY := r.Y + (MenuBarH-b.glyphHeight())/2
		b.drawText(p, textX, textY, name, ink)
	}
}

// OnEvent: a click on a name toggles its menu (Active = idx or -1).
// Also honours mnemonic keyboard shortcuts on EventKeyDown when the
// Code carries an "Alt+X" hint (X = one of the top-level names'
// first letter, case-insensitive) — matches the GNOME/Windows
// menu-bar Alt+letter convention. The host is responsible for
// formatting the key event's Code as "Alt+F" etc. before forwarding.
func (b *MenuBar) OnEvent(ev Event) {
	switch ev.Kind {
	case EventMouseMove:
		// Track the name under the pointer for the hover highlight; clear it
		// when the pointer leaves the strip.
		if ev.Y < 0 || ev.Y >= MenuBarH {
			b.hoverName = 0
			return
		}
		b.hoverName = 0
		cx := 0
		for i := range b.Names {
			w := b.NameWidth(i)
			if ev.X >= cx && ev.X < cx+w {
				b.hoverName = i + 1
				return
			}
			cx += w
		}
	case EventClick:
		if ev.Y >= MenuBarH {
			return
		}
		// Auto-sized widths: walk the names + find whichever cell
		// contains ev.X. The old fixed-width formula (ev.X /
		// MenuBarItemW) breaks when names are wider than the default.
		idx := -1
		cx := 0
		for i := range b.Names {
			w := b.NameWidth(i)
			if ev.X >= cx && ev.X < cx+w {
				idx = i
				break
			}
			cx += w
		}
		if idx < 0 {
			return
		}
		if b.Active == idx {
			b.Active = -1
		} else {
			b.Active = idx
		}
	case EventKeyDown:
		// Mnemonic: "Alt+X" opens the FIRST menu whose Name starts with
		// X (case-insensitive). Escape closes the open menu. While a menu
		// is open, ArrowLeft/ArrowRight move Active between top-level menus
		// (wrapping) and ArrowDown enters the open menu's first item. A
		// disabled MenuBar ignores keys. Any other Code is ignored.
		if b.Disabled {
			return
		}
		if ev.Code == "Escape" {
			b.Active = -1
			return
		}
		switch ev.Code {
		case "ArrowLeft", "ArrowRight":
			if b.Active < 0 || len(b.Names) == 0 {
				return
			}
			dir := 1
			if ev.Code == "ArrowLeft" {
				dir = -1
			}
			b.Active = (b.Active + dir + len(b.Names)) % len(b.Names)
			return
		case "ArrowDown":
			// Enter the open menu: highlight its first enabled item.
			if b.Active < 0 || b.Active >= len(b.Menus) {
				return
			}
			if m := b.Menus[b.Active]; m != nil {
				m.moveHover(1)
			}
			return
		}
		const prefix = "Alt+"
		if len(ev.Code) != len(prefix)+1 || ev.Code[:len(prefix)] != prefix {
			return
		}
		want := ev.Code[len(prefix)]
		if want >= 'a' && want <= 'z' {
			want = want - 'a' + 'A'
		}
		for i, name := range b.Names {
			if name == "" {
				continue
			}
			first := name[0]
			if first >= 'a' && first <= 'z' {
				first = first - 'a' + 'A'
			}
			if first == want {
				b.Active = i
				return
			}
		}
	}
}

// HandleShortcut walks every menu's items and fires the Action of the
// first item whose Shortcut equals code (case-sensitive; the host is
// expected to normalise Ctrl+N vs Cmd+N before calling). Returns true
// if an item fired, false if no match. Menu ordering + item ordering
// give a deterministic priority — first match wins.
//
// Skipped: separators, disabled items (nil Action). A matching item
// with a submenu still fires its Action (if any); the submenu is not
// opened by a shortcut.
//
// Typical usage from a wasmbox client's Go main:
//
//	case "keydown":
//	    code := formatShortcut(ev)   // host builds "Ctrl+N" etc.
//	    if state.menuBar.HandleShortcut(code) { render(); return }
//	    state.editor.OnEvent(...)    // fallthrough: forward to focus
func (b *MenuBar) HandleShortcut(code string) bool {
	if code == "" {
		return false
	}
	for _, m := range b.Menus {
		if m == nil {
			continue
		}
		for _, it := range m.Items {
			if it.Separator || it.Action == nil {
				continue
			}
			if it.Shortcut == code {
				it.Action()
				return true
			}
		}
	}
	return false
}

// Mnemonic returns the first letter of the i-th menu name (upper-case,
// or 0 if the index is out of range / the name is empty). Useful for a
// host that wants to draw "_F_ile"-style underlines under the mnemonic
// character.
func (b *MenuBar) Mnemonic(i int) byte {
	if i < 0 || i >= len(b.Names) {
		return 0
	}
	n := b.Names[i]
	if n == "" {
		return 0
	}
	c := n[0]
	if c >= 'a' && c <= 'z' {
		c = c - 'a' + 'A'
	}
	return c
}

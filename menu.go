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
type MenuItem struct {
	Label     string
	Action    func()
	Submenu   *Menu
	Separator bool
	Shortcut  string
}

// Menu is a vertical popover-style list of MenuItems. Used by the
// compositor's right-click root menu, by MenuBar drop-downs and by
// any widget that needs an Openbox-style picker.
type Menu struct {
	Base
	Items    []MenuItem
	Hover    int // index of hovered row, -1 if none
	OnClose  func()
}

// MenuRowH is the pixel height of a menu row.
const MenuRowH = 22

// MenuSeparatorH is the height of a separator row.
const MenuSeparatorH = 6

// NewMenu builds a Menu with the given items + Hover at -1.
func NewMenu(items []MenuItem) *Menu { return &Menu{Items: items, Hover: -1} }

// Draw paints the menu's body + every row + a hover highlight on the
// currently-hovered row.
func (m *Menu) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	y := r.Y + 2
	for i, it := range m.Items {
		if it.Separator {
			sep := y + MenuSeparatorH/2
			fillRect(p, r.X+4, sep, r.W-8, 1, theme.SurfaceAlt)
			y += MenuSeparatorH
			continue
		}
		if i == m.Hover && it.Action != nil {
			fillRect(p, r.X+1, y, r.W-2, MenuRowH, theme.Accent)
		}
		ink := theme.OnSurface
		if it.Action == nil && !it.Separator {
			ink = theme.SurfaceAlt // disabled = greyed out
		} else if i == m.Hover {
			ink = theme.Background // hovered row: invert ink
		}
		textY := y + (MenuRowH-GlyphHeight)/2
		DrawText(p, r.X+8, textY, it.Label, ink)
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
			sw := TextWidth(it.Shortcut)
			sx := r.X + r.W - 8 - sw
			shortcutInk := theme.SurfaceAlt
			if i == m.Hover && it.Action != nil {
				shortcutInk = theme.Background
			}
			DrawText(p, sx, textY, it.Shortcut, shortcutInk)
		}
		y += MenuRowH
	}
}

// OnEvent: a click on an enabled row fires its Action + closes the
// menu via OnClose (if wired).
func (m *Menu) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	idx := m.rowAt(ev.Y)
	if idx < 0 || idx >= len(m.Items) {
		return
	}
	it := m.Items[idx]
	if it.Separator || it.Action == nil {
		return
	}
	it.Action()
	if m.OnClose != nil {
		m.OnClose()
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
	w := TextWidth(b.Names[i]) + 2*MenuBarItemPadX
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
		if i == b.Active {
			fillRect(p, ix, r.Y, iw, MenuBarH, theme.Accent)
			ink = theme.Background
		}
		tw := TextWidth(name)
		textX := ix + (iw-tw)/2
		textY := r.Y + (MenuBarH-GlyphHeight)/2
		DrawText(p, textX, textY, name, ink)
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
		// X (case-insensitive). Escape closes the open menu. Any other
		// Code is ignored.
		if ev.Code == "Escape" {
			b.Active = -1
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

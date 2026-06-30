// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// MenuItem is one row in a Menu. Label is the human text; Action is
// the callback fired on click. A nil Action turns the row into a
// disabled (greyed-out) entry; a non-empty Submenu lets it open a
// nested Menu (popover) on hover or click.
//
// Separator items render as a thin SurfaceAlt line + are not
// clickable. They have empty Label + nil Action.
type MenuItem struct {
	Label     string
	Action    func()
	Submenu   *Menu
	Separator bool
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
func (m *Menu) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := m.Bounds()
	fillRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(surface, surfaceW, r.X, r.Y, r.W, r.H, theme.Border)
	y := r.Y + 2
	for i, it := range m.Items {
		if it.Separator {
			sep := y + MenuSeparatorH/2
			fillRect(surface, surfaceW, r.X+4, sep, r.W-8, 1, theme.SurfaceAlt)
			y += MenuSeparatorH
			continue
		}
		if i == m.Hover && it.Action != nil {
			fillRect(surface, surfaceW, r.X+1, y, r.W-2, MenuRowH, theme.Accent)
		}
		ink := theme.OnSurface
		if it.Action == nil && !it.Separator {
			ink = theme.SurfaceAlt // disabled = greyed out
		} else if i == m.Hover {
			ink = theme.Background // hovered row: invert ink
		}
		textY := y + (MenuRowH-GlyphHeight)/2
		DrawText(surface, surfaceW, r.X+8, textY, it.Label, ink)
		if it.Submenu != nil {
			// ▶ chevron on the right edge to signal a nested menu.
			cx := r.X + r.W - 8
			cy := y + MenuRowH/2
			for t := 0; t < 4; t++ {
				fillRect(surface, surfaceW, cx-2+t, cy-t, 1, 1+2*t, ink)
			}
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

// MenuBarItemW is the pixel width allocated per top-level name.
const MenuBarItemW = 60

// NewMenuBar builds a MenuBar (Active = -1).
func NewMenuBar() *MenuBar { return &MenuBar{Active: -1} }

// AddMenu appends (name, menu) to the bar.
func (b *MenuBar) AddMenu(name string, m *Menu) {
	b.Names = append(b.Names, name)
	b.Menus = append(b.Menus, m)
}

// Draw paints the bar + every name + a highlight on the Active name.
func (b *MenuBar) Draw(surface []byte, surfaceW int, theme *Theme) {
	r := b.Bounds()
	fillRect(surface, surfaceW, r.X, r.Y, r.W, MenuBarH, theme.SurfaceAlt)
	for i, name := range b.Names {
		ix := r.X + i*MenuBarItemW
		ink := theme.OnSurface
		if i == b.Active {
			fillRect(surface, surfaceW, ix, r.Y, MenuBarItemW, MenuBarH, theme.Accent)
			ink = theme.Background
		}
		tw := TextWidth(name)
		textX := ix + (MenuBarItemW-tw)/2
		textY := r.Y + (MenuBarH-GlyphHeight)/2
		DrawText(surface, surfaceW, textX, textY, name, ink)
	}
}

// OnEvent: a click on a name toggles its menu (Active = idx or -1).
func (b *MenuBar) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	if ev.Y >= MenuBarH {
		return
	}
	idx := ev.X / MenuBarItemW
	if idx < 0 || idx >= len(b.Names) {
		return
	}
	if b.Active == idx {
		b.Active = -1
	} else {
		b.Active = idx
	}
}

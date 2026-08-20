// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ContextMenu is a right-click popup: a Menu that appears at an arbitrary point
// (the cursor), auto-sizes to its items, clamps itself inside the surface so it
// never spills off an edge, and dismisses when the user clicks outside it. It
// is the overlay wrapper the widget model was missing around the bare Menu,
// mirroring how DropDown/DatePicker own their pop-ups.
//
// The ContextMenu's own Bounds is the whole surface it may cover (so it can
// catch an outside-click anywhere); AnchorX/AnchorY and incoming event
// coordinates are in that same frame. Call Popup(x, y) to show it at a point.
type ContextMenu struct {
	Base
	Menu             *Menu
	open             *mvvm.Observable[bool] // reactive via Open()
	AnchorX, AnchorY int
}

// Open is reactive state as a shared [mvvm.Observable]; edits Set it. Lazily created.
func (c *ContextMenu) Open() *mvvm.Observable[bool] {
	if c.open == nil {
		c.open = mvvm.NewObservable[bool](false)
	}
	return c.open
}

// ContextMenuMinW is the floor on a context menu's width so a menu of very
// short labels still reads as a panel.
const ContextMenuMinW = 96

// NewContextMenu wraps the given Menu as a (closed) context menu.
func NewContextMenu(menu *Menu) *ContextMenu { return &ContextMenu{Menu: menu} }

// Popup opens the menu anchored at (x, y) and wires the Menu's OnClose so that
// activating an item (or the menu closing itself) also closes the overlay.
func (c *ContextMenu) Popup(x, y int) {
	c.AnchorX, c.AnchorY = x, y
	c.Open().Set(true)
	c.Menu.OnClose = func() { c.Open().Set(false) }
}

// Close hides the menu.
func (c *ContextMenu) Close() { c.Open().Set(false) }

// menuSize measures the popup: width is the widest row (label + shortcut or
// submenu chevron) floored at ContextMenuMinW; height is the summed row heights
// plus the 4px body inset.
func (c *ContextMenu) menuSize() (w, h int) { return c.Menu.preferredSize() }

// MenuBounds is the rect the Menu occupies: the measured size placed at the
// anchor, then shifted so it stays fully inside the surface (c.Bounds()).
func (c *ContextMenu) MenuBounds() Rect {
	w, h := c.menuSize()
	surf := c.Bounds()
	// A menu taller than the surface is clamped to the surface height so it
	// stays on screen; the wrapped Menu then scrolls its rows (Menu.scroll)
	// rather than spilling off the bottom. A menu that fits keeps its exact
	// measured height, so short menus are unchanged.
	if h > surf.H {
		h = surf.H
	}
	x, y := c.AnchorX, c.AnchorY
	if x+w > surf.X+surf.W {
		x = surf.X + surf.W - w
	}
	if y+h > surf.Y+surf.H {
		y = surf.Y + surf.H - h
	}
	if x < surf.X {
		x = surf.X
	}
	if y < surf.Y {
		y = surf.Y
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// Draw paints the Menu at its clamped bounds when open; nothing when closed.
func (c *ContextMenu) Draw(p painter.Painter, theme *Theme) {
	if !c.Open().Get() {
		return
	}
	c.Menu.SetBounds(c.MenuBounds())
	c.Menu.Draw(p, theme)
}

// OnEvent routes a click inside the menu to the Menu (translated to its local
// frame, so the hit row's Action fires and closes the overlay via OnClose); a
// click anywhere outside dismisses the menu.
func (c *ContextMenu) OnEvent(ev Event) {
	if !c.Open().Get() {
		return
	}
	mb := c.MenuBounds()
	if ev.Kind == EventMouseMove {
		// Forward the move to the Menu (translated into its frame) so its row
		// highlight follows the pointer; a move outside the menu clears it.
		c.Menu.SetBounds(mb)
		c.Menu.OnEvent(Event{Kind: EventMouseMove, X: ev.X - mb.X, Y: ev.Y - mb.Y})
		return
	}
	if ev.Kind == EventScroll {
		// Forward the wheel to the wrapped Menu so a context menu taller than
		// the surface scrolls its rows.
		c.Menu.SetBounds(mb)
		c.Menu.OnEvent(ev)
		return
	}
	if ev.Kind == EventKeyDown {
		// Keyboard drives the wrapped Menu directly: Up/Down move the hover,
		// Enter/Space fire the hovered item, Escape closes (via the Menu's
		// OnClose that Popup wired to clear c.Open().Get()).
		c.Menu.SetBounds(mb)
		c.Menu.OnEvent(ev)
		return
	}
	if ev.Kind != EventClick {
		return
	}
	c.Menu.SetBounds(mb)
	// A click counts as "inside" when it lands on the menu body OR on an open
	// submenu that spills beyond it -- otherwise a click on a submenu row would
	// be mistaken for an outside-click and dismiss the whole popup.
	inside := ev.X >= mb.X && ev.X < mb.X+mb.W && ev.Y >= mb.Y && ev.Y < mb.Y+mb.H
	if !inside {
		if _, cb, ok := c.Menu.openSubmenu(); ok && cb.Contains(ev.X, ev.Y) {
			inside = true
		}
	}
	if inside {
		c.Menu.OnEvent(Event{Kind: EventClick, X: ev.X - mb.X, Y: ev.Y - mb.Y})
		return
	}
	c.Open().Set(false) // outside click → dismiss
}

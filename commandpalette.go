// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// PaletteCommand is one entry in a CommandPalette: a human-readable Label the
// user searches for and an Action to run when it is chosen. Action may be nil
// (e.g. a placeholder or disabled entry); activating such a command simply
// dismisses the palette without running anything.
type PaletteCommand struct {
	Label  string
	Action func()
}

// CommandPalette is a centered overlay that combines a search-query input row
// with a filtered, keyboard-navigable list of commands — the "Ctrl+Shift+P"
// palette pattern. It layers a SearchEntry-style query field over a ListBox-
// style result list and, like ContextMenu, catches an outside-click anywhere
// on its surface to dismiss itself.
//
// The palette's own Bounds is the whole surface it may cover (so it can catch
// an outside-click anywhere); the panel is measured and centered inside that
// frame, and incoming event coordinates are in that same surface frame.
//
// Selected always indexes into the FILTERED list (the indices returned by
// filtered()), never into Commands directly, and is re-clamped after any
// mutation to Query so it can never point past the end of a shrinking list.
type CommandPalette struct {
	Base
	Commands  []PaletteCommand
	Query     string
	Selected  int
	Visible   bool
	OnDismiss func()
}

// PaletteMinW is the floor on the panel width so a palette of short labels
// still reads as a dialog.
const PaletteMinW = 240

// PaletteRowH is the pixel height of every row (the query row and each result
// row).
const PaletteRowH = 18

// PalettePadX is the horizontal padding between the panel border and its text
// content.
const PalettePadX = 8

// palettePadY is the vertical inset above the query row and below the last
// result row.
const palettePadY = 4

// paletteCaret is the trailing marker drawn after the query text to hint at the
// text-entry caret. The toolkit's bitmap font has no blinking-caret glyph, so
// underscore stands in.
const paletteCaret = "_"

// NewCommandPalette builds a hidden CommandPalette over the given commands.
// Query starts empty and Selected at 0; call Open to show it.
func NewCommandPalette(cmds []PaletteCommand) *CommandPalette {
	return &CommandPalette{Commands: cmds}
}

// Open shows the palette, clearing any prior query and selection so it always
// reopens in a fresh state.
func (c *CommandPalette) Open() {
	c.Visible = true
	c.Query = ""
	c.Selected = 0
}

// Dismiss hides the palette and resets its query + selection. It does NOT call
// OnDismiss itself: OnDismiss is a cancellation signal invoked only by the
// event handlers that dismiss on user intent (Escape / outside-click), mirroring
// how ContextMenu keeps activation and cancellation on separate paths.
func (c *CommandPalette) Dismiss() {
	c.Visible = false
	c.Query = ""
	c.Selected = 0
}

// filtered returns indices of Commands whose Label contains Query,
// case-insensitively (substring match, not fuzzy/subsequence). An empty Query
// matches everything.
func (c *CommandPalette) filtered() []int {
	q := strings.ToLower(c.Query)
	var out []int
	for i, cmd := range c.Commands {
		if q == "" || strings.Contains(strings.ToLower(cmd.Label), q) {
			out = append(out, i)
		}
	}
	return out
}

// clampSelected keeps Selected inside [0, len(filtered())-1]. On an empty
// filtered list Selected pins to 0. Called after every Query mutation so a
// shrinking list never leaves Selected dangling past the end.
func (c *CommandPalette) clampSelected() {
	n := len(c.filtered())
	if n == 0 {
		c.Selected = 0
		return
	}
	if c.Selected < 0 {
		c.Selected = 0
	}
	if c.Selected >= n {
		c.Selected = n - 1
	}
}

// panelBounds measures the panel and centers it inside the surface (Bounds()).
// Width is the widest visible label (query + caret and every filtered label)
// plus horizontal padding, floored at PaletteMinW. Height is one query row plus
// one row per filtered command, plus the top+bottom vertical inset.
func (c *CommandPalette) panelBounds() Rect {
	rows := c.filtered()
	w := PaletteMinW
	if lw := c.textWidth(c.Query+paletteCaret) + 2*PalettePadX; lw > w {
		w = lw
	}
	for _, i := range rows {
		if lw := c.textWidth(c.Commands[i].Label) + 2*PalettePadX; lw > w {
			w = lw
		}
	}
	h := (1+len(rows))*PaletteRowH + 2*palettePadY
	surf := c.Bounds()
	x := surf.X + (surf.W-w)/2
	y := surf.Y + (surf.H-h)/2
	return Rect{X: x, Y: y, W: w, H: h}
}

// Draw paints the centered panel when Visible: a query row (the current Query
// plus a trailing caret marker) followed by one row per filtered command, with
// the Selected filtered row highlighted in Theme.Accent. Nothing is drawn when
// hidden. An empty filtered list still renders the panel with just the query
// row.
func (c *CommandPalette) Draw(p painter.Painter, theme *Theme) {
	if !c.Visible {
		return
	}
	pb := c.panelBounds()
	fillRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Surface)
	strokeRect(p, pb.X, pb.Y, pb.W, pb.H, theme.Border)

	// Query row.
	qy := pb.Y + palettePadY
	textOff := (PaletteRowH - c.glyphHeight()) / 2
	c.drawText(p, pb.X+PalettePadX, qy+textOff, c.Query+paletteCaret, theme.OnSurface)

	// Result rows.
	for row, i := range c.filtered() {
		ry := pb.Y + palettePadY + (row+1)*PaletteRowH
		ink := theme.OnSurface
		if row == c.Selected {
			fillRect(p, pb.X+1, ry, pb.W-2, PaletteRowH, theme.Accent)
			ink = theme.Background
		}
		c.drawText(p, pb.X+PalettePadX, ry+textOff, c.Commands[i].Label, ink)
	}
}

// OnEvent drives the palette while Visible: EventChar appends to Query,
// Backspace trims it, ArrowUp/ArrowDown move Selected within the filtered list
// (clamped, no wraparound — matching ListBox/ContextMenu), Enter/row-click runs
// the selected command then dismisses, and Escape / outside-click dismisses and
// fires OnDismiss. Events while hidden are ignored.
func (c *CommandPalette) OnEvent(ev Event) {
	if !c.Visible {
		return
	}
	switch ev.Kind {
	case EventChar:
		if ev.Code == "" {
			return
		}
		c.Query += ev.Code
		c.clampSelected()
	case EventKeyDown:
		c.onKey(ev.Code)
	case EventClick:
		c.onClick(ev)
	}
}

// onKey handles the keyboard commands routed from OnEvent.
func (c *CommandPalette) onKey(code string) {
	switch code {
	case "Backspace":
		runes := []rune(c.Query)
		if len(runes) > 0 {
			c.Query = string(runes[:len(runes)-1])
			c.clampSelected()
		}
	case "ArrowDown":
		if n := len(c.filtered()); c.Selected < n-1 {
			c.Selected++
		}
	case "ArrowUp":
		if c.Selected > 0 {
			c.Selected--
		}
	case "Enter":
		c.activate()
	case "Escape":
		c.Dismiss()
		if c.OnDismiss != nil {
			c.OnDismiss()
		}
	}
}

// onClick routes a click: onto a result row it selects + runs that command;
// anywhere outside the panel it dismisses (firing OnDismiss). A click inside the
// panel but not on a result row (the query row or padding) is ignored.
func (c *CommandPalette) onClick(ev Event) {
	pb := c.panelBounds()
	inside := ev.X >= pb.X && ev.X < pb.X+pb.W && ev.Y >= pb.Y && ev.Y < pb.Y+pb.H
	if !inside {
		c.Dismiss()
		if c.OnDismiss != nil {
			c.OnDismiss()
		}
		return
	}
	// Row 0 is the query row; result rows start at row 1.
	rel := ev.Y - (pb.Y + palettePadY)
	if rel < PaletteRowH {
		return // query row / above the first result
	}
	// rel >= PaletteRowH here (the query row was handled above), so row >= 0.
	row := rel/PaletteRowH - 1
	if row >= len(c.filtered()) {
		return
	}
	c.Selected = row
	c.activate()
}

// activate runs the currently selected command's Action (if any) and then
// dismisses the palette. It is safe on an empty filtered list and on a nil
// Action: both simply dismiss without running anything.
func (c *CommandPalette) activate() {
	f := c.filtered()
	if c.Selected < len(f) {
		if action := c.Commands[f[c.Selected]].Action; action != nil {
			action()
		}
	}
	c.Dismiss()
}

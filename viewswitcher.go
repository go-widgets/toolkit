// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ViewSwitcher is a libadwaita/GTK-style horizontal segmented tab
// picker: an evenly-divided strip of same-width segments where
// exactly one is highlighted as the active view. Clicking a segment
// swaps Current and fires OnChange with the new index.
//
// The strip's background is Theme.SurfaceAlt; the active segment
// paints in Theme.Accent with the accent-inverted ink
// (theme.Extra["OnAccent"] with a Theme.Background fallback, matching
// what Button, ListBox, TreeView, and Table already do). A 1-pixel
// Theme.Border line sits along the strip's bottom edge so the
// switcher reads as a discrete band above the switched content.
//
// A ViewSwitcher with no Views paints only the background + bottom
// border; clicks are ignored. This lets a caller assemble the
// widget before it knows which views the app will surface without
// tripping a nil-Views guard downstream.
type ViewSwitcher struct {
	Base
	Views    []string
	Current  int
	OnChange func(i int)
}

// Sizing constants for the strip's default vertical extent and its
// horizontal end padding. ViewSwitcher's Draw does not require
// Bounds.H == ViewSwitcherH; the constant is exposed so callers
// building a HeaderBar-like layout can allocate a matching strip.
const (
	// ViewSwitcherH is the default vertical extent in pixels.
	ViewSwitcherH = 32
	// ViewSwitcherPadX is the horizontal padding at the strip's left
	// and right edges. Reserved for future asymmetric layouts; the
	// current segment layout divides the full width evenly.
	ViewSwitcherPadX = 12
)

// NewViewSwitcher constructs a ViewSwitcher over views with the
// initial highlighted segment at current. current is clamped into
// the [0, len(views)-1] range, or forced to 0 when views is empty,
// so the widget is never in a hard-to-reason "index out of range"
// state.
func NewViewSwitcher(views []string, current int) *ViewSwitcher {
	n := len(views)
	if n == 0 {
		current = 0
	} else if current < 0 {
		current = 0
	} else if current >= n {
		current = n - 1
	}
	return &ViewSwitcher{Views: views, Current: current}
}

// Draw paints the strip background, then each segment with the
// active one highlighted in Theme.Accent, then the 1-pixel bottom
// border. Segments share the same width via integer division of
// Bounds.W by len(Views); any left-over pixel column on the right
// remains SurfaceAlt (this matches how HeaderBar's title strip
// tolerates non-integer central strips).
func (v *ViewSwitcher) Draw(p painter.Painter, theme *Theme) {
	r := v.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	n := len(v.Views)
	if n > 0 {
		segW := r.W / n
		for i, title := range v.Views {
			sx := r.X + i*segW
			ink := theme.OnSurface
			if i == v.Current {
				fillRect(p, sx, r.Y, segW, r.H, theme.Accent)
				ink = accentInk(theme)
			}
			tw := TextWidth(title)
			tx := sx + (segW-tw)/2
			ty := r.Y + (r.H-GlyphHeight)/2
			DrawText(p, tx, ty, title, ink)
		}
	}
	// Bottom border line.
	fillRect(p, r.X, r.Y+r.H-1, r.W, 1, theme.Border)
}

// OnEvent handles a click by locating which segment the X coordinate
// lands on and updating Current + firing OnChange. Non-click events,
// clicks with an empty Views slice, clicks on a zero-width strip and
// clicks that fall outside every segment are all no-ops.
func (v *ViewSwitcher) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	n := len(v.Views)
	if n == 0 {
		return
	}
	r := v.Bounds()
	segW := r.W / n
	if segW <= 0 {
		return
	}
	idx := ev.X / segW
	if idx < 0 || idx >= n {
		return
	}
	v.Current = idx
	if v.OnChange != nil {
		v.OnChange(idx)
	}
}

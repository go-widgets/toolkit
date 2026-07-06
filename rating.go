// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Rating is a horizontal star-rating strip: Max square cells drawn
// left-to-right, each carrying an ASCII asterisk overlay. Cells with
// index < Value fill in Theme.Accent (the "filled" state); cells with
// index >= Value fill in Theme.SurfaceAlt (the "empty" state). The
// star glyph itself is drawn as the ASCII "*" character because the
// toolkit's 5x7 bitmap font only covers ASCII — a Unicode "★" would
// render blank via DrawText's font5x7 lookup fall-through.
//
// A click on cell index i sets Value = i+1 (so the leftmost cell
// yields 1, the rightmost Max) and fires OnChange when non-nil.
// Clicks outside the strip (Y outside the cell row, X to the right of
// the last cell) are ignored — the parent container already routes only
// hits inside Bounds() but a stray x >= Max*(RatingStarW+RatingStarGap)
// would otherwise resolve to an out-of-range index.
type Rating struct {
	Base
	Value    int
	Max      int
	OnChange func(v int)
}

// Rating sizing constants. Cells are square so the strip reads as a
// row of tiles; the small gap keeps them visually distinct without
// eating layout width.
const (
	// RatingStarW is the per-cell edge in pixels.
	RatingStarW = 14
	// RatingStarGap is the horizontal spacing between two successive
	// cells (pixels of surface visible between them).
	RatingStarGap = 2
)

// NewRating constructs a Rating with the given value and max. Max
// defaults to 5 when non-positive; Value is clamped to the [0, Max]
// interval so a bogus caller input can never render more filled cells
// than Max.
func NewRating(value, max int) *Rating {
	if max <= 0 {
		max = 5
	}
	if value < 0 {
		value = 0
	}
	if value > max {
		value = max
	}
	return &Rating{Value: value, Max: max}
}

// Draw paints Max cells left-to-right. Filled cells use Theme.Accent +
// the accent-inverted ink; empty cells use Theme.SurfaceAlt +
// Theme.OnSurface. Every cell carries an ASCII "*" overlay so the row
// reads as stars even when the palette is monochrome.
func (r *Rating) Draw(p painter.Painter, theme *Theme) {
	b := r.Bounds()
	ink := accentInk(theme)
	for i := 0; i < r.Max; i++ {
		x := b.X + i*(RatingStarW+RatingStarGap)
		fill := theme.SurfaceAlt
		glyphInk := theme.OnSurface
		if i < r.Value {
			fill = theme.Accent
			glyphInk = ink
		}
		fillRect(p, x, b.Y, RatingStarW, RatingStarW, fill)
		tw := TextWidth("*")
		tx := x + (RatingStarW-tw)/2
		ty := b.Y + (RatingStarW-GlyphHeight)/2
		DrawText(p, tx, ty, "*", glyphInk)
	}
}

// OnEvent handles a click by resolving the star index from ev.X and
// setting Value = index+1. Non-click events are ignored (matches
// Switch / ToggleButton). Clicks with X to the right of the last cell
// (index >= Max) are ignored so a spurious hit doesn't push Value
// past Max.
func (r *Rating) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	idx := ev.X / (RatingStarW + RatingStarGap)
	if idx < 0 || idx >= r.Max {
		return
	}
	r.Value = idx + 1
	if r.OnChange != nil {
		r.OnChange(r.Value)
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Rating is a horizontal star-rating strip: Max square cells drawn
// left-to-right, each carrying an ASCII asterisk overlay. Cells with
// index < Value fill in Theme.Accent (the "filled" state); cells with
// index >= Value fill in Theme.SurfaceAlt (the "empty" state). The
// star glyph itself is drawn as the ASCII "*" character because the
// toolkit's 5x7 bitmap font only covers ASCII — a Unicode "★" would
// render blank via DrawText's font5x7 lookup fall-through.
//
// A click on cell index i sets Value to i+1 (so the leftmost cell
// yields 1, the rightmost Max), notifying the Value Observable's subscribers.
// Clicks outside the strip (Y outside the cell row, X to the right of
// the last cell) are ignored — the parent container already routes only
// hits inside Bounds() but a stray x >= Max*(RatingStarW+RatingStarGap)
// would otherwise resolve to an out-of-range index.
type Rating struct {
	Base
	focusState
	// Max is the number of cells (config). The reactive rating is MVVM-only: the
	// current value lives in an unexported Observable exposed via [Rating.Value].
	Max int

	value *mvvm.Observable[int]
}

// Value is the current rating as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Value field. A click or a
// key adjustment Sets it (clamped to [0, Max]); subscribers are notified.
func (r *Rating) Value() *mvvm.Observable[int] {
	if r.value == nil {
		r.value = mvvm.NewObservable(0)
	}
	return r.value
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
	r := &Rating{Max: max}
	r.value = mvvm.NewObservable(value)
	return r
}

// Draw paints Max cells left-to-right. Filled cells use Theme.Accent +
// the accent-inverted ink; empty cells use Theme.SurfaceAlt +
// Theme.OnSurface. Every cell carries an ASCII "*" overlay so the row
// reads as stars even when the palette is monochrome.
func (r *Rating) Draw(p painter.Painter, theme *Theme) {
	b := r.Bounds()
	ink := accentInk(theme)
	// Cell edge and gap route through scaled so the strip grows with HiDPI and
	// touch density; each equals its constant at compact/1x (byte-identical). Draw
	// and OnEvent both derive the cell pitch from these, so the drawn cells and the
	// click-to-index mapping can never drift.
	starW, pitch := scaled(RatingStarW), scaled(RatingStarW)+scaled(RatingStarGap)
	for i := 0; i < r.Max; i++ {
		x := b.X + i*pitch
		fill := theme.SurfaceAlt
		glyphInk := theme.OnSurface
		if i < r.Value().Get() {
			fill = theme.Accent
			glyphInk = ink
		}
		fillRect(p, x, b.Y, starW, starW, fill)
		tw := r.textWidth("*")
		tx := x + (starW-tw)/2
		ty := b.Y + (starW-r.glyphHeight())/2
		r.drawText(p, tx, ty, "*", glyphInk)
	}
	r.drawFocusRing(p, theme, b)
}

// OnEvent handles a click by resolving the star index from ev.X and
// setting Value = index+1. Non-click events are ignored (matches
// Switch / ToggleButton). Clicks with X to the right of the last cell
// (index >= Max) are ignored so a spurious hit doesn't push Value
// past Max.
func (r *Rating) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown {
		if r.Disabled {
			return
		}
		// Left/Right adjust the rating by one star; Home clears it (0), End fills
		// it (Max). Each reuses the same clamp path as a click.
		switch ev.Code {
		case "ArrowRight", "ArrowUp":
			r.setValue(r.Value().Get() + 1)
		case "ArrowLeft", "ArrowDown":
			r.setValue(r.Value().Get() - 1)
		case "Home":
			r.setValue(0)
		case "End":
			r.setValue(r.Max)
		}
		return
	}
	if ev.Kind != EventClick {
		return
	}
	idx := ev.X / (scaled(RatingStarW) + scaled(RatingStarGap))
	if idx < 0 || idx >= r.Max {
		return
	}
	r.setValue(idx + 1)
}

// HitRect is the rating strip's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]). A
// star row is only ~14 logical pixels tall, so under DensityTouch its hit height
// grows to the >=44px finger floor for a comfortable vertical reach while the
// drawn stars are untouched; byte-identical to Bounds under DensityCompact. The
// per-cell index still derives from the drawn cell pitch, so which star a press
// selects is unaffected by the clamp.
func (r *Rating) HitRect() Rect { return touchHitRect(r.Bounds()) }

// HitTest reports whether a surface point falls on the rating strip's
// (touch-clamped) hit rect.
func (r *Rating) HitTest(px, py int) bool { return r.HitRect().Contains(px, py) }

// setValue clamps v to [0, Max] and Sets the Value Observable — the shared
// mutate path for a click and every key adjustment. Subscribers are notified on
// change (an unchanged value is a no-op, per mvvm.Observable).
func (r *Rating) setValue(v int) {
	if v < 0 {
		v = 0
	}
	if v > r.Max {
		v = r.Max
	}
	r.Value().Set(v)
}

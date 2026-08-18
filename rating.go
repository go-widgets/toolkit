// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Rating is a horizontal star-rating strip: Max cells drawn left-to-right, each
// carrying a real five-pointed star. Cells with index < Value paint the filled
// (gold) star; cells with index >= Value paint the empty (grey) star. The star
// is a proper concave 10-vertex polygon, anti-aliased through the painter's
// path-fill capability on pixel back-ends and scanline-filled on a cell grid, so
// the row reads as stars — not squares, not an ASCII "*".
//
// The two tones come from the [Theme] (StarFilled / StarEmpty) so a rating
// re-tints with the rest of the palette in light and dark; a host may override
// either per-widget via [Rating.FilledColor] / [Rating.EmptyColor].
//
// A click on cell index i sets Value to i+1 (so the leftmost cell yields 1, the
// rightmost Max), notifying the Value Observable's subscribers. Clicks outside
// the strip (X to the right of the last cell) are ignored — the parent container
// already routes only hits inside Bounds() but a stray x >= Max*pitch would
// otherwise resolve to an out-of-range index.
type Rating struct {
	Base
	focusState
	// Max is the number of cells (config). The reactive rating is MVVM-only: the
	// current value lives in an unexported Observable exposed via [Rating.Value].
	Max int

	// FilledColor / EmptyColor override the themed star tones for this widget
	// only. Each is honoured when opaque (A != 0); a zero value (the default)
	// inherits Theme.StarFilled / Theme.StarEmpty (or their fallbacks). They are
	// set-once appearance config, not reactive state.
	FilledColor RGBA
	EmptyColor  RGBA

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

// Rating sizing constants. Each star occupies a square cell so the strip reads
// as an evenly spaced row; the small gap keeps successive stars visually
// distinct without eating layout width.
const (
	// RatingStarW is the per-cell edge in pixels.
	RatingStarW = 14
	// RatingStarGap is the horizontal spacing between two successive
	// cells (pixels of surface visible between them).
	RatingStarGap = 2
	// RatingStarPad is the inset (logical px) between a cell's edge and the
	// star's outer points, so stars don't touch across the gap.
	RatingStarPad = 1
)

// NewRating constructs a Rating with the given value and max. Max
// defaults to 5 when non-positive; Value is clamped to the [0, Max]
// interval so a bogus caller input can never render more filled stars
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

// ratingStarInnerRatio is the inner/outer radius ratio of the star's concave
// vertices. 0.42 gives a slightly chunkier star than a true pentagram (~0.382),
// which reads more clearly at the ~14px cell size.
const ratingStarInnerRatio = 0.42

// defaultStarFilled is the built-in gold used when neither the widget nor the
// theme supplies a filled tone. It reads as "scored" on light and dark grounds
// alike; the empty fallback is the theme's own Border grey (see emptyColor).
var defaultStarFilled = RGB(0xF5, 0xB8, 0x00)

// filledColor resolves the scored-star tone: the per-widget FilledColor when
// opaque, else Theme.StarFilled when the theme sets it, else the built-in gold.
func (r *Rating) filledColor(theme *Theme) RGBA {
	if r.FilledColor.A != 0 {
		return r.FilledColor
	}
	if theme.StarFilled.A != 0 {
		return theme.StarFilled
	}
	return defaultStarFilled
}

// emptyColor resolves the un-scored-star tone: the per-widget EmptyColor when
// opaque, else Theme.StarEmpty when the theme sets it, else the theme's Border
// grey (present in every palette).
func (r *Rating) emptyColor(theme *Theme) RGBA {
	if r.EmptyColor.A != 0 {
		return r.EmptyColor
	}
	if theme.StarEmpty.A != 0 {
		return theme.StarEmpty
	}
	return theme.Border
}

// starPolygon returns the 10 vertices of a point-up five-pointed star inscribed
// in a circle of radius outer about (cx, cy), with the concave vertices at
// radius inner. Vertices alternate outer/inner starting at the top point (angle
// -90°) and step 36° clockwise, so tracing them yields the classic simple
// (non-self-intersecting) concave star; a non-zero OR even-odd fill paints it
// identically. Coordinates are float64 pixels for the anti-aliased path builder.
func starPolygon(cx, cy, outer, inner float64) [][2]float64 {
	pts := make([][2]float64, 10)
	for k := 0; k < 10; k++ {
		ang := -math.Pi/2 + float64(k)*math.Pi/5
		rad := outer
		if k%2 == 1 {
			rad = inner
		}
		pts[k] = [2]float64{cx + rad*math.Cos(ang), cy + rad*math.Sin(ang)}
	}
	return pts
}

// starPath builds a closed painter.Path from starPolygon's vertices for the
// anti-aliased FillPath / StrokePath route.
func starPath(cx, cy, outer, inner float64) *painter.Path {
	pts := starPolygon(cx, cy, outer, inner)
	path := painter.NewPath().MoveTo(pts[0][0], pts[0][1])
	for _, pt := range pts[1:] {
		path.LineTo(pt[0], pt[1])
	}
	return path.Close()
}

// starPointsInt rounds starPolygon's vertices to integer pixels for the
// cell-grid fallback (fillPolygon / drawPolygon operate on [][2]int).
func starPointsInt(cx, cy, outer, inner float64) [][2]int {
	fp := starPolygon(cx, cy, outer, inner)
	pts := make([][2]int, len(fp))
	for i, pt := range fp {
		pts[i] = [2]int{int(math.Round(pt[0])), int(math.Round(pt[1]))}
	}
	return pts
}

// drawStar paints one star centred at (cx, cy) with the given radii: fill under
// an optional thin outline. On a pixel back-end (PathPainter) it fills the star
// polygon anti-aliased; on a cell grid it falls back to the shared integer
// scanline fill + polygon outline. The outline is skipped when transparent
// (outline.A == 0) or when strokeW <= 0.
func drawStar(p painter.Painter, cx, cy, outer, inner float64, fill, outline RGBA, strokeW float64) {
	if pp, ok := p.(painter.PathPainter); ok {
		path := starPath(cx, cy, outer, inner)
		pp.FillPath(path, fill, painter.NonZero)
		if outline.A != 0 && strokeW > 0 {
			pp.StrokePath(path, outline, strokeW)
		}
		return
	}
	pts := starPointsInt(cx, cy, outer, inner)
	fillPolygon(p, pts, fill)
	if outline.A != 0 {
		drawPolygon(p, pts, outline)
	}
}

// Draw paints Max stars left-to-right. Stars at index < Value use the filled
// (gold) tone; the rest use the empty (grey) tone. A thin outline — a darkened
// blend of each star's own fill — defines the shape against the surface,
// especially the grey empty stars. The cell edge and pitch route through scaled
// so the strip grows with HiDPI and touch density; Draw and OnEvent derive the
// pitch from the same constants, so the drawn stars and the click-to-index
// mapping can never drift.
func (r *Rating) Draw(p painter.Painter, theme *Theme) {
	b := r.Bounds()
	starW, pitch := scaled(RatingStarW), scaled(RatingStarW)+scaled(RatingStarGap)
	outer := float64(starW)/2 - float64(scaled(RatingStarPad))
	inner := outer * ratingStarInnerRatio
	strokeW := float64(strokeWidth())
	filled, empty := r.filledColor(theme), r.emptyColor(theme)
	val := r.Value().Get()
	for i := 0; i < r.Max; i++ {
		x := b.X + i*pitch
		cx := float64(x) + float64(starW)/2
		cy := float64(b.Y) + float64(starW)/2
		fill := empty
		if i < val {
			fill = filled
		}
		// Outline = 75% fill + 25% black: a darker sibling of the tone itself,
		// so it stays themed (no raw RGBA) and defines grey stars on any ground.
		outline := blendRGBA(fill, RGB(0, 0, 0), 0.75)
		drawStar(p, cx, cy, outer, inner, fill, outline, strokeW)
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

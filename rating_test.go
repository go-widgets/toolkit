// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"

	"github.com/go-widgets/painter"
)

// TestRatingNewDefaultMax covers the max <= 0 -> 5 branch of NewRating.
func TestRatingNewDefaultMax(t *testing.T) {
	r := NewRating(2, 0)
	if r.Max != 5 {
		t.Fatalf("default Max = %d, want 5", r.Max)
	}
	r = NewRating(2, -3)
	if r.Max != 5 {
		t.Fatalf("negative Max = %d, want 5", r.Max)
	}
}

// TestRatingNewKeepsPositiveMax covers the positive-max half of the
// branch.
func TestRatingNewKeepsPositiveMax(t *testing.T) {
	r := NewRating(2, 7)
	if r.Max != 7 {
		t.Fatalf("Max = %d, want 7", r.Max)
	}
}

// TestRatingNewClampsValueNegative covers value < 0 -> 0.
func TestRatingNewClampsValueNegative(t *testing.T) {
	r := NewRating(-4, 5)
	if r.Value().Get() != 0 {
		t.Fatalf("negative Value clamped to %d, want 0", r.Value().Get())
	}
}

// TestRatingNewClampsValueAboveMax covers value > max -> max.
func TestRatingNewClampsValueAboveMax(t *testing.T) {
	r := NewRating(99, 5)
	if r.Value().Get() != 5 {
		t.Fatalf("Value clamped to %d, want 5", r.Value().Get())
	}
}

// TestRatingNewKeepsInRangeValue covers the in-range case (both
// clamp branches skipped).
func TestRatingNewKeepsInRangeValue(t *testing.T) {
	r := NewRating(3, 5)
	if r.Value().Get() != 3 {
		t.Fatalf("Value = %d, want 3 (unchanged)", r.Value().Get())
	}
}

// --- Star geometry -------------------------------------------------------

// cellCentre returns the drawn centre pixel of star cell i for a strip whose
// Bounds origin is (0,0) at the default (compact, 1x) scale, mirroring Draw's
// own arithmetic so a pixel probe lands on the star's interior.
func cellCentre(i int) (int, int) {
	pitch := RatingStarW + RatingStarGap
	return i*pitch + RatingStarW/2, RatingStarW / 2
}

// TestStarPolygonExactVertices pins the star geometry to exact coordinates for a
// known cell/size: a point-up five-pointed star, outer radius 6, inner radius
// 6*0.42, centred on (7,7) — the centre of cell 0 at the compact 14px cell. This
// is the control on the shape: a regression in the angle sweep, the inner ratio,
// or the top-point orientation moves these numbers.
func TestStarPolygonExactVertices(t *testing.T) {
	const cx, cy, outer = 7.0, 7.0, 6.0
	inner := outer * ratingStarInnerRatio // 2.52
	wantF := [10][2]float64{
		{7.000000, 1.000000},
		{8.481219, 4.961277},
		{12.706339, 5.145898},
		{9.396662, 7.778723},
		{10.526712, 11.854102},
		{7.000000, 9.520000},
		{3.473288, 11.854102},
		{4.603338, 7.778723},
		{1.293661, 5.145898},
		{5.518781, 4.961277},
	}
	got := starPolygon(cx, cy, outer, inner)
	if len(got) != 10 {
		t.Fatalf("starPolygon returned %d vertices, want 10", len(got))
	}
	for k := range wantF {
		if math.Abs(got[k][0]-wantF[k][0]) > 1e-6 || math.Abs(got[k][1]-wantF[k][1]) > 1e-6 {
			t.Errorf("vertex %d = %.6f, want %.6f", k, got[k], wantF[k])
		}
	}
	// The top point must sit straight above the centre (x == cx, y minimal).
	if got[0][0] != cx || got[0][1] != cy-outer {
		t.Errorf("top point = %.3f, want {%.3f, %.3f}", got[0], cx, cy-outer)
	}

	// The integer-rounded polygon the cell-grid fallback fills.
	wantI := [10][2]int{
		{7, 1}, {8, 5}, {13, 5}, {9, 8}, {11, 12},
		{7, 10}, {3, 12}, {5, 8}, {1, 5}, {6, 5},
	}
	gi := starPointsInt(cx, cy, outer, inner)
	for k := range wantI {
		if gi[k] != wantI[k] {
			t.Errorf("int vertex %d = %v, want %v", k, gi[k], wantI[k])
		}
	}
}

// TestStarPathClosed verifies starPath builds a non-empty closed path (10 line
// legs + a closing segment) that the pixel back-end can rasterise.
func TestStarPathClosed(t *testing.T) {
	p := starPath(7, 7, 6, 2.52)
	if p == nil {
		t.Fatal("starPath returned nil")
	}
	// Fill it into a buffer and confirm the centre is painted (non-empty area).
	surf := makeSurface(16, 16)
	pp := newP(surf, 16)
	pp.FillPath(p, RGB(0x11, 0x22, 0x33), painter.NonZero)
	if got := pixelAt(surf, 16, 7, 7); got != (RGB(0x11, 0x22, 0x33)) {
		t.Fatalf("star centre = %+v, want the fill colour (path enclosed no area?)", got)
	}
}

// --- Star rendering (pixel back-end, anti-aliased) -----------------------

// TestRatingDrawFilledCentreYellow probes the interior centre pixel of a filled
// star: it must be exactly the theme's gold StarFilled tone (an interior pixel
// has full path coverage, so AA does not dilute it).
func TestRatingDrawFilledCentreYellow(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(2, 5) // cells 0,1 filled; 2,3,4 empty
	surfW := 5*(RatingStarW+RatingStarGap) + 4
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)

	cx0, cy0 := cellCentre(0)
	if got := pixelAt(surf, surfW, cx0, cy0); got != theme.StarFilled {
		t.Fatalf("filled star centre = %+v, want StarFilled %+v", got, theme.StarFilled)
	}
	cx3, cy3 := cellCentre(3)
	if got := pixelAt(surf, surfW, cx3, cy3); got != theme.StarEmpty {
		t.Fatalf("empty star centre = %+v, want StarEmpty %+v", got, theme.StarEmpty)
	}
}

// TestRatingDrawAllEmpty covers the value-0 edge: every star centre is grey.
func TestRatingDrawAllEmpty(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(0, 5)
	surfW := 5 * (RatingStarW + RatingStarGap)
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	for i := 0; i < 5; i++ {
		cx, cy := cellCentre(i)
		if got := pixelAt(surf, surfW, cx, cy); got != theme.StarEmpty {
			t.Fatalf("cell %d centre = %+v, want StarEmpty %+v", i, got, theme.StarEmpty)
		}
	}
}

// TestRatingDrawAllFilled covers the value==Max edge: every star centre is gold.
func TestRatingDrawAllFilled(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(5, 5)
	surfW := 5 * (RatingStarW + RatingStarGap)
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	for i := 0; i < 5; i++ {
		cx, cy := cellCentre(i)
		if got := pixelAt(surf, surfW, cx, cy); got != theme.StarFilled {
			t.Fatalf("cell %d centre = %+v, want StarFilled %+v", i, got, theme.StarFilled)
		}
	}
}

// TestRatingDrawDarkTheme proves the stars re-tint with the palette: under the
// dark theme a filled star is the dark gold and an empty star the dark grey.
func TestRatingDrawDarkTheme(t *testing.T) {
	theme := DefaultDark()
	r := NewRating(1, 3)
	surfW := 3 * (RatingStarW + RatingStarGap)
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	cx0, cy0 := cellCentre(0)
	if got := pixelAt(surf, surfW, cx0, cy0); got != theme.StarFilled {
		t.Fatalf("dark filled centre = %+v, want StarFilled %+v", got, theme.StarFilled)
	}
	cx1, cy1 := cellCentre(1)
	if got := pixelAt(surf, surfW, cx1, cy1); got != theme.StarEmpty {
		t.Fatalf("dark empty centre = %+v, want StarEmpty %+v", got, theme.StarEmpty)
	}
}

// TestRatingDrawPerWidgetOverride covers the FilledColor/EmptyColor override
// branches: an opaque per-widget colour wins over the theme.
func TestRatingDrawPerWidgetOverride(t *testing.T) {
	theme := DefaultLight()
	fill := RGB(0x12, 0x34, 0x56)
	empty := RGB(0x65, 0x43, 0x21)
	r := NewRating(1, 2)
	r.FilledColor = fill
	r.EmptyColor = empty
	surfW := 2 * (RatingStarW + RatingStarGap)
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	cx0, cy0 := cellCentre(0)
	if got := pixelAt(surf, surfW, cx0, cy0); got != fill {
		t.Fatalf("override filled centre = %+v, want %+v", got, fill)
	}
	cx1, cy1 := cellCentre(1)
	if got := pixelAt(surf, surfW, cx1, cy1); got != empty {
		t.Fatalf("override empty centre = %+v, want %+v", got, empty)
	}
}

// TestRatingColorThemeFallback covers the "theme leaves the star fields zero"
// branch: filledColor falls back to the built-in gold, emptyColor to the theme's
// Border grey. A bare Theme (no StarFilled/StarEmpty) exercises both.
func TestRatingColorThemeFallback(t *testing.T) {
	theme := &Theme{Border: RGB(0x80, 0x82, 0x84), Accent: RGB(0, 0, 0)}
	r := NewRating(1, 2)
	if got := r.filledColor(theme); got != defaultStarFilled {
		t.Fatalf("filled fallback = %+v, want defaultStarFilled %+v", got, defaultStarFilled)
	}
	if got := r.emptyColor(theme); got != theme.Border {
		t.Fatalf("empty fallback = %+v, want Border %+v", got, theme.Border)
	}
	// And it renders that way.
	surfW := 2 * (RatingStarW + RatingStarGap)
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	surf := makeSurface(surfW, RatingStarW)
	r.Draw(newP(surf, surfW), theme)
	cx0, cy0 := cellCentre(0)
	if got := pixelAt(surf, surfW, cx0, cy0); got != defaultStarFilled {
		t.Fatalf("fallback filled centre = %+v, want %+v", got, defaultStarFilled)
	}
	cx1, cy1 := cellCentre(1)
	if got := pixelAt(surf, surfW, cx1, cy1); got != theme.Border {
		t.Fatalf("fallback empty centre = %+v, want Border %+v", got, theme.Border)
	}
}

// TestRatingColorThemeSetWins covers the middle branch: when the widget sets no
// override but the theme DOES carry StarFilled/StarEmpty, the theme values win
// over the built-in fallbacks.
func TestRatingColorThemeSetWins(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(0, 5)
	if got := r.filledColor(theme); got != theme.StarFilled {
		t.Fatalf("filledColor = %+v, want theme StarFilled %+v", got, theme.StarFilled)
	}
	if got := r.emptyColor(theme); got != theme.StarEmpty {
		t.Fatalf("emptyColor = %+v, want theme StarEmpty %+v", got, theme.StarEmpty)
	}
}

// --- Cell-grid fallback (non-PathPainter back-end) -----------------------

// nonPathPainter is a base-only Painter over an RGBA buffer: it implements the
// fixed primitive set but NOT PathPainter, so drawStar takes its integer
// scanline fallback (fillPolygon + drawPolygon). It lets a pixel probe verify
// the fallback fills the star too.
type nonPathPainter struct {
	buf  []byte
	w, h int
}

var _ painter.Painter = (*nonPathPainter)(nil)

func (np *nonPathPainter) FillRect(r painter.Rect, c RGBA) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			np.PutPixel(x, y, c)
		}
	}
}
func (np *nonPathPainter) StrokeRect(r painter.Rect, c RGBA, lineW int)               {}
func (np *nonPathPainter) FillRoundRect(r painter.Rect, radius int, c RGBA)           {}
func (np *nonPathPainter) StrokeRoundRect(r painter.Rect, radius int, c RGBA, lw int) {}
func (np *nonPathPainter) Text(x, y int, s string, ink RGBA)                          {}
func (np *nonPathPainter) Size() (int, int)                                           { return np.w, np.h }
func (np *nonPathPainter) PutPixel(x, y int, c RGBA) {
	if x < 0 || y < 0 || x >= np.w || y >= np.h {
		return
	}
	o := (y*np.w + x) * 4
	np.buf[o], np.buf[o+1], np.buf[o+2], np.buf[o+3] = c.R, c.G, c.B, c.A
}

// TestPixelPainterIsPathPainter documents the capability split the fallback
// hinges on: the pixel back-end rasterises paths, the base-only painter does not.
func TestPixelPainterIsPathPainter(t *testing.T) {
	if _, ok := any(newP(makeSurface(4, 4), 4)).(painter.PathPainter); !ok {
		t.Fatal("PixelPainter must implement PathPainter")
	}
	if _, ok := any(&nonPathPainter{}).(painter.PathPainter); ok {
		t.Fatal("nonPathPainter must NOT implement PathPainter")
	}
}

// TestRatingDrawFallbackFillsStar exercises the non-PathPainter branch of
// drawStar via a full Rating.Draw: the star centre still lands on the fill.
func TestRatingDrawFallbackFillsStar(t *testing.T) {
	theme := DefaultLight()
	r := NewRating(1, 2)
	surfW := 2 * (RatingStarW + RatingStarGap)
	np := &nonPathPainter{buf: makeSurface(surfW, RatingStarW), w: surfW, h: RatingStarW}
	r.SetBounds(Rect{X: 0, Y: 0, W: surfW, H: RatingStarW})
	r.Draw(np, theme)
	cx0, cy0 := cellCentre(0)
	if got := pixelAt(np.buf, surfW, cx0, cy0); got != theme.StarFilled {
		t.Fatalf("fallback filled centre = %+v, want StarFilled %+v", got, theme.StarFilled)
	}
	cx1, cy1 := cellCentre(1)
	if got := pixelAt(np.buf, surfW, cx1, cy1); got != theme.StarEmpty {
		t.Fatalf("fallback empty centre = %+v, want StarEmpty %+v", got, theme.StarEmpty)
	}
}

// TestDrawStarOutlineToggle drives drawStar directly to cover both outline
// branches (present / skipped) on both back-ends: a transparent outline (A==0)
// and a zero stroke width skip the StrokePath / drawPolygon call.
func TestDrawStarOutlineToggle(t *testing.T) {
	fill := RGB(0x22, 0x44, 0x66)
	// Pixel back-end, outline skipped (transparent).
	surf := makeSurface(16, 16)
	drawStar(newP(surf, 16), 8, 8, 6, 2.5, fill, RGBA{}, 1)
	if got := pixelAt(surf, 16, 8, 8); got != fill {
		t.Fatalf("no-outline pixel centre = %+v, want fill %+v", got, fill)
	}
	// Pixel back-end, outline skipped (zero width).
	surf2 := makeSurface(16, 16)
	drawStar(newP(surf2, 16), 8, 8, 6, 2.5, fill, RGB(0, 0, 0), 0)
	if got := pixelAt(surf2, 16, 8, 8); got != fill {
		t.Fatalf("zero-width pixel centre = %+v, want fill %+v", got, fill)
	}
	// Fallback back-end, outline skipped (transparent) — drawPolygon not called.
	np := &nonPathPainter{buf: makeSurface(16, 16), w: 16, h: 16}
	drawStar(np, 8, 8, 6, 2.5, fill, RGBA{}, 1)
	if got := pixelAt(np.buf, 16, 8, 8); got != fill {
		t.Fatalf("fallback no-outline centre = %+v, want fill %+v", got, fill)
	}
	// Fallback back-end, outline present — drawPolygon paints the rim.
	np2 := &nonPathPainter{buf: makeSurface(16, 16), w: 16, h: 16}
	outline := RGB(0x01, 0x02, 0x03)
	drawStar(np2, 8, 8, 6, 2.5, fill, outline, 1)
	if got := pixelAt(np2.buf, 16, 8, 8); got != fill {
		t.Fatalf("fallback outlined centre = %+v, want fill %+v", got, fill)
	}
	if got := pixelAt(np2.buf, 16, 8, 2); got != outline { // top point (8,2)
		t.Fatalf("fallback outline top = %+v, want outline %+v", got, outline)
	}
}

// --- Value / MVVM --------------------------------------------------------

// TestRatingValueObservable covers the zero-value lazy-init of the Value
// accessor and the host binding path: a Rating built as a bare struct (no
// NewRating) still yields a usable Observable, and Setting it from outside is
// reflected by the widget (there is no imperative Value field).
func TestRatingValueObservable(t *testing.T) {
	r := &Rating{Max: 5} // no NewRating → value Observable is nil until accessed
	if r.Value().Get() != 0 {
		t.Fatalf("zero-value Rating Value = %d, want 0", r.Value().Get())
	}
	seen := -1
	r.Value().Subscribe(func(v int) { seen = v })
	r.Value().Set(4) // a host drives the rating through the Observable
	if r.Value().Get() != 4 || seen != 4 {
		t.Fatalf("host Set: value=%d subscriber=%d, want 4/4", r.Value().Get(), seen)
	}
}

// TestRatingClickFillsToIndex verifies OnEvent turns a click at cell k
// into Value = k+1 and notifies the Value Observable.
func TestRatingClickFillsToIndex(t *testing.T) {
	got := -1
	r := NewRating(0, 5)
	r.Value().Subscribe(func(v int) { got = v })
	r.SetBounds(Rect{X: 0, Y: 0, W: 5 * (RatingStarW + RatingStarGap), H: RatingStarW})
	// Click at x = 2*(RatingStarW+RatingStarGap) + 3 -> cell index 2.
	r.OnEvent(Event{Kind: EventClick, X: 2*(RatingStarW+RatingStarGap) + 3, Y: RatingStarW / 2})
	if r.Value().Get() != 3 {
		t.Fatalf("after click on cell 2, Value = %d, want 3", r.Value().Get())
	}
	if got != 3 {
		t.Fatalf("Value subscriber got %d, want 3", got)
	}
}

// TestRatingClickFirstCell exercises the leftmost cell (index 0).
func TestRatingClickFirstCell(t *testing.T) {
	r := NewRating(4, 5)
	r.OnEvent(Event{Kind: EventClick, X: 3, Y: RatingStarW / 2})
	if r.Value().Get() != 1 {
		t.Fatalf("after click on cell 0, Value = %d, want 1", r.Value().Get())
	}
}

// TestRatingClickLastCell exercises the rightmost cell (index Max-1).
func TestRatingClickLastCell(t *testing.T) {
	r := NewRating(0, 5)
	r.OnEvent(Event{Kind: EventClick, X: 4*(RatingStarW+RatingStarGap) + 3, Y: RatingStarW / 2})
	if r.Value().Get() != 5 {
		t.Fatalf("after click on cell 4, Value = %d, want 5", r.Value().Get())
	}
}

// TestRatingClickOutsideStripIgnored covers the idx >= Max branch: a
// click to the right of the last cell must not change Value.
func TestRatingClickOutsideStripIgnored(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventClick, X: 6 * (RatingStarW + RatingStarGap), Y: 0})
	if r.Value().Get() != 2 {
		t.Fatalf("click past strip should be ignored: Value = %d, want 2", r.Value().Get())
	}
}

// TestRatingClickNegativeXIgnored covers the idx < 0 guard. A single-pixel
// negative X yields idx = 0 (0/pitch == 0) — so we need X <= -pitch to reach
// idx == -1 and exercise the guard.
func TestRatingClickNegativeXIgnored(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventClick, X: -(RatingStarW + RatingStarGap + 1), Y: 0})
	if r.Value().Get() != 2 {
		t.Fatalf("negative-X click should be ignored: Value = %d, want 2", r.Value().Get())
	}
}

// TestRatingIgnoresNonClick guards the early-return in OnEvent: any
// non-click, non-key event must leave Value unchanged.
func TestRatingIgnoresNonClick(t *testing.T) {
	r := NewRating(2, 5)
	r.OnEvent(Event{Kind: EventChar, Code: "a"})
	if r.Value().Get() != 2 {
		t.Fatalf("Char should not change Value: got %d, want 2", r.Value().Get())
	}
}

// TestRatingClickNoSubscriber checks a click still updates Value with no
// subscriber attached (no panic).
func TestRatingClickNoSubscriber(t *testing.T) {
	r := NewRating(0, 5)
	r.OnEvent(Event{Kind: EventClick, X: 3, Y: 0})
	if r.Value().Get() != 1 {
		t.Fatal("click must update Value even without OnChange callback")
	}
}

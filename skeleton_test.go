// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestSkeletonNewTextDefaultLines covers the default-lines branch of
// NewSkeleton: SkeletonText + non-positive lines -> 3.
func TestSkeletonNewTextDefaultLines(t *testing.T) {
	s := NewSkeleton(SkeletonText, 0)
	if s.Lines != 3 {
		t.Fatalf("default Lines = %d, want 3", s.Lines)
	}
	s = NewSkeleton(SkeletonText, -5)
	if s.Lines != 3 {
		t.Fatalf("negative Lines = %d, want 3", s.Lines)
	}
}

// TestSkeletonNewTextKeepsPositiveLines covers the "already-positive"
// half of the branch.
func TestSkeletonNewTextKeepsPositiveLines(t *testing.T) {
	s := NewSkeleton(SkeletonText, 5)
	if s.Lines != 5 {
		t.Fatalf("Lines = %d, want 5", s.Lines)
	}
}

// TestSkeletonNewNonTextIgnoresLines covers the "kind != SkeletonText"
// branch: the lines value is stored verbatim even when non-positive.
func TestSkeletonNewNonTextIgnoresLines(t *testing.T) {
	s := NewSkeleton(SkeletonAvatar, -1)
	if s.Lines != -1 {
		t.Fatalf("Avatar Lines = %d, want -1 (stored verbatim)", s.Lines)
	}
	s = NewSkeleton(SkeletonBlock, 0)
	if s.Lines != 0 {
		t.Fatalf("Block Lines = %d, want 0", s.Lines)
	}
}

// TestSkeletonDrawText verifies SkeletonText draws N bars in SurfaceAlt
// with the last one narrower.
func TestSkeletonDrawText(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonText, 3)
	const w = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: 80})
	surf := makeSurface(w, 80)
	s.Draw(newP(surf, w), theme)

	// First bar covers full width. Sample at x = w-1, y in bar 0.
	if got := pixelAt(surf, w, w-1, 3); got != theme.SurfaceAlt {
		t.Fatalf("bar 0 right edge = %+v, want SurfaceAlt", got)
	}
	// Last bar is 60% width -> a pixel at x = w-1 in the last bar's
	// row must NOT be SurfaceAlt (it's outside the shortened bar).
	lastY := 2*(SkeletonLineH+SkeletonLineGap) + SkeletonLineH/2
	if got := pixelAt(surf, w, w-1, lastY); got == theme.SurfaceAlt {
		t.Fatal("last bar should be 60% width -- right edge must be untouched")
	}
	// But its left side should still be filled.
	if got := pixelAt(surf, w, 2, lastY); got != theme.SurfaceAlt {
		t.Fatalf("last bar left edge = %+v, want SurfaceAlt", got)
	}
}

// TestSkeletonDrawAvatar verifies SkeletonAvatar draws the three-band
// pill in SurfaceAlt.
func TestSkeletonDrawAvatar(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonAvatar, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: AvatarSize, H: AvatarSize})
	surf := makeSurface(AvatarSize, AvatarSize)
	s.Draw(newP(surf, AvatarSize), theme)
	if got := pixelAt(surf, AvatarSize, 2, 0); got != theme.SurfaceAlt {
		t.Fatalf("avatar top row = %+v, want SurfaceAlt", got)
	}
	// Corner clipped.
	if pixelAt(surf, AvatarSize, 0, 0) == theme.SurfaceAlt {
		t.Fatal("top-left corner should be clipped")
	}
}

// TestSkeletonDrawBlock verifies SkeletonBlock fills the inset rect in
// SurfaceAlt.
func TestSkeletonDrawBlock(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonBlock, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	// Centre pixel filled.
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("block centre = %+v, want SurfaceAlt", got)
	}
	// Padded corner NOT filled (SkeletonLinePad = 4 -> (0,0) is outside).
	if got := pixelAt(surf, 40, 0, 0); got == theme.SurfaceAlt {
		t.Fatal("block corner should stay in sentinel colour (inset by SkeletonLinePad)")
	}
}

// TestSkeletonDrawTextZeroLines covers Lines <= 0 on a text skeleton
// created via the struct literal (bypassing NewSkeleton's default).
// The Draw loop must simply skip.
func TestSkeletonDrawTextZeroLines(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonText, Lines: 0}
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	// No fill happened -- every pixel is still the sentinel.
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	if got := pixelAt(surf, 40, 5, 5); got != sentinel {
		t.Fatalf("expected sentinel at (5,5), got %+v", got)
	}
}

// TestSkeletonDrawTextSingleLine covers Lines == 1: the sole bar IS the
// last bar, so it renders at 60% width.
func TestSkeletonDrawTextSingleLine(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonText, 1)
	const w = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: 20})
	surf := makeSurface(w, 20)
	s.Draw(newP(surf, w), theme)
	// The single bar is 60% wide -- right edge must be untouched.
	if got := pixelAt(surf, w, w-1, SkeletonLineH/2); got == theme.SurfaceAlt {
		t.Fatal("single-bar skeleton should render at 60% width")
	}
	// Left side is filled.
	if got := pixelAt(surf, w, 2, SkeletonLineH/2); got != theme.SurfaceAlt {
		t.Fatalf("single-bar left = %+v, want SurfaceAlt", got)
	}
}

// TestSkeletonDrawDarkTheme sanity-covers a second theme so the theme
// wiring isn't accidentally coupled to DefaultLight.
func TestSkeletonDrawDarkTheme(t *testing.T) {
	theme := DefaultDark()
	s := NewSkeleton(SkeletonBlock, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("dark block = %+v, want SurfaceAlt", got)
	}
}

// --- rounded Rect / Circle variants --------------------------------------

// brightness sums the RGB channels — a coarse "lighter vs darker" metric
// used by the shimmer tests.
func brightness(c RGBA) int { return int(c.R) + int(c.G) + int(c.B) }

// TestSkeletonDrawRect verifies SkeletonRect fills a rounded block in
// SurfaceAlt: the centre is filled, a corner is clipped by the default
// radius, and nothing lands outside Bounds.
func TestSkeletonDrawRect(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonRect, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("rect centre = %+v, want SurfaceAlt", got)
	}
	// Corner clipped by SkeletonRectRadius -> not fully filled.
	if got := pixelAt(surf, 40, 0, 0); got == theme.SurfaceAlt {
		t.Fatal("rect corner should be rounded away (radius clip)")
	}
}

// TestSkeletonDrawRectCustomRadius covers rectRadius' Radius>0 branch.
func TestSkeletonDrawRectCustomRadius(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonRect, Radius: 12}
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	// A larger radius clips more: the pixel at (2,2) (inside the default-6
	// corner box but well inside a 12 box's arc) must be rounded away.
	if got := pixelAt(surf, 40, 1, 1); got == theme.SurfaceAlt {
		t.Fatal("rect(radius=12) corner should be rounded away")
	}
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("rect centre = %+v, want SurfaceAlt", got)
	}
}

// TestSkeletonDrawCircleSquare verifies a square-bounds SkeletonCircle
// fills its centre and clips its corners (H<d false branch: H==W).
func TestSkeletonDrawCircleSquare(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonCircle, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	surf := makeSurface(40, 40)
	s.Draw(newP(surf, 40), theme)
	if got := pixelAt(surf, 40, 20, 20); got != theme.SurfaceAlt {
		t.Fatalf("circle centre = %+v, want SurfaceAlt", got)
	}
	// Corner is well outside the inscribed circle -> untouched sentinel.
	if got := pixelAt(surf, 40, 0, 0); got == theme.SurfaceAlt {
		t.Fatal("circle corner must be outside the disc")
	}
}

// TestSkeletonDrawCircleWide verifies a wide (W>H) SkeletonCircle
// inscribes a disc of diameter H and CENTRES it horizontally (covers the
// H<d true branch + the (r.W-d)/2 offset).
func TestSkeletonDrawCircleWide(t *testing.T) {
	theme := DefaultLight()
	s := NewSkeleton(SkeletonCircle, 0)
	s.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	surf := makeSurface(60, 40)
	s.Draw(newP(surf, 60), theme)
	// Disc diameter 40, centred at x offset (60-40)/2 = 10 -> disc centre
	// at (30,20).
	if got := pixelAt(surf, 60, 30, 20); got != theme.SurfaceAlt {
		t.Fatalf("wide-circle centre = %+v, want SurfaceAlt", got)
	}
	// x=2 is left of the disc (which starts at x=10) -> untouched.
	if got := pixelAt(surf, 60, 2, 20); got == theme.SurfaceAlt {
		t.Fatal("region left of a centred disc must stay unfilled")
	}
}

// TestSkeletonTextCustomConfig covers effLineH/effLineGap/effLastFrac's
// override branches + textBarRadius' Radius>0 branch, asserting bars land
// at the tuned y/heights/widths.
func TestSkeletonTextCustomConfig(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonText, Lines: 2, LineH: 20, LineGap: 10, LastFrac: 0.5, Radius: 4}
	const w = 100
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: 60})
	surf := makeSurface(w, 60)
	s.Draw(newP(surf, w), theme)
	// Bar 0 interior (full width): y in [0,20).
	if got := pixelAt(surf, w, 50, 10); got != theme.SurfaceAlt {
		t.Fatalf("bar0 interior = %+v, want SurfaceAlt", got)
	}
	// Bar 1 starts at y = 20 + 10 = 30, height 20, width = 50 (LastFrac).
	if got := pixelAt(surf, w, 25, 40); got != theme.SurfaceAlt {
		t.Fatalf("bar1 interior = %+v, want SurfaceAlt", got)
	}
	// Beyond the 50% last-bar width -> untouched.
	if got := pixelAt(surf, w, 60, 40); got == theme.SurfaceAlt {
		t.Fatal("last bar should stop at 50% width")
	}
	// Radius=4 clips bar0's top-left corner.
	if got := pixelAt(surf, w, 0, 0); got == theme.SurfaceAlt {
		t.Fatal("text bar corner should be rounded (Radius=4)")
	}
}

// --- shimmer -------------------------------------------------------------

// TestSkeletonShimmerSweep is the core shimmer assertion: a pixel is flat
// base grey at phase 0, LIGHTER once the band centre sweeps over it, and
// the band never paints outside Bounds.
func TestSkeletonShimmerSweep(t *testing.T) {
	theme := DefaultLight()
	base := theme.SurfaceAlt
	area := Rect{X: 10, Y: 10, W: 80, H: 40}
	const sw, sh = 120, 80

	// The lit pixel: local (41,20). Compute the phase that centres the
	// band on it, mirroring skeleton.go's own math so the test tracks the
	// implementation.
	xx, yy := 41, 20
	skew := skeletonShimmerSkew * float64(area.H-1-yy)
	u := float64(xx) + skew
	bandW := int(float64(area.W) * skeletonBandFrac)
	span := float64(area.W) + skeletonShimmerSkew*float64(area.H)
	phase := (u + float64(bandW)) / (span + 2*float64(bandW))
	px, py := area.X+xx, area.Y+yy

	// Phase 0: band parked off the leading edge -> flat base everywhere.
	s := &Skeleton{Kind: SkeletonRect}
	s.SetBounds(area)
	s.SetPhase(0)
	surf0 := makeSurface(sw, sh)
	s.Draw(newP(surf0, sw), theme)
	if got := pixelAt(surf0, sw, px, py); got != base {
		t.Fatalf("phase 0: lit pixel = %+v, want flat base %+v", got, base)
	}

	// Swept phase: the pixel is now lighter than base.
	s.SetPhase(phase)
	surf1 := makeSurface(sw, sh)
	s.Draw(newP(surf1, sw), theme)
	lit := pixelAt(surf1, sw, px, py)
	if brightness(lit) <= brightness(base) {
		t.Fatalf("swept: pixel %+v not lighter than base %+v", lit, base)
	}

	// Band stays inside Bounds: pixels outside the widget rect are still
	// the sentinel on the swept frame.
	for _, pt := range [][2]int{{5, 5}, {area.X + area.W, area.Y + 20}, {area.X + 20, area.Y - 1}, {115, 75}} {
		if got := pixelAt(surf1, sw, pt[0], pt[1]); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
			t.Fatalf("shimmer escaped bounds at (%d,%d): %+v", pt[0], pt[1], got)
		}
	}
}

// TestSkeletonShimmerThemeContrast asserts the band lifts the base in
// BOTH themes and that the two themes' lit pixels differ.
func TestSkeletonShimmerThemeContrast(t *testing.T) {
	area := Rect{X: 0, Y: 0, W: 80, H: 40}
	const sw, sh = 80, 40
	xx, yy := 41, 20
	skew := skeletonShimmerSkew * float64(area.H-1-yy)
	u := float64(xx) + skew
	bandW := int(float64(area.W) * skeletonBandFrac)
	span := float64(area.W) + skeletonShimmerSkew*float64(area.H)
	phase := (u + float64(bandW)) / (span + 2*float64(bandW))

	draw := func(theme *Theme) (base, lit RGBA) {
		s := &Skeleton{Kind: SkeletonRect}
		s.SetBounds(area)
		s.SetPhase(phase)
		surf := makeSurface(sw, sh)
		s.Draw(newP(surf, sw), theme)
		return theme.SurfaceAlt, pixelAt(surf, sw, xx, yy)
	}
	lb, ll := draw(DefaultLight())
	db, dl := draw(DefaultDark())
	if brightness(ll) <= brightness(lb) {
		t.Fatalf("light: lit %+v not lighter than base %+v", ll, lb)
	}
	if brightness(dl) <= brightness(db) {
		t.Fatalf("dark: lit %+v not lighter than base %+v", dl, db)
	}
	if ll == dl {
		t.Fatal("light and dark lit pixels must differ")
	}
}

// TestSkeletonShimmerTinyWidth covers the bandW<1 -> bandW=1 clamp on a
// 2px-wide animated skeleton (still paints without panicking).
func TestSkeletonShimmerTinyWidth(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonRect}
	s.SetBounds(Rect{X: 0, Y: 0, W: 2, H: 10})
	s.SetPhase(0.5)
	surf := makeSurface(2, 10)
	s.Draw(newP(surf, 2), theme) // must not panic; bandW clamps to 1
}

// TestSkeletonShimmerDegenerateArea covers shimmer's area.W<=0 guard: an
// animated skeleton with a zero-width bound draws nothing and returns.
func TestSkeletonShimmerDegenerateArea(t *testing.T) {
	theme := DefaultLight()
	s := &Skeleton{Kind: SkeletonRect}
	s.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	s.SetPhase(0.5)
	surf := makeSurface(4, 10)
	s.Draw(newP(surf, 4), theme)
	// Nothing painted -> every pixel still the sentinel.
	if got := pixelAt(surf, 4, 1, 5); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatalf("zero-width animated skeleton painted %+v", got)
	}
}

// TestSkeletonSetPhaseWrap covers SetPhase's wrap into [0,1) for both
// over- and under-flow.
func TestSkeletonSetPhaseWrap(t *testing.T) {
	s := &Skeleton{}
	s.SetPhase(1.75)
	if s.Phase < 0.749 || s.Phase > 0.751 {
		t.Fatalf("SetPhase(1.75) Phase = %v, want ~0.75", s.Phase)
	}
	if !s.Animated {
		t.Fatal("SetPhase should switch Animated on")
	}
	s.SetPhase(-0.25)
	if s.Phase < 0.749 || s.Phase > 0.751 {
		t.Fatalf("SetPhase(-0.25) Phase = %v, want ~0.75", s.Phase)
	}
}

// --- SkeletonGroup + presets ---------------------------------------------

// TestSkeletonGroupDrawPositions verifies a group translates each child
// into surface coordinates (Bounds origin + local rect) and forwards its
// shimmer state.
func TestSkeletonGroupDrawPositions(t *testing.T) {
	theme := DefaultLight()
	g := &SkeletonGroup{}
	g.SetBounds(Rect{X: 10, Y: 10, W: 100, H: 100})
	g.Add(NewSkeleton(SkeletonRect, 0), Rect{X: 5, Y: 5, W: 20, H: 20})
	if len(g.Items()) != 1 {
		t.Fatalf("Items() len = %d, want 1", len(g.Items()))
	}
	if got := g.Items()[0].Local; got != (Rect{X: 5, Y: 5, W: 20, H: 20}) {
		t.Fatalf("item local = %+v", got)
	}
	g.SetPhase(0.5)
	surf := makeSurface(120, 120)
	g.Draw(newP(surf, 120), theme)
	// Child was positioned at (15,15,20,20); its centre (25,25) is filled.
	if got := pixelAt(surf, 120, 25, 25); got != theme.SurfaceAlt && brightness(got) < brightness(theme.SurfaceAlt) {
		t.Fatalf("child centre = %+v, want base-or-lighter", got)
	}
	child := g.Items()[0].Skel
	if !child.Animated || child.Phase != 0.5 {
		t.Fatalf("child shimmer not forwarded: Animated=%v Phase=%v", child.Animated, child.Phase)
	}
	if got := child.Bounds(); got != (Rect{X: 15, Y: 15, W: 20, H: 20}) {
		t.Fatalf("child bounds = %+v, want translated {15,15,20,20}", got)
	}
}

// TestSkeletonGroupSetPhaseWrap covers the group SetPhase wrap.
func TestSkeletonGroupSetPhaseWrap(t *testing.T) {
	g := &SkeletonGroup{}
	g.SetPhase(2.25)
	if g.Phase < 0.249 || g.Phase > 0.251 {
		t.Fatalf("group SetPhase(2.25) = %v, want ~0.25", g.Phase)
	}
	if !g.Animated {
		t.Fatal("group SetPhase should switch Animated on")
	}
}

// TestSkeletonGroupA11y covers the presentation-role accessor.
func TestSkeletonGroupA11y(t *testing.T) {
	if got := (&SkeletonGroup{}).A11y().Role; got != RolePresentation {
		t.Fatalf("SkeletonGroup role = %v, want RolePresentation", got)
	}
}

// TestNewSkeletonCard asserts the card preset's sub-element rects land at
// the computed positions and all sit within the card bounds.
func TestNewSkeletonCard(t *testing.T) {
	b := Rect{X: 0, Y: 0, W: 200, H: 150}
	card := NewSkeletonCard(b)
	items := card.Items()
	if len(items) != 3 {
		t.Fatalf("card items = %d, want 3", len(items))
	}
	// Avatar circle.
	if got := items[0].Local; got != (Rect{X: 8, Y: 8, W: 40, H: 40}) {
		t.Fatalf("avatar local = %+v", got)
	}
	if items[0].Skel.Kind != SkeletonCircle {
		t.Fatalf("item0 kind = %v, want SkeletonCircle", items[0].Skel.Kind)
	}
	// Two-line header: x = 8+40+8 = 56, headerH = 2*10+6 = 26,
	// y = 8 + (40-26)/2 = 15, w = 200-56-8 = 136.
	if got := items[1].Local; got != (Rect{X: 56, Y: 15, W: 136, H: 26}) {
		t.Fatalf("header local = %+v", got)
	}
	if items[1].Skel.Kind != SkeletonText || items[1].Skel.Lines != 2 {
		t.Fatalf("header not a 2-line text skeleton: %+v", items[1].Skel)
	}
	// Media block below: by = 8+40+8 = 56, w = 184, h = 150-56-8 = 86.
	if got := items[2].Local; got != (Rect{X: 8, Y: 56, W: 184, H: 86}) {
		t.Fatalf("media local = %+v", got)
	}
	// Every child within card bounds.
	for i, it := range items {
		if it.Local.X < 0 || it.Local.Y < 0 ||
			it.Local.X+it.Local.W > b.W || it.Local.Y+it.Local.H > b.H {
			t.Fatalf("item %d %+v escapes card bounds %+v", i, it.Local, b)
		}
	}
	// Draw it animated for render coverage (circle + text + rect + shimmer).
	card.SetPhase(0.4)
	surf := makeSurface(b.W, b.H)
	card.Draw(newP(surf, b.W), DefaultDark())
}

// TestNewPageSkeleton asserts the page preset's line-groups + image blocks
// stack in order within bounds.
func TestNewPageSkeleton(t *testing.T) {
	b := Rect{X: 0, Y: 0, W: 300, H: 600}
	page := NewPageSkeleton(b)
	items := page.Items()
	if len(items) != 5 {
		t.Fatalf("page items = %d, want 5", len(items))
	}
	wantKinds := []SkeletonKind{SkeletonRect, SkeletonText, SkeletonRect, SkeletonText, SkeletonRect}
	for i, k := range wantKinds {
		if items[i].Skel.Kind != k {
			t.Fatalf("item %d kind = %v, want %v", i, items[i].Skel.Kind, k)
		}
	}
	// Top bar at the top, full inner width (pad=12 -> W=276, H=24).
	if got := items[0].Local; got != (Rect{X: 12, Y: 12, W: 276, H: 24}) {
		t.Fatalf("top bar local = %+v", got)
	}
	// Last block: y = 12+24+16 +42+16 +90+16 +42+16 = 274, h = 90.
	if got := items[4].Local; got != (Rect{X: 12, Y: 274, W: 276, H: 90}) {
		t.Fatalf("last image local = %+v", got)
	}
	// In-order, monotonically increasing y, all within bounds.
	prevBottom := -1
	for i, it := range items {
		if it.Local.Y <= prevBottom {
			t.Fatalf("item %d y=%d not below previous bottom %d", i, it.Local.Y, prevBottom)
		}
		if it.Local.X+it.Local.W > b.W || it.Local.Y+it.Local.H > b.H {
			t.Fatalf("item %d %+v escapes page bounds %+v", i, it.Local, b)
		}
		prevBottom = it.Local.Y + it.Local.H
	}
	page.SetPhase(0.7)
	surf := makeSurface(b.W, b.H)
	page.Draw(newP(surf, b.W), DefaultLight())
}

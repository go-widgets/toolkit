// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// spyWidget is a minimal Widget implementation for layout tests: it
// records the last Bounds it was assigned + the last Event it
// received so a test can assert layout + event routing without
// pulling in Button/Label semantics.
type spyWidget struct {
	Base
	drawCount int
	lastEvent Event
	evCount   int
}

func (s *spyWidget) Draw(p painter.Painter, theme *Theme) {
	s.drawCount++
	_, _ = p, theme
}

func (s *spyWidget) OnEvent(ev Event) {
	s.lastEvent = ev
	s.evCount++
}

// --- HBox ----------------------------------------------------------------

func TestHBoxZeroChildrenLayoutNoOp(t *testing.T) {
	h := NewHBox()
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20}) // must not divide by zero
	if h.Bounds().W != 100 {
		t.Fatalf("HBox should still record its own Bounds: %+v", h.Bounds())
	}
}

func TestHBoxSingleChildFillsBox(t *testing.T) {
	h := NewHBox()
	c := &spyWidget{}
	h.Append(c)
	h.SetBounds(Rect{X: 10, Y: 5, W: 80, H: 20})
	got := c.Bounds()
	want := Rect{X: 10, Y: 5, W: 80, H: 20}
	if got != want {
		t.Fatalf("single-child Bounds = %+v, want %+v", got, want)
	}
}

func TestHBoxThreeChildrenEqualWidthsWithSpacing(t *testing.T) {
	h := NewHBox()
	h.Spacing = 10
	c1, c2, c3 := &spyWidget{}, &spyWidget{}, &spyWidget{}
	h.Append(c1)
	h.Append(c2)
	h.Append(c3)
	// Total spacing = 10*2 = 20, leaving 90-20=70 / 3 = 23 cells.
	h.SetBounds(Rect{X: 0, Y: 0, W: 90, H: 10})
	want := []Rect{
		{X: 0, Y: 0, W: 23, H: 10},
		{X: 33, Y: 0, W: 23, H: 10},
		{X: 66, Y: 0, W: 23, H: 10},
	}
	for i, c := range []*spyWidget{c1, c2, c3} {
		if c.Bounds() != want[i] {
			t.Errorf("child %d Bounds = %+v, want %+v", i, c.Bounds(), want[i])
		}
	}
}

func TestHBoxDefaultSpacingApplied(t *testing.T) {
	h := NewHBox()
	c1, c2 := &spyWidget{}, &spyWidget{}
	h.Append(c1)
	h.Append(c2)
	// Default spacing = 4; total gap = 4; cellW = (100-4)/2 = 48.
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	if c1.Bounds().W != 48 {
		t.Fatalf("default spacing not applied: c1.W = %d, want 48", c1.Bounds().W)
	}
	if c2.Bounds().X != 52 {
		t.Fatalf("c2 not offset by cellW+spacing: c2.X = %d, want 52", c2.Bounds().X)
	}
}

func TestHBoxNegativeSpacingClampedToZero(t *testing.T) {
	h := NewHBox()
	h.Spacing = -5
	c1, c2 := &spyWidget{}, &spyWidget{}
	h.Append(c1)
	h.Append(c2)
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	if c1.Bounds().W != 50 {
		t.Fatalf("negative spacing should clamp to 0; c1.W = %d, want 50", c1.Bounds().W)
	}
}

func TestHBoxEventDispatchToCorrectChild(t *testing.T) {
	h := NewHBox()
	h.Spacing = 0 // keep arithmetic clean; clamp branch covered elsewhere
	h.Spacing = -1
	c1, c2 := &spyWidget{}, &spyWidget{}
	h.Append(c1)
	h.Append(c2)
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	// c1 occupies x in [0,50), c2 in [50,100). Event at parent-local
	// (50,10) maps to surface (50,10) which is inside c2 (whose
	// Bounds.X == 50). Inside c2, local X should be 0.
	h.OnEvent(Event{Kind: EventClick, X: 50, Y: 10})
	if c1.evCount != 0 {
		t.Fatalf("c1 should not receive event; got count %d", c1.evCount)
	}
	if c2.evCount != 1 {
		t.Fatalf("c2 should receive event; got count %d", c2.evCount)
	}
	if c2.lastEvent.X != 0 || c2.lastEvent.Y != 10 {
		t.Fatalf("c2 received local (%d,%d), want (0,10)", c2.lastEvent.X, c2.lastEvent.Y)
	}
}

func TestHBoxEventMissNoDispatch(t *testing.T) {
	h := NewHBox()
	c := &spyWidget{}
	h.Append(c)
	h.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	// Event well past the box: hits nothing, must not panic.
	h.OnEvent(Event{Kind: EventClick, X: 500, Y: 500})
	if c.evCount != 0 {
		t.Fatalf("miss should not dispatch; count = %d", c.evCount)
	}
}

func TestHBoxDrawFansOut(t *testing.T) {
	h := NewHBox()
	c1, c2 := &spyWidget{}, &spyWidget{}
	h.Append(c1)
	h.Append(c2)
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	h.Draw(newP(nil, 0), DefaultLight())
	if c1.drawCount != 1 || c2.drawCount != 1 {
		t.Fatalf("each child should be drawn once; got %d/%d", c1.drawCount, c2.drawCount)
	}
}

// --- VBox ----------------------------------------------------------------

func TestVBoxZeroChildrenLayoutNoOp(t *testing.T) {
	v := NewVBox()
	v.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 100})
	if v.Bounds().H != 100 {
		t.Fatalf("VBox should still record its own Bounds: %+v", v.Bounds())
	}
}

func TestVBoxSingleChildFillsBox(t *testing.T) {
	v := NewVBox()
	c := &spyWidget{}
	v.Append(c)
	v.SetBounds(Rect{X: 5, Y: 10, W: 20, H: 80})
	want := Rect{X: 5, Y: 10, W: 20, H: 80}
	if c.Bounds() != want {
		t.Fatalf("single-child Bounds = %+v, want %+v", c.Bounds(), want)
	}
}

func TestVBoxThreeChildrenEqualHeights(t *testing.T) {
	v := NewVBox()
	v.Spacing = 10
	c1, c2, c3 := &spyWidget{}, &spyWidget{}, &spyWidget{}
	v.Append(c1)
	v.Append(c2)
	v.Append(c3)
	// Total spacing = 20; cellH = (90-20)/3 = 23.
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 90})
	want := []Rect{
		{X: 0, Y: 0, W: 10, H: 23},
		{X: 0, Y: 33, W: 10, H: 23},
		{X: 0, Y: 66, W: 10, H: 23},
	}
	for i, c := range []*spyWidget{c1, c2, c3} {
		if c.Bounds() != want[i] {
			t.Errorf("child %d Bounds = %+v, want %+v", i, c.Bounds(), want[i])
		}
	}
}

func TestVBoxDefaultSpacingApplied(t *testing.T) {
	v := NewVBox()
	c1, c2 := &spyWidget{}, &spyWidget{}
	v.Append(c1)
	v.Append(c2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 100})
	if c1.Bounds().H != 48 {
		t.Fatalf("default spacing not applied: c1.H = %d, want 48", c1.Bounds().H)
	}
}

func TestVBoxNegativeSpacingClampedToZero(t *testing.T) {
	v := NewVBox()
	v.Spacing = -3
	c1, c2 := &spyWidget{}, &spyWidget{}
	v.Append(c1)
	v.Append(c2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 100})
	if c1.Bounds().H != 50 {
		t.Fatalf("negative spacing should clamp to 0; c1.H = %d, want 50", c1.Bounds().H)
	}
}

func TestVBoxEventDispatchToCorrectChild(t *testing.T) {
	v := NewVBox()
	v.Spacing = -1 // clamped to 0
	c1, c2 := &spyWidget{}, &spyWidget{}
	v.Append(c1)
	v.Append(c2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 100})
	// c1 y in [0,50), c2 y in [50,100). Event at (10,50) targets c2.
	v.OnEvent(Event{Kind: EventClick, X: 10, Y: 50})
	if c2.evCount != 1 || c2.lastEvent.Y != 0 || c2.lastEvent.X != 10 {
		t.Fatalf("c2 dispatch wrong: count=%d local=(%d,%d)", c2.evCount, c2.lastEvent.X, c2.lastEvent.Y)
	}
	if c1.evCount != 0 {
		t.Fatalf("c1 should not receive event")
	}
}

func TestVBoxEventMissNoDispatch(t *testing.T) {
	v := NewVBox()
	c := &spyWidget{}
	v.Append(c)
	v.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	v.OnEvent(Event{Kind: EventClick, X: 500, Y: 500})
	if c.evCount != 0 {
		t.Fatalf("miss should not dispatch")
	}
}

func TestVBoxDrawFansOut(t *testing.T) {
	v := NewVBox()
	c := &spyWidget{}
	v.Append(c)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	v.Draw(newP(nil, 0), DefaultLight())
	if c.drawCount != 1 {
		t.Fatalf("child should be drawn once; got %d", c.drawCount)
	}
}

// --- Grid ----------------------------------------------------------------

func TestNewGridClampsNonPositiveDims(t *testing.T) {
	g := NewGrid(0, -3)
	// Must not divide by zero in SetBounds.
	g.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	if g.cols != 1 || g.rows != 1 {
		t.Fatalf("dims should clamp to 1; got cols=%d rows=%d", g.cols, g.rows)
	}
}

func TestGridAttachAndLayoutLowerRight(t *testing.T) {
	g := NewGrid(2, 2)
	c := &spyWidget{}
	g.Attach(c, 1, 1)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	// 2x2 grid, cellW=50, cellH=40 -> lower-right at (50,40,50,40).
	want := Rect{X: 50, Y: 40, W: 50, H: 40}
	if c.Bounds() != want {
		t.Fatalf("lower-right cell = %+v, want %+v", c.Bounds(), want)
	}
}

func TestGridAttachClampsOutOfRange(t *testing.T) {
	g := NewGrid(2, 2)
	cNeg := &spyWidget{}
	cBig := &spyWidget{}
	g.Attach(cNeg, -5, -5)
	g.Attach(cBig, 99, 99)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	// Negative -> (0,0) cell.
	if cNeg.Bounds() != (Rect{X: 0, Y: 0, W: 50, H: 40}) {
		t.Fatalf("neg-clamp child = %+v, want (0,0)-cell", cNeg.Bounds())
	}
	// Out-of-range high -> (cols-1, rows-1).
	if cBig.Bounds() != (Rect{X: 50, Y: 40, W: 50, H: 40}) {
		t.Fatalf("oversize-clamp child = %+v, want bottom-right", cBig.Bounds())
	}
}

func TestGridSetBoundsNoChildrenIsNoOp(t *testing.T) {
	g := NewGrid(3, 3)
	g.SetBounds(Rect{X: 1, Y: 2, W: 9, H: 9})
	if g.Bounds().W != 9 {
		t.Fatalf("Grid Bounds not recorded: %+v", g.Bounds())
	}
}

func TestGridEventDispatch(t *testing.T) {
	g := NewGrid(2, 2)
	a, b := &spyWidget{}, &spyWidget{}
	g.Attach(a, 0, 0)
	g.Attach(b, 1, 1)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	// Hit lower-right cell at parent-local (60,50): surface (60,50)
	// inside b's (50,40,50,40). Local should be (10,10).
	g.OnEvent(Event{Kind: EventClick, X: 60, Y: 50})
	if b.evCount != 1 || b.lastEvent.X != 10 || b.lastEvent.Y != 10 {
		t.Fatalf("Grid dispatch to (1,1) wrong: count=%d local=(%d,%d)",
			b.evCount, b.lastEvent.X, b.lastEvent.Y)
	}
	if a.evCount != 0 {
		t.Fatalf("(0,0) child should not receive the (1,1) event")
	}
}

func TestGridEventMissNoDispatch(t *testing.T) {
	g := NewGrid(2, 2)
	a := &spyWidget{}
	g.Attach(a, 0, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	g.OnEvent(Event{Kind: EventClick, X: 500, Y: 500})
	if a.evCount != 0 {
		t.Fatalf("miss should not dispatch")
	}
}

func TestGridDrawFansOut(t *testing.T) {
	g := NewGrid(1, 1)
	c := &spyWidget{}
	g.Attach(c, 0, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	g.Draw(newP(nil, 0), DefaultLight())
	if c.drawCount != 1 {
		t.Fatalf("child not drawn; count = %d", c.drawCount)
	}
}

// TestGridZeroConfigUnchanged pins the backward-compatible equal-cell layout: no
// Spacing, no fixed tracks → exactly the historical total/n cells at col*cell.
func TestGridZeroConfigUnchanged(t *testing.T) {
	g := NewGrid(2, 2)
	a, b := &spyWidget{}, &spyWidget{}
	g.Attach(a, 0, 0)
	g.Attach(b, 1, 1)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	if a.Bounds() != (Rect{X: 0, Y: 0, W: 50, H: 40}) {
		t.Fatalf("zero-config (0,0) = %+v", a.Bounds())
	}
	if b.Bounds() != (Rect{X: 50, Y: 40, W: 50, H: 40}) {
		t.Fatalf("zero-config (1,1) = %+v", b.Bounds())
	}
}

// TestGridSpacingGutters checks the inter-cell gutter on both axes.
func TestGridSpacingGutters(t *testing.T) {
	g := NewGrid(2, 2)
	g.Spacing = 10
	c := &spyWidget{}
	g.Attach(c, 1, 1)
	// cols: avail = 100-10 = 90, flex 45 each; col1 off = 45+10 = 55.
	// rows: avail = 100-10 = 90, flex 45 each; row1 off = 55.
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if c.Bounds() != (Rect{X: 55, Y: 55, W: 45, H: 45}) {
		t.Fatalf("gutter cell = %+v, want {55,55,45,45}", c.Bounds())
	}
}

// TestGridNegativeSpacingClamped clamps a negative Spacing to zero (flush).
func TestGridNegativeSpacingClamped(t *testing.T) {
	g := NewGrid(2, 1)
	g.Spacing = -5
	c := &spyWidget{}
	g.Attach(c, 1, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	if c.Bounds() != (Rect{X: 50, Y: 0, W: 50, H: 10}) {
		t.Fatalf("negative-spacing clamp = %+v, want {50,0,50,10}", c.Bounds())
	}
}

// TestGridFixedAndFlexTracks mixes fixed and flexible tracks on both axes plus a
// gutter, exercising trackFixed (in-range) and the fixed-track branch of gridTracks.
func TestGridFixedAndFlexTracks(t *testing.T) {
	g := NewGrid(3, 2)
	g.Spacing = 10
	g.ColWidths = []int{20, 0, 0} // col0 fixed 20, col1/col2 flex
	g.RowHeights = []int{0, 40}   // row0 flex, row1 fixed 40
	first, last := &spyWidget{}, &spyWidget{}
	g.Attach(first, 0, 0)
	g.Attach(last, 2, 1)
	// cols W=140: avail = 140 - 10*2 - 20 = 100, flex 50 each.
	//   col0: 20 @0; col1: 50 @30; col2: 50 @90.
	// rows H=100: avail = 100 - 10*1 - 40 = 50, one flex → 50.
	//   row0: 50 @0; row1: 40 @60.
	g.SetBounds(Rect{X: 0, Y: 0, W: 140, H: 100})
	if first.Bounds() != (Rect{X: 0, Y: 0, W: 20, H: 50}) {
		t.Fatalf("fixed/flex (0,0) = %+v, want {0,0,20,50}", first.Bounds())
	}
	if last.Bounds() != (Rect{X: 90, Y: 60, W: 50, H: 40}) {
		t.Fatalf("fixed/flex (2,1) = %+v, want {90,60,50,40}", last.Bounds())
	}
}

// TestGridAllFixedOverflow covers the all-fixed case (no flexible tracks) whose
// fixed sizes exceed the extent, exercising the avail<0 clamp and flexCount==0.
func TestGridAllFixedOverflow(t *testing.T) {
	g := NewGrid(2, 1)
	g.ColWidths = []int{80, 80} // sum 160 > 100
	c := &spyWidget{}
	g.Attach(c, 1, 0)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	// Fixed tracks keep their size regardless; col1 offset = 80.
	if c.Bounds() != (Rect{X: 80, Y: 0, W: 80, H: 10}) {
		t.Fatalf("all-fixed overflow = %+v, want {80,0,80,10}", c.Bounds())
	}
}

// TestGridEmptyRectCollapsesChildren asserts a Grid handed an empty rect collapses
// every attached child to Rect{} (the leaf-collapse invariant).
func TestGridEmptyRectCollapsesChildren(t *testing.T) {
	g := NewGrid(2, 2)
	c := &spyWidget{}
	g.Attach(c, 1, 1)
	g.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80}) // real bounds first
	g.SetBounds(Rect{})                          // now collapse
	if c.Bounds() != (Rect{}) {
		t.Fatalf("empty grid should collapse child, got %+v", c.Bounds())
	}
}

// TestBoxSpacingLiteralAndDefaults pins the new spacing semantics: the constructors
// seed DefaultBoxSpacing, and a literal Spacing = 0 now yields a flush box.
func TestBoxSpacingLiteralAndDefaults(t *testing.T) {
	if NewHBox().Spacing != DefaultBoxSpacing || NewVBox().Spacing != DefaultBoxSpacing {
		t.Fatalf("constructors must seed DefaultBoxSpacing (%d)", DefaultBoxSpacing)
	}
	// Literal zero → flush: two equal children each 50 of 100 with no gap.
	v := NewVBox()
	v.Spacing = 0
	c1, c2 := &spyWidget{}, &spyWidget{}
	v.Append(c1)
	v.Append(c2)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 100})
	if c1.Bounds().H != 50 || c2.Bounds().Y != 50 {
		t.Fatalf("Spacing=0 should be flush: c1.H=%d c2.Y=%d, want 50/50", c1.Bounds().H, c2.Bounds().Y)
	}
}

// TestBoxEmptyRectCollapses asserts HBox/VBox handed an empty rect collapse their
// children to Rect{} (the leaf-collapse invariant, independent of CardLayout).
func TestBoxEmptyRectCollapses(t *testing.T) {
	h := NewHBox()
	hc := &spyWidget{}
	h.Append(hc)
	h.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	h.SetBounds(Rect{X: 1, Y: 2, W: 0, H: 20}) // W<=0
	if hc.Bounds() != (Rect{}) {
		t.Fatalf("HBox empty collapse: %+v", hc.Bounds())
	}
	v := NewVBox()
	vc := &spyWidget{}
	v.Append(vc)
	v.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 40})
	v.SetBounds(Rect{X: 1, Y: 2, W: 20, H: 0}) // H<=0
	if vc.Bounds() != (Rect{}) {
		t.Fatalf("VBox empty collapse: %+v", vc.Bounds())
	}
}

// --- Frame ---------------------------------------------------------------

func TestFrameDrawBorderAtCorners(t *testing.T) {
	const w, h = 32, 32
	theme := DefaultLight()
	child := &spyWidget{}
	f := NewFrame(child)
	f.SetBounds(Rect{X: 4, Y: 4, W: 20, H: 20})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	// All four corners painted in Theme.Border.
	corners := [][2]int{{4, 4}, {23, 4}, {4, 23}, {23, 23}}
	for _, c := range corners {
		if got := pixelAt(buf, w, c[0], c[1]); got != theme.Border {
			t.Errorf("corner (%d,%d) = %+v, want Border", c[0], c[1], got)
		}
	}
	if child.drawCount != 1 {
		t.Fatalf("Frame should delegate Draw to child; count = %d", child.drawCount)
	}
}

func TestFrameInsetsChildByBorderAndPadding(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	// Default padding = 4 -> inset = 1+4 = 5.
	f.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 30})
	want := Rect{X: 5, Y: 5, W: 20, H: 20}
	if child.Bounds() != want {
		t.Fatalf("child Bounds = %+v, want %+v", child.Bounds(), want)
	}
}

func TestFrameCustomPaddingApplied(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.Padding = 2
	f.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	// inset = 1+2 = 3.
	want := Rect{X: 3, Y: 3, W: 14, H: 14}
	if child.Bounds() != want {
		t.Fatalf("child Bounds = %+v, want %+v", child.Bounds(), want)
	}
}

func TestFrameNegativePaddingClampsToZero(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.Padding = -7
	f.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	// inset = 1+0 = 1.
	want := Rect{X: 1, Y: 1, W: 8, H: 8}
	if child.Bounds() != want {
		t.Fatalf("child Bounds = %+v, want %+v", child.Bounds(), want)
	}
}

func TestFrameNilChildSetBoundsAndDrawSafe(t *testing.T) {
	f := NewFrame(nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	buf := makeSurface(16, 16)
	f.Draw(newP(buf, 16), DefaultLight()) // must not panic
	// Border still painted at top-left.
	if got := pixelAt(buf, 16, 0, 0); got != DefaultLight().Border {
		t.Fatalf("border should still paint without child; got %+v", got)
	}
}

func TestFrameNilChildOnEventNoPanic(t *testing.T) {
	f := NewFrame(nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // must not panic
}

func TestFrameEventForwardedToChild(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.Padding = 2
	f.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	// Child at (3,3,14,14). Parent-local event (5,5) maps to surface
	// (5,5), inside child; local (5-3, 5-3) = (2,2).
	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if child.evCount != 1 || child.lastEvent.X != 2 || child.lastEvent.Y != 2 {
		t.Fatalf("forwarded event wrong: count=%d local=(%d,%d)",
			child.evCount, child.lastEvent.X, child.lastEvent.Y)
	}
}

func TestFrameEventOutsideChildIgnored(t *testing.T) {
	child := &spyWidget{}
	f := NewFrame(child)
	f.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	// Event at (0,0) is on the border, NOT inside the inset child.
	f.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	if child.evCount != 0 {
		t.Fatalf("border-only event should not reach child; count=%d", child.evCount)
	}
}

func TestHBoxFlexAndFixed(t *testing.T) {
	h := NewHBox()
	h.Spacing = -1
	fixedW, flex1, flex2 := &spyWidget{}, &spyWidget{}, &spyWidget{}
	h.AddFixed(fixedW, 20)
	h.AddFlex(flex1, 1)
	h.AddFlex(flex2, 3)
	// W=100, fixed=20, avail=80, flexTotal=4 → flex1=20, flex2=60.
	h.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 10})
	if got := fixedW.Bounds(); got != (Rect{X: 0, Y: 0, W: 20, H: 10}) {
		t.Fatalf("fixed = %+v", got)
	}
	if got := flex1.Bounds(); got != (Rect{X: 20, Y: 0, W: 20, H: 10}) {
		t.Fatalf("flex1 = %+v", got)
	}
	if got := flex2.Bounds(); got != (Rect{X: 40, Y: 0, W: 60, H: 10}) {
		t.Fatalf("flex2 = %+v", got)
	}
}

func TestVBoxFlexAndFixed(t *testing.T) {
	v := NewVBox()
	v.Spacing = -1
	head, body := &spyWidget{}, &spyWidget{}
	v.AddFixed(head, 30)
	v.AddFlex(body, 1)
	// H=100, fixed=30, avail=70 → body fills 70.
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 100})
	if got := head.Bounds(); got != (Rect{X: 0, Y: 0, W: 10, H: 30}) {
		t.Fatalf("head = %+v", got)
	}
	if got := body.Bounds(); got != (Rect{X: 0, Y: 30, W: 10, H: 70}) {
		t.Fatalf("body = %+v", got)
	}
}

func TestBoxAddClampsAndOverflow(t *testing.T) {
	// Negative flex clamps to 1; a single flex child fills the box.
	h := NewHBox()
	h.Spacing = -1
	c := &spyWidget{}
	h.AddFlex(c, -5)
	h.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 10})
	if c.Bounds().W != 50 {
		t.Fatalf("flex clamp width = %d, want 50", c.Bounds().W)
	}
	// Negative fixed clamps to 0.
	v := NewVBox()
	v.Spacing = -1
	d := &spyWidget{}
	v.AddFixed(d, -3)
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 50})
	if d.Bounds().H != 0 {
		t.Fatalf("fixed clamp height = %d, want 0", d.Bounds().H)
	}
	e := &spyWidget{}
	v.AddFlex(e, -1) // VBox AddFlex clamp
	v.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 40})
	if e.Bounds().H != 40 {
		t.Fatalf("vbox flex clamp height = %d, want 40", e.Bounds().H)
	}
	// Fixed larger than the box → avail clamps to 0 (no negative sizes).
	h2 := NewHBox()
	h2.Spacing = -1
	big, flex := &spyWidget{}, &spyWidget{}
	h2.AddFixed(big, 100)
	h2.AddFlex(flex, 1)
	h2.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 10})
	if flex.Bounds().W != 0 {
		t.Fatalf("overflow flex width = %d, want 0", flex.Bounds().W)
	}
	// HBox AddFixed negative clamps to 0.
	h3 := NewHBox()
	h3.Spacing = -1
	fx := &spyWidget{}
	h3.AddFixed(fx, -2)
	h3.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 10})
	if fx.Bounds().W != 0 {
		t.Fatalf("hbox fixed clamp width = %d, want 0", fx.Bounds().W)
	}
}

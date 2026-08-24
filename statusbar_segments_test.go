// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// sbPainter builds a throwaway pixel painter big enough for a w×h Statusbar.
func sbPainter(w, h int) *painter.PixelPainter { return newP(make([]byte, 4*w*h), w) }

// --- interactive segment layout ------------------------------------------

// TestStatusbarGroupLayoutX pins the x placement of the three groups: Left packs
// from x=0, Right packs against the right edge, Center is centred in the bar.
func TestStatusbarGroupLayoutX(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	sb := &Statusbar{
		Left:   []StatusSegment{{MinW: 40}, {MinW: 40}, {MinW: 40}},
		Center: []StatusSegment{{MinW: 50}},
		Right:  []StatusSegment{{MinW: 30}, {MinW: 30}},
	}
	const W, H = 200, StatusbarH
	sb.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})

	boxes := sb.boxes()
	if len(boxes) != 6 {
		t.Fatalf("boxes = %d, want 6", len(boxes))
	}
	// Left: 0, 40, 80 — each 40 wide, packed from the left edge.
	for i, wantX := range []int{0, 40, 80} {
		if got := boxes[i].rect.X; got != wantX {
			t.Errorf("left[%d].X = %d, want %d", i, got, wantX)
		}
		if got := boxes[i].rect.W; got != 40 {
			t.Errorf("left[%d].W = %d, want 40", i, got)
		}
	}
	// Right (boxes 3,4): rsum=60 → start 140, then 170; rightmost edge == W.
	if got := boxes[3].rect.X; got != 140 {
		t.Errorf("right[0].X = %d, want 140", got)
	}
	if got := boxes[4].rect.X; got != 170 {
		t.Errorf("right[1].X = %d, want 170", got)
	}
	if edge := boxes[4].rect.X + boxes[4].rect.W; edge != W {
		t.Errorf("right group rightmost edge = %d, want %d", edge, W)
	}
	// Center (box 5): centred → (200-50)/2 = 75.
	if got := boxes[5].rect.X; got != 75 {
		t.Errorf("center.X = %d, want 75", got)
	}
	// Every box spans the full bar height.
	for i, b := range boxes {
		if b.rect.H != H {
			t.Errorf("box[%d].H = %d, want %d", i, b.rect.H, H)
		}
	}
}

// TestStatusbarClickRoutesToOwnSegment is the core routing proof: a click inside
// segment 2's rect fires ITS OnClick and neither neighbour's.
func TestStatusbarClickRoutesToOwnSegment(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	fired := -1
	mk := func(i int) func() { return func() { fired = i } }
	sb := &Statusbar{Left: []StatusSegment{
		{MinW: 40, OnClick: mk(0)},
		{MinW: 40, OnClick: mk(1)},
		{MinW: 40, OnClick: mk(2)},
	}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})

	// Segment 2 spans [40,80): click its centre.
	sb.OnEvent(Event{Kind: EventClick, X: 60, Y: StatusbarH / 2})
	if fired != 1 {
		t.Fatalf("click in segment index 1 fired %d, want 1", fired)
	}
	// Segment 0 spans [0,40).
	fired = -1
	sb.OnEvent(Event{Kind: EventClick, X: 5, Y: 2})
	if fired != 0 {
		t.Fatalf("click in segment 0 fired %d, want 0", fired)
	}
}

// TestStatusbarClickInGapIsNoOp covers OnEvent's no-match return: a click past
// every segment's box does nothing.
func TestStatusbarClickInGapIsNoOp(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	fired := false
	sb := &Statusbar{Left: []StatusSegment{{MinW: 40, OnClick: func() { fired = true }}}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	sb.OnEvent(Event{Kind: EventClick, X: 150, Y: 2}) // right of the single 40px cell
	if fired {
		t.Fatal("click in the gap must not fire any segment")
	}
	// Out of the vertical band too.
	sb.OnEvent(Event{Kind: EventClick, X: 10, Y: 999})
	if fired {
		t.Fatal("click below the bar must not fire")
	}
}

// TestStatusbarTextSegmentNonClickAndNoHandler covers the two OnEvent no-ops on a
// plain text segment: a non-click event, and a click on a handler-less segment.
func TestStatusbarTextSegmentNonClickAndNoHandler(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	fired := false
	sb := &Statusbar{Left: []StatusSegment{
		{Text: "handler", MinW: 40, OnClick: func() { fired = true }},
		{Text: "plain", MinW: 40}, // no OnClick
	}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})

	// A hover (non-click) over the handler segment must not fire it.
	sb.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 2})
	if fired {
		t.Fatal("non-click over a text segment must not fire OnClick")
	}
	// A click over the handler-less segment [40,80) is a no-op (no panic).
	sb.OnEvent(Event{Kind: EventClick, X: 60, Y: 2})
	if fired {
		t.Fatal("click landed on the wrong segment")
	}
}

// TestStatusbarDisabledSwallowsEvents covers OnEvent's Disabled early return.
func TestStatusbarDisabledSwallowsEvents(t *testing.T) {
	fired := false
	sb := &Statusbar{Left: []StatusSegment{{MinW: 40, OnClick: func() { fired = true }}}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	sb.Disabled().Set(true)
	sb.OnEvent(Event{Kind: EventClick, X: 5, Y: 2})
	if fired {
		t.Fatal("a Disabled Statusbar must swallow clicks")
	}
}

// --- hosted widget segments ----------------------------------------------

// TestStatusbarWidgetSegmentForwardsTranslated proves a widget-hosting segment
// receives the event in its OWN local coordinates, and that its box width comes
// from the widget's own bounds.
func TestStatusbarWidgetSegmentForwardsTranslated(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	rw := &recordingWidget{}
	rw.SetBounds(Rect{X: 0, Y: 0, W: 30, H: StatusbarH}) // W>0 → drives segWidth
	sb := &Statusbar{Left: []StatusSegment{
		{Text: "pad", MinW: 40},
		{Widget: rw},
	}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})

	// The widget box sits at x=40 (after the 40px pad cell), width 30.
	boxes := sb.boxes()
	if boxes[1].rect.X != 40 || boxes[1].rect.W != 30 {
		t.Fatalf("widget box = (X=%d,W=%d), want (40,30)", boxes[1].rect.X, boxes[1].rect.W)
	}
	// Click at surface-local (45,5): the widget must see (5,5).
	sb.OnEvent(Event{Kind: EventClick, X: 45, Y: 5})
	if len(rw.events) != 1 {
		t.Fatalf("hosted widget saw %d events, want 1", len(rw.events))
	}
	if rw.events[0].X != 5 || rw.events[0].Y != 5 {
		t.Fatalf("forwarded event local coords = (%d,%d), want (5,5)", rw.events[0].X, rw.events[0].Y)
	}
}

// TestStatusbarWidgetSegmentZeroWidthFallsBackToText covers segWidth's branch
// where a hosted widget has no bounds yet (W==0): the segment sizes from its Text.
func TestStatusbarWidgetSegmentZeroWidthFallsBackToText(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	sb := &Statusbar{Left: []StatusSegment{
		{Text: "ab", Widget: &recordingWidget{}}, // widget has no bounds → W==0
	}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	want := sb.textWidth("ab") + 2*scaled(StatusbarPadX)
	if got := sb.boxes()[0].rect.W; got != want {
		t.Fatalf("zero-width hosted widget box W = %d, want text-sized %d", got, want)
	}
}

// --- SegmentAt -----------------------------------------------------------

// TestStatusbarSegmentAt covers the public hit query: a hit returns the segment,
// a miss returns nil.
func TestStatusbarSegmentAt(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	first := StatusSegment{Text: "one", MinW: 40}
	sb := &Statusbar{Left: []StatusSegment{first, {Text: "two", MinW: 40}}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})

	if seg := sb.SegmentAt(10, 2); seg == nil || seg.Text != "one" {
		t.Fatalf("SegmentAt over cell 0 = %v, want the 'one' segment", seg)
	}
	if seg := sb.SegmentAt(150, 2); seg != nil {
		t.Fatalf("SegmentAt over empty space = %v, want nil", seg)
	}
}

// --- Draw ----------------------------------------------------------------

// TestStatusbarDrawGroups exercises drawGroups: a bar with both a text segment
// and a hosted-widget segment. The widget's Draw must be invoked and its bounds
// set to the segment box.
func TestStatusbarDrawGroups(t *testing.T) {
	SetDensity(DensityCompact)
	defer SetDensity(DensityCompact)

	rw := &recordingWidget{}
	rw.SetBounds(Rect{X: 0, Y: 0, W: 30, H: StatusbarH})
	sb := &Statusbar{
		Left:  []StatusSegment{{Text: "Ln 1", MinW: 40}},
		Right: []StatusSegment{{Widget: rw}},
	}
	const W, H = 200, StatusbarH
	sb.SetBounds(Rect{X: 5, Y: 7, W: W, H: H})
	sb.Draw(sbPainter(W+8, H+8), DefaultLight())

	if rw.draws != 1 {
		t.Fatalf("hosted widget drawn %d times, want 1", rw.draws)
	}
	// The single Right widget box sits flush to the right edge; Draw offsets it by
	// the bar's origin (X=5,Y=7).
	if got := rw.Bounds(); got.X != 5+W-30 || got.Y != 7 || got.W != 30 || got.H != H {
		t.Fatalf("hosted widget bounds after Draw = %+v", got)
	}
}

// --- a11y ----------------------------------------------------------------

// TestStatusbarA11yWithGroups proves the a11y Value folds the plain Segments and
// every interactive segment's label into one " | "-joined string, and that
// statusSegLabel resolves each label form.
func TestStatusbarA11yWithGroups(t *testing.T) {
	sb := &Statusbar{
		Segments: []string{"Ln 1"},
		Left:     []StatusSegment{{Text: "UTF-8"}},                  // text label
		Center:   []StatusSegment{{Widget: NewButton("Save", nil)}}, // accessible Name
		Right: []StatusSegment{
			{Widget: NewSwitch(false)},   // accessible, Name empty → Value "off"
			{Widget: &recordingWidget{}}, // not accessible → ""
		},
	}
	got := sb.A11y()
	want := A11yInfo{Role: RoleStatus, Value: "Ln 1 | UTF-8 | Save | off | "}
	if got != want {
		t.Fatalf("A11y = %+v, want %+v", got, want)
	}
}

// TestStatusbarA11yNoGroupsUnchanged pins the back-compat a11y: with no
// interactive groups the Value is exactly strings.Join(Segments, " | ").
func TestStatusbarA11yNoGroupsUnchanged(t *testing.T) {
	sb := NewStatusbar([]string{"Ln 1", "UTF-8"})
	if got := sb.A11y(); got != (A11yInfo{Role: RoleStatus, Value: "Ln 1 | UTF-8"}) {
		t.Fatalf("A11y (no groups) = %+v, want the legacy joined value", got)
	}
}

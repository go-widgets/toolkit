// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"
)

// approxEq reports whether a and b are within eps of each other. It is the
// only float comparator the multi-touch tests rely on, so TestMultiTouchControlRun
// validates it against known-good and known-bad inputs before any recognizer
// test depends on it. Most assertions below are on inputs proven bit-exact
// (see the check program in the PR notes), but approxEq guards the few that are
// not required to be.
func approxEq(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// begin/update/end recorder for a MultiTouchRecognizer. Every callback appends
// here so a test can assert exact counts and the exact state each phase saw.
type mtRec struct {
	begins  []MultiTouchState
	updates []MultiTouchState
	ends    []MultiTouchState
	pinch   []float64
	rotate  []float64
	panX    []float64
	panY    []float64
}

func newMTRec(g *MultiTouchRecognizer) *mtRec {
	r := &mtRec{}
	g.OnMultiBegin = func(m MultiTouchState) { r.begins = append(r.begins, m) }
	g.OnMultiUpdate = func(m MultiTouchState) { r.updates = append(r.updates, m) }
	g.OnMultiEnd = func(m MultiTouchState) { r.ends = append(r.ends, m) }
	g.OnPinch = func(s float64) { r.pinch = append(r.pinch, s) }
	g.OnRotate = func(rad float64) { r.rotate = append(r.rotate, rad) }
	g.OnPan = func(dx, dy float64) { r.panX = append(r.panX, dx); r.panY = append(r.panY, dy) }
	return r
}

func start(g *MultiTouchRecognizer, id string, x, y int) {
	g.Feed(Event{Kind: EventTouchStart, Code: id, X: x, Y: y})
}
func move(g *MultiTouchRecognizer, id string, x, y int) {
	g.Feed(Event{Kind: EventTouchMove, Code: id, X: x, Y: y})
}
func end(g *MultiTouchRecognizer, id string, x, y int) {
	g.Feed(Event{Kind: EventTouchEnd, Code: id, X: x, Y: y})
}

// TestMultiTouchControlRun is the control run required before the recognizer
// tests trust the helpers: it proves (1) approxEq flags a known-good match and
// a known-bad mismatch, and (2) a fresh recognizer driven by a hand-built,
// known-good synthetic two-contact sequence yields the hand-computed scale —
// and NOT a deliberately wrong "known-bad" value.
func TestMultiTouchControlRun(t *testing.T) {
	// (1) comparator: known-good within eps, known-bad beyond it.
	if !approxEq(2.0, 2.0, 1e-9) {
		t.Fatal("approxEq rejected an exact match")
	}
	if !approxEq(2.0, 2.0+1e-12, 1e-9) {
		t.Fatal("approxEq rejected a within-eps match")
	}
	if approxEq(2.0, 2.1, 1e-9) {
		t.Fatal("approxEq accepted a known-bad mismatch")
	}

	// (2) known-good synthetic sequence: contacts 10px apart, one moved to
	// 20px apart -> hand-computed scale exactly 2.0, never the bad 1.0.
	g := NewMultiTouchRecognizer()
	var gotScale float64
	g.OnPinch = func(s float64) { gotScale = s }
	start(g, "a", 0, 0)
	start(g, "b", 10, 0) // engage, initSpan = 10
	move(g, "b", 20, 0)  // span 20 -> scale 2
	if !approxEq(gotScale, 2.0, 1e-12) {
		t.Fatalf("control-run scale = %v, want known-good 2.0", gotScale)
	}
	if approxEq(gotScale, 1.0, 1e-9) {
		t.Fatal("control-run scale matched the known-bad 1.0")
	}
}

func TestMultiTouchNewIsIdle(t *testing.T) {
	g := NewMultiTouchRecognizer()
	if g.Engaged() {
		t.Fatal("fresh recognizer is engaged, want idle")
	}
	if len(g.contacts) != 0 {
		t.Fatalf("fresh recognizer has %d contacts, want 0", len(g.contacts))
	}
	// State on an idle recognizer is the zero value.
	if s := g.State(); s != (MultiTouchState{}) {
		t.Fatalf("fresh State = %+v, want zero", s)
	}
}

func TestMultiTouchBeginState(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	if g.Engaged() {
		t.Fatal("engaged after one contact, want not engaged")
	}
	if len(r.begins) != 0 {
		t.Fatalf("OnMultiBegin fired %d times after one contact, want 0", len(r.begins))
	}
	start(g, "b", 10, 0) // second contact engages
	if !g.Engaged() {
		t.Fatal("not engaged after second contact")
	}
	if len(r.begins) != 1 {
		t.Fatalf("OnMultiBegin fired %d times, want 1", len(r.begins))
	}
	got := r.begins[0]
	want := MultiTouchState{Scale: 1, Rotation: 0, CenterX: 5, CenterY: 0, PanX: 0, PanY: 0, Span: 10}
	if got != want {
		t.Fatalf("begin state = %+v, want %+v", got, want)
	}
	// No update/pinch/etc. fire merely from engaging.
	if len(r.updates)+len(r.pinch)+len(r.rotate)+len(r.panX) != 0 {
		t.Fatal("update callbacks fired at begin, want none")
	}
}

func TestMultiTouchPinchExactScales(t *testing.T) {
	cases := []struct {
		name      string
		bx, by    int
		wantScale float64
	}{
		{"spread-2x", 20, 0, 2.0},
		{"pinch-half", 5, 0, 0.5},
		{"unchanged", 10, 0, 1.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewMultiTouchRecognizer()
			var got float64
			fired := 0
			g.OnPinch = func(s float64) { fired++; got = s }
			start(g, "a", 0, 0)
			start(g, "b", 10, 0) // initSpan = 10
			move(g, "b", c.bx, c.by)
			if fired != 1 {
				t.Fatalf("OnPinch fired %d times, want 1", fired)
			}
			if got != c.wantScale {
				t.Fatalf("scale = %v, want exactly %v", got, c.wantScale)
			}
		})
	}
}

func TestMultiTouchRotateExactAngles(t *testing.T) {
	cases := []struct {
		name    string
		bx, by  int // b moves here from (10,0); a stays at (0,0), initAngle 0
		wantRad float64
	}{
		{"quarter-turn-positive", 0, 10, math.Pi / 2},
		{"quarter-turn-negative", 0, -10, -math.Pi / 2},
		{"half-turn", -10, 0, math.Pi},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := NewMultiTouchRecognizer()
			var got float64
			fired := 0
			g.OnRotate = func(rad float64) { fired++; got = rad }
			start(g, "a", 0, 0)
			start(g, "b", 10, 0) // initAngle = atan2(0,10) = 0
			move(g, "b", c.bx, c.by)
			if fired != 1 {
				t.Fatalf("OnRotate fired %d times, want 1", fired)
			}
			if got != c.wantRad {
				t.Fatalf("rotation = %v, want exactly %v", got, c.wantRad)
			}
		})
	}
}

// TestMultiTouchRotateWrap exercises both normalization branches of normAngle
// with exact expected values (verified bit-exact).
func TestMultiTouchRotateWrap(t *testing.T) {
	// Negative-wrap branch (a <= -Pi): init angle Pi (b left of a), rotate to
	// -Pi/2 (b below a): raw = -3Pi/2 -> +2Pi -> Pi/2.
	t.Run("neg-wrap", func(t *testing.T) {
		g := NewMultiTouchRecognizer()
		var got float64
		g.OnRotate = func(rad float64) { got = rad }
		start(g, "a", 0, 0)
		start(g, "b", -10, 0) // initAngle = atan2(0,-10) = Pi
		move(g, "b", 0, -10)  // angle = atan2(-10,0) = -Pi/2
		if got != math.Pi/2 {
			t.Fatalf("rotation = %v, want exactly Pi/2 (%v)", got, math.Pi/2)
		}
	})
	// Positive-wrap branch (a > Pi): init angle -Pi/2 (b below a), rotate to
	// Pi (b left of a): raw = 3Pi/2 -> -2Pi -> -Pi/2.
	t.Run("pos-wrap", func(t *testing.T) {
		g := NewMultiTouchRecognizer()
		var got float64
		g.OnRotate = func(rad float64) { got = rad }
		start(g, "a", 0, 0)
		start(g, "b", 0, -10) // initAngle = atan2(-10,0) = -Pi/2
		move(g, "b", -10, 0)  // angle = atan2(0,-10) = Pi
		if got != -math.Pi/2 {
			t.Fatalf("rotation = %v, want exactly -Pi/2 (%v)", got, -math.Pi/2)
		}
	})
}

func TestMultiTouchPanExactTranslation(t *testing.T) {
	g := NewMultiTouchRecognizer()
	var dx, dy float64
	fired := 0
	g.OnPan = func(x, y float64) { fired++; dx, dy = x, y }
	start(g, "a", 0, 0)
	start(g, "b", 10, 0) // begin centroid (5,0)
	move(g, "a", 2, 4)   // now a(2,4) b(10,0): centroid (6,2)
	if fired != 1 {
		t.Fatalf("OnPan fired %d times, want 1", fired)
	}
	if dx != 1 || dy != 2 {
		t.Fatalf("pan = (%v,%v), want (1,2)", dx, dy)
	}
	// The full state must agree with the pan and report the new centroid.
	s := g.State()
	if s.CenterX != 6 || s.CenterY != 2 || s.PanX != 1 || s.PanY != 2 {
		t.Fatalf("state centroid/pan = (%v,%v)/(%v,%v), want (6,2)/(1,2)", s.CenterX, s.CenterY, s.PanX, s.PanY)
	}
}

func TestMultiTouchZeroInitialSpanHoldsScaleAtOne(t *testing.T) {
	g := NewMultiTouchRecognizer()
	var got float64
	g.OnPinch = func(s float64) { got = s }
	start(g, "a", 5, 5)
	start(g, "b", 5, 5) // same point: initSpan = 0
	if s := g.State(); s.Scale != 1 || s.Span != 0 {
		t.Fatalf("begin with zero span: Scale=%v Span=%v, want 1/0", s.Scale, s.Span)
	}
	move(g, "b", 5, 15) // span now 10, but scale must stay 1 (no ratio from 0)
	if got != 1 {
		t.Fatalf("scale = %v after moving from a zero span, want held at 1", got)
	}
	if s := g.State(); s.Span != 10 {
		t.Fatalf("Span = %v, want 10 (span is still reported)", s.Span)
	}
}

func TestMultiTouchUpdateCallbackOrderingAndCounts(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	start(g, "b", 10, 0)
	move(g, "b", 20, 0) // one anchor move -> exactly one of each update callback
	move(g, "a", 0, 0)  // a re-set to same spot still counts as an anchor move
	if len(r.begins) != 1 {
		t.Fatalf("begins = %d, want 1", len(r.begins))
	}
	if len(r.updates) != 2 || len(r.pinch) != 2 || len(r.rotate) != 2 || len(r.panX) != 2 {
		t.Fatalf("update counts: updates=%d pinch=%d rotate=%d pan=%d, want all 2",
			len(r.updates), len(r.pinch), len(r.rotate), len(r.panX))
	}
	if len(r.ends) != 0 {
		t.Fatalf("ends = %d, want 0 (nothing lifted yet)", len(r.ends))
	}
}

func TestMultiTouchEndOnAnchorLift(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	start(g, "b", 10, 0)
	move(g, "b", 20, 0) // scale 2, so the end state is well-defined
	end(g, "b", 20, 0)  // anchor lifts -> end
	if g.Engaged() {
		t.Fatal("still engaged after an anchor lifted")
	}
	if len(r.ends) != 1 {
		t.Fatalf("ends = %d, want 1", len(r.ends))
	}
	if r.ends[0].Scale != 2 {
		t.Fatalf("end state Scale = %v, want the last value 2", r.ends[0].Scale)
	}
	// One contact remains; nothing re-engages.
	if len(r.begins) != 1 {
		t.Fatalf("begins = %d, want 1 (one survivor cannot re-engage)", len(r.begins))
	}
}

func TestMultiTouchThirdContactHandoffReEngages(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	start(g, "b", 10, 0) // engage on (a,b)
	start(g, "c", 0, 10) // third contact: tracked, no new begin/end
	if len(r.begins) != 1 || len(r.ends) != 0 {
		t.Fatalf("after third contact: begins=%d ends=%d, want 1/0", len(r.begins), len(r.ends))
	}
	if g.anchorA != "a" || g.anchorB != "b" {
		t.Fatalf("anchors = (%q,%q), want (a,b)", g.anchorA, g.anchorB)
	}
	// Lift anchor a: gesture ends, then survivors (b,c) re-engage.
	end(g, "a", 0, 0)
	if len(r.ends) != 1 {
		t.Fatalf("ends = %d, want 1", len(r.ends))
	}
	if len(r.begins) != 2 {
		t.Fatalf("begins = %d, want 2 (re-engage on survivors)", len(r.begins))
	}
	if !g.Engaged() {
		t.Fatal("not engaged after handoff, want engaged on (b,c)")
	}
	if g.anchorA != "b" || g.anchorB != "c" {
		t.Fatalf("post-handoff anchors = (%q,%q), want (b,c)", g.anchorA, g.anchorB)
	}
}

func TestMultiTouchNonAnchorMoveDoesNothing(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	start(g, "b", 10, 0) // anchors (a,b)
	start(g, "c", 0, 10) // non-anchor
	move(g, "c", 99, 99) // moving a non-anchor changes nothing, fires nothing
	if len(r.updates) != 0 || len(r.pinch) != 0 {
		t.Fatalf("non-anchor move fired updates=%d pinch=%d, want 0/0", len(r.updates), len(r.pinch))
	}
	// Its position is still tracked, though.
	if i := g.indexOf("c"); i < 0 || g.contacts[i].x != 99 || g.contacts[i].y != 99 {
		t.Fatal("non-anchor contact position was not tracked")
	}
}

func TestMultiTouchNonAnchorLiftKeepsGesture(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 0, 0)
	start(g, "b", 10, 0)
	start(g, "c", 0, 10)
	end(g, "c", 0, 10) // lifting a non-anchor must not end or re-begin
	if !g.Engaged() {
		t.Fatal("gesture ended when a non-anchor lifted")
	}
	if len(r.ends) != 0 || len(r.begins) != 1 {
		t.Fatalf("non-anchor lift: begins=%d ends=%d, want 1/0", len(r.begins), len(r.ends))
	}
}

func TestMultiTouchMoveUnknownContactIgnored(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	move(g, "ghost", 5, 5) // no such contact
	if len(g.contacts) != 0 {
		t.Fatalf("unknown move created a contact: %d", len(g.contacts))
	}
	// Even while engaged, a move for an unknown id is ignored.
	start(g, "a", 0, 0)
	start(g, "b", 10, 0)
	move(g, "ghost", 1, 1)
	if len(r.updates) != 0 {
		t.Fatalf("unknown-id move fired %d updates, want 0", len(r.updates))
	}
}

func TestMultiTouchEndUnknownContactIgnored(t *testing.T) {
	g := NewMultiTouchRecognizer()
	end(g, "ghost", 0, 0) // no contacts at all
	if g.Engaged() || len(g.contacts) != 0 {
		t.Fatal("ending an unknown contact perturbed state")
	}
}

func TestMultiTouchSingleContactLift(t *testing.T) {
	g := NewMultiTouchRecognizer()
	r := newMTRec(g)
	start(g, "a", 3, 3)
	end(g, "a", 3, 3) // only contact lifts: never engaged, nothing fires
	if len(g.contacts) != 0 {
		t.Fatalf("contact not removed: %d live", len(g.contacts))
	}
	if len(r.begins) != 0 || len(r.ends) != 0 {
		t.Fatalf("single-contact lift fired begins=%d ends=%d, want 0/0", len(r.begins), len(r.ends))
	}
}

func TestMultiTouchDuplicateStartUpdatesInPlace(t *testing.T) {
	g := NewMultiTouchRecognizer()
	start(g, "a", 0, 0)
	start(g, "a", 4, 0) // same id restarts in place, not a new contact
	if len(g.contacts) != 1 {
		t.Fatalf("duplicate start added a contact: %d live, want 1", len(g.contacts))
	}
	start(g, "b", 4, 0) // now engage: a is at (4,0), b at (4,0) -> span 0
	if s := g.State(); s.Span != 0 {
		t.Fatalf("Span = %v, want 0 (duplicate start moved a onto b)", s.Span)
	}
}

func TestMultiTouchNonTouchEventIgnored(t *testing.T) {
	g := NewMultiTouchRecognizer()
	g.Feed(Event{Kind: EventClick, X: 5, Y: 5})
	if g.Engaged() || len(g.contacts) != 0 {
		t.Fatal("a non-touch event perturbed the recognizer")
	}
}

func TestMultiTouchNilCallbacksNeverPanic(t *testing.T) {
	g := NewMultiTouchRecognizer() // every callback left nil
	// Full life-cycle including a three-finger handoff and a lift.
	start(g, "a", 0, 0)
	start(g, "b", 10, 0)
	move(g, "b", 20, 0)
	move(g, "a", 2, 4)
	start(g, "c", 0, 10)
	end(g, "a", 2, 4) // end + re-begin, all with nil callbacks
	end(g, "b", 20, 0)
	end(g, "c", 0, 10)
	if g.Engaged() {
		t.Fatal("still engaged after every contact lifted")
	}
}

// TestMultiTouchNormAngleDefaultBranch pins the default (no-wrap) branch of
// normAngle with an exact small angle.
func TestMultiTouchNormAngleDefaultBranch(t *testing.T) {
	if normAngle(0) != 0 {
		t.Fatalf("normAngle(0) = %v, want 0", normAngle(0))
	}
	if normAngle(math.Pi) != math.Pi { // boundary: Pi stays (interval is (-Pi, Pi])
		t.Fatalf("normAngle(Pi) = %v, want Pi", normAngle(math.Pi))
	}
	if normAngle(-math.Pi) != math.Pi { // -Pi folds up to +Pi
		t.Fatalf("normAngle(-Pi) = %v, want Pi", normAngle(-math.Pi))
	}
}

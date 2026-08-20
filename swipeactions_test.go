// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"

	"github.com/go-widgets/painter"
)

// --- test doubles ----------------------------------------------------------

// swContent records every event it is handed and paints white at its bounds, so
// a test can prove SwipeActions forwarded (and translated) an event and can see
// where the content was drawn.
type swContent struct {
	Base
	events []Event
}

func (r *swContent) OnEvent(ev Event) { r.events = append(r.events, ev) }
func (r *swContent) Draw(p painter.Painter, _ *Theme) {
	p.FillRect(r.Bounds(), RGB(0xFF, 0xFF, 0xFF))
}

// swipeFill is one recorded FillRect.
type swipeFill struct {
	r Rect
	c RGBA
}

// swipeRec is a recording painter that also implements painter.Clipper, so the
// clip push/pop path in Draw is exercised and every lane / content fill can be
// inspected by exact rectangle and colour.
type swipeRec struct {
	fills  []swipeFill
	texts  []string
	clips  []Rect
	popped int
}

func (p *swipeRec) FillRect(r Rect, c RGBA)              { p.fills = append(p.fills, swipeFill{r, c}) }
func (p *swipeRec) StrokeRect(Rect, RGBA, int)           {}
func (p *swipeRec) FillRoundRect(r Rect, _ int, c RGBA)  { p.fills = append(p.fills, swipeFill{r, c}) }
func (p *swipeRec) StrokeRoundRect(Rect, int, RGBA, int) {}
func (p *swipeRec) PutPixel(int, int, RGBA)              {}
func (p *swipeRec) Text(_, _ int, s string, _ RGBA)      { p.texts = append(p.texts, s) }
func (p *swipeRec) Size() (int, int)                     { return 300, 40 }
func (p *swipeRec) PushClip(r Rect)                      { p.clips = append(p.clips, r) }
func (p *swipeRec) PopClip()                             { p.popped++ }

// findFill returns the first recorded fill with colour c, or (zero, false).
func (p *swipeRec) findFill(c RGBA) (Rect, bool) {
	for _, f := range p.fills {
		if f.c == c {
			return f.r, true
		}
	}
	return Rect{}, false
}

// --- fixtures --------------------------------------------------------------

const (
	swRowW = 300
	swRowH = 40
	// With MetricScale 1 and DensityCompact, one lane is exactly ActionWidth px.
	swAW = swipeActionWidth // 72
)

// resetSwipeGlobals pins the metric scale and density to their defaults for the
// duration of a test, so the exact-pixel assertions do not depend on a global a
// sibling test left changed.
func resetSwipeGlobals(t *testing.T) {
	t.Helper()
	ps, pd := MetricScale(), Density()
	SetMetricScale(1)
	SetDensity(DensityCompact)
	t.Cleanup(func() {
		SetMetricScale(ps)
		SetDensity(pd)
	})
}

// newTrailSA builds a row with three trailing actions (the third destructive)
// and a recording content, its bounds the standard 300x40 row.
func newTrailSA(onC func()) *SwipeActions {
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{X: 0, Y: 0, W: swRowW, H: swRowH})
	sa.Trailing = []SwipeAction{
		{Label: "A", Color: RGB(1, 0, 0)},
		{Label: "B", Color: RGB(2, 0, 0)},
		{Label: "C", Color: RGB(3, 0, 0), Destructive: true, OnInvoke: onC},
	}
	return sa
}

// settle advances an in-flight settle to completion (bounded), returning the
// number of ticks it took.
func settle(sa *SwipeActions) int {
	n := 0
	for sa.Settling() && n < 100000 {
		sa.Tick(1.0 / 60.0)
		n++
	}
	return n
}

// --- control run -----------------------------------------------------------
//
// Before trusting the widget's snap decisions, validate the METHOD: an
// independent reference of the same threshold arithmetic must reproduce a set of
// literally hand-computed targets. Only once the control (reference == hand math)
// holds do we assert the instrument (SwipeActions.endDrag) reproduces the very
// same targets — proving the expected values are authored, not merely whatever
// the widget happens to emit.

// refSnap is the reference snap: it re-derives the target offset and which
// primary (if any) fires, independently of endDrag. tw/lw are the set widths,
// dThresh the destructive threshold, all in device pixels.
func refSnap(off, vel, proj float64, tw, lw, dThresh int, hasTrail, hasLead, destFull bool) (target float64, fired string) {
	projected := off + vel*proj
	switch {
	case off < 0 && hasTrail:
		mag := int(math.Round(-projected))
		if mag < 0 {
			mag = 0
		}
		switch {
		case destFull && mag >= dThresh:
			return 0, "trail"
		case mag >= tw/2:
			return -float64(tw), ""
		default:
			return 0, ""
		}
	case off > 0 && hasLead:
		mag := int(math.Round(projected))
		if mag < 0 {
			mag = 0
		}
		switch {
		case destFull && mag >= dThresh:
			return 0, "lead"
		case mag >= lw/2:
			return float64(lw), ""
		default:
			return 0, ""
		}
	default:
		return 0, ""
	}
}

func TestSwipeControlRunSnapReference(t *testing.T) {
	// tw = 3*72 = 216, lw = 2*72 = 144, dThresh = 300*3/4 = 225.
	const tw, lw, dThresh = 216, 144, 225
	cases := []struct {
		name       string
		off, vel   float64
		proj       float64
		wantTarget float64
		wantFired  string
	}{
		// Trailing side (off<0).
		{"trail open at exactly half", -108, 0, 0, -216, ""},
		{"trail closed just under half", -107, 0, 0, 0, ""},
		{"trail open above half", -180, 0, 0, -216, ""},
		{"trail destructive at threshold", -225, 0, 0, 0, "trail"},
		{"trail open just under destructive", -224, 0, 0, -216, ""},
		{"trail flick opens from shallow", -50, -2000, 0.05, -216, ""},
		// Leading side (off>0).
		{"lead open at exactly half", 72, 0, 0, 144, ""},
		{"lead closed just under half", 71, 0, 0, 0, ""},
		{"lead destructive at threshold", 225, 0, 0, 0, "lead"},
		// No reveal.
		{"neutral closes", 0, 0, 0, 0, ""},
	}
	for _, c := range cases {
		gotT, gotF := refSnap(c.off, c.vel, c.proj, tw, lw, dThresh, true, true, true)
		if gotT != c.wantTarget || gotF != c.wantFired {
			t.Errorf("%s: refSnap = (%v,%q), want (%v,%q)", c.name, gotT, gotF, c.wantTarget, c.wantFired)
		}
	}
}

// The instrument must reproduce the control's targets exactly.
func TestSwipeEndDragMatchesReference(t *testing.T) {
	resetSwipeGlobals(t)
	const tw, lw, dThresh = 216, 144, 225
	cases := []struct {
		off, vel, proj float64
	}{
		{-108, 0, 0}, {-107, 0, 0}, {-180, 0, 0}, {-225, 0, 0}, {-224, 0, 0},
		{-50, -2000, 0.05}, {72, 0, 0}, {71, 0, 0}, {225, 0, 0}, {0, 0, 0},
	}
	for _, c := range cases {
		fired := 0
		sa := NewSwipeActions(&swContent{})
		sa.SetBounds(Rect{W: swRowW, H: swRowH})
		sa.Projection = c.proj
		sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2", Destructive: true, OnInvoke: func() { fired++ }}}
		sa.Trailing = []SwipeAction{{Label: "A"}, {Label: "B"}, {Label: "C", Destructive: true, OnInvoke: func() { fired++ }}}
		sa.off = c.off
		sa.endDrag(c.vel)

		wantT, wantF := refSnap(c.off, c.vel, c.proj, tw, lw, dThresh, true, true, true)
		if sa.target != wantT {
			t.Errorf("off=%v vel=%v: target=%v, want %v", c.off, c.vel, sa.target, wantT)
		}
		wantFired := 0
		if wantF != "" {
			wantFired = 1
		}
		if fired != wantFired {
			t.Errorf("off=%v vel=%v: fired=%d, want %d", c.off, c.vel, fired, wantFired)
		}
	}
}

// --- geometry --------------------------------------------------------------

func TestSwipeGeometryExact(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2"}}

	if got := sa.actionWidth(); got != swAW {
		t.Fatalf("actionWidth = %d, want %d", got, swAW)
	}
	if got := sa.trailingWidth(); got != 3*swAW {
		t.Fatalf("trailingWidth = %d, want %d", got, 3*swAW)
	}
	if got := sa.leadingWidth(); got != 2*swAW {
		t.Fatalf("leadingWidth = %d, want %d", got, 2*swAW)
	}
	if got := sa.destructiveThreshold(); got != 225 {
		t.Fatalf("destructiveThreshold = %d, want 225", got)
	}
	// Trailing home lanes at off=-216: 84,156,228 each 72 wide, last edge 300.
	wantTrail := []Rect{{X: 84, W: 72, H: 40}, {X: 156, W: 72, H: 40}, {X: 228, W: 72, H: 40}}
	for i, w := range wantTrail {
		if got := sa.trailingLaneRect(i, -3*swAW); got != w {
			t.Errorf("trailing lane %d = %+v, want %+v", i, got, w)
		}
	}
	// Leading home lanes at off=+144: 0,72.
	wantLead := []Rect{{W: 72, H: 40}, {X: 72, W: 72, H: 40}}
	for i, w := range wantLead {
		if got := sa.leadingLaneRect(i, 2*swAW); got != w {
			t.Errorf("leading lane %d = %+v, want %+v", i, got, w)
		}
	}
}

func TestSwipeActionWidthClampsNegative(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(nil)
	sa.ActionWidth = -5
	if got := sa.actionWidth(); got != 0 {
		t.Fatalf("actionWidth(-5) = %d, want 0", got)
	}
}

func TestSwipeActionWidthTouchFloor(t *testing.T) {
	resetSwipeGlobals(t)
	SetDensity(DensityTouch)
	sa := NewSwipeActions(nil)
	sa.ActionWidth = 20 // scaled(20) = 30 (×1.5), below the 44 touch floor
	if got := sa.actionWidth(); got != 44 {
		t.Fatalf("touch actionWidth = %d, want the 44px floor", got)
	}
}

func TestSwipeDestructiveThresholdDisabled(t *testing.T) {
	sa := NewSwipeActions(nil)
	sa.DestructiveDen = 0
	if got := sa.destructiveThreshold(); got != math.MaxInt {
		t.Fatalf("threshold with zero denominator = %d, want MaxInt", got)
	}
}

func TestSwipePrimaryIndex(t *testing.T) {
	sa := NewSwipeActions(nil)
	// Empty sets: no primary.
	if got := sa.primaryTrailing(); got != -1 {
		t.Fatalf("empty primaryTrailing = %d, want -1", got)
	}
	if got := sa.primaryLeading(); got != -1 {
		t.Fatalf("empty primaryLeading = %d, want -1", got)
	}
	// Unflagged: edge action (rightmost trailing, leftmost leading).
	sa.Trailing = []SwipeAction{{Label: "A"}, {Label: "B"}}
	sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2"}}
	if got := sa.primaryTrailing(); got != 1 {
		t.Fatalf("unflagged primaryTrailing = %d, want 1 (edge)", got)
	}
	if got := sa.primaryLeading(); got != 0 {
		t.Fatalf("unflagged primaryLeading = %d, want 0 (edge)", got)
	}
	// Flagged wins over the edge.
	sa.Trailing[0].Destructive = true
	sa.Leading[1].Destructive = true
	if got := sa.primaryTrailing(); got != 0 {
		t.Fatalf("flagged primaryTrailing = %d, want 0", got)
	}
	if got := sa.primaryLeading(); got != 1 {
		t.Fatalf("flagged primaryLeading = %d, want 1", got)
	}
}

// --- open / close / settle -------------------------------------------------

func TestSwipeOpenTrailingSettlesExact(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OpenTrailing()
	if sa.State() != SwipeTrailingOpen {
		t.Fatalf("state = %v, want TrailingOpen", sa.State())
	}
	if !sa.Settling() {
		t.Fatal("expected a settle in flight after OpenTrailing")
	}
	settle(sa)
	if sa.Settling() {
		t.Fatal("still settling after the settle loop")
	}
	if sa.off != -216 || sa.Offset() != -216 {
		t.Fatalf("rest offset = %v, want exactly -216", sa.off)
	}
}

func TestSwipeOpenLeadingSettlesExact(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2"}}
	sa.OpenLeading()
	settle(sa)
	if sa.State() != SwipeLeadingOpen || sa.off != 144 {
		t.Fatalf("leading rest = (%v, %v), want (LeadingOpen, 144)", sa.State(), sa.off)
	}
}

func TestSwipeCloseSettlesToZero(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	sa.Close()
	settle(sa)
	if sa.State() != SwipeClosed || sa.off != 0 {
		t.Fatalf("close rest = (%v, %v), want (Closed, 0)", sa.State(), sa.off)
	}
}

func TestSwipeOpenEmptySetIsNoOp(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(nil)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.OpenTrailing()
	sa.OpenLeading()
	if sa.Settling() || sa.State() != SwipeClosed {
		t.Fatalf("empty-set open changed state to %v / settling=%v", sa.State(), sa.Settling())
	}
}

func TestSwipeTickWithoutSettleIsNoOp(t *testing.T) {
	sa := NewSwipeActions(nil)
	if sa.Tick(1.0 / 60.0) {
		t.Fatal("Tick with no settle reported still-settling")
	}
}

// The settle springs onto the target rather than jumping there instantly — it
// takes more than one tick and never overshoots past the target.
func TestSwipeSettleAnimatesMonotonically(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OpenTrailing() // target -216 from 0
	prev := 0.0
	ticks := 0
	for sa.Settling() && ticks < 100000 {
		sa.Tick(1.0 / 60.0)
		if sa.off < -216-1e-6 {
			t.Fatalf("offset %v overshot past the target -216", sa.off)
		}
		if sa.off > prev+1e-9 {
			t.Fatalf("offset moved backwards: %v then %v", prev, sa.off)
		}
		prev = sa.off
		ticks++
	}
	if ticks < 2 {
		t.Fatalf("settle finished in %d ticks; it should animate, not snap", ticks)
	}
}

// --- drag reveal -----------------------------------------------------------

func touch(kind EventKind, x, y int) Event { return Event{Kind: kind, X: x, Y: y, Code: "t1"} }

func TestSwipeDragRevealsOneForOne(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchStart, 200, 20))
	sa.OnEvent(touch(EventTouchMove, 120, 20)) // dragged left 80
	if sa.off != -80 {
		t.Fatalf("offset after 80px left drag = %v, want -80 (one-for-one)", sa.off)
	}
	sa.OnEvent(touch(EventTouchMove, 100, 20)) // 20 more
	if sa.off != -100 {
		t.Fatalf("offset = %v, want -100", sa.off)
	}
}

// A pull past the far bound rubber-bands (resisted), not one-for-one.
func TestSwipeDragRubberBandsPastBound(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchStart, 200, 20))
	sa.OnEvent(touch(EventTouchMove, -200, 20)) // raw -400, past min -300
	over := 100.0                               // -300 - (-400)
	want := -300.0 - over/(1+over/momentumMaxOverscroll)
	if math.Abs(sa.off-want) > 1e-9 {
		t.Fatalf("rubber-banded offset = %v, want %v", sa.off, want)
	}
	if sa.off <= -400 || sa.off >= -300 {
		t.Fatalf("offset %v is not resisted between -400 and -300", sa.off)
	}
}

// A whole synthetic swipe: drag past the open threshold, release, settle open.
func TestSwipeGestureOpensAndSettles(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.Projection = 0 // decide on resting offset alone
	sa.OnEvent(touch(EventTouchStart, 200, 20))
	sa.OnEvent(touch(EventTouchMove, 80, 20)) // -120, past half (108)
	sa.OnEvent(touch(EventTouchEnd, 80, 20))
	if sa.State() != SwipeTrailingOpen {
		t.Fatalf("state after release = %v, want TrailingOpen", sa.State())
	}
	settle(sa)
	if sa.off != -216 {
		t.Fatalf("settled offset = %v, want -216", sa.off)
	}
}

// A dominantly vertical drag is disowned so a list can scroll through the row.
func TestSwipeVerticalDragIsDisowned(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchStart, 150, 5))
	sa.OnEvent(touch(EventTouchMove, 156, 60)) // dy 55 >> dx 6
	if sa.off != 0 {
		t.Fatalf("vertical drag moved the reveal to %v, want 0", sa.off)
	}
	sa.OnEvent(touch(EventTouchEnd, 156, 60))
	if sa.State() != SwipeClosed || sa.Settling() {
		t.Fatalf("vertical drag changed state to %v (settling=%v)", sa.State(), sa.Settling())
	}
}

// A vertical bail from an already-open row restores the open resting offset.
func TestSwipeVerticalBailRestoresOpenState(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	sa.OnEvent(touch(EventTouchStart, 150, 20))
	sa.OnEvent(touch(EventTouchMove, 152, 80)) // vertical
	if sa.off != -216 {
		t.Fatalf("bail restored offset to %v, want -216", sa.off)
	}
}

// Below the slop, the orientation stays undecided and nothing moves.
func TestSwipeTinyMoveStaysUndecided(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchStart, 150, 20))
	sa.OnEvent(touch(EventTouchMove, 152, 21)) // < slop on both axes
	if sa.oriented || sa.off != 0 {
		t.Fatalf("tiny move decided orientation (%v) or moved (%v)", sa.oriented, sa.off)
	}
}

func TestSwipeMoveWithoutPressIgnored(t *testing.T) {
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchMove, 100, 20))
	if sa.off != 0 {
		t.Fatalf("orphan move changed offset to %v", sa.off)
	}
}

func TestSwipeReleaseWithoutPressIgnored(t *testing.T) {
	sa := newTrailSA(nil)
	sa.OnEvent(touch(EventTouchEnd, 100, 20)) // no press, no vertical, not dragging
	if sa.Settling() {
		t.Fatal("orphan release started a settle")
	}
}

// --- snap thresholds (exact) -----------------------------------------------

func TestSwipeSnapThresholdsExact(t *testing.T) {
	resetSwipeGlobals(t)
	cases := []struct {
		name       string
		off        float64
		wantTarget float64
		wantState  SwipeOpenState
		wantFired  int
	}{
		{"open at exactly half", -108, -216, SwipeTrailingOpen, 0},
		{"closed just under half", -107, 0, SwipeClosed, 0},
		{"destructive at threshold", -225, 0, SwipeClosed, 1},
		{"open just under destructive", -224, -216, SwipeTrailingOpen, 0},
	}
	for _, c := range cases {
		fired := 0
		sa := newTrailSA(func() { fired++ })
		sa.Projection = 0
		sa.off = c.off
		sa.endDrag(0)
		if sa.target != c.wantTarget {
			t.Errorf("%s: target=%v, want %v", c.name, sa.target, c.wantTarget)
		}
		if sa.State() != c.wantState {
			t.Errorf("%s: state=%v, want %v", c.name, sa.State(), c.wantState)
		}
		if fired != c.wantFired {
			t.Errorf("%s: fired=%d, want %d", c.name, fired, c.wantFired)
		}
	}
}

func TestSwipeEndDragDefaultCloses(t *testing.T) {
	resetSwipeGlobals(t)
	// off is negative but there are no trailing actions -> default branch.
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L"}}
	sa.off = -50
	sa.endDrag(0)
	if sa.target != 0 || sa.State() != SwipeClosed {
		t.Fatalf("default branch target=%v state=%v, want 0/Closed", sa.target, sa.State())
	}
}

func TestSwipeLeadingSnapAndDestructive(t *testing.T) {
	resetSwipeGlobals(t)
	// Leading open at half.
	fired := 0
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2", Destructive: true, OnInvoke: func() { fired++ }}}
	sa.off = 72 // lw/2
	sa.endDrag(0)
	if sa.target != 144 || sa.State() != SwipeLeadingOpen {
		t.Fatalf("leading half: target=%v state=%v, want 144/LeadingOpen", sa.target, sa.State())
	}
	// Leading destructive.
	fired = 0
	sa.off = 225
	sa.endDrag(0)
	if sa.target != 0 || fired != 1 || sa.State() != SwipeClosed {
		t.Fatalf("leading destructive: target=%v fired=%d state=%v", sa.target, fired, sa.State())
	}
}

// --- invoke fires exactly once + closes ------------------------------------

func TestSwipeInvokeTrailingFiresOnceAndCloses(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := newTrailSA(func() { fired++ })
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	sa.InvokeTrailing(2)
	if fired != 1 {
		t.Fatalf("InvokeTrailing fired %d times, want exactly 1", fired)
	}
	if sa.State() != SwipeClosed {
		t.Fatalf("state after invoke = %v, want Closed", sa.State())
	}
}

func TestSwipeInvokeOutOfRangeIgnored(t *testing.T) {
	sa := newTrailSA(nil)
	sa.InvokeTrailing(-1)
	sa.InvokeTrailing(99)
	sa.InvokeLeading(0) // no leading actions
	if sa.Settling() {
		t.Fatal("out-of-range invoke started a settle")
	}
}

func TestSwipeInvokeNilCallbackSafe(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Trailing = []SwipeAction{{Label: "NoOp"}} // OnInvoke nil
	sa.InvokeTrailing(0)                         // must not panic
	if sa.State() != SwipeClosed {
		t.Fatalf("state = %v, want Closed", sa.State())
	}
}

// A tap on a revealed lane invokes exactly that action; a tap elsewhere closes.
func TestSwipeTapOnLaneInvokes(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := newTrailSA(nil)
	sa.Trailing[0].OnInvoke = func() { fired++ }
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	// Lane 0 home rect is x in [84,156). Tap its centre via a press/release pair.
	sa.OnEvent(touch(EventTouchStart, 120, 20))
	sa.OnEvent(touch(EventTouchEnd, 120, 20))
	if fired != 1 || sa.State() != SwipeClosed {
		t.Fatalf("lane tap: fired=%d state=%v, want 1/Closed", fired, sa.State())
	}
}

func TestSwipeTapOffLaneCloses(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := newTrailSA(func() { fired++ })
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	sa.OnEvent(touch(EventTouchStart, 10, 20)) // content area, not a lane
	sa.OnEvent(touch(EventTouchEnd, 10, 20))
	if fired != 0 || sa.State() != SwipeClosed {
		t.Fatalf("off-lane tap: fired=%d state=%v, want 0/Closed", fired, sa.State())
	}
}

func TestSwipeTapOnLeadingLaneInvokes(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1", OnInvoke: func() { fired++ }}, {Label: "L2"}}
	sa.off = 144
	sa.setState(SwipeLeadingOpen)
	sa.OnEvent(touch(EventTouchStart, 36, 20)) // lane 0 in [0,72)
	sa.OnEvent(touch(EventTouchEnd, 36, 20))
	if fired != 1 || sa.State() != SwipeClosed {
		t.Fatalf("leading lane tap: fired=%d state=%v", fired, sa.State())
	}
}

func TestSwipeTapOffLeadingLaneCloses(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1"}}
	sa.off = 72
	sa.setState(SwipeLeadingOpen)
	sa.OnEvent(touch(EventTouchStart, 250, 20)) // far from the leading lane
	sa.OnEvent(touch(EventTouchEnd, 250, 20))
	if sa.State() != SwipeClosed {
		t.Fatalf("leading off-lane tap state = %v, want Closed", sa.State())
	}
}

// The child action button carries the SAME invoke path — the a11y-invoke route.
func TestSwipeA11yButtonInvokes(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := newTrailSA(func() { fired++ })
	sa.layout()
	// Dispatch a click straight to the accessibility vehicle for action 2.
	sa.trailBtns[2].OnEvent(Event{Kind: EventClick})
	if fired != 1 || sa.State() != SwipeClosed {
		t.Fatalf("a11y button invoke: fired=%d state=%v, want 1/Closed", fired, sa.State())
	}
}

// --- accessibility ---------------------------------------------------------

func TestSwipeA11yStateValue(t *testing.T) {
	sa := newTrailSA(nil)
	if got := sa.A11y(); got.Role != RoleGroup || got.Value != "closed" {
		t.Fatalf("closed A11y = %+v, want group/closed", got)
	}
	sa.setState(SwipeTrailingOpen)
	if got := sa.A11y().Value; got != "trailing actions revealed" {
		t.Fatalf("trailing A11y value = %q", got)
	}
	sa.setState(SwipeLeadingOpen)
	if got := sa.A11y().Value; got != "leading actions revealed" {
		t.Fatalf("leading A11y value = %q", got)
	}
}

func TestSwipeChildrenExposeActionsAtHomeRects(t *testing.T) {
	resetSwipeGlobals(t)
	label := NewLabel("Row")
	sa := NewSwipeActions(label)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "Archive"}}
	sa.Trailing = []SwipeAction{{Label: "A"}, {Label: "B"}, {Label: "C"}}

	kids := sa.Children()
	// content + 1 leading + 3 trailing = 5
	if len(kids) != 5 {
		t.Fatalf("Children len = %d, want 5", len(kids))
	}
	if kids[0] != Widget(label) {
		t.Fatalf("first child is not the content")
	}
	// The trailing buttons sit at their fully-open home rects.
	wantTrail := []Rect{{X: 84, W: 72, H: 40}, {X: 156, W: 72, H: 40}, {X: 228, W: 72, H: 40}}
	for i, w := range wantTrail {
		if got := sa.trailBtns[i].Bounds(); got != w {
			t.Errorf("trailing button %d bounds = %+v, want %+v", i, got, w)
		}
	}
	// WalkA11y reports each action as a named button.
	names := map[string]bool{}
	buttons := 0
	for _, n := range WalkA11y(sa) {
		if n.Role == RoleButton {
			buttons++
			names[n.Name] = true
		}
	}
	if buttons != 4 {
		t.Fatalf("WalkA11y found %d button nodes, want 4", buttons)
	}
	for _, want := range []string{"Archive", "A", "B", "C"} {
		if !names[want] {
			t.Errorf("action %q missing from the a11y tree", want)
		}
	}
}

func TestSwipeChildrenNilContentSkipped(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(nil)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Trailing = []SwipeAction{{Label: "A"}}
	kids := sa.Children()
	if len(kids) != 1 {
		t.Fatalf("nil-content Children len = %d, want 1 (the action)", len(kids))
	}
	for i, k := range kids {
		if k == nil {
			t.Fatalf("child %d is nil", i)
		}
	}
}

// syncButtons rebuilds on a count change and refreshes labels on a match.
func TestSwipeSyncButtonsRebuildAndRefresh(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.layout()
	first := sa.trailBtns[0]
	sa.layout() // same count: keep instances, refresh
	if sa.trailBtns[0] != first {
		t.Fatal("same-count layout rebuilt the buttons instead of reusing them")
	}
	sa.Trailing[0].Label = "Renamed"
	sa.layout()
	if sa.trailBtns[0].Label().Get() != "Renamed" {
		t.Fatalf("button label not refreshed: %q", sa.trailBtns[0].Label().Get())
	}
	// Count change rebuilds.
	sa.Trailing = sa.Trailing[:2]
	sa.layout()
	if len(sa.trailBtns) != 2 {
		t.Fatalf("after shrink, buttons = %d, want 2", len(sa.trailBtns))
	}
}

// --- drawing ---------------------------------------------------------------

func TestSwipeDrawClosedOnlyContent(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	// No lane colours should appear when shut.
	for _, c := range []RGBA{RGB(1, 0, 0), RGB(2, 0, 0), RGB(3, 0, 0)} {
		if _, ok := rec.findFill(c); ok {
			t.Fatalf("lane colour %v painted while shut", c)
		}
	}
	// Content (white) painted at the row, unshifted.
	if r, ok := rec.findFill(RGB(0xFF, 0xFF, 0xFF)); !ok || r != (Rect{W: 300, H: 40}) {
		t.Fatalf("content fill = %+v (ok=%v), want the whole row", r, ok)
	}
	// Clip pushed and popped exactly once.
	if len(rec.clips) != 1 || rec.clips[0] != (Rect{W: 300, H: 40}) || rec.popped != 1 {
		t.Fatalf("clip = %+v pops=%d, want one push of the row + one pop", rec.clips, rec.popped)
	}
}

func TestSwipeDrawOpenTrailingLanes(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -216 // fully open, not primed (216 < 225)
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	wantLanes := map[RGBA]Rect{
		RGB(1, 0, 0): {X: 84, W: 72, H: 40},
		RGB(2, 0, 0): {X: 156, W: 72, H: 40},
		RGB(3, 0, 0): {X: 228, W: 72, H: 40},
	}
	for c, w := range wantLanes {
		if r, ok := rec.findFill(c); !ok || r != w {
			t.Errorf("lane %v = %+v (ok=%v), want %+v", c, r, ok, w)
		}
	}
	// Content shifted left by 216.
	if r, _ := rec.findFill(RGB(0xFF, 0xFF, 0xFF)); r != (Rect{X: -216, W: 300, H: 40}) {
		t.Fatalf("content fill = %+v, want shifted to x=-216", r)
	}
}

func TestSwipeDrawPrimedDestructiveStrip(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -260 // 260 >= 225 -> primed; primary is action C (colour 3,0,0)
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	// The primary colour fills the whole revealed strip [300-260 .. 300] = {40,0,260,40}.
	if r, ok := rec.findFill(RGB(3, 0, 0)); !ok || r != (Rect{X: 40, W: 260, H: 40}) {
		t.Fatalf("primed strip = %+v (ok=%v), want {40,0,260,40}", r, ok)
	}
	// The non-primary lane colours must NOT appear.
	if _, ok := rec.findFill(RGB(1, 0, 0)); ok {
		t.Fatal("a non-primary lane painted while primed")
	}
}

func TestSwipeDrawPrimedLeadingStrip(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1", Color: RGB(7, 0, 0)}, {Label: "L2", Color: RGB(8, 0, 0)}}
	sa.off = 240 // >= 225 -> primed; leading primary is index 0 (colour 7,0,0)
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	if r, ok := rec.findFill(RGB(7, 0, 0)); !ok || r != (Rect{W: 240, H: 40}) {
		t.Fatalf("leading primed strip = %+v (ok=%v), want {0,0,240,40}", r, ok)
	}
}

func TestSwipeDrawNilContentNoPanic(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(nil)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Trailing = []SwipeAction{{Label: "A", Color: RGB(1, 0, 0)}}
	sa.off = -72
	sa.Draw(&swipeRec{}, DefaultLight())
}

func TestSwipeDrawNonClipperPainter(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -216
	// plainPainter implements Painter but NOT Clipper: the fallback must still draw.
	p := &plainPainter{w: 300, h: 40}
	sa.Draw(p, DefaultLight())
	if len(p.drawn) == 0 {
		t.Fatal("nothing drawn on a non-clipping painter")
	}
}

// paintLane handles an icon action, a label action and a bare action.
func TestSwipePaintLaneVariants(t *testing.T) {
	resetSwipeGlobals(t)
	iconCalls := 0
	sa := NewSwipeActions(nil)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Trailing = []SwipeAction{
		{Icon: func(painter.Painter, Rect, RGBA) { iconCalls++ }, Color: RGB(1, 0, 0)},
		{Label: "Text", Color: RGB(2, 0, 0)},
		{Color: RGB(3, 0, 0)}, // neither icon nor label
	}
	sa.off = -216
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	if iconCalls != 1 {
		t.Fatalf("icon callback called %d times, want 1", iconCalls)
	}
	found := false
	for _, s := range rec.texts {
		if s == "Text" {
			found = true
		}
	}
	if !found {
		t.Fatal("label action did not draw its text")
	}
}

func TestSwipeLaneColorAndInk(t *testing.T) {
	th := DefaultLight()
	// Explicit colour wins.
	if got := laneColor(th, SwipeAction{Color: RGB(9, 9, 9)}); got != RGB(9, 9, 9) {
		t.Fatalf("explicit lane colour = %v", got)
	}
	// Destructive default = danger red.
	if got := laneColor(th, SwipeAction{Destructive: true}); got != dangerInk {
		t.Fatalf("destructive default colour = %v, want dangerInk", got)
	}
	// Plain default = accent.
	if got := laneColor(th, SwipeAction{}); got != th.Accent {
		t.Fatalf("plain default colour = %v, want accent", got)
	}
	// Ink: explicit vs white default.
	if got := laneInk(SwipeAction{Ink: RGB(1, 2, 3)}); got != RGB(1, 2, 3) {
		t.Fatalf("explicit ink = %v", got)
	}
	if got := laneInk(SwipeAction{}); got != RGB(0xFF, 0xFF, 0xFF) {
		t.Fatalf("default ink = %v, want white", got)
	}
}

// --- forwarding / misc events ----------------------------------------------

func TestSwipeForwardsToContentWhenClosed(t *testing.T) {
	resetSwipeGlobals(t)
	rec := &swContent{}
	sa := NewSwipeActions(rec)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.OnEvent(Event{Kind: EventScroll, X: 50, Y: 5, Delta: 2})
	if len(rec.events) != 1 || rec.events[0].Kind != EventScroll || rec.events[0].X != 50 {
		t.Fatalf("scroll not forwarded verbatim: %+v", rec.events)
	}
}

func TestSwipeForwardTranslatesByOffset(t *testing.T) {
	resetSwipeGlobals(t)
	rec := &swContent{}
	sa := NewSwipeActions(rec)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.off = 30 // content shifted right 30
	sa.OnEvent(Event{Kind: EventScroll, X: 50, Y: 5})
	if rec.events[0].X != 20 {
		t.Fatalf("forwarded X = %d, want 50-30=20", rec.events[0].X)
	}
}

func TestSwipeForwardNilContentSafe(t *testing.T) {
	sa := NewSwipeActions(nil)
	sa.OnEvent(Event{Kind: EventScroll, X: 1, Y: 1}) // must not panic
}

func TestSwipeClosedTapForwardsClick(t *testing.T) {
	resetSwipeGlobals(t)
	rec := &swContent{}
	sa := NewSwipeActions(rec)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.OnEvent(touch(EventTouchStart, 40, 20))
	sa.OnEvent(touch(EventTouchEnd, 40, 20)) // tap, closed -> forward click
	var click *Event
	for i := range rec.events {
		if rec.events[i].Kind == EventClick {
			click = &rec.events[i]
		}
	}
	if click == nil || click.X != 40 {
		t.Fatalf("closed tap did not forward a click at x=40: %+v", rec.events)
	}
}

func TestSwipeEscapeClosesWhenOpen(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.off = -216
	sa.setState(SwipeTrailingOpen)
	sa.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if sa.State() != SwipeClosed {
		t.Fatalf("Escape did not close: state=%v", sa.State())
	}
}

func TestSwipeKeyDownNonEscapeForwarded(t *testing.T) {
	resetSwipeGlobals(t)
	rec := &swContent{}
	sa := NewSwipeActions(rec)
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(rec.events) != 1 || rec.events[0].Code != "Enter" {
		t.Fatalf("non-Escape key not forwarded: %+v", rec.events)
	}
}

func TestSwipeMouseDragPathOpens(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.Projection = 0
	// Desktop mouse sequence: EventClick press, EventMouseDrag, EventMouseUp.
	sa.OnEvent(Event{Kind: EventClick, X: 200, Y: 20})
	sa.OnEvent(Event{Kind: EventMouseDrag, X: 80, Y: 20})
	sa.OnEvent(Event{Kind: EventMouseUp, X: 80, Y: 20})
	if sa.State() != SwipeTrailingOpen {
		t.Fatalf("mouse drag did not open: state=%v", sa.State())
	}
}

func TestSwipeDisabledIgnoresEvents(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.Disabled().Set(true)
	sa.OnEvent(touch(EventTouchStart, 200, 20))
	sa.OnEvent(touch(EventTouchMove, 80, 20))
	sa.OnEvent(touch(EventTouchEnd, 80, 20))
	if sa.off != 0 || sa.Settling() || sa.State() != SwipeClosed {
		t.Fatalf("disabled row reacted: off=%v settling=%v state=%v", sa.off, sa.Settling(), sa.State())
	}
}

// --- accessors -------------------------------------------------------------

func TestSwipeZeroValueAccessors(t *testing.T) {
	sa := &SwipeActions{} // no NewSwipeActions: open observable is nil
	if sa.State() != SwipeClosed {
		t.Fatalf("zero-value State = %v, want Closed", sa.State())
	}
	if sa.IsOpen() {
		t.Fatal("zero-value IsOpen = true")
	}
	if sa.Open() == nil {
		t.Fatal("Open() returned nil after lazy allocation")
	}
}

func TestSwipeRestingOffset(t *testing.T) {
	resetSwipeGlobals(t)
	sa := newTrailSA(nil)
	sa.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2"}}
	if got := sa.restingOffset(SwipeTrailingOpen); got != -216 {
		t.Fatalf("resting trailing = %v, want -216", got)
	}
	if got := sa.restingOffset(SwipeLeadingOpen); got != 144 {
		t.Fatalf("resting leading = %v, want 144", got)
	}
	if got := sa.restingOffset(SwipeClosed); got != 0 {
		t.Fatalf("resting closed = %v, want 0", got)
	}
}

// drawSet returns immediately for an empty set even when the offset points its
// way (a swipe toward a side that has no actions).
func TestSwipeDrawEmptySetNoLanes(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L"}}
	sa.off = -50 // trailing side, but Trailing is empty
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight()) // must not paint any trailing lane / panic
	if _, ok := rec.findFill(RGB(1, 0, 0)); ok {
		t.Fatal("painted a lane for an empty trailing set")
	}
}

// A non-primed leading reveal paints each leading lane at its anchored position.
func TestSwipeDrawOpenLeadingLanes(t *testing.T) {
	resetSwipeGlobals(t)
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "L1", Color: RGB(5, 0, 0)}, {Label: "L2", Color: RGB(6, 0, 0)}}
	sa.off = 144 // fully open, below destructive (225): non-primed loop
	rec := &swipeRec{}
	sa.Draw(rec, DefaultLight())
	if r, ok := rec.findFill(RGB(5, 0, 0)); !ok || r != (Rect{W: 72, H: 40}) {
		t.Fatalf("leading lane 0 = %+v (ok=%v), want {0,0,72,40}", r, ok)
	}
	if r, ok := rec.findFill(RGB(6, 0, 0)); !ok || r != (Rect{X: 72, W: 72, H: 40}) {
		t.Fatalf("leading lane 1 = %+v (ok=%v), want {72,0,72,40}", r, ok)
	}
}

// The leading child button carries the InvokeLeading path (the a11y route).
func TestSwipeLeadingA11yButtonInvokes(t *testing.T) {
	resetSwipeGlobals(t)
	fired := 0
	sa := NewSwipeActions(&swContent{})
	sa.SetBounds(Rect{W: swRowW, H: swRowH})
	sa.Leading = []SwipeAction{{Label: "Archive", OnInvoke: func() { fired++ }}}
	sa.layout()
	sa.leadBtns[0].OnEvent(Event{Kind: EventClick})
	if fired != 1 || sa.State() != SwipeClosed {
		t.Fatalf("leading a11y button: fired=%d state=%v, want 1/Closed", fired, sa.State())
	}
}

// A release velocity that projects the reveal back past zero clamps the snap
// magnitude at 0 (it does not go negative) on both sides.
func TestSwipeProjectionClampsNegativeMagnitude(t *testing.T) {
	resetSwipeGlobals(t)
	// Trailing: shallow open, flicked hard the CLOSING way -> projected positive.
	sa := newTrailSA(nil)
	sa.Projection = 0.05
	sa.off = -50
	sa.endDrag(2000) // projected = -50 + 100 = +50 -> mag clamps to 0 -> close
	if sa.target != 0 || sa.State() != SwipeClosed {
		t.Fatalf("trailing closing flick: target=%v state=%v, want 0/Closed", sa.target, sa.State())
	}
	// Leading: mirror.
	sa2 := NewSwipeActions(&swContent{})
	sa2.SetBounds(Rect{W: swRowW, H: swRowH})
	sa2.Leading = []SwipeAction{{Label: "L1"}, {Label: "L2"}}
	sa2.Projection = 0.05
	sa2.off = 50
	sa2.endDrag(-2000) // projected = 50 - 100 = -50 -> mag clamps to 0 -> close
	if sa2.target != 0 || sa2.State() != SwipeClosed {
		t.Fatalf("leading closing flick: target=%v state=%v, want 0/Closed", sa2.target, sa2.State())
	}
}

func TestSwipeDragBoundsByPresentSets(t *testing.T) {
	resetSwipeGlobals(t)
	// Only trailing: min = -W, max = 0.
	sa := newTrailSA(nil)
	if lo, hi := sa.dragMin(), sa.dragMax(); lo != -300 || hi != 0 {
		t.Fatalf("trailing-only drag bounds = [%v,%v], want [-300,0]", lo, hi)
	}
	// Only leading: min = 0, max = W.
	sa2 := NewSwipeActions(nil)
	sa2.SetBounds(Rect{W: swRowW, H: swRowH})
	sa2.Leading = []SwipeAction{{Label: "L"}}
	if lo, hi := sa2.dragMin(), sa2.dragMax(); lo != 0 || hi != 300 {
		t.Fatalf("leading-only drag bounds = [%v,%v], want [0,300]", lo, hi)
	}
}

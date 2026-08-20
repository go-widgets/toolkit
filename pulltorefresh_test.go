// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// PullToRefresh reuses exactEq (momentum_test.go) for bit-exact float assertions:
// the pull distance is a deterministic function of the drag, so the tests pin
// the exact value, not an approximation.

// --- Control run -----------------------------------------------------------
//
// Before trusting the widget's exact-value assertions, validate the METHOD. The
// pull distance is the rubber-band curve of the embedded Momentum engine with
// bounds [0,0]: for a net downward finger travel `raw`, the revealed pull is
//
//	pull = raw / (1 + raw/M)
//
// where M is the rubber asymptote (effMax). refPull is an INDEPENDENT reference
// of that curve; the control below proves refPull reproduces a set of literally
// hand-computed pull distances, and only then do the widget tests assert the
// widget reproduces refPull. This proves the expected values are authored, not
// merely whatever the widget happens to emit.

// refPull is the reference rubber-band curve, spelled out independently of
// Momentum. A non-positive raw reveals no pull (the finger is at or above home).
func refPull(raw, m float64) float64 {
	if raw <= 0 {
		return 0
	}
	return raw / (1 + raw/m)
}

func TestPullToRefreshControlRunRubberCurve(t *testing.T) {
	// Hand-computed with M = 160 (the compact/1x effMax):
	//   raw   0 -> 0
	//   raw  40 -> 40/(1+40/160)   = 40/1.25 = 32
	//   raw 160 -> 160/(1+160/160) = 160/2   = 80
	//   raw 480 -> 480/(1+480/160) = 480/4   = 120
	const m = 160
	cases := []struct {
		raw, want float64
	}{
		{0, 0}, {40, 32}, {160, 80}, {480, 120},
	}
	for _, c := range cases {
		// Control: the independent reference reproduces the hand math exactly.
		if got := refPull(c.raw, m); got != c.want {
			t.Fatalf("refPull(%v): got %v, want %v (hand-computed)", c.raw, got, c.want)
		}
	}

	// Instrument: the widget, driven to each raw drag, reproduces the reference
	// bit-for-bit. Density is the compact 1x baseline so effMax == 160.
	defer restoreDensity()
	restoreDensity()
	for _, c := range cases[1:] { // raw 0 begins no pull (needs a downward move)
		w := NewPullToRefresh(nil)
		w.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
		if got := w.effMax(); got != m {
			t.Fatalf("effMax at compact/1x = %d, want %d", got, m)
		}
		w.TouchDown(Event{Kind: EventTouchStart, Y: 0})
		w.TouchMove(Event{Kind: EventTouchMove, Y: int(c.raw)})
		exactEq(t, w.Pull(), refPull(c.raw, m), "widget pull for raw=%v", c.raw)
		exactEq(t, w.Pull(), c.want, "widget pull literal for raw=%v", c.raw)
	}
}

// --- helpers ---------------------------------------------------------------

// newPTR returns a compact/1x PullToRefresh with the given child at a fixed
// bounds. The caller must restoreDensity().
func newPTR(child Widget) *PullToRefresh {
	w := NewPullToRefresh(child)
	w.SetBounds(Rect{X: 10, Y: 20, W: 200, H: 300})
	return w
}

// pull drives a fresh downward drag from Y=0 to Y=d and returns the widget.
func pull(w *PullToRefresh, d int) {
	w.TouchDown(Event{Kind: EventTouchStart, Y: 0})
	w.TouchMove(Event{Kind: EventTouchMove, Y: d})
}

// settleSpring ticks until the embedded spring has come to rest (bounded), so a
// test can assert the exact resting pull distance.
func settleSpring(t *testing.T, w *PullToRefresh) {
	t.Helper()
	for i := 0; i < 100000; i++ {
		if !w.spring.Settling() {
			return
		}
		w.Tick(1.0 / 60)
	}
	t.Fatalf("spring never settled")
}

// fillChild fills its whole bounds with col, so a test can locate where the
// content was painted (and therefore how far it was pushed down).
type fillChild struct {
	Base
	col RGBA
}

func (c *fillChild) Draw(p painter.Painter, _ *Theme) {
	r := c.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, c.col)
}

// eventChild records the last event it received, for forwarding tests.
type eventChild struct {
	Base
	last   Event
	gotAny bool
}

func (c *eventChild) OnEvent(ev Event) { c.last, c.gotAny = ev, true }

// recPainter implements painter.Painter but NOT Translator/Clipper, so Draw
// takes its fallback (offset-the-bounds) path.
type recPainter struct{ fills []painter.Rect }

func (r *recPainter) FillRect(rc painter.Rect, _ RGBA)             {}
func (r *recPainter) StrokeRect(painter.Rect, RGBA, int)           {}
func (r *recPainter) FillRoundRect(painter.Rect, int, RGBA)        {}
func (r *recPainter) StrokeRoundRect(painter.Rect, int, RGBA, int) {}
func (r *recPainter) PutPixel(int, int, RGBA)                      {}
func (r *recPainter) Text(int, int, string, RGBA)                  {}
func (r *recPainter) Size() (int, int)                             { return 1000, 1000 }

// recFillChild is like fillChild but records the exact rect it was drawn at,
// which reveals the fallback offset applied to its bounds.
type recFillChild struct {
	Base
	drawnAt Rect
}

func (c *recFillChild) Draw(p painter.Painter, _ *Theme) { c.drawnAt = c.Bounds() }

// --- device metrics at compact/1x ------------------------------------------

func TestPullToRefreshCompactMetrics(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := NewPullToRefresh(nil)
	if got := w.effThreshold(); got != 64 {
		t.Fatalf("effThreshold = %d, want 64", got)
	}
	if got := w.effRest(); got != 48 {
		t.Fatalf("effRest = %d, want 48", got)
	}
	if got := w.indicatorSide(); got != 24 {
		t.Fatalf("indicatorSide = %d, want 24", got)
	}
	// Threshold <= 0 falls back to the default.
	w.Threshold = -5
	if got := w.effThreshold(); got != 64 {
		t.Fatalf("effThreshold with Threshold=-5 = %d, want 64 (default)", got)
	}
	w.Threshold = 100
	if got := w.effThreshold(); got != 100 {
		t.Fatalf("effThreshold with Threshold=100 = %d, want 100", got)
	}
}

// Under DensityTouch the threshold/rest/indicator grow through scaled +
// TouchTarget: scaled multiplies by 1.5, TouchTarget clamps up to the 44px floor.
func TestPullToRefreshTouchDensityMetrics(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	SetDensity(DensityTouch)
	w := NewPullToRefresh(nil)
	// scaled(64) = round(64*1.5) = 96; not clamped (threshold takes no floor).
	if got := w.effThreshold(); got != 96 {
		t.Fatalf("touch effThreshold = %d, want 96", got)
	}
	// scaled(48) = 72 >= 44 -> TouchTarget passes it through.
	if got := w.effRest(); got != 72 {
		t.Fatalf("touch effRest = %d, want 72", got)
	}
	// scaled(24) = 36 < 44 -> TouchTarget clamps up to the 44px finger floor.
	if got := w.indicatorSide(); got != 44 {
		t.Fatalf("touch indicatorSide = %d, want 44 (clamped up)", got)
	}
	// scaled(160) = 240.
	if got := w.effMax(); got != 240 {
		t.Fatalf("touch effMax = %d, want 240", got)
	}
}

// --- state machine: threshold crossing -------------------------------------

func TestPullToRefreshThresholdCrossing(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	const m = 160.0
	// refPull(106,160) = 63.759... < 64  (just below the arm threshold)
	// refPull(107,160) = 64.119... >= 64 (just above)
	if refPull(106, m) >= 64 {
		t.Fatalf("control: refPull(106) = %v, expected < 64", refPull(106, m))
	}
	if refPull(107, m) < 64 {
		t.Fatalf("control: refPull(107) = %v, expected >= 64", refPull(107, m))
	}

	below := newPTR(nil)
	pull(below, 106)
	if below.State() != PullPulling {
		t.Fatalf("raw=106 state = %v, want PullPulling", below.State())
	}
	exactEq(t, below.Pull(), refPull(106, m), "raw=106 pull")

	above := newPTR(nil)
	pull(above, 107)
	if above.State() != PullArmed {
		t.Fatalf("raw=107 state = %v, want PullArmed", above.State())
	}
	exactEq(t, above.Pull(), refPull(107, m), "raw=107 pull")
}

// Dragging back up below the threshold after arming disarms (armed -> pulling),
// and dragging above home reveals no pull (Pull's negative branch floors to 0).
func TestPullToRefreshDisarmAndNegativePull(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	pull(w, 200) // armed
	if w.State() != PullArmed {
		t.Fatalf("after raw=200 state = %v, want PullArmed", w.State())
	}
	// Drag back up to a small net-down: disarms to pulling.
	w.TouchMove(Event{Kind: EventTouchMove, Y: 50})
	if w.State() != PullPulling {
		t.Fatalf("after dragging back to raw=50 state = %v, want PullPulling", w.State())
	}
	// Drag above the start (net upward): the spring offset goes negative and
	// Pull floors it to exactly 0.
	w.TouchMove(Event{Kind: EventTouchMove, Y: -40})
	exactEq(t, w.Pull(), 0, "pull above home is floored")
	if w.State() != PullPulling {
		t.Fatalf("net-upward state = %v, want PullPulling", w.State())
	}
}

// --- release: unarmed springs home -----------------------------------------

func TestPullToRefreshReleaseUnarmedSpringsHome(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	fired := 0
	w := newPTR(nil)
	w.OnRefresh = func() { fired++ }
	pull(w, 50) // below threshold -> pulling
	if w.State() != PullPulling {
		t.Fatalf("state = %v, want PullPulling", w.State())
	}
	w.TouchUp()
	if w.State() != PullIdle {
		t.Fatalf("after unarmed release state = %v, want PullIdle", w.State())
	}
	if !w.Animating() {
		t.Fatalf("spring-back should be Animating")
	}
	settleSpring(t, w)
	exactEq(t, w.Pull(), 0, "unarmed release settles exactly home")
	if w.Animating() {
		t.Fatalf("settled widget should not be Animating")
	}
	if fired != 0 {
		t.Fatalf("OnRefresh fired %d times on an unarmed release, want 0", fired)
	}
}

// --- release: armed refreshes ----------------------------------------------

func TestPullToRefreshReleaseArmedRefreshes(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	fired := 0
	w := newPTR(nil)
	w.OnRefresh = func() { fired++ }
	pull(w, 300) // well past threshold -> armed
	if w.State() != PullArmed {
		t.Fatalf("state = %v, want PullArmed", w.State())
	}
	w.TouchUp()
	if w.State() != PullRefreshing {
		t.Fatalf("after armed release state = %v, want PullRefreshing", w.State())
	}
	if !w.Refreshing() {
		t.Fatalf("Refreshing() = false, want true")
	}
	if fired != 1 {
		t.Fatalf("OnRefresh fired %d times, want 1", fired)
	}
	if !w.spinner.Active().Get() {
		t.Fatalf("spinner not active while refreshing")
	}
	settleSpring(t, w)
	exactEq(t, w.Pull(), float64(w.effRest()), "refreshing rests at the rest height")
	if !w.Animating() {
		t.Fatalf("refreshing widget must keep animating (spinner)")
	}
	// Done returns it home.
	w.Done()
	if w.State() != PullIdle {
		t.Fatalf("after Done state = %v, want PullIdle", w.State())
	}
	if w.spinner.Active().Get() {
		t.Fatalf("spinner still active after Done")
	}
	settleSpring(t, w)
	exactEq(t, w.Pull(), 0, "Done settles exactly home")
	if w.Animating() {
		t.Fatalf("idle widget after Done should not animate")
	}
}

// --- programmatic Refresh / Done edge cases --------------------------------

func TestPullToRefreshProgrammaticRefresh(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	fired := 0
	w := newPTR(nil)
	w.OnRefresh = func() { fired++ }
	w.Refresh()
	if w.State() != PullRefreshing || fired != 1 {
		t.Fatalf("after Refresh state=%v fired=%d, want PullRefreshing, 1", w.State(), fired)
	}
	// Spring UP to rest from home.
	settleSpring(t, w)
	exactEq(t, w.Pull(), float64(w.effRest()), "programmatic refresh springs up to rest")
	// A second Refresh while already refreshing is a no-op (does not re-fire).
	w.Refresh()
	if fired != 1 {
		t.Fatalf("Refresh while refreshing re-fired OnRefresh (%d times)", fired)
	}
	w.Done()
	if w.State() != PullIdle {
		t.Fatalf("state after Done = %v, want PullIdle", w.State())
	}
}

func TestPullToRefreshRefreshNilCallback(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil) // OnRefresh nil
	w.Refresh()      // must not panic on the nil callback
	if w.State() != PullRefreshing {
		t.Fatalf("state = %v, want PullRefreshing", w.State())
	}
}

func TestPullToRefreshDoneWhenNotRefreshing(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	w.Done() // no-op on idle
	if w.State() != PullIdle {
		t.Fatalf("Done on idle changed state to %v", w.State())
	}
}

// --- gating: not at top, no drag, refreshing move --------------------------

func TestPullToRefreshNotAtTopDoesNotPull(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	w.AtTop = func() bool { return false }
	w.TouchDown(Event{Kind: EventTouchStart, Y: 0})
	if consumed := w.TouchMove(Event{Kind: EventTouchMove, Y: 200}); consumed {
		t.Fatalf("TouchMove consumed a pull while not at top")
	}
	if w.State() != PullIdle {
		t.Fatalf("state = %v, want PullIdle (no pull when not at top)", w.State())
	}
	exactEq(t, w.Pull(), 0, "no pull when not at top")
}

func TestPullToRefreshMoveWithoutDown(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	if consumed := w.TouchMove(Event{Kind: EventTouchMove, Y: 100}); consumed {
		t.Fatalf("TouchMove without a TouchDown consumed a pull")
	}
	exactEq(t, w.Pull(), 0, "no pull without a down")
}

func TestPullToRefreshMoveWhileRefreshing(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	w.Refresh()
	w.TouchDown(Event{Kind: EventTouchStart, Y: 0}) // down while refreshing: no reset
	if consumed := w.TouchMove(Event{Kind: EventTouchMove, Y: 100}); consumed {
		t.Fatalf("TouchMove consumed a pull while refreshing")
	}
	if w.State() != PullRefreshing {
		t.Fatalf("refreshing interrupted by a touch, state = %v", w.State())
	}
}

// A down-then-up drag that never moves down (starts moving up) does not begin a
// pull; the else branch of TouchMove just tracks lastY.
func TestPullToRefreshUpwardFirstMoveNoPull(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	w.TouchDown(Event{Kind: EventTouchStart, Y: 100})
	if consumed := w.TouchMove(Event{Kind: EventTouchMove, Y: 60}); consumed { // upward
		t.Fatalf("upward move began a pull")
	}
	if w.State() != PullIdle {
		t.Fatalf("state = %v, want PullIdle", w.State())
	}
}

func TestPullToRefreshTouchUpWithoutDrag(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	w.TouchUp() // not dragging: no-op
	if w.State() != PullIdle {
		t.Fatalf("state = %v, want PullIdle", w.State())
	}
	// Down then up with no move: dragging true, pulling false, no transition.
	w.TouchDown(Event{Kind: EventTouchStart, Y: 0})
	w.TouchUp()
	if w.State() != PullIdle {
		t.Fatalf("down+up with no move state = %v, want PullIdle", w.State())
	}
}

// A touch-down while a spring-back is settling catches it and resets to a clean
// zero baseline.
func TestPullToRefreshTouchDownCatchesSpringBack(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	pull(w, 50)
	w.TouchUp() // spring-back begins
	w.Tick(1.0 / 60)
	if !w.spring.Settling() {
		t.Fatalf("spring should still be settling after one tick")
	}
	w.TouchDown(Event{Kind: EventTouchStart, Y: 0})
	exactEq(t, w.Pull(), 0, "touch-down resets the pull baseline to 0")
	if w.spring.Settling() {
		t.Fatalf("touch-down did not stop the spring-back")
	}
}

// --- Tick / Animating branches ---------------------------------------------

func TestPullToRefreshTickNonPositiveDt(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	pull(w, 300)
	w.TouchUp() // refreshing, spinner active, spring settling
	before := w.Pull()
	phase := w.spinner.Phase
	w.Tick(0)  // no-op
	w.Tick(-1) // no-op
	if w.Pull() != before {
		t.Fatalf("Tick(<=0) advanced the spring")
	}
	if w.spinner.Phase != phase {
		t.Fatalf("Tick(<=0) advanced the spinner")
	}
}

func TestPullToRefreshTickSpinnerOnlyWhenActive(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	// Idle: spinner inactive, spring at rest -> Tick advances neither, and
	// Animating is false.
	w := newPTR(nil)
	if w.Animating() {
		t.Fatalf("fresh idle widget should not animate")
	}
	p0 := w.spinner.Phase
	w.Tick(1.0 / 60)
	if w.spinner.Phase != p0 {
		t.Fatalf("Tick advanced an inactive spinner")
	}
	// Refreshing: spinner active -> Tick advances its phase.
	w.Refresh()
	settleSpring(t, w)
	p1 := w.spinner.Phase
	w.Tick(1.0 / 60)
	if w.spinner.Phase == p1 {
		t.Fatalf("Tick did not advance the active spinner")
	}
}

// --- indicator geometry ----------------------------------------------------

func TestPullToRefreshIndicatorRect(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil) // bounds {10,20,200,300}, side 24
	pull(w, 480)     // pull = 120 exactly
	if got := w.PullInt(); got != 120 {
		t.Fatalf("PullInt = %d, want 120", got)
	}
	// x = 10 + (200-24)/2 = 98 ; y = 20 + (120-24)/2 = 68.
	want := Rect{X: 98, Y: 68, W: 24, H: 24}
	if got := w.indicatorRect(); got != want {
		t.Fatalf("indicatorRect = %+v, want %+v", got, want)
	}
}

func TestPullToRefreshIndicatorRectClampedToTop(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	pull(w, 10) // small pull: gap < side, so the centred y would rise above the top
	ir := w.indicatorRect()
	if ir.Y != 20 { // clamped to bounds top (r.Y)
		t.Fatalf("indicatorRect.Y = %d, want 20 (clamped to top)", ir.Y)
	}
}

// --- drawing: chevron ink + strict bounds ----------------------------------

// paintedPixels renders w onto a sentinel surface and returns the count of
// pixels equal to `want`, failing if any non-sentinel pixel lands outside limit.
func paintedPixels(t *testing.T, w *PullToRefresh, want RGBA, limit Rect) int {
	t.Helper()
	const stride = 240
	theme := DefaultLight()
	buf := makeSurface(stride, stride)
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	w.Draw(newP(buf, stride), theme)
	n := 0
	for y := 0; y < stride; y++ {
		for x := 0; x < stride; x++ {
			px := pixelAt(buf, stride, x, y)
			if px == sentinel {
				continue
			}
			if !limit.Contains(x, y) {
				t.Fatalf("painted at (%d,%d) outside %+v", x, y, limit)
			}
			if px == want {
				n++
			}
		}
	}
	return n
}

func TestPullToRefreshDrawPullingChevron(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	theme := DefaultLight()
	w := newPTR(nil)
	pull(w, 50) // pulling, below threshold
	ir := w.indicatorRect()
	// The chevron paints in OnSurface while pulling, and only inside the box.
	if got := paintedPixels(t, w, theme.OnSurface, ir); got == 0 {
		t.Fatalf("pulling chevron painted no OnSurface pixels")
	}
	if got := paintedPixels(t, w, theme.Accent, ir); got != 0 {
		t.Fatalf("pulling chevron painted %d Accent pixels, want 0", got)
	}
}

func TestPullToRefreshDrawArmedChevron(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	theme := DefaultLight()
	w := newPTR(nil)
	pull(w, 300) // armed
	ir := w.indicatorRect()
	if got := paintedPixels(t, w, theme.Accent, ir); got == 0 {
		t.Fatalf("armed chevron painted no Accent pixels")
	}
}

func TestPullToRefreshDrawRefreshingSpinner(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	theme := DefaultLight()
	w := newPTR(nil)
	w.Refresh()
	settleSpring(t, w)
	ir := w.indicatorRect()
	if got := paintedPixels(t, w, theme.Accent, ir); got == 0 {
		t.Fatalf("refreshing spinner painted no Accent pixels")
	}
}

// An idle widget with no pull paints no indicator at all (all pixels sentinel).
func TestPullToRefreshDrawIdleNoIndicator(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	const stride = 240
	theme := DefaultLight()
	buf := makeSurface(stride, stride)
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}
	w := newPTR(nil) // idle, child nil
	w.Draw(newP(buf, stride), theme)
	for y := 0; y < stride; y++ {
		for x := 0; x < stride; x++ {
			if pixelAt(buf, stride, x, y) != sentinel {
				t.Fatalf("idle widget painted at (%d,%d)", x, y)
			}
		}
	}
}

// --- drawing: child pushed down (translator path) --------------------------

func TestPullToRefreshDrawChildPushedDown(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	const stride = 240
	theme := DefaultLight()
	childCol := RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xFF}
	child := &fillChild{col: childCol}
	w := newPTR(child) // bounds {10,20,200,300}
	pull(w, 480)       // pull = 120
	p := w.PullInt()
	buf := makeSurface(stride, stride)
	w.Draw(newP(buf, stride), theme)
	r := w.Bounds()
	// The child is translated down by pull, clipped to bounds: its top row lands
	// at r.Y+pull; the row just above (in the revealed gap) is not child colour.
	if px := pixelAt(buf, stride, r.X, r.Y+p); px != childCol {
		t.Fatalf("child top row at y=%d = %+v, want child colour (pushed down by %d)", r.Y+p, px, p)
	}
	if px := pixelAt(buf, stride, r.X, r.Y+p-1); px == childCol {
		t.Fatalf("child colour leaked into the gap at y=%d", r.Y+p-1)
	}
}

// --- drawing: fallback path (no Translator/Clipper) ------------------------

func TestPullToRefreshDrawFallbackNoTranslator(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	theme := DefaultLight()
	child := &recFillChild{}
	w := newPTR(child)
	pull(w, 480) // pull = 120
	rp := &recPainter{}
	w.Draw(rp, theme) // recPainter is not a Translator/Clipper -> fallback path
	r := w.Bounds()
	// Fallback offsets the child's bounds by +pull directly.
	if child.drawnAt.Y != r.Y+w.PullInt() {
		t.Fatalf("fallback child drawn at Y=%d, want %d", child.drawnAt.Y, r.Y+w.PullInt())
	}
	// And the child's bounds are restored to the un-offset rect afterwards.
	if child.Bounds().Y != r.Y {
		t.Fatalf("child bounds not restored: Y=%d, want %d", child.Bounds().Y, r.Y)
	}
}

// --- drawPullChevron direct: tip orientation + tiny-box guard --------------

func TestDrawPullChevronGuardsAndShape(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	const stride = 64
	ink := RGBA{R: 0xAB, G: 0xCD, B: 0xEF, A: 0xFF}

	// Tiny box: side < 3 -> paints nothing.
	buf := makeSurface(stride, stride)
	drawPullChevron(newP(buf, stride), Rect{X: 5, Y: 5, W: 2, H: 2}, false, ink)
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] == ink.R && buf[i+1] == ink.G && buf[i+2] == ink.B {
			t.Fatalf("tiny chevron painted a pixel")
		}
	}

	// Down chevron: the tip (narrowest row) sits BELOW the widest row. Wide box
	// so the r.H < r.W branch of the side pick is taken.
	if widest, tip := chevronExtents(t, Rect{X: 8, Y: 8, W: 40, H: 20}, false, ink); tip <= widest {
		t.Fatalf("down chevron: tip y=%d not below widest row y=%d", tip, widest)
	}
	// Up chevron: the tip sits ABOVE the widest row. Tall box so the r.H >= r.W
	// branch is taken.
	if widest, tip := chevronExtents(t, Rect{X: 8, Y: 8, W: 20, H: 40}, true, ink); tip >= widest {
		t.Fatalf("up chevron: tip y=%d not above widest row y=%d", tip, widest)
	}
}

// chevronExtents renders a chevron and returns the y of its widest painted row
// and the y of its narrowest (tip) painted row.
func chevronExtents(t *testing.T, r Rect, up bool, ink RGBA) (widestY, tipY int) {
	t.Helper()
	const stride = 64
	buf := makeSurface(stride, stride)
	drawPullChevron(newP(buf, stride), r, up, ink)
	widths := map[int]int{}
	for y := 0; y < stride; y++ {
		for x := 0; x < stride; x++ {
			o := (y*stride + x) * 4
			if buf[o] == ink.R && buf[o+1] == ink.G && buf[o+2] == ink.B {
				widths[y]++
			}
		}
	}
	if len(widths) == 0 {
		t.Fatalf("chevron painted nothing")
	}
	widestY, tipY = -1, -1
	maxW, minW := -1, 1<<30
	for y, wd := range widths {
		if wd > maxW {
			maxW, widestY = wd, y
		}
		if wd < minW {
			minW, tipY = wd, y
		}
	}
	return widestY, tipY
}

// --- OnEvent routing + child forwarding ------------------------------------

func TestPullToRefreshOnEventRoutesTouchPull(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	fired := 0
	w := newPTR(nil)
	w.OnRefresh = func() { fired++ }
	w.OnEvent(Event{Kind: EventTouchStart, Y: 0})
	w.OnEvent(Event{Kind: EventTouchMove, Y: 300}) // consumed as a pull -> armed
	if w.State() != PullArmed {
		t.Fatalf("OnEvent pull state = %v, want PullArmed", w.State())
	}
	w.OnEvent(Event{Kind: EventTouchEnd, Y: 300}) // armed release -> refresh
	if w.State() != PullRefreshing || fired != 1 {
		t.Fatalf("OnEvent release state=%v fired=%d, want PullRefreshing, 1", w.State(), fired)
	}
}

func TestPullToRefreshOnEventForwardsToChild(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	child := &eventChild{}
	w := newPTR(child)
	// A non-touch event forwards to the child, translated by the (zero) pull.
	w.OnEvent(Event{Kind: EventClick, X: 5, Y: 7})
	if !child.gotAny {
		t.Fatalf("child received no forwarded event")
	}
	if child.last.Kind != EventClick {
		t.Fatalf("forwarded kind = %v, want EventClick", child.last.Kind)
	}

	// A touch-move that is NOT consumed as a pull (not at top) forwards too.
	child.gotAny = false
	w.AtTop = func() bool { return false }
	w.OnEvent(Event{Kind: EventTouchStart, Y: 0})
	w.OnEvent(Event{Kind: EventTouchMove, Y: 200})
	if !child.gotAny {
		t.Fatalf("unconsumed touch-move was not forwarded to the child")
	}

	// A touch-end that is not a pull/armed release still forwards (else branch).
	child.gotAny = false
	w.OnEvent(Event{Kind: EventTouchEnd, Y: 200})
	if !child.gotAny {
		t.Fatalf("touch-end (no pull) was not forwarded to the child")
	}
}

func TestPullToRefreshOnEventTouchEndConsumedWhenPulling(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	child := &eventChild{}
	w := newPTR(child)
	w.OnEvent(Event{Kind: EventTouchStart, Y: 0})
	w.OnEvent(Event{Kind: EventTouchMove, Y: 50}) // pulling (below threshold)
	child.gotAny = false
	w.OnEvent(Event{Kind: EventTouchEnd, Y: 50}) // consumed (pulling) -> NOT forwarded
	if child.gotAny {
		t.Fatalf("touch-end during a pull was forwarded to the child")
	}
	if w.State() != PullIdle {
		t.Fatalf("state after unarmed release = %v, want PullIdle", w.State())
	}
}

func TestPullToRefreshOnEventDisabled(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	child := &eventChild{}
	w := newPTR(child)
	w.Disabled().Set(true)
	w.OnEvent(Event{Kind: EventClick, X: 1, Y: 1})
	if child.gotAny {
		t.Fatalf("disabled widget forwarded an event")
	}
	w.OnEvent(Event{Kind: EventTouchStart, Y: 0})
	w.OnEvent(Event{Kind: EventTouchMove, Y: 200})
	if w.State() != PullIdle {
		t.Fatalf("disabled widget pulled, state = %v", w.State())
	}
}

func TestPullToRefreshForwardToNilChild(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)                         // no child
	w.OnEvent(Event{Kind: EventClick, X: 1}) // must not panic
}

// --- container / accessibility contracts -----------------------------------

func TestPullToRefreshChildren(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	if got := (&PullToRefresh{}).Children(); len(got) != 0 {
		t.Fatalf("nil-child Children() = %v, want empty", got)
	}
	child := &eventChild{}
	w := newPTR(child)
	got := w.Children()
	if len(got) != 1 || got[0] != child {
		t.Fatalf("Children() = %v, want [child]", got)
	}
}

func TestPullToRefreshChildOffset(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	pull(w, 480) // pull = 120
	dx, dy := w.ChildOffset()
	if dx != 0 || dy != 120 {
		t.Fatalf("ChildOffset = (%d,%d), want (0,120)", dx, dy)
	}
}

func TestPullToRefreshA11y(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	w := newPTR(nil)
	if info := w.A11y(); info.Role != RoleGroup || info.Value != "" {
		t.Fatalf("idle A11y = %+v, want {group, \"\"}", info)
	}
	w.Refresh()
	if info := w.A11y(); info.Role != RoleGroup || info.Value != "busy" {
		t.Fatalf("refreshing A11y = %+v, want {group, busy}", info)
	}
}

// The child stays reachable through a generic accessibility walk, placed at its
// pushed-down position.
func TestPullToRefreshWalkA11yReachesChild(t *testing.T) {
	defer restoreDensity()
	restoreDensity()
	lbl := NewLabel("row")
	w := newPTR(lbl)
	lbl.SetBounds(w.Bounds()) // emulate post-layout: child fills the container
	pull(w, 480)              // pull = 120
	nodes := WalkA11y(w)
	var found *A11yNode
	for i := range nodes {
		if nodes[i].Role == RoleText && nodes[i].Name == "row" {
			found = &nodes[i]
		}
	}
	if found == nil {
		t.Fatalf("child label not reached by WalkA11y")
	}
	// The label's bounds were set to the container bounds during layout; the walk
	// adds the ChildOffset (0, pull).
	if found.Rect.Y != w.Bounds().Y+w.PullInt() {
		t.Fatalf("child a11y Y = %d, want %d (pushed down)", found.Rect.Y, w.Bounds().Y+w.PullInt())
	}
}

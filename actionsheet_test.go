// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// asStub is a recording leaf used as a bottom sheet's Content: it remembers the
// events forwarded to it so a test can prove the sheet translated a press into
// its frame.
type asStub struct {
	Base
	events []Event
}

func (s *asStub) Draw(p painter.Painter, _ *Theme) { p.FillRect(s.Bounds(), painter.RGBA{A: 1}) }
func (s *asStub) OnEvent(ev Event)                 { s.events = append(s.events, ev) }

// asRecorder is a painter that captures the fills a widget emits so a test can
// assert exact scrim + panel geometry.
type asRecorder struct {
	fills  []asFill
	rounds []asRound
	w, h   int
}

type asFill struct {
	r painter.Rect
	c painter.RGBA
}
type asRound struct {
	r      painter.Rect
	radius int
	c      painter.RGBA
}

func (p *asRecorder) FillRect(r painter.Rect, c painter.RGBA) {
	p.fills = append(p.fills, asFill{r, c})
}
func (p *asRecorder) StrokeRect(painter.Rect, painter.RGBA, int) {}
func (p *asRecorder) FillRoundRect(r painter.Rect, radius int, c painter.RGBA) {
	p.rounds = append(p.rounds, asRound{r, radius, c})
}
func (p *asRecorder) StrokeRoundRect(painter.Rect, int, painter.RGBA, int) {}
func (p *asRecorder) PutPixel(int, int, painter.RGBA)                      {}
func (p *asRecorder) Text(int, int, string, painter.RGBA)                  {}
func (p *asRecorder) Size() (int, int)                                     { return p.w, p.h }

// newBottomSheet200 builds a laid-out bottom sheet whose panel is exactly 200
// device px tall on a 300x400 surface (metric scale 1, density compact), for
// clean exact-position assertions.
func newBottomSheet200(content Widget) *ActionSheet {
	a := NewBottomSheet(content)
	a.PreferredHeight = 200
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	return a
}

// --- Control run: the animator slide -------------------------------------
//
// Before trusting the widget's exact per-tick positions, validate the METHOD:
// an independent, hand-written reference of the same eased slide must reproduce
// literally hand-computed positions. Only once the control (reference == hand
// math) holds do we assert the widget reproduces the very same literals.

// refEaseOutCubicSlide is the reference slide: elapsed time integrated by dt,
// eased by 1-(1-t)^3, interpolated from -> to. Spelled out independently of
// sheetSlide.
func refEaseOutCubicSlide(from, to, dur, dt float64, steps int) []float64 {
	out := make([]float64, 0, steps)
	elapsed := 0.0
	for i := 0; i < steps; i++ {
		elapsed += dt
		if elapsed > dur {
			elapsed = dur
		}
		t := elapsed / dur
		u := 1 - t
		out = append(out, from+(to-from)*(1-u*u*u))
	}
	return out
}

func TestActionSheetSlideControlRun(t *testing.T) {
	// Hand-computed for from=200, to=0, dur=1, dt=0.25 (t = .25/.5/.75/1):
	//   eased = 1-(1-t)^3 = 0.578125 / 0.875 / 0.984375 / 1
	//   hidden = 200*(1-eased) = 84.375 / 25 / 3.125 / 0
	want := []float64{84.375, 25, 3.125, 0}
	got := refEaseOutCubicSlide(200, 0, 1, 0.25, 4)
	for i := range want {
		exactEq(t, got[i], want[i], "reference slide step %d", i)
	}
}

func TestActionSheetOpenSlideMatchesReference(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.SlideDuration = 1
	a.Open()
	if a.State() != ActionSheetOpening {
		t.Fatalf("state after Open = %v, want Opening", a.State())
	}
	if !a.Animating() {
		t.Fatal("Animating() false right after Open")
	}
	// Hidden starts at the full panel height (fully off-screen).
	exactEq(t, a.hidden, 200, "hidden right after Open")

	want := refEaseOutCubicSlide(200, 0, 1, 0.25, 4)
	for i, w := range want {
		a.Tick(0.25)
		exactEq(t, a.hidden, w, "open tick %d", i)
	}
	if a.State() != ActionSheetOpen {
		t.Fatalf("state after slide = %v, want Open", a.State())
	}
	if a.Animating() {
		t.Fatal("Animating() still true after slide finished")
	}
	if !a.Presented().Get() {
		t.Fatal("Presented observable not true after Open")
	}
}

func TestActionSheetDismissSlideEaseInCubic(t *testing.T) {
	dismissed := 0
	a := newBottomSheet200(&asStub{})
	a.SlideDuration = 1
	a.OnDismiss = func() { dismissed++ }
	// Jump straight to fully open.
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	exactEq(t, a.hidden, 0, "hidden fully open")

	a.Dismiss()
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state after Dismiss = %v, want Dismissing", a.State())
	}
	// EaseInCubic: eased = t^3; hidden = 0 + 200*t^3 = 25/... at t=.25/.5/.75/1:
	//   200*(0.015625/0.125/0.421875/1) = 3.125 / 25 / 84.375 / 200
	want := []float64{3.125, 25, 84.375, 200}
	for i, w := range want {
		a.Tick(0.25)
		exactEq(t, a.hidden, w, "dismiss tick %d", i)
	}
	if a.State() != ActionSheetClosed {
		t.Fatalf("state after dismiss slide = %v, want Closed", a.State())
	}
	if dismissed != 1 {
		t.Fatalf("OnDismiss fired %d times, want exactly 1", dismissed)
	}
	if a.Presented().Get() {
		t.Fatal("Presented observable still true after dismiss")
	}
}

// --- Density / touch-target sizing --------------------------------------

func TestActionSheetActionRowHitTargetPerDensity(t *testing.T) {
	defer SetDensity(DensityCompact)
	cases := []struct {
		level   DensityLevel
		wantRow int
		wantMin int
	}{
		{DensityCompact, 44, 0},
		{DensityComfortable, 55, 36},
		{DensityTouch, 66, 44},
	}
	for _, c := range cases {
		SetDensity(c.level)
		a := NewActionSheet("t")
		a.AddAction("A", nil)
		a.AddAction("B", nil)
		a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
		if got := a.actionRowH(); got != c.wantRow {
			t.Errorf("density %v: actionRowH = %d, want %d", c.level, got, c.wantRow)
		}
		if a.actionRowH() < 44 {
			t.Errorf("density %v: row %d < 44 finger floor", c.level, a.actionRowH())
		}
		if MinHitTarget() != c.wantMin {
			t.Errorf("density %v: MinHitTarget = %d, want %d", c.level, MinHitTarget(), c.wantMin)
		}
		if a.actionRowH() < MinHitTarget() {
			t.Errorf("density %v: row below MinHitTarget", c.level)
		}
		rows, _ := a.actionRects()
		for i, r := range rows {
			if r.H != c.wantRow {
				t.Errorf("density %v: row %d rect H = %d, want %d", c.level, i, r.H, c.wantRow)
			}
		}
	}
}

func TestActionSheetHeightGrowsWithDensity(t *testing.T) {
	defer SetDensity(DensityCompact)
	build := func() *ActionSheet {
		a := NewActionSheet("t")
		a.AddAction("A", nil)
		a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
		return a
	}
	SetDensity(DensityCompact)
	compact := build().sheetHpx
	SetDensity(DensityTouch)
	touch := build().sheetHpx
	if touch <= compact {
		t.Fatalf("touch height %d not greater than compact %d", touch, compact)
	}
}

// --- Drag-to-dismiss: threshold + fling ---------------------------------

func TestActionSheetSlowDragPastThresholdDismisses(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	// Drag down 120 px slowly (dt=1 => velocity 120 px/s, well below fling).
	a.DragBegin(200)         // handle strip at panel top (Y=200)
	a.DragMove(200+120, 1.0) // move to hidden=120
	exactEq(t, a.hidden, 120, "hidden after slow drag")
	// visibleFraction = (200-120)/200 = 0.4 < 0.5 => dismiss.
	a.DragRelease()
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state = %v, want Dismissing (past threshold)", a.State())
	}
	for a.Animating() {
		a.Tick(1.0 / 60.0)
	}
	if a.State() != ActionSheetClosed {
		t.Fatalf("state after settle = %v, want Closed", a.State())
	}
	exactEq(t, a.hidden, 200, "hidden after dismiss settle")
}

func TestActionSheetSlowDragShortSettlesBackToDetent(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	a.DragBegin(200)
	a.DragMove(200+80, 1.0) // hidden=80, visible=0.6 > 0.5 => settle
	a.DragRelease()
	if a.State() != ActionSheetSettling {
		t.Fatalf("state = %v, want Settling", a.State())
	}
	for a.Animating() {
		a.Tick(1.0 / 60.0)
	}
	if a.State() != ActionSheetOpen {
		t.Fatalf("state after settle = %v, want Open", a.State())
	}
	exactEq(t, a.hidden, 0, "hidden after settle to full detent")
}

func TestActionSheetDownwardFlingDismissesRegardlessOfPosition(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	// Barely move (hidden 30, 85% still visible) but very fast: v = 30/0.01 = 3000.
	a.DragBegin(200)
	a.DragMove(200+30, 0.01)
	if a.visibleFraction() <= 0.5 {
		t.Fatalf("precondition: visibleFraction %v should be well above the dismiss floor", a.visibleFraction())
	}
	a.DragRelease()
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state = %v, want Dismissing (downward fling)", a.State())
	}
}

func TestActionSheetUpwardFlingSettlesToFullDetent(t *testing.T) {
	a := NewBottomSheet(&asStub{})
	a.PreferredHeight = 200
	a.Detents = []float64{DetentHalf, DetentFull}
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	// Sit near the half detent (hidden 100), then flick up hard.
	a.DragBegin(200)
	a.DragMove(200+100, 1.0)     // hidden 100
	a.DragMove(200+100-20, 0.01) // fast upward: v = -20/0.01 = -2000
	a.DragRelease()
	if a.State() != ActionSheetSettling {
		t.Fatalf("state = %v, want Settling", a.State())
	}
	for a.Animating() {
		a.Tick(1.0 / 60.0)
	}
	exactEq(t, a.hidden, 0, "hidden after upward fling to full")
}

func TestActionSheetSettleFaithfullyDrivesTheEngine(t *testing.T) {
	// Control: the sheet's hidden must equal the momentum engine's offset on
	// every tick — proof the widget faithfully composes the (independently
	// control-run) Momentum engine rather than doing its own thing.
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	a.DragBegin(200)
	a.DragMove(200+70, 1.0)
	a.DragRelease()
	for a.Animating() {
		a.Tick(1.0 / 60.0)
		exactEq(t, a.hidden, a.phys.Offset(), "hidden tracks engine offset")
	}
}

// --- Scrim / Esc / Back dismiss -----------------------------------------

func TestActionSheetScrimTapDismisses(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	// A press above the panel (panel starts at Y=200) is on the scrim.
	a.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state after scrim tap = %v, want Dismissing", a.State())
	}
}

func TestActionSheetEscAndBackDismiss(t *testing.T) {
	for _, code := range []string{"Escape", "Esc", "Back", "GoBack", "BrowserBack"} {
		a := newBottomSheet200(&asStub{})
		a.Open()
		for a.Animating() {
			a.Tick(1)
		}
		a.OnEvent(Event{Kind: EventKeyDown, Code: code})
		if a.State() != ActionSheetDismissing {
			t.Fatalf("key %q: state = %v, want Dismissing", code, a.State())
		}
	}
	// A non-dismiss key does nothing.
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	a.OnEvent(Event{Kind: EventKeyDown, Code: "a"})
	if a.State() != ActionSheetOpen {
		t.Fatalf("non-dismiss key changed state to %v", a.State())
	}
}

func TestIsDismissKey(t *testing.T) {
	for _, c := range []string{"Escape", "Esc", "Back", "GoBack", "BrowserBack"} {
		if !isDismissKey(c) {
			t.Errorf("isDismissKey(%q) = false, want true", c)
		}
	}
	for _, c := range []string{"", "Enter", "a", "Space"} {
		if isDismissKey(c) {
			t.Errorf("isDismissKey(%q) = true, want false", c)
		}
	}
}

// --- Action / Cancel / Content event routing ----------------------------

func TestActionSheetActionTapFiresAndDismisses(t *testing.T) {
	fired := 0
	a := NewActionSheet("Pick")
	a.AddAction("First", func() { fired++ })
	a.AddAction("Second", func() { fired++ })
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	rows, _ := a.actionRects()
	center := Event{Kind: EventClick, X: rows[0].X + rows[0].W/2, Y: rows[0].Y + rows[0].H/2}
	a.OnEvent(center)
	if fired != 1 {
		t.Fatalf("action fired %d times, want 1", fired)
	}
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state after action tap = %v, want Dismissing", a.State())
	}
}

func TestActionSheetCancelTapDismisses(t *testing.T) {
	cancelled := 0
	a := NewActionSheet("Pick")
	a.AddAction("First", nil)
	a.SetCancel("Cancel", func() { cancelled++ })
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	_, cancel := a.actionRects()
	a.OnEvent(Event{Kind: EventClick, X: cancel.X + cancel.W/2, Y: cancel.Y + cancel.H/2})
	if cancelled != 1 {
		t.Fatalf("cancel fired %d times, want 1", cancelled)
	}
	if a.State() != ActionSheetDismissing {
		t.Fatalf("state after cancel = %v, want Dismissing", a.State())
	}
}

func TestActionSheetForwardsToContent(t *testing.T) {
	content := &asStub{}
	a := newBottomSheet200(content)
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	cr := a.contentRect()
	a.OnEvent(Event{Kind: EventClick, X: cr.X + 5, Y: cr.Y + 5})
	if len(content.events) != 1 {
		t.Fatalf("content saw %d events, want 1", len(content.events))
	}
	// Translated into the content's local frame: (cr.X+5) - cr.X = 5.
	if got := content.events[0]; got.X != 5 || got.Y != 5 {
		t.Fatalf("forwarded event local coords = (%d,%d), want (5,5)", got.X, got.Y)
	}
}

func TestActionSheetPressOnHandleStartsDrag(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	hb := a.handleRect()
	a.OnEvent(Event{Kind: EventTouchStart, X: hb.X + hb.W/2, Y: hb.Y + 2})
	if a.State() != ActionSheetDragging {
		t.Fatalf("state after handle press = %v, want Dragging", a.State())
	}
	// A move + release through OnEvent drives the momentum tracker.
	a.OnEvent(Event{Kind: EventTouchMove, X: hb.X + hb.W/2, Y: hb.Y + 2 + 150})
	a.OnEvent(Event{Kind: EventTouchEnd, X: hb.X + hb.W/2, Y: hb.Y + 2 + 150})
	if a.State() != ActionSheetDismissing && a.State() != ActionSheetClosed {
		t.Fatalf("state after drag+release = %v, want Dismissing/Closed", a.State())
	}
}

func TestActionSheetMouseMoveAndDragForwardWhenNotDragging(t *testing.T) {
	content := &asStub{}
	a := newBottomSheet200(content)
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	cr := a.contentRect()
	a.OnEvent(Event{Kind: EventMouseMove, X: cr.X + 1, Y: cr.Y + 1})
	a.OnEvent(Event{Kind: EventMouseDrag, X: cr.X + 1, Y: cr.Y + 1})
	a.OnEvent(Event{Kind: EventMouseUp, X: cr.X + 1, Y: cr.Y + 1})
	if len(content.events) != 3 {
		t.Fatalf("content saw %d events, want 3 (move/drag/up)", len(content.events))
	}
}

// --- Modality: HitTest ---------------------------------------------------

func TestActionSheetHitTestModality(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	if a.HitTest(10, 10) {
		t.Fatal("closed sheet should be event-transparent")
	}
	a.Open()
	if !a.HitTest(10, 10) {
		t.Fatal("open sheet should catch a scrim press inside bounds")
	}
	if a.HitTest(10, 500) {
		t.Fatal("press outside bounds should not hit")
	}
}

// --- Drawing -------------------------------------------------------------

func TestActionSheetDrawScrimAndPanel(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	rec := &asRecorder{w: 300, h: 400}
	a.Draw(rec, DefaultLight())

	// Scrim: a full-bounds fill at the default alpha (visibleFraction == 1).
	foundScrim := false
	for _, f := range rec.fills {
		if f.r == (painter.Rect{X: 0, Y: 0, W: 300, H: 400}) && f.c == (painter.RGBA{A: ActionSheetScrimAlpha}) {
			foundScrim = true
		}
	}
	if !foundScrim {
		t.Fatalf("no full-bounds scrim at alpha %d in %+v", ActionSheetScrimAlpha, rec.fills)
	}
	// Panel: a rounded fill anchored at Y=200, H=200.
	foundPanel := false
	for _, r := range rec.rounds {
		if r.r.Y == 200 && r.r.H == 200 && r.r.W == 300 {
			foundPanel = true
		}
	}
	if !foundPanel {
		t.Fatalf("no panel round-rect at Y=200 H=200 in %+v", rec.rounds)
	}
}

func TestActionSheetDrawScrimFadesToZero(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.state = ActionSheetDragging // visible state, but...
	a.hidden = 200                // ...fully hidden => visibleFraction 0
	if a.visibleFraction() != 0 {
		t.Fatalf("visibleFraction = %v, want 0", a.visibleFraction())
	}
	rec := &asRecorder{w: 300, h: 400}
	a.Draw(rec, DefaultLight())
	for _, f := range rec.fills {
		if f.r == (painter.Rect{X: 0, Y: 0, W: 300, H: 400}) && f.c.A > 0 {
			t.Fatalf("scrim drawn with alpha %d at zero visibility", f.c.A)
		}
	}
}

func TestActionSheetDrawClosedIsBlank(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	rec := &asRecorder{w: 300, h: 400}
	a.Draw(rec, DefaultLight())
	if len(rec.fills) != 0 || len(rec.rounds) != 0 {
		t.Fatalf("closed sheet drew %d fills / %d rounds, want none", len(rec.fills), len(rec.rounds))
	}
	// Unlaid-out sheet (sheetHpx 0) also draws nothing.
	b := &ActionSheet{state: ActionSheetOpen}
	b.Draw(rec, DefaultLight())
	if len(rec.fills) != 0 {
		t.Fatal("un-laid-out sheet drew something")
	}
}

func TestActionSheetDrawActionModeAndCancel(t *testing.T) {
	a := NewActionSheet("Menu")
	a.AddAction("One", nil)
	a.SetCancel("Cancel", nil)
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	rec := &asRecorder{w: 300, h: 800}
	a.Draw(rec, DefaultLight()) // exercises title + action + cancel draw paths
	if len(rec.rounds) == 0 {
		t.Fatal("action sheet drew no rounded panels/buttons")
	}
}

// --- Accessibility -------------------------------------------------------

func TestActionSheetA11y(t *testing.T) {
	a := NewActionSheet("Options")
	if got := a.A11y(); got.Role != RoleDialog || got.Name != "Options" || got.Value != "" {
		t.Fatalf("closed A11y = %+v, want dialog/Options/'' ", got)
	}
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	a.Open()
	if got := a.A11y(); got.Value != "modal" {
		t.Fatalf("open A11y Value = %q, want modal", got.Value)
	}
}

func TestActionSheetChildren(t *testing.T) {
	content := &asStub{}
	bs := NewBottomSheet(content)
	if got := bs.Children(); len(got) != 1 || got[0] != content {
		t.Fatalf("bottom-sheet Children = %v, want [content]", got)
	}
	a := NewActionSheet("t")
	a.AddAction("A", nil)
	a.Actions = append(a.Actions, nil) // a nil slot must be skipped
	a.AddAction("B", nil)
	a.SetCancel("Cancel", nil)
	got := a.Children()
	if len(got) != 3 {
		t.Fatalf("action Children len = %d, want 3 (A, B, Cancel)", len(got))
	}
}

// --- MVVM binding --------------------------------------------------------

func TestActionSheetPresentedObservableDrivesTheSheet(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	p := a.Presented() // allocate via the accessor branch
	if p.Get() {
		t.Fatal("fresh sheet reports presented")
	}
	p.Set(true)
	if a.State() != ActionSheetOpening {
		t.Fatalf("Set(true) state = %v, want Opening", a.State())
	}
	for a.Animating() {
		a.Tick(1)
	}
	p.Set(false)
	if a.State() != ActionSheetDismissing {
		t.Fatalf("Set(false) state = %v, want Dismissing", a.State())
	}
}

func TestActionSheetOpenAllocatesObservable(t *testing.T) {
	a := newBottomSheet200(&asStub{}) // Presented() never called yet
	a.Open()
	if !a.Presented().Get() {
		t.Fatal("Open did not publish presented=true")
	}
	// Re-setting the same value is a no-op (covers the Get()==v branch).
	a.setPresented(true)
	if !a.Presented().Get() {
		t.Fatal("setPresented(true) changed a true observable")
	}
}

// --- Guards / helpers / defaults ----------------------------------------

func TestActionSheetOpenDismissGuards(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Dismiss() // closed => no-op
	if a.State() != ActionSheetClosed {
		t.Fatalf("Dismiss on closed changed state to %v", a.State())
	}
	a.Open()
	st := a.State()
	a.Open() // already opening => no-op
	if a.State() != st {
		t.Fatal("second Open changed state")
	}
	for a.Animating() {
		a.Tick(1)
	}
	a.Dismiss()
	a.Dismiss() // already dismissing => no-op
	if a.State() != ActionSheetDismissing {
		t.Fatal("second Dismiss changed state")
	}
}

func TestActionSheetDragGuards(t *testing.T) {
	// Not draggable: DragBegin is a no-op.
	a := NewActionSheet("t")
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	a.Open()
	a.DragBegin(10)
	if a.State() == ActionSheetDragging {
		t.Fatal("non-draggable sheet started a drag")
	}
	// DragMove / DragRelease without an active drag are no-ops.
	b := newBottomSheet200(&asStub{})
	b.DragMove(5, 0.1)
	b.DragRelease()
	// Closed sheet: DragBegin is a no-op.
	b.DragBegin(5)
	if b.State() == ActionSheetDragging {
		t.Fatal("closed sheet started a drag")
	}
}

func TestActionSheetDefaultsAndClamps(t *testing.T) {
	a := &ActionSheet{}
	if a.slideDur() != ActionSheetSlideDuration {
		t.Error("slideDur default wrong")
	}
	if a.dismissFraction() != ActionSheetDismissFraction {
		t.Error("dismissFraction default wrong")
	}
	if a.flingVelocity() != ActionSheetFlingVelocity {
		t.Error("flingVelocity default wrong")
	}
	if a.frameSeconds() != ActionSheetFrameSeconds {
		t.Error("frameSeconds default wrong")
	}
	a.SlideDuration, a.DismissFraction, a.FlingVelocity, a.FrameSeconds = 2, 0.3, 900, 0.1
	if a.slideDur() != 2 || a.dismissFraction() != 0.3 || a.flingVelocity() != 900 || a.frameSeconds() != 0.1 {
		t.Error("explicit tuning not honoured")
	}

	// SetBounds clamps hidden into [0, sheetH] on both ends.
	b := newBottomSheet200(&asStub{})
	b.hidden = -5
	b.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	if b.hidden != 0 {
		t.Fatalf("negative hidden not clamped: %v", b.hidden)
	}
	b.hidden = 10000
	b.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	if b.hidden != float64(b.sheetHpx) {
		t.Fatalf("over-large hidden not clamped: %v want %d", b.hidden, b.sheetHpx)
	}
}

func TestActionSheetComputeSheetHContentClamps(t *testing.T) {
	// Default: half the surface.
	a := NewBottomSheet(&asStub{})
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	if a.sheetHpx != 200 {
		t.Errorf("default height = %d, want 200 (half)", a.sheetHpx)
	}
	// Clamp to surface height when too tall.
	a.PreferredHeight = 1000
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	if a.sheetHpx != 400 {
		t.Errorf("over-tall height = %d, want 400 (surface)", a.sheetHpx)
	}
	// Floor at chrome + pads when tiny.
	a.PreferredHeight = 5
	a.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})
	minH := scaled(ActionSheetPad) + a.chromeH() + scaled(ActionSheetPad)
	if a.sheetHpx != minH {
		t.Errorf("tiny height = %d, want floor %d", a.sheetHpx, minH)
	}
}

func TestActionSheetChromeHeightCombos(t *testing.T) {
	base := scaled(ActionSheetHandleH)
	title := scaled(ActionSheetTitleH)
	cases := []struct {
		handle bool
		title  string
		want   int
	}{
		{false, "", 0},
		{true, "", base},
		{false, "T", title},
		{true, "T", base + title},
	}
	for _, c := range cases {
		a := &ActionSheet{ShowHandle: c.handle, Title: c.title}
		if got := a.chromeH(); got != c.want {
			t.Errorf("chromeH(handle=%v,title=%q) = %d, want %d", c.handle, c.title, got, c.want)
		}
	}
}

func TestActionSheetDetentHelpers(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	// Default detents => single full.
	if d := a.detents(); len(d) != 1 || d[0] != DetentFull {
		t.Fatalf("default detents = %v, want [1.0]", d)
	}
	if a.openHidden() != 0 {
		t.Fatalf("openHidden with full detent = %v, want 0", a.openHidden())
	}
	// Degenerate detents (all <= 0) fall back to full.
	a.Detents = []float64{0}
	if a.openHidden() != 0 {
		t.Fatalf("openHidden fallback = %v, want 0", a.openHidden())
	}
	// Nearest detent among half/full.
	a.Detents = []float64{DetentHalf, DetentFull}
	a.hidden = 90
	if got := a.nearestDetentHidden(); got != 100 {
		t.Fatalf("nearestDetentHidden(90) = %v, want 100 (half)", got)
	}
	a.hidden = 40
	if got := a.nearestDetentHidden(); got != 0 {
		t.Fatalf("nearestDetentHidden(40) = %v, want 0 (full)", got)
	}
}

func TestActionSheetAddActionAndCancelNilFns(t *testing.T) {
	a := NewActionSheet("t")
	b := a.AddAction("A", nil) // nil fn must be safe
	b.OnClick()                // runs the wrapper: nil fn skipped, then Dismiss
	c := a.SetCancel("Cancel", nil)
	c.OnClick()
	if len(a.Actions) != 1 || a.Cancel == nil {
		t.Fatal("AddAction/SetCancel wiring wrong")
	}
}

// --- sheetSlide unit ------------------------------------------------------

func TestSheetSlideZeroDuration(t *testing.T) {
	var s sheetSlide
	s.start(5, 9, 0, nil) // nil ease => Linear; zero dur => immediately at `to`
	if s.value() != 9 {
		t.Fatalf("zero-duration slide value = %v, want 9", s.value())
	}
	if v := s.advance(0); v != 9 { // non-positive dt does not advance
		t.Fatalf("advance(0) = %v, want 9", v)
	}
	if !s.done() {
		t.Fatal("zero-duration slide not done")
	}
}

func TestSheetSlideLinearDefault(t *testing.T) {
	var s sheetSlide
	s.start(0, 10, 1, nil) // Linear
	exactEq(t, s.advance(0.5), 5, "linear midpoint")
	exactEq(t, s.advance(0.5), 10, "linear end")
	if !s.done() {
		t.Fatal("slide not done at end")
	}
}

// --- Tick no-op ----------------------------------------------------------

func TestActionSheetOnEventClosedIsNoOp(t *testing.T) {
	content := &asStub{}
	a := newBottomSheet200(content)
	a.OnEvent(Event{Kind: EventClick, X: 10, Y: 10}) // closed => ignored
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if a.State() != ActionSheetClosed || len(content.events) != 0 {
		t.Fatal("closed sheet reacted to an event")
	}
}

func TestActionSheetDrawTitleWithHandle(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Title = "Details" // handle (on) + title together
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	rec := &asRecorder{w: 300, h: 400}
	a.Draw(rec, DefaultLight())
	// The title strip sits below the handle; the handle bar itself is a round.
	if len(rec.rounds) == 0 {
		t.Fatal("nothing drawn for a titled, handled sheet")
	}
}

func TestActionSheetUnlaidOutHelpers(t *testing.T) {
	a := &ActionSheet{state: ActionSheetOpen} // sheetHpx == 0
	if a.visibleFraction() != 0 {
		t.Fatalf("visibleFraction with no bounds = %v, want 0", a.visibleFraction())
	}
	a.layout() // must early-return, not panic
	// handleRect is empty when the handle is off.
	b := NewActionSheet("t")
	b.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 800})
	if b.handleRect() != (Rect{}) {
		t.Fatalf("handleRect with no handle = %+v, want zero", b.handleRect())
	}
}

func TestActionSheetOpenStopsInFlightSpring(t *testing.T) {
	// A drag builds the momentum engine; a subsequent Open must stop it (covers
	// stopPhysics with a live engine).
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	a.DragBegin(200)
	a.DragMove(200+40, 1.0)
	a.DragRelease() // seeds a settling spring
	if a.phys == nil || !a.phys.Settling() {
		t.Fatal("expected a live settling spring after release")
	}
	a.Dismiss() // must stop the spring and take over with the ease slide
	if a.phys.Settling() {
		t.Fatal("Dismiss did not stop the in-flight spring")
	}
}

func TestActionSheetReleaseAtRestFinishesImmediately(t *testing.T) {
	// Begin and release a drag without moving: velocity 0, already at the full
	// detent, so the spring is at rest and the settle completes on release
	// (covers the instant-finish branch in DragRelease).
	a := newBottomSheet200(&asStub{})
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	a.DragBegin(200)
	a.DragRelease()
	if a.State() != ActionSheetOpen {
		t.Fatalf("state after no-op drag release = %v, want Open", a.State())
	}
	if a.Animating() {
		t.Fatal("a no-op release should not leave the sheet animating")
	}
	exactEq(t, a.hidden, 0, "hidden after no-op release")
}

func TestActionSheetTickNoOpWhenIdle(t *testing.T) {
	a := newBottomSheet200(&asStub{})
	a.Tick(1) // closed, nothing animating
	if a.State() != ActionSheetClosed || a.hidden != 0 {
		t.Fatal("Tick moved an idle closed sheet")
	}
	a.Open()
	for a.Animating() {
		a.Tick(1)
	}
	before := a.hidden
	a.Tick(1) // open, at rest
	if a.hidden != before {
		t.Fatal("Tick moved a resting open sheet")
	}
}

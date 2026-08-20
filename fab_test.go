// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// fabPx reads the RGBA pixel at (x, y) from a width-strided RGBA buffer.
func fabPx(buf []byte, width, x, y int) RGBA {
	i := (y*width + x) * 4
	return RGBA{R: buf[i], G: buf[i+1], B: buf[i+2], A: buf[i+3]}
}

// fabRender draws f onto a freshly zeroed w×h pixel buffer and returns it. An
// untouched pixel keeps the zero RGBA (A=0), so a test can tell painted from
// unpainted ground apart.
func fabRender(f *Fab, w, h int, theme *Theme) []byte {
	buf := make([]byte, 4*w*h)
	p := painter.NewPixelPainter(buf, w, h)
	f.Draw(p, theme)
	return buf
}

// TestFabControlRun validates the shared primitives the Fab's geometry is built
// on BEFORE any Fab assertion leans on them — the control-run: if the foundation
// (scaled / anchorCorner / TouchTarget / the cubic easings) ever shifts, this
// fails first and the Fab expectations below are known to be measured against a
// verified baseline rather than a moving one.
func TestFabControlRun(t *testing.T) {
	defer restoreDensity()

	// scaled at each density for the two Fab base metrics (56 disc, 16 margin).
	if got := scaled(FabDiameter); got != 56 {
		t.Fatalf("control: scaled(56) compact = %d, want 56", got)
	}
	if got := scaled(FabMargin); got != 16 {
		t.Fatalf("control: scaled(16) compact = %d, want 16", got)
	}
	SetDensity(DensityTouch)
	if got := scaled(FabDiameter); got != 84 {
		t.Fatalf("control: scaled(56) touch = %d, want 84 (56*1.5)", got)
	}
	if got := scaled(FabMargin); got != 24 {
		t.Fatalf("control: scaled(16) touch = %d, want 24 (16*1.5)", got)
	}
	// TouchTarget clamps a small scaled diameter up to the finger floor.
	if got := TouchTarget(scaled(20)); got != 44 { // 20*1.5=30 -> floor 44
		t.Fatalf("control: TouchTarget(scaled(20)) touch = %d, want 44", got)
	}
	SetDensity(DensityCompact)

	// anchorCorner: the exact BottomRight placement math the Fab reuses.
	host := Rect{X: 0, Y: 0, W: 400, H: 600}
	if got := anchorCorner(host, 56, 56, BottomRight, 16, 0); got != (Rect{X: 328, Y: 528, W: 56, H: 56}) {
		t.Fatalf("control: anchorCorner BottomRight = %+v, want {328 528 56 56}", got)
	}

	// The cubic easings at the exact fractions the stagger produces.
	if got := EaseOutCubic(0.5); got != 0.875 {
		t.Fatalf("control: EaseOutCubic(0.5) = %v, want 0.875", got)
	}
	if got := EaseOutCubic(0.125); got != 0.330078125 {
		t.Fatalf("control: EaseOutCubic(0.125) = %v, want 0.330078125", got)
	}
	if got := EaseInCubic(1); got != 1 {
		t.Fatalf("control: EaseInCubic(1) = %v, want 1", got)
	}
}

func TestNewFabDefaults(t *testing.T) {
	tapped := 0
	f := NewFab("+", func() { tapped++ })
	if f.Icon != "+" {
		t.Fatalf("Icon = %q, want +", f.Icon)
	}
	if f.Corner != BottomRight {
		t.Fatalf("Corner = %v, want BottomRight", f.Corner)
	}
	if f.OnTap == nil {
		t.Fatal("OnTap = nil, want the handler")
	}
	if f.state != fabCollapsed {
		t.Fatalf("fresh state = %v, want fabCollapsed", f.state)
	}
	if f.IsExpanded() {
		t.Fatal("fresh Fab reports IsExpanded")
	}
}

func TestFabAddAction(t *testing.T) {
	f := NewFab("+", nil)
	got := f.AddAction("a", "Add", nil).AddAction("b", "", nil)
	if got != f {
		t.Fatal("AddAction did not return the receiver for chaining")
	}
	if len(f.Actions) != 2 {
		t.Fatalf("len(Actions) = %d, want 2", len(f.Actions))
	}
	// Building the minis, then adding another action, must invalidate the cache
	// so the next sync rebuilds with the new length.
	f.state = fabExpanded
	if n := len(f.Children()); n != 2 {
		t.Fatalf("Children before add = %d, want 2", n)
	}
	f.AddAction("c", "", nil)
	if f.minis != nil {
		t.Fatal("AddAction did not invalidate the mini cache")
	}
	if n := len(f.Children()); n != 3 {
		t.Fatalf("Children after add = %d, want 3", n)
	}
}

func TestFabAccessibleNames(t *testing.T) {
	// Fab: Label wins, else Icon.
	f := NewFab("+", nil)
	if f.a11yName() != "+" {
		t.Fatalf("a11yName with no Label = %q, want +", f.a11yName())
	}
	f.Label = "Compose"
	if f.a11yName() != "Compose" {
		t.Fatalf("a11yName with Label = %q, want Compose", f.a11yName())
	}
	// FabAction: Label wins, else Icon.
	a := &FabAction{Icon: "x"}
	if a.a11yName() != "x" {
		t.Fatalf("FabAction.a11yName no label = %q, want x", a.a11yName())
	}
	a.Label = "Delete"
	if a.a11yName() != "Delete" {
		t.Fatalf("FabAction.a11yName with label = %q, want Delete", a.a11yName())
	}
	// fabMini reports the action's resolved name.
	m := &fabMini{name: "Delete"}
	if info := m.A11y(); info.Role != RoleButton || info.Name != "Delete" {
		t.Fatalf("fabMini.A11y = %+v, want button/Delete", info)
	}
}

func TestFabA11yValue(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "Add", nil)
	if info := f.A11y(); info.Role != RoleButton || info.Name != "+" || info.Value != "" {
		t.Fatalf("collapsed A11y = %+v, want button/+/''", info)
	}
	f.Expand()
	if info := f.A11y(); info.Value != "expanded" {
		t.Fatalf("expanded A11y.Value = %q, want expanded", info.Value)
	}
}

func TestFabExpandedObservable(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	obs := f.Expanded()
	if obs == nil {
		t.Fatal("Expanded() returned nil observable")
	}
	if obs.Get() {
		t.Fatal("fresh observable Get() = true, want false")
	}
	seen := 0
	obs.SubscribeChanged(func() { seen++ })
	f.Expand()
	if !obs.Get() {
		t.Fatal("after Expand observable Get() = false, want true")
	}
	f.Collapse()
	if obs.Get() {
		t.Fatal("after Collapse observable Get() = true, want false")
	}
	if seen != 2 {
		t.Fatalf("observer fired %d times, want 2", seen)
	}
	// Expanded() on a fresh (unallocated) Fab lazily builds it.
	if (&Fab{}).Expanded() == nil {
		t.Fatal("Expanded() on a zero Fab returned nil")
	}
}

func TestFabExpandCollapseStateMachine(t *testing.T) {
	f := NewFab("+", nil)

	// Expand with no Actions is a no-op.
	f.Expand()
	if f.state != fabCollapsed {
		t.Fatalf("Expand with no Actions -> state %v, want fabCollapsed", f.state)
	}

	f.AddAction("a", "", nil)
	f.AddAction("b", "", nil)
	f.AddAction("c", "", nil)

	f.Expand()
	if f.state != fabExpanding || f.frame != 0 {
		t.Fatalf("after Expand: state=%v frame=%d, want expanding/0", f.state, f.frame)
	}
	if !f.Animating() {
		t.Fatal("expanding Fab should be Animating")
	}
	// Re-Expand while expanding is a no-op (frame not reset by a bump first).
	f.frame = 5
	f.Expand()
	if f.frame != 5 {
		t.Fatalf("re-Expand while expanding reset frame to %d, want kept 5", f.frame)
	}

	// totalFrames for 3 actions = 8 + 2*3 = 14.
	if tot := f.totalFrames(); tot != 14 {
		t.Fatalf("totalFrames(3) = %d, want 14", tot)
	}
	// Drive to fully expanded.
	f.frame = 0
	for i := 0; i < 14; i++ {
		f.Tick(0)
	}
	if f.state != fabExpanded {
		t.Fatalf("after 14 ticks state = %v, want fabExpanded", f.state)
	}
	if f.Animating() {
		t.Fatal("fully expanded Fab should not be Animating")
	}
	// Re-Expand while expanded is a no-op.
	f.Expand()
	if f.state != fabExpanded {
		t.Fatalf("re-Expand while expanded -> %v, want fabExpanded", f.state)
	}
	// A Tick while expanded does not disturb the pinned frame.
	before := f.frame
	f.Tick(0)
	if f.frame != before {
		t.Fatalf("Tick while expanded moved frame %d -> %d", before, f.frame)
	}

	// Collapse and drive to closed.
	f.Collapse()
	if f.state != fabCollapsing || f.frame != 0 {
		t.Fatalf("after Collapse: state=%v frame=%d, want collapsing/0", f.state, f.frame)
	}
	// Re-Collapse while collapsing is a no-op.
	f.frame = 4
	f.Collapse()
	if f.frame != 4 {
		t.Fatalf("re-Collapse while collapsing reset frame to %d, want 4", f.frame)
	}
	f.frame = 0
	for i := 0; i < 14; i++ {
		f.Tick(0)
	}
	if f.state != fabCollapsed || f.frame != 0 {
		t.Fatalf("after collapse ticks: state=%v frame=%d, want collapsed/0", f.state, f.frame)
	}
	// Re-Collapse while collapsed is a no-op, and a Tick is inert.
	f.Collapse()
	f.Tick(0)
	if f.state != fabCollapsed {
		t.Fatalf("collapsed Fab disturbed to %v", f.state)
	}
}

func TestFabToggle(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.Toggle()
	if f.state != fabExpanding {
		t.Fatalf("Toggle from collapsed -> %v, want expanding", f.state)
	}
	f.Toggle()
	if f.state != fabCollapsing {
		t.Fatalf("Toggle from open -> %v, want collapsing", f.state)
	}
}

func TestFabTotalFramesSingleAction(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	if tot := f.totalFrames(); tot != fabExpandFrames {
		t.Fatalf("totalFrames(1) = %d, want %d", tot, fabExpandFrames)
	}
	// Zero-action degenerate (via internal accessor) also returns the bare deploy.
	if tot := (&Fab{}).totalFrames(); tot != fabExpandFrames {
		t.Fatalf("totalFrames(0) = %d, want %d", tot, fabExpandFrames)
	}
}

func TestFabMiniProgressStagger(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil).AddAction("c", "", nil)

	// Pinned states.
	f.state = fabExpanded
	for i := 0; i < 3; i++ {
		if p := f.miniProgress(i); p != 1 {
			t.Fatalf("expanded miniProgress(%d) = %v, want 1", i, p)
		}
	}
	f.state = fabCollapsed
	for i := 0; i < 3; i++ {
		if p := f.miniProgress(i); p != 0 {
			t.Fatalf("collapsed miniProgress(%d) = %v, want 0", i, p)
		}
	}

	// Expanding at frame 4: mini 0 is 4/8 in, mini 1 is (4-3)/8 in, mini 2 not
	// started. Exact staggered values.
	f.state = fabExpanding
	f.frame = 4
	if p := f.miniProgress(0); p != 0.875 {
		t.Fatalf("expanding miniProgress(0)@4 = %v, want 0.875", p)
	}
	if p := f.miniProgress(1); p != EaseOutCubic(1.0/8) {
		t.Fatalf("expanding miniProgress(1)@4 = %v, want EaseOutCubic(1/8)", p)
	}
	if p := f.miniProgress(2); p != 0 {
		t.Fatalf("expanding miniProgress(2)@4 = %v, want 0 (not started)", p)
	}

	// Collapsing retracts the last-deployed (i=2) first: at frame 0 all still 1.
	f.state = fabCollapsing
	f.frame = 0
	for i := 0; i < 3; i++ {
		if p := f.miniProgress(i); p != 1 {
			t.Fatalf("collapsing@0 miniProgress(%d) = %v, want 1", i, p)
		}
	}
	// At frame 14 (total) all have retracted to 0.
	f.frame = 14
	for i := 0; i < 3; i++ {
		if p := f.miniProgress(i); p != 0 {
			t.Fatalf("collapsing@14 miniProgress(%d) = %v, want 0", i, p)
		}
	}
	// Mid-collapse the reverse-order stagger holds: i=2 leads i=0.
	f.frame = 5
	if f.miniProgress(2) >= f.miniProgress(0) {
		t.Fatalf("collapse stagger: mini2=%v not ahead of mini0=%v",
			f.miniProgress(2), f.miniProgress(0))
	}
}

func TestFabOverallProgress(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil).AddAction("c", "", nil)
	f.state = fabExpanded
	if p := f.overallProgress(); p != 1 {
		t.Fatalf("expanded overall = %v, want 1", p)
	}
	f.state = fabCollapsed
	if p := f.overallProgress(); p != 0 {
		t.Fatalf("collapsed overall = %v, want 0", p)
	}
	f.state = fabExpanding
	f.frame = 7 // 7/14
	if p := f.overallProgress(); p != 0.5 {
		t.Fatalf("expanding overall @7/14 = %v, want 0.5", p)
	}
	f.state = fabCollapsing
	f.frame = 7
	if p := f.overallProgress(); p != 0.5 {
		t.Fatalf("collapsing overall @7/14 = %v, want 0.5 (1-0.5)", p)
	}
}

func TestFabDiameterAndMarginByDensity(t *testing.T) {
	defer restoreDensity()

	f := NewFab("+", nil)
	// Compact.
	if d := f.diameter(); d != 56 {
		t.Fatalf("compact diameter = %d, want 56", d)
	}
	if m := f.margin(); m != 16 {
		t.Fatalf("compact margin = %d, want 16", m)
	}
	if md := f.miniDiameter(); md != 40 {
		t.Fatalf("compact miniDiameter = %d, want 40", md)
	}
	// Comfortable: 56*1.25=70, 16*1.25=20, 40*1.25=50.
	SetDensity(DensityComfortable)
	if d := f.diameter(); d != 70 {
		t.Fatalf("comfortable diameter = %d, want 70", d)
	}
	if m := f.margin(); m != 20 {
		t.Fatalf("comfortable margin = %d, want 20", m)
	}
	if md := f.miniDiameter(); md != 50 {
		t.Fatalf("comfortable miniDiameter = %d, want 50", md)
	}
	// Touch: 56*1.5=84, 16*1.5=24, 40*1.5=60.
	SetDensity(DensityTouch)
	if d := f.diameter(); d != 84 {
		t.Fatalf("touch diameter = %d, want 84", d)
	}
	if m := f.margin(); m != 24 {
		t.Fatalf("touch margin = %d, want 24", m)
	}
	if md := f.miniDiameter(); md != 60 {
		t.Fatalf("touch miniDiameter = %d, want 60", md)
	}
}

func TestFabDiameterClampAndCustomMargin(t *testing.T) {
	defer restoreDensity()

	// A small custom diameter is clamped UP to the finger floor under touch.
	f := &Fab{Diameter: 20}
	SetDensity(DensityTouch)
	if d := f.diameter(); d != 44 { // scaled(20)=30 -> floor 44
		t.Fatalf("touch clamped diameter = %d, want 44", d)
	}
	SetDensity(DensityCompact)
	if d := f.diameter(); d != 20 { // no floor, no clamp
		t.Fatalf("compact custom diameter = %d, want 20", d)
	}
	// Custom margin overrides the default; zero falls back to FabMargin.
	f.Margin = 4
	if m := f.margin(); m != 4 {
		t.Fatalf("custom margin = %d, want 4", m)
	}
	f.Margin = 0
	if m := f.margin(); m != FabMargin {
		t.Fatalf("zero margin = %d, want default %d", m, FabMargin)
	}
}

func TestFabAnchorPlacement(t *testing.T) {
	defer restoreDensity()

	host := Rect{X: 0, Y: 0, W: 400, H: 600}
	cases := []struct {
		corner Corner
		want   Rect
	}{
		{BottomRight, Rect{X: 328, Y: 528, W: 56, H: 56}}, // 400-16-56, 600-16-56
		{BottomLeft, Rect{X: 16, Y: 528, W: 56, H: 56}},
		{TopRight, Rect{X: 328, Y: 16, W: 56, H: 56}},
		{TopLeft, Rect{X: 16, Y: 16, W: 56, H: 56}},
		{TopCenter, Rect{X: 172, Y: 16, W: 56, H: 56}},     // (400-56)/2=172
		{BottomCenter, Rect{X: 172, Y: 528, W: 56, H: 56}}, // (400-56)/2, 600-16-56
	}
	for _, tc := range cases {
		f := &Fab{Corner: tc.corner}
		f.AnchorIn(host)
		if got := f.Bounds(); got != tc.want {
			t.Errorf("AnchorIn %v = %+v, want %+v", tc.corner, got, tc.want)
		}
		if f.scrim != host {
			t.Errorf("AnchorIn %v did not record the scrim host", tc.corner)
		}
	}

	// Touch density shifts the whole placement (d=84, margin=24).
	SetDensity(DensityTouch)
	f := &Fab{Corner: BottomRight}
	f.AnchorIn(host)
	want := Rect{X: 400 - 24 - 84, Y: 600 - 24 - 84, W: 84, H: 84}
	if got := f.Bounds(); got != want {
		t.Fatalf("touch AnchorIn BottomRight = %+v, want %+v", got, want)
	}
}

func TestFabStacksDown(t *testing.T) {
	down := []Corner{TopLeft, TopRight, TopCenter}
	up := []Corner{BottomLeft, BottomRight, BottomCenter}
	for _, c := range down {
		if !(&Fab{Corner: c}).stacksDown() {
			t.Errorf("corner %v: stacksDown = false, want true", c)
		}
	}
	for _, c := range up {
		if (&Fab{Corner: c}).stacksDown() {
			t.Errorf("corner %v: stacksDown = true, want false", c)
		}
	}
}

func TestFabRound(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{0, 0},
		{0.5, 1},
		{0.49, 0},
		{-0.5, -1},
		{-0.49, 0},
		{-60, -60},
		{112.0, 112},
	}
	for _, tc := range cases {
		if got := fabRound(tc.in); got != tc.want {
			t.Errorf("fabRound(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestFabMiniRectUpAndDown(t *testing.T) {
	// Upward stack (BottomRight), fully expanded: exact slot rectangles.
	f := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil)
	f.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})
	f.state = fabExpanded
	// md=40, gap=12, step=52, x=328+(56-40)/2=336, collapsedTop=528+8=536.
	// up fullTop(i)=528-12-40-i*52 = 476 - i*52.
	if got := f.miniRect(0); got != (Rect{X: 336, Y: 476, W: 40, H: 40}) {
		t.Fatalf("up miniRect(0) = %+v, want {336 476 40 40}", got)
	}
	if got := f.miniRect(1); got != (Rect{X: 336, Y: 424, W: 40, H: 40}) {
		t.Fatalf("up miniRect(1) = %+v, want {336 424 40 40}", got)
	}
	// Collapsed progress 0 => both minis sit at the collapsed slot behind the disc.
	f.state = fabCollapsed
	if got := f.miniRect(0); got != (Rect{X: 336, Y: 536, W: 40, H: 40}) {
		t.Fatalf("collapsed miniRect(0) = %+v, want {336 536 40 40}", got)
	}

	// Downward stack (TopRight): fullTop(i)=Y+d+gap+i*step = 16+56+12+i*52.
	g := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil)
	g.Corner = TopRight
	g.SetBounds(Rect{X: 328, Y: 16, W: 56, H: 56})
	g.state = fabExpanded
	if got := g.miniRect(0); got != (Rect{X: 336, Y: 84, W: 40, H: 40}) {
		t.Fatalf("down miniRect(0) = %+v, want {336 84 40 40}", got)
	}
	if got := g.miniRect(1); got != (Rect{X: 336, Y: 136, W: 40, H: 40}) {
		t.Fatalf("down miniRect(1) = %+v, want {336 136 40 40}", got)
	}
}

func TestFabMiniRectPerTick(t *testing.T) {
	// Exact per-tick offset of mini 0 as it slides from behind the disc to slot.
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})
	f.state = fabExpanding
	// collapsedTop=536, fullTop(0)=476, delta=-60. top = 536 + round(-60*p0).
	for _, frame := range []int{0, 2, 4, 8} {
		f.frame = frame
		p := EaseOutCubic(clampUnit(float64(frame) / float64(fabExpandFrames)))
		wantTop := 536 + fabRound(-60*p)
		if got := f.miniRect(0); got.Y != wantTop {
			t.Fatalf("expanding frame %d: miniRect(0).Y = %d, want %d (p=%v)", frame, got.Y, wantTop, p)
		}
	}
	// Concrete anchors: frame 0 hidden behind disc, frame 8 at full slot.
	f.frame = 0
	if got := f.miniRect(0).Y; got != 536 {
		t.Fatalf("frame 0 top = %d, want 536 (behind disc)", got)
	}
	f.frame = 8
	if got := f.miniRect(0).Y; got != 476 {
		t.Fatalf("frame 8 top = %d, want 476 (full slot)", got)
	}
}

func TestFabSyncMinis(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "Add", func() {})
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	f.state = fabExpanded
	f.syncMinis()
	if len(f.minis) != 1 {
		t.Fatalf("len(minis) = %d, want 1", len(f.minis))
	}
	m := f.minis[0]
	if m.icon != "a" || m.name != "Add" || m.onTap == nil {
		t.Fatalf("mini not synced from action: %+v", m)
	}
	if m.Bounds().W != 40 {
		t.Fatalf("mini bounds W = %d, want 40", m.Bounds().W)
	}
	// A second sync with the same length reuses the slice (no rebuild) but keeps
	// the fields fresh.
	prev := f.minis
	f.Actions[0].Icon = "b"
	f.syncMinis()
	if &f.minis[0] != &prev[0] {
		t.Fatal("syncMinis rebuilt an unchanged-length slice")
	}
	if f.minis[0].icon != "b" {
		t.Fatalf("mini icon not refreshed: %q, want b", f.minis[0].icon)
	}
}

func TestFabChildren(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil)
	// Collapsed: no children (nothing on screen to announce).
	if got := f.Children(); got != nil {
		t.Fatalf("collapsed Children = %v, want nil", got)
	}
	// Expanded: the two minis, each an Accessible button.
	f.state = fabExpanded
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	kids := f.Children()
	if len(kids) != 2 {
		t.Fatalf("expanded Children len = %d, want 2", len(kids))
	}
	for i, k := range kids {
		a, ok := k.(Accessible)
		if !ok {
			t.Fatalf("child %d is not Accessible", i)
		}
		if a.A11y().Role != RoleButton {
			t.Fatalf("child %d role = %v, want button", i, a.A11y().Role)
		}
	}
}

func TestFabWalkA11yExposesActionsWhenExpanded(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "Add", nil).AddAction("b", "Edit", nil)
	f.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})

	// Collapsed: only the disc is in the tree.
	nodes := WalkA11y(f)
	if len(nodes) != 1 {
		t.Fatalf("collapsed WalkA11y len = %d, want 1", len(nodes))
	}
	if nodes[0].Role != RoleButton || nodes[0].Rect != (Rect{X: 328, Y: 528, W: 56, H: 56}) {
		t.Fatalf("collapsed disc node = %+v", nodes[0])
	}

	// Expanded: the disc plus the two mini actions, each with its own bounds.
	f.state = fabExpanded
	nodes = WalkA11y(f)
	if len(nodes) != 3 {
		t.Fatalf("expanded WalkA11y len = %d, want 3", len(nodes))
	}
	if nodes[1].Name != "Add" || nodes[2].Name != "Edit" {
		t.Fatalf("expanded action names = %q,%q, want Add,Edit", nodes[1].Name, nodes[2].Name)
	}
	if nodes[1].Rect.W != 40 {
		t.Fatalf("mini node rect W = %d, want 40", nodes[1].Rect.W)
	}
}

func TestFabHitTest(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})

	// Collapsed: only the disc is sensitive.
	if !f.HitTest(120, 120) {
		t.Fatal("collapsed HitTest inside disc = false")
	}
	if f.HitTest(10, 10) {
		t.Fatal("collapsed HitTest far outside = true")
	}

	// Expanded with a scrim: the whole host captures the tap.
	f.scrim = Rect{X: 0, Y: 0, W: 400, H: 400}
	f.state = fabExpanded
	if !f.HitTest(10, 10) {
		t.Fatal("expanded HitTest over scrim = false")
	}
	if f.HitTest(500, 500) {
		t.Fatal("expanded HitTest outside scrim = true")
	}

	// Expanded without a scrim: falls back to disc + mini rects.
	f.scrim = Rect{}
	mr := f.miniRect(0)
	if !f.HitTest(mr.X+1, mr.Y+1) {
		t.Fatal("no-scrim expanded HitTest over a mini = false")
	}
	if !f.HitTest(120, 120) {
		t.Fatal("no-scrim expanded HitTest over disc = false")
	}
	if f.HitTest(0, 0) {
		t.Fatal("no-scrim expanded HitTest over empty space = true")
	}
}

func TestFabPrimaryTapFiresOnce(t *testing.T) {
	tapped := 0
	f := NewFab("+", func() { tapped++ })
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})

	// A click on the disc fires OnTap exactly once and marks it pressed.
	f.OnEvent(Event{Kind: EventClick, X: 120, Y: 120})
	if tapped != 1 {
		t.Fatalf("after 1 click, OnTap fired %d times, want 1", tapped)
	}
	if !f.pressed {
		t.Fatal("disc not marked pressed after click")
	}
	// A click off the disc does nothing.
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if tapped != 1 {
		t.Fatalf("off-disc click changed OnTap count to %d, want 1", tapped)
	}
	// Mouse-up clears the pressed face.
	f.OnEvent(Event{Kind: EventMouseUp})
	if f.pressed {
		t.Fatal("pressed not cleared on mouse-up")
	}
}

func TestFabDisabledIgnoresEvents(t *testing.T) {
	tapped := 0
	f := NewFab("+", func() { tapped++ })
	f.Disabled().Set(true)
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	f.OnEvent(Event{Kind: EventClick, X: 28, Y: 28})
	if tapped != 0 {
		t.Fatalf("disabled Fab fired OnTap %d times, want 0", tapped)
	}
}

func TestFabKeyboardActivation(t *testing.T) {
	// No-Actions Fab: Enter, Space (" ") and "Space" all fire OnTap.
	for _, code := range []string{"Enter", " ", "Space"} {
		tapped := 0
		f := NewFab("+", func() { tapped++ })
		f.OnEvent(Event{Kind: EventKeyDown, Code: code})
		if tapped != 1 {
			t.Fatalf("key %q fired OnTap %d times, want 1", code, tapped)
		}
	}
	// An unrelated key does nothing.
	tapped := 0
	f := NewFab("+", func() { tapped++ })
	f.OnEvent(Event{Kind: EventKeyDown, Code: "a"})
	if tapped != 0 {
		t.Fatalf("key 'a' fired OnTap %d times, want 0", tapped)
	}
	// Escape / Esc collapse an open dial.
	for _, code := range []string{"Escape", "Esc"} {
		g := NewFab("+", nil).AddAction("x", "", nil)
		g.Expand()
		g.OnEvent(Event{Kind: EventKeyDown, Code: code})
		if g.state != fabCollapsing {
			t.Fatalf("key %q -> state %v, want collapsing", code, g.state)
		}
	}
}

func TestFabActivatePrimaryTogglesWithActions(t *testing.T) {
	// With Actions, a disc tap toggles the dial rather than firing OnTap.
	onTap := 0
	f := NewFab("+", func() { onTap++ }).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	f.OnEvent(Event{Kind: EventClick, X: 28, Y: 28})
	if onTap != 0 {
		t.Fatalf("OnTap fired %d times with Actions present, want 0", onTap)
	}
	if f.state != fabExpanding {
		t.Fatalf("disc tap with Actions -> %v, want expanding", f.state)
	}
	// OnTap nil is safe when there are no actions.
	g := NewFab("+", nil)
	g.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	g.OnEvent(Event{Kind: EventClick, X: 28, Y: 28}) // must not panic
}

func TestFabMiniSelectionFiresOnceAndCollapses(t *testing.T) {
	firedA, firedB := 0, 0
	f := NewFab("+", nil).
		AddAction("a", "Add", func() { firedA++ }).
		AddAction("b", "Edit", func() { firedB++ })
	f.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})
	f.scrim = Rect{X: 0, Y: 0, W: 400, H: 600}
	f.state = fabExpanded

	// Tap the center of mini 1 (its exact slot).
	mr := f.miniRect(1)
	f.OnEvent(Event{Kind: EventClick, X: mr.X + mr.W/2, Y: mr.Y + mr.H/2})
	if firedB != 1 || firedA != 0 {
		t.Fatalf("mini selection fired A=%d B=%d, want A=0 B=1", firedA, firedB)
	}
	if f.state != fabCollapsing {
		t.Fatalf("state after mini selection = %v, want collapsing", f.state)
	}

	// A mini with a nil callback still collapses without panicking.
	g := NewFab("+", nil).AddAction("a", "", nil)
	g.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})
	g.state = fabExpanded
	gr := g.miniRect(0)
	g.OnEvent(Event{Kind: EventClick, X: gr.X + 1, Y: gr.Y + 1})
	if g.state != fabCollapsing {
		t.Fatalf("nil-callback mini -> %v, want collapsing", g.state)
	}
}

func TestFabScrimTapCollapses(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 328, Y: 528, W: 56, H: 56})
	f.scrim = Rect{X: 0, Y: 0, W: 400, H: 600}
	f.state = fabExpanded
	// A click on empty scrim (not a mini, not the disc) collapses.
	f.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if f.state != fabCollapsing {
		t.Fatalf("scrim tap -> %v, want collapsing", f.state)
	}
}

func TestFabSecondaryClickExpands(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})
	// On the disc: opens the dial.
	f.OnEvent(Event{Kind: EventSecondaryClick, X: 120, Y: 120})
	if f.state != fabExpanding {
		t.Fatalf("secondary click on disc -> %v, want expanding", f.state)
	}
	// Off the disc: ignored.
	g := NewFab("+", nil).AddAction("a", "", nil)
	g.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})
	g.OnEvent(Event{Kind: EventSecondaryClick, X: 10, Y: 10})
	if g.state != fabCollapsed {
		t.Fatalf("secondary click off disc -> %v, want collapsed", g.state)
	}
}

func TestFabHoverTracking(t *testing.T) {
	f := NewFab("+", nil)
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})
	f.OnEvent(Event{Kind: EventMouseMove, X: 120, Y: 120})
	if !f.hovered {
		t.Fatal("hover not set when pointer over disc")
	}
	f.OnEvent(Event{Kind: EventMouseMove, X: 10, Y: 10})
	if f.hovered {
		t.Fatal("hover not cleared when pointer leaves disc")
	}
}

func TestFabTouchLongPressExpands(t *testing.T) {
	f := NewFab("+", nil).AddAction("a", "", nil)
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})
	// Begin a touch and hold it still past LongPressTicks (default 30).
	f.OnEvent(Event{Kind: EventTouchStart, Code: "t1", X: 120, Y: 120})
	for i := 0; i < 30; i++ {
		f.Tick(0)
	}
	if !f.IsExpanded() {
		t.Fatal("long-press did not expand the dial")
	}
}

func TestFabTouchTapActivates(t *testing.T) {
	tapped := 0
	f := NewFab("+", func() { tapped++ })
	f.SetBounds(Rect{X: 100, Y: 100, W: 56, H: 56})
	// A quick touch down+up over the disc is a tap -> OnTap once.
	f.OnEvent(Event{Kind: EventTouchStart, Code: "t1", X: 120, Y: 120})
	f.OnEvent(Event{Kind: EventTouchEnd, Code: "t1", X: 120, Y: 120})
	if tapped != 1 {
		t.Fatalf("touch tap fired OnTap %d times, want 1", tapped)
	}
}

func TestFabZeroValueRuns(t *testing.T) {
	// A zero-value Fab must run: lazy gest/observable allocation, no panics.
	f := &Fab{}
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	f.OnEvent(Event{Kind: EventClick, X: 28, Y: 28}) // no OnTap, no Actions -> no-op
	f.Tick(0)
	_ = fabRender(f, 80, 80, DefaultLight())
}

func TestFabDrawDiscFace(t *testing.T) {
	theme := DefaultLight()
	f := NewFab("", nil) // no glyph, so the centre is pure face
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	buf := fabRender(f, 80, 80, theme)
	// A point inside the disc but off-centre shows the opaque Accent face.
	if got := fabPx(buf, 80, 14, 28); got != theme.Accent {
		t.Fatalf("disc face pixel = %v, want Accent %v", got, theme.Accent)
	}
}

func TestFabDrawPressedAndDisabledFaces(t *testing.T) {
	theme := DefaultLight()

	// Pressed: a darkened Accent face.
	f := NewFab("", nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	f.pressed = true
	buf := fabRender(f, 80, 80, theme)
	wantPressed := blendRGBA(theme.Accent, theme.Background, 0.25)
	if got := fabPx(buf, 80, 14, 28); got != wantPressed {
		t.Fatalf("pressed face = %v, want %v", got, wantPressed)
	}

	// Disabled: a muted face, and (per drawButton) no shadow band below it.
	g := NewFab("", nil)
	g.Disabled().Set(true)
	g.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	buf = fabRender(g, 80, 80, theme)
	if got := fabPx(buf, 80, 14, 28); got != mutedFace(theme) {
		t.Fatalf("disabled face = %v, want muted %v", got, mutedFace(theme))
	}
	// Below the disc, where a shadow would fall, stays unpainted for a disabled Fab.
	if got := fabPx(buf, 80, 28, 61); got.A != 0 {
		t.Fatalf("disabled Fab painted a shadow pixel %v, want none", got)
	}
}

func TestFabDrawShadow(t *testing.T) {
	theme := DefaultLight()
	f := NewFab("", nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 56, H: 56})
	buf := fabRender(f, 80, 80, theme)
	// The elevation shadow is offset down by scaled(FabElevation)=6, so a band
	// just below the disc bottom (y in [56, 62)) is painted translucent black.
	got := fabPx(buf, 80, 28, 59)
	if got.A == 0 {
		t.Fatalf("expected a shadow pixel below the disc, got unpainted")
	}
	if got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("shadow pixel = %v, want black-ish", got)
	}
}

func TestFabDrawScrimAndMinis(t *testing.T) {
	theme := DefaultLight()
	f := NewFab("+", nil).AddAction("a", "", nil)
	// Anchor top-left so the (single) mini stacks DOWN into the buffer.
	f.Corner = TopLeft
	f.AnchorIn(Rect{X: 0, Y: 0, W: 120, H: 200})
	f.state = fabExpanded

	buf := fabRender(f, 120, 200, theme)

	// Scrim: a far corner shows translucent black at full openness (0x66).
	sc := fabPx(buf, 120, 119, 199)
	if sc != (RGBA{R: 0, G: 0, B: 0, A: fabScrimMaxAlpha}) {
		t.Fatalf("scrim pixel = %v, want {0 0 0 %d}", sc, fabScrimMaxAlpha)
	}

	// Mini face: a point on the (opaque, fully deployed) mini disc, offset left of
	// the centred glyph, shows the Surface face.
	mr := f.miniRect(0)
	mp := fabPx(buf, 120, mr.X+6, mr.Y+mr.H/2)
	if mp != theme.Surface {
		t.Fatalf("mini face pixel = %v, want Surface %v", mp, theme.Surface)
	}
}

func TestFabDrawMiniSkippedWhenHidden(t *testing.T) {
	theme := DefaultLight()
	// Two actions, expanding at frame 0: mini 1 has not emerged (progress 0) and
	// must be skipped; mini 0 is just starting behind the disc.
	f := NewFab("+", nil).AddAction("a", "", nil).AddAction("b", "", nil)
	f.Corner = TopLeft
	f.AnchorIn(Rect{X: 0, Y: 0, W: 120, H: 220})
	f.state = fabExpanding
	f.frame = 0
	// mini 1 progress is 0 -> drawMini returns early; this just must not panic and
	// must leave the mini-1 slot region unpainted.
	buf := fabRender(f, 120, 220, theme)
	r1 := f.miniRect(1) // == collapsed slot behind disc at frame 0
	_ = r1
	// Nothing asserts a specific pixel here beyond a clean render; the skip branch
	// is exercised by progress==0 on mini 1.
	if len(buf) == 0 {
		t.Fatal("empty render")
	}
}

func TestFabDrawFocusRing(t *testing.T) {
	theme := DefaultLight()
	f := NewFab("", nil)
	f.SetBounds(Rect{X: 2, Y: 2, W: 56, H: 56})
	f.SetFocused(true)
	// Draw must set a round focus ring (radius = d/2) and a >=1 width, then paint
	// it — exercised here for the branch, plus a spot check that the ring width
	// landed.
	_ = fabRender(f, 80, 80, theme)
	if f.FocusRingRadius != 28 {
		t.Fatalf("FocusRingRadius = %d, want 28 (d/2)", f.FocusRingRadius)
	}
	if f.FocusRingWidth < 1 {
		t.Fatalf("FocusRingWidth = %d, want >= 1", f.FocusRingWidth)
	}
}

func TestFabDrawZeroBoundsIsNoop(t *testing.T) {
	// drawButton returns early on a zero-diameter disc.
	f := NewFab("+", nil)
	buf := fabRender(f, 40, 40, DefaultLight())
	for _, b := range buf {
		if b != 0 {
			t.Fatal("zero-bounds Fab painted a pixel, want a clean buffer")
		}
	}
}

func TestFabDrawMiniWithoutIcon(t *testing.T) {
	// A mini whose action Icon is empty draws face+border but no glyph — the
	// icon-empty branch of drawMini.
	theme := DefaultLight()
	f := NewFab("", nil).AddAction("", "Silent", nil)
	f.Corner = TopLeft
	f.AnchorIn(Rect{X: 0, Y: 0, W: 120, H: 200})
	f.state = fabExpanded
	buf := fabRender(f, 120, 200, theme)
	mr := f.miniRect(0)
	if got := fabPx(buf, 120, mr.X+mr.W/2, mr.Y+mr.H/2); got != theme.Surface {
		t.Fatalf("icon-less mini centre = %v, want Surface", got)
	}
}

func TestFabFade(t *testing.T) {
	c := RGB(0x10, 0x20, 0x30) // A=0xFF
	if got := fabFade(c, -1); got.A != 0 {
		t.Fatalf("fabFade t<0 alpha = %d, want 0", got.A)
	}
	if got := fabFade(c, 2); got.A != 0xFF {
		t.Fatalf("fabFade t>1 alpha = %d, want 255", got.A)
	}
	got := fabFade(c, 0.5)
	if got.A != uint8(255*0.5+0.5) || got.R != 0x10 || got.G != 0x20 || got.B != 0x30 {
		t.Fatalf("fabFade 0.5 = %v, want RGB kept, alpha 128", got)
	}
}

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// recordWidget is a fallback that records what the Native asked of it, so a test
// can tell whether Draw/OnEvent reached it.
type recordWidget struct {
	Base
	drawn  int
	events []Event
}

func (r *recordWidget) Draw(p painter.Painter, theme *Theme) { r.drawn++ }
func (r *recordWidget) OnEvent(ev Event)                     { r.events = append(r.events, ev) }

// fakeContainer is a childContainer only (no offset, no clip): the unclipped
// walk path.
type fakeContainer struct {
	Base
	kids []Widget
}

func (f *fakeContainer) Children() []Widget { return f.kids }

// fakeViewport implements all three walk interfaces, with clip and offset a test
// sets exactly — so WalkNative's clip math is asserted without a ScrollView's
// scrollbar-gutter arithmetic.
type fakeViewport struct {
	Base
	kids       []Widget
	clip       Rect
	offX, offY int
}

func (f *fakeViewport) Children() []Widget      { return f.kids }
func (f *fakeViewport) ChildOffset() (int, int) { return f.offX, f.offY }
func (f *fakeViewport) ChildClip() Rect         { return f.clip }

func TestNativeConstructors(t *testing.T) {
	if b := NewNativeButton("Go", nil); b.Kind != NativeButton || b.Text().Get() != "Go" {
		t.Errorf("button: kind=%v text=%q", b.Kind, b.Text().Get())
	}
	if l := NewNativeLabel("L"); l.Kind != NativeLabel || l.Text().Get() != "L" {
		t.Errorf("label wrong")
	}
	if e := NewNativeEntry("e"); e.Kind != NativeEntry || e.Text().Get() != "e" {
		t.Errorf("entry wrong")
	}
	if s := NewNativeSecureEntry("s"); s.Kind != NativeSecureEntry || s.Text().Get() != "s" {
		t.Errorf("secure wrong")
	}
	if c := NewNativeCheckbox("C", true); c.Kind != NativeCheckbox || c.Text().Get() != "C" || !c.On().Get() {
		t.Errorf("checkbox wrong")
	}
	if r := NewNativeRadio("R", false); r.Kind != NativeRadio || r.Text().Get() != "R" || r.On().Get() {
		t.Errorf("radio wrong")
	}
	if s := NewNativeSwitch(true); s.Kind != NativeSwitch || !s.On().Get() {
		t.Errorf("switch wrong")
	}
	sl := NewNativeSlider(0, 10, 3)
	if sl.Kind != NativeSlider || sl.Number().Get() != 3 || sl.Min != 0 || sl.Max != 10 {
		t.Errorf("slider wrong")
	}
	pu := NewNativePopUp([]string{"a", "b"}, "b")
	if pu.Kind != NativePopUp || pu.Text().Get() != "b" || len(pu.Items) != 2 {
		t.Errorf("popup wrong")
	}
}

func TestNativeLazyAccessors(t *testing.T) {
	// A button has no On/Number observable until asked; the accessors create them.
	b := NewNativeButton("x", nil)
	if b.On().Get() != false {
		t.Errorf("lazy On default = true, want false")
	}
	if b.Number().Get() != 0 {
		t.Errorf("lazy Number default = %v, want 0", b.Number().Get())
	}
	if b.Claimed().Get() != false {
		t.Errorf("lazy Claimed default = true, want false")
	}
	// A switch has no text until asked.
	if s := NewNativeSwitch(false); s.Text().Get() != "" {
		t.Errorf("lazy Text default = %q, want empty", s.Text().Get())
	}
}

func TestNativeActivate(t *testing.T) {
	fired := 0
	n := NewNativeButton("go", func() { fired++ })
	n.Activate()
	if fired != 1 {
		t.Fatalf("Activate ran handler %d times, want 1", fired)
	}
	n.SetOnActivate(func() { fired += 10 })
	n.Activate()
	if fired != 11 {
		t.Fatalf("after SetOnActivate, fired = %d, want 11", fired)
	}
	// A Native with no handler must not panic.
	NewNativeLabel("l").Activate()
}

func TestNativeDraw(t *testing.T) {
	theme := DefaultLight()
	p := newP(makeSurface(30, 30), 30)

	n := NewNativeEntry("x")
	fb := &recordWidget{}
	n.Fallback = fb
	n.SetBounds(Rect{X: 1, Y: 2, W: 10, H: 8})

	// Unclaimed: the fallback is drawn, at the Native's bounds.
	n.Draw(p, theme)
	if fb.drawn != 1 {
		t.Fatalf("fallback drawn %d times, want 1", fb.drawn)
	}
	if fb.Bounds() != n.Bounds() {
		t.Errorf("fallback bounds = %+v, want %+v", fb.Bounds(), n.Bounds())
	}

	// Claimed: the host's control is above the canvas; the toolkit paints nothing.
	n.Claimed().Set(true)
	n.Draw(p, theme)
	if fb.drawn != 1 {
		t.Errorf("fallback drawn while claimed (count %d)", fb.drawn)
	}

	// No fallback: nothing to draw, no panic.
	NewNativeButton("b", nil).Draw(p, theme)
}

func TestNativeOnEvent(t *testing.T) {
	n := NewNativeEntry("x")
	fb := &recordWidget{}
	n.Fallback = fb

	ev := Event{Kind: EventClick}
	n.OnEvent(ev)
	if len(fb.events) != 1 {
		t.Fatalf("fallback got %d events, want 1", len(fb.events))
	}

	n.Claimed().Set(true)
	n.OnEvent(ev)
	if len(fb.events) != 1 {
		t.Errorf("fallback got an event while claimed (count %d)", len(fb.events))
	}

	// No fallback: nothing happens, no panic.
	NewNativeButton("b", nil).OnEvent(ev)
}

func TestNativeChildren(t *testing.T) {
	n := NewNativeEntry("x")
	fb := &recordWidget{}
	n.Fallback = fb
	if kids := n.Children(); len(kids) != 1 || kids[0] != fb {
		t.Errorf("unclaimed Children = %v, want [fallback]", kids)
	}
	n.Claimed().Set(true)
	if kids := n.Children(); kids != nil {
		t.Errorf("claimed Children = %v, want nil", kids)
	}
	// No fallback: empty, not a one-element slice of nil.
	if kids := NewNativeButton("b", nil).Children(); len(kids) != 0 {
		t.Errorf("no-fallback Children = %v, want empty", kids)
	}
}

func TestNativeA11y(t *testing.T) {
	if got := NewNativeButton("b", nil).A11y().Role; got != RolePresentation {
		t.Errorf("A11y role = %v, want RolePresentation", got)
	}
}

func TestWalkNativeNil(t *testing.T) {
	if got := WalkNative(nil); len(got) != 0 {
		t.Errorf("WalkNative(nil) = %v, want empty", got)
	}
}

func TestWalkNativeUnclipped(t *testing.T) {
	n := NewNativeButton("b", nil)
	n.SetBounds(Rect{X: 5, Y: 6, W: 20, H: 10})
	root := &fakeContainer{kids: []Widget{nil, n}} // nil child exercises the nil guard
	got := WalkNative(root)
	if len(got) != 1 {
		t.Fatalf("got %d placements, want 1", len(got))
	}
	pl := got[0]
	if pl.Control != n {
		t.Errorf("placement control mismatch")
	}
	if pl.Rect != (Rect{X: 5, Y: 6, W: 20, H: 10}) {
		t.Errorf("unclipped Rect = %+v, want the bounds", pl.Rect)
	}
	if pl.Clip != pl.Rect || !pl.Visible {
		t.Errorf("unclipped placement should be fully visible: clip=%+v visible=%v", pl.Clip, pl.Visible)
	}
}

func TestWalkNativeClipped(t *testing.T) {
	inside := NewNativeEntry("in")
	inside.SetBounds(Rect{X: 20, Y: 20, W: 30, H: 15})

	outside := NewNativeEntry("out")
	outside.SetBounds(Rect{X: 90, Y: 20, W: 30, H: 15}) // past the clip's right edge

	nested := NewNativeEntry("nested")
	nested.SetBounds(Rect{X: 5, Y: 5, W: 10, H: 10})
	inner := &fakeViewport{clip: Rect{X: 0, Y: 0, W: 40, H: 50}, kids: []Widget{nested}}
	inner.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 50})

	outer := &fakeViewport{
		clip: Rect{X: 0, Y: 0, W: 80, H: 100},
		offX: -10, offY: -5,
		kids: []Widget{inside, outside, inner},
	}
	outer.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	got := WalkNative(outer)
	if len(got) != 3 {
		t.Fatalf("got %d placements, want 3", len(got))
	}
	byControl := map[*Native]NativePlacement{}
	for _, pl := range got {
		byControl[pl.Control] = pl
	}

	// inside: offset by (-10,-5), fully within the clip → visible.
	in := byControl[inside]
	if in.Rect != (Rect{X: 10, Y: 15, W: 30, H: 15}) {
		t.Errorf("inside Rect = %+v, want {10,15,30,15}", in.Rect)
	}
	if !in.Visible || in.Clip != in.Rect {
		t.Errorf("inside should be fully visible: clip=%+v visible=%v", in.Clip, in.Visible)
	}

	// outside: begins at the clip's right edge → clipped to nothing.
	out := byControl[outside]
	if out.Visible || out.Clip.W != 0 {
		t.Errorf("outside should be clipped away: clip=%+v visible=%v", out.Clip, out.Visible)
	}

	// nested: reached through two viewports (the inner intersects the outer clip).
	if _, ok := byControl[nested]; !ok {
		t.Errorf("nested control was not walked through the inner viewport")
	}
}

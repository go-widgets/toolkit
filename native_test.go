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
	li := NewNativeList([]string{"un", "deux", "trois"}, 1)
	if li.Kind != NativeList || li.Number().Get() != 1 || len(li.Items) != 3 {
		t.Errorf("list wrong")
	}
	// Nothing chosen is -1, not row zero: a list that claims a selection it
	// does not have makes every caller act on the wrong row.
	if none := NewNativeList([]string{"un"}, -1); none.Number().Get() != -1 {
		t.Errorf("a list with no selection reports row %v, want -1", none.Number().Get())
	}
}

// TestANativeListCarriesItsRowBothWays covers the one thing a list is for: the
// person moves the selection, and the model must hear about it.
func TestANativeListCarriesItsRowBothWays(t *testing.T) {
	li := NewNativeList([]string{"un", "deux", "trois"}, 0)
	li.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 60})

	got := WalkNative(li)
	if len(got) != 1 {
		t.Fatalf("a list produced %d descriptors, want 1", len(got))
	}
	c := got[0]
	if c.Kind != NativeList {
		t.Errorf("descriptor kind = %v, want NativeList", c.Kind)
	}
	if len(c.Items) != 3 {
		t.Errorf("descriptor carries %d items, want 3", len(c.Items))
	}
	// The host reports a new row through the descriptor; the observable is
	// what the application reads, so this is the whole two-way path.
	if c.OnNumber == nil {
		t.Fatal("a list descriptor has no way to report the chosen row")
	}
	c.OnNumber(2)
	if li.Number().Get() != 2 {
		t.Errorf("after the host reported row 2 the list holds %v", li.Number().Get())
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
	n.Key = "btn"
	n.SetBounds(Rect{X: 5, Y: 6, W: 20, H: 10})
	root := &fakeContainer{kids: []Widget{nil, n}} // nil child exercises the nil guard
	got := WalkNative(root)
	if len(got) != 1 {
		t.Fatalf("got %d descriptors, want 1", len(got))
	}
	pl := got[0]
	if pl.Key != "btn" || pl.Kind != NativeButton {
		t.Errorf("descriptor identity: key=%q kind=%v", pl.Key, pl.Kind)
	}
	if pl.Rect != (Rect{X: 5, Y: 6, W: 20, H: 10}) {
		t.Errorf("unclipped Rect = %+v, want the bounds", pl.Rect)
	}
	if pl.Clip != pl.Rect || !pl.Visible {
		t.Errorf("unclipped descriptor should be fully visible: clip=%+v visible=%v", pl.Clip, pl.Visible)
	}
}

func TestWalkNativeAdaptsValueAndCallbacks(t *testing.T) {
	activated := 0
	claimed := false
	n := NewNativeEntry("hello")
	n.Key = "e"
	n.SetOnActivate(func() { activated++ })
	n.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	n.Claimed().Subscribe(func(b bool) { claimed = b })

	got := WalkNative(&fakeContainer{kids: []Widget{n}})
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	c := got[0]
	if c.Text != "hello" {
		t.Errorf("descriptor Text = %q, want hello (read from the observable)", c.Text)
	}
	// The change callback writes back through the widget's observable.
	c.OnText("world")
	if n.Text().Get() != "world" {
		t.Errorf("OnText did not write back: observable = %q", n.Text().Get())
	}
	// Bool/Number callbacks are wired even for an entry (harmless lazy-create).
	c.OnBool(true)
	if !n.On().Get() {
		t.Error("OnBool did not write back")
	}
	c.OnNumber(3)
	if n.Number().Get() != 3 {
		t.Error("OnNumber did not write back")
	}
	c.OnActivate()
	if activated != 1 {
		t.Errorf("OnActivate ran %d times, want 1", activated)
	}
	c.OnClaim(true)
	if !claimed || !n.Claimed().Get() {
		t.Error("OnClaim did not set the widget claimed")
	}
}

func TestWalkNativeAdaptsBoolAndNumberValues(t *testing.T) {
	cb := NewNativeCheckbox("c", true)
	cb.Key = "c"
	cb.SetBounds(Rect{W: 10, H: 10})
	sl := NewNativeSlider(0, 10, 4)
	sl.Key = "s"
	sl.SetBounds(Rect{W: 10, H: 10})

	got := WalkNative(&fakeContainer{kids: []Widget{cb, sl}})
	byKey := map[string]NativeControl{}
	for _, pl := range got {
		byKey[pl.Key] = pl
	}
	if !byKey["c"].On {
		t.Error("checkbox descriptor On = false, want true (read from the observable)")
	}
	if byKey["s"].Number != 4 || byKey["s"].Min != 0 || byKey["s"].Max != 10 {
		t.Errorf("slider descriptor = %+v, want Number 4, Min 0, Max 10", byKey["s"])
	}
}

func TestWalkNativeSynthesisesKeyWhenUnset(t *testing.T) {
	// A widget-tree app need not name its controls: WalkNative gives an unkeyed
	// Native a stable per-widget identity.
	n := NewNativeButton("go", nil) // no Key
	n.SetBounds(Rect{W: 10, H: 10})
	got := WalkNative(&fakeContainer{kids: []Widget{n}})
	if len(got) != 1 {
		t.Fatalf("got %d, want 1", len(got))
	}
	if got[0].Key == "" {
		t.Error("unkeyed Native produced an empty descriptor Key; want a synthesised one")
	}
	// Stable across walks for the same widget.
	if again := WalkNative(&fakeContainer{kids: []Widget{n}}); again[0].Key != got[0].Key {
		t.Errorf("synthesised Key not stable: %q vs %q", again[0].Key, got[0].Key)
	}
}

func TestSurfaceNativeControls(t *testing.T) {
	// Nil source → nil, no panic.
	var s Surface
	if got := s.NativeControls(); got != nil {
		t.Errorf("nil-source NativeControls = %v, want nil", got)
	}
	// A set source is called through.
	s.Controls = func() []NativeControl {
		return []NativeControl{{Kind: NativeButton, Key: "ok"}}
	}
	got := s.NativeControls()
	if len(got) != 1 || got[0].Key != "ok" {
		t.Errorf("NativeControls = %+v, want one control keyed ok", got)
	}
}

func TestWalkNativeClipped(t *testing.T) {
	inside := NewNativeEntry("in")
	inside.Key = "in"
	inside.SetBounds(Rect{X: 20, Y: 20, W: 30, H: 15})

	outside := NewNativeEntry("out")
	outside.Key = "out"
	outside.SetBounds(Rect{X: 90, Y: 20, W: 30, H: 15}) // past the clip's right edge

	nested := NewNativeEntry("nested")
	nested.Key = "nested"
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
		t.Fatalf("got %d descriptors, want 3", len(got))
	}
	byKey := map[string]NativeControl{}
	for _, pl := range got {
		byKey[pl.Key] = pl
	}

	// inside: offset by (-10,-5), fully within the clip → visible.
	in := byKey["in"]
	if in.Rect != (Rect{X: 10, Y: 15, W: 30, H: 15}) {
		t.Errorf("inside Rect = %+v, want {10,15,30,15}", in.Rect)
	}
	if !in.Visible || in.Clip != in.Rect {
		t.Errorf("inside should be fully visible: clip=%+v visible=%v", in.Clip, in.Visible)
	}

	// outside: begins at the clip's right edge → clipped to nothing.
	out := byKey["out"]
	if out.Visible || out.Clip.W != 0 {
		t.Errorf("outside should be clipped away: clip=%+v visible=%v", out.Clip, out.Visible)
	}

	// nested: reached through two viewports (the inner intersects the outer clip).
	if _, ok := byKey["nested"]; !ok {
		t.Errorf("nested control was not walked through the inner viewport")
	}
}

// TestAControlCarriesItsMenuToTheHost covers the verbs of a row.
//
// A fixed row of buttons is a dialogue's shape: they must all fit, all the
// time, whether or not any applies to what is selected. A context menu is the
// other shape — the verbs that apply to THIS row, where the row is.
func TestAControlCarriesItsMenuToTheHost(t *testing.T) {
	picked := 0
	li := NewNativeList([]string{"un", "deux"}, 0)
	li.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 60})
	li.Menu = []NativeMenuItem{
		{Label: "Retry", Pick: func() { picked++ }},
		{}, // a separator has no name and needs none
		{Label: "Remove"},
	}

	got := WalkNative(li)
	if len(got) != 1 {
		t.Fatalf("a list produced %d descriptors", len(got))
	}
	if len(got[0].Menu) != 3 {
		t.Fatalf("the descriptor carries %d menu items, want 3", len(got[0].Menu))
	}
	if got[0].Menu[1].Label != "" {
		t.Errorf("the separator came through as %q", got[0].Menu[1].Label)
	}
	// A verb that does not apply right now is inert, not missing.
	if got[0].Menu[2].Pick != nil {
		t.Error("an item with no handler grew one")
	}
	got[0].Menu[0].Pick()
	if picked != 1 {
		t.Errorf("choosing the first item ran it %d times", picked)
	}
	// A control with no menu carries none, rather than an empty one a host
	// would have to tell apart from "the same menu as last frame".
	plain := NewNativeButton("x", nil)
	plain.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	if m := WalkNative(plain)[0].Menu; m != nil {
		t.Errorf("a control with no menu carries %v", m)
	}
}

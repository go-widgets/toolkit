package toolkit

import "testing"

// scrolledColumn builds a column of n entries, each 24 high, inside a scroll
// view only tall enough for two of them — the shape of every settings panel
// there has ever been.
func scrolledColumn(t *testing.T, n, viewH int) (*ScrollView, []*Entry) {
	t.Helper()
	col := NewVBox()
	col.Spacing = 0
	entries := make([]*Entry, n)
	for i := range entries {
		entries[i] = NewEntry("")
		col.AddFixed(entries[i], 24)
	}
	sv := &ScrollView{Child: col}
	sv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: viewH})
	col.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24 * n})
	sv.SetContentSize(120, 24*n)
	return sv, entries
}

func TestTheFocusWalkReachesThroughAScrollView(t *testing.T) {
	// Without this a ScrollView is a wall: a panel of controls in a scrolling
	// column has none of them reachable by Tab, however many there are.
	sv, entries := scrolledColumn(t, 3, 100)
	got := focusListOf(sv)
	if len(got) != len(entries) {
		t.Fatalf("the walk reaches %d of %d controls through a ScrollView",
			len(got), len(entries))
	}
}

func TestTheFocusWalkReachesThroughAFormField(t *testing.T) {
	// A FormField is a label and an input. Focus belongs on the input; without
	// this the walk stops at the label and every labelled field is skipped.
	in := NewEntry("")
	in.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	ff := &FormField{Label: "Name", Child: in}
	ff.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 44})
	got := focusListOf(ff)
	if len(got) != 1 || got[0] != Widget(in) {
		t.Fatalf("the walk reaches %d controls through a FormField, want the input", len(got))
	}
}

func TestTabbingBelowTheFoldScrollsTheControlIntoSight(t *testing.T) {
	// Reaching a control by Tab and leaving it out of sight would trade one
	// defect for a worse one: a cursor blinking where nobody can see it. Ten
	// controls in a view three deep leaves eight below the fold.
	sv, entries := scrolledColumn(t, 10, 72)
	root := NewVBox()
	root.AddFlex(sv, 1)
	root.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 72})
	sv.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 72})
	sv.Child.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 240})
	sv.SetContentSize(120, 240)

	vp := sv.viewport()
	for i := range entries {
		root.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		f := focusedInList(focusListOf(root))
		if f == nil {
			t.Fatalf("Tab %d focused nothing", i+1)
		}
		at := f.Bounds().Y - vp.Y
		off := sv.OffsetY().Get()
		if at < off || at+f.Bounds().H > off+vp.H {
			t.Errorf("Tab %d focused a control at %d..%d with the window at %d..%d: "+
				"it is out of sight", i+1, at, at+f.Bounds().H, off, off+vp.H)
		}
	}
	if sv.OffsetY().Get() == 0 {
		t.Error("tabbing to the bottom of the column never scrolled")
	}
	// And back up again, which is the case a one-directional fix gets wrong.
	for range entries {
		root.OnEvent(Event{Kind: EventKeyDown, Code: "Shift+Tab"})
		f := focusedInList(focusListOf(root))
		at, off := f.Bounds().Y-vp.Y, sv.OffsetY().Get()
		if at < off || at+f.Bounds().H > off+vp.H {
			t.Errorf("Shift+Tab left a control at %d..%d out of the window at %d..%d",
				at, at+f.Bounds().H, off, off+vp.H)
		}
	}
}

func TestRevealDeltaAsksForTheRightMove(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		at, size, offset, extent int
		want                     int
	}{
		{"already inside", 10, 20, 0, 100, 0},
		{"before the window", 10, 20, 40, 100, -30},
		{"after the window", 150, 20, 0, 100, 70},
		{"exactly filling it", 0, 100, 0, 100, 0},
		{"taller than the window, and after it", 150, 300, 0, 100, 150},
		{"taller than the window, and before it", 10, 300, 40, 100, -30},
	} {
		if got := revealDelta(tc.at, tc.size, tc.offset, tc.extent); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestRevealingAsksNothingWhenNothingIsFocused(t *testing.T) {
	sv, _ := scrolledColumn(t, 4, 72)
	sv.Scroll(0, 20)
	was := sv.OffsetY().Get()
	sv.revealFocused()
	if sv.OffsetY().Get() != was {
		t.Errorf("the view scrolled to %d with nothing focused, from %d",
			sv.OffsetY().Get(), was)
	}
}

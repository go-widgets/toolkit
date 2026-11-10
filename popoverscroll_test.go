// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// A ScrollView paints its child translated, it does not move it, so a popup that
// is placed from the control's own bounds lands where the control would be
// unscrolled (go-widgets/toolkit#558: a drop-down list in a scrolled settings
// pane could not be reached with a pointer). Every test here SCROLLS FIRST, then
// opens the popup, and then uses the pointer where the popup is drawn.

const scrollOff = 300

// scrolledTarget puts target (h tall) in the middle of a column 624 pixels long,
// inside a 100-pixel viewport scrolled to it, under a PopoverHost on a taller
// surface (a popup hangs out of the viewport). The target is painted at the top
// of the viewport; its bounds say it is scrollOff pixels lower.
func scrolledTarget(target Widget, h int) (*PopoverHost, *ScrollView, RGBA) {
	green := RGB(0, 0xFF, 0)
	top, bottom := &filler{ink: RGB(0xFF, 0, 0)}, &filler{ink: green}
	col := NewVBox()
	col.AddFixed(top, scrollOff)
	col.AddFixed(target, h)
	col.AddFixed(bottom, 300)
	sv := NewScrollView(col)
	host := NewPopoverHost(sv)
	host.SetBounds(Rect{W: 200, H: 300})
	sv.SetBounds(Rect{W: 200, H: 100})
	col.SetBounds(Rect{W: 200, H: 624}) // a column cannot measure itself: it is placed
	sv.SetContentSize(200, 624)
	sv.OffsetY().Set(scrollOff)
	return host, sv, green
}

// popupFacts checks what must hold for one popup owner in a scrolled column.
func popupFacts(t *testing.T, host *PopoverHost, sv *ScrollView, raw Rect, behind RGBA) (pb Rect, row int, owner PopoverOwner) {
	t.Helper()
	if _, dy := sv.ChildOffset(); dy != -scrollOff {
		t.Fatalf("the column is not scrolled: %d", dy)
	}
	owners := PopoverOwners(host)
	if len(owners) != 1 {
		t.Fatalf("owners = %d", len(owners))
	}
	pb = owners[0].PopoverBounds()
	if pb.Y != raw.Y-scrollOff || pb.X != raw.X || pb.H != raw.H {
		t.Fatalf("the popup is at %+v, want the unscrolled %+v moved up by %d", pb, raw, scrollOff)
	}
	// drawn there, over the widget the column paints in that place
	th := DefaultDark()
	buf := make([]byte, 200*300*4)
	host.Draw(painterOver(buf, 200, 300), th)
	if got := pixelAt(buf, 200, 4, pb.Y+pb.H/2); got == behind {
		t.Errorf("the popup is not drawn where it is: the column shows through at y=%d", pb.Y+pb.H/2)
	}
	return pb, 0, owners[0]
}

func TestADropDownInAScrolledColumnIsReachedWithAPointer(t *testing.T) {
	dd := NewDropDown([]string{"three", "six", "nine", "twelve"}, 0)
	host, sv, green := scrolledTarget(dd, 24)
	dd.Open().Set(true)
	pb, _, _ := popupFacts(t, host, sv, dd.PopoverBounds(), green)
	row := pb.H / len(dd.Options)
	host.OnEvent(Event{Kind: EventClick, X: 10, Y: pb.Y + 2*row + row/2})
	if dd.Selected().Get() != 2 || dd.Open().Get() {
		t.Errorf("a click on the third row: selected %d, open %v", dd.Selected().Get(), dd.Open().Get())
	}
	// the wheel over the list scrolls the list, not the column
	dd.Open().Set(true)
	off := sv.OffsetY().Get()
	host.OnEvent(Event{Kind: EventScroll, X: 10, Y: pb.Y + 4, Delta: 1})
	if sv.OffsetY().Get() != off {
		t.Error("the wheel over the list scrolled the column behind it")
	}
	// a click outside the list closes it
	host.OnEvent(Event{Kind: EventClick, X: 150, Y: 250})
	if dd.Open().Get() {
		t.Error("a click elsewhere left the list open")
	}
}

func TestAFontPickerInAScrolledColumnIsReachedWithAPointer(t *testing.T) {
	f := NewFontPicker([]string{"Inter", "Georgia", "Fira Code"})
	host, sv, green := scrolledTarget(f, 24)
	f.family.Open().Set(true)
	pb, _, _ := popupFacts(t, host, sv, f.family.PopoverBounds(), green)
	row := pb.H / len(f.family.Options)
	host.OnEvent(Event{Kind: EventClick, X: 10, Y: pb.Y + 2*row + row/2})
	if f.Spec().Get().Family != "Fira Code" {
		t.Errorf("family %q", f.Spec().Get().Family)
	}
}

func TestAnInspectorRowInAScrolledPaneIsReachedWithAPointer(t *testing.T) {
	in := NewInspector([]InspectorRow{
		{Label: "a", ID: "a", Kind: InspectorText}, {Label: "b", ID: "b", Kind: InspectorText}, {Label: "c", ID: "c", Kind: InspectorText},
		{Label: "d", ID: "d", Kind: InspectorText}, {Label: "e", ID: "e", Kind: InspectorText},
		{Label: "Pos", ID: "pos", Kind: InspectorEnum, Choices: []string{"left", "right", "top"}},
	})
	sv := NewScrollView(in)
	host := NewPopoverHost(sv)
	host.SetBounds(Rect{W: 240, H: 300})
	sv.SetBounds(Rect{W: 240, H: 70})
	_, h := in.Measure(240, 0)
	sv.OffsetY().Set(h) // scrolled to the end: the enum row is the last
	dd := in.byID["pos"].enum
	dd.Open().Set(true)
	_, dy := sv.ChildOffset()
	if dy == 0 {
		t.Fatal("not scrolled")
	}
	owners := PopoverOwners(host)
	if len(owners) != 1 {
		t.Fatalf("owners %d", len(owners))
	}
	pb := owners[0].PopoverBounds()
	if raw := dd.PopoverBounds(); pb.Y != raw.Y+dy {
		t.Fatalf("popup at %d, want %d", pb.Y, raw.Y+dy)
	}
	row := pb.H / len(dd.Options)
	host.OnEvent(Event{Kind: EventClick, X: pb.X + 4, Y: pb.Y + row + row/2})
	if in.Get("pos") != "right" {
		t.Errorf("pos = %q", in.Get("pos"))
	}
}

// shifted is a container that paints its child (0, dy) from its bounds, as a
// ScrollView does, without the scrolling: it is the seam the popup walk reads.
type shifted struct {
	Base
	kid Widget
	dy  int
}

func (s *shifted) Children() []Widget      { return []Widget{s.kid} }
func (s *shifted) ChildOffset() (int, int) { return 0, s.dy }

// OnEvent hands the event to the child as the child's own frame sees it.
func (s *shifted) OnEvent(ev Event) {
	ev.Y -= s.dy
	s.kid.OnEvent(ev)
}

func TestARibbonMenuInAShiftedContainerIsWhereTheRibbonIs(t *testing.T) {
	run := &sampleRun{}
	r := run.ribbon()
	r.SetBounds(Rect{W: 1100, H: r.Height()})
	box := &shifted{kid: r, dy: 40}
	host := NewPopoverHost(box)
	host.SetBounds(Rect{W: 1100, H: 400})
	// the split menu is opened with a pointer, where the ribbon is PAINTED
	c := r.cells()[rtCell(r, "New slide")]
	host.OnEvent(Event{Kind: EventClick, X: c.arrow.X + c.arrow.W/2, Y: c.arrow.Y + c.arrow.H/2 + 40})
	owners := PopoverOwners(host)
	if len(owners) != 1 || !r.PopoverOpen() {
		t.Fatalf("owners %d", len(owners))
	}
	raw := r.PopoverBounds()
	if got := owners[0].PopoverBounds(); got.Y != raw.Y+40 {
		t.Fatalf("the menu is at %d, want %d", got.Y, raw.Y+40)
	}
	// the host draws it shifted and routes a click on a row there
	th := DefaultLight()
	buf := make([]byte, 1100*400*4)
	host.Draw(painterOver(buf, 1100, 400), th)
	pb := owners[0].PopoverBounds()
	if got := rtPixelAt(buf, 1100, pb.X+pb.W/2, pb.Y+pb.H-2); got != th.Surface {
		t.Errorf("the menu is not drawn where it is: %v", got)
	}
	_, rows := r.popupGeometry(r.pop, r.metrics())
	x, y := rtCenter(rows[1])
	host.OnEvent(Event{Kind: EventClick, X: x, Y: y + 40})
	if r.PopoverOpen() || len(run.ids) == 0 || run.ids[len(run.ids)-1] != "slide.code" {
		t.Errorf("a click on the shifted row ran %v", run.ids)
	}
}

// ComboBox and DatePicker are not PopoverOwners: they draw their lists from their
// own Draw, in the painter the ScrollView translated, so the list is PLACED right
// in a scrolled column. What they share with no scroll at all is that the list
// hangs out of the control's bounds, so a container routes a click on it to
// whatever lies there (go-widgets/toolkit#559); here a scrolled column opens each
// from a click on the control, painted where it is.
func TestAComboBoxAndADatePickerOpenFromAClickInAScrolledColumn(t *testing.T) {
	c := NewComboBox([]string{"alpha", "beta"})
	host, _, _ := scrolledTarget(c, 24)
	host.OnEvent(Event{Kind: EventClick, X: 10, Y: 10}) // the control is painted at the top of the viewport
	if !c.Open().Get() {
		t.Error("a click on the painted combo box did not open it")
	}
	dp := NewDatePicker(2026, 11, 1)
	host2, _, _ := scrolledTarget(dp, 24)
	host2.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if !dp.Open().Get() {
		t.Error("a click on the painted date picker did not open it")
	}
}

// plainPaint hides the painter's translation: a back-end that cannot translate
// draws the list unshifted, as before, and does not fail.
type plainPaint struct{ painter.Painter }

func TestAShiftedPopupOnAPainterThatCannotTranslate(t *testing.T) {
	dd := NewDropDown([]string{"a", "b"}, 0)
	host, _, _ := scrolledTarget(dd, 24)
	dd.Open().Set(true)
	buf := make([]byte, 200*300*4)
	host.Draw(plainPaint{painterOver(buf, 200, 300)}, DefaultDark())
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// widthSized reports a height that depends on its width, the way a card does:
// twice the rows it needs to wrap its content.
type widthSized struct {
	Base
	content int // total content width
}

func (x *widthSized) Measure(width int) int {
	if width <= 0 {
		return 0
	}
	rows := (x.content + width - 1) / width
	return rows * 10
}

// bothSized answers on both axes, the [Measurer] shape.
type bothSized struct {
	Base
	w, h int
}

func (x *bothSized) Measure(_, _ int) (int, int) { return x.w, x.h }

// TestAColumnSizesACardToWhatItMeasures, and re-measures it when the column is
// resized -- which is the whole point.
//
// A fixed size is right at one window size and wrong at every other. The
// measured case here is the one that was reported: a run of rows kept its height
// when the window shrank, so it overflowed and the button bar pinned below it
// was drawn over the text.
func TestAColumnSizesACardToWhatItMeasures(t *testing.T) {
	card := &widthSized{content: 600}
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(card)

	col.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 500})
	wide := card.Bounds().H
	if want := 10; wide != want {
		t.Errorf("at 600 wide the card is %d tall, want %d", wide, want)
	}

	col.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 500})
	narrow := card.Bounds().H
	if want := 30; narrow != want {
		t.Errorf("at 200 wide the card is %d tall, want %d", narrow, want)
	}
	if narrow <= wide {
		t.Error("the card did not re-measure when the column was resized")
	}
}

// TestNaturalDoesNotEatTheSlack: a natural child takes what it measures and no
// more, so a flex sibling still absorbs the rest. This is what keeps a button bar
// at the bottom of a column instead of half way up it.
func TestNaturalDoesNotEatTheSlack(t *testing.T) {
	card := &widthSized{content: 300}
	filler := &Label{}
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(card)
	col.AddFlex(filler, 1)
	col.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 400})

	if got := card.Bounds().H; got != 10 {
		t.Errorf("the natural child is %d tall, want its measured 10", got)
	}
	if got := filler.Bounds().H; got != 390 {
		t.Errorf("the flex child got %d of the remaining 390", got)
	}
	if card.Bounds().Y != 0 || filler.Bounds().Y != 10 {
		t.Errorf("the children are stacked wrong: %+v then %+v",
			card.Bounds(), filler.Bounds())
	}
}

// TestARowSizesAChildToWhatItMeasures: the horizontal counterpart, which can
// only use the two-axis Measurer -- a height at a width says nothing about a
// width.
func TestARowSizesAChildToWhatItMeasures(t *testing.T) {
	button := &bothSized{w: 80, h: 24}
	rest := &Label{}
	row := NewHBox()
	row.Spacing = 0
	row.AddNatural(button)
	row.AddFlex(rest, 1)
	row.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 40})

	if got := button.Bounds().W; got != 80 {
		t.Errorf("the natural child is %d wide, want its measured 80", got)
	}
	if got := rest.Bounds().W; got != 320 {
		t.Errorf("the flex child got %d of the remaining 320", got)
	}
	// A width-only measurer is no use on a row's main axis, so such a child
	// falls back to its own bounds rather than being given a height as a width.
	card := &widthSized{content: 600}
	row2 := NewHBox()
	row2.Spacing = 0
	row2.AddNatural(card)
	card.SetBounds(Rect{W: 55, H: 11})
	row2.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 40})
	if got := card.Bounds().W; got != 55 {
		t.Errorf("a width-only measurer in a row got %d wide, want its own 55", got)
	}
}

// TestNaturalFallsBackToTheChildsOwnBounds, and a child that measures nothing at
// all keeps whatever the caller asked for.
func TestNaturalFallsBackToTheChildsOwnBounds(t *testing.T) {
	plain := &Label{}
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(plain)
	// After the append: adding to an unsized box collapses its children, so a
	// caller sizing a child does it once the box holds it.
	plain.SetBounds(Rect{W: 100, H: 17})
	col.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	if got := plain.Bounds().H; got != 17 {
		t.Errorf("a plain child is %d tall, want the 17 it carried", got)
	}

	// Nothing to measure and no bounds either: the child keeps the equal-flex
	// share it would have had, rather than collapsing to nothing.
	empty := &Label{}
	col2 := NewVBox()
	col2.Spacing = 0
	col2.AddNatural(empty)
	col2.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})
	if got := empty.Bounds().H; got == 0 {
		t.Error("a child that measures nothing collapsed to no height at all")
	}
}

// TestAContainerItemCanAskToBeNatural: the same thing through the Container /
// Layout model, where it is a field on the Item.
func TestAContainerItemCanAskToBeNatural(t *testing.T) {
	card := &widthSized{content: 400}
	bar := &Label{}
	l := NewBoxLayout()
	l.Vertical = true
	l.Spacing = 0
	c := NewContainer(l)
	c.Add(Item{Widget: card, Natural: true})
	c.Add(Item{Widget: bar, Size: 40})
	c.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})

	if got := card.Bounds().H; got != 10 {
		t.Errorf("the natural item is %d tall, want its measured 10", got)
	}
	if got := bar.Bounds().Y; got != 10 {
		t.Errorf("the fixed item is at y=%d, want it right under the measured one", got)
	}

	// An explicit Size or Flex is an answer to the same question and wins.
	pinned := &widthSized{content: 400}
	c2 := NewContainer(l)
	c2.Add(Item{Widget: pinned, Natural: true, Size: 77})
	c2.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	if got := pinned.Bounds().H; got != 77 {
		t.Errorf("an explicit Size was overridden by Natural: %d", got)
	}
}

// TestASettingsGroupInAColumnIsSizedByItsRows ties it to the widget this was
// added for: the group already reports its own height, so a column asks it
// instead of the caller hand-summing row heights into a constant.
func TestASettingsGroupInAColumnIsSizedByItsRows(t *testing.T) {
	dd := NewDropDown([]string{"3", "6", "9"}, 0)
	dd.SetBounds(Rect{W: 80, H: 24})
	g := NewSettingsGroup("The desk",
		&SettingRow{Title: "Screens", Subtitle: "how many", Control: dd},
		NewSettingRow("Cover the menu bar", NewSwitch(true)),
	)
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(g)
	col.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 900})

	if got, want := g.Bounds().H, g.Measure(500); got != want {
		t.Errorf("the group is %d tall in the column, it measures %d", got, want)
	}
	if g.Bounds().H >= 900 {
		t.Error("the group took the whole column instead of what it needs")
	}
}

// TestAColumnFallsBackToTheTwoAxisMeasurer: a widget that answers on both axes
// but not "height at a width" (an AlignBox, a Padding) is still sized to what it
// measures in a column.
func TestAColumnFallsBackToTheTwoAxisMeasurer(t *testing.T) {
	both := &bothSized{w: 90, h: 24}
	rest := &Label{}
	col := NewVBox()
	col.Spacing = 0
	col.AddNatural(both)
	col.AddFlex(rest, 1)
	col.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})

	if got := both.Bounds().H; got != 24 {
		t.Errorf("the child is %d tall, want the 24 it measures", got)
	}
	if got := rest.Bounds().H; got != 176 {
		t.Errorf("the flex sibling got %d of the remaining 176", got)
	}
}

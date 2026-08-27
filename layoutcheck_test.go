// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

// slack is a container that hands its child WHATEVER rectangle it is told to,
// so a test can produce a badly laid-out tree on purpose.
type slack struct {
	Base
	child Widget
	give  Rect
}

func (s *slack) Children() []Widget { return nonNil(s.child) }
func (s *slack) SetBounds(r Rect) {
	s.Base.SetBounds(r)
	if s.child != nil {
		s.child.SetBounds(s.give)
	}
}

// TestLayoutProblemsFindsAChildOutsideItsParent, which is always a defect: a
// widget must not draw outside its bounds, so a child sticking out of its parent
// is drawing over a sibling or off the surface.
func TestLayoutProblemsFindsAChildOutsideItsParent(t *testing.T) {
	kid := NewLabel("out")
	bad := &slack{child: kid, give: Rect{X: 90, Y: 10, W: 50, H: 20}}
	bad.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	got := LayoutProblems(bad)
	if len(got) != 1 {
		t.Fatalf("found %d problems in a tree with one: %v", len(got), got)
	}
	if got[0].Kind != LayoutOutside {
		t.Errorf("reported %v", got[0].Kind)
	}
	if got[0].Widget != Widget(kid) || got[0].Parent != Widget(bad) {
		t.Error("the problem names the wrong widgets")
	}
	// The message says where to look and what the rectangles were: a report that
	// only says "wrong" sends a person back to the tree with nothing.
	msg := got[0].Error()
	for _, want := range []string{"slack", "Label", "outside its parent", "X:90"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the message does not say %q: %s", want, msg)
		}
	}

	// The same child, placed inside, is no problem at all.
	bad.give = Rect{X: 10, Y: 10, W: 50, H: 20}
	bad.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if got := LayoutProblems(bad); len(got) != 0 {
		t.Errorf("a child inside its parent was reported: %v", got)
	}
}

// TestLayoutProblemsFindsAChildNobodyPlaced.
//
// This is the shape of the defect that was in four widgets of this package: they
// positioned their children inside Draw and nowhere else, so until the first
// frame every child was at the origin with no size -- invisible to a click, to
// an accessibility walk and to a host sizing a window from the tree.
func TestLayoutProblemsFindsAChildNobodyPlaced(t *testing.T) {
	kid := NewLabel("unplaced")
	lazy := &slack{child: kid} // gives Rect{}
	lazy.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	got := LayoutProblems(lazy)
	if len(got) != 1 || got[0].Kind != LayoutUnplaced {
		t.Fatalf("found %v", got)
	}
	if !strings.Contains(got[0].Error(), "never placed") {
		t.Errorf("the message reads %q", got[0].Error())
	}

	// A parent with no rectangle of its own has nothing to place a child in, so
	// its empty children are not the parent's fault and are not reported. That
	// is how a hidden container leaves no leaf with stale bounds.
	lazy.SetBounds(Rect{})
	if got := LayoutProblems(lazy); len(got) != 0 {
		t.Errorf("an empty parent's empty child was reported: %v", got)
	}
}

// TestLayoutProblemsDescends: a defect several levels down is found, and the
// path says where.
func TestLayoutProblemsDescends(t *testing.T) {
	leaf := NewLabel("deep")
	inner := &slack{child: leaf, give: Rect{X: 500, Y: 0, W: 10, H: 10}}
	outer := &slack{child: inner, give: Rect{X: 0, Y: 0, W: 100, H: 100}}
	outer.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})

	got := LayoutProblems(outer)
	if len(got) != 1 {
		t.Fatalf("found %d problems: %v", len(got), got)
	}
	if n := strings.Count(got[0].Path, ">"); n != 2 {
		t.Errorf("the path %q does not name the two levels", got[0].Path)
	}
}

// TestLayoutProblemsOnATreeItCannotWalk: a leaf, and nil, are both silent.
func TestLayoutProblemsOnATreeItCannotWalk(t *testing.T) {
	if got := LayoutProblems(nil); got != nil {
		t.Errorf("a nil tree reported %v", got)
	}
	leaf := NewLabel("alone")
	leaf.SetBounds(Rect{W: 10, H: 10})
	if got := LayoutProblems(leaf); got != nil {
		t.Errorf("a widget with no children reported %v", got)
	}
	// A container that yields a nil child -- one built incrementally -- is not a
	// crash and not a problem.
	empty := &slack{}
	empty.SetBounds(Rect{W: 10, H: 10})
	if got := LayoutProblems(empty); got != nil {
		t.Errorf("a container with no child reported %v", got)
	}
}

// TestTheToolkitsOwnFormLaysOutCleanly is the check applied to the arrangement
// this was written for: a page of settings cards under a border layout, which is
// what an application builds. Nothing in it may be outside its parent.
func TestTheToolkitsOwnFormLaysOutCleanly(t *testing.T) {
	dd := NewDropDown([]string{"3", "6", "9"}, 0)
	dd.SetBounds(Rect{W: 80, H: 24})
	card := NewSettingsGroup("The desk",
		&SettingRow{Title: "Screens", Subtitle: "how many", Control: dd},
		NewSettingRow("Cover the menu bar", NewSwitch(true)),
	)
	l := NewBoxLayout()
	l.Vertical = true
	l.Spacing = 8
	page := NewContainer(l)
	page.Add(Item{Widget: card})

	bar := NewContainer(NewBoxLayout())
	bar.Add(Item{Widget: NewButton("Save", nil), Size: 96})
	frame := NewContainer(BorderLayout{})
	frame.Add(Item{Widget: bar, Region: RegionSouth, Size: 40})
	frame.Add(Item{Widget: page, Region: RegionCenter})
	frame.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})

	for _, p := range LayoutProblems(frame) {
		if p.Kind == LayoutOutside {
			t.Errorf("%v", p)
		}
	}
}

// holesInIt is a container of the kind an APPLICATION may write: it yields a nil
// child, which no container in this package does but nothing stops one outside
// it from doing.
type holesInIt struct{ Base }

func (h *holesInIt) Children() []Widget { return []Widget{nil, NewLabel("real")} }

// TestLayoutProblemsSurvivesANilChild: the check walks a foreign tree, so it
// must not be the thing that crashes when the tree is odd. A nil child is
// skipped, and the real one beside it is still examined.
func TestLayoutProblemsSurvivesANilChild(t *testing.T) {
	h := &holesInIt{}
	h.SetBounds(Rect{X: 0, Y: 0, W: 50, H: 50})
	got := LayoutProblems(h)
	if len(got) != 1 {
		t.Fatalf("found %d problems, want the one real unplaced child: %v", len(got), got)
	}
	if got[0].Kind != LayoutUnplaced {
		t.Errorf("reported %v", got[0].Kind)
	}
}

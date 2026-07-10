// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// popoverStubChild is a Widget that records the last event it saw so
// tests can verify Popover.OnEvent's forwarding + coordinate translation.
type popoverStubChild struct {
	Base
	events    []Event
	drawCount int
}

func (s *popoverStubChild) Draw(p painter.Painter, theme *Theme) {
	_ = p
	_ = theme
	s.drawCount++
}

func (s *popoverStubChild) OnEvent(ev Event) {
	s.events = append(s.events, ev)
}

// --- Constructor ---------------------------------------------------------

func TestNewPopoverDefaults(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	if p.Visible {
		t.Fatal("fresh Popover must be hidden")
	}
	if p.Child != child {
		t.Fatal("NewPopover did not carry Child")
	}
	if p.Title != "" {
		t.Fatalf("Title = %q, want empty", p.Title)
	}
}

// --- Draw: !Visible is a no-op ------------------------------------------

func TestPopoverDrawHiddenNoOp(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	surf := makeSurface(120, 60)
	before := make([]byte, len(surf))
	copy(before, surf)
	p.Draw(newP(surf, 120), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw on hidden Popover touched byte %d", i)
		}
	}
	if child.drawCount != 0 {
		t.Fatalf("Child.Draw ran %d times on hidden Popover", child.drawCount)
	}
}

// --- Draw: Visible without title paints Surface + Border + Child ------

func TestPopoverDrawVisibleWithoutTitle(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.Visible = true
	p.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	theme := DefaultLight()
	buf := makeSurface(120, 60)
	p.Draw(newP(buf, 120), theme)
	// Interior fill is Surface.
	if pixelAt(buf, 120, 60, 30) != theme.Surface {
		t.Fatalf("interior fill = %+v, want Surface", pixelAt(buf, 120, 30, 30))
	}
	// Top-left border pixel is Border.
	if pixelAt(buf, 120, 0, 0) != theme.Border {
		t.Fatalf("border pixel = %+v, want Border", pixelAt(buf, 120, 0, 0))
	}
	if child.drawCount != 1 {
		t.Fatalf("Child.Draw ran %d times, want 1", child.drawCount)
	}
	// Child bounds should be inset by PopoverPad on all sides + no
	// header (Title is empty).
	want := Rect{
		X: PopoverPadX,
		Y: PopoverPadY,
		W: 120 - 2*PopoverPadX,
		H: 60 - 2*PopoverPadY,
	}
	if child.Bounds() != want {
		t.Fatalf("Child.Bounds = %+v, want %+v", child.Bounds(), want)
	}
}

// --- Draw: with Title header ------------------------------------------

func TestPopoverDrawVisibleWithTitle(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.Visible = true
	p.Title = "Menu"
	p.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	theme := DefaultLight()
	buf := makeSurface(120, 80)
	p.Draw(newP(buf, 120), theme)
	// Look for the title ink (OnSurface) among the top rows.
	found := false
	for y := 0; y < GlyphHeight()+PopoverPadY+2 && !found; y++ {
		for x := 0; x < 120; x++ {
			if pixelAt(buf, 120, x, y) == theme.OnSurface {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnSurface title glyph pixel found in header strip")
	}
	// Child bounds should be pushed down by the header height.
	wantY := PopoverPadY + GlyphHeight() + PopoverPadY
	if child.Bounds().Y != wantY {
		t.Fatalf("Child.Bounds.Y = %d, want %d", child.Bounds().Y, wantY)
	}
}

// --- Draw: nil Child is still valid -----------------------------------

func TestPopoverDrawVisibleNilChild(t *testing.T) {
	p := NewPopover(nil)
	p.Visible = true
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	theme := DefaultLight()
	buf := makeSurface(60, 40)
	// Just prove it does not panic and still paints the frame.
	p.Draw(newP(buf, 60), theme)
	if pixelAt(buf, 60, 30, 20) != theme.Surface {
		t.Fatalf("nil-Child Popover interior = %+v, want Surface", pixelAt(buf, 60, 30, 20))
	}
}

// --- Draw: zero-width bounds is a no-op ------------------------------

func TestPopoverDrawZeroWidthBounds(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.Visible = true
	p.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 40})
	surf := makeSurface(20, 40)
	before := make([]byte, len(surf))
	copy(before, surf)
	// Child still runs Draw (with a zero-sized rect); the painter's
	// clipping keeps its output out of the surface.
	p.Draw(newP(surf, 20), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("zero-width Popover Draw painted byte %d", i)
		}
	}
}

// --- OnEvent: !Visible drops the event ------------------------------

func TestPopoverOnEventWhenHidden(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if len(child.events) != 0 {
		t.Fatalf("hidden Popover forwarded %d events", len(child.events))
	}
}

// --- OnEvent: nil Child is inert ------------------------------------

func TestPopoverOnEventNilChild(t *testing.T) {
	p := NewPopover(nil)
	p.Visible = true
	p.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	// Just prove it does not panic.
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
}

// --- OnEvent: click forwards to Child with translated coords -------

func TestPopoverOnEventForwardsToChild(t *testing.T) {
	child := &popoverStubChild{}
	p := NewPopover(child)
	p.Visible = true
	p.SetBounds(Rect{X: 100, Y: 200, W: 120, H: 60})
	// After Draw the Popover lays out its child at inset bounds.
	buf := makeSurface(300, 300)
	p.Draw(newP(buf, 300), DefaultLight())
	// Widget-local (in Popover frame) event at (10, 10).
	p.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if len(child.events) != 1 {
		t.Fatalf("child got %d events, want 1", len(child.events))
	}
	got := child.events[0]
	// Surface coords of the event: (100+10, 200+10) = (110, 210).
	// Child bounds: (100+PopoverPadX, 200+PopoverPadY, ...).
	// Child-local: (110 - (100+PopoverPadX), 210 - (200+PopoverPadY))
	wantX := 10 - PopoverPadX
	wantY := 10 - PopoverPadY
	if got.X != wantX || got.Y != wantY {
		t.Fatalf("child event coords = (%d,%d), want (%d,%d)", got.X, got.Y, wantX, wantY)
	}
	if got.Kind != EventClick {
		t.Fatalf("child event kind = %d, want EventClick", got.Kind)
	}
}

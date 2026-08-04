// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- construction ----------------------------------------------------------

func TestNewAccordionStartsAllCollapsed(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}})
	if a.Expanded != -1 {
		t.Fatalf("Expanded = %d, want -1", a.Expanded)
	}
	if a.Multiple {
		t.Fatal("Multiple should default false")
	}
}

// --- exclusive toggling ------------------------------------------------------

func TestAccordionExclusiveExpandCollapsesSiblings(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 300})

	// Click header 0 (y in [0, ExpanderHeaderH)) -> expands section 0.
	a.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if a.Expanded != 0 {
		t.Fatalf("Expanded = %d, want 0", a.Expanded)
	}

	// Click header 1: with section 0 expanded, header 1 sits below
	// section 0's body (which occupies the whole remaining space),
	// i.e. at y = 2*ExpanderHeaderH + remaining. Compute it via
	// sectionRects so the test doesn't hardcode the layout formula.
	headers, _ := a.sectionRects()
	h1 := headers[1]
	a.OnEvent(Event{Kind: EventClick, X: h1.X + 5, Y: h1.Y + 5})
	if a.Expanded != 1 {
		t.Fatalf("Expanded = %d, want 1 (section 0 should have collapsed)", a.Expanded)
	}

	// Clicking the same (now open) header again fully collapses.
	headers, _ = a.sectionRects()
	h1 = headers[1]
	a.OnEvent(Event{Kind: EventClick, X: h1.X + 5, Y: h1.Y + 5})
	if a.Expanded != -1 {
		t.Fatalf("Expanded = %d, want -1 after re-clicking open header", a.Expanded)
	}
}

// --- Multiple mode -----------------------------------------------------------

func TestAccordionMultipleTogglesIndependently(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}})
	a.Multiple = true
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 200})

	// Expand section 0.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if !a.isExpanded(0) {
		t.Fatal("section 0 should be expanded")
	}

	// Expand section 1 too — must NOT collapse section 0.
	headers, _ := a.sectionRects()
	h1 := headers[1]
	a.OnEvent(Event{Kind: EventClick, X: h1.X + 5, Y: h1.Y + 5})
	if !a.isExpanded(0) || !a.isExpanded(1) {
		t.Fatalf("both sections should stay expanded independently: 0=%v 1=%v", a.isExpanded(0), a.isExpanded(1))
	}

	// Collapse section 0 independently.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if a.isExpanded(0) {
		t.Fatal("section 0 should have collapsed")
	}
	if !a.isExpanded(1) {
		t.Fatal("section 1 should remain expanded")
	}
}

// TestAccordionMultipleSplitsRemainingSpace pins the remainder-
// absorption rule: with 2 sections expanded, the last expanded
// section gets the odd leftover pixel.
func TestAccordionMultipleSplitsRemainingSpace(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}})
	a.Multiple = true
	// H = 2*ExpanderHeaderH (headers) + 5 (remaining, odd).
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 2*ExpanderHeaderH + 5})
	a.multiExpanded = map[int]bool{0: true, 1: true}

	_, bodies := a.sectionRects()
	if bodies[0].H != 2 {
		t.Fatalf("first expanded body H = %d, want 2 (5/2 floor)", bodies[0].H)
	}
	if bodies[1].H != 3 {
		t.Fatalf("last expanded body H = %d, want 3 (2 + remainder 1)", bodies[1].H)
	}
	if bodies[0].Y != ExpanderHeaderH {
		t.Fatalf("first body Y = %d, want %d", bodies[0].Y, ExpanderHeaderH)
	}
	if bodies[1].Y != bodies[0].Y+bodies[0].H+ExpanderHeaderH {
		t.Fatalf("second body Y = %d, want right after first body + its own header", bodies[1].Y)
	}
}

// --- body click routing -------------------------------------------------------

func TestAccordionBodyClickRoutesToExpandedSection(t *testing.T) {
	body := &recordingWidget{}
	a := NewAccordion([]AccordionSection{{Title: "A", Body: body}})
	a.SetBounds(Rect{X: 30, Y: 20, W: 200, H: 100})
	a.Expanded = 0

	// Body spans [ExpanderHeaderH, H) in widget-local space.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: ExpanderHeaderH + 10})
	if len(body.events) != 1 {
		t.Fatalf("body click routed %d events, want 1", len(body.events))
	}
	got := body.events[0]
	if got.X != 5 || got.Y != 10 {
		t.Fatalf("content received %+v, want {5,10} (translated past header + origin)", got)
	}
}

func TestAccordionBodyClickIgnoredWhenNilBody(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A", Body: nil}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	a.Expanded = 0
	// Must not panic; click lands inside the (nil-bodied) expanded
	// section's body rect.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: ExpanderHeaderH + 5})
}

func TestAccordionBodyClickIgnoredWhenCollapsed(t *testing.T) {
	body := &recordingWidget{}
	a := NewAccordion([]AccordionSection{{Title: "A", Body: body}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// Expanded stays -1: clicking where the body WOULD be must be a
	// no-op, since bodies[0].H is 0 while collapsed.
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: ExpanderHeaderH + 5})
	if len(body.events) != 0 {
		t.Fatal("collapsed section body must not receive events")
	}
}

// --- misc no-ops --------------------------------------------------------------

func TestAccordionIgnoresNonClick(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// Enter/Space toggle the focused section as of Wave 3; an unrelated key
	// (Tab) must not.
	a.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if a.Expanded != -1 {
		t.Fatal("KeyDown must not toggle a section")
	}
}

func TestAccordionClickOutsideAnyHeaderIsNoOp(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}})
	// Bounds taller than the single header + nothing expanded, so
	// there's dead space below the header that belongs to no rect.
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 90})
	if a.Expanded != -1 {
		t.Fatalf("Expanded = %d, want -1 (click landed outside any header/body)", a.Expanded)
	}
}

func TestAccordionEmptySectionsNoPanic(t *testing.T) {
	a := NewAccordion(nil)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	a.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	a.Draw(newP(makeSurface(100, 100), 100), DefaultLight())
}

// --- rendering -----------------------------------------------------------------

func TestAccordionDrawAllCollapsed(t *testing.T) {
	const w, h = 200, 300
	theme := DefaultLight()
	bodyA := &recordingWidget{}
	bodyB := &recordingWidget{}
	a := NewAccordion([]AccordionSection{
		{Title: "A", Body: bodyA},
		{Title: "B", Body: bodyB},
	})
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	a.Draw(newP(makeSurface(w, h), w), theme)
	if bodyA.draws != 0 || bodyB.draws != 0 {
		t.Fatal("no section is expanded (-1): neither body should draw")
	}
	// Both header backgrounds must be painted at their respective rows.
	buf := makeSurface(w, h)
	a.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 100, 2) != theme.SurfaceAlt {
		t.Fatalf("header 0 background = %+v, want SurfaceAlt", pixelAt(buf, w, 100, 2))
	}
	if pixelAt(buf, w, 100, ExpanderHeaderH+2) != theme.SurfaceAlt {
		t.Fatalf("header 1 background = %+v, want SurfaceAlt", pixelAt(buf, w, 100, ExpanderHeaderH+2))
	}
}

func TestAccordionDrawExpandedSectionRendersBody(t *testing.T) {
	const w, h = 200, 300
	theme := DefaultLight()
	bodyA := &recordingWidget{}
	bodyB := &recordingWidget{}
	a := NewAccordion([]AccordionSection{
		{Title: "A", Body: bodyA},
		{Title: "B", Body: bodyB},
	})
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	a.Expanded = 0
	a.Draw(newP(makeSurface(w, h), w), theme)
	if bodyA.draws != 1 {
		t.Fatalf("expanded body A draws = %d, want 1", bodyA.draws)
	}
	if bodyB.draws != 0 {
		t.Fatalf("collapsed body B draws = %d, want 0", bodyB.draws)
	}
}

func TestAccordionDrawNilBodyNoPanic(t *testing.T) {
	const w, h = 100, 100
	a := NewAccordion([]AccordionSection{{Title: "A", Body: nil}})
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	a.Expanded = 0
	a.Draw(newP(makeSurface(w, h), w), DefaultLight())
}

// TestAccordionDrawZeroRemainingSpaceSkipsBody covers the sectionRects
// "remaining < 0 -> clamp to 0" branch + the Draw "br.H == 0 -> skip"
// branch: Bounds is exactly tall enough for the header row alone, so
// even though the section is "expanded" it has no body pixels.
func TestAccordionDrawZeroRemainingSpaceSkipsBody(t *testing.T) {
	body := &recordingWidget{}
	a := NewAccordion([]AccordionSection{{Title: "A", Body: body}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: ExpanderHeaderH}) // no room left over
	a.Expanded = 0
	a.Draw(newP(makeSurface(100, ExpanderHeaderH), 100), DefaultLight())
	if body.draws != 0 {
		t.Fatal("zero-height body rect must not be drawn")
	}
}

// TestAccordionSectionRectsNegativeRemainingClamped exercises Bounds
// shorter than the header stack itself (more sections than fit).
func TestAccordionSectionRectsNegativeRemainingClamped(t *testing.T) {
	a := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: ExpanderHeaderH}) // shorter than 3 headers
	a.Expanded = 0
	_, bodies := a.sectionRects()
	if bodies[0].H != 0 {
		t.Fatalf("body H = %d, want 0 (clamped, negative remaining)", bodies[0].H)
	}
}

func TestAccordionDrawChevronDirection(t *testing.T) {
	const w, h = 100, 100
	theme := DefaultLight()
	a := NewAccordion([]AccordionSection{{Title: "A"}})
	a.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})

	collapsed := makeSurface(w, h)
	a.Draw(newP(collapsed, w), theme)

	a.Expanded = 0
	expanded := makeSurface(w, h)
	a.Draw(newP(expanded, w), theme)

	// The chevron pixels differ in shape between states; at minimum
	// the two renders must not be byte-identical near the chevron
	// origin (cx=6, cy=ExpanderHeaderH/2).
	same := true
	for y := 0; y < ExpanderHeaderH; y++ {
		for x := 0; x < 16; x++ {
			if pixelAt(collapsed, w, x, y) != pixelAt(expanded, w, x, y) {
				same = false
			}
		}
	}
	if same {
		t.Fatal("expanded vs collapsed chevron rendering should differ")
	}
}

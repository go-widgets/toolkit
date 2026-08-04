// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor -----------------------------------------------------------

func TestNewWizardStoresSteps(t *testing.T) {
	steps := []WizardStep{{Title: "One"}, {Title: "Two"}}
	w := NewWizard(steps)
	if len(w.Steps) != 2 || w.Current != 0 {
		t.Fatalf("NewWizard round-trip broken: %+v", w)
	}
}

// --- Next() ------------------------------------------------------------------

func TestWizardNextBlockedByCanAdvanceFalse(t *testing.T) {
	w := NewWizard([]WizardStep{
		{Title: "A", CanAdvance: func() bool { return false }},
		{Title: "B"},
	})
	w.Next()
	if w.Current != 0 {
		t.Fatalf("Current = %d, want 0 (blocked by CanAdvance)", w.Current)
	}
}

func TestWizardNextAllowedWhenCanAdvanceTrue(t *testing.T) {
	w := NewWizard([]WizardStep{
		{Title: "A", CanAdvance: func() bool { return true }},
		{Title: "B"},
	})
	w.Next()
	if w.Current != 1 {
		t.Fatalf("Current = %d, want 1", w.Current)
	}
}

func TestWizardNextAllowedWhenCanAdvanceNil(t *testing.T) {
	w := NewWizard([]WizardStep{
		{Title: "A"},
		{Title: "B"},
	})
	w.Next()
	if w.Current != 1 {
		t.Fatalf("Current = %d, want 1 (nil CanAdvance == always allowed)", w.Current)
	}
}

func TestWizardNextOnLastStepInvokesOnFinish(t *testing.T) {
	finished := 0
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.Current = 1 // last step
	w.OnFinish = func() { finished++ }
	w.Next()
	if finished != 1 {
		t.Fatalf("OnFinish called %d times, want 1", finished)
	}
	if w.Current != 1 {
		t.Fatalf("Current = %d, want unchanged 1 (Next() must not push out of range)", w.Current)
	}
}

func TestWizardNextOnLastStepWithNilOnFinishNoPanic(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}})
	w.Next() // must not panic; OnFinish is nil
	if w.Current != 0 {
		t.Fatalf("Current = %d, want 0", w.Current)
	}
}

// Next() on the last step finishes unconditionally — even when that
// step's own CanAdvance would forbid moving on. The gate only governs
// advancing to a next step, not the finish action.
func TestWizardNextOnLastStepIgnoresCanAdvance(t *testing.T) {
	finished := 0
	w := NewWizard([]WizardStep{
		{Title: "A"},
		{Title: "B", CanAdvance: func() bool { return false }},
	})
	w.Current = 1
	w.OnFinish = func() { finished++ }
	w.Next()
	if finished != 1 {
		t.Fatalf("OnFinish called %d times, want 1 (Next() must not gate finish on CanAdvance)", finished)
	}
}

// --- Back() ------------------------------------------------------------------

func TestWizardBackDecrements(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.Current = 1
	w.Back()
	if w.Current != 0 {
		t.Fatalf("Current = %d, want 0", w.Current)
	}
}

func TestWizardBackClampsAtZero(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.Back()
	w.Back()
	w.Back()
	if w.Current != 0 {
		t.Fatalf("Current = %d, want 0 (clamped, never negative)", w.Current)
	}
}

// --- nextButtonLabel / currentCanAdvance helpers ----------------------------

func TestWizardNextButtonLabelSwitchesToFinishOnLastStep(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	if got := w.nextButtonLabel(); got != "Next" {
		t.Fatalf("label at step 0 = %q, want Next", got)
	}
	w.Current = 1
	if got := w.nextButtonLabel(); got != "Finish" {
		t.Fatalf("label at last step = %q, want Finish", got)
	}
}

func TestWizardCurrentCanAdvanceOutOfRangeDefaultsTrue(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}})
	w.Current = 5 // out of range on the high side
	if !w.currentCanAdvance() {
		t.Fatal("currentCanAdvance() with Current out of range should default true")
	}
	w.Current = -1 // out of range on the low side
	if !w.currentCanAdvance() {
		t.Fatal("currentCanAdvance() with negative Current should default true")
	}
}

// --- Draw ----------------------------------------------------------------

func TestWizardDrawEmptyStepsNoop(t *testing.T) {
	const wpx, hpx = 200, 200
	theme := DefaultLight()
	w := NewWizard(nil)
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme)
	for y := 0; y < hpx; y++ {
		for x := 0; x < wpx; x++ {
			if pixelAt(buf, wpx, x, y).R != 0xC8 {
				t.Fatalf("empty Wizard painted (%d,%d) = %+v", x, y, pixelAt(buf, wpx, x, y))
			}
		}
	}
}

func TestWizardDrawPaintsStripAndBody(t *testing.T) {
	const wpx, hpx = 300, 240
	theme := DefaultLight()
	body := &spyWidget{}
	w := NewWizard([]WizardStep{
		{Title: "One", Body: body},
		{Title: "Two"},
	})
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme)

	// Strip badge 0 should be Accent (Current == 0). The strip band is
	// WizardStripH tall; Steps centres the badge row vertically inside it.
	yMid := (WizardStripH-StepBoxH)/2 + StepBoxH/2
	if pixelAt(buf, wpx, StepBoxW/2, yMid) != theme.Accent {
		t.Fatalf("strip badge0 = %+v, want Accent", pixelAt(buf, wpx, StepBoxW/2, yMid))
	}
	if body.drawCount != 1 {
		t.Fatalf("active step's Body.Draw called %d times, want 1", body.drawCount)
	}
	wantBounds := w.bodyRect()
	if body.Bounds() != wantBounds {
		t.Fatalf("Body bounds = %+v, want %+v", body.Bounds(), wantBounds)
	}
}

func TestWizardDrawSkipsNilBody(t *testing.T) {
	const wpx, hpx = 300, 240
	theme := DefaultLight()
	w := NewWizard([]WizardStep{{Title: "One"}}) // Body left nil
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme) // must not panic
}

func TestWizardDrawSkipsBodyWhenCurrentOutOfRange(t *testing.T) {
	const wpx, hpx = 300, 240
	theme := DefaultLight()
	body := &spyWidget{}
	w := NewWizard([]WizardStep{{Title: "One", Body: body}})
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	w.Current = 7 // out of range
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme)
	if body.drawCount != 0 {
		t.Fatalf("Body.Draw called %d times, want 0 (Current out of range)", body.drawCount)
	}
}

func TestWizardDrawBackButtonDimmedOnFirstStep(t *testing.T) {
	const wpx, hpx = 300, 240
	theme := DefaultLight()
	w := NewWizard([]WizardStep{{Title: "One"}, {Title: "Two"}})
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	br := w.backRect()
	// Sample near the left edge (inside the rounded fill, past the corner
	// radius, but clear of the centred "Back" label glyphs) so the pixel
	// reflects the button's face colour rather than its ink.
	cx, cy := br.X+10, br.Y+br.H/2

	// Current == 0: Back is dimmed (ButtonSecondary -> SurfaceAlt fill).
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme)
	if pixelAt(buf, wpx, cx, cy) != theme.SurfaceAlt {
		t.Fatalf("Back fill on first step = %+v, want SurfaceAlt (dimmed)", pixelAt(buf, wpx, cx, cy))
	}

	// Current == 1: Back is enabled (ButtonDefault -> Surface fill).
	w.Current = 1
	buf2 := makeSurface(wpx, hpx)
	w.Draw(newP(buf2, wpx), theme)
	if pixelAt(buf2, wpx, cx, cy) != theme.Surface {
		t.Fatalf("Back fill past first step = %+v, want Surface (enabled)", pixelAt(buf2, wpx, cx, cy))
	}
}

func TestWizardDrawNextButtonDimmedWhenCanAdvanceFalse(t *testing.T) {
	const wpx, hpx = 300, 240
	theme := DefaultLight()
	allowed := true
	w := NewWizard([]WizardStep{
		{Title: "One", CanAdvance: func() bool { return allowed }},
		{Title: "Two"},
	})
	w.SetBounds(Rect{X: 0, Y: 0, W: wpx, H: hpx})
	nr := w.nextRect()
	cx, cy := nr.X+nr.W/2, nr.Y+nr.H/2

	// Enabled: ButtonProminent -> Accent fill.
	buf := makeSurface(wpx, hpx)
	w.Draw(newP(buf, wpx), theme)
	if pixelAt(buf, wpx, cx, cy) != theme.Accent {
		t.Fatalf("Next fill when allowed = %+v, want Accent", pixelAt(buf, wpx, cx, cy))
	}

	// Disabled: ButtonSecondary -> SurfaceAlt fill.
	allowed = false
	buf2 := makeSurface(wpx, hpx)
	w.Draw(newP(buf2, wpx), theme)
	if pixelAt(buf2, wpx, cx, cy) != theme.SurfaceAlt {
		t.Fatalf("Next fill when disallowed = %+v, want SurfaceAlt (dimmed)", pixelAt(buf2, wpx, cx, cy))
	}
}

// --- OnEvent: button clicks --------------------------------------------------

func TestWizardOnEventEmptyStepsNoop(t *testing.T) {
	w := NewWizard(nil)
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	w.OnEvent(Event{Kind: EventClick, X: 10, Y: 10}) // must not panic
}

func TestWizardBackButtonClickCallsBack(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	w.Current = 1
	br := w.backRect()
	// Bounds().X/Y is 0, so widget-local == surface here.
	w.OnEvent(Event{Kind: EventClick, X: br.X + br.W/2, Y: br.Y + br.H/2})
	if w.Current != 0 {
		t.Fatalf("Current = %d, want 0 after Back click", w.Current)
	}
}

func TestWizardBackButtonDisabledClickNoop(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	br := w.backRect()
	w.OnEvent(Event{Kind: EventClick, X: br.X + br.W/2, Y: br.Y + br.H/2})
	if w.Current != 0 {
		t.Fatalf("Current = %d, want unchanged 0 (Back disabled on first step)", w.Current)
	}
}

func TestWizardNextButtonClickCallsNext(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	nr := w.nextRect()
	w.OnEvent(Event{Kind: EventClick, X: nr.X + nr.W/2, Y: nr.Y + nr.H/2})
	if w.Current != 1 {
		t.Fatalf("Current = %d, want 1 after Next click", w.Current)
	}
}

func TestWizardNextButtonDisabledClickNoop(t *testing.T) {
	w := NewWizard([]WizardStep{
		{Title: "A", CanAdvance: func() bool { return false }},
		{Title: "B"},
	})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	nr := w.nextRect()
	w.OnEvent(Event{Kind: EventClick, X: nr.X + nr.W/2, Y: nr.Y + nr.H/2})
	if w.Current != 0 {
		t.Fatalf("Current = %d, want unchanged 0 (Next disabled by CanAdvance)", w.Current)
	}
}

func TestWizardFinishButtonClickInvokesOnFinish(t *testing.T) {
	finished := 0
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	w.Current = 1 // last step: Next button reads "Finish"
	w.OnFinish = func() { finished++ }
	nr := w.nextRect()
	w.OnEvent(Event{Kind: EventClick, X: nr.X + nr.W/2, Y: nr.Y + nr.H/2})
	if finished != 1 {
		t.Fatalf("OnFinish called %d times, want 1", finished)
	}
}

// --- OnEvent: body click forwarding -----------------------------------------

func TestWizardBodyClickForwardedWithTranslatedCoords(t *testing.T) {
	body := &spyWidget{}
	w := NewWizard([]WizardStep{{Title: "A", Body: body}})
	w.SetBounds(Rect{X: 20, Y: 10, W: 300, H: 240})
	bodyR := w.bodyRect() // surface coords, since Bounds().X/Y != 0

	// Pick a surface point strictly inside the body area, then express
	// it as a Wizard-local event coordinate (subtract the Wizard's own
	// origin), matching the "events are widget-local" convention.
	surfaceX, surfaceY := bodyR.X+15, bodyR.Y+25
	localX, localY := surfaceX-w.Bounds().X, surfaceY-w.Bounds().Y
	w.OnEvent(Event{Kind: EventClick, X: localX, Y: localY})

	if body.evCount != 1 {
		t.Fatalf("Body.OnEvent called %d times, want 1", body.evCount)
	}
	wantLocalX := surfaceX - bodyR.X
	wantLocalY := surfaceY - bodyR.Y
	if body.lastEvent.X != wantLocalX || body.lastEvent.Y != wantLocalY {
		t.Fatalf("Body received local (%d,%d), want (%d,%d)",
			body.lastEvent.X, body.lastEvent.Y, wantLocalX, wantLocalY)
	}
}

func TestWizardClickInDeadZoneNoop(t *testing.T) {
	body := &spyWidget{}
	w := NewWizard([]WizardStep{{Title: "A", Body: body}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	// A point in the top strip band: not a button, not the body area.
	w.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if body.evCount != 0 {
		t.Fatalf("Body.OnEvent called %d times, want 0 (click in dead strip zone)", body.evCount)
	}
}

func TestWizardNonClickEventForwardedRegardlessOfPosition(t *testing.T) {
	body := &spyWidget{}
	w := NewWizard([]WizardStep{{Title: "A", Body: body}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	// X/Y target the strip band, well outside the body area — but a
	// non-click event always forwards to the active step (mirrors
	// Notebook's OnEvent in notebook.go).
	w.OnEvent(Event{Kind: EventKeyDown, X: 5, Y: 5, Code: "Enter"})
	if body.evCount != 1 {
		t.Fatalf("Body.OnEvent called %d times, want 1 (non-click always forwards)", body.evCount)
	}
	if body.lastEvent.Code != "Enter" {
		t.Fatalf("Body received Code = %q, want Enter", body.lastEvent.Code)
	}
}

func TestWizardOnEventSkipsForwardWhenCurrentOutOfRange(t *testing.T) {
	body := &spyWidget{}
	w := NewWizard([]WizardStep{{Title: "A", Body: body}})
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	w.Current = 9 // out of range
	w.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if body.evCount != 0 {
		t.Fatalf("Body.OnEvent called %d times, want 0 (Current out of range)", body.evCount)
	}
}

func TestWizardOnEventNilBodyNoPanic(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}}) // Body left nil
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 240})
	w.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // must not panic
}

// TestWizardButtonPressFeedback: clicking Back/Next arms the pressed feedback
// on that button, which EventMouseUp clears; PressFeedback=false opts out.
func TestWizardButtonPressFeedback(t *testing.T) {
	w := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	w.Current = 1 // on step B: Back enabled, Next enabled
	w.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})

	// Click Next → armed + advanced.
	nr := w.nextRect()
	w.OnEvent(Event{Kind: EventClick, X: nr.X + nr.W/2, Y: nr.Y + nr.H/2})
	if w.pressedBtn != wizBtnNext {
		t.Fatalf("Next click: pressedBtn=%d, want wizBtnNext", w.pressedBtn)
	}
	if w.Current != 2 {
		t.Fatalf("Next did not advance: Current=%d", w.Current)
	}
	// Draw renders the pressed Next button (covers the SetPressed branch).
	w.Draw(newP(makeSurface(300, 200), 300), DefaultLight())
	// Release clears it.
	w.OnEvent(Event{Kind: EventMouseUp})
	if w.pressedBtn != wizBtnNone {
		t.Fatal("EventMouseUp should clear the pressed button")
	}

	// Click Back (now on step C) → armed + moved back; Draw pressed.
	w.Current = 1
	br := w.backRect()
	w.OnEvent(Event{Kind: EventClick, X: br.X + br.W/2, Y: br.Y + br.H/2})
	if w.pressedBtn != wizBtnBack || w.Current != 0 {
		t.Fatalf("Back click: pressedBtn=%d Current=%d", w.pressedBtn, w.Current)
	}
	w.Draw(newP(makeSurface(300, 200), 300), DefaultLight())

	// PressFeedback=false opts out.
	w2 := NewWizard([]WizardStep{{Title: "A"}, {Title: "B"}})
	w2.Current, w2.PressFeedback = 1, false
	w2.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	br2 := w2.backRect()
	w2.OnEvent(Event{Kind: EventClick, X: br2.X + br2.W/2, Y: br2.Y + br2.H/2})
	if w2.pressedBtn != wizBtnNone {
		t.Fatal("PressFeedback=false should not arm the pressed button")
	}
}

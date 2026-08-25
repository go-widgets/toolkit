// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Dialog: close (×) button -------------------------------------------

// A Closable dialog draws a close button and a click on it fires OnClose.
func TestDialogCloseButtonFiresOnClose(t *testing.T) {
	closed := false
	d := NewDialog("Confirm", NewLabel("body"))
	d.Closable = true
	d.OnClose = func() { closed = true }
	d.SetBounds(Rect{X: 100, Y: 100, W: 300, H: 200})
	// The close button sits at the trailing edge of the title bar. Click its
	// centre, converted to dialog-local coordinates.
	cb := d.closeButton().Bounds()
	d.OnEvent(Event{Kind: EventClick, X: cb.X + cb.W/2 - 100, Y: cb.Y + cb.H/2 - 100})
	if !closed {
		t.Fatal("close button click did not fire OnClose")
	}
}

// A Closable dialog with a nil OnClose does not panic when its × is clicked.
func TestDialogCloseButtonNilOnCloseNoPanic(t *testing.T) {
	d := NewDialog("X", nil)
	d.Closable = true
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	cb := d.closeButton().Bounds()
	d.OnEvent(Event{Kind: EventClick, X: cb.X + cb.W/2, Y: cb.Y + cb.H/2})
}

// A click on a Closable dialog that misses the × still reaches the content, and
// the × is drawn.
func TestDialogClosableClickElsewhereReachesContent(t *testing.T) {
	body := &recordingWidget{}
	d := NewDialog("X", body)
	d.Closable = true
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	d.Draw(newP(makeSurface(400, 300), 400), DefaultLight())
	// Click well left of the trailing-edge close button, in the content body.
	d.OnEvent(Event{Kind: EventClick, X: 20, Y: 100})
	if len(body.events) != 1 {
		t.Fatalf("content event count = %d, want 1", len(body.events))
	}
}

// A non-Closable dialog builds no close button and ignores nothing extra — the
// existing content path is unchanged.
func TestDialogNotClosableNoCloseButton(t *testing.T) {
	d := NewDialog("X", nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	d.Draw(newP(makeSurface(400, 300), 400), DefaultLight())
	if d.closeBtn != nil {
		t.Fatal("a non-Closable dialog should not have built a close button")
	}
}

// --- Dialog: optional top input bar -------------------------------------

// With an Input set, keyboard characters and Backspace route to the field, and a
// click on the input strip focuses it.
func TestDialogInputReceivesKeyboardAndClickFocus(t *testing.T) {
	se := NewSearchEntry("")
	d := NewDialog("Find", NewLabel("results"))
	d.Input = se
	d.SetBounds(Rect{X: 10, Y: 10, W: 300, H: 220})

	// Typing routes to the field.
	d.OnEvent(Event{Kind: EventChar, Code: "h"})
	d.OnEvent(Event{Kind: EventChar, Code: "i"})
	if got := se.Text().Get(); got != "hi" {
		t.Fatalf("after typing, input text = %q, want %q", got, "hi")
	}
	// Backspace routes too.
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if got := se.Text().Get(); got != "h" {
		t.Fatalf("after backspace, input text = %q, want %q", got, "h")
	}
	// A click on the input strip focuses the field.
	se.SetFocused(false)
	ib := se.Bounds()
	d.OnEvent(Event{Kind: EventClick, X: ib.X + ib.W/2 - 10, Y: ib.Y + ib.H/2 - 10})
	if !se.Focused() {
		t.Fatal("click on the input strip did not focus the field")
	}
}

// A click that misses the input strip falls through to the content, and the
// input bar is drawn.
func TestDialogInputClickMissReachesContent(t *testing.T) {
	body := &recordingWidget{}
	se := NewSearchEntry("")
	d := NewDialog("Find", body)
	d.Input = se
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 220})
	d.Draw(newP(makeSurface(400, 300), 400), DefaultLight())
	// Click low in the content body, below both title bar and input strip.
	d.OnEvent(Event{Kind: EventClick, X: 40, Y: 150})
	if len(body.events) != 1 {
		t.Fatalf("content event count = %d, want 1", len(body.events))
	}
}

// The content body starts below the input strip when an Input is present, and
// fills to the title bar when it is not.
func TestDialogInputShiftsContentDown(t *testing.T) {
	body := &recordingWidget{}
	d := NewDialog("X", body)
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 220})
	noInputTop := body.Bounds().Y

	d.Input = NewSearchEntry("")
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 220})
	withInputTop := body.Bounds().Y
	if withInputTop != noInputTop+scaled(DialogInputH) {
		t.Fatalf("content top with input = %d, want %d (shifted by DialogInputH)",
			withInputTop, noInputTop+scaled(DialogInputH))
	}
}

// An Entry works as the input bar too (its Placeholder path), exercising the
// non-SearchEntry DialogInput.
func TestDialogInputAcceptsEntry(t *testing.T) {
	e := &Entry{Placeholder: "search…"}
	d := NewDialog("Find", nil)
	d.Input = e
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 220})
	d.Draw(newP(makeSurface(400, 300), 400), DefaultLight())
	d.OnEvent(Event{Kind: EventChar, Code: "a"})
	if got := e.Text().Get(); got != "a" {
		t.Fatalf("entry input text = %q, want %q", got, "a")
	}
}

// --- ModalWindow --------------------------------------------------------

func TestNewModalWindowShape(t *testing.T) {
	m := NewModalWindow("Title", NewLabel("body"), NewButton("OK", nil))
	if m.Scrim == nil || !m.Scrim.Interactive {
		t.Fatal("scrim missing or not interactive")
	}
	if m.Panel == nil || !m.Panel.Closable || m.Panel.Title != "Title" {
		t.Fatalf("panel = %+v, want a Closable dialog titled Title", m.Panel)
	}
	if !m.CloseOnScrim {
		t.Fatal("CloseOnScrim should default true from the constructor")
	}
	if len(m.Panel.Buttons) != 1 {
		t.Fatalf("panel buttons = %d, want 1", len(m.Panel.Buttons))
	}
}

// The panel's × button dismisses the modal via OnClose.
func TestModalWindowCloseButtonDismisses(t *testing.T) {
	closed := false
	m := NewModalWindow("T", NewLabel("b"))
	m.OnClose = func() { closed = true }
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	cb := m.Panel.closeButton().Bounds()
	// Modal-local click on the panel's close button (modal at origin).
	m.OnEvent(Event{Kind: EventClick, X: cb.X + cb.W/2, Y: cb.Y + cb.H/2})
	if !closed {
		t.Fatal("panel close button did not dismiss the modal")
	}
}

// Escape dismisses the modal.
func TestModalWindowEscapeDismisses(t *testing.T) {
	closed := false
	m := NewModalWindow("T", nil)
	m.OnClose = func() { closed = true }
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if !closed {
		t.Fatal("Escape did not dismiss the modal")
	}
}

// A click on the scrim outside the panel dismisses when CloseOnScrim is set.
func TestModalWindowScrimClickDismisses(t *testing.T) {
	closed := false
	m := NewModalWindow("T", nil)
	m.OnClose = func() { closed = true }
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	// Top-left corner is well outside the centred panel.
	m.OnEvent(Event{Kind: EventClick, X: 2, Y: 2})
	if !closed {
		t.Fatal("scrim click did not dismiss the modal")
	}
}

// With CloseOnScrim false, an outside click is swallowed (no dismissal).
func TestModalWindowScrimClickSwallowedWhenStrict(t *testing.T) {
	closed := false
	m := NewModalWindow("T", nil)
	m.CloseOnScrim = false
	m.OnClose = func() { closed = true }
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	m.OnEvent(Event{Kind: EventClick, X: 2, Y: 2})
	if closed {
		t.Fatal("a strict modal must not dismiss on an outside click")
	}
}

// A click inside the panel is forwarded to it (reaches a panel button).
func TestModalWindowClickInsidePanelForwarded(t *testing.T) {
	clicked := false
	ok := NewButton("OK", func() { clicked = true })
	m := NewModalWindow("T", NewLabel("b"), ok)
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	bb := ok.Bounds()
	m.OnEvent(Event{Kind: EventClick, X: bb.X + bb.W/2, Y: bb.Y + bb.H/2})
	if !clicked {
		t.Fatal("a click inside the panel did not reach the panel's button")
	}
}

// Non-click events (keyboard) forward to the panel — a typed character reaches
// the panel's input bar.
func TestModalWindowKeyboardForwardsToPanelInput(t *testing.T) {
	m, se := NewSearchModal("Find", NewLabel("results"))
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	if !se.Focused() {
		t.Fatal("NewSearchModal should focus the search field")
	}
	m.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := se.Text().Get(); got != "z" {
		t.Fatalf("search field text = %q, want %q", got, "z")
	}
}

// Escape with a nil OnClose does not panic; a nil-panel modal ignores events.
func TestModalWindowNilSafety(t *testing.T) {
	m := NewModalWindow("T", nil) // OnClose left nil
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"}) // dismiss with nil OnClose
	m.OnEvent(Event{Kind: EventClick, X: 2, Y: 2})       // scrim dismiss with nil OnClose

	// A hand-built modal with no scrim/panel must not panic on any path.
	bare := &ModalWindow{}
	bare.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	bare.Draw(newP(makeSurface(100, 100), 100), DefaultDark())
	bare.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	bare.OnEvent(Event{Kind: EventChar, Code: "q"})
	if len(bare.Children()) != 0 {
		t.Fatalf("bare modal children = %d, want 0", len(bare.Children()))
	}
}

// PanelW/PanelH override the default size, and the panel is clamped to a modal
// smaller than the requested panel.
func TestModalWindowPanelSizeAndClamp(t *testing.T) {
	m := NewModalWindow("T", nil)
	m.PanelW, m.PanelH = 200, 150
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	if pb := m.Panel.Bounds(); pb.W != scaled(200) || pb.H != scaled(150) {
		t.Fatalf("panel size = %dx%d, want %dx%d", pb.W, pb.H, scaled(200), scaled(150))
	}
	// A modal smaller than the panel clamps the panel to the modal.
	m.SetBounds(Rect{X: 0, Y: 0, W: scaled(120), H: scaled(90)})
	if pb := m.Panel.Bounds(); pb.W != scaled(120) || pb.H != scaled(90) {
		t.Fatalf("clamped panel size = %dx%d, want %dx%d", pb.W, pb.H, scaled(120), scaled(90))
	}
}

// Draw paints without panic, and the modal centres its panel.
func TestModalWindowDrawAndCentre(t *testing.T) {
	m := NewModalWindow("T", NewLabel("b"), NewButton("OK", nil))
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	m.Draw(newP(makeSurface(800, 600), 800), DefaultDark())
	pb := m.Panel.Bounds()
	if pb.X <= 0 || pb.Y <= 0 || pb.X+pb.W >= 800 {
		t.Fatalf("panel not centred within the modal: %+v", pb)
	}
}

// The modal is presentational and yields its scrim + panel to the a11y walk, so
// the panel's own dialog node (and its content) is what a reader sees.
func TestModalWindowAccessibility(t *testing.T) {
	m := NewModalWindow("Preferences", NewLabel("body"))
	m.SetBounds(Rect{X: 0, Y: 0, W: 800, H: 600})
	if got := m.A11y().Role; got != RolePresentation {
		t.Fatalf("modal Role = %q, want %q", got, RolePresentation)
	}
	kids := m.Children()
	if len(kids) != 2 || kids[0] != Widget(m.Scrim) || kids[1] != Widget(m.Panel) {
		t.Fatalf("children = %v, want [scrim panel]", kids)
	}
	// The walk skips the presentational modal + scrim and surfaces the panel's
	// dialog node named by its title.
	var dialogSeen bool
	for _, n := range WalkA11y(m) {
		if n.Role == RoleDialog && n.Name == "Preferences" {
			dialogSeen = true
		}
	}
	if !dialogSeen {
		t.Fatal("the panel's RoleDialog node was not reached through the modal")
	}
}

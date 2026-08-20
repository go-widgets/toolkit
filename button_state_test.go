// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestButtonSelectedAndDanger covers the sticky Selected (accent fill) and the
// ButtonDanger (red border/ink) draw branches.
func TestButtonSelectedAndDanger(t *testing.T) {
	th := DefaultLight()

	// Selected fills with Accent — assert an accent pixel in the body.
	sel := NewButton("Reddit", nil)
	sel.Selected().Set(true)
	sel.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(60, 20)
	sel.Draw(newP(buf, 60), th)
	// Sample a body pixel left of the centred label so we hit the fill, not text.
	if px := pixelAt(buf, 60, 4, 10); px != th.Accent {
		t.Fatalf("selected body = %+v, want Accent %+v", px, th.Accent)
	}

	// ButtonDanger draws a red border — assert a red pixel on the top edge.
	del := NewButton("Delete", nil)
	del.Style = ButtonDanger
	del.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf2 := makeSurface(60, 20)
	del.Draw(newP(buf2, 60), th)
	found := false
	for x := 0; x < 60 && !found; x++ {
		if pixelAt(buf2, 60, x, 0) == dangerInk {
			found = true
		}
	}
	if !found {
		t.Fatal("ButtonDanger should stroke a red border")
	}
}

// TestButtonPressFeedback: a click presses the button (pressed face + fires
// OnClick); the release clears it. This is the visual click feedback a host
// gets just by routing EventClick/EventMouseUp.
func TestButtonPressFeedback(t *testing.T) {
	th := DefaultLight()
	fired := 0
	b := NewButton("Go", func() { fired++ })
	b.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})

	b.OnEvent(Event{Kind: EventClick})
	if !b.pressed {
		t.Fatal("EventClick should press the button")
	}
	if fired != 1 {
		t.Fatalf("EventClick should fire OnClick once, got %d", fired)
	}
	// The pressed face is Accent (distinct from the resting Surface face).
	buf := makeSurface(60, 20)
	b.Draw(newP(buf, 60), th)
	if px := pixelAt(buf, 60, 4, 10); px != th.Accent {
		t.Fatalf("pressed button face = %v, want Accent %v", px, th.Accent)
	}

	b.OnEvent(Event{Kind: EventMouseUp})
	if b.pressed {
		t.Fatal("EventMouseUp should release the button")
	}
	buf2 := makeSurface(60, 20)
	b.Draw(newP(buf2, 60), th)
	if px := pixelAt(buf2, 60, 4, 10); px == th.Accent {
		t.Fatal("released button should not keep the Accent (pressed) face")
	}

	// A nil OnClick still presses without panicking.
	nb := NewButton("Y", nil)
	nb.OnEvent(Event{Kind: EventClick})
	if !nb.pressed {
		t.Fatal("nil-OnClick button should still press")
	}
}

// TestButtonPressFeedbackDisabled: a button with PressFeedback off does not
// press on click (but still fires OnClick).
func TestButtonPressFeedbackDisabled(t *testing.T) {
	fired := false
	b := NewButton("Q", func() { fired = true })
	b.PressFeedback = false
	b.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	b.OnEvent(Event{Kind: EventClick})
	if b.pressed {
		t.Fatal("PressFeedback=false should not press the button")
	}
	if !fired {
		t.Fatal("OnClick should still fire with feedback disabled")
	}
}

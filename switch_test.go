// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestSwitchClickToggles covers the happy-path click flow: two clicks
// flip On on and back off, and the OnToggle callback receives the same
// value each time. Mirrors ToggleButton's double-click test so the
// coverage story is symmetric between the two toggle widgets.
func TestSwitchClickToggles(t *testing.T) {
	got := false
	s := NewSwitch(false)
	s.OnToggle = func(on bool) { got = on }
	s.OnEvent(Event{Kind: EventClick})
	if !s.On || !got {
		t.Fatalf("after click: On=%v got=%v", s.On, got)
	}
	s.OnEvent(Event{Kind: EventClick})
	if s.On || got {
		t.Fatalf("after second click: On=%v got=%v", s.On, got)
	}
}

// TestSwitchIgnoresNonClick guards the early-return in OnEvent: any
// non-click event (typed here as EventKeyDown) must leave On unchanged.
func TestSwitchIgnoresNonClick(t *testing.T) {
	s := NewSwitch(false)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if s.On {
		t.Fatal("KeyDown should not toggle Switch")
	}
}

// TestSwitchNilCallbackNoPanic covers the "OnToggle == nil" branch of
// OnEvent — the click still flips On, but the callback is skipped.
func TestSwitchNilCallbackNoPanic(t *testing.T) {
	s := NewSwitch(false)
	s.OnEvent(Event{Kind: EventClick})
	if !s.On {
		t.Fatal("click must flip On even without OnToggle callback")
	}
}

// TestSwitchDrawOnAndOff exercises Draw in both track states + verifies
// the knob position moves. Sample points:
//   - track pixel far from the knob (proves track colour = Accent/SurfaceAlt)
//   - knob-centre pixel (proves knob is drawn in Surface and slides)
func TestSwitchDrawOnAndOff(t *testing.T) {
	const w, h = 60, 24
	theme := DefaultLight()
	s := NewSwitch(true)
	// Bounds: X=2, Y=2, W=30, H=16 → knob is 12x12.
	// On knob sits at X=2+30-12-2=18, Y=4; Off knob sits at X=2+2=4, Y=4.
	s.SetBounds(Rect{X: 2, Y: 2, W: 30, H: 16})

	// On: track = Accent, knob at right (18..30).
	on := makeSurface(w, h)
	s.Draw(newP(on, w), theme)
	if pixelAt(on, w, 5, 10) != theme.Accent {
		t.Fatalf("on-state track at (5,10) = %+v, want Accent", pixelAt(on, w, 5, 10))
	}
	// Knob centre in On state (~24, 10) should be Surface.
	if pixelAt(on, w, 24, 10) != theme.Surface {
		t.Fatalf("on-state knob at (24,10) = %+v, want Surface", pixelAt(on, w, 24, 10))
	}

	// Off: track = SurfaceAlt, knob at left (4..16).
	s.On = false
	off := makeSurface(w, h)
	s.Draw(newP(off, w), theme)
	if pixelAt(off, w, 27, 10) != theme.SurfaceAlt {
		t.Fatalf("off-state track at (27,10) = %+v, want SurfaceAlt", pixelAt(off, w, 27, 10))
	}
	if pixelAt(off, w, 10, 10) != theme.Surface {
		t.Fatalf("off-state knob at (10,10) = %+v, want Surface", pixelAt(off, w, 10, 10))
	}

	// Border pixel at top-left corner drawn in both states.
	if pixelAt(off, w, 2, 2) != theme.Border {
		t.Fatalf("off corner border = %+v, want Border", pixelAt(off, w, 2, 2))
	}
}

// TestSwitchDrawTinyBoundsNoPanic exercises the degenerate branch where
// knob dimensions would go non-positive; fillRect / strokeRect swallow
// it, but the code path still needs coverage.
func TestSwitchDrawTinyBoundsNoPanic(t *testing.T) {
	theme := DefaultLight()
	s := NewSwitch(true)
	s.SetBounds(Rect{X: 0, Y: 0, W: 4, H: 2}) // 2*switchPad > H → knob 0-sized
	s.Draw(newP(makeSurface(8, 8), 8), theme)
}

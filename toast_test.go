// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewToastDefaults(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	if tt.Text != "hi" {
		t.Fatalf("Text = %q, want %q", tt.Text, "hi")
	}
	if tt.Kind != ToastInfo {
		t.Fatalf("Kind = %d, want ToastInfo", tt.Kind)
	}
	if tt.Visible {
		t.Fatal("fresh Toast must be hidden")
	}
	if tt.Life != 0 {
		t.Fatalf("Life = %d, want 0", tt.Life)
	}
}

// --- Draw: hidden --------------------------------------------------------

func TestToastDrawHiddenNoOp(t *testing.T) {
	tt := NewToast("x", ToastInfo)
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	surf := makeSurface(60, 20)
	before := make([]byte, len(surf))
	copy(before, surf)
	tt.Draw(newP(surf, 60), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw on hidden Toast touched byte %d: %d -> %d", i, before[i], surf[i])
		}
	}
}

// --- Draw: zero-width bounds --------------------------------------------

func TestToastDrawZeroWidthBoundsSkipsFill(t *testing.T) {
	// Position the widget so its whole footprint sits above the
	// surface -- text glyphs then clip out per-pixel and the fillRect
	// guard is the only pixel-writing path exercised, which we assert
	// leaves the buffer untouched.
	tt := NewToast("x", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: -30, W: 0, H: 20}) // zero W -> fillRect guard
	surf := makeSurface(20, 20)
	before := make([]byte, len(surf))
	copy(before, surf)
	tt.Draw(newP(surf, 20), DefaultLight())
	for i := range surf {
		if surf[i] != before[i] {
			t.Fatalf("Draw at zero W painted byte %d", i)
		}
	}
	// Zero H as well -- exercises the second guard branch.
	tt.SetBounds(Rect{X: 0, Y: -30, W: 20, H: 0})
	surf2 := makeSurface(20, 20)
	copy(before, surf2)
	tt.Draw(newP(surf2, 20), DefaultLight())
	for i := range surf2 {
		if surf2[i] != before[i] {
			t.Fatalf("Draw at zero H painted byte %d", i)
		}
	}
}

// --- Draw: each Kind paints its documented face ------------------------

func TestToastDrawKindColours(t *testing.T) {
	theme := DefaultLight()
	cases := []struct {
		kind ToastKind
		want RGBA
	}{
		{ToastInfo, theme.Accent},
		{ToastSuccess, RGB(0x2E, 0x8B, 0x57)},
		{ToastWarning, RGB(0xE0, 0xA0, 0x30)},
		{ToastError, RGB(0xC0, 0x30, 0x30)},
	}
	for _, c := range cases {
		tt := NewToast("!", c.kind)
		tt.Visible = true
		tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
		buf := makeSurface(60, 20)
		tt.Draw(newP(buf, 60), theme)
		// Sample a pixel well inside the pill face, away from stroke +
		// text glyphs.
		if got := pixelAt(buf, 60, 40, 15); got != c.want {
			t.Fatalf("kind %d fill = %+v, want %+v", c.kind, got, c.want)
		}
	}
}

// --- Draw: dark theme + Extra OnAccent override ------------------------

func TestToastDrawUsesOnAccentFromExtra(t *testing.T) {
	tt := NewToast("XYZ", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 20})
	theme := DefaultDark()
	custom := RGB(0xAB, 0xCD, 0xEF)
	theme.Extra = map[string]RGBA{"OnAccent": custom}
	buf := makeSurface(80, 20)
	tt.Draw(newP(buf, 80), theme)
	// Somewhere in the pill body must be at least one custom-coloured
	// text glyph pixel -- proves accentInk resolved to Extra["OnAccent"].
	found := false
	for y := 0; y < 20 && !found; y++ {
		for x := 0; x < 80; x++ {
			if pixelAt(buf, 80, x, y) == custom {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no OnAccent-coloured glyph pixel found in Toast body")
	}
}

// --- Draw: fallback when Extra is nil ----------------------------------

func TestToastDrawAccentInkFallbackWithNilExtra(t *testing.T) {
	tt := NewToast("q", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	theme := DefaultLight()
	theme.Extra = nil
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	// Fill still painted in Accent regardless of ink resolution.
	if pixelAt(buf, 60, 40, 15) != theme.Accent {
		t.Fatal("nil-Extra Toast body fill != Accent")
	}
}

// --- Draw: fallback when Extra map has no OnAccent key ----------------

func TestToastDrawAccentInkFallbackWithExtraNoKey(t *testing.T) {
	tt := NewToast("q", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	theme := DefaultLight()
	theme.Extra = map[string]RGBA{} // present but empty -> ok=false path
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	if pixelAt(buf, 60, 40, 15) != theme.Accent {
		t.Fatal("empty-Extra Toast body fill != Accent")
	}
}

// --- Tick: Life > 0 decrements without hiding --------------------------

func TestToastTickWithLifeAboveZeroDecrements(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 5
	tt.Tick()
	if tt.Life != 4 {
		t.Fatalf("Life after Tick = %d, want 4", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("Toast should stay Visible while Life > 0")
	}
}

// --- Tick: Life == 0 is a no-op (sticky sentinel) ---------------------

func TestToastTickWithLifeZeroNoOp(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 0 // sticky
	tt.Tick()
	if tt.Life != 0 {
		t.Fatalf("Life on sticky Toast = %d, want 0", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("sticky Toast should stay Visible on Tick")
	}
}

// --- Tick: hitting zero flips Visible to false -----------------------

func TestToastTickReachingZeroHides(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = 1
	tt.Tick()
	if tt.Life != 0 {
		t.Fatalf("Life = %d, want 0", tt.Life)
	}
	if tt.Visible {
		t.Fatal("Tick that reaches 0 must clear Visible")
	}
}

// --- Tick: negative Life is treated as sticky (no-op) -----------------

func TestToastTickWithNegativeLifeNoOp(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.Life = -3 // pathological input; guard treats it as sticky
	tt.Tick()
	if tt.Life != -3 {
		t.Fatalf("Life = %d, want -3", tt.Life)
	}
	if !tt.Visible {
		t.Fatal("negative-Life Toast should stay Visible on Tick")
	}
}

// --- Action button: AnchorIn width ---------------------------------------

// TestToastAnchorInPlainWidthUnchanged pins the no-action AnchorIn width
// formula to its pre-action-button value: textWidth(Text) + 2*ToastPadX.
// "Copied" is 6 runes * 6px advance = 36, so width = 36 + 20 = 56.
func TestToastAnchorInPlainWidthUnchanged(t *testing.T) {
	tt := NewToast("Copied", ToastInfo)
	host := Rect{X: 0, Y: 0, W: 400, H: 300}
	tt.AnchorIn(host, TopRight, 0)
	if got := tt.Bounds().W; got != 56 {
		t.Fatalf("plain AnchorIn width = %d, want 56", got)
	}
	if got := tt.Bounds().H; got != 19 {
		t.Fatalf("plain AnchorIn height = %d, want 19", got)
	}
}

// TestToastAnchorInWidthGrowsWithAction asserts the action slot's width
// (gap + 1px divider + button padding on both sides of the label) is
// folded into AnchorIn's total width, and pins the exact pixel value:
// "Undo" is 4 runes * 6px = 24, actionSlotW = 3*10 + 1 + 24 = 55, so total
// width = 56 (plain) + (55 - 10) = 101. Height is untouched by the action.
func TestToastAnchorInWidthGrowsWithAction(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 400, H: 300}

	plain := NewToast("Copied", ToastInfo)
	plain.AnchorIn(host, TopRight, 0)

	withAction := NewToast("Copied", ToastInfo)
	withAction.ActionLabel = "Undo"
	withAction.AnchorIn(host, TopRight, 0)

	if got := withAction.Bounds().W; got != 101 {
		t.Fatalf("action AnchorIn width = %d, want 101", got)
	}
	if withAction.Bounds().W <= plain.Bounds().W {
		t.Fatalf("action toast width %d should exceed plain width %d",
			withAction.Bounds().W, plain.Bounds().W)
	}
	if withAction.Bounds().H != plain.Bounds().H {
		t.Fatalf("action toast height %d should equal plain height %d",
			withAction.Bounds().H, plain.Bounds().H)
	}
}

// --- Action button: Draw --------------------------------------------------

// TestToastDrawActionButtonOnlyWhenLabelSet paints two otherwise-identical
// toasts at the exact width AnchorIn would compute (101px, per the AnchorIn
// test above) and asserts the divider column (local x = 56, spanning the
// full pill height) is Border-coloured only when ActionLabel is set.
func TestToastDrawActionButtonOnlyWhenLabelSet(t *testing.T) {
	theme := DefaultLight()
	const w, h = 110, 19

	plain := NewToast("Copied", ToastInfo)
	plain.Visible = true
	plain.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})
	pBuf := makeSurface(w, h)
	plain.Draw(newP(pBuf, w), theme)
	if got := pixelAt(pBuf, w, 56, 9); got == theme.Border {
		t.Fatalf("plain toast painted a divider at (56,9): %+v", got)
	}

	withAction := NewToast("Copied", ToastInfo)
	withAction.ActionLabel = "Undo"
	withAction.Visible = true
	withAction.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})
	aBuf := makeSurface(w, h)
	withAction.Draw(newP(aBuf, w), theme)
	if got := pixelAt(aBuf, w, 56, 9); got != theme.Border {
		t.Fatalf("divider pixel (56,9) = %+v, want Border %+v", got, theme.Border)
	}
	// The divider spans the full pill height, not just the sampled row.
	if got := pixelAt(aBuf, w, 56, 0); got != theme.Border {
		t.Fatalf("divider top pixel (56,0) = %+v, want Border %+v", got, theme.Border)
	}
	if got := pixelAt(aBuf, w, 56, 18); got != theme.Border {
		t.Fatalf("divider bottom pixel (56,18) = %+v, want Border %+v", got, theme.Border)
	}
}

// TestToastDrawActionLabelPaintsInk scans the action button's interior
// (local x in [67, 91), matching ax=46 + 2*ToastPadX + 1 = 67 through
// +textWidth("Undo")=24) for at least one accent-inverted ink pixel,
// proving the action label itself is rendered (not just the divider).
func TestToastDrawActionLabelPaintsInk(t *testing.T) {
	theme := DefaultLight()
	ink := accentInk(theme)
	const w, h = 110, 19

	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})
	buf := makeSurface(w, h)
	tt.Draw(newP(buf, w), theme)

	found := false
	for y := 0; y < h && !found; y++ {
		for x := 67; x < 91; x++ {
			if pixelAt(buf, w, x, y) == ink {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no ink-coloured pixel found in the action label's slot")
	}
}

// TestToastDrawNoActionByteForByteUnchanged pins the message-text ink
// pixel that pre-action-button Toast tests already assert on (the kind
// colour + accent-inverted text), proving a zero-value ActionLabel keeps
// Draw's fill + text path identical to before this feature landed.
func TestToastDrawNoActionByteForByteUnchanged(t *testing.T) {
	theme := DefaultLight()
	tt := NewToast("!", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	if got := pixelAt(buf, 60, 40, 15); got != theme.Accent {
		t.Fatalf("no-action fill = %+v, want Accent %+v", got, theme.Accent)
	}
}

// --- Action button: OnEvent -----------------------------------------------

// TestToastActionClickRunsActionAndHides clicks at the divider boundary
// (local x = 56, the first pixel of the button's clickable zone per the
// >= comparison in OnEvent) and asserts Action fires + the toast hides.
func TestToastActionClickRunsActionAndHides(t *testing.T) {
	called := false
	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.Action = func() { called = true }
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})

	tt.OnEvent(Event{Kind: EventClick, X: 56, Y: 9})

	if !called {
		t.Fatal("clicking the action button did not run Action")
	}
	if tt.Visible {
		t.Fatal("clicking the action button should hide the toast")
	}
}

// TestToastActionClickJustOutsideBoundaryNoOp clicks one pixel left of the
// button's clickable zone (still inside the gap before the divider) and
// asserts neither Action nor Visible is touched.
func TestToastActionClickJustOutsideBoundaryNoOp(t *testing.T) {
	called := false
	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.Action = func() { called = true }
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})

	tt.OnEvent(Event{Kind: EventClick, X: 55, Y: 9})

	if called {
		t.Fatal("click in the message zone must not run Action")
	}
	if !tt.Visible {
		t.Fatal("click in the message zone must not hide the toast")
	}
}

// TestToastActionClickNilActionStillHides proves Action is nil-guarded:
// a click on the button still hides the toast when Action was never set.
func TestToastActionClickNilActionStillHides(t *testing.T) {
	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})

	tt.OnEvent(Event{Kind: EventClick, X: 90, Y: 9}) // must not panic

	if tt.Visible {
		t.Fatal("nil-Action button click should still hide the toast")
	}
}

// TestToastOnEventIgnoresNonClickEvents asserts OnEvent's action-routing
// only reacts to EventClick -- a keyboard event on an action toast must
// not run Action or mutate Visible.
func TestToastOnEventIgnoresNonClickEvents(t *testing.T) {
	called := false
	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.Action = func() { called = true }
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 101, H: 19})

	tt.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})

	if called {
		t.Fatal("non-click event must not run Action")
	}
	if !tt.Visible {
		t.Fatal("non-click event must not hide the toast")
	}
}

// TestToastOnEventNoopWhenNoActionLabel asserts a plain (no ActionLabel)
// toast's OnEvent is a complete no-op on click -- it neither panics
// (Action stays nil-checked-away) nor mutates Visible, matching the
// pre-action-button default behaviour (Base's no-op OnEvent).
func TestToastOnEventNoopWhenNoActionLabel(t *testing.T) {
	tt := NewToast("hi", ToastInfo)
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})

	tt.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // must not panic

	if !tt.Visible {
		t.Fatal("click on a no-action toast must not hide it")
	}
}

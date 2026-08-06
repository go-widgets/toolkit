// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

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

// TestToastActionsWEmpty covers actionsW's no-action branch directly: a plain
// toast (no ActionLabel, no Actions) has a zero action zone, while a legacy
// single-ActionLabel toast has a positive one. Pins the "0 when no action"
// contract and keeps the branch covered.
func TestToastActionsWEmpty(t *testing.T) {
	if got := (&Toast{}).actionsW(); got != 0 {
		t.Fatalf("actionsW with no action = %d, want 0", got)
	}
	withLabel := &Toast{ActionLabel: "Undo"}
	if got := withLabel.actionsW(); got <= 0 {
		t.Fatalf("actionsW with a label = %d, want > 0", got)
	}
	// The legacy single-ActionLabel width must equal the pre-multi-action slot
	// (3*ToastPadX + 1 + textWidth("Undo")), proving byte-exact back-compat.
	if got, want := withLabel.actionsW(), 3*ToastPadX+1+withLabel.textWidth("Undo"); got != want {
		t.Fatalf("legacy single-action width = %d, want %d", got, want)
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

// --- Icon: vector glyph painted left of the text -------------------------

// TestToastVectorIconShiftsTextAndPaints proves a vector Icon reserves a slot
// on the left (widening the pill + pushing the text right) and is actually
// invoked during Draw.
func TestToastVectorIconShiftsTextAndPaints(t *testing.T) {
	theme := DefaultLight()
	host := Rect{X: 0, Y: 0, W: 400, H: 300}

	plain := NewToast("Hi", ToastInfo)
	plain.AnchorIn(host, TopLeft, 0)

	sentinel := RGB(0x11, 0x22, 0x33)
	iconCalls := 0
	withIcon := NewToast("Hi", ToastInfo)
	withIcon.Icon = func(p painter.Painter, r Rect, _ RGBA) {
		iconCalls++
		fillRect(p, r.X, r.Y, r.W, r.H, sentinel)
	}
	withIcon.AnchorIn(host, TopLeft, 0)

	// The icon slot = glyphHeight + ToastPadX wider than the plain pill.
	if got, want := withIcon.Bounds().W-plain.Bounds().W, withIcon.glyphHeight()+ToastPadX; got != want {
		t.Fatalf("icon slot width = %d, want %d", got, want)
	}
	if withIcon.iconSlotW() != withIcon.glyphHeight()+ToastPadX {
		t.Fatalf("iconSlotW() = %d", withIcon.iconSlotW())
	}

	withIcon.SetBounds(Rect{X: 0, Y: 0, W: withIcon.Bounds().W, H: withIcon.Bounds().H})
	withIcon.Visible = true
	buf := makeSurface(withIcon.Bounds().W, withIcon.Bounds().H)
	withIcon.Draw(newP(buf, withIcon.Bounds().W), theme)
	if iconCalls != 1 {
		t.Fatalf("Icon func called %d times, want 1", iconCalls)
	}
	// A sentinel-coloured pixel proves the icon painted inside its slot.
	found := false
	for y := 0; y < withIcon.Bounds().H && !found; y++ {
		for x := 0; x < withIcon.Bounds().W; x++ {
			if pixelAt(buf, withIcon.Bounds().W, x, y) == sentinel {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("no sentinel icon pixel painted")
	}
}

// TestToastImageIconBeatsVector proves a valid Pixels image is drawn (the
// hasImage path) and takes precedence over a vector Icon.
func TestToastImageIconBeatsVector(t *testing.T) {
	theme := DefaultLight()
	// 2x2 solid magenta image.
	px := make([]byte, 2*2*4)
	for i := 0; i+3 < len(px); i += 4 {
		px[i], px[i+1], px[i+2], px[i+3] = 0xFF, 0x00, 0xFF, 0xFF
	}
	iconCalled := false
	tt := NewToast("Hi", ToastInfo)
	tt.Pixels, tt.IW, tt.IH = px, 2, 2
	tt.Icon = func(painter.Painter, Rect, RGBA) { iconCalled = true }
	if !tt.hasImage() || !tt.hasIcon() {
		t.Fatal("hasImage/hasIcon should be true for a valid image")
	}
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(60, 20)
	tt.Draw(newP(buf, 60), theme)
	if iconCalled {
		t.Fatal("a valid image must suppress the vector Icon")
	}
}

// TestToastInvalidImageFallsToVector proves a too-short Pixels buffer is
// ignored so the vector Icon draws instead.
func TestToastInvalidImageFallsToVector(t *testing.T) {
	tt := NewToast("Hi", ToastInfo)
	tt.Pixels, tt.IW, tt.IH = []byte{1, 2, 3}, 2, 2 // too short
	if tt.hasImage() {
		t.Fatal("short buffer should not count as an image")
	}
	called := false
	tt.Icon = func(painter.Painter, Rect, RGBA) { called = true }
	tt.Visible = true
	tt.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	tt.Draw(newP(make([]byte, 60*20*4), 60), DefaultLight())
	if !called {
		t.Fatal("invalid image should fall back to the vector Icon")
	}
}

// --- Multi-line: distinct title + body rows ------------------------------

// TestToastMultiLineGrowsHeightAndPaintsBoth checks Lines stacks each row,
// growing the pill height, and that the widest row drives the width.
func TestToastMultiLineGrowsHeightAndPaintsBoth(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 400, H: 300}
	single := NewToast("Title", ToastInfo)
	single.AnchorIn(host, TopLeft, 0)

	multi := NewToast("", ToastInfo)
	multi.Lines = []string{"Title", "A longer body line"}
	multi.AnchorIn(host, TopLeft, 0)

	if len(multi.lines()) != 2 {
		t.Fatalf("lines() = %d, want 2", len(multi.lines()))
	}
	wantH := multi.contentH() + 2*ToastPadY
	if multi.Bounds().H != wantH {
		t.Fatalf("multi-line H = %d, want %d", multi.Bounds().H, wantH)
	}
	if multi.Bounds().H <= single.Bounds().H {
		t.Fatalf("multi-line pill %d should be taller than single %d", multi.Bounds().H, single.Bounds().H)
	}
	// Width tracks the widest line, not the empty Text.
	if multi.Bounds().W != multi.linesW()+2*ToastPadX {
		t.Fatalf("multi-line W = %d, want %d", multi.Bounds().W, multi.linesW()+2*ToastPadX)
	}

	multi.SetBounds(Rect{X: 0, Y: 0, W: multi.Bounds().W, H: multi.Bounds().H})
	multi.Visible = true
	theme := DefaultLight()
	ink := accentInk(theme)
	buf := makeSurface(multi.Bounds().W, multi.Bounds().H)
	multi.Draw(newP(buf, multi.Bounds().W), theme)
	// Count distinct rows carrying ink: expect >= 2 text bands.
	rows := map[int]bool{}
	for y := 0; y < multi.Bounds().H; y++ {
		for x := 0; x < multi.Bounds().W; x++ {
			if pixelAt(buf, multi.Bounds().W, x, y) == ink {
				rows[y] = true
				break
			}
		}
	}
	// Two text lines separated by a gap must occupy two non-contiguous bands.
	bands, prev := 0, -2
	ys := make([]int, 0, len(rows))
	for y := range rows {
		ys = append(ys, y)
	}
	sortInts(ys)
	for _, y := range ys {
		if y != prev+1 {
			bands++
		}
		prev = y
	}
	if bands < 2 {
		t.Fatalf("multi-line toast painted %d text bands, want >= 2", bands)
	}
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j-1] > a[j]; j-- {
			a[j-1], a[j] = a[j], a[j-1]
		}
	}
}

// --- Multi-action: several buttons ---------------------------------------

// TestToastMultiActionLayoutAndClicks lays out two action buttons and asserts
// each one's click runs its own callback (and only that one), then hides.
func TestToastMultiActionLayoutAndClicks(t *testing.T) {
	host := Rect{X: 0, Y: 0, W: 400, H: 300}
	var log []string
	tt := NewToast("Deleted", ToastInfo)
	tt.Actions = []ToastAction{
		{Label: "Undo", Callback: func() { log = append(log, "undo") }},
		{Label: "Dismiss", Callback: func() { log = append(log, "dismiss") }},
	}
	tt.AnchorIn(host, TopRight, 0)
	if len(tt.acts()) != 2 {
		t.Fatalf("acts() = %d, want 2", len(tt.acts()))
	}

	// Width folds in the whole action zone.
	wantW := tt.linesW() + 2*ToastPadX + tt.actionsW() - ToastPadX
	if tt.Bounds().W != wantW {
		t.Fatalf("multi-action W = %d, want %d", tt.Bounds().W, wantW)
	}

	tt.SetBounds(Rect{X: 0, Y: 0, W: tt.Bounds().W, H: tt.Bounds().H})
	tt.Visible = true
	theme := DefaultLight()
	buf := makeSurface(tt.Bounds().W, tt.Bounds().H)
	tt.Draw(newP(buf, tt.Bounds().W), theme)
	// Two dividers -> at least two Border columns in the action zone.
	dividers := 0
	zoneStart := tt.Bounds().W - tt.actionsW() + ToastPadX
	for x := zoneStart; x < tt.Bounds().W; x++ {
		if pixelAt(buf, tt.Bounds().W, x, tt.Bounds().H/2) == theme.Border {
			dividers++
		}
	}
	if dividers < 2 {
		t.Fatalf("multi-action drew %d divider columns, want >= 2", dividers)
	}

	// Click the first button (just past its divider).
	bx := tt.Bounds().W - tt.actionsW() + ToastPadX
	tt.OnEvent(Event{Kind: EventClick, X: bx + 2, Y: tt.Bounds().H / 2})
	if len(log) != 1 || log[0] != "undo" {
		t.Fatalf("first-button click log = %v, want [undo]", log)
	}
	if tt.Visible {
		t.Fatal("action click should hide the toast")
	}

	// Reset and click the second button.
	tt.Visible = true
	log = nil
	seg0 := 1 + 2*ToastPadX + tt.textWidth("Undo")
	tt.OnEvent(Event{Kind: EventClick, X: bx + seg0 + 2, Y: tt.Bounds().H / 2})
	if len(log) != 1 || log[0] != "dismiss" {
		t.Fatalf("second-button click log = %v, want [dismiss]", log)
	}
}

// TestToastMultiActionClickOutsideNoOp clicks left of the first button and in
// the message zone: no callback runs, the toast stays visible.
func TestToastMultiActionClickOutsideNoOp(t *testing.T) {
	fired := false
	tt := NewToast("Msg", ToastInfo)
	tt.Actions = []ToastAction{{Label: "Undo", Callback: func() { fired = true }}}
	tt.AnchorIn(Rect{X: 0, Y: 0, W: 400, H: 300}, TopRight, 0)
	tt.SetBounds(Rect{X: 0, Y: 0, W: tt.Bounds().W, H: tt.Bounds().H})
	tt.Visible = true
	tt.OnEvent(Event{Kind: EventClick, X: 0, Y: tt.Bounds().H / 2}) // message zone
	if fired || !tt.Visible {
		t.Fatal("click outside the action zone must be a no-op")
	}
}

// --- Action button: ButtonRects (host hit-testing) ------------------------

// TestToastButtonRectsNilWhenNoActions pins the no-action contract: a plain
// toast (no ActionLabel, no Actions) exposes no button rectangles.
func TestToastButtonRectsNilWhenNoActions(t *testing.T) {
	tt := NewToast("Copied", ToastInfo)
	tt.AnchorIn(Rect{X: 0, Y: 0, W: 400, H: 300}, TopRight, 0)
	if got := tt.ButtonRects(); got != nil {
		t.Fatalf("ButtonRects with no actions = %v, want nil", got)
	}
}

// TestToastButtonRectsLegacySingle asserts the legacy ActionLabel path yields
// exactly one rect at the precise laid-out position, full pill height, matching
// the divider column Draw paints.
func TestToastButtonRectsLegacySingle(t *testing.T) {
	tt := NewToast("Copied", ToastInfo)
	tt.ActionLabel = "Undo"
	tt.AnchorIn(Rect{X: 0, Y: 0, W: 400, H: 300}, TopRight, 0)
	b := tt.Bounds()

	rects := tt.ButtonRects()
	if len(rects) != 1 {
		t.Fatalf("legacy single-action ButtonRects len = %d, want 1", len(rects))
	}
	wantX := b.W - tt.actionsW() + ToastPadX
	wantW := 1 + 2*ToastPadX + tt.textWidth("Undo")
	want := Rect{X: wantX, Y: 0, W: wantW, H: b.H}
	if rects[0] != want {
		t.Fatalf("legacy button rect = %+v, want %+v", rects[0], want)
	}
}

// TestToastButtonRectsMultiExactAndTilesActionZone asserts each button rect's
// EXACT X/Y/W/H, that the buttons tile the action zone edge-to-edge (no gaps,
// no overlap), and that the last rect ends flush with the pill's right edge.
func TestToastButtonRectsMultiExactAndTilesActionZone(t *testing.T) {
	tt := NewToast("Deleted", ToastInfo)
	tt.Actions = []ToastAction{
		{Label: "Undo", Callback: func() {}},
		{Label: "Dismiss", Callback: func() {}},
	}
	tt.AnchorIn(Rect{X: 0, Y: 0, W: 400, H: 300}, TopRight, 0)
	b := tt.Bounds()

	rects := tt.ButtonRects()
	if len(rects) != 2 {
		t.Fatalf("multi-action ButtonRects len = %d, want 2", len(rects))
	}

	seg0 := 1 + 2*ToastPadX + tt.textWidth("Undo")
	seg1 := 1 + 2*ToastPadX + tt.textWidth("Dismiss")
	x0 := b.W - tt.actionsW() + ToastPadX
	want := []Rect{
		{X: x0, Y: 0, W: seg0, H: b.H},
		{X: x0 + seg0, Y: 0, W: seg1, H: b.H},
	}
	for i := range want {
		if rects[i] != want[i] {
			t.Fatalf("button rect %d = %+v, want %+v", i, rects[i], want[i])
		}
	}
	// Edge-to-edge tiling: rect 1 starts exactly where rect 0 ends.
	if rects[1].X != rects[0].X+rects[0].W {
		t.Fatalf("buttons not contiguous: rect0 ends at %d, rect1 starts at %d",
			rects[0].X+rects[0].W, rects[1].X)
	}
	// The last button ends flush with the pill's right edge.
	if end := rects[1].X + rects[1].W; end != b.W {
		t.Fatalf("last button ends at %d, want pill right edge %d", end, b.W)
	}
}

// TestToastButtonRectsAgreeWithOnEvent proves the host hit-test path and the
// widget's own OnEvent routing are one and the same: clicking the CENTRE of each
// ButtonRects rect fires exactly that action's callback (index-precise), and a
// click one pixel left of the first rect fires nothing.
func TestToastButtonRectsAgreeWithOnEvent(t *testing.T) {
	var log []string
	tt := NewToast("Deleted", ToastInfo)
	tt.Actions = []ToastAction{
		{Label: "Undo", Callback: func() { log = append(log, "undo") }},
		{Label: "Dismiss", Callback: func() { log = append(log, "dismiss") }},
	}
	tt.AnchorIn(Rect{X: 0, Y: 0, W: 400, H: 300}, TopRight, 0)
	tt.SetBounds(Rect{X: 0, Y: 0, W: tt.Bounds().W, H: tt.Bounds().H})
	rects := tt.ButtonRects()
	names := []string{"undo", "dismiss"}

	for i, r := range rects {
		log = nil
		tt.Visible = true
		cx := r.X + r.W/2
		cy := r.Y + r.H/2
		tt.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
		if len(log) != 1 || log[0] != names[i] {
			t.Fatalf("click centre of button %d fired %v, want [%s]", i, log, names[i])
		}
		if tt.Visible {
			t.Fatalf("click on button %d should hide the toast", i)
		}
	}

	// One pixel left of the first button: outside every rect -> no fire.
	log = nil
	tt.Visible = true
	tt.OnEvent(Event{Kind: EventClick, X: rects[0].X - 1, Y: rects[0].H / 2})
	if len(log) != 0 || !tt.Visible {
		t.Fatalf("click just left of the first button fired %v (visible=%v), want none",
			log, tt.Visible)
	}
}

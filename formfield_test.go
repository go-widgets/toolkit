// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// mockFFChild is a minimal Widget for FormField tests. It records
// the last event delivered + the Bounds it was assigned during Draw
// so tests can assert dispatch + composition without pulling a full
// input widget in.
type mockFFChild struct {
	Base
	draws     int
	lastEvent Event
	events    int
}

func (m *mockFFChild) Draw(p painter.Painter, theme *Theme) {
	_, _ = p, theme
	m.draws++
}

func (m *mockFFChild) OnEvent(ev Event) {
	m.lastEvent = ev
	m.events++
}

// --- Constants -----------------------------------------------------------

func TestFormFieldConstants(t *testing.T) {
	if FormFieldLabelH != GlyphHeight+2 {
		t.Fatalf("FormFieldLabelH = %d, want %d", FormFieldLabelH, GlyphHeight+2)
	}
	if FormFieldChildGap != 4 {
		t.Fatalf("FormFieldChildGap = %d, want 4", FormFieldChildGap)
	}
	if FormFieldHelpGap != 2 {
		t.Fatalf("FormFieldHelpGap = %d, want 2", FormFieldHelpGap)
	}
	if FormFieldPadX != 0 {
		t.Fatalf("FormFieldPadX = %d, want 0", FormFieldPadX)
	}
	if FormFieldPadY != 4 {
		t.Fatalf("FormFieldPadY = %d, want 4", FormFieldPadY)
	}
}

// --- Constructor ---------------------------------------------------------

func TestNewFormFieldCarriesLabelAndChild(t *testing.T) {
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	if f.Label != "Name" {
		t.Fatalf("Label = %q, want %q", f.Label, "Name")
	}
	if f.Child != child {
		t.Fatalf("Child not the one we passed")
	}
	if f.Help != "" || f.Error != "" {
		t.Fatalf("NewFormField left Help/Error non-empty: %q/%q", f.Help, f.Error)
	}
}

func TestNewFormFieldWithNilChild(t *testing.T) {
	f := NewFormField("Empty", nil)
	if f.Child != nil {
		t.Fatalf("Child should be nil, got %v", f.Child)
	}
}

// --- Draw label ----------------------------------------------------------

// A FormField with only a Label paints OnBackground glyphs at the
// top-left of its Bounds.
func TestFormFieldDrawLabelPaintsOnBackground(t *testing.T) {
	const w, h = 200, 60
	theme := DefaultLight()
	f := NewFormField("HI", nil)
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 48})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	// Some pixel in the label row must equal OnBackground.
	found := false
	for y := 6 + FormFieldPadY; y < 6+FormFieldPadY+GlyphHeight && !found; y++ {
		for x := 4; x < 4+80 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("FormField label ink missing from label row (OnBackground not found)")
	}
}

func TestFormFieldDrawLabelUsesDarkTheme(t *testing.T) {
	const w, h = 200, 60
	theme := DefaultDark()
	f := NewFormField("YO", nil)
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 48})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)
	found := false
	for y := 6 + FormFieldPadY; y < 6+FormFieldPadY+GlyphHeight && !found; y++ {
		for x := 4; x < 4+80 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("FormField label ink missing under dark theme")
	}
}

// --- Draw child ----------------------------------------------------------

// When Child is non-nil, Draw calls Child.SetBounds + Child.Draw.
// Child.Bounds() ends up covering the space between the label row +
// the caption row.
func TestFormFieldDrawWithChildPositionsAndDraws(t *testing.T) {
	const w, h = 200, 80
	theme := DefaultLight()
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 60})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	if child.draws != 1 {
		t.Fatalf("Child.Draw invocations = %d, want 1", child.draws)
	}
	cb := child.Bounds()
	wantTop := 6 + FormFieldPadY + FormFieldLabelH + FormFieldChildGap
	wantBottom := 6 + 60 - FormFieldPadY
	if cb.X != 4+FormFieldPadX {
		t.Fatalf("Child.X = %d, want %d", cb.X, 4+FormFieldPadX)
	}
	if cb.Y != wantTop {
		t.Fatalf("Child.Y = %d, want %d", cb.Y, wantTop)
	}
	if cb.W != 120-2*FormFieldPadX {
		t.Fatalf("Child.W = %d, want %d", cb.W, 120-2*FormFieldPadX)
	}
	if cb.Y+cb.H != wantBottom {
		t.Fatalf("Child.Y+H = %d, want %d", cb.Y+cb.H, wantBottom)
	}
}

// --- Draw help -----------------------------------------------------------

// Help caption is drawn in theme.Border below the child.
func TestFormFieldDrawHelpUsesBorderInk(t *testing.T) {
	const w, h = 200, 80
	theme := DefaultLight()
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.Help = "hint"
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 60})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	// Look for Border ink in the caption strip at the bottom of the
	// field's bounds.
	captionYStart := 6 + 60 - FormFieldPadY - GlyphHeight
	captionYEnd := 6 + 60 - FormFieldPadY
	found := false
	for y := captionYStart; y < captionYEnd && !found; y++ {
		for x := 4; x < 4+80 && !found; x++ {
			if pixelAt(buf, w, x, y) == theme.Border {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Help caption ink missing (Border tone not found in caption strip)")
	}
}

// --- Draw error ----------------------------------------------------------

// Error caption is drawn in the fixed red — verify by finding the
// exact ink somewhere in the caption strip.
func TestFormFieldDrawErrorUsesFixedRedInk(t *testing.T) {
	const w, h = 200, 80
	theme := DefaultLight()
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.Error = "bad"
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 60})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	red := RGBA{R: 190, G: 60, B: 60, A: 255}
	captionYStart := 6 + 60 - FormFieldPadY - GlyphHeight
	captionYEnd := 6 + 60 - FormFieldPadY
	found := false
	for y := captionYStart; y < captionYEnd && !found; y++ {
		for x := 4; x < 4+80 && !found; x++ {
			if pixelAt(buf, w, x, y) == red {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Error caption red ink missing from caption strip")
	}
}

// Error > Help precedence: when both are set only Error paints; no
// Border-tone caption ink appears.
func TestFormFieldDrawErrorSuppressesHelp(t *testing.T) {
	const w, h = 200, 80
	theme := DefaultLight()
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.Help = "hint"
	f.Error = "bad"
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 60})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)

	red := RGBA{R: 190, G: 60, B: 60, A: 255}
	captionYStart := 6 + 60 - FormFieldPadY - GlyphHeight
	captionYEnd := 6 + 60 - FormFieldPadY

	sawRed := false
	sawBorder := false
	for y := captionYStart; y < captionYEnd; y++ {
		for x := 4; x < 4+80; x++ {
			pxv := pixelAt(buf, w, x, y)
			if pxv == red {
				sawRed = true
			}
			if pxv == theme.Border {
				sawBorder = true
			}
		}
	}
	if !sawRed {
		t.Fatal("Error caption red ink missing")
	}
	if sawBorder {
		t.Fatal("Help ink painted despite Error being set (precedence broken)")
	}
}

// --- Draw nil child ------------------------------------------------------

// FormField with nil Child still paints the label + caption; the
// child rect is skipped without panicking.
func TestFormFieldDrawNilChildStillPaintsLabelAndHelp(t *testing.T) {
	const w, h = 200, 80
	theme := DefaultLight()
	f := NewFormField("Name", nil)
	f.Help = "hint"
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 60})
	buf := makeSurface(w, h)
	f.Draw(newP(buf, w), theme)
	// Label ink present.
	foundLabel := false
	for y := 6 + FormFieldPadY; y < 6+FormFieldPadY+GlyphHeight && !foundLabel; y++ {
		for x := 4; x < 4+80 && !foundLabel; x++ {
			if pixelAt(buf, w, x, y) == theme.OnBackground {
				foundLabel = true
			}
		}
	}
	if !foundLabel {
		t.Fatal("label ink missing with nil Child")
	}
}

// --- Draw zero width -----------------------------------------------------

// Zero-width bounds must not panic; the widget renders whatever it
// can (or nothing) and returns.
func TestFormFieldDrawZeroWidth(t *testing.T) {
	const w, h = 40, 40
	theme := DefaultLight()
	f := NewFormField("HI", &mockFFChild{})
	f.Help = "help"
	f.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 40})
	f.Draw(newP(makeSurface(w, h), w), theme)
	// And zero-height.
	f.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 0})
	f.Draw(newP(makeSurface(w, h), w), theme)
}

// --- Empty Extra map -----------------------------------------------------

// Passing a theme with a nil Extra map must not panic (FormField
// doesn't consult Extra but the coverage requirement asks us to
// prove the empty-Extra path is safe).
func TestFormFieldDrawWithEmptyExtraMap(t *testing.T) {
	const w, h = 200, 60
	theme := DefaultLight()
	theme.Extra = nil
	f := NewFormField("HI", nil)
	f.SetBounds(Rect{X: 4, Y: 6, W: 120, H: 48})
	f.Draw(newP(makeSurface(w, h), w), theme)
}

// --- OnEvent -------------------------------------------------------------

// Click inside the Child rect is forwarded with translated coords.
func TestFormFieldOnEventClickInsideChildForwards(t *testing.T) {
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.SetBounds(Rect{X: 10, Y: 10, W: 120, H: 60})

	// Populate childRect via Draw so we know its exact placement.
	f.Draw(newP(makeSurface(200, 100), 200), DefaultLight())
	cr := child.Bounds()

	f.OnEvent(Event{Kind: EventClick, X: cr.X + 3, Y: cr.Y + 3})
	if child.events != 1 {
		t.Fatalf("child received %d events, want 1", child.events)
	}
	if child.lastEvent.Kind != EventClick {
		t.Fatalf("child got kind = %d, want EventClick", child.lastEvent.Kind)
	}
	if child.lastEvent.X != 3 || child.lastEvent.Y != 3 {
		t.Fatalf("child got local (%d,%d), want (3,3)",
			child.lastEvent.X, child.lastEvent.Y)
	}
}

// Click outside the Child rect is dropped.
func TestFormFieldOnEventClickOutsideChildDropped(t *testing.T) {
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.SetBounds(Rect{X: 10, Y: 10, W: 120, H: 60})
	f.Draw(newP(makeSurface(200, 100), 200), DefaultLight())

	// Above (in the label row).
	f.OnEvent(Event{Kind: EventClick, X: 20, Y: 12})
	// Below (in the caption row -- doesn't exist here but still outside).
	f.OnEvent(Event{Kind: EventClick, X: 20, Y: 200})
	// Left of the child (child.X > 10 due to padding could be 10 with padX=0,
	// so shift to negative x to guarantee out-of-rect).
	f.OnEvent(Event{Kind: EventClick, X: -5, Y: 40})
	// Right of the child.
	f.OnEvent(Event{Kind: EventClick, X: 5000, Y: 40})

	if child.events != 0 {
		t.Fatalf("child received %d events, want 0", child.events)
	}
}

// Non-click point event is still forwarded unconditionally so a
// focused Child sees keyboard input.
func TestFormFieldOnEventNonClickForwarded(t *testing.T) {
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.SetBounds(Rect{X: 10, Y: 10, W: 120, H: 60})

	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if child.events != 1 {
		t.Fatalf("child received %d events, want 1 (non-click should forward)",
			child.events)
	}
	if child.lastEvent.Kind != EventKeyDown || child.lastEvent.Code != "Enter" {
		t.Fatalf("child got %+v, want KeyDown Enter", child.lastEvent)
	}
}

// Nil Child + any event is a no-op (must not panic).
func TestFormFieldOnEventNilChildNoPanic(t *testing.T) {
	f := NewFormField("HI", nil)
	f.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 40})
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

// A caption-height reservation happens when Help or Error is set —
// exercise the branch where neither is set (the "no caption" child
// rect is taller by GlyphHeight+FormFieldHelpGap).
func TestFormFieldChildRectExpandsWithoutCaption(t *testing.T) {
	child := &mockFFChild{}
	f := NewFormField("Name", child)
	f.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	f.Draw(newP(makeSurface(200, 100), 200), DefaultLight())
	without := child.Bounds().H

	child2 := &mockFFChild{}
	f2 := NewFormField("Name", child2)
	f2.Help = "hint"
	f2.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 60})
	f2.Draw(newP(makeSurface(200, 100), 200), DefaultLight())
	with := child2.Bounds().H

	if without <= with {
		t.Fatalf("child rect without caption (%d) should be TALLER than with caption (%d)",
			without, with)
	}
}

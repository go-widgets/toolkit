// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"os"
	"testing"

	"github.com/go-widgets/painter"
)

// sampleBoard builds a small three-column board used across the tests,
// with a custom-accent first card (exercising the KanbanCard.Accent
// override branch) and default-accent cards elsewhere.
func sampleBoard() *Kanban {
	return NewKanban([]KanbanColumn{
		{Title: "To Do", Cards: []KanbanCard{
			{Title: "Design", Subtitle: "spec", Accent: RGB(0xE0, 0x40, 0x40)},
			{Title: "Research", Subtitle: "links"},
		}},
		{Title: "Doing", Cards: []KanbanCard{
			{Title: "Build", Subtitle: "impl"},
		}},
		{Title: "Done", Cards: []KanbanCard{
			{Title: "Ship", Subtitle: "v1"},
		}},
	})
}

func rgbaEq(t *testing.T, img *image.RGBA, x, y int, want RGBA, what string) {
	t.Helper()
	c := img.RGBAAt(x, y)
	if c.R != want.R || c.G != want.G || c.B != want.B || c.A != want.A {
		t.Errorf("%s at (%d,%d): got %v, want %v", what, x, y, c, want)
	}
}

func TestNewKanban(t *testing.T) {
	k := NewKanban(nil)
	if k.SelectedCol != -1 || k.SelectedCard != -1 {
		t.Fatalf("NewKanban seeds: got (%d,%d), want (-1,-1)", k.SelectedCol, k.SelectedCard)
	}
}

// TestKanbanRenderPixels renders a selected-card board and asserts the
// column panel, a card fill, an accent stripe and the selection tint all
// land in the expected colours, then writes the board to a PNG the parent
// can eyeball.
func TestKanbanRenderPixels(t *testing.T) {
	theme := DefaultLight()
	k := sampleBoard()
	k.SelectedCol, k.SelectedCard = 1, 0 // highlight "Build"

	const w, h = 600, 300
	img, err := RenderImage(k, w, h, theme)
	if err != nil {
		t.Fatalf("RenderImage: %v", err)
	}

	// Geometry mirrors kanban.go: colW = (600 - 2*KanbanColGap)/3.
	colW := (w - 2*KanbanColGap) / 3

	// Column 0 panel is SurfaceAlt (top-left corner, before any text).
	rgbaEq(t, img, 2, 2, theme.SurfaceAlt, "column panel")

	// Column 0, card 0: local X = 0+KanbanCardGap, Y = KanbanHeaderH+KanbanCardGap.
	c0x := KanbanCardGap
	c0y := KanbanHeaderH + KanbanCardGap
	// Accent stripe of card 0 is the custom red.
	rgbaEq(t, img, c0x+kanbanStripeInset+1, c0y+kanbanStripeInset+4, RGB(0xE0, 0x40, 0x40), "custom accent stripe")
	// Interior fill (away from stripe/text/rounded corner) is Surface.
	rgbaEq(t, img, c0x+colW-2*KanbanCardGap-8, c0y+KanbanCardH-8, theme.Surface, "card fill")

	// Column 1, card 0 is selected -> tinted fill + default (theme) accent
	// stripe.
	c1x := colW + KanbanColGap + KanbanCardGap
	rgbaEq(t, img, c1x+kanbanStripeInset+1, c0y+kanbanStripeInset+4, theme.Accent, "default accent stripe")
	rgbaEq(t, img, c1x+colW-2*KanbanCardGap-8, c0y+KanbanCardH-8, kanbanSelTint(theme), "selection tint")

	png, err := RenderPNG(k, w, h, theme)
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if err := os.WriteFile("/tmp/tk-kanban-demo.png", png, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// TestKanbanClickSelects covers the OnEvent click path: a hit selects the
// card and fires OnCardClick; a miss (header / gap) is a no-op; a
// non-click event is ignored.
func TestKanbanClickSelects(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3

	var gotCol, gotCard = -9, -9
	k.OnCardClick = func(col, card int) { gotCol, gotCard = col, card }

	// Click inside column 0, card 1.
	lr := k.cardLocalRect(0, 1, colW)
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 5, Y: lr.Y + 5})
	if k.SelectedCol != 0 || k.SelectedCard != 1 {
		t.Fatalf("selection: got (%d,%d), want (0,1)", k.SelectedCol, k.SelectedCard)
	}
	if gotCol != 0 || gotCard != 1 {
		t.Fatalf("OnCardClick: got (%d,%d), want (0,1)", gotCol, gotCard)
	}

	// Click in the header band (Y within KanbanHeaderH) misses every card.
	k.SelectedCol, k.SelectedCard = 5, 5
	k.OnEvent(Event{Kind: EventClick, X: 10, Y: 2})
	if k.SelectedCol != 5 || k.SelectedCard != 5 {
		t.Fatalf("header click mutated selection: (%d,%d)", k.SelectedCol, k.SelectedCard)
	}

	// A non-click event is ignored entirely.
	k.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if k.SelectedCol != 5 || k.SelectedCard != 5 {
		t.Fatalf("non-click event mutated selection: (%d,%d)", k.SelectedCol, k.SelectedCard)
	}
}

// TestKanbanClickNilCallback exercises the nil-OnCardClick branch of
// OnEvent: a hit still updates the selection without a callback set.
func TestKanbanClickNilCallback(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3
	lr := k.cardLocalRect(2, 0, colW)
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 3, Y: lr.Y + 3})
	if k.SelectedCol != 2 || k.SelectedCard != 0 {
		t.Fatalf("selection: got (%d,%d), want (2,0)", k.SelectedCol, k.SelectedCard)
	}
}

// TestKanbanEmptyClick covers cardAt's empty-columns guard: a click on a
// board with no columns is a no-op.
func TestKanbanEmptyClick(t *testing.T) {
	k := NewKanban(nil)
	k.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	k.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if k.SelectedCol != -1 || k.SelectedCard != -1 {
		t.Fatalf("empty click mutated selection: (%d,%d)", k.SelectedCol, k.SelectedCard)
	}
}

// TestKanbanDrawGuards covers the two early returns in Draw (zero extent,
// no columns) and colWidth's negative-total floor.
func TestKanbanDrawGuards(t *testing.T) {
	theme := DefaultLight()

	// Zero-width bounds: Draw returns before touching the painter.
	buf := make([]byte, 4*10*10)
	p := painter.NewPixelPainter(buf, 10, 10)
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	k.Draw(p, theme) // r.W <= 0
	k.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 0})
	k.Draw(p, theme) // r.H <= 0

	// No columns: Draw returns after the n == 0 check.
	empty := NewKanban(nil)
	if _, err := RenderImage(empty, 40, 40, theme); err != nil {
		t.Fatalf("RenderImage(empty): %v", err)
	}

	// Narrow board with many columns drives colWidth's total < 0 branch.
	if _, err := RenderImage(sampleBoard(), 4, 60, theme); err != nil {
		t.Fatalf("RenderImage(narrow): %v", err)
	}
}

// TestKanbanDrawNoSelection renders with the default (-1,-1) selection so
// the selCol/selCard resolution takes its "no highlight" path (the
// selected-card branch of drawCard is exercised separately by the pixel
// test).
func TestKanbanDrawNoSelection(t *testing.T) {
	if _, err := RenderImage(sampleBoard(), 300, 200, DefaultLight()); err != nil {
		t.Fatalf("RenderImage: %v", err)
	}
}

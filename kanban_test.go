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
	if k.SelectedCol().Get() != -1 || k.SelectedCard().Get() != -1 {
		t.Fatalf("NewKanban seeds: got (%d,%d), want (-1,-1)", k.SelectedCol().Get(), k.SelectedCard().Get())
	}
}

// TestKanbanBareAccessors exercises the nil→init lazy path of both selection
// accessors on a zero-value &Kanban{} (no constructor), and that a host binding
// via Subscribe on each observable observes the widget's own click mutation.
func TestKanbanBareAccessors(t *testing.T) {
	var k Kanban
	// Nil observables initialise to the zero value on first access.
	if got := k.SelectedCol().Get(); got != 0 {
		t.Fatalf("bare SelectedCol().Get() = %d, want 0", got)
	}
	if got := k.SelectedCard().Get(); got != 0 {
		t.Fatalf("bare SelectedCard().Get() = %d, want 0", got)
	}
	// Host binds both; the widget's own click Set path notifies on change.
	gotCol, gotCard := -1, -1
	k.SelectedCol().Subscribe(func(v int) { gotCol = v })
	k.SelectedCard().Subscribe(func(v int) { gotCard = v })
	k.Columns = []KanbanColumn{
		{Title: "A", Cards: []KanbanCard{{Title: "a0"}}},
		{Title: "B", Cards: []KanbanCard{{Title: "b0"}, {Title: "b1"}}},
	}
	k.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 300})
	colW := k.colWidth()
	lr := k.cardLocalRect(1, 1, colW) // column 1 (0->1), card 1 (0->1): both change
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 3, Y: lr.Y + 3})
	if k.SelectedCol().Get() != 1 || gotCol != 1 {
		t.Fatalf("after click: SelectedCol=%d host=%d, want 1/1", k.SelectedCol().Get(), gotCol)
	}
	if k.SelectedCard().Get() != 1 || gotCard != 1 {
		t.Fatalf("after click: SelectedCard=%d host=%d, want 1/1", k.SelectedCard().Get(), gotCard)
	}
	// Host drives the selection directly through the observables.
	k.SelectedCard().Set(0)
	if gotCard != 0 {
		t.Fatalf("host Set not observed: host=%d, want 0", gotCard)
	}
}

// TestKanbanRenderPixels renders a selected-card board and asserts the
// column panel, a card fill, an accent stripe and the selection tint all
// land in the expected colours, then writes the board to a PNG the parent
// can eyeball.
func TestKanbanRenderPixels(t *testing.T) {
	theme := DefaultLight()
	k := sampleBoard()
	k.SelectedCol().Set(1) // highlight "Build"
	k.SelectedCard().Set(0)

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
	if k.SelectedCol().Get() != 0 || k.SelectedCard().Get() != 1 {
		t.Fatalf("selection: got (%d,%d), want (0,1)", k.SelectedCol().Get(), k.SelectedCard().Get())
	}
	if gotCol != 0 || gotCard != 1 {
		t.Fatalf("OnCardClick: got (%d,%d), want (0,1)", gotCol, gotCard)
	}

	// Click in the header band (Y within KanbanHeaderH) misses every card.
	k.SelectedCol().Set(5)
	k.SelectedCard().Set(5)
	k.OnEvent(Event{Kind: EventClick, X: 10, Y: 2})
	if k.SelectedCol().Get() != 5 || k.SelectedCard().Get() != 5 {
		t.Fatalf("header click mutated selection: (%d,%d)", k.SelectedCol().Get(), k.SelectedCard().Get())
	}

	// A non-click event is ignored entirely.
	k.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if k.SelectedCol().Get() != 5 || k.SelectedCard().Get() != 5 {
		t.Fatalf("non-click event mutated selection: (%d,%d)", k.SelectedCol().Get(), k.SelectedCard().Get())
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
	if k.SelectedCol().Get() != 2 || k.SelectedCard().Get() != 0 {
		t.Fatalf("selection: got (%d,%d), want (2,0)", k.SelectedCol().Get(), k.SelectedCard().Get())
	}
}

// TestKanbanEmptyClick covers cardAt's empty-columns guard: a click on a
// board with no columns is a no-op.
func TestKanbanEmptyClick(t *testing.T) {
	k := NewKanban(nil)
	k.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	k.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if k.SelectedCol().Get() != -1 || k.SelectedCard().Get() != -1 {
		t.Fatalf("empty click mutated selection: (%d,%d)", k.SelectedCol().Get(), k.SelectedCard().Get())
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

// TestKanbanDragMovesCard exercises the full drag lifecycle: EventClick grabs
// a card, EventMouseDrag marks the gesture and tracks the pointer, a mid-drag
// Draw paints the overlay, and EventMouseUp drops the card at the target
// column/slot -- mutating Columns and firing OnCardMove with the landing spot.
func TestKanbanDragMovesCard(t *testing.T) {
	theme := DefaultLight()
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3

	moves := [4]int{-9, -9, -9, -9}
	k.OnCardMove = func(fc, fcd, tc, ti int) { moves = [4]int{fc, fcd, tc, ti} }

	lr := k.cardLocalRect(0, 0, colW) // grab "Design"
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 4, Y: lr.Y + 4})
	if !k.dragging || k.moved {
		t.Fatalf("after grab: dragging=%v moved=%v, want true/false", k.dragging, k.moved)
	}

	target := k.cardLocalRect(2, 0, colW) // into column 2, slot 0
	k.OnEvent(Event{Kind: EventMouseDrag, X: target.X + 4, Y: target.Y + 4})
	if !k.moved || k.dragX != target.X+4 || k.dragY != target.Y+4 {
		t.Fatalf("after drag: moved=%v drag=(%d,%d)", k.moved, k.dragX, k.dragY)
	}

	if _, err := RenderImage(k, 600, 300, theme); err != nil { // paints the overlay
		t.Fatalf("RenderImage mid-drag: %v", err)
	}

	k.OnEvent(Event{Kind: EventMouseUp, X: target.X + 4, Y: target.Y + 4})
	if k.dragging {
		t.Fatalf("still dragging after mouse up")
	}
	if got := k.Columns[2].Cards[0].Title; got != "Design" {
		t.Fatalf("col2 card0 = %q, want Design", got)
	}
	if len(k.Columns[0].Cards) != 1 || k.Columns[0].Cards[0].Title != "Research" {
		t.Fatalf("col0 after move = %+v", k.Columns[0].Cards)
	}
	if moves != [4]int{0, 0, 2, 0} {
		t.Fatalf("OnCardMove args = %v, want [0 0 2 0]", moves)
	}
}

// TestKanbanDragReleaseInPlace: a grab immediately released (no EventMouseDrag)
// is a plain click -- no card moves and OnCardMove never fires. Also covers the
// EventMouseDrag/EventMouseUp "not dragging" guards.
func TestKanbanDragReleaseInPlace(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3

	fired := false
	k.OnCardMove = func(int, int, int, int) { fired = true }

	// Guards: events before any grab are no-ops.
	k.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: 40})
	k.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: 40})

	lr := k.cardLocalRect(1, 0, colW)
	k.OnEvent(Event{Kind: EventClick, X: lr.X + 3, Y: lr.Y + 3})
	k.OnEvent(Event{Kind: EventMouseUp, X: lr.X + 3, Y: lr.Y + 3})
	if fired {
		t.Fatalf("OnCardMove fired for a press-release in place")
	}
	if len(k.Columns[1].Cards) != 1 || k.Columns[1].Cards[0].Title != "Build" {
		t.Fatalf("column 1 changed on a plain click: %+v", k.Columns[1].Cards)
	}
	if k.dragging {
		t.Fatalf("dragging still set after release")
	}
}

// TestKanbanColumnAt covers columnAt's middle-return, last-column fall-through
// and both clamps.
func TestKanbanColumnAt(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3
	if got := k.columnAt(-50); got != 0 {
		t.Fatalf("columnAt(-50) = %d, want 0", got)
	}
	if got := k.columnAt(k.colLocalX(1, colW) + 2); got != 1 {
		t.Fatalf("columnAt(col1) = %d, want 1", got)
	}
	if got := k.columnAt(100000); got != 2 {
		t.Fatalf("columnAt(huge) = %d, want 2", got)
	}
}

// TestKanbanDropIndexAt covers dropIndexAt's above-first, mid-slot rounding and
// past-end clamp.
func TestKanbanDropIndexAt(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	slot := KanbanCardH + KanbanCardGap
	if got := k.dropIndexAt(0, KanbanHeaderH-1); got != 0 {
		t.Fatalf("above first = %d, want 0", got)
	}
	if got := k.dropIndexAt(0, KanbanHeaderH+KanbanCardGap+slot); got != 1 {
		t.Fatalf("second slot = %d, want 1", got)
	}
	if got := k.dropIndexAt(0, 100000); got != 2 { // col0 has 2 cards
		t.Fatalf("past end = %d, want 2 (clamped)", got)
	}
}

// TestKanbanMoveCard covers moveCard's invalid-source guard, same-column
// downward shift (toIdx decrement), cross-column append with a > len clamp, and
// the toIdx < 0 clamp.
func TestKanbanMoveCard(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})

	k.moveCard(0, 99, 1, 0) // invalid source: no-op
	if len(k.Columns[0].Cards) != 2 {
		t.Fatalf("invalid source mutated col0: %+v", k.Columns[0].Cards)
	}

	k.moveCard(0, 0, 0, 2) // Design to end of col0 -> [Research, Design]
	if got := []string{k.Columns[0].Cards[0].Title, k.Columns[0].Cards[1].Title}; got[0] != "Research" || got[1] != "Design" {
		t.Fatalf("same-col move = %v, want [Research Design]", got)
	}

	k.moveCard(0, 0, 1, 99) // Research -> col1 end (clamped)
	last := k.Columns[1].Cards[len(k.Columns[1].Cards)-1].Title
	if last != "Research" {
		t.Fatalf("cross-col append last = %q, want Research", last)
	}

	k.moveCard(2, 0, 0, -5) // Ship -> col0 front (toIdx clamped up to 0)
	if k.Columns[0].Cards[0].Title != "Ship" {
		t.Fatalf("neg toIdx front = %q, want Ship", k.Columns[0].Cards[0].Title)
	}
}

// TestKanbanDragOverlayStaleCard covers drawDragOverlay's stale-dragCard guard:
// a drag flagged with an out-of-range card still paints the drop indicator but
// skips the ghost, without panicking or painting outside Bounds.
func TestKanbanDragOverlayStaleCard(t *testing.T) {
	theme := DefaultLight()
	k := sampleBoard()
	k.dragging, k.moved = true, true
	k.dragCol, k.dragCard = 0, 99 // stale
	k.dragX, k.dragY = 50, 120
	if _, err := RenderImage(k, 600, 300, theme); err != nil {
		t.Fatalf("RenderImage stale-card drag: %v", err)
	}
}

// TestKanbanExportedHitAndMove covers the exported CardAt / MoveCard wrappers a
// host uses to build a right-click context menu and drive a move from it.
func TestKanbanExportedHitAndMove(t *testing.T) {
	k := sampleBoard()
	k.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 300})
	colW := (600 - 2*KanbanColGap) / 3

	lr := k.cardLocalRect(0, 1, colW) // "Research"
	if col, card := k.CardAt(lr.X+3, lr.Y+3); col != 0 || card != 1 {
		t.Fatalf("CardAt = (%d,%d), want (0,1)", col, card)
	}
	if col, _ := k.CardAt(5, 2); col != -1 { // header
		t.Fatalf("CardAt(header) col = %d, want -1", col)
	}
	k.MoveCard(0, 0, 2, 0) // "Design" -> Done, slot 0
	if k.Columns[2].Cards[0].Title != "Design" {
		t.Fatalf("MoveCard did not move Design to column 2: %+v", k.Columns[2].Cards)
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Kanban renders a Trello-style board: a row of equal-width columns
// (lists) laid side by side across the widget's Bounds, each a titled
// header band above a vertical stack of rounded cards. It is the
// missing "workflow board" primitive next to Table (a data grid) and
// ListBox (a single column of items) -- Kanban is many columns, each a
// stack of two-line cards, the shape a to-do / progress board needs.
//
// Visual (per column):
//
//	+-----------------------+
//	| Title            (n)  |  <- KanbanHeaderH, SurfaceAlt, count Badge
//	+-----------------------+
//	| |  Card title         |  <- KanbanCardH, Surface, accent stripe
//	| |  muted subtitle     |
//	+-----------------------+
//	| |  Card title         |
//	| |  muted subtitle     |
//	+-----------------------+
//
// Columns share the horizontal budget equally (like a fixed HBox),
// separated by KanbanColGap. Each card carries a left accent stripe --
// KanbanCard.Accent, or Theme.Accent when that is the zero value -- so a
// board can colour-code cards by category without the host hand-drawing
// anything. The selected card (SelectedCol/SelectedCard) paints on an
// accent-tinted fill with an accent border.
//
// A column whose cards overflow its height clips to its body (via
// painter.Clipper, the same graceful degradation Table relies on) AND
// scrolls independently: the wheel over a column shifts that column's card
// stack (colScroll) so cards past the fold are reachable, each column clipped
// to its own window. Clicking a card selects it and fires OnCardClick; clicks
// on headers, gaps or dead space are no-ops.
type Kanban struct {
	Base
	// Columns are the board's lists, left to right.
	Columns []KanbanColumn
	// OnCardClick fires when a card is clicked, with the 0-indexed column
	// and card. Nil is safe (the click still updates the selection).
	OnCardClick func(col, card int)
	// OnCardMove fires when a drag drops a card at a new position, with the
	// source (fromCol, fromCard) and the destination (toCol, toIdx) the card
	// now occupies. Nil is safe -- the board still updates Columns in place.
	OnCardMove func(fromCol, fromCard, toCol, toIdx int)

	// Drag state, following the toolkit's grab-on-EventClick / track-on-
	// EventMouseDrag / release-on-EventMouseUp convention (as Table's column
	// resize and RangeSlider do): dragging is true between grabbing a card and
	// releasing it, dragCol/dragCard is the grabbed card, dragX/dragY the
	// latest pointer in widget-local coords, and moved guards OnCardMove so a
	// press-release in place stays a plain click rather than a no-op "move".
	dragging     bool
	moved        bool
	dragCol      int
	dragCard     int
	dragX, dragY int

	// colScroll is the per-column vertical scroll offset in pixels: the amount
	// column i's card stack is shifted up so cards past the fold are reachable.
	// The wheel over a column shifts its entry (see scrollColumn), Draw and the
	// card hit-test read it through colScrollAt (which clamps on the fly, so a
	// value left stale after a column shrank is harmless), and each column clips
	// to its own body. Grown lazily to len(Columns); a nil/short slice means
	// "every column at offset 0" — byte-identical to before scrolling existed.
	colScroll []int

	// selectedCol / selectedCard identify the highlighted card. The reactive
	// selection is MVVM-only: the current (col, card) lives in two unexported
	// Observables exposed via [Kanban.SelectedCol] / [Kanban.SelectedCard], each
	// -1 (the value NewKanban seeds) for "no selection". An out-of-range pair
	// collapses to "no highlight" in Draw the same defensive way Table collapses
	// a stale Selected.
	selectedCol  *mvvm.Observable[int]
	selectedCard *mvvm.Observable[int]
}

// SelectedCol is the highlighted card's column index as a shared
// [mvvm.Observable]: a host binds it (Set / Subscribe / two-way) — there is no
// settable SelectedCol field. A card click Sets it (paired with SelectedCard);
// subscribers are notified. A bare &Kanban{} lazily seeds 0 on first access,
// matching the old zero-value field.
func (k *Kanban) SelectedCol() *mvvm.Observable[int] {
	if k.selectedCol == nil {
		k.selectedCol = mvvm.NewObservable(0)
	}
	return k.selectedCol
}

// SelectedCard is the highlighted card's index within its column as a shared
// [mvvm.Observable]: a host binds it (Set / Subscribe / two-way) — there is no
// settable SelectedCard field. A card click Sets it (paired with SelectedCol);
// subscribers are notified. A bare &Kanban{} lazily seeds 0 on first access,
// matching the old zero-value field.
func (k *Kanban) SelectedCard() *mvvm.Observable[int] {
	if k.selectedCard == nil {
		k.selectedCard = mvvm.NewObservable(0)
	}
	return k.selectedCard
}

// KanbanCard is one card in a column: a bold Title over a muted
// Subtitle, with a left accent stripe drawn in Accent -- or Theme.Accent
// when Accent is the zero RGBA (A==0), the same "unset falls back to the
// theme" convention Badge.Fill uses. Either text may be "" to omit that
// line.
type KanbanCard struct {
	Title    string
	Subtitle string
	Accent   RGBA
}

// KanbanColumn is one list: a header Title above its stack of Cards.
type KanbanColumn struct {
	Title string
	Cards []KanbanCard
}

// Kanban layout constants, exported so a host can measure a board the
// same way it reads TableRowHeight / CardHeaderH.
const (
	// KanbanHeaderH is the pixel height of a column's header band.
	KanbanHeaderH = 28
	// KanbanCardH is the pixel height of one card.
	KanbanCardH = 46
	// KanbanColGap is the horizontal gap between adjacent columns.
	KanbanColGap = 8
	// KanbanCardGap is the vertical gap between stacked cards (and the
	// horizontal inset of a card from its column's edges).
	KanbanCardGap = 6
	// KanbanCardPadX is the inner horizontal text inset inside a card /
	// column header.
	KanbanCardPadX = 8
	// KanbanStripeW is the pixel width of a card's left accent stripe.
	KanbanStripeW = 4
	// KanbanCardRadius is the corner radius of a card's rounded body.
	KanbanCardRadius = 6
)

// kanbanStripeInset is the pixel inset of the accent stripe from a
// card's top/bottom/left edges, so the stripe reads as an inner bar
// rather than touching the rounded corners.
const kanbanStripeInset = 6

// NewKanban builds a Kanban from cols with no card selected
// (SelectedCol / SelectedCard both -1), mirroring how NewTable seeds
// Selected to -1.
func NewKanban(cols []KanbanColumn) *Kanban {
	k := &Kanban{Columns: cols}
	k.selectedCol = mvvm.NewObservable(-1)
	k.selectedCard = mvvm.NewObservable(-1)
	return k
}

// colWidth is the equal pixel width each column claims: the widget's
// width minus the inter-column gaps, divided evenly. It floors at 0 so a
// board too narrow to hold its gaps never yields a negative width (the
// painter's own clipping then keeps the sub-pixel columns harmless).
// Callers guard len(Columns) > 0 before calling, so the divisor is
// always positive.
func (k *Kanban) colWidth() int {
	n := len(k.Columns)
	total := k.Bounds().W - (n-1)*KanbanColGap
	if total < 0 {
		total = 0
	}
	return total / n
}

// colLocalX is column i's left edge in widget-local coordinates.
func (k *Kanban) colLocalX(i, colW int) int {
	return i * (colW + KanbanColGap)
}

// cardLocalRect is card ci of column i in widget-local coordinates: a
// card is inset from its column by KanbanCardGap on each horizontal
// edge and stacked below the header with a KanbanCardGap lead and gap,
// shifted up by the column's scroll offset so a scrolled stack reveals
// its lower cards. Draw offsets the result by the widget's Bounds origin;
// cardAt tests against it directly, so hit-testing follows the scroll.
func (k *Kanban) cardLocalRect(i, ci, colW int) Rect {
	return Rect{
		X: k.colLocalX(i, colW) + KanbanCardGap,
		Y: KanbanHeaderH + KanbanCardGap + ci*(KanbanCardH+KanbanCardGap) - k.colScrollAt(i),
		W: colW - 2*KanbanCardGap,
		H: KanbanCardH,
	}
}

// cardSlot is the vertical stride of one card in a column's stack: the card
// height plus the gap below it. One wheel notch scrolls by this much.
const cardSlot = KanbanCardH + KanbanCardGap

// colBodyH is the visible pixel height of a column's card area — the bounds
// height below the header band and its 1px divider. Floored at 0.
func (k *Kanban) colBodyH() int {
	h := k.Bounds().H - KanbanHeaderH - 1
	if h < 0 {
		h = 0
	}
	return h
}

// colContentH is the total pixel height column i's card stack occupies (a lead
// gap plus one slot per card), or 0 for an empty column.
func (k *Kanban) colContentH(i int) int {
	n := len(k.Columns[i].Cards)
	if n == 0 {
		return 0
	}
	return KanbanCardGap + n*cardSlot
}

// colMaxScroll is the highest scroll offset that still leaves the last card
// against the column's bottom edge: content minus body, floored at 0 so a
// column that already fits never scrolls.
func (k *Kanban) colMaxScroll(i int) int {
	m := k.colContentH(i) - k.colBodyH()
	if m < 0 {
		m = 0
	}
	return m
}

// colScrollAt returns column i's scroll offset clamped to [0, colMaxScroll(i)]
// without mutating anything, so a nil/short slice or a value left stale after a
// column shrank never paints or hit-tests outside the valid window.
func (k *Kanban) colScrollAt(i int) int {
	if i < 0 || i >= len(k.colScroll) {
		return 0
	}
	s := k.colScroll[i]
	if s < 0 {
		s = 0
	}
	if m := k.colMaxScroll(i); s > m {
		s = m
	}
	return s
}

// scrollColumn shifts column i's scroll offset by delta pixels, clamped to
// [0, colMaxScroll(i)] and written back (growing colScroll to len(Columns) the
// first time a column is scrolled). Out-of-range columns are ignored.
func (k *Kanban) scrollColumn(i, delta int) {
	if i < 0 || i >= len(k.Columns) {
		return
	}
	if len(k.colScroll) < len(k.Columns) {
		grown := make([]int, len(k.Columns))
		copy(grown, k.colScroll)
		k.colScroll = grown
	}
	s := k.colScroll[i] + delta
	if s < 0 {
		s = 0
	}
	if m := k.colMaxScroll(i); s > m {
		s = m
	}
	k.colScroll[i] = s
}

// Draw paints every column: a SurfaceAlt panel + header band with the
// title and a count Badge, a Border divider, then the column's cards
// clipped to the column body. The selected card (if any) paints on an
// accent-tinted fill with an accent border; every other card on Surface
// with a Border stroke and its left accent stripe.
func (k *Kanban) Draw(p painter.Painter, theme *Theme) {
	r := k.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	n := len(k.Columns)
	if n == 0 {
		return
	}
	colW := k.colWidth()

	// Resolve the highlighted card, collapsing an out-of-range pair to
	// "none" so a stale or unconstructed Selected* never enters the
	// accent branch for a card that no longer exists.
	selCol, selCard := -1, -1
	sc, scd := k.SelectedCol().Get(), k.SelectedCard().Get()
	if sc >= 0 && sc < n &&
		scd >= 0 && scd < len(k.Columns[sc].Cards) {
		selCol, selCard = sc, scd
	}

	for i, col := range k.Columns {
		colX := r.X + k.colLocalX(i, colW)
		// Column panel + header band.
		fillRect(p, colX, r.Y, colW, r.H, theme.SurfaceAlt)
		hty := r.Y + (KanbanHeaderH-k.glyphHeight())/2
		k.drawText(p, colX+KanbanCardPadX, hty, col.Title, theme.OnSurface)
		k.drawCountBadge(p, theme, colX, colW, len(col.Cards))
		// Divider under the header.
		fillRect(p, colX, r.Y+KanbanHeaderH, colW, 1, theme.Border)

		// Cards, clipped to the column body so an overflowing stack never
		// spills past the widget's bottom edge.
		bodyY := r.Y + KanbanHeaderH + 1
		withClip(p, Rect{X: colX, Y: bodyY, W: colW, H: r.Y + r.H - bodyY}, func() {
			for ci, card := range col.Cards {
				lr := k.cardLocalRect(i, ci, colW)
				cr := Rect{X: r.X + lr.X, Y: r.Y + lr.Y, W: lr.W, H: lr.H}
				k.drawCard(p, theme, cr, card, i == selCol && ci == selCard)
			}
		})
	}

	// While a card is being dragged, overlay a drop indicator at the target
	// slot and a floating ghost of the grabbed card under the pointer.
	if k.dragging && k.moved {
		k.drawDragOverlay(p, theme, r, colW)
	}
}

// drawDragOverlay paints the in-flight drag feedback: a 2px accent insertion
// bar spanning the target column at the drop slot, and a selected-tint ghost
// of the grabbed card centred on the pointer. Both are clamped/clipped to the
// widget so the overlay never paints outside Bounds().
func (k *Kanban) drawDragOverlay(p painter.Painter, theme *Theme, r Rect, colW int) {
	toCol := k.columnAt(k.dragX)
	toIdx := k.dropIndexAt(toCol, k.dragY)
	colX := r.X + k.colLocalX(toCol, colW)
	slot := cardSlot
	indY := r.Y + KanbanHeaderH + KanbanCardGap + toIdx*slot - KanbanCardGap/2 - k.colScrollAt(toCol)
	bodyY := r.Y + KanbanHeaderH + 1
	withClip(p, Rect{X: colX, Y: bodyY, W: colW, H: r.Y + r.H - bodyY}, func() {
		fillRect(p, colX+KanbanCardGap, indY, colW-2*KanbanCardGap, 2, theme.Accent)
	})
	if k.dragCol < len(k.Columns) && k.dragCard < len(k.Columns[k.dragCol].Cards) {
		card := k.Columns[k.dragCol].Cards[k.dragCard]
		gw := colW - 2*KanbanCardGap
		gx := clampInt(r.X+k.dragX-gw/2, r.X, r.X+r.W-gw)
		gy := clampInt(r.Y+k.dragY-KanbanCardH/2, r.Y, r.Y+r.H-KanbanCardH)
		withClip(p, r, func() {
			k.drawCard(p, theme, Rect{X: gx, Y: gy, W: gw, H: KanbanCardH}, card, true)
		})
	}
}

// drawCountBadge paints the per-column card-count Badge, right-aligned in
// the header band, reusing Badge's rounded pill so the count reads the
// same as any other counter in the toolkit.
func (k *Kanban) drawCountBadge(p painter.Painter, theme *Theme, colX, colW, count int) {
	txt := strconv.Itoa(count)
	bw := k.textWidth(txt) + 2*BadgePadX
	bh := k.glyphHeight() + 2*BadgePadY
	bx := colX + colW - KanbanCardPadX - bw
	by := k.Bounds().Y + (KanbanHeaderH-bh)/2
	b := &Badge{Text: txt}
	b.Font = k.Font
	b.SetBounds(Rect{X: bx, Y: by, W: bw, H: bh})
	b.Draw(p, theme)
}

// drawCard paints one card at surface rect cr: a rounded body (Surface,
// or an accent tint when selected), a left accent stripe, the title and
// muted subtitle, and a rounded border (Accent when selected, Border
// otherwise). accent falls back to Theme.Accent when the card leaves its
// own Accent unset (A==0), mirroring Badge.Fill.
func (k *Kanban) drawCard(p painter.Painter, theme *Theme, cr Rect, card KanbanCard, selected bool) {
	accent := theme.Accent
	if card.Accent.A != 0 {
		accent = card.Accent
	}
	fill := theme.Surface
	border := theme.Border
	if selected {
		fill = kanbanSelTint(theme)
		border = theme.Accent
	}
	fillRoundRect(p, cr.X, cr.Y, cr.W, cr.H, KanbanCardRadius, fill)
	// Left accent stripe, inset from the rounded corners.
	fillRect(p, cr.X+kanbanStripeInset, cr.Y+kanbanStripeInset, KanbanStripeW, cr.H-2*kanbanStripeInset, accent)
	// Two text lines to the right of the stripe.
	textX := cr.X + kanbanStripeInset + KanbanStripeW + KanbanCardPadX
	k.drawText(p, textX, cr.Y+kanbanStripeInset, card.Title, theme.OnSurface)
	k.drawText(p, textX, cr.Y+kanbanStripeInset+k.glyphHeight()+2, card.Subtitle, dimInk(theme))
	strokeRoundRect(p, cr.X, cr.Y, cr.W, cr.H, KanbanCardRadius, border)
}

// kanbanSelTint is the fill under the selected card: a one-third Accent /
// two-thirds Surface blend -- enough accent wash to read as selected
// against the plain-Surface cards, without the full-accent slab Table
// uses for a whole row (a card keeps its dark title text legible). Built
// the same channel-blend way as dimInk so it holds up in any theme.
func kanbanSelTint(theme *Theme) RGBA {
	a, s := theme.Accent, theme.Surface
	return RGBA{
		R: uint8((int(a.R) + 2*int(s.R)) / 3),
		G: uint8((int(a.G) + 2*int(s.G)) / 3),
		B: uint8((int(a.B) + 2*int(s.B)) / 3),
		A: 255,
	}
}

// cardAt maps widget-local (localX, localY) to the (col, card) it lands
// on, or (-1, -1) for a click on a header, a gap or dead space. It walks
// the same cardLocalRect geometry Draw paints with, so a hit resolves to
// exactly the card the user sees under the pointer.
func (k *Kanban) cardAt(localX, localY int) (int, int) {
	n := len(k.Columns)
	if n == 0 {
		return -1, -1
	}
	colW := k.colWidth()
	for i := range k.Columns {
		for ci := range k.Columns[i].Cards {
			lr := k.cardLocalRect(i, ci, colW)
			if localX >= lr.X && localX < lr.X+lr.W &&
				localY >= lr.Y && localY < lr.Y+lr.H {
				return i, ci
			}
		}
	}
	return -1, -1
}

// columnAt returns the column index whose horizontal span (including the gap
// to its right) contains widget-local localX, clamped to [0, n-1] so a drag
// released left of the board drops in the first column and right of it in the
// last. Callers guard len(Columns) > 0.
func (k *Kanban) columnAt(localX int) int {
	n := len(k.Columns)
	colW := k.colWidth()
	for i := 0; i < n-1; i++ {
		if localX < k.colLocalX(i, colW)+colW+KanbanColGap {
			return i
		}
	}
	return n - 1
}

// dropIndexAt returns the insertion index within column col for a card
// released at widget-local localY: the number of card slots whose midline the
// pointer has passed, clamped to [0, len(cards)]. It rounds to the nearest
// slot boundary so dropping over the top half of a card inserts above it.
func (k *Kanban) dropIndexAt(col, localY int) int {
	cards := len(k.Columns[col].Cards)
	// Add the column's scroll offset so a drop over a scrolled stack targets the
	// slot the pointer visually sits on, not the unscrolled one.
	rel := localY - (KanbanHeaderH + KanbanCardGap) + k.colScrollAt(col)
	if rel < 0 {
		return 0
	}
	slot := cardSlot
	idx := (rel + slot/2) / slot
	if idx > cards {
		idx = cards
	}
	return idx
}

// moveCard removes the card at (fromCol, fromCard) and re-inserts it at toIdx
// in toCol, updating Selected* to its landing spot. Out-of-range sources are
// ignored; toIdx is clamped to the destination's new length. When moving down
// within the same column the removal shifts later slots up by one, so toIdx is
// decremented to keep the visual drop position.
func (k *Kanban) moveCard(fromCol, fromCard, toCol, toIdx int) {
	if fromCol < 0 || fromCol >= len(k.Columns) ||
		fromCard < 0 || fromCard >= len(k.Columns[fromCol].Cards) {
		return
	}
	card := k.Columns[fromCol].Cards[fromCard]
	src := k.Columns[fromCol].Cards
	k.Columns[fromCol].Cards = append(src[:fromCard], src[fromCard+1:]...)
	if toCol == fromCol && toIdx > fromCard {
		toIdx--
	}
	dst := k.Columns[toCol].Cards
	if toIdx < 0 {
		toIdx = 0
	}
	if toIdx > len(dst) {
		toIdx = len(dst)
	}
	dst = append(dst, KanbanCard{})
	copy(dst[toIdx+1:], dst[toIdx:])
	dst[toIdx] = card
	k.Columns[toCol].Cards = dst
	k.SelectedCol().Set(toCol)
	k.SelectedCard().Set(toIdx)
}

// CardAt maps widget-local (x, y) to the (column, card) it lands on, or
// (-1, -1) for a header, gap or dead space. Exposed so a host can hit-test a
// right-click and build a context menu for the card under the cursor.
func (k *Kanban) CardAt(x, y int) (col, card int) { return k.cardAt(x, y) }

// MoveCard removes the card at (fromCol, fromCard) and re-inserts it at toIdx
// in toCol, updating Selected* to its landing spot (out-of-range sources are
// ignored; toIdx is clamped). Exposed so a host can drive the same move a drag
// performs from a menu action.
func (k *Kanban) MoveCard(fromCol, fromCard, toCol, toIdx int) {
	k.moveCard(fromCol, fromCard, toCol, toIdx)
}

// OnEvent drives selection and card drag-and-drop. On EventClick it grabs the
// card under the pointer (selecting it and firing OnCardClick); on
// EventMouseDrag it tracks the pointer and marks the gesture a drag; on
// EventMouseUp it drops the grabbed card at the target column/slot, mutating
// Columns and firing OnCardMove. A press-release with no intervening drag
// leaves the board a plain click. Both callbacks are nil-safe.
func (k *Kanban) OnEvent(ev Event) {
	switch ev.Kind {
	case EventScroll:
		// Native wheel scroll: shift the card stack of the column under the
		// pointer by Delta card slots (clamped at both ends). A no-op when the
		// column already fits or the board is empty.
		if len(k.Columns) == 0 {
			return
		}
		k.scrollColumn(k.columnAt(ev.X), ev.Delta*cardSlot)
	case EventClick:
		col, card := k.cardAt(ev.X, ev.Y)
		if col < 0 {
			return
		}
		k.dragging, k.moved = true, false
		k.dragCol, k.dragCard = col, card
		k.dragX, k.dragY = ev.X, ev.Y
		k.SelectedCol().Set(col)
		k.SelectedCard().Set(card)
		if k.OnCardClick != nil {
			k.OnCardClick(col, card)
		}
	case EventMouseDrag:
		if !k.dragging {
			return
		}
		k.dragX, k.dragY = ev.X, ev.Y
		k.moved = true
	case EventMouseUp:
		if !k.dragging {
			return
		}
		fromCol, fromCard, wasMoved := k.dragCol, k.dragCard, k.moved
		k.dragging, k.moved = false, false
		if !wasMoved {
			return
		}
		toCol := k.columnAt(ev.X)
		toIdx := k.dropIndexAt(toCol, ev.Y)
		k.moveCard(fromCol, fromCard, toCol, toIdx)
		if k.OnCardMove != nil {
			k.OnCardMove(fromCol, fromCard, toCol, k.SelectedCard().Get())
		}
	}
}

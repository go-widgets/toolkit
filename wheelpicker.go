// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strings"

	"github.com/go-widgets/painter"
)

// WheelPicker is the iOS-style spinning wheel of discrete values: one or more
// COLUMNS, each a vertical strip of string values under a fixed, centred
// selection band. A flick spins a column with inertial deceleration and, once
// the coast dies, the strip SNAPS so a row sits exactly under the band — it can
// never rest half-way between two values. It generalises the specialised
// [DatePicker] / [TimePicker] steppers: a date wheel is three columns
// (day/month/year), a time wheel is two (hour/minute), and any enumerated field
// is one column of its labels.
//
// # Reusing the momentum engine for BOTH the spin and the snap
//
// Each column owns a single-axis [Momentum] engine — the same deterministic,
// clock-free fling+rubber-band engine [MomentumScroller] drives — so the spin
// physics are not reinvented here: a release hands the engine a velocity, the
// engine coasts under exponential deceleration, and a flick past the first/last
// row stretches against the rubber band and springs back, exactly as it does for
// a scroll view.
//
// The SNAP-to-nearest-row reuses that same engine's spring rather than adding a
// second physics path. Momentum already knows how to spring an offset that sits
// PAST a bound back ONTO that bound and rest there exactly (see
// [Momentum.tickSpring]); a detent is just a bound. So when a coast comes to
// rest between two rows, the column momentarily sets the engine's clamp window to
// the degenerate span [D, D] at the nearest detent D and re-flings from rest: the
// current offset is now "past" that bound, the engine springs it onto D, and the
// instant it arrives it snaps to the exact detent and stops — the very same
// spring that lands a scroll view precisely on its edge, retargeted at a row
// boundary. When the snap finishes the real bounds are restored.
//
// # Row units, not pixels
//
// A column's offset is measured in ROWS, not pixels: offset 0 shows item 0 under
// the band, offset 1 shows item 1, and the selected index is simply the offset
// rounded to the nearest whole row. Keeping the physics in row units makes the
// widget independent of [Density] and [MetricScale] — the pixel row height only
// enters at Draw time and when converting a finger's pixel travel into rows — so
// a density flip never desynchronises the value under the band, and every detent
// is an exact integer the tests can assert to the bit.
//
// # Determinism
//
// Like the engine it composes, the widget reads no clock. The inertial path is
// driven by explicit calls — [WheelPicker.TouchDown], [WheelPicker.TouchMove]
// (which carries its own dt), [WheelPicker.TouchUp] and a per-frame
// [WheelPicker.Tick] — mirroring [MomentumScroller], so a host supplies the
// elapsed time and the same inputs always produce the same offsets. The discrete
// path — keyboard arrows, a wheel notch, a tap above/below the band — runs
// through [WheelPicker.OnEvent] and needs no clock at all.
type WheelPicker struct {
	Base

	// VisibleRows is how many rows the wheel shows at once, including the centred
	// selection row. It is coerced to an odd number >= 1 by NewWheelPicker (and by
	// the accessor) so there is always a single, unambiguous centre row. The
	// default is wheelVisibleRows.
	VisibleRows int

	// OnChange fires whenever any column's selected index changes — while a spin
	// crosses a row, when a snap settles, on a keyboard step, a wheel notch or a
	// tap. It reports the zero-based column and the new selected row index. A nil
	// OnChange is safe (the change is applied silently).
	OnChange func(col, index int)

	columns []*wheelColumn

	// focus is the column the keyboard acts on (Arrow Up/Down change its value;
	// Arrow Left/Right move focus between columns). It stays in range for any
	// non-empty column set.
	focus int

	// Touch-drag state for the inertial path. dragCol locks the gesture to the
	// column the finger first landed on; track smooths the release velocity.
	track    VelocityTracker
	dragging bool
	dragCol  int
	lastY    int
}

// wheelColumn is one spinning strip: its immutable list of values, the momentum
// engine that carries its offset (in ROWS), the last selected index it reported,
// and a snapping flag marking the brief spring-to-detent phase during which the
// engine's bounds are the degenerate [D, D] rather than the real [0, n-1].
type wheelColumn struct {
	items    []string
	mom      *Momentum
	index    int
	snapping bool
}

// WheelPicker metrics (logical pixels, routed through scaled). Only Draw and the
// pixel<->row conversion use them; the physics are unit-less rows.
const (
	// wheelRowH is the base height of a single row.
	wheelRowH = 28
	// wheelVisibleRows is the default (odd) count of rows shown at once.
	wheelVisibleRows = 5
	// wheelColMinW is the floor a column's drawn width is clamped up to, so a
	// narrow bounds still gives each column a legible strip.
	wheelColMinW = 40
)

// Row-unit momentum tuning for a wheel column. Friction and the spring constants
// are unit-free or per-time, so they carry over from the pixel engine unchanged;
// StopVelocity, MaxOverscroll and SnapDistance are re-expressed in ROWS so the
// glide, the rubber band and the settle threshold read naturally on a wheel.
const (
	wheelFriction      = 0.06 // fraction of velocity kept per second (same feel as scroll)
	wheelStopVelocity  = 0.3  // rows/s below which a coast is stopped
	wheelStiffness     = 200.0
	wheelDamping       = 28.0
	wheelMaxOverscroll = 0.9  // rows of rubber-band stretch past the first/last row
	wheelSnapDistance  = 0.01 // rows; sub-detent overshoot counts as "home"
)

// newWheelColumn builds a column of items with a row-unit momentum engine, its
// offset at row 0 and bounds spanning the whole list.
func newWheelColumn(items []string) *wheelColumn {
	m := &Momentum{
		Friction:      wheelFriction,
		StopVelocity:  wheelStopVelocity,
		Bounce:        true,
		Stiffness:     wheelStiffness,
		Damping:       wheelDamping,
		MaxOverscroll: wheelMaxOverscroll,
		SnapDistance:  wheelSnapDistance,
	}
	c := &wheelColumn{items: items, mom: m}
	c.applyBounds()
	return c
}

// NewWheelPicker builds a wheel with one column per string slice, each column
// initialised to select its first row (index 0). VisibleRows defaults to
// wheelVisibleRows. A caller may pass no columns (an empty wheel that draws its
// frame and ignores input) or a column with no items (a blank strip); neither is
// an error, so a data-driven caller never has to guard the degenerate cases.
func NewWheelPicker(columns ...[]string) *WheelPicker {
	w := &WheelPicker{VisibleRows: wheelVisibleRows}
	for _, items := range columns {
		w.columns = append(w.columns, newWheelColumn(items))
	}
	return w
}

// NumColumns is the number of columns in the wheel.
func (w *WheelPicker) NumColumns() int { return len(w.columns) }

// visibleRows returns VisibleRows coerced to an odd number >= 1, so drawing and
// hit-testing always have a single centre row regardless of what a caller stored.
func (w *WheelPicker) visibleRows() int {
	n := w.VisibleRows
	if n < 1 {
		n = 1
	}
	if n%2 == 0 {
		n++
	}
	return n
}

// rowHeight is a row's height in DEVICE pixels at the current HiDPI scale and
// touch density, floored at a hit-target-sized band so a finger can land a row
// under [DensityTouch]. It is the sole place a pixel size enters the widget; the
// physics never see it.
func (w *WheelPicker) rowHeight() int {
	return TouchTarget(scaled(wheelRowH))
}

// --- Column geometry & index math -----------------------------------------

// count is the number of items in a column.
func (c *wheelColumn) count() int { return len(c.items) }

// maxOffset is the highest valid offset (in rows): the last row's index, or 0
// when the column has zero or one item (nothing to spin).
func (c *wheelColumn) maxOffset() float64 {
	if c.count() <= 1 {
		return 0
	}
	return float64(c.count() - 1)
}

// applyBounds resets the engine's clamp window to the real [0, maxOffset] span.
// It is skipped mid-snap, when the bounds are deliberately the degenerate detent
// span, so a snap in flight is never disturbed.
func (c *wheelColumn) applyBounds() {
	if c.snapping {
		return
	}
	c.mom.SetBounds(0, c.maxOffset())
}

// offsetRows is the column's current offset in rows (may be fractional mid-spin
// or slightly out of range mid-rubber-band).
func (c *wheelColumn) offsetRows() float64 { return c.mom.Offset() }

// indexAt is the selected index the current offset maps to: the offset rounded
// to the nearest whole row, clamped into [0, count-1]. An empty column reports 0.
func (c *wheelColumn) indexAt() int {
	if c.count() == 0 {
		return 0
	}
	i := int(math.Round(c.offsetRows()))
	if i < 0 {
		i = 0
	}
	if i >= c.count() {
		i = c.count() - 1
	}
	return i
}

// nearestDetent is the offset (in rows) of the row nearest the current offset —
// the exact integer the snap springs onto. It reuses indexAt's clamped rounding
// so the detent can never fall outside the list.
func (c *wheelColumn) nearestDetent() float64 { return float64(c.indexAt()) }

// --- Spin + snap (momentum reuse) -----------------------------------------

// beginSnap starts the spring-to-nearest-row phase, reusing the momentum spring:
// it collapses the engine's bounds to the degenerate span [D, D] at the nearest
// detent D and re-flings from the current (off-detent) offset, which — being
// "past" that bound — the engine springs onto D and rests. It reports whether a
// snap was actually needed; an offset already exactly on a detent needs none.
func (c *wheelColumn) beginSnap() bool {
	off := c.offsetRows()
	d := c.nearestDetent()
	if off == d {
		return false
	}
	// Collapse to the detent as a degenerate bound. SetBounds does NOT re-clamp
	// the live offset, so it stays at off (past the new bound); Fling from rest
	// then enters the spring, which lands exactly on d.
	c.mom.SetBounds(d, d)
	c.snapping = true
	c.mom.Fling(0)
	return true
}

// endSnap concludes a finished snap: it clears the flag, restores the real
// bounds and re-seats the (now exactly-on-detent) offset within them.
func (c *wheelColumn) endSnap() {
	rest := c.mom.Offset()
	c.snapping = false
	c.mom.SetBounds(0, c.maxOffset())
	c.mom.SetOffset(rest)
}

// tick advances the column by dt seconds and reports whether it still owes
// motion. Three cases: (1) a snap in flight advances the spring and, when it
// lands, restores bounds; (2) a coast that just died between rows kicks off a
// snap; (3) a coast still in flight simply reports it. It never reads a clock.
func (c *wheelColumn) tick(dt float64) bool {
	moving := c.mom.Tick(dt)
	if c.snapping {
		if !moving {
			c.endSnap()
		}
		return moving
	}
	if !moving {
		return c.beginSnap()
	}
	return true
}

// --- WheelPicker inertial path (host supplies dt, mirrors MomentumScroller) --

// columnAt maps a widget-local x to the column under it, or -1 when the wheel has
// no columns or x is outside them. Columns divide the widget width evenly.
func (w *WheelPicker) columnAt(localX int) int {
	n := len(w.columns)
	if n == 0 {
		return -1
	}
	r := w.Bounds()
	if localX < 0 || localX >= r.W {
		return -1
	}
	// localX is in [0, r.W), so localX*n/r.W is in [0, n) — never needs clamping.
	return localX * n / r.W
}

// TouchDown starts a finger drag on the column under ev (widget-local coords),
// stopping any coast or snap on that column first so the strip tracks the finger
// from where it actually sits. It arms the gesture; a following TouchMove pans
// it. A press outside every column is ignored.
func (w *WheelPicker) TouchDown(ev Event) {
	if w.Disabled().Get() {
		return
	}
	col := w.columnAt(ev.X)
	if col < 0 {
		return
	}
	c := w.columns[col]
	c.snapping = false
	c.applyBounds()
	c.mom.Stop()
	c.mom.BeginDrag()
	w.track.Reset()
	w.dragging = true
	w.dragCol = col
	w.lastY = ev.Y
	w.focus = col
}

// TouchMove pans the active column by the finger's vertical travel since the
// previous sample, taken dt seconds ago. The strip follows the finger — dragging
// DOWN (increasing y) reveals earlier rows, a decreasing offset — so the row
// delta is the pixel travel negated and divided by the row height. It smooths the
// velocity for the eventual fling and fires OnChange as the drag crosses rows. A
// move with no armed drag, or a zero row height, is a no-op.
func (w *WheelPicker) TouchMove(ev Event, dt float64) {
	if !w.dragging || w.Disabled().Get() {
		return
	}
	rowH := w.rowHeight()
	if rowH <= 0 {
		return
	}
	c := w.columns[w.dragCol]
	dPix := float64(w.lastY - ev.Y)
	dRows := dPix / float64(rowH)
	w.lastY = ev.Y
	c.mom.DragBy(dRows)
	w.track.Sample(dRows, dt)
	w.reconcile(w.dragCol)
}

// TouchUp releases the drag, flinging the active column at the velocity the
// tracker smoothed from the recent samples (a release while stretched past the
// first/last row springs home regardless of the flick). Harmless with no drag in
// progress. The coast + snap then play out under Tick.
func (w *WheelPicker) TouchUp() {
	if !w.dragging {
		return
	}
	c := w.columns[w.dragCol]
	c.mom.EndDrag(w.track.Velocity())
	w.dragging = false
}

// Tick advances every column by dt seconds — driving both a live coast and the
// spring-to-detent snap that follows it — firing OnChange for any column whose
// selected index changed, and reports whether ANY column still owes motion so a
// host knows to schedule another frame. A non-positive dt or an all-rest wheel is
// effectively a no-op.
func (w *WheelPicker) Tick(dt float64) bool {
	moving := false
	for j, c := range w.columns {
		if c.tick(dt) {
			moving = true
		}
		w.reconcile(j)
	}
	return moving
}

// Settling reports whether any column still owes motion — a live coast or spring
// (Momentum.Settling), OR a column resting BETWEEN two rows that has yet to snap
// onto one. The second case matters because a release with a tiny velocity comes
// to rest immediately inside the momentum engine, so without it a host would stop
// ticking and leave the strip stranded mid-row; reporting the pending snap keeps
// the host calling Tick until every column sits exactly on a detent. A column
// under an active finger drag is excluded — the drag, not a snap, owns it — so a
// stray Tick during a drag never yanks the strip onto a row under the finger.
func (w *WheelPicker) Settling() bool {
	for _, c := range w.columns {
		if c.mom.Settling() {
			return true
		}
	}
	if w.dragging {
		return false
	}
	for _, c := range w.columns {
		if c.offsetRows() != c.nearestDetent() {
			return true
		}
	}
	return false
}

// reconcile recomputes a column's selected index from its offset and fires
// OnChange (with the exact column and new index) when it changed. It is the one
// place the index is published, so every path — drag, fling, snap, keyboard,
// wheel, tap — reports changes identically.
func (w *WheelPicker) reconcile(col int) {
	c := w.columns[col]
	idx := c.indexAt()
	if idx != c.index {
		c.index = idx
		if w.OnChange != nil {
			w.OnChange(col, idx)
		}
	}
}

// --- Programmatic + discrete selection -------------------------------------

// SelectedIndex is the selected row index of a column, or -1 for an out-of-range
// column so a caller can tell "no such column" from "row 0".
func (w *WheelPicker) SelectedIndex(col int) int {
	if col < 0 || col >= len(w.columns) {
		return -1
	}
	return w.columns[col].index
}

// SelectedValue is the selected value string of a column, or "" for an
// out-of-range or empty column.
func (w *WheelPicker) SelectedValue(col int) string {
	if col < 0 || col >= len(w.columns) {
		return ""
	}
	c := w.columns[col]
	if c.index < 0 || c.index >= c.count() {
		return ""
	}
	return c.items[c.index]
}

// Focus is the column the keyboard currently acts on.
func (w *WheelPicker) Focus() int { return w.focus }

// SetFocus points the keyboard at a column, ignoring an out-of-range request so
// the focus never lands on a non-existent column.
func (w *WheelPicker) SetFocus(col int) {
	if col >= 0 && col < len(w.columns) {
		w.focus = col
	}
}

// SetIndex jumps a column to select idx, WITHOUT any fling or snap animation:
// the offset is set straight onto the detent and all motion halts, so a
// programmatic or keyboard selection is instantaneous. idx is clamped into
// range; OnChange fires if the index actually changed. An out-of-range or empty
// column is a no-op.
func (w *WheelPicker) SetIndex(col, idx int) {
	if col < 0 || col >= len(w.columns) {
		return
	}
	c := w.columns[col]
	if c.count() == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= c.count() {
		idx = c.count() - 1
	}
	c.snapping = false
	c.mom.SetBounds(0, c.maxOffset())
	c.mom.SetOffset(float64(idx))
	w.reconcile(col)
}

// Step nudges the focused column's selection by delta rows (typically +1 / -1),
// clamped at the ends — the keyboard / wheel primitive. It routes through
// SetIndex, so it too is instantaneous and fires OnChange on a real change.
func (w *WheelPicker) Step(delta int) {
	if w.focus < 0 || w.focus >= len(w.columns) {
		return
	}
	w.SetIndex(w.focus, w.columns[w.focus].index+delta)
}

// --- OnEvent: discrete (clock-free) interactions ---------------------------

// OnEvent handles the pointer/keyboard interactions that need no elapsed time:
// a mouse wheel notch (EventScroll) steps the column under the pointer; a tap
// (EventClick) above or below the band steps the tapped column toward the tapped
// row; and keyboard arrows move the selection or the focus. The inertial finger
// path is NOT here — it needs per-sample dt — and lives in TouchDown/TouchMove/
// TouchUp/Tick instead. A disabled wheel ignores everything.
func (w *WheelPicker) OnEvent(ev Event) {
	if w.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventScroll:
		w.onScroll(ev)
	case EventClick:
		w.onClick(ev)
	case EventKeyDown:
		w.onKey(ev)
	}
}

// onScroll steps the column under the pointer by the wheel Delta (in rows),
// focusing it so a following key press continues on the same column.
func (w *WheelPicker) onScroll(ev Event) {
	col := w.columnAt(ev.X)
	if col < 0 {
		return
	}
	w.focus = col
	w.SetIndex(col, w.columns[col].index+ev.Delta)
}

// onClick steps the tapped column toward the tapped row: a tap on the centre
// band does nothing, a tap one row above steps up by one, two rows above by two,
// and symmetrically below — the discrete "nudge toward what I pointed at"
// affordance. The row delta is the tap's distance from the band centre in whole
// rows.
func (w *WheelPicker) onClick(ev Event) {
	col := w.columnAt(ev.X)
	if col < 0 {
		return
	}
	rowH := w.rowHeight()
	if rowH <= 0 {
		return
	}
	r := w.Bounds()
	centreY := r.H / 2
	delta := int(math.Round(float64(ev.Y-centreY) / float64(rowH)))
	if delta == 0 {
		return
	}
	w.focus = col
	w.SetIndex(col, w.columns[col].index+delta)
}

// onKey drives keyboard control WITHOUT any fling: Arrow Up/Down change the
// focused column's selection by one row; Arrow Left/Right move focus between
// columns; Home/End jump the focused column to its first/last row. Every step is
// instantaneous (SetIndex), so the keyboard is a precise alternative to the spin.
func (w *WheelPicker) onKey(ev Event) {
	switch ev.Code {
	case "ArrowUp":
		w.Step(-1)
	case "ArrowDown":
		w.Step(1)
	case "ArrowLeft":
		w.SetFocus(w.focus - 1)
	case "ArrowRight":
		w.SetFocus(w.focus + 1)
	case "Home":
		if w.focus >= 0 && w.focus < len(w.columns) {
			w.SetIndex(w.focus, 0)
		}
	case "End":
		if w.focus >= 0 && w.focus < len(w.columns) {
			w.SetIndex(w.focus, w.columns[w.focus].count()-1)
		}
	}
}

// --- Drawing ---------------------------------------------------------------

// Draw paints the wheel: a framed surface, then each column's strip of values
// scrolled to its offset and clipped to the column, with the row nearest the band
// inked in full OnSurface and the rest muted so the selection reads at a glance,
// and finally the centred selection band drawn as two Accent rules across the
// whole width. A disabled wheel paints a muted face.
func (w *WheelPicker) Draw(p painter.Painter, theme *Theme) {
	r := w.Bounds()
	surface := theme.Surface
	if w.Disabled().Get() {
		surface = mutedFace(theme)
	}
	fillRect(p, r.X, r.Y, r.W, r.H, surface)

	rowH := w.rowHeight()
	for j := range w.columns {
		w.drawColumn(p, theme, j, rowH)
	}

	// Centred selection band: two horizontal rules bracketing the centre row,
	// spanning every column so the band reads as one continuous selector.
	bandTop := r.Y + r.H/2 - rowH/2
	bandBottom := bandTop + rowH
	rule := theme.Accent
	if w.Disabled().Get() {
		rule = mutedInk(theme)
	}
	fillRect(p, r.X, bandTop, r.W, strokeWidth(), rule)
	fillRect(p, r.X, bandBottom, r.W, strokeWidth(), rule)

	// Column separators (skip the outer edges).
	if n := len(w.columns); n > 1 {
		for j := 1; j < n; j++ {
			cx := r.X + j*r.W/n
			fillRect(p, cx, r.Y, strokeWidth(), r.H, theme.Border)
		}
	}

	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}

// columnRect is the absolute rectangle column j occupies (an even share of the
// widget width, the last column absorbing any rounding remainder).
func (w *WheelPicker) columnRect(j int) Rect {
	r := w.Bounds()
	n := len(w.columns)
	x0 := r.X + j*r.W/n
	x1 := r.X + (j+1)*r.W/n
	return Rect{X: x0, Y: r.Y, W: x1 - x0, H: r.H}
}

// drawColumn paints one column's visible rows, clipped to the column rect. Rows
// are positioned by their signed row-distance from the offset, so the selected
// row sits on the band centre and its neighbours fan out above and below.
func (w *WheelPicker) drawColumn(p painter.Painter, theme *Theme, j, rowH int) {
	c := w.columns[j]
	cr := w.columnRect(j)
	centreY := cr.Y + cr.H/2
	off := c.offsetRows()
	sel := c.indexAt()

	// Widen the drawn window one row beyond the visible band so a partly-scrolled
	// row entering from either edge is painted.
	half := w.visibleRows()/2 + 1
	lo := sel - half
	hi := sel + half

	ink := theme.OnSurface
	muted := blendRGBA(theme.OnSurface, theme.Surface, 0.55)
	if w.Disabled().Get() {
		ink = mutedInk(theme)
		muted = ink
	}

	withClip(p, cr, func() {
		for i := lo; i <= hi; i++ {
			if i < 0 || i >= c.count() {
				continue
			}
			rowCentre := centreY + int(math.Round((float64(i)-off)*float64(rowH)))
			ty := rowCentre - w.glyphHeight()/2
			s := c.items[i]
			tx := cr.X + (cr.W-w.textWidth(s))/2
			col := muted
			if i == sel {
				col = ink
			}
			w.drawText(p, tx, ty, s, col)
		}
	})
}

// --- Accessibility ---------------------------------------------------------

// A11y reports the wheel as a group whose Value is the selected value of every
// column joined by a space (e.g. "09 30" for a time wheel), so a screen reader
// announces the whole current selection. It mirrors how [TimePicker] reports its
// composite value as a group.
func (w *WheelPicker) A11y() A11yInfo {
	parts := make([]string, 0, len(w.columns))
	for j := range w.columns {
		parts = append(parts, w.SelectedValue(j))
	}
	return A11yInfo{Role: RoleGroup, Value: strings.Join(parts, " ")}
}

// Compile-time check that WheelPicker is Accessible (kept with the widget so the
// shared a11y.go registry needs no edit).
var _ Accessible = (*WheelPicker)(nil)

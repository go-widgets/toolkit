// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

// ts is a fixed, host-formatted timestamp — the widget never reads the clock, so
// tests hand it a constant and stay deterministic.
const ts = "12:00:00"

// --- history: Append / Len / Clear / MaxEntries eviction -----------------

func TestLogViewAppendLenClear(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	if lv.Len() != 0 {
		t.Fatalf("fresh LogView Len = %d, want 0", lv.Len())
	}
	lv.Append(ts, LogInfo, "one")
	lv.Append(ts, LogWarn, "two")
	if lv.Len() != 2 {
		t.Fatalf("Len after 2 appends = %d, want 2", lv.Len())
	}
	lv.Clear()
	if lv.Len() != 0 {
		t.Fatalf("Len after Clear = %d, want 0", lv.Len())
	}
	if lv.sv.OffsetX().Get() != 0 || lv.sv.OffsetY().Get() != 0 {
		t.Fatalf("Clear left offset (%d,%d), want (0,0)", lv.sv.OffsetX().Get(), lv.sv.OffsetY().Get())
	}
}

// Eviction drops the OLDEST entries once the count exceeds a positive MaxEntries.
func TestLogViewMaxEntriesEviction(t *testing.T) {
	lv := NewLogView()
	lv.MaxEntries = 3
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	for i := 0; i < 5; i++ { // "0".."4"; last three survive
		lv.Append(ts, LogInfo, string(rune('0'+i)))
	}
	if lv.Len() != 3 {
		t.Fatalf("Len = %d, want 3 (bounded)", lv.Len())
	}
	if got := lv.entries[0].lines[0]; got != "2" {
		t.Fatalf("oldest surviving entry = %q, want %q (eviction dropped the wrong end)", got, "2")
	}
	if got := lv.entries[2].lines[0]; got != "4" {
		t.Fatalf("newest entry = %q, want %q", got, "4")
	}
}

// A non-positive MaxEntries is unbounded: nothing is ever evicted.
func TestLogViewUnboundedByDefault(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	for i := 0; i < 50; i++ {
		lv.Append(ts, LogInfo, "x")
	}
	if lv.Len() != 50 {
		t.Fatalf("unbounded Len = %d, want 50", lv.Len())
	}
}

// --- pixel: timestamp column + level colouring ---------------------------

// A timestamp is drawn (dim) at the start of the row, and the message follows in
// the level's ink, in a distinct column to its right.
func TestLogViewTimestampColumnAndInfoInk(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lv.Append(ts, LogInfo, "hello")

	buf := makeSurface(w, h)
	lv.Draw(newP(buf, w), theme)

	pad := scaled(logPadX)
	tsW := lv.textWidth(ts)
	msgX := lv.rows[0].msgX
	rowH := lv.rowHeight()

	// Dim timestamp ink appears in the timestamp column [pad, pad+tsW).
	tsCol := Rect{X: pad, Y: 0, W: tsW, H: rowH}
	if !scanFor(buf, w, tsCol, dimInk(theme)) {
		t.Fatal("no dim timestamp ink in the timestamp column")
	}
	// The neutral message ink (OnSurface) appears in the message column, and NOT
	// back in the timestamp column (the two columns are distinct).
	msgCol := Rect{X: msgX, Y: 0, W: msgX, H: rowH}
	if !scanFor(buf, w, msgCol, theme.OnSurface) {
		t.Fatal("no Info (OnSurface) message ink in the message column")
	}
	if scanFor(buf, w, tsCol, theme.OnSurface) {
		t.Fatal("message ink bled into the timestamp column")
	}
}

// Warn is amber, Error is red, Debug is dim — each level tints its own row.
func TestLogViewLevelColouring(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lv.Append(ts, LogWarn, "warn")
	lv.Append(ts, LogError, "err")
	lv.Append(ts, LogDebug, "dbg")

	buf := makeSurface(w, h)
	lv.Draw(newP(buf, w), theme)

	rowH := lv.rowHeight()
	msgX := lv.rows[0].msgX
	amber := RGB(0xE0, 0xA0, 0x30)
	red := RGB(0xC0, 0x30, 0x30)
	// Row 0 (Warn) amber, row 1 (Error) red, row 2 (Debug) dim — each in its own
	// message column band.
	band := func(row int) Rect { return Rect{X: msgX, Y: row * rowH, W: w - msgX - scaled(logPadX), H: rowH} }
	if !scanFor(buf, w, band(0), amber) {
		t.Fatal("Warn row not tinted amber")
	}
	if !scanFor(buf, w, band(1), red) {
		t.Fatal("Error row not tinted red")
	}
	if !scanFor(buf, w, band(2), dimInk(theme)) {
		t.Fatal("Debug row not tinted dim")
	}
	// The amber/red do not appear on the Debug row (levels are per-row).
	if scanFor(buf, w, band(2), amber) || scanFor(buf, w, band(2), red) {
		t.Fatal("a warn/error tint leaked onto the Debug row")
	}
}

// levelInk resolves each level and falls back to the neutral ink for an
// out-of-range value.
func TestLevelInk(t *testing.T) {
	theme := DefaultLight()
	if got := levelInk(LogInfo, theme); got != theme.OnSurface {
		t.Fatalf("Info ink = %+v, want OnSurface", got)
	}
	if got := levelInk(LogDebug, theme); got != dimInk(theme) {
		t.Fatalf("Debug ink = %+v, want dimInk", got)
	}
	if got := levelInk(LogWarn, theme); got != RGB(0xE0, 0xA0, 0x30) {
		t.Fatalf("Warn ink = %+v, want amber", got)
	}
	if got := levelInk(LogError, theme); got != RGB(0xC0, 0x30, 0x30) {
		t.Fatalf("Error ink = %+v, want red", got)
	}
	if got := levelInk(LogLevel(99), theme); got != theme.OnSurface {
		t.Fatalf("out-of-range level ink = %+v, want the neutral OnSurface fallback", got)
	}
}

// --- multi-line messages -------------------------------------------------

// An embedded newline becomes its own row: the message is not clipped to a
// single line, and only the FIRST row carries the timestamp.
func TestLogViewMultiLineRows(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lv.Append(ts, LogInfo, "aaaa\nbb") // two lines, second shorter

	if len(lv.rows) != 2 {
		t.Fatalf("multi-line entry produced %d rows, want 2", len(lv.rows))
	}
	if !lv.rows[0].first || lv.rows[0].timestamp != ts {
		t.Fatal("first row must carry the timestamp")
	}
	if lv.rows[1].first || lv.rows[1].timestamp != "" {
		t.Fatal("continuation row must NOT carry a timestamp")
	}

	buf := makeSurface(w, h)
	lv.Draw(newP(buf, w), theme)
	rowH := lv.rowHeight()
	msgX := lv.rows[0].msgX
	// The continuation line's ink is painted on the SECOND row (not clipped away).
	row2 := Rect{X: msgX, Y: rowH, W: w - msgX - scaled(logPadX), H: rowH}
	if !scanFor(buf, w, row2, theme.OnSurface) {
		t.Fatal("continuation line was clipped — no ink on the second row")
	}
	// No timestamp dim ink in the second row's timestamp column.
	tsCol2 := Rect{X: scaled(logPadX), Y: rowH, W: lv.textWidth(ts), H: rowH}
	if scanFor(buf, w, tsCol2, dimInk(theme)) {
		t.Fatal("a timestamp was drawn on the continuation row")
	}
}

// --- auto-scroll: follow only at the bottom ------------------------------

// While pinned to the bottom, every Append follows the newest entry.
func TestLogViewAutoScrollFollowsAtBottom(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44}) // ~4 rows visible
	for i := 0; i < 12; i++ {
		lv.Append(ts, LogInfo, "line")
	}
	if lv.sv.maxOffsetY() == 0 {
		t.Fatal("history did not overflow the viewport — test can't prove following")
	}
	if lv.sv.OffsetY().Get() != lv.sv.maxOffsetY() {
		t.Fatalf("at-bottom Append did not follow: OffsetY=%d, max=%d", lv.sv.OffsetY().Get(), lv.sv.maxOffsetY())
	}
}

// A user scrolled up (held scroll) is NOT yanked down by an incoming entry.
func TestLogViewHeldScrollNotFollowed(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	for i := 0; i < 12; i++ {
		lv.Append(ts, LogInfo, "line")
	}
	// Scroll to the very top (a big negative wheel delta, clamped at 0).
	lv.OnEvent(Event{Kind: EventScroll, Delta: -100})
	if lv.sv.OffsetY().Get() != 0 {
		t.Fatalf("expected scroll to top, OffsetY=%d", lv.sv.OffsetY().Get())
	}
	lv.Append(ts, LogError, "late line") // must NOT follow
	if lv.sv.OffsetY().Get() != 0 {
		t.Fatalf("held scroll was yanked down: OffsetY=%d, want 0", lv.sv.OffsetY().Get())
	}
	if lv.sv.maxOffsetY() == 0 {
		t.Fatal("history should still overflow after the extra entry")
	}
}

// --- wheel / drag / horizontal scrolling forwarded to the viewport -------

func TestLogViewWheelScroll(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	for i := 0; i < 12; i++ {
		lv.Append(ts, LogInfo, "line")
	}
	bottom := lv.sv.OffsetY().Get() // pinned at max
	lv.OnEvent(Event{Kind: EventScroll, Delta: -2})
	if lv.sv.OffsetY().Get() >= bottom {
		t.Fatalf("wheel up did not scroll: OffsetY=%d, was %d", lv.sv.OffsetY().Get(), bottom)
	}
	lv.OnEvent(Event{Kind: EventScroll, Delta: 10}) // back down, clamps at max
	if lv.sv.OffsetY().Get() != bottom {
		t.Fatalf("wheel down did not return to bottom: OffsetY=%d, want %d", lv.sv.OffsetY().Get(), bottom)
	}
}

// A content-drag gesture is forwarded to the viewport's pan engine.
func TestLogViewDragScroll(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	for i := 0; i < 12; i++ {
		lv.Append(ts, LogInfo, "line")
	}
	lv.OnEvent(Event{Kind: EventScroll, Delta: -100}) // to the top first
	lv.OnEvent(Event{Kind: EventClick, X: 40, Y: 20})
	lv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 4}) // drag up → scroll down
	lv.OnEvent(Event{Kind: EventMouseUp, X: 40, Y: 4})
	if lv.sv.OffsetY().Get() < 0 || lv.sv.OffsetY().Get() > lv.sv.maxOffsetY() {
		t.Fatalf("drag left OffsetY out of range: %d (max %d)", lv.sv.OffsetY().Get(), lv.sv.maxOffsetY())
	}
}

// A wide line scrolls horizontally via the wheel's DeltaX.
func TestLogViewHorizontalScroll(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 44})
	lv.Append(ts, LogInfo, strings.Repeat("x", 120)) // far wider than the viewport
	lv.OnEvent(Event{Kind: EventScroll, DeltaX: 3})
	if lv.sv.OffsetX().Get() <= 0 {
		t.Fatalf("horizontal wheel did not scroll: OffsetX=%d", lv.sv.OffsetX().Get())
	}
}

// --- windowed rendering + empty log --------------------------------------

// Scrolled to the top of a tall history, Draw paints the first window of rows
// (the last-clamp branch is NOT taken); an empty log paints nothing.
func TestLogViewWindowedAndEmptyDraw(t *testing.T) {
	const w, h = 200, 44
	theme := DefaultLight()
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	for i := 0; i < 30; i++ {
		lv.Append(ts, LogInfo, "line")
	}
	lv.OnEvent(Event{Kind: EventScroll, Delta: -100}) // top: first window, last < len
	buf := makeSurface(w, h)
	lv.Draw(newP(buf, w), theme)
	if !scanFor(buf, w, Rect{X: 0, Y: 0, W: w, H: h}, theme.OnSurface) {
		t.Fatal("top window painted no rows")
	}

	// Empty log: Draw must not panic and paints only the Surface + border.
	lv.Clear()
	lv.Draw(newP(makeSurface(w, h), w), theme)
	if lv.Len() != 0 {
		t.Fatalf("Len after clear = %d, want 0", lv.Len())
	}
}

// --- accessibility -------------------------------------------------------

// A LogView reports the ARIA log role with a live entry count.
func TestLogViewA11y(t *testing.T) {
	lv := NewLogView()
	lv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 44})
	if got := lv.A11y(); got.Role != RoleLog || got.Value != "0 entries" {
		t.Fatalf("empty A11y = %+v, want role=log value=\"0 entries\"", got)
	}
	lv.Append(ts, LogInfo, "one")
	lv.Append(ts, LogError, "two")
	if got := lv.A11y(); got.Value != "2 entries" {
		t.Fatalf("A11y value = %q, want \"2 entries\"", got.Value)
	}
	// It is announced through the shared collector (not silently skipped).
	infos := CollectA11y([]Widget{lv})
	if len(infos) != 1 || infos[0].Role != RoleLog {
		t.Fatalf("CollectA11y = %+v, want one log entry", infos)
	}
}

// --- bare &LogView{} (lazy init) -----------------------------------------

// A zero-value LogView (no constructor) is usable: the first call wires the
// viewport lazily.
func TestLogViewBareStruct(t *testing.T) {
	const w, h = 120, 33
	theme := DefaultLight()
	var lv LogView
	lv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lv.Append(ts, LogInfo, "bare")
	if lv.Len() != 1 {
		t.Fatalf("bare LogView Len = %d, want 1", lv.Len())
	}
	lv.Draw(newP(makeSurface(w, h), w), theme)
	// HitTest comes from Base and covers the bounds.
	if !lv.HitTest(10, 10) {
		t.Fatal("HitTest should hit inside bounds")
	}
}

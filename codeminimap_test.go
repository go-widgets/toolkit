// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Distinct token colours for the pixel assertions — none collides with the
// 0xC8 surface sentinel or with any DefaultLight theme tone.
var (
	cmRed     = RGB(0xF0, 0x00, 0x00)
	cmGreen   = RGB(0x00, 0xF0, 0x00)
	cmBlue    = RGB(0x00, 0x00, 0xF0)
	cmMagenta = RGB(0xF0, 0x00, 0xF0)
)

// TestCodeMinimapMultiColourSegmentsAndTopAnchored proves the overview paints
// one coloured segment per token run (multi-colour rows, not one solid bar),
// leaves indentation as a gap, and — crucially — anchors the block to the top
// WITHOUT stretching a short buffer to fill the column: the area below the last
// row stays the plain SurfaceAlt panel.
func TestCodeMinimapMultiColourSegmentsAndTopAnchored(t *testing.T) {
	const w, h = 40, 60
	theme := DefaultLight()

	m := NewCodeMinimap()
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	// line0 "ab cd": [ab]=red gap [cd]=green. line1 "  ef": indent then [ef]=blue.
	// line2 "gh": ADJACENT runs [g]=red[h]=magenta (a colour change with no space),
	// so the inner run splits on colour, not only on whitespace.
	m.Update(
		[]string{"ab cd", "  ef", "gh"},
		[][]TextSpan{
			{{Start: 0, End: 2, Color: cmRed}, {Start: 3, End: 5, Color: cmGreen}},
			{{Start: 2, End: 4, Color: cmBlue}},
			{{Start: 0, End: 1, Color: cmRed}, {Start: 1, End: 2, Color: cmMagenta}},
		},
		0, 0, // no viewport band in this test
	)

	// Introspection: 2 (row0) + 1 (row1) + 2 (row2) = 5 token segments.
	if got := m.segmentCount(theme.OnSurface); got != 5 {
		t.Fatalf("segmentCount = %d, want 5 (multi-token rows)", got)
	}

	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), theme)

	// Row 0 (y 0..1): red at x2..3, green at x5..6 (indent gap between them).
	if !scanHasColor(buf, w, 2, 0, 3, 1, cmRed) {
		t.Fatal("row0 red segment [ab] not painted")
	}
	if !scanHasColor(buf, w, 5, 0, 6, 1, cmGreen) {
		t.Fatal("row0 green segment [cd] not painted")
	}
	// The single-pixel indentation gap between the runs (x4) stays unpainted
	// panel — proving the two runs are distinct segments, not one bar.
	if pixelAt(buf, w, 4, 0) != theme.SurfaceAlt {
		t.Fatalf("gap between runs at (4,0) = %+v, want SurfaceAlt", pixelAt(buf, w, 4, 0))
	}
	// Row 1 (y 3..4): blue at x4..5, leading indentation (x2..3) left blank.
	if !scanHasColor(buf, w, 4, 3, 5, 4, cmBlue) {
		t.Fatal("row1 blue segment [ef] not painted")
	}
	if pixelAt(buf, w, 2, 3) != theme.SurfaceAlt {
		t.Fatalf("indentation gap at (2,3) = %+v, want SurfaceAlt", pixelAt(buf, w, 2, 3))
	}
	// Row 2 (y 6..7): adjacent red (x2) then magenta (x3).
	if pixelAt(buf, w, 2, 6) != cmRed {
		t.Fatalf("row2 (2,6) = %+v, want red", pixelAt(buf, w, 2, 6))
	}
	if pixelAt(buf, w, 3, 6) != cmMagenta {
		t.Fatalf("row2 (3,6) = %+v, want magenta", pixelAt(buf, w, 3, 6))
	}

	// Left divider is Border.
	if pixelAt(buf, w, 0, 30) != theme.Border {
		t.Fatalf("left divider at (0,30) = %+v, want Border", pixelAt(buf, w, 0, 30))
	}
	// Top-anchored: the 3-line block occupies y 0..7; everything below is the
	// plain panel, NOT stretched segments. (10,40) is deep in the blank region.
	if pixelAt(buf, w, 10, 40) != theme.SurfaceAlt {
		t.Fatalf("blank area at (10,40) = %+v, want SurfaceAlt (buffer must NOT stretch)", pixelAt(buf, w, 10, 40))
	}
	for _, c := range []RGBA{cmRed, cmGreen, cmBlue, cmMagenta} {
		if scanHasColor(buf, w, 1, 9, w-1, h-1, c) {
			t.Fatalf("token colour %+v painted below the block — short buffer wrongly stretched", c)
		}
	}
}

// TestCodeMinimapEmptyAndZeroArea covers the degenerate inputs: an empty buffer
// paints only the panel + divider (no segments, no band), and zero-area bounds
// short-circuit before any painting.
func TestCodeMinimapEmptyAndZeroArea(t *testing.T) {
	const w, h = 40, 60
	theme := DefaultLight()

	// Empty buffer.
	empty := &CodeMinimap{}
	empty.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	if got := empty.segmentCount(theme.OnSurface); got != 0 {
		t.Fatalf("empty segmentCount = %d, want 0", got)
	}
	if rowH, dr := empty.metrics(); rowH != 0 || dr != 0 {
		t.Fatalf("empty metrics = (%d,%d), want (0,0)", rowH, dr)
	}
	if got := empty.LineAt(30); got != 0 {
		t.Fatalf("empty LineAt = %d, want 0", got)
	}
	buf := makeSurface(w, h)
	empty.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 10, 30) != theme.SurfaceAlt {
		t.Fatalf("empty panel at (10,30) = %+v, want SurfaceAlt", pixelAt(buf, w, 10, 30))
	}
	if pixelAt(buf, w, 0, 30) != theme.Border {
		t.Fatalf("empty divider at (0,30) = %+v, want Border", pixelAt(buf, w, 0, 30))
	}

	// Zero WIDTH: Draw returns before painting; forEachSegment's r.W<=0 guard
	// also returns even though the buffer is non-empty.
	zw := &CodeMinimap{}
	zw.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 60})
	zw.Update([]string{"x"}, nil, 0, 0)
	if got := zw.segmentCount(theme.OnSurface); got != 0 {
		t.Fatalf("zero-width segmentCount = %d, want 0", got)
	}
	zbuf := makeSurface(w, h)
	zw.Draw(newP(zbuf, w), theme) // must not paint anything
	if pixelAt(zbuf, w, 0, 0) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("zero-width minimap painted despite empty area")
	}

	// Zero HEIGHT: the other half of the Draw guard.
	zh := &CodeMinimap{}
	zh.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 0})
	zh.Update([]string{"x"}, nil, 0, 0)
	zhbuf := makeSurface(w, h)
	zh.Draw(newP(zhbuf, w), theme)
	if pixelAt(zhbuf, w, 0, 0) != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
		t.Fatal("zero-height minimap painted despite empty area")
	}
}

// TestCodeMinimapViewportBand proves the translucent accent band is drawn over
// the visible range (its opaque Accent stroke is the detectable marker) and that
// both clamps fire: the minimum band height, and the bottom-edge clamp that
// keeps the band inside the drawn content.
func TestCodeMinimapViewportBand(t *testing.T) {
	theme := DefaultLight()
	tenLines := make([]string, 10)
	for i := range tenLines {
		tenLines[i] = "x"
	}

	// --- normal band: fits without either clamp -------------------------
	t.Run("normal", func(t *testing.T) {
		const w, h = 40, 60
		m := &CodeMinimap{}
		m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
		m.Update(tenLines, nil, 2, 3) // displayRows=10, rowH=3 → band y6..14
		buf := makeSurface(w, h)
		m.Draw(newP(buf, w), theme)
		if !scanHasColor(buf, w, 0, 6, w-1, 14, theme.Accent) {
			t.Fatal("viewport band stroke (Accent) not painted over the visible range")
		}
		// No band above the visible range (rows 0..1 carry only OnSurface glyphs).
		if scanHasColor(buf, w, 0, 0, w-1, 5, theme.Accent) {
			t.Fatal("Accent painted above the visible range — band misplaced")
		}
	})

	// --- min-height clamp: a 1-line viewport still shows a >=scaled(6) band --
	t.Run("min-height-clamp", func(t *testing.T) {
		const w, h = 40, 60
		m := &CodeMinimap{}
		m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
		m.Update(tenLines, nil, 0, 1) // vh would be 3; clamps up to scaled(6)=6
		buf := makeSurface(w, h)
		m.Draw(newP(buf, w), theme)
		// The clamped band's bottom stroke lands at y=5 (0+6-1); an unclamped
		// 3-px band would have its bottom stroke at y=2, so Accent at y=5 proves
		// the minimum-height clamp fired.
		if !scanHasColor(buf, w, 0, 5, w-1, 5, theme.Accent) {
			t.Fatal("min-height clamp did not extend the band to scaled(6)")
		}
	})

	// --- bottom clamp: a band near the end is trimmed to the content height --
	t.Run("bottom-clamp", func(t *testing.T) {
		const w, h = 40, 40 // surface TALLER than the 30-tall minimap
		m := &CodeMinimap{}
		m.SetBounds(Rect{X: 0, Y: 0, W: w, H: 30}) // displayRows=10, contentH=30
		m.Update(tenLines, nil, 9, 5)              // vy=27; unclamped vh=6 would reach y32
		buf := makeSurface(w, h)
		m.Draw(newP(buf, w), theme)
		// Clamped: band bottom stroke at y=29 (27+3-1), inside the content.
		if !scanHasColor(buf, w, 0, 27, w-1, 29, theme.Accent) {
			t.Fatal("band not painted at the bottom of the content")
		}
		// Unclamped it would have spilled to y=30..32 (past the 30-tall minimap);
		// assert nothing painted there.
		if scanHasColor(buf, w, 0, 30, w-1, h-1, theme.Accent) {
			t.Fatal("band spilled past the content height — bottom clamp failed")
		}
	})
}

// TestCodeMinimapOverflowCompression proves a buffer taller than the widget is
// compressed by sampling one line per row (not spilled past the bounds), and
// that the row→line sampling is proportional.
func TestCodeMinimapOverflowCompression(t *testing.T) {
	const w, h = 40, 15 // rowH=3 → maxRows=5
	theme := DefaultLight()

	lines := make([]string, 20)
	spans := make([][]TextSpan, 20)
	for i := range lines {
		lines[i] = "z"
	}
	lines[0], spans[0] = "z", []TextSpan{{Start: 0, End: 1, Color: cmRed}}   // row 0 samples line 0
	lines[4], spans[4] = "z", []TextSpan{{Start: 0, End: 1, Color: cmGreen}} // row 1 samples line 4

	m := &CodeMinimap{}
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	m.Update(lines, spans, 0, 0)

	rowH, dr := m.metrics()
	if rowH != 3 || dr != 5 {
		t.Fatalf("metrics = (%d,%d), want (3,5) — 20 lines compressed into 5 rows", rowH, dr)
	}
	// Proportional sampling: row 1 (y=3) maps to line 4.
	if got := m.LineAt(3); got != 4 {
		t.Fatalf("LineAt(3) = %d, want 4 (row1 samples line4)", got)
	}

	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), theme)
	if !scanHasColor(buf, w, 2, 0, 2, 1, cmRed) {
		t.Fatal("row0 (sampled line0) red not painted")
	}
	if !scanHasColor(buf, w, 2, 3, 2, 4, cmGreen) {
		t.Fatal("row1 (sampled line4) green not painted")
	}
	// Only 5 rows fit (y 0..14); nothing is drawn past the bounds.
	if scanHasColor(buf, w, 0, 15, w-1, h-1, cmRed) || scanHasColor(buf, w, 0, 15, w-1, h-1, cmGreen) {
		t.Fatal("compressed overview spilled past its bounds")
	}
}

// TestCodeMinimapLineAt covers the pointer→line mapping across the fixed-row
// geometry: above the block clamps to line 0, inside maps by row, below clamps
// to the last line.
func TestCodeMinimapLineAt(t *testing.T) {
	m := &CodeMinimap{}
	m.SetBounds(Rect{X: 5, Y: 10, W: 20, H: 60}) // displayRows=4, rowH=3
	m.Update([]string{"a", "b", "c", "d"}, nil, 0, 0)

	cases := []struct {
		y, want int
		note    string
	}{
		{0, 0, "y above the block clamps to line 0 (rel<0)"},
		{10, 0, "top of the block → row0 → line0"},
		{13, 1, "second row → line1"},
		{17, 2, "third row → line2"},
		{10000, 3, "y below the block clamps to the last line (row>=displayRows)"},
	}
	for _, c := range cases {
		if got := m.LineAt(c.y); got != c.want {
			t.Fatalf("LineAt(%d) = %d, want %d — %s", c.y, got, c.want, c.note)
		}
	}
}

// TestCodeMinimapMetricsMaxRowsFloor covers the maxRows<1 floor: a minimap only
// two device pixels tall still draws one row rather than none.
func TestCodeMinimapMetricsMaxRowsFloor(t *testing.T) {
	m := &CodeMinimap{}
	m.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 2}) // rowH=3 > H → maxRows floors to 1
	m.Update([]string{"a", "b"}, nil, 0, 0)
	if rowH, dr := m.metrics(); rowH != 3 || dr != 1 {
		t.Fatalf("metrics = (%d,%d), want (3,1) — maxRows floors to 1", rowH, dr)
	}
	// Zero-height bounds return (0,0).
	m.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 0})
	if rowH, dr := m.metrics(); rowH != 0 || dr != 0 {
		t.Fatalf("zero-height metrics = (%d,%d), want (0,0)", rowH, dr)
	}
}

// TestCodeMinimapNarrowClampsColumnsAndFallbackInk covers the razor-thin width
// path (column clamp + the atLeast1 floors) together with the neutral-ink
// fallback for a line with no covering span (short/nil spans slice).
func TestCodeMinimapNarrowClampsColumnsAndFallbackInk(t *testing.T) {
	const w, h = 6, 20 // usableW=atLeast1(6-4)=2 → maxCols=2
	theme := DefaultLight()

	m := &CodeMinimap{}
	m.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	// Line has 6 runes but only 2 columns fit; spans is nil so both drawn runes
	// take the neutral fallback ink and merge into ONE segment.
	m.Update([]string{"abcdef"}, nil, 0, 0)

	if got := m.segmentCount(theme.OnSurface); got != 1 {
		t.Fatalf("narrow segmentCount = %d, want 1 (clamped to 2 cols, one merged run)", got)
	}
	buf := makeSurface(w, h)
	m.Draw(newP(buf, w), theme)
	// The merged neutral run occupies x2..3 (pad=2, 2 columns), y0..1.
	if !scanHasColor(buf, w, 2, 0, 3, 1, theme.OnSurface) {
		t.Fatal("fallback-ink segment not painted for an unspanned line")
	}
}

// TestCodeMinimapOnEvent covers the click/drag → OnScrollToLine wiring, plus the
// no-op paths: a nil callback, a disabled widget, and an unrelated event kind.
func TestCodeMinimapOnEvent(t *testing.T) {
	got := -1
	m := &CodeMinimap{OnScrollToLine: func(line int) { got = line }}
	m.SetBounds(Rect{X: 0, Y: 10, W: 40, H: 60}) // displayRows=4, rowH=3
	m.Update([]string{"a", "b", "c", "d"}, nil, 0, 0)

	// Click: widget-local Y=3 re-anchors to surface y=13 → row1 → line1.
	m.OnEvent(Event{Kind: EventClick, X: 5, Y: 3})
	if got != 1 {
		t.Fatalf("click mapped to line %d, want 1", got)
	}
	// Drag: widget-local Y=6 → surface y=16 → row2 → line2.
	m.OnEvent(Event{Kind: EventMouseDrag, X: 5, Y: 6})
	if got != 2 {
		t.Fatalf("drag mapped to line %d, want 2", got)
	}
	// Unrelated event kind: no callback.
	got = -1
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got != -1 {
		t.Fatalf("non-click/drag event fired OnScrollToLine (got %d)", got)
	}
	// Disabled: consumes the click without calling back.
	m.Disabled = true
	m.OnEvent(Event{Kind: EventClick, X: 5, Y: 3})
	if got != -1 {
		t.Fatalf("disabled minimap fired OnScrollToLine (got %d)", got)
	}
	m.Disabled = false

	// Nil callback: a click must not panic.
	m.OnScrollToLine = nil
	m.OnEvent(Event{Kind: EventClick, X: 5, Y: 3})
}

// TestCodeMinimapA11y pins the accessibility description: an img whose value is
// the buffer line count (like the chart widgets).
func TestCodeMinimapA11y(t *testing.T) {
	m := &CodeMinimap{}
	if got := m.A11y(); got.Role != RoleImg || got.Value != "0 lines" {
		t.Fatalf("empty A11y = %+v, want {img, \"0 lines\"}", got)
	}
	m.Update([]string{"a", "b", "c"}, nil, 0, 0)
	if got := m.A11y(); got.Role != RoleImg || got.Value != "3 lines" {
		t.Fatalf("A11y = %+v, want {img, \"3 lines\"}", got)
	}
	// It participates in CollectA11y (not filtered out as presentational).
	if got := CollectA11y([]Widget{m}); len(got) != 1 || got[0].Value != "3 lines" {
		t.Fatalf("CollectA11y = %+v, want one img with \"3 lines\"", got)
	}
}

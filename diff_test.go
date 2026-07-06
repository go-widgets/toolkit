// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- Constructor ---------------------------------------------------------

func TestNewDiffStoresLines(t *testing.T) {
	lines := []DiffLine{
		{Text: "ctx", Kind: DiffContext},
		{Text: "add", Kind: DiffAdded},
	}
	d := NewDiff(lines)
	if len(d.Lines) != 2 || d.Lines[1].Kind != DiffAdded {
		t.Fatalf("NewDiff round-trip broken: %+v", d.Lines)
	}
}

func TestNewDiffNilBecomesEmptySlice(t *testing.T) {
	d := NewDiff(nil)
	if d.Lines == nil {
		t.Fatal("NewDiff(nil) should normalise Lines to a non-nil empty slice")
	}
	if len(d.Lines) != 0 {
		t.Fatalf("NewDiff(nil) len = %d, want 0", len(d.Lines))
	}
}

// --- Draw branches -------------------------------------------------------

// Empty Lines: only the outer body + border paint. No row fills, no
// glyph pixels.
func TestDiffDrawEmptyLines(t *testing.T) {
	const w, h = 60, 30
	theme := DefaultLight()
	d := NewDiff(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// The frame edge lands in Border.
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("empty Diff top-left border = %+v, want Border", pixelAt(buf, w, 0, 0))
	}
	// No Added / Removed tint anywhere.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == diffAddedFill {
				t.Fatalf("empty Diff painted an added-tint at (%d,%d)", x, y)
			}
			if pixelAt(buf, w, x, y) == diffRemovedFill {
				t.Fatalf("empty Diff painted a removed-tint at (%d,%d)", x, y)
			}
		}
	}
}

// Added row: the added-tint fills its row band.
func TestDiffDrawAddedLinePaintsGreenTint(t *testing.T) {
	const w, h = 80, 30
	theme := DefaultLight()
	d := NewDiff([]DiffLine{{Text: "hello", Kind: DiffAdded}})
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 30})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// Row 0 lives at y = DiffPadY .. DiffPadY+DiffLineH.
	// Sample a pixel well inside the row band, past the outer stroke.
	if pixelAt(buf, w, w-3, DiffPadY+1) != diffAddedFill {
		t.Fatalf("Added row tint = %+v, want diffAddedFill",
			pixelAt(buf, w, w-3, DiffPadY+1))
	}
}

// Removed row: the removed-tint fills its row band.
func TestDiffDrawRemovedLinePaintsRedTint(t *testing.T) {
	const w, h = 80, 30
	theme := DefaultLight()
	d := NewDiff([]DiffLine{{Text: "gone", Kind: DiffRemoved}})
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 30})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, w-3, DiffPadY+1) != diffRemovedFill {
		t.Fatalf("Removed row tint = %+v, want diffRemovedFill",
			pixelAt(buf, w, w-3, DiffPadY+1))
	}
}

// Context row: fill remains Surface.
func TestDiffDrawContextLineKeepsSurfaceFill(t *testing.T) {
	const w, h = 80, 30
	theme := DefaultLight()
	d := NewDiff([]DiffLine{{Text: "same", Kind: DiffContext}})
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 30})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// Row must be Surface, NOT any diff tint.
	got := pixelAt(buf, w, w-3, DiffPadY+1)
	if got == diffAddedFill || got == diffRemovedFill {
		t.Fatalf("context row painted a tint: %+v", got)
	}
	if got != theme.Surface {
		t.Fatalf("context row fill = %+v, want Surface", got)
	}
}

// Mixed kinds: all three tints present in the same buffer. Exercises
// every switch arm in a single Draw pass.
func TestDiffDrawMixedKinds(t *testing.T) {
	const w, h = 80, 40
	theme := DefaultLight()
	d := NewDiff([]DiffLine{
		{Text: "ctx", Kind: DiffContext},
		{Text: "add", Kind: DiffAdded},
		{Text: "del", Kind: DiffRemoved},
	})
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// Row 1 = Added.
	yAdded := DiffPadY + 1*DiffLineH + 1
	if pixelAt(buf, w, w-3, yAdded) != diffAddedFill {
		t.Fatalf("row 1 tint = %+v, want Added", pixelAt(buf, w, w-3, yAdded))
	}
	// Row 2 = Removed.
	yRemoved := DiffPadY + 2*DiffLineH + 1
	if pixelAt(buf, w, w-3, yRemoved) != diffRemovedFill {
		t.Fatalf("row 2 tint = %+v, want Removed", pixelAt(buf, w, w-3, yRemoved))
	}
}

// Prefix glyphs must land: '+', '-', ' ' at the leftmost text
// position. As of v0.9.2 the Added row uses diffAddedInk (dark green)
// and the Removed row uses diffRemovedInk (dark red) so the text
// stays readable on the fixed light-green / light-red row fills in
// dark themes too.
func TestDiffDrawPaintsPrefixGlyphs(t *testing.T) {
	const w, h = 80, 40
	theme := DefaultLight()
	d := NewDiff([]DiffLine{
		{Text: "", Kind: DiffAdded},
		{Text: "", Kind: DiffRemoved},
	})
	d.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	// Prefix column starts at DiffPadX, glyph is 5 wide. Ink per row
	// matches Draw's per-Kind ink selection.
	wants := []struct {
		label string
		ink   RGBA
	}{
		{"+ prefix", diffAddedInk},
		{"- prefix", diffRemovedInk},
	}
	for i, w2 := range wants {
		y0 := DiffPadY + i*DiffLineH
		found := false
		for y := y0; y < y0+GlyphHeight && !found; y++ {
			for x := DiffPadX; x < DiffPadX+5; x++ {
				if pixelAt(buf, w, x, y) == w2.ink {
					found = true
					break
				}
			}
		}
		if !found {
			t.Fatalf("%s glyph not painted at row %d", w2.label, i)
		}
	}
}

// Dark theme: same branches, different palette.
func TestDiffDrawDarkTheme(t *testing.T) {
	const w, h = 60, 30
	theme := DefaultDark()
	d := NewDiff([]DiffLine{{Text: "c", Kind: DiffContext}})
	d.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 30})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0) != theme.Border {
		t.Fatalf("dark border top-left = %+v, want Border", pixelAt(buf, w, 0, 0))
	}
}

// Zero-width bounds must not panic.
func TestDiffDrawZeroBounds(t *testing.T) {
	const w, h = 10, 10
	theme := DefaultLight()
	d := NewDiff([]DiffLine{{Text: "hi", Kind: DiffAdded}})
	d.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 0})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if pixelAt(buf, w, 0, 0).R != 0xC8 {
		t.Fatal("zero-bounds Diff painted pixels")
	}
}

// Default OnEvent (inherited from Base) must not panic when driven.
func TestDiffDefaultOnEventNoPanic(t *testing.T) {
	d := NewDiff(nil)
	d.OnEvent(Event{Kind: EventClick})
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

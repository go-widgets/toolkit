// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

// newTestPalette returns a CommandPalette over a 400x300 surface with four
// commands (two containing "File", plus a nil-Action variant is added by
// callers that need it). fired records each activated command's label.
func newTestPalette() (*CommandPalette, *[]string) {
	fired := &[]string{}
	cmds := []PaletteCommand{
		{Label: "Open File", Action: func() { *fired = append(*fired, "Open File") }},
		{Label: "Save File", Action: func() { *fired = append(*fired, "Save File") }},
		{Label: "Close Window", Action: func() { *fired = append(*fired, "Close Window") }},
		{Label: "Quit", Action: func() { *fired = append(*fired, "Quit") }},
	}
	cp := NewCommandPalette(cmds)
	cp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	return cp, fired
}

// --- Constructor / lifecycle --------------------------------------------

func TestNewCommandPaletteHiddenWithCommands(t *testing.T) {
	cp, _ := newTestPalette()
	if cp.Visible {
		t.Fatal("new palette should start hidden")
	}
	if len(cp.Commands) != 4 {
		t.Fatalf("commands not stored: got %d", len(cp.Commands))
	}
}

func TestOpenResetsState(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Query = "stale"
	cp.Selected = 3
	cp.Open()
	if !cp.Visible || cp.Query != "" || cp.Selected != 0 {
		t.Fatalf("Open did not reset: visible=%v query=%q selected=%d", cp.Visible, cp.Query, cp.Selected)
	}
}

func TestDismissResetsWithoutOnDismiss(t *testing.T) {
	cp, _ := newTestPalette()
	calls := 0
	cp.OnDismiss = func() { calls++ }
	cp.Open()
	cp.Query = "x"
	cp.Selected = 2
	cp.Dismiss()
	if cp.Visible || cp.Query != "" || cp.Selected != 0 {
		t.Fatalf("Dismiss did not reset: visible=%v query=%q selected=%d", cp.Visible, cp.Query, cp.Selected)
	}
	if calls != 0 {
		t.Fatalf("Dismiss must not call OnDismiss itself, got %d calls", calls)
	}
}

// --- filtered ------------------------------------------------------------

func TestFilteredSubstringCaseInsensitive(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Query = "file"
	got := cp.filtered()
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("substring filter = %v, want [0 1]", got)
	}
	cp.Query = "FILE" // case-insensitive
	if got := cp.filtered(); len(got) != 2 {
		t.Fatalf("case-insensitive filter = %v, want 2 matches", got)
	}
	cp.Query = "" // empty matches all
	if got := cp.filtered(); len(got) != 4 {
		t.Fatalf("empty query = %v, want all 4", got)
	}
	cp.Query = "zzz" // no match
	if got := cp.filtered(); len(got) != 0 {
		t.Fatalf("no-match filter = %v, want empty", got)
	}
}

// --- clampSelected -------------------------------------------------------

func TestClampSelected(t *testing.T) {
	cp, _ := newTestPalette()
	// Empty filtered list pins Selected to 0.
	cp.Query = "zzz"
	cp.Selected = 5
	cp.clampSelected()
	if cp.Selected != 0 {
		t.Fatalf("empty-list clamp: Selected=%d, want 0", cp.Selected)
	}
	// Negative clamps up to 0.
	cp.Query = ""
	cp.Selected = -3
	cp.clampSelected()
	if cp.Selected != 0 {
		t.Fatalf("negative clamp: Selected=%d, want 0", cp.Selected)
	}
	// Past-end clamps to last index of the filtered list.
	cp.Query = "file" // 2 matches
	cp.Selected = 9
	cp.clampSelected()
	if cp.Selected != 1 {
		t.Fatalf("past-end clamp: Selected=%d, want 1", cp.Selected)
	}
	// In-range is left untouched.
	cp.Selected = 0
	cp.clampSelected()
	if cp.Selected != 0 {
		t.Fatalf("in-range clamp changed Selected to %d", cp.Selected)
	}
}

// --- EventChar / Backspace ----------------------------------------------

func TestCharAppendsAndFilters(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	for _, r := range "file" {
		cp.OnEvent(Event{Kind: EventChar, Code: string(r)})
	}
	if cp.Query != "file" {
		t.Fatalf("Query = %q, want %q", cp.Query, "file")
	}
	if got := cp.filtered(); len(got) != 2 {
		t.Fatalf("after typing, filtered = %v, want 2", got)
	}
}

func TestCharEmptyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventChar, Code: ""})
	if cp.Query != "" {
		t.Fatalf("empty char changed Query to %q", cp.Query)
	}
}

func TestCharReClampsSelected(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.Selected = 3 // last of the 4 unfiltered
	cp.OnEvent(Event{Kind: EventChar, Code: "f"})
	cp.OnEvent(Event{Kind: EventChar, Code: "i"}) // "fi" -> 2 matches
	if cp.Selected != 1 {
		t.Fatalf("Selected not re-clamped after typing: got %d, want 1", cp.Selected)
	}
}

func TestBackspaceTrims(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.Query = "file"
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if cp.Query != "fil" {
		t.Fatalf("Backspace: Query=%q, want %q", cp.Query, "fil")
	}
}

func TestBackspaceOnEmptyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if cp.Query != "" {
		t.Fatalf("Backspace on empty changed Query to %q", cp.Query)
	}
}

// --- Arrow navigation ----------------------------------------------------

func TestArrowNavClampsBothEnds(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open() // empty query -> 4 filtered rows
	// Down walks 0->1->2->3 then clamps at 3.
	for i := 0; i < 6; i++ {
		cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	}
	if cp.Selected != 3 {
		t.Fatalf("ArrowDown clamp: Selected=%d, want 3", cp.Selected)
	}
	// Up walks back to 0 then clamps at 0.
	for i := 0; i < 6; i++ {
		cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if cp.Selected != 0 {
		t.Fatalf("ArrowUp clamp: Selected=%d, want 0", cp.Selected)
	}
}

func TestArrowNavEmptyFilteredNoPanic(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.Query = "zzz" // no matches
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if cp.Selected != 0 {
		t.Fatalf("empty-list nav moved Selected to %d", cp.Selected)
	}
}

func TestUnknownKeyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if !cp.Visible || cp.Query != "" || cp.Selected != 0 {
		t.Fatal("unknown key mutated palette")
	}
}

// --- Enter ---------------------------------------------------------------

func TestEnterRunsSelectedAndDismisses(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open() // all 4 shown
	cp.Selected = 1
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*fired) != 1 || (*fired)[0] != "Save File" {
		t.Fatalf("Enter fired %v, want [Save File]", *fired)
	}
	if cp.Visible || cp.Query != "" || cp.Selected != 0 {
		t.Fatalf("Enter did not dismiss/reset: visible=%v query=%q selected=%d", cp.Visible, cp.Query, cp.Selected)
	}
}

func TestEnterNilActionNoPanicStillDismisses(t *testing.T) {
	cp := NewCommandPalette([]PaletteCommand{{Label: "Placeholder", Action: nil}})
	cp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if cp.Visible {
		t.Fatal("nil-Action Enter should still dismiss")
	}
}

func TestEnterEmptyFilteredNoPanic(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	cp.Query = "zzz" // nothing matches
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*fired) != 0 {
		t.Fatalf("Enter on empty filtered fired %v", *fired)
	}
	if cp.Visible {
		t.Fatal("Enter on empty filtered should still dismiss")
	}
}

// --- Escape --------------------------------------------------------------

func TestEscapeDismissesAndCallsOnDismiss(t *testing.T) {
	cp, _ := newTestPalette()
	calls := 0
	cp.OnDismiss = func() { calls++ }
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if cp.Visible {
		t.Fatal("Escape did not dismiss")
	}
	if calls != 1 {
		t.Fatalf("Escape OnDismiss calls = %d, want 1", calls)
	}
}

func TestEscapeNilOnDismissNoPanic(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if cp.Visible {
		t.Fatal("Escape did not dismiss")
	}
}

// --- Click ---------------------------------------------------------------

// clickResultRow returns an event landing on the centre of filtered result
// row `row` (0-based within the filtered list).
func clickResultRow(cp *CommandPalette, row int) Event {
	pb := cp.panelBounds()
	y := pb.Y + palettePadY + (row+1)*PaletteRowH + PaletteRowH/2
	return Event{Kind: EventClick, X: pb.X + PalettePadX, Y: y}
}

func TestClickResultRowRunsIt(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open() // all 4 shown
	cp.OnEvent(clickResultRow(cp, 2))
	if len(*fired) != 1 || (*fired)[0] != "Close Window" {
		t.Fatalf("row click fired %v, want [Close Window]", *fired)
	}
	if cp.Visible {
		t.Fatal("row click should dismiss")
	}
}

func TestClickQueryRowIsNoOp(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	pb := cp.panelBounds()
	// Click inside the query row band.
	cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: pb.Y + palettePadY + PaletteRowH/2})
	if !cp.Visible || len(*fired) != 0 {
		t.Fatalf("query-row click: visible=%v fired=%v", cp.Visible, *fired)
	}
}

func TestClickBelowResultsIsNoOp(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	pb := cp.panelBounds()
	// Bottom padding strip: inside the panel but past the last result row.
	cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: pb.Y + pb.H - 1})
	if !cp.Visible || len(*fired) != 0 {
		t.Fatalf("below-results click: visible=%v fired=%v", cp.Visible, *fired)
	}
}

func TestClickOutsideDismissesAndCallsOnDismiss(t *testing.T) {
	cp, fired := newTestPalette()
	calls := 0
	cp.OnDismiss = func() { calls++ }
	cp.Open()
	cp.OnEvent(Event{Kind: EventClick, X: 1, Y: 1}) // corner, outside centred panel
	if cp.Visible {
		t.Fatal("outside click did not dismiss")
	}
	if calls != 1 {
		t.Fatalf("outside-click OnDismiss calls = %d, want 1", calls)
	}
	if len(*fired) != 0 {
		t.Fatalf("outside click fired an action: %v", *fired)
	}
}

func TestClickOutsideNilOnDismissNoPanic(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventClick, X: 1, Y: 1})
	if cp.Visible {
		t.Fatal("outside click did not dismiss")
	}
}

func TestEventWhileHiddenIsIgnored(t *testing.T) {
	cp, _ := newTestPalette()
	// Not opened -> hidden.
	cp.OnEvent(Event{Kind: EventChar, Code: "x"})
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if cp.Query != "" || cp.Visible {
		t.Fatalf("hidden palette reacted: query=%q visible=%v", cp.Query, cp.Visible)
	}
}

// --- Draw ----------------------------------------------------------------

func TestDrawHiddenPaintsNothing(t *testing.T) {
	cp, _ := newTestPalette()
	surf := makeSurface(400, 300)
	cp.Draw(newP(surf, 400), DefaultLight())
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 255}
	if got := pixelAt(surf, 400, 200, 150); got != sentinel {
		t.Fatalf("hidden palette painted at centre: %+v", got)
	}
}

func TestDrawVisiblePaintsPanelBorder(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	surf := makeSurface(400, 300)
	theme := DefaultLight()
	cp.Draw(newP(surf, 400), theme)
	if countInk(surf, 400, 300, theme.Border) == 0 {
		t.Fatal("visible palette drew no border")
	}
	// The Selected (row 0) result row is highlighted in Accent.
	if countInk(surf, 400, 300, theme.Accent) == 0 {
		t.Fatal("selected row not highlighted in Accent")
	}
}

func TestDrawEmptyFilteredRendersQueryRowOnly(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.Query = "zzz" // nothing matches
	surf := makeSurface(400, 300)
	theme := DefaultLight()
	cp.Draw(newP(surf, 400), theme) // must not panic
	if countInk(surf, 400, 300, theme.Border) == 0 {
		t.Fatal("empty-filtered palette still draws its panel frame")
	}
	// No result rows => no Accent highlight.
	if countInk(surf, 400, 300, theme.Accent) != 0 {
		t.Fatal("empty filtered list should not highlight any row")
	}
}

func TestDrawDarkTheme(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	surf := makeSurface(400, 300)
	theme := DefaultDark()
	cp.Draw(newP(surf, 400), theme)
	if countInk(surf, 400, 300, theme.Accent) == 0 {
		t.Fatal("dark-theme highlight missing")
	}
}

// --- panelBounds width growth -------------------------------------------

func TestPanelBoundsFloorAndGrowth(t *testing.T) {
	// Short labels + short query: width floors at PaletteMinW.
	cp, _ := newTestPalette()
	if w := cp.panelBounds().W; w != PaletteMinW {
		t.Fatalf("short palette width = %d, want floor %d", w, PaletteMinW)
	}
	// A very long query grows the width past the floor (query-width arm).
	cp.Query = strings.Repeat("w", 80)
	if w := cp.panelBounds().W; w <= PaletteMinW {
		t.Fatalf("long-query width = %d, want > %d", w, PaletteMinW)
	}
	// A very long label grows the width past the floor (label-width arm).
	wide := NewCommandPalette([]PaletteCommand{{Label: strings.Repeat("W", 80)}})
	wide.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	if w := wide.panelBounds().W; w <= PaletteMinW {
		t.Fatalf("long-label width = %d, want > %d", w, PaletteMinW)
	}
}

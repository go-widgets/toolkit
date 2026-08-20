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
	if cp.Visible().Get() {
		t.Fatal("new palette should start hidden")
	}
	if len(cp.Commands) != 4 {
		t.Fatalf("commands not stored: got %d", len(cp.Commands))
	}
}

func TestOpenResetsState(t *testing.T) {
	cp, _ := newTestPalette()
	cp.query = "stale"
	cp.selected = 3
	cp.Open()
	if !cp.Visible().Get() || cp.query != "" || cp.selected != 0 {
		t.Fatalf("Open did not reset: visible=%v query=%q selected=%d", cp.Visible().Get(), cp.query, cp.selected)
	}
}

func TestDismissResetsWithoutOnDismiss(t *testing.T) {
	cp, _ := newTestPalette()
	calls := 0
	cp.OnDismiss = func() { calls++ }
	cp.Open()
	cp.query = "x"
	cp.selected = 2
	cp.Dismiss()
	if cp.Visible().Get() || cp.query != "" || cp.selected != 0 {
		t.Fatalf("Dismiss did not reset: visible=%v query=%q selected=%d", cp.Visible().Get(), cp.query, cp.selected)
	}
	if calls != 0 {
		t.Fatalf("Dismiss must not call OnDismiss itself, got %d calls", calls)
	}
}

// --- filtered ------------------------------------------------------------

func TestFilteredSubstringCaseInsensitive(t *testing.T) {
	cp, _ := newTestPalette()
	cp.query = "file"
	got := cp.filtered()
	if len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("substring filter = %v, want [0 1]", got)
	}
	cp.query = "FILE" // case-insensitive
	if got := cp.filtered(); len(got) != 2 {
		t.Fatalf("case-insensitive filter = %v, want 2 matches", got)
	}
	cp.query = "" // empty matches all
	if got := cp.filtered(); len(got) != 4 {
		t.Fatalf("empty query = %v, want all 4", got)
	}
	cp.query = "zzz" // no match
	if got := cp.filtered(); len(got) != 0 {
		t.Fatalf("no-match filter = %v, want empty", got)
	}
}

// --- clampSelected -------------------------------------------------------

func TestClampSelected(t *testing.T) {
	cp, _ := newTestPalette()
	// Empty filtered list pins Selected to 0.
	cp.query = "zzz"
	cp.selected = 5
	cp.clampSelected()
	if cp.selected != 0 {
		t.Fatalf("empty-list clamp: Selected=%d, want 0", cp.selected)
	}
	// Negative clamps up to 0.
	cp.query = ""
	cp.selected = -3
	cp.clampSelected()
	if cp.selected != 0 {
		t.Fatalf("negative clamp: Selected=%d, want 0", cp.selected)
	}
	// Past-end clamps to last index of the filtered list.
	cp.query = "file" // 2 matches
	cp.selected = 9
	cp.clampSelected()
	if cp.selected != 1 {
		t.Fatalf("past-end clamp: Selected=%d, want 1", cp.selected)
	}
	// In-range is left untouched.
	cp.selected = 0
	cp.clampSelected()
	if cp.selected != 0 {
		t.Fatalf("in-range clamp changed Selected to %d", cp.selected)
	}
}

// --- EventChar / Backspace ----------------------------------------------

func TestCharAppendsAndFilters(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	for _, r := range "file" {
		cp.OnEvent(Event{Kind: EventChar, Code: string(r)})
	}
	if cp.query != "file" {
		t.Fatalf("Query = %q, want %q", cp.query, "file")
	}
	if got := cp.filtered(); len(got) != 2 {
		t.Fatalf("after typing, filtered = %v, want 2", got)
	}
}

func TestCharEmptyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventChar, Code: ""})
	if cp.query != "" {
		t.Fatalf("empty char changed Query to %q", cp.query)
	}
}

func TestCharReClampsSelected(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.selected = 3 // last of the 4 unfiltered
	cp.OnEvent(Event{Kind: EventChar, Code: "f"})
	cp.OnEvent(Event{Kind: EventChar, Code: "i"}) // "fi" -> 2 matches
	if cp.selected != 1 {
		t.Fatalf("Selected not re-clamped after typing: got %d, want 1", cp.selected)
	}
}

func TestBackspaceTrims(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.query = "file"
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if cp.query != "fil" {
		t.Fatalf("Backspace: Query=%q, want %q", cp.query, "fil")
	}
}

func TestBackspaceOnEmptyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if cp.query != "" {
		t.Fatalf("Backspace on empty changed Query to %q", cp.query)
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
	if cp.selected != 3 {
		t.Fatalf("ArrowDown clamp: Selected=%d, want 3", cp.selected)
	}
	// Up walks back to 0 then clamps at 0.
	for i := 0; i < 6; i++ {
		cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	}
	if cp.selected != 0 {
		t.Fatalf("ArrowUp clamp: Selected=%d, want 0", cp.selected)
	}
}

func TestArrowNavEmptyFilteredNoPanic(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.query = "zzz" // no matches
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if cp.selected != 0 {
		t.Fatalf("empty-list nav moved Selected to %d", cp.selected)
	}
}

func TestUnknownKeyIsNoOp(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if !cp.Visible().Get() || cp.query != "" || cp.selected != 0 {
		t.Fatal("unknown key mutated palette")
	}
}

// --- Enter ---------------------------------------------------------------

func TestEnterRunsSelectedAndDismisses(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open() // all 4 shown
	cp.selected = 1
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*fired) != 1 || (*fired)[0] != "Save File" {
		t.Fatalf("Enter fired %v, want [Save File]", *fired)
	}
	if cp.Visible().Get() || cp.query != "" || cp.selected != 0 {
		t.Fatalf("Enter did not dismiss/reset: visible=%v query=%q selected=%d", cp.Visible().Get(), cp.query, cp.selected)
	}
}

func TestEnterNilActionNoPanicStillDismisses(t *testing.T) {
	cp := NewCommandPalette([]PaletteCommand{{Label: "Placeholder", Action: nil}})
	cp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	cp.Open()
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if cp.Visible().Get() {
		t.Fatal("nil-Action Enter should still dismiss")
	}
}

func TestEnterEmptyFilteredNoPanic(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	cp.query = "zzz" // nothing matches
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*fired) != 0 {
		t.Fatalf("Enter on empty filtered fired %v", *fired)
	}
	if cp.Visible().Get() {
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
	if cp.Visible().Get() {
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
	if cp.Visible().Get() {
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
	if cp.Visible().Get() {
		t.Fatal("row click should dismiss")
	}
}

func TestClickQueryRowIsNoOp(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	pb := cp.panelBounds()
	// Click inside the query row band.
	cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: pb.Y + palettePadY + PaletteRowH/2})
	if !cp.Visible().Get() || len(*fired) != 0 {
		t.Fatalf("query-row click: visible=%v fired=%v", cp.Visible().Get(), *fired)
	}
}

func TestClickBelowResultsIsNoOp(t *testing.T) {
	cp, fired := newTestPalette()
	cp.Open()
	pb := cp.panelBounds()
	// Bottom padding strip: inside the panel but past the last result row.
	cp.OnEvent(Event{Kind: EventClick, X: pb.X + PalettePadX, Y: pb.Y + pb.H - 1})
	if !cp.Visible().Get() || len(*fired) != 0 {
		t.Fatalf("below-results click: visible=%v fired=%v", cp.Visible().Get(), *fired)
	}
}

func TestClickOutsideDismissesAndCallsOnDismiss(t *testing.T) {
	cp, fired := newTestPalette()
	calls := 0
	cp.OnDismiss = func() { calls++ }
	cp.Open()
	cp.OnEvent(Event{Kind: EventClick, X: 1, Y: 1}) // corner, outside centred panel
	if cp.Visible().Get() {
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
	if cp.Visible().Get() {
		t.Fatal("outside click did not dismiss")
	}
}

func TestEventWhileHiddenIsIgnored(t *testing.T) {
	cp, _ := newTestPalette()
	// Not opened -> hidden.
	cp.OnEvent(Event{Kind: EventChar, Code: "x"})
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if cp.query != "" || cp.Visible().Get() {
		t.Fatalf("hidden palette reacted: query=%q visible=%v", cp.query, cp.Visible().Get())
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
	cp.query = "zzz" // nothing matches
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
	cp.query = strings.Repeat("w", 80)
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

// --- Host-driver accessors ----------------------------------------------

// TestPaletteQueryAccessors covers Query/SetQuery: SetQuery updates the text,
// re-filters, and re-clamps a stale selection.
func TestPaletteQueryAccessors(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open()
	cp.SetSelected(3) // last of 4 unfiltered
	cp.SetQuery("file")
	if cp.Query() != "file" {
		t.Fatalf("Query() = %q, want %q", cp.Query(), "file")
	}
	if len(cp.FilteredCommands()) != 2 {
		t.Fatalf("after SetQuery filtered = %d, want 2", len(cp.FilteredCommands()))
	}
	if cp.Selected() != 1 {
		t.Fatalf("SetQuery did not re-clamp selection: got %d, want 1", cp.Selected())
	}
}

// TestPaletteSelectedAccessors covers Selected/SetSelected clamping.
func TestPaletteSelectedAccessors(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open() // 4 rows
	cp.SetSelected(2)
	if cp.Selected() != 2 {
		t.Fatalf("SetSelected(2) -> %d, want 2", cp.Selected())
	}
	cp.SetSelected(99) // past end -> clamps to 3
	if cp.Selected() != 3 {
		t.Fatalf("SetSelected(99) -> %d, want 3 (clamped)", cp.Selected())
	}
	cp.SetSelected(-5) // negative -> clamps to 0
	if cp.Selected() != 0 {
		t.Fatalf("SetSelected(-5) -> %d, want 0 (clamped)", cp.Selected())
	}
}

// TestPaletteMoveSelection covers MoveSelection in both directions with
// end-clamping (no wraparound).
func TestPaletteMoveSelection(t *testing.T) {
	cp, _ := newTestPalette()
	cp.Open() // 4 rows, selection 0
	cp.MoveSelection(2)
	if cp.Selected() != 2 {
		t.Fatalf("MoveSelection(+2) -> %d, want 2", cp.Selected())
	}
	cp.MoveSelection(10) // clamps at last row (3)
	if cp.Selected() != 3 {
		t.Fatalf("MoveSelection past end -> %d, want 3", cp.Selected())
	}
	cp.MoveSelection(-10) // clamps at 0
	if cp.Selected() != 0 {
		t.Fatalf("MoveSelection past start -> %d, want 0", cp.Selected())
	}
}

// TestPaletteFilteredCommands covers FilteredCommands under a query and empty.
func TestPaletteFilteredCommands(t *testing.T) {
	cp, _ := newTestPalette()
	all := cp.FilteredCommands()
	if len(all) != 4 {
		t.Fatalf("empty-query FilteredCommands = %d, want 4", len(all))
	}
	cp.SetQuery("file")
	got := cp.FilteredCommands()
	if len(got) != 2 || got[0].Label != "Open File" || got[1].Label != "Save File" {
		t.Fatalf("FilteredCommands = %v, want the two File commands", got)
	}
	cp.SetQuery("zzz")
	if len(cp.FilteredCommands()) != 0 {
		t.Fatal("no-match FilteredCommands should be empty")
	}
}

// TestPaletteHandleKeyDrivesDirectly proves HandleKey lets a host feed keys —
// including while the palette is not gated on Visible — running the same
// type/arrow/enter path OnEvent uses, and ignoring non-keyboard events.
func TestPaletteHandleKeyDrivesDirectly(t *testing.T) {
	fired := &[]string{}
	cmds := []PaletteCommand{
		{Label: "Open File", Action: func() { *fired = append(*fired, "Open File") }},
		{Label: "Save File", Action: func() { *fired = append(*fired, "Save File") }},
		{Label: "Quit", Action: func() { *fired = append(*fired, "Quit") }},
	}
	cp := NewCommandPalette(cmds)
	cp.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 300})
	// HandleKey works without Open() (host manages visibility).
	cp.HandleKey(Event{Kind: EventChar, Code: "f"})
	cp.HandleKey(Event{Kind: EventChar, Code: "i"}) // "fi" -> Open/Save File
	if cp.Query() != "fi" {
		t.Fatalf("HandleKey type -> Query %q, want %q", cp.Query(), "fi")
	}
	if len(cp.FilteredCommands()) != 2 {
		t.Fatalf("HandleKey filter -> %d, want 2", len(cp.FilteredCommands()))
	}
	// Empty EventChar code is a no-op.
	cp.HandleKey(Event{Kind: EventChar, Code: ""})
	if cp.Query() != "fi" {
		t.Fatalf("empty char changed Query to %q", cp.Query())
	}
	// A non-keyboard event is ignored.
	cp.HandleKey(Event{Kind: EventClick, X: 1, Y: 1})
	if cp.Query() != "fi" {
		t.Fatal("HandleKey should ignore non-keyboard events")
	}
	// Arrow moves within the filtered list; Enter activates the selection.
	cp.HandleKey(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if cp.Selected() != 1 {
		t.Fatalf("HandleKey ArrowDown -> %d, want 1", cp.Selected())
	}
	cp.HandleKey(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(*fired) != 1 || (*fired)[0] != "Save File" {
		t.Fatalf("HandleKey Enter fired %v, want [Save File]", *fired)
	}
}

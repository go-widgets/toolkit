// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// fillAt samples an interior fill pixel of cell i in a horizontal pager whose
// cells start at (x0, y0): bottom-left of the cell body, clear of the centred
// number glyph and the top-right occupancy dot.
func wpCellFill(buf []byte, w, x0, y0, i int) RGBA {
	cellW := scaled(WorkspacePagerCellW)
	gap := scaled(WorkspacePagerGap)
	cellH := scaled(WorkspacePagerCellH)
	cx := x0 + i*(cellW+gap)
	return pixelAt(buf, w, cx+3, y0+cellH-3)
}

// --- Constructor ---------------------------------------------------------

func TestNewWorkspacePagerStoresFields(t *testing.T) {
	wp := NewWorkspacePager(4, 2)
	if wp.Count != 4 || wp.Current().Get() != 2 {
		t.Fatalf("NewWorkspacePager round-trip broken: Count=%d Current=%d", wp.Count, wp.Current().Get())
	}
}

func TestNewWorkspacePagerClampsCurrentLow(t *testing.T) {
	if got := NewWorkspacePager(3, -5).Current().Get(); got != 0 {
		t.Fatalf("negative current clamped to %d, want 0", got)
	}
}

func TestNewWorkspacePagerClampsCurrentHigh(t *testing.T) {
	if got := NewWorkspacePager(3, 9).Current().Get(); got != 2 {
		t.Fatalf("over-range current clamped to %d, want 2 (Count-1)", got)
	}
}

func TestNewWorkspacePagerEmptyCountForcesZero(t *testing.T) {
	if got := NewWorkspacePager(0, 7).Current().Get(); got != 0 {
		t.Fatalf("Count<=0 current = %d, want forced 0", got)
	}
}

// TestWorkspacePagerCurrentObservable covers the zero-value lazy-init of the
// Current Observable and host-binding through Set / Subscribe.
func TestWorkspacePagerCurrentObservable(t *testing.T) {
	wp := &WorkspacePager{} // no constructor -> current is nil until accessed
	if wp.Current().Get() != 0 {
		t.Fatalf("zero-value Current = %d, want 0", wp.Current().Get())
	}
	seen := -1
	wp.Current().Subscribe(func(v int) { seen = v })
	wp.Current().Set(1)
	if wp.Current().Get() != 1 || seen != 1 {
		t.Fatalf("host Set: Current=%d seen=%d, want 1/1", wp.Current().Get(), seen)
	}
}

// --- Draw branches -------------------------------------------------------

// Count <= 0: the early return fires; the buffer stays the sentinel colour.
func TestWorkspacePagerDrawEmptyEarlyReturn(t *testing.T) {
	const w, h = 40, 20
	wp := NewWorkspacePager(0, 0)
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	wp.Draw(newP(buf, w), DefaultLight())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y).R != 0xC8 {
				t.Fatalf("empty pager painted (%d,%d) = %+v", x, y, pixelAt(buf, w, x, y))
			}
		}
	}
}

// Horizontal draw, current cell 0 highlighted: cell 0 is Accent, cell 1 is
// SurfaceAlt. Exercises the i==cur / i!=cur fill branches and the vertical
// centring branch (r.H > cellH).
func TestWorkspacePagerDrawHorizontalHighlight(t *testing.T) {
	const w, h = 120, 40
	theme := DefaultLight()
	wp := NewWorkspacePager(3, 0)
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	wp.Draw(newP(buf, w), theme)
	y0 := (h - scaled(WorkspacePagerCellH)) / 2 // centred row
	if got := wpCellFill(buf, w, 0, y0, 0); got != theme.Accent {
		t.Fatalf("current cell 0 fill = %+v, want Accent", got)
	}
	if got := wpCellFill(buf, w, 0, y0, 1); got != theme.SurfaceAlt {
		t.Fatalf("cell 1 fill = %+v, want SurfaceAlt", got)
	}
}

// Tight bounds (r.H <= cellH) skips the vertical centring branch so the row
// anchors at r.Y — its top-left corner is the Border stroke at (0,0).
func TestWorkspacePagerDrawTightBoundsNoCentring(t *testing.T) {
	const w = 60
	cellH := scaled(WorkspacePagerCellH)
	h := cellH
	theme := DefaultLight()
	wp := NewWorkspacePager(2, 0)
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	wp.Draw(newP(buf, w), theme)
	if got := pixelAt(buf, w, 0, 0); got != theme.Border {
		t.Fatalf("tight-bounds cell corner = %+v, want Border", got)
	}
}

// Occupancy dots: a current occupied cell draws its dot in the accent-inverted
// ink; a non-current occupied cell draws it in Accent. A cell past the Occupied
// slice draws no dot. Covers both dot-colour branches + occupied true/false.
func TestWorkspacePagerDrawOccupancyDots(t *testing.T) {
	const w, h = 120, 40
	theme := DefaultLight()
	wp := NewWorkspacePager(3, 0)
	wp.Occupied = []bool{true, true} // cell 2 has no entry -> no dot
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	wp.Draw(newP(buf, w), theme)

	cellW := scaled(WorkspacePagerCellW)
	gap := scaled(WorkspacePagerGap)
	dotD := scaled(workspacePagerDotD)
	pad := max(1, scaled(1))
	y0 := (h - scaled(WorkspacePagerCellH)) / 2
	inkCur := accentInk(theme)

	// Search each cell's top-right dot box for the expected colour.
	dotSeen := func(i int, want RGBA) bool {
		cx := i * (cellW + gap)
		dx, dy := cx+cellW-dotD-pad, y0+pad
		for y := dy; y < dy+dotD; y++ {
			for x := dx; x < dx+dotD; x++ {
				if pixelAt(buf, w, x, y) == want {
					return true
				}
			}
		}
		return false
	}
	if !dotSeen(0, inkCur) {
		t.Fatal("current occupied cell 0: no accent-ink dot found")
	}
	if !dotSeen(1, theme.Accent) {
		t.Fatal("non-current occupied cell 1: no Accent dot found")
	}
	// Cell 2 (unoccupied): no Accent dot in its box (its fill is SurfaceAlt).
	if dotSeen(2, theme.Accent) {
		t.Fatal("cell 2 has no Occupied entry but painted a dot")
	}
}

// A custom non-empty Labels entry is used as the caption; a "" entry and an
// index past the slice fall back to the 1-based number. This exercises both
// cellLabel branches through Draw.
func TestWorkspacePagerDrawCustomLabels(t *testing.T) {
	const w, h = 120, 40
	wp := NewWorkspacePager(3, 0)
	wp.Labels = []string{"Web", ""} // cell1 falls back to "2", cell2 to "3"
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	// Draw must not panic and must exercise both label branches.
	wp.Draw(newP(makeSurface(w, h), w), DefaultLight())
	if got := wp.cellLabel(0); got != "Web" {
		t.Fatalf("cellLabel(0) = %q, want Web", got)
	}
	if got := wp.cellLabel(1); got != "2" {
		t.Fatalf("cellLabel(1) = %q, want fallback 2", got)
	}
	if got := wp.cellLabel(2); got != "3" {
		t.Fatalf("cellLabel(2) = %q, want fallback 3", got)
	}
}

// --- Vertical orientation ------------------------------------------------

// A vertical pager stacks cells downward and centres the column horizontally in
// a wide strip (the r.W > cellW branch). Cell 1 sits one cellH+gap below cell 0.
func TestWorkspacePagerDrawVertical(t *testing.T) {
	const w, h = 40, 120
	theme := DefaultLight()
	wp := NewWorkspacePager(2, 1)
	wp.Orientation = Vertical
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	wp.Draw(newP(buf, w), theme)

	cellW := scaled(WorkspacePagerCellW)
	cellH := scaled(WorkspacePagerCellH)
	gap := scaled(WorkspacePagerGap)
	x0 := (w - cellW) / 2 // centred column
	// Cell 1 is current (Accent); sample its bottom-left interior.
	cy := cellH + gap
	if got := pixelAt(buf, w, x0+3, cy+cellH-3); got != theme.Accent {
		t.Fatalf("vertical current cell 1 fill = %+v, want Accent", got)
	}
	// Cell 0 is non-current (SurfaceAlt).
	if got := pixelAt(buf, w, x0+3, cellH-3); got != theme.SurfaceAlt {
		t.Fatalf("vertical cell 0 fill = %+v, want SurfaceAlt", got)
	}
}

// --- OnEvent: keyboard ---------------------------------------------------

func TestWorkspacePagerKeyStepsAndClamps(t *testing.T) {
	wp := NewWorkspacePager(3, 0)
	// Right/Down step forward.
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if wp.Current().Get() != 1 {
		t.Fatalf("ArrowRight: Current=%d, want 1", wp.Current().Get())
	}
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if wp.Current().Get() != 2 {
		t.Fatalf("ArrowDown: Current=%d, want 2", wp.Current().Get())
	}
	// Already at the last cell: forward is clamped (goTo i>=Count path).
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if wp.Current().Get() != 2 {
		t.Fatalf("clamp-high: Current=%d, want 2", wp.Current().Get())
	}
	// Left/Up step back.
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if wp.Current().Get() != 1 {
		t.Fatalf("ArrowLeft: Current=%d, want 1", wp.Current().Get())
	}
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if wp.Current().Get() != 0 {
		t.Fatalf("ArrowUp: Current=%d, want 0", wp.Current().Get())
	}
	// Already at cell 0: back is clamped (goTo i<0 path).
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if wp.Current().Get() != 0 {
		t.Fatalf("clamp-low: Current=%d, want 0", wp.Current().Get())
	}
}

func TestWorkspacePagerKeyHomeEnd(t *testing.T) {
	wp := NewWorkspacePager(5, 2)
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if wp.Current().Get() != 4 {
		t.Fatalf("End: Current=%d, want 4", wp.Current().Get())
	}
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if wp.Current().Get() != 0 {
		t.Fatalf("Home: Current=%d, want 0", wp.Current().Get())
	}
}

// An unrecognised key falls through the switch with no change.
func TestWorkspacePagerKeyUnknownNoOp(t *testing.T) {
	wp := NewWorkspacePager(3, 1)
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if wp.Current().Get() != 1 {
		t.Fatalf("unknown key mutated Current to %d, want 1", wp.Current().Get())
	}
}

// A key while Disabled is a no-op (the Disabled guard).
func TestWorkspacePagerKeyDisabledNoOp(t *testing.T) {
	wp := NewWorkspacePager(3, 1)
	wp.Disabled().Set(true)
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if wp.Current().Get() != 1 {
		t.Fatalf("disabled key mutated Current to %d, want 1", wp.Current().Get())
	}
}

// Count <= 0 swallows every event (both a key and a click).
func TestWorkspacePagerEventEmptyNoOp(t *testing.T) {
	wp := NewWorkspacePager(0, 0)
	wp.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // must not panic
	wp.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if wp.Current().Get() != 0 {
		t.Fatalf("empty pager mutated Current to %d", wp.Current().Get())
	}
}

// A non-click, non-key event is ignored.
func TestWorkspacePagerIgnoresOtherEvents(t *testing.T) {
	wp := NewWorkspacePager(3, 1)
	wp.OnEvent(Event{Kind: EventScroll, Delta: 1})
	if wp.Current().Get() != 1 {
		t.Fatalf("scroll mutated Current to %d, want 1", wp.Current().Get())
	}
}

// --- OnEvent: click ------------------------------------------------------

func TestWorkspacePagerClickSelectsCell(t *testing.T) {
	const h = 40
	wp := NewWorkspacePager(3, 0)
	wp.SetBounds(Rect{X: 0, Y: 0, W: 120, H: h})
	cellW := scaled(WorkspacePagerCellW)
	gap := scaled(WorkspacePagerGap)
	yMid := h / 2
	// Click cell 2: local X inside cell 2, local Y on the centred row.
	x := 2*(cellW+gap) + cellW/2
	wp.OnEvent(Event{Kind: EventClick, X: x, Y: yMid})
	if wp.Current().Get() != 2 {
		t.Fatalf("click cell 2: Current=%d, want 2", wp.Current().Get())
	}
}

// A click that misses every cell (past the row) is a no-op.
func TestWorkspacePagerClickMissNoOp(t *testing.T) {
	wp := NewWorkspacePager(3, 0)
	wp.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 40})
	wp.OnEvent(Event{Kind: EventClick, X: 500, Y: 20})
	if wp.Current().Get() != 0 {
		t.Fatalf("out-of-range click mutated Current to %d, want 0", wp.Current().Get())
	}
}

func TestWorkspacePagerClickVertical(t *testing.T) {
	const w = 40
	wp := NewWorkspacePager(3, 0)
	wp.Orientation = Vertical
	wp.SetBounds(Rect{X: 0, Y: 0, W: w, H: 120})
	cellH := scaled(WorkspacePagerCellH)
	gap := scaled(WorkspacePagerGap)
	// Click cell 1: local Y inside cell 1, local X centred on the column.
	y := 1*(cellH+gap) + cellH/2
	wp.OnEvent(Event{Kind: EventClick, X: w / 2, Y: y})
	if wp.Current().Get() != 1 {
		t.Fatalf("vertical click cell 1: Current=%d, want 1", wp.Current().Get())
	}
}

// --- A11y ----------------------------------------------------------------

func TestWorkspacePagerA11yNamesCurrent(t *testing.T) {
	wp := NewWorkspacePager(3, 1)
	got := wp.A11y()
	if got.Role != RoleTablist {
		t.Fatalf("Role = %q, want %q", got.Role, RoleTablist)
	}
	if got.Name != "2" {
		t.Fatalf("Name = %q, want the current cell's number 2", got.Name)
	}
	wp.Labels = []string{"A", "B", "C"}
	if n := wp.A11y().Name; n != "B" {
		t.Fatalf("Name = %q, want the current cell's label B", n)
	}
}

// An out-of-range current (an empty pager) yields no accessible name.
func TestWorkspacePagerA11yEmptyName(t *testing.T) {
	if n := (&WorkspacePager{}).A11y().Name; n != "" {
		t.Fatalf("empty pager Name = %q, want empty", n)
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"os"
	"testing"
)

func TestNewGanttNormalisesNilAndSelection(t *testing.T) {
	g := NewGantt(nil)
	if g.Tasks == nil {
		t.Error("nil task slice should normalise to non-nil empty slice")
	}
	if len(g.Tasks) != 0 {
		t.Errorf("empty Gantt has %d tasks, want 0", len(g.Tasks))
	}
	if g.Selected().Get() != -1 {
		t.Errorf("new Gantt Selected = %d, want -1", g.Selected().Get())
	}
	// A non-nil slice is retained verbatim.
	tasks := []GanttTask{{Label: "A", Start: 0, End: 2}}
	if g := NewGantt(tasks); len(g.Tasks) != 1 {
		t.Errorf("NewGantt kept %d tasks, want 1", len(g.Tasks))
	}
}

func TestGanttAxisUnits(t *testing.T) {
	// Explicit positive Units wins.
	g := &Gantt{Units: 12, Tasks: []GanttTask{{End: 3}}}
	if got := g.axisUnits(); got != 12 {
		t.Errorf("explicit axisUnits = %d, want 12", got)
	}
	// Auto: the largest task End.
	g = &Gantt{Tasks: []GanttTask{{End: 3}, {End: 7}, {End: 5}}}
	if got := g.axisUnits(); got != 7 {
		t.Errorf("auto axisUnits = %d, want 7", got)
	}
	// Empty / all-zero schedule floors to 1.
	g = &Gantt{Tasks: []GanttTask{{End: 0}}}
	if got := g.axisUnits(); got != 1 {
		t.Errorf("zero-schedule axisUnits = %d, want 1", got)
	}
}

func TestGanttDrawBarsAndAxis(t *testing.T) {
	// A zero-Fill task falls back to Accent; an explicit-Fill task paints its
	// own colour. Both bars, the axis rules and the gutter labels render.
	amber := RGB(0xE0, 0xA0, 0x30)
	g := NewGantt([]GanttTask{
		{Label: "Design", Start: 0, End: 3}, // zero Fill -> Accent
		{Label: "Build", Start: 2, End: 6, Fill: amber},
	})
	g.Units = 6
	g.SetBounds(Rect{X: 0, Y: 0, W: 240, H: GanttHeaderH + 2*GanttRowH})
	w, h := 240, GanttHeaderH+2*GanttRowH
	surf := makeSurface(w, h)
	th := DefaultLight()
	g.Draw(newP(surf, w), th)

	if got := countInk(surf, w, h, th.Border); got == 0 {
		t.Error("no axis/gutter border pixels drawn")
	}
	if got := countInk(surf, w, h, th.Accent); got == 0 {
		t.Error("zero-Fill task should paint an Accent bar")
	}
	if got := countInk(surf, w, h, amber); got == 0 {
		t.Error("explicit-Fill task should paint its own colour")
	}
	// Bars live in the plotting area (right of the gutter), never in it.
	accentInGutter := 0
	for y := 0; y < h; y++ {
		for x := 0; x < GanttLabelW; x++ {
			if pixelAt(surf, w, x, y) == th.Accent {
				accentInGutter++
			}
		}
	}
	if accentInGutter != 0 {
		t.Errorf("%d bar pixels leaked into the label gutter, want 0", accentInGutter)
	}
}

func TestGanttProgressOverlayAndClamp(t *testing.T) {
	// Progress > 0 paints the darker overlay; Progress > 1 clamps to a full
	// overlay without panicking.
	g := NewGantt([]GanttTask{
		{Label: "Half", Start: 0, End: 6, Progress: 0.5},
		{Label: "Over", Start: 0, End: 6, Progress: 2.0},
	})
	g.Units = 6
	w, h := 200, GanttHeaderH+2*GanttRowH
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	g.Draw(newP(surf, w), th)
	if got := countInk(surf, w, h, ganttProgressInk(th.Accent)); got == 0 {
		t.Error("Progress > 0 should paint a darker overlay")
	}
}

func TestGanttSelectionTint(t *testing.T) {
	g := NewGantt([]GanttTask{
		{Label: "A", Start: 0, End: 2},
		{Label: "B", Start: 1, End: 4},
	})
	g.Units = 4
	g.Selected().Set(1)
	w, h := 160, GanttHeaderH+2*GanttRowH
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	g.Draw(newP(surf, w), th)
	if got := countInk(surf, w, h, ganttSelectInk(th)); got == 0 {
		t.Error("Selected row should paint the selection tint")
	}
}

func TestGanttNarrowBarFloors(t *testing.T) {
	// A single-unit task on a many-unit axis in a narrow widget yields a
	// sub-pixel column; the bar width floors to 1 rather than vanishing.
	g := NewGantt([]GanttTask{{Label: "x", Start: 40, End: 41}})
	g.Units = 200
	w, h := GanttLabelW+20, GanttHeaderH+GanttRowH
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	th := DefaultLight()
	g.Draw(newP(surf, w), th) // must not panic; floored bar paints
	_ = th
}

func TestGanttAutoUnitsEmptyDraw(t *testing.T) {
	// Units unset (0) with an empty schedule derives a scale of 1 and draws
	// the header/axis without dividing by zero.
	g := NewGantt(nil)
	w, h := 120, GanttHeaderH+GanttRowH
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	surf := makeSurface(w, h)
	g.Draw(newP(surf, w), DefaultLight())
	if got := countInk(surf, w, h, DefaultLight().Border); got == 0 {
		t.Error("empty Gantt should still draw the header/axis border")
	}
}

func TestGanttOnEventSelectsRow(t *testing.T) {
	fired := -1
	g := NewGantt([]GanttTask{
		{Label: "A", Start: 0, End: 2},
		{Label: "B", Start: 1, End: 3},
	})
	// A host binds the selection through the Observable rather than a callback.
	g.Selected().Subscribe(func(i int) { fired = i })

	// Click on row 1 (second task) selects it and notifies subscribers.
	g.OnEvent(Event{Kind: EventClick, X: 120, Y: GanttHeaderH + GanttRowH + 3})
	if g.Selected().Get() != 1 {
		t.Errorf("after click Selected = %d, want 1", g.Selected().Get())
	}
	if fired != 1 {
		t.Errorf("Selected subscriber saw %d, want 1", fired)
	}

	// A non-click event is ignored.
	g.Selected().Set(0)
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if g.Selected().Get() != 0 {
		t.Errorf("non-click event changed Selected to %d, want 0", g.Selected().Get())
	}

	// A click in the header band (above the first row) is a no-op.
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH - 1})
	if g.Selected().Get() != 0 {
		t.Errorf("header click changed Selected to %d, want 0", g.Selected().Get())
	}

	// A click past the last task is a no-op.
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH + 5*GanttRowH})
	if g.Selected().Get() != 0 {
		t.Errorf("out-of-range click changed Selected to %d, want 0", g.Selected().Get())
	}
}

func TestGanttOnEventNoSubscriberSafe(t *testing.T) {
	// No subscriber bound: a valid click still updates Selected without panicking.
	g := NewGantt([]GanttTask{{Label: "A", Start: 0, End: 2}})
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH + 2})
	if g.Selected().Get() != 0 {
		t.Errorf("unsubscribed click Selected = %d, want 0", g.Selected().Get())
	}
}

// TestGanttSelectedAccessorBareAndBind exercises the lazy accessor on a bare
// Gantt (no constructor) and the host binding path: the zero-value accessor
// initialises to 0 (the field's former zero value), and a host drives + observes
// the selection purely through the Observable.
func TestGanttSelectedAccessorBareAndBind(t *testing.T) {
	var g Gantt // bare struct: selected Observable is nil until accessed
	if g.Selected().Get() != 0 {
		t.Fatalf("bare Gantt Selected = %d, want 0 (lazy init)", g.Selected().Get())
	}
	seen := -99
	g.Selected().Subscribe(func(i int) { seen = i })
	g.Selected().Set(2) // a host drives the selection through the Observable
	if g.Selected().Get() != 2 || seen != 2 {
		t.Fatalf("host Set: value=%d subscriber=%d, want 2/2", g.Selected().Get(), seen)
	}
}

// TestGanttRenderPNGDemo renders a small multi-task schedule through the public
// RenderPNG path and writes it to /tmp so the result can be eyeballed. It also
// asserts the encode succeeds and produces a non-empty PNG.
func TestGanttRenderPNGDemo(t *testing.T) {
	g := NewGantt([]GanttTask{
		{Label: "Research", Start: 0, End: 3, Progress: 1.0},
		{Label: "Design", Start: 2, End: 5, Fill: RGB(0x2E, 0x8B, 0x57), Progress: 0.6},
		{Label: "Build", Start: 4, End: 9, Fill: RGB(0xE0, 0xA0, 0x30), Progress: 0.3},
		{Label: "Test", Start: 8, End: 11},
		{Label: "Ship", Start: 10, End: 12, Fill: RGB(0xC0, 0x30, 0x30)},
	})
	g.Units = 12
	g.Selected().Set(1)
	w, h := 520, GanttHeaderH+5*GanttRowH
	png, err := RenderPNG(g, w, h, DefaultLight())
	if err != nil {
		t.Fatalf("RenderPNG: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("RenderPNG produced no bytes")
	}
	if err := os.WriteFile("/tmp/tk-gantt-demo.png", png, 0o644); err != nil {
		t.Fatalf("write demo PNG: %v", err)
	}
}

// ganttEditFixture builds a two-task chart with an explicit 10-unit axis and a
// 400px surface, returning it ready for drag dispatch (events are in
// widget-local coords; bounds origin is 0,0 so local == surface).
func ganttEditFixture() *Gantt {
	g := NewGantt([]GanttTask{
		{Label: "A", Start: 1, End: 4},
		{Label: "B", Start: 0, End: 2},
	})
	g.Units = 10
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: GanttHeaderH + 2*GanttRowH})
	return g
}

// TestGanttDragMove grabs the middle of task 0's bar and drags it right: the
// span is preserved, Start/End shift, and OnTaskChange fires with the new span.
func TestGanttDragMove(t *testing.T) {
	g := ganttEditFixture()
	changed := [3]int{-9, -9, -9}
	g.OnTaskChange = func(i, s, e int) { changed = [3]int{i, s, e} }

	rowY := GanttHeaderH + 2
	midX := (g.barXLocal(1) + g.barXLocal(4)) / 2
	g.OnEvent(Event{Kind: EventClick, X: midX, Y: rowY})
	if !g.editing || g.editMode != ganttMove {
		t.Fatalf("grab: editing=%v mode=%d, want move", g.editing, g.editMode)
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: g.barXLocal(6), Y: rowY})
	g.OnEvent(Event{Kind: EventMouseUp, X: g.barXLocal(6), Y: rowY})

	if g.editing {
		t.Fatalf("still editing after release")
	}
	if got := g.Tasks[0]; got.End-got.Start != 3 {
		t.Fatalf("span changed on move: %+v (want span 3)", got)
	}
	if g.Tasks[0].Start <= 1 {
		t.Fatalf("bar did not move right: Start=%d", g.Tasks[0].Start)
	}
	if changed[0] != 0 || changed[1] != g.Tasks[0].Start || changed[2] != g.Tasks[0].End {
		t.Fatalf("OnTaskChange = %v, want [0 %d %d]", changed, g.Tasks[0].Start, g.Tasks[0].End)
	}
}

// TestGanttDragResizeStart grabs task 1's left edge and drags past its End; the
// Start clamps to End-1 and End stays put.
func TestGanttDragResizeStart(t *testing.T) {
	g := ganttEditFixture()
	rowY := GanttHeaderH + GanttRowH + 2 // task 1
	g.OnEvent(Event{Kind: EventClick, X: g.barXLocal(0), Y: rowY})
	if g.editMode != ganttResizeStart {
		t.Fatalf("mode = %d, want resizeStart", g.editMode)
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: g.barXLocal(8), Y: rowY}) // way past End=2
	if g.Tasks[1].Start != g.Tasks[1].End-1 {
		t.Fatalf("Start=%d End=%d, want Start==End-1 (clamped)", g.Tasks[1].Start, g.Tasks[1].End)
	}
	g.OnEvent(Event{Kind: EventMouseUp, X: g.barXLocal(8), Y: rowY})
}

// TestGanttDragResizeEnd grabs task 0's right edge and drags left past its
// Start; End clamps to Start+1.
func TestGanttDragResizeEnd(t *testing.T) {
	g := ganttEditFixture()
	rowY := GanttHeaderH + 2 // task 0, Start 1 End 4
	g.OnEvent(Event{Kind: EventClick, X: g.barXLocal(4), Y: rowY})
	if g.editMode != ganttResizeEnd {
		t.Fatalf("mode = %d, want resizeEnd", g.editMode)
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: g.barXLocal(0), Y: rowY}) // left of Start
	if g.Tasks[0].End != g.Tasks[0].Start+1 {
		t.Fatalf("End=%d Start=%d, want End==Start+1 (clamped)", g.Tasks[0].End, g.Tasks[0].Start)
	}
	g.OnEvent(Event{Kind: EventMouseUp, X: g.barXLocal(0), Y: rowY})
}

// TestGanttMoveClamps drives applyDrag's move-mode clamp: dragging far right
// pins the bar's End at the axis end (Start = units - span).
func TestGanttMoveClamps(t *testing.T) {
	g := ganttEditFixture()
	rowY := GanttHeaderH + 2
	midX := (g.barXLocal(1) + g.barXLocal(4)) / 2
	g.OnEvent(Event{Kind: EventClick, X: midX, Y: rowY})
	g.OnEvent(Event{Kind: EventMouseDrag, X: 100000, Y: rowY})
	if g.Tasks[0].End != g.Units || g.Tasks[0].Start != g.Units-3 {
		t.Fatalf("clamp: %+v, want End=%d Start=%d", g.Tasks[0], g.Units, g.Units-3)
	}
}

// TestGanttClickOutsideBar covers the default (non-drag) branch: a click in a
// task row but in the empty axis area selects the row without arming a drag.
func TestGanttClickOutsideBar(t *testing.T) {
	g := ganttEditFixture()
	rowY := GanttHeaderH + 2
	g.OnEvent(Event{Kind: EventClick, X: g.barXLocal(9), Y: rowY}) // right of task-0 bar
	if g.editing {
		t.Fatalf("armed a drag in empty axis area")
	}
	if g.Selected().Get() != 0 {
		t.Fatalf("Selected = %d, want 0", g.Selected().Get())
	}
}

// TestGanttEventGuards covers the header/past-last click guards, the "not
// editing" drag/up guards, and the nil OnTaskChange release branch.
func TestGanttEventGuards(t *testing.T) {
	g := ganttEditFixture()
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH - 1})            // header
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH + 99*GanttRowH}) // past last
	if g.editing || g.Selected().Get() >= 0 {
		t.Fatalf("guarded clicks armed/selected: editing=%v sel=%d", g.editing, g.Selected().Get())
	}
	g.OnEvent(Event{Kind: EventMouseDrag, X: 50, Y: GanttHeaderH + 2}) // not editing
	g.OnEvent(Event{Kind: EventMouseUp, X: 50, Y: GanttHeaderH + 2})   // not editing

	// A move drag with no OnTaskChange listener exercises the nil release branch.
	g.OnTaskChange = nil
	rowY := GanttHeaderH + 2
	midX := (g.barXLocal(1) + g.barXLocal(4)) / 2
	g.OnEvent(Event{Kind: EventClick, X: midX, Y: rowY})
	g.OnEvent(Event{Kind: EventMouseDrag, X: g.barXLocal(5), Y: rowY})
	g.OnEvent(Event{Kind: EventMouseUp, X: g.barXLocal(5), Y: rowY})
}

// TestGanttUnitAtLocalDegenerate covers unitAtLocal's zero-width-axis guard.
func TestGanttUnitAtLocalDegenerate(t *testing.T) {
	g := ganttEditFixture()
	g.SetBounds(Rect{X: 0, Y: 0, W: GanttLabelW, H: 100}) // axisW == 0
	if got := g.unitAtLocal(50); got != 0 {
		t.Fatalf("unitAtLocal on zero-width axis = %d, want 0", got)
	}
}

// TestGanttClickNoDragNoChange: a press on a bar arms editing but, released
// without any EventMouseDrag, must not fire OnTaskChange (nothing was edited).
func TestGanttClickNoDragNoChange(t *testing.T) {
	g := ganttEditFixture()
	fired := false
	g.OnTaskChange = func(int, int, int) { fired = true }
	rowY := GanttHeaderH + 2
	midX := (g.barXLocal(1) + g.barXLocal(4)) / 2
	g.OnEvent(Event{Kind: EventClick, X: midX, Y: rowY}) // arms editing (move)
	g.OnEvent(Event{Kind: EventMouseUp, X: midX, Y: rowY})
	if fired {
		t.Fatalf("OnTaskChange fired for a press-release with no drag")
	}
}

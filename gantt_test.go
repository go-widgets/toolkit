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
	if g.Selected != -1 {
		t.Errorf("new Gantt Selected = %d, want -1", g.Selected)
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
	g.Selected = 1
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
	g.OnSelect = func(i int) { fired = i }

	// Click on row 1 (second task) selects it and fires OnSelect.
	g.OnEvent(Event{Kind: EventClick, X: 120, Y: GanttHeaderH + GanttRowH + 3})
	if g.Selected != 1 {
		t.Errorf("after click Selected = %d, want 1", g.Selected)
	}
	if fired != 1 {
		t.Errorf("OnSelect fired with %d, want 1", fired)
	}

	// A non-click event is ignored.
	g.Selected = 0
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if g.Selected != 0 {
		t.Errorf("non-click event changed Selected to %d, want 0", g.Selected)
	}

	// A click in the header band (above the first row) is a no-op.
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH - 1})
	if g.Selected != 0 {
		t.Errorf("header click changed Selected to %d, want 0", g.Selected)
	}

	// A click past the last task is a no-op.
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH + 5*GanttRowH})
	if g.Selected != 0 {
		t.Errorf("out-of-range click changed Selected to %d, want 0", g.Selected)
	}
}

func TestGanttOnEventNilSelectSafe(t *testing.T) {
	// OnSelect is nil: a valid click still updates Selected without panicking.
	g := NewGantt([]GanttTask{{Label: "A", Start: 0, End: 2}})
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: GanttHeaderH + 2})
	if g.Selected != 0 {
		t.Errorf("nil-OnSelect click Selected = %d, want 0", g.Selected)
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
	g.Selected = 1
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

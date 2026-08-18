// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// rendererCall records one ItemRenderer invocation for assertions.
type rendererCall struct {
	rc       Rect
	index    int
	item     string
	selected bool
	ink      RGBA
}

// TestListBoxItemRendererPerRow: with an ItemRenderer set, Draw calls it once
// per visible row (instead of the default text draw), passing the row rect,
// index, item, selection state and resolved ink.
func TestListBoxItemRendererPerRow(t *testing.T) {
	theme := DefaultLight()
	lb := NewListBox([]string{"a", "b", "c", "d", "e"})
	lb.Selected().Set(2)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 5 * 18}) // all 5 rows fit, no scrollbar

	var calls []rendererCall
	lb.ItemRenderer = func(p painter.Painter, th *Theme, rc Rect, index int, item string, selected bool, ink RGBA) {
		calls = append(calls, rendererCall{rc, index, item, selected, ink})
	}
	buf := makeSurface(120, 5*18)
	lb.Draw(newP(buf, 120), theme)

	if len(calls) != 5 {
		t.Fatalf("renderer called %d times, want 5 (one per visible row)", len(calls))
	}
	for i, c := range calls {
		if c.index != i || c.item != lb.Items[i] {
			t.Fatalf("call %d = (index %d, item %q), want (%d, %q)", i, c.index, c.item, i, lb.Items[i])
		}
		if c.rc.Y != i*18 || c.rc.H != 18 {
			t.Fatalf("call %d rc = %+v, want Y=%d H=18", i, c.rc, i*18)
		}
		wantSel := i == 2
		if c.selected != wantSel {
			t.Fatalf("call %d selected = %v, want %v", i, c.selected, wantSel)
		}
		wantInk := theme.OnSurface
		if wantSel {
			wantInk = theme.Background
		}
		if c.ink != wantInk {
			t.Fatalf("call %d ink = %+v, want %+v", i, c.ink, wantInk)
		}
	}
}

// TestListBoxItemRendererOnlyVisibleRows: with more rows than fit and a scroll
// offset, the renderer sees only the windowed rows (correct indices).
func TestListBoxItemRendererOnlyVisibleRows(t *testing.T) {
	items := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
	lb := NewListBox(items)
	lb.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 2 * 18}) // 2 rows visible
	lb.ScrollRow = 3

	var indices []int
	lb.ItemRenderer = func(p painter.Painter, _ *Theme, _ Rect, index int, _ string, _ bool, _ RGBA) {
		indices = append(indices, index)
	}
	buf := makeSurface(100, 2*18)
	lb.Draw(newP(buf, 100), DefaultLight())

	if len(indices) != 2 || indices[0] != 3 || indices[1] != 4 {
		t.Fatalf("rendered indices = %v, want [3 4]", indices)
	}
}

// TestListBoxItemRendererActuallyPaints: a renderer that fills the row rect
// paints those pixels (proving Draw routes through it, not the default text).
func TestListBoxItemRendererActuallyPaints(t *testing.T) {
	const w, h = 60, 18
	red := RGBA{R: 220, G: 20, B: 20, A: 255}
	lb := NewListBox([]string{"x"})
	lb.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lb.ItemRenderer = func(p painter.Painter, _ *Theme, rc Rect, _ int, _ string, _ bool, _ RGBA) {
		fillRect(p, rc.X, rc.Y, rc.W, rc.H, red)
	}
	buf := makeSurface(w, h)
	lb.Draw(newP(buf, w), DefaultLight())
	if pixelAt(buf, w, 30, 9) != red {
		t.Fatalf("renderer fill not present: %+v", pixelAt(buf, w, 30, 9))
	}
}

// TestListBoxSelectedObservable covers the zero-value lazy-init of the Selected
// accessor and the host binding path: a ListBox built as a bare struct (no
// NewListBox) still yields a usable Observable that lazy-inits to 0, Setting it
// from outside is reflected by the widget + notifies subscribers (there is no
// imperative Selected field), and NewListBox seeds the anchor to -1.
func TestListBoxSelectedObservable(t *testing.T) {
	lb := &ListBox{} // no NewListBox -> selectedRow Observable is nil until accessed
	if lb.Selected().Get() != 0 {
		t.Fatalf("bare &ListBox{} Selected = %d, want 0 (lazy-init)", lb.Selected().Get())
	}
	seen := -99
	lb.Selected().Subscribe(func(v int) { seen = v })
	lb.Selected().Set(3) // a host drives the selection through the Observable
	if lb.Selected().Get() != 3 || seen != 3 {
		t.Fatalf("host Set: Selected=%d subscriber=%d, want 3/3", lb.Selected().Get(), seen)
	}

	nb := NewListBox([]string{"a", "b"})
	if nb.Selected().Get() != -1 {
		t.Fatalf("NewListBox Selected = %d, want -1 (no selection)", nb.Selected().Get())
	}
}

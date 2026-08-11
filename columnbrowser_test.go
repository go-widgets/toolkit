// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// treeProvider is a static ColumnProvider over an in-memory tree, used to drive
// the ColumnBrowser deterministically.
type treeProvider struct {
	children map[string][]ColumnNode
	reject   map[string]bool
	preview  map[string][]string
}

func (p *treeProvider) Children(key string) ([]ColumnNode, bool) {
	if p.reject[key] {
		return nil, false
	}
	return p.children[key], true
}

func (p *treeProvider) Preview(node ColumnNode) []string { return p.preview[node.Key] }

var (
	folderC = RGB(0x22, 0x88, 0x44)
	fileC   = RGB(0x88, 0x88, 0x88)
)

// sampleTree wires a root with a folder, a couple of files, a locked (rejected)
// folder, and a deeply nested folder chain.
func sampleTree() *treeProvider {
	return &treeProvider{
		reject: map[string]bool{"locked": true},
		children: map[string][]ColumnNode{
			"root": {
				{Name: "Docs", Key: "docs", Container: true, Icon: solidIcon(folderC)},
				{Name: "readme.txt", Key: "readme", Icon: solidIcon(fileC)},
				{Name: "Locked", Key: "locked", Container: true},
				{Name: "A leaf whose name is far too long to fit its narrow column", Key: "long"},
			},
			"docs": {
				{Name: "Deep", Key: "deep", Container: true},
				{Name: "note.md", Key: "note"},
			},
			"deep": {},
		},
		preview: map[string][]string{
			"readme": {"A rather long descriptive first line exceeding the pane", "1 KB"},
		},
	}
}

func newSampleBrowser(w, h int) *ColumnBrowser {
	cv := NewColumnBrowser(sampleTree())
	cv.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	cv.SetRoot("root")
	return cv
}

func TestColumnBrowserRootAndDrillDown(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	if cv.ColumnCount() != 1 {
		t.Fatalf("initial ColumnCount = %d, want 1", cv.ColumnCount())
	}
	// Click "Docs" (col0, row0) -> opens a second column.
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10})
	if cv.ColumnCount() != 2 {
		t.Fatalf("after Docs ColumnCount = %d, want 2", cv.ColumnCount())
	}
	// Click "Deep" (col1, row0). col1 spans local X [220,440).
	cv.OnEvent(Event{Kind: EventClick, X: 270, Y: 10})
	if cv.ColumnCount() != 3 {
		t.Fatalf("after Deep ColumnCount = %d, want 3", cv.ColumnCount())
	}
}

func TestColumnBrowserRejectedChildTruncates(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // open Docs -> 2 columns
	// Click "Locked" (col0, row2, Y in [52,78)): provider rejects it, so the
	// strip truncates to just col0 and no new column opens.
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 60})
	if cv.ColumnCount() != 1 {
		t.Fatalf("after Locked ColumnCount = %d, want 1", cv.ColumnCount())
	}
}

func TestColumnBrowserSetRootRejected(t *testing.T) {
	p := sampleTree()
	p.reject["root"] = true
	cv := NewColumnBrowser(p)
	cv.SetBounds(Rect{X: 0, Y: 0, W: 700, H: 300})
	cv.SetRoot("root")
	if cv.ColumnCount() != 0 {
		t.Fatalf("rejected root ColumnCount = %d, want 0", cv.ColumnCount())
	}
}

func TestColumnBrowserLeafPreviewAndActivate(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	var activated string
	cv.OnActivate = func(n ColumnNode) { activated = n.Key }

	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // open Docs (2 columns)
	// Click "readme.txt" (col0, row1, Y in [26,52)): a leaf. Strip truncates to
	// col0 and a preview pane appears; first pick does NOT activate.
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 40})
	if cv.ColumnCount() != 1 || cv.preview == nil {
		t.Fatalf("after readme: cols=%d preview=%v", cv.ColumnCount(), cv.preview)
	}
	if activated != "" {
		t.Fatalf("first pick should not activate, got %q", activated)
	}
	// Re-pick the same leaf -> OnActivate fires.
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 40})
	if activated != "readme" {
		t.Fatalf("re-pick activated %q, want readme", activated)
	}

	// The preview pane is painted after the single column.
	th := DefaultLight()
	w, h := 700, 300
	buf := makeSurface(w, h)
	cv.Draw(newP(buf, w), th)
	if got := pixelAt(buf, w, 240, 250); got != th.SurfaceAlt {
		t.Fatalf("preview pane fill = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// The leaf's big preview icon is centred near the pane top.
	if got := pixelAt(buf, w, 330, 78); got != fileC {
		t.Fatalf("preview icon = %+v, want fileC %+v", got, fileC)
	}
}

func TestColumnBrowserActivateNilCallback(t *testing.T) {
	cv := newSampleBrowser(700, 300)                  // OnActivate nil
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 40}) // pick readme
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 40}) // re-pick: nil callback, no panic
	if cv.preview == nil {
		t.Fatalf("preview should be set")
	}
}

func TestColumnBrowserPreviewLongNameNoIconNoLines(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	// Click "long" leaf (col0, row3, Y in [78,104)): no icon, no preview lines,
	// and a name long enough to force eliding in the preview pane.
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 90})
	if cv.preview == nil || len(cv.preview.lines) != 0 {
		t.Fatalf("expected an icon-less, line-less preview")
	}
	cv.Draw(newP(makeSurface(700, 300), 700), DefaultLight()) // exercises elision
}

func TestColumnBrowserDrawRowChrome(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	th := DefaultLight()
	w, h := 700, 300
	buf := makeSurface(w, h)
	cv.Draw(newP(buf, w), th)

	// "Docs" (row0) leading folder icon at its icon cell centre.
	if got := pixelAt(buf, w, 16, 13); got != folderC {
		t.Fatalf("Docs folder icon = %+v, want %+v", got, folderC)
	}
	// The hairline separator on the first column's right edge.
	if got := pixelAt(buf, w, cbColumnWidth-1, 150); got != th.Border {
		t.Fatalf("column separator = %+v, want Border %+v", got, th.Border)
	}
}

func TestColumnBrowserHorizontalScrollAnchorsRight(t *testing.T) {
	cv := newSampleBrowser(300, 300)
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // open Docs -> 2 columns
	// With two columns (440) in a 300 viewport the strip anchors right.
	if cv.scrollX != 140 {
		t.Fatalf("scrollX after Docs = %d, want 140", cv.scrollX)
	}
	// Open "Deep" in col1: its widget-local left edge is 1*220 - scrollX = 80.
	localLeft := 1*cv.ColumnWidth - cv.scrollX
	cv.OnEvent(Event{Kind: EventClick, X: localLeft + 50, Y: 10})
	if cv.ColumnCount() != 3 {
		t.Fatalf("expected 3 columns, got %d", cv.ColumnCount())
	}
	// 3 columns * 220 = 660 content in a 300 viewport -> scrolled by 360.
	if cv.scrollX != 360 {
		t.Fatalf("scrollX = %d, want 360", cv.scrollX)
	}
	// The deepest column is laid out within the viewport (X = -360 + 2*220).
	if deep := cv.cols[2].list.Bounds(); deep.X != 80 {
		t.Fatalf("deepest column X = %d, want 80", deep.X)
	}
}

func TestColumnBrowserClickMissAndScroll(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	// Click far right, past the single 220-wide column: no column, no-op.
	cv.OnEvent(Event{Kind: EventClick, X: 500, Y: 10})
	if cv.ColumnCount() != 1 {
		t.Fatalf("miss click changed the strip: %d", cv.ColumnCount())
	}
	// A wheel event over the column is forwarded to its ListBox.
	cv.OnEvent(Event{Kind: EventScroll, X: 50, Y: 10, Delta: 1})
}

func TestColumnBrowserDisabledInert(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	cv.Disabled = true
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // would open Docs if enabled
	if cv.ColumnCount() != 1 {
		t.Fatalf("disabled browser drilled: %d columns", cv.ColumnCount())
	}
}

func TestColumnBrowserOnPickGuards(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	cv.onPick(99, 0) // column index out of range
	cv.onPick(0, 99) // row index out of range
	if cv.ColumnCount() != 1 {
		t.Fatalf("guarded onPick mutated the strip: %d", cv.ColumnCount())
	}
}

func TestColumnBrowserRelayoutZeroWidth(t *testing.T) {
	cv := NewColumnBrowser(sampleTree())
	cv.SetRoot("root")
	cv.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 100}) // zero width: relayout early-returns
	// A subsequent positive layout still works.
	cv.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 100})
	if cv.ColumnCount() != 1 {
		t.Fatalf("ColumnCount = %d, want 1", cv.ColumnCount())
	}
}

func TestColumnBrowserA11y(t *testing.T) {
	cv := newSampleBrowser(700, 300)
	if got := cv.A11y(); got.Role != RoleTree || got.Value != "" {
		t.Fatalf("fresh A11y = %+v, want tree with empty value", got)
	}
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // pick "Docs"
	if got := cv.A11y(); got.Role != RoleTree || got.Value != "Docs" {
		t.Fatalf("picked A11y = %+v, want tree/Docs", got)
	}
}

// ExampleColumnBrowser navigates a two-level tree and reports the open column
// count after drilling into a folder.
func ExampleColumnBrowser() {
	cv := NewColumnBrowser(sampleTree())
	cv.SetBounds(Rect{X: 0, Y: 0, W: 700, H: 300})
	cv.SetRoot("root")
	cv.Draw(newP(makeSurface(700, 300), 700), DefaultLight())
	cv.OnEvent(Event{Kind: EventClick, X: 50, Y: 10}) // open "Docs"
	fmt.Printf("open columns: %d\n", cv.ColumnCount())
	// Output: open columns: 2
}

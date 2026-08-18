// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestIsoMultiViewSharesModelLocalViewState proves two IsoDiagram widgets over
// ONE document stay synchronised on the model while keeping pan / zoom /
// selection strictly local to each view.
func TestIsoMultiViewSharesModelLocalViewState(t *testing.T) {
	doc := NewIsoDoc()
	v1 := NewIsoDiagram(doc)
	v2 := NewIsoDiagramView(doc)
	v1.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	v2.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	// A repaint of the second view proves it is subscribed to the shared doc.
	painted := 0
	v2.OnInvalidate = func() { painted++ }

	// Mutate through v1: v2 sees it (same document) and is invalidated.
	id := v1.commitPlace(3, 3)
	if _, ok := v2.Doc().Node(id); !ok {
		t.Fatal("a node placed via v1 is not visible through v2")
	}
	if painted == 0 {
		t.Fatal("v2 was not invalidated by a v1 edit (not subscribed to the shared doc)")
	}

	// Selection is local: v1 selected the placed node, v2's selection is empty.
	if v1.Selected() != id {
		t.Fatalf("v1 selection = %q, want %q", v1.Selected(), id)
	}
	if len(v2.Selection()) != 0 {
		t.Fatal("v1's selection leaked into v2")
	}

	// Pan is local: panning v1 leaves v2's projection put.
	before := v2.Projection().Origin
	v1.Pan(40, 25)
	if v2.Projection().Origin != before {
		t.Fatal("panning v1 moved v2's view")
	}
	if v1.Projection().Origin == before {
		t.Fatal("v1 pan did not move its own view")
	}

	// Zoom is local: zooming v2 leaves v1's tile size put.
	v1Tile := v1.Projection().TileW
	v2.ZoomAt(IsoZoomStep, 100, 100)
	if v1.Projection().TileW != v1Tile {
		t.Fatal("zooming v2 changed v1's tile size")
	}

	// Undo stacks are local too: v2 never recorded v1's placement.
	if v2.CanUndo() {
		t.Fatal("v1's edit landed on v2's undo stack")
	}
}

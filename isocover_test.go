// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestIsoSelectClearBranches covers the id=="" clear arm of every mono Select*.
func TestIsoSelectClearBranches(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.selReplace(IsoEntityRef{IsoEntityConnector, "c"})
	d.SelectConnector("")
	if d.SelectedConnector() != "" {
		t.Fatal("SelectConnector(\"\") did not clear")
	}
	d.selReplace(IsoEntityRef{IsoEntityZone, "z"})
	d.SelectZone("")
	if d.SelectedZone() != "" {
		t.Fatal("SelectZone(\"\") did not clear")
	}
	d.selReplace(IsoEntityRef{IsoEntityText, "t"})
	d.SelectText("")
	if d.SelectedText() != "" {
		t.Fatal("SelectText(\"\") did not clear")
	}
}

// TestIsoHiddenLayerRenderSkips drives Draw with a selected node and a selected
// zone on a hidden layer, plus a hidden connector and text, so every
// visibility-skip in the overlay/selection passes runs.
func TestIsoHiddenLayerRenderSkips(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 500})
	d.Doc().PutLayer(IsoLayer{ID: "H", Visible: false, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "nH", X: 2, Y: 2, Label: "hn", Layer: "H"})
	d.Doc().PutNode(IsoNode{ID: "mH", X: 4, Y: 2, Layer: "H"})
	d.Doc().PutConnector(IsoConnector{ID: "cH", From: "nH", To: "mH", Layer: "H"})
	d.Doc().PutZone(IsoZone{ID: "zH", X: 0, Y: 0, W: 2, H: 2, Label: "g", Layer: "H"})
	d.Doc().PutText(IsoText{ID: "tH", X: 6, Y: 6, Text: "x", Layer: "H"})

	// A selected node on a hidden layer: its outline pass must skip it.
	d.selReplace(IsoEntityRef{IsoEntityNode, "nH"})
	if _, err := RenderImage(d, 500, 500, theme); err != nil {
		t.Fatal(err)
	}
	// A single selected zone on a hidden layer: overlay + soleSelectedZone skip.
	d.selReplace(IsoEntityRef{IsoEntityZone, "zH"})
	if _, err := RenderImage(d, 500, 500, theme); err != nil {
		t.Fatal(err)
	}
}

// TestIsoNonPickableHitTestMisses proves a locked layer's connector, zone and
// text are not hit-tested.
func TestIsoNonPickableHitTestMisses(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 500})
	d.Doc().PutLayer(IsoLayer{ID: "L", Visible: true, Locked: true, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1, Layer: "L"})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 1, Layer: "L"})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Layer: "L"})
	d.Doc().PutZone(IsoZone{ID: "z", X: 6, Y: 6, W: 2, H: 2, Layer: "L"})
	d.Doc().PutText(IsoText{ID: "t", X: 8, Y: 1, Text: "x", Layer: "L"})

	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	ax, ay := nodeCenterLocal(d, na)
	bx, by := nodeCenterLocal(d, nb)
	if _, ok := d.connectorAtLocal((ax+bx)/2, (ay+by)/2); ok {
		t.Fatal("a locked-layer connector was hit")
	}
	zc, _ := d.Doc().Zone("z")
	zx, zy := zoneCentreLocal(d, zc)
	if _, ok := d.zoneAtLocal(zx, zy); ok {
		t.Fatal("a locked-layer zone was hit")
	}
	tt, _ := d.Doc().Text("t")
	tb := d.textBox(tt)
	if _, ok := d.textAtLocal(tb.X+tb.W/2, tb.Y+tb.H/2); ok {
		t.Fatal("a locked-layer text was hit")
	}
}

// TestIsoBlitHiddenPerFamily exercises blitHidden reporting true from a hidden
// connector and from a hidden zone (no hidden node in either case).
func TestIsoBlitHiddenPerFamily(t *testing.T) {
	// hidden connector only.
	d := NewIsoDiagram(nil)
	d.Doc().PutLayer(IsoLayer{ID: "H", Visible: false, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 2, Y: 2})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "a", To: "b", Layer: "H"})
	if !d.blitHidden() {
		t.Fatal("hidden connector did not make blitHidden true")
	}
	// hidden zone only.
	d2 := NewIsoDiagram(nil)
	d2.Doc().PutLayer(IsoLayer{ID: "H", Visible: false, Order: 1})
	d2.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d2.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 1, H: 1, Layer: "H"})
	if !d2.blitHidden() {
		t.Fatal("hidden zone did not make blitHidden true")
	}
}

// TestIsoGroupMoveViaZoneAndText covers the group-move handoff when the pressed
// member is a zone, then a text.
func TestIsoGroupMoveViaZoneAndText(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 600})
	d.Doc().PutNode(IsoNode{ID: "n", X: 6, Y: 6})
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 2, H: 2})
	d.Doc().PutText(IsoText{ID: "t", X: 9, Y: 0, Text: "x"})
	sel := func() {
		d.selReplace(
			IsoEntityRef{IsoEntityNode, "n"},
			IsoEntityRef{IsoEntityZone, "z"},
			IsoEntityRef{IsoEntityText, "t"},
		)
	}

	// Press on the zone body -> group move.
	sel()
	zc, _ := d.Doc().Zone("z")
	zx, zy := zoneCentreLocal(d, zc)
	tx, ty := groundCenterLocal(d, 1, 1) // still inside the zone quad but a new cell
	d.OnEvent(Event{Kind: EventClick, X: zx, Y: zy})
	if d.gesture != isoGestureGroupMove {
		t.Fatalf("press on a selected zone started %v, want group move", d.gesture)
	}
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty})

	// Press on the text -> group move.
	sel()
	tt, _ := d.Doc().Text("t")
	tb := d.textBox(tt)
	d.OnEvent(Event{Kind: EventClick, X: tb.X + tb.W/2, Y: tb.Y + tb.H/2})
	if d.gesture != isoGestureGroupMove {
		t.Fatalf("press on a selected text started %v, want group move", d.gesture)
	}
	d.OnEvent(Event{Kind: EventMouseUp, X: tb.X + tb.W/2, Y: tb.Y + tb.H/2})
}

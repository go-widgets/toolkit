// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- drag payload codec --------------------------------------------------

func TestIsoIconPayloadRoundTrip(t *testing.T) {
	id, ok := DecodeIsoIconPayload(EncodeIsoIconPayload("server"))
	if !ok || id != "server" {
		t.Fatalf("round-trip got %q,%v want server,true", id, ok)
	}
}

func TestIsoIconPayloadAmongMany(t *testing.T) {
	payload := JoinDropPayload([]string{"file:/tmp/x", EncodeIsoIconPayload("database"), "other"})
	id, ok := DecodeIsoIconPayload(payload)
	if !ok || id != "database" {
		t.Fatalf("multi-item decode got %q,%v want database,true", id, ok)
	}
}

func TestIsoIconPayloadRejects(t *testing.T) {
	for _, p := range []string{"", "file:/tmp/x", IsoIconDragPrefix /* empty id */} {
		if id, ok := DecodeIsoIconPayload(p); ok {
			t.Fatalf("payload %q wrongly decoded to %q", p, id)
		}
	}
}

// --- drop onto the canvas ------------------------------------------------

func TestIsoDropPlacesIconNodeAtExactTileUndoable(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	// A screen point that unprojects to cell (5,2): the drop must land exactly
	// there, carrying the dropped icon, as one undoable edit.
	x, y := groundCenterLocal(d, 5, 2)
	gx, gy := d.cellAtLocal(x, y)
	if gx != 5 || gy != 2 {
		t.Fatalf("test setup: point maps to (%d,%d), want (5,2)", gx, gy)
	}
	d.OnEvent(Event{Kind: EventDrop, X: x, Y: y, Code: EncodeIsoIconPayload("database")})

	nodes := d.Doc().Nodes()
	if len(nodes) != 1 {
		t.Fatalf("drop placed %d nodes, want 1", len(nodes))
	}
	if nodes[0].X != 5 || nodes[0].Y != 2 {
		t.Fatalf("dropped node at (%d,%d), want exact tile (5,2)", nodes[0].X, nodes[0].Y)
	}
	if nodes[0].Icon != "database" {
		t.Fatalf("dropped node carries icon %q, want database", nodes[0].Icon)
	}
	if d.Selected() != nodes[0].ID {
		t.Fatalf("dropped node not selected")
	}
	if !d.CanUndo() {
		t.Fatal("drop was not undoable")
	}
	d.Undo()
	if n := len(d.Doc().Nodes()); n != 0 {
		t.Fatalf("undo left %d nodes, want 0", n)
	}
}

func TestIsoDropIgnoresForeignPayload(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.OnEvent(Event{Kind: EventDrop, X: 100, Y: 100, Code: "file:/tmp/x"})
	if n := len(d.Doc().Nodes()); n != 0 {
		t.Fatalf("foreign drop placed %d nodes, want 0", n)
	}
	if d.CanUndo() {
		t.Fatal("foreign drop must not push an undo snapshot")
	}
}

func TestIsoDiagramAcceptsDrop(t *testing.T) {
	d := NewIsoDiagram(nil)
	if !d.AcceptsDrop(EncodeIsoIconPayload("server")) {
		t.Fatal("must accept an icon payload")
	}
	if d.AcceptsDrop("file:/tmp/x") {
		t.Fatal("must reject a foreign payload")
	}
	if d.AcceptsDrop("") {
		t.Fatal("must reject an empty payload")
	}
}

// --- click-to-place placement mode ---------------------------------------

func TestIsoPlacementIconTapPlacesIconNode(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	invalidated := 0
	d.OnInvalidate = func() { invalidated++ }
	if d.PlacementIcon() != "" {
		t.Fatal("placement starts disarmed")
	}
	d.SetPlacementIcon("cloud")
	if d.PlacementIcon() != "cloud" {
		t.Fatalf("armed icon = %q, want cloud", d.PlacementIcon())
	}
	if invalidated == 0 {
		t.Fatal("arming placement did not invalidate")
	}
	if d.PlacementIconObservable().Get() != "cloud" {
		t.Fatal("observable does not expose the armed icon")
	}

	x, y := groundCenterLocal(d, 3, 4)
	d.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	d.OnEvent(Event{Kind: EventMouseUp, X: x, Y: y})
	nodes := d.Doc().Nodes()
	if len(nodes) != 1 || nodes[0].X != 3 || nodes[0].Y != 4 || nodes[0].Icon != "cloud" {
		t.Fatalf("armed tap placed %+v, want one cloud node at (3,4)", nodes)
	}
	d.Undo()
	if n := len(d.Doc().Nodes()); n != 0 {
		t.Fatalf("undo left %d nodes, want 0", n)
	}
}

func TestIsoDisarmedTapPlacesBareNode(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.SetPlacementIcon("cloud")
	d.SetPlacementIcon("") // disarm: control-run — behaves as before the palette
	x, y := groundCenterLocal(d, 1, 1)
	d.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	d.OnEvent(Event{Kind: EventMouseUp, X: x, Y: y})
	nodes := d.Doc().Nodes()
	if len(nodes) != 1 || nodes[0].Icon != "" {
		t.Fatalf("disarmed tap placed %+v, want one bare (icon-less) node", nodes)
	}
}

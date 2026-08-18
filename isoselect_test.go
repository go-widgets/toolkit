// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-gfx/gfx/iso"
)

// zoneCentreLocal is the widget-local pixel at the ground centre of a zone's
// rectangle — a stable point inside its body for click tests.
func zoneCentreLocal(d *IsoDiagram, z IsoZone) (int, int) {
	return localOf(d, iso.V(float64(z.X)+float64(z.W)/2, float64(z.Y)+float64(z.H)/2, 0))
}

// --- selection-set primitives -------------------------------------------

func TestIsoSelectionSetPrimitives(t *testing.T) {
	d := NewIsoDiagram(nil)
	a := IsoEntityRef{Kind: IsoEntityNode, ID: "a"}
	b := IsoEntityRef{Kind: IsoEntityNode, ID: "b"}
	if len(d.Selection()) != 0 || d.IsSelected(a) {
		t.Fatal("fresh widget has a selection")
	}
	if d.SelectionList() != d.selSet {
		t.Fatal("SelectionList is not the backing observable")
	}
	// selReplace: equal is a no-op (SubscribeChanged count proves it).
	changes := 0
	d.selSet.SubscribeChanged(func() { changes++ })
	d.selReplace(a)
	d.selReplace(a) // equal -> no change
	if changes == 0 {
		t.Fatal("selReplace did not fire")
	}
	stable := changes
	d.selReplace(a)
	if changes != stable {
		t.Fatal("redundant selReplace fired a change")
	}
	// different element, same length.
	d.selReplace(b)
	if !d.IsSelected(b) || d.IsSelected(a) {
		t.Fatalf("selReplace(b) selection = %v", d.Selection())
	}
	// different length.
	d.selReplace(a, b)
	if len(d.Selection()) != 2 {
		t.Fatalf("selReplace(a,b) = %v", d.Selection())
	}
	// selAdd: present is a no-op, absent appends.
	d.selAdd(a)
	if len(d.Selection()) != 2 {
		t.Fatal("selAdd of a present ref grew the set")
	}
	c := IsoEntityRef{Kind: IsoEntityNode, ID: "c"}
	d.selAdd(c)
	if len(d.Selection()) != 3 {
		t.Fatal("selAdd of an absent ref did not append")
	}
	// selToggle removes when present, adds when absent.
	d.selToggle(c)
	if d.IsSelected(c) {
		t.Fatal("toggle did not remove present ref")
	}
	d.selToggle(c)
	if !d.IsSelected(c) {
		t.Fatal("toggle did not add absent ref")
	}
	// selRemoveKind: connectors absent -> no change; nodes present -> cleared.
	d.selRemoveKind(IsoEntityConnector)
	if len(d.Selection()) != 3 {
		t.Fatal("removing an absent kind changed the set")
	}
	d.selRemoveKind(IsoEntityNode)
	if len(d.Selection()) != 0 {
		t.Fatalf("removing nodes left %v", d.Selection())
	}
}

// --- mono compatibility accessors ---------------------------------------

func TestIsoMonoReflectsLastOfKind(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.selReplace(
		IsoEntityRef{IsoEntityNode, "n1"},
		IsoEntityRef{IsoEntityZone, "z1"},
		IsoEntityRef{IsoEntityNode, "n2"}, // later node wins the node channel
		IsoEntityRef{IsoEntityConnector, "c1"},
		IsoEntityRef{IsoEntityText, "t1"},
	)
	if d.Selected() != "n2" {
		t.Fatalf("Selected() = %q, want last node n2", d.Selected())
	}
	if d.SelectedConnector() != "c1" {
		t.Fatalf("SelectedConnector() = %q", d.SelectedConnector())
	}
	if d.SelectedZone() != "z1" {
		t.Fatalf("SelectedZone() = %q", d.SelectedZone())
	}
	if d.SelectedText() != "t1" {
		t.Fatalf("SelectedText() = %q", d.SelectedText())
	}
	// Clearing one kind empties just that channel.
	d.selRemoveKind(IsoEntityNode)
	if d.Selected() != "" {
		t.Fatalf("node channel not cleared: %q", d.Selected())
	}
	if d.SelectedZone() != "z1" {
		t.Fatal("clearing nodes disturbed the zone channel")
	}
}

func TestIsoRefreshMonoOnSelectFires(t *testing.T) {
	d := NewIsoDiagram(nil)
	var got []string
	d.OnSelect = func(id string) { got = append(got, id) }
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"})
	// Adding a non-node keeps the node channel unchanged -> no extra OnSelect.
	d.selAdd(IsoEntityRef{IsoEntityZone, "z"})
	d.selRemoveKind(IsoEntityNode)
	if len(got) != 2 || got[0] != "a" || got[1] != "" {
		t.Fatalf("OnSelect calls = %v, want [a, \"\"]", got)
	}
}

// --- additive click / toggle --------------------------------------------

func TestIsoAdditiveClickTogglesEachKind(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 500})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.Doc().PutZone(IsoZone{ID: "z", X: 6, Y: 6, W: 2, H: 2})
	d.Doc().PutText(IsoText{ID: "t", X: 8, Y: 1, Text: "x"})
	na, _ := d.Doc().Node("a")
	nb, _ := d.Doc().Node("b")
	ax, ay := nodeCenterLocal(d, na)
	bx, by := nodeCenterLocal(d, nb)
	// Ctrl-click node a, then Shift-click node b: additive, both selected.
	d.OnEvent(Event{Kind: EventClick, X: ax, Y: ay, Ctrl: true})
	d.OnEvent(Event{Kind: EventMouseUp, X: ax, Y: ay, Ctrl: true})
	d.OnEvent(Event{Kind: EventClick, X: bx, Y: by, Shift: true})
	d.OnEvent(Event{Kind: EventMouseUp, X: bx, Y: by, Shift: true})
	if !d.IsSelected(IsoEntityRef{IsoEntityNode, "a"}) || !d.IsSelected(IsoEntityRef{IsoEntityNode, "b"}) {
		t.Fatalf("additive node clicks selected %v", d.Selection())
	}
	// Ctrl-click a again toggles it off.
	d.OnEvent(Event{Kind: EventClick, X: ax, Y: ay, Ctrl: true})
	if d.IsSelected(IsoEntityRef{IsoEntityNode, "a"}) {
		t.Fatal("re-Ctrl-click did not deselect a")
	}
	// Additive-click the connector, the zone and the text too.
	mx, my := (ax+bx)/2, (ay+by)/2
	d.OnEvent(Event{Kind: EventClick, X: mx, Y: my, Ctrl: true})
	zc, _ := d.Doc().Zone("z")
	zx, zy := zoneCentreLocal(d, zc)
	d.OnEvent(Event{Kind: EventClick, X: zx, Y: zy, Ctrl: true})
	tt, _ := d.Doc().Text("t")
	tb := d.textBox(tt)
	d.OnEvent(Event{Kind: EventClick, X: tb.X + tb.W/2, Y: tb.Y + tb.H/2, Ctrl: true})
	if !d.IsSelected(IsoEntityRef{IsoEntityConnector, "ab"}) ||
		!d.IsSelected(IsoEntityRef{IsoEntityZone, "z"}) ||
		!d.IsSelected(IsoEntityRef{IsoEntityText, "t"}) {
		t.Fatalf("additive connector/zone/text clicks selected %v", d.Selection())
	}
}

// --- marquee ------------------------------------------------------------

// TestIsoMarqueeRefsExact proves the marquee selects EXACTLY the entities whose
// projected box intersects the rectangle: a broad sweep catches every pickable
// entity (skipping a hidden layer and a dangling connector), a corner sweep
// catches nothing.
func TestIsoMarqueeRefsExact(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 600})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 3, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.Doc().PutConnector(IsoConnector{ID: "dangling", From: "a", To: "ghost"}) // path !ok
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 2, H: 2})
	d.Doc().PutText(IsoText{ID: "t", X: 2, Y: 2, Text: "hi"})
	// A hidden layer with one entity of each kind — never pickable, never marqueed.
	d.Doc().PutLayer(IsoLayer{ID: "H", Visible: false, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "hn", X: 1, Y: 1, Layer: "H"})
	d.Doc().PutConnector(IsoConnector{ID: "hc", From: "a", To: "b", Layer: "H"})
	d.Doc().PutZone(IsoZone{ID: "hz", X: 0, Y: 0, W: 2, H: 2, Layer: "H"})
	d.Doc().PutText(IsoText{ID: "ht", X: 2, Y: 2, Text: "hi", Layer: "H"})

	got := map[IsoEntityRef]bool{}
	for _, r := range d.marqueeRefs(0, 0, 600, 600) {
		got[r] = true
	}
	want := []IsoEntityRef{
		{IsoEntityNode, "a"}, {IsoEntityNode, "b"},
		{IsoEntityConnector, "ab"},
		{IsoEntityZone, "z"}, {IsoEntityText, "t"},
	}
	if len(got) != len(want) {
		t.Fatalf("broad marquee selected %d, want %d: %v", len(got), len(want), d.marqueeRefs(0, 0, 600, 600))
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("broad marquee missed %+v", w)
		}
	}
	// A tiny rectangle in an empty corner selects nothing.
	if refs := d.marqueeRefs(595, 595, 599, 599); len(refs) != 0 {
		t.Fatalf("corner marquee selected %v", refs)
	}
}

// TestIsoMarqueeGesture drives the marquee through the event pipeline (a
// Shift-drag on empty ground) and paints its preview.
func TestIsoMarqueeGesture(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 500})
	d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3})
	na, _ := d.Doc().Node("a")
	nbx := d.nodeBox(na)
	// Start on empty ground with Shift; sweep a rectangle around node a.
	d.OnEvent(Event{Kind: EventClick, X: int(nbx.minX) - 20, Y: int(nbx.minY) - 20, Shift: true})
	d.OnEvent(Event{Kind: EventMouseDrag, X: int(nbx.maxX) + 20, Y: int(nbx.maxY) + 20, Shift: true})
	d.Draw(painterPixel(make([]byte, 4*500*500), 500, 500), DefaultLight()) // marquee preview branch
	d.OnEvent(Event{Kind: EventMouseUp, X: int(nbx.maxX) + 20, Y: int(nbx.maxY) + 20, Shift: true})
	if !d.IsSelected(IsoEntityRef{IsoEntityNode, "a"}) || len(d.Selection()) != 1 {
		t.Fatalf("marquee gesture selected %v", d.Selection())
	}
}

// --- group move ---------------------------------------------------------

func TestIsoGroupMove(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 600})
	d.Doc().PutNode(IsoNode{ID: "n1", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "n2", X: 5, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n1", To: "n2"})
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 2, H: 2})
	d.Doc().PutText(IsoText{ID: "t", X: 7, Y: 1, Text: "x"})
	d.selReplace(
		IsoEntityRef{IsoEntityNode, "n1"}, IsoEntityRef{IsoEntityNode, "n2"},
		IsoEntityRef{IsoEntityConnector, "c"},
		IsoEntityRef{IsoEntityZone, "z"}, IsoEntityRef{IsoEntityText, "t"},
	)
	px, py := groundCenterLocal(d, 1, 1) // press on n1's own cell
	tx, ty := groundCenterLocal(d, 3, 3) // drop two cells over on each axis
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	if d.gesture != isoGestureGroupMove {
		t.Fatalf("press on a multi-selected member started gesture %v, want group move", d.gesture)
	}
	d.OnEvent(Event{Kind: EventMouseDrag, X: tx, Y: ty})
	d.OnEvent(Event{Kind: EventMouseUp, X: tx, Y: ty}) // release at same cell -> no-op branch
	n1, _ := d.Doc().Node("n1")
	n2, _ := d.Doc().Node("n2")
	z, _ := d.Doc().Zone("z")
	tt, _ := d.Doc().Text("t")
	if n1.X != 3 || n1.Y != 3 || n2.X != 7 || n2.Y != 3 {
		t.Fatalf("nodes moved to %v %v, want (3,3) (7,3)", n1, n2)
	}
	if z.X != 2 || z.Y != 2 || tt.X != 9 || tt.Y != 3 {
		t.Fatalf("zone/text moved to %v %v, want (2,2) (9,3)", z, tt)
	}
	// One undoable op restores every position.
	d.Undo()
	n1, _ = d.Doc().Node("n1")
	z, _ = d.Doc().Zone("z")
	if n1.X != 1 || n1.Y != 1 || z.X != 0 || z.Y != 0 {
		t.Fatalf("undo left n1=%v z=%v", n1, z)
	}
}

// --- group delete / recolour / select-all -------------------------------

func TestIsoGroupDelete(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "m", X: 4, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n", To: "m"})
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 1, H: 1})
	d.Doc().PutText(IsoText{ID: "t", X: 2, Y: 2})
	// Select only n (not m) plus the connector, zone and text: deleting n also
	// cascades its connector, so the explicit connector ref exercises the
	// connector delete arm and its already-gone id is a harmless no-op.
	d.selReplace(
		IsoEntityRef{IsoEntityNode, "n"},
		IsoEntityRef{IsoEntityConnector, "c"},
		IsoEntityRef{IsoEntityZone, "z"},
		IsoEntityRef{IsoEntityText, "t"},
	)
	d.DeleteSelection()
	if len(d.Doc().Zones()) != 0 || len(d.Doc().Texts()) != 0 {
		t.Fatal("DeleteSelection left entities behind")
	}
	if _, ok := d.Doc().Node("n"); ok {
		t.Fatal("DeleteSelection left node n")
	}
	if len(d.Doc().Connectors()) != 0 {
		t.Fatal("DeleteSelection left the connector")
	}
	if len(d.Selection()) != 0 {
		t.Fatal("selection not cleared after delete")
	}
	d.Undo()
	if len(d.Doc().Zones()) != 1 || len(d.Doc().Texts()) != 1 {
		t.Fatal("undo did not restore the deleted entities")
	}
	// Empty-selection DeleteSelection is a no-op.
	d.selClear()
	before := d.CanUndo()
	d.DeleteSelection()
	if d.CanUndo() != before {
		t.Fatal("deleting an empty selection pushed an undo entry")
	}
}

func TestIsoSetSelectionColor(t *testing.T) {
	d := NewIsoDiagram(nil)
	blue := RGBA{R: 0, G: 0, B: 220, A: 255}
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "c"})
	d.Doc().PutZone(IsoZone{ID: "z", W: 1, H: 1})
	d.Doc().PutText(IsoText{ID: "t"})
	// Nothing selected -> false.
	if d.SetSelectionColor(blue) {
		t.Fatal("recolouring an empty selection returned true")
	}
	d.selReplace(
		IsoEntityRef{IsoEntityNode, "n"}, IsoEntityRef{IsoEntityConnector, "c"},
		IsoEntityRef{IsoEntityZone, "z"}, IsoEntityRef{IsoEntityText, "t"},
	)
	if !d.SetSelectionColor(blue) {
		t.Fatal("recolour of a fresh selection returned false")
	}
	n, _ := d.Doc().Node("n")
	c, _ := d.connectorByID("c")
	z, _ := d.Doc().Zone("z")
	tx, _ := d.Doc().Text("t")
	if n.Color != blue || c.Color != blue || z.Color != blue || tx.Color != blue {
		t.Fatalf("colours = %v %v %v %v", n.Color, c.Color, z.Color, tx.Color)
	}
	// Redundant recolour changes nothing -> false.
	if d.SetSelectionColor(blue) {
		t.Fatal("redundant recolour returned true")
	}
	d.Undo()
	n, _ = d.Doc().Node("n")
	if n.Color == blue {
		t.Fatal("undo did not restore the node colour")
	}
}

func TestIsoSelectAll(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "c"})
	d.Doc().PutZone(IsoZone{ID: "z", W: 1, H: 1})
	d.Doc().PutText(IsoText{ID: "t"})
	// One of every kind on a locked layer: never selected by SelectAll.
	d.Doc().PutLayer(IsoLayer{ID: "L", Visible: true, Locked: true, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "ln", X: 2, Y: 2, Layer: "L"})
	d.Doc().PutConnector(IsoConnector{ID: "lc", Layer: "L"})
	d.Doc().PutZone(IsoZone{ID: "lz", W: 1, H: 1, Layer: "L"})
	d.Doc().PutText(IsoText{ID: "lt", Layer: "L"})
	d.SelectAll()
	if len(d.Selection()) != 4 {
		t.Fatalf("SelectAll selected %d, want 4 pickable: %v", len(d.Selection()), d.Selection())
	}
	if d.IsSelected(IsoEntityRef{IsoEntityNode, "ln"}) {
		t.Fatal("SelectAll selected a locked-layer node")
	}
	// Ctrl-A drives it through the key handler.
	d.selClear()
	d.OnEvent(Event{Kind: EventKeyDown, Code: "a"}) // no Ctrl -> ignored
	if len(d.Selection()) != 0 {
		t.Fatal("plain 'a' selected")
	}
	d.OnEvent(Event{Kind: EventKeyDown, Code: "A", Ctrl: true})
	if len(d.Selection()) != 4 {
		t.Fatal("Ctrl-A did not select all")
	}
}

// --- prune / locked-layer picking ---------------------------------------

func TestIsoPruneSelectionMixed(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "m", X: 2, Y: 2})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n", To: "m"})
	d.Doc().PutZone(IsoZone{ID: "z", W: 1, H: 1})
	d.Doc().PutText(IsoText{ID: "t", X: 3, Y: 3})
	d.selReplace(
		IsoEntityRef{IsoEntityNode, "n"}, IsoEntityRef{IsoEntityConnector, "c"},
		IsoEntityRef{IsoEntityZone, "z"}, IsoEntityRef{IsoEntityText, "t"},
	)
	d.pruneSelection() // nothing stale -> no change
	if len(d.Selection()) != 4 {
		t.Fatalf("prune dropped live refs: %v", d.Selection())
	}
	d.Doc().RemoveText("t") // orphan the text ref
	d.pruneSelection()
	if len(d.Selection()) != 3 || d.IsSelected(IsoEntityRef{IsoEntityText, "t"}) {
		t.Fatalf("prune did not drop the orphaned text: %v", d.Selection())
	}
}

func TestIsoLockedLayerNotSelectable(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutLayer(IsoLayer{ID: "L", Visible: true, Locked: true, Order: 1})
	d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3, Layer: "L"})
	na, _ := d.Doc().Node("a")
	px, py := nodeCenterLocal(d, na)
	if _, ok := d.nodeAtLocal(px, py); ok {
		t.Fatal("a locked-layer node was picked")
	}
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	if d.Selected() != "" {
		t.Fatalf("clicking a locked-layer node selected %q", d.Selected())
	}
	// Unlocking makes it selectable again.
	d.SetLayerLocked("L", false)
	if _, ok := d.nodeAtLocal(px, py); !ok {
		t.Fatal("unlocked-layer node not pickable")
	}
}

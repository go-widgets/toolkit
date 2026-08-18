// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"

	stdcolor "image/color"
)

// --- IsoDoc layer store -------------------------------------------------

func TestIsoDocLayerCRUD(t *testing.T) {
	d := NewIsoDoc()
	if len(d.Layers()) != 0 {
		t.Fatal("fresh doc has layers")
	}
	if _, ok := d.Layer("x"); ok {
		t.Fatal("missing layer reported present")
	}
	d.PutLayer(IsoLayer{ID: "a", Name: "A", Visible: true, Order: 1})
	d.PutLayer(IsoLayer{ID: "b", Name: "B", Visible: true, Order: 2})
	if len(d.Layers()) != 2 {
		t.Fatalf("layers = %d, want 2", len(d.Layers()))
	}
	// Upsert replaces in place.
	d.PutLayer(IsoLayer{ID: "a", Name: "A2", Order: 5})
	got, ok := d.Layer("a")
	if !ok || got.Name != "A2" || got.Order != 5 {
		t.Fatalf("upsert layer = %+v ok=%v", got, ok)
	}
	if len(d.Layers()) != 2 {
		t.Fatalf("upsert grew the set to %d", len(d.Layers()))
	}
	// LayerList is the same backing observable.
	if d.LayerList().Len() != 2 {
		t.Fatalf("LayerList len = %d", d.LayerList().Len())
	}
	d.RemoveLayer("missing") // no-op
	if len(d.Layers()) != 2 {
		t.Fatal("removing a missing layer changed the set")
	}
	d.RemoveLayer("a")
	if _, ok := d.Layer("a"); ok {
		t.Fatal("RemoveLayer left the layer")
	}
	if len(d.Layers()) != 1 {
		t.Fatalf("after remove layers = %d, want 1", len(d.Layers()))
	}
}

func TestIsoDocLayerSubscribeFires(t *testing.T) {
	d := NewIsoDoc()
	n := 0
	un := d.Subscribe(func() { n++ })
	d.PutLayer(IsoLayer{ID: "a", Visible: true})
	if n == 0 {
		t.Fatal("layer edit did not fire the document subscription")
	}
	un()
	got := n
	d.PutLayer(IsoLayer{ID: "b", Visible: true})
	if n != got {
		t.Fatal("subscription fired after unsubscribe")
	}
}

// --- layer property resolution ------------------------------------------

func TestIsoLayerResolvers(t *testing.T) {
	d := NewIsoDiagram(nil)
	// Unknown / default layer: order 0, visible, unlocked, pickable.
	if d.layerOrder("") != 0 || !d.layerVisible("") || d.layerLocked("") || !d.pickable("") {
		t.Fatal("default layer resolvers wrong")
	}
	d.Doc().PutLayer(IsoLayer{ID: "L", Order: 3, Visible: false, Locked: true})
	if d.layerOrder("L") != 3 {
		t.Fatalf("order = %d", d.layerOrder("L"))
	}
	if d.layerVisible("L") {
		t.Fatal("hidden layer reported visible")
	}
	if !d.layerLocked("L") {
		t.Fatal("locked layer reported unlocked")
	}
	if d.pickable("L") {
		t.Fatal("hidden+locked layer reported pickable")
	}
}

// --- widget layer commands ----------------------------------------------

func TestIsoAddLayerStacksOnTop(t *testing.T) {
	d := NewIsoDiagram(nil)
	id := d.AddLayer("first")
	l, ok := d.Doc().Layer(id)
	if !ok || l.Name != "first" || !l.Visible || l.Order != 1 {
		t.Fatalf("added layer = %+v ok=%v, want order 1 visible", l, ok)
	}
	id2 := d.AddLayer("second")
	l2, _ := d.Doc().Layer(id2)
	if l2.Order != 2 {
		t.Fatalf("second layer order = %d, want 2 (one above top)", l2.Order)
	}
	if len(d.Layers()) != 2 {
		t.Fatalf("Layers() = %d, want 2", len(d.Layers()))
	}
	if !d.CanUndo() {
		t.Fatal("AddLayer should be undoable")
	}
	d.Undo()
	if _, ok := d.Doc().Layer(id2); ok {
		t.Fatal("undo did not remove the added layer")
	}
}

func TestIsoNextLayerIDSkipsCollision(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutLayer(IsoLayer{ID: "L1", Visible: true}) // occupies the first generated id
	id := d.AddLayer("x")
	if id == "L1" {
		t.Fatalf("nextLayerID collided with existing L1")
	}
}

func TestIsoDeleteLayerReassignsAndRejects(t *testing.T) {
	d := NewIsoDiagram(nil)
	if d.DeleteLayer("") {
		t.Fatal("deleting the default layer should be rejected")
	}
	if d.DeleteLayer("ghost") {
		t.Fatal("deleting a missing layer should be rejected")
	}
	id := d.AddLayer("work")
	// Entities on the layer (one of every family) plus a default-layer node that
	// must be left untouched.
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1, Layer: id})
	d.Doc().PutNode(IsoNode{ID: "keep", X: 2, Y: 2})
	d.Doc().PutNode(IsoNode{ID: "m", X: 3, Y: 3, Layer: id})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "n", To: "m", Layer: id})
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 1, H: 1, Layer: id})
	d.Doc().PutText(IsoText{ID: "t", X: 4, Y: 4, Layer: id})
	if !d.DeleteLayer(id) {
		t.Fatal("DeleteLayer returned false for a real layer")
	}
	if _, ok := d.Doc().Layer(id); ok {
		t.Fatal("layer record survived DeleteLayer")
	}
	n, _ := d.Doc().Node("n")
	c, _ := d.connectorByID("c")
	z, _ := d.Doc().Zone("z")
	tx, _ := d.Doc().Text("t")
	keep, _ := d.Doc().Node("keep")
	if n.Layer != "" || c.Layer != "" || z.Layer != "" || tx.Layer != "" {
		t.Fatalf("entities not reassigned to default: %q %q %q %q", n.Layer, c.Layer, z.Layer, tx.Layer)
	}
	if keep.Layer != "" {
		t.Fatal("a default-layer entity was disturbed")
	}
}

func TestIsoLayerEditCommands(t *testing.T) {
	d := NewIsoDiagram(nil)
	// Absent layer: every setter returns false.
	if d.RenameLayer("x", "y") || d.SetLayerVisible("x", false) ||
		d.SetLayerLocked("x", true) || d.SetLayerOrder("x", 9) {
		t.Fatal("a setter mutated an absent layer")
	}
	id := d.AddLayer("orig")
	if !d.RenameLayer(id, "renamed") {
		t.Fatal("rename failed")
	}
	if d.RenameLayer(id, "renamed") {
		t.Fatal("redundant rename returned true")
	}
	if !d.SetLayerVisible(id, false) || d.SetLayerVisible(id, false) {
		t.Fatal("visibility setter change/no-change wrong")
	}
	if !d.SetLayerLocked(id, true) || d.SetLayerLocked(id, true) {
		t.Fatal("lock setter change/no-change wrong")
	}
	if !d.SetLayerOrder(id, 7) || d.SetLayerOrder(id, 7) {
		t.Fatal("order setter change/no-change wrong")
	}
	l, _ := d.Doc().Layer(id)
	if l.Name != "renamed" || l.Visible || !l.Locked || l.Order != 7 {
		t.Fatalf("layer after edits = %+v", l)
	}
}

func TestIsoAssignLayer(t *testing.T) {
	d := NewIsoDiagram(nil)
	id := d.AddLayer("dest")
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutConnector(IsoConnector{ID: "c"})
	d.Doc().PutZone(IsoZone{ID: "z", W: 1, H: 1})
	d.Doc().PutText(IsoText{ID: "t"})
	// Missing entity, and no-op (already on the target) both return false.
	if d.AssignLayer(IsoEntityRef{Kind: IsoEntityNode, ID: "ghost"}, id) {
		t.Fatal("assigning a missing node returned true")
	}
	for _, ref := range []IsoEntityRef{
		{IsoEntityNode, "n"}, {IsoEntityConnector, "c"},
		{IsoEntityZone, "z"}, {IsoEntityText, "t"},
	} {
		if !d.AssignLayer(ref, id) {
			t.Fatalf("assign %+v failed", ref)
		}
		if d.AssignLayer(ref, id) {
			t.Fatalf("redundant assign %+v returned true", ref)
		}
	}
	n, _ := d.Doc().Node("n")
	if n.Layer != id {
		t.Fatalf("node layer = %q, want %q", n.Layer, id)
	}
}

func TestIsoAssignSelectionToLayer(t *testing.T) {
	d := NewIsoDiagram(nil)
	id := d.AddLayer("dest")
	d.Doc().PutNode(IsoNode{ID: "n", X: 1, Y: 1})
	d.Doc().PutZone(IsoZone{ID: "z", W: 1, H: 1})
	if d.AssignSelectionToLayer(id) {
		t.Fatal("assigning an empty selection returned true")
	}
	d.selReplace(IsoEntityRef{IsoEntityNode, "n"}, IsoEntityRef{IsoEntityZone, "z"})
	if !d.AssignSelectionToLayer(id) {
		t.Fatal("assigning a fresh selection failed")
	}
	if d.AssignSelectionToLayer(id) {
		t.Fatal("redundant selection assign returned true")
	}
	n, _ := d.Doc().Node("n")
	z, _ := d.Doc().Zone("z")
	if n.Layer != id || z.Layer != id {
		t.Fatalf("selection not reassigned: %q %q", n.Layer, z.Layer)
	}
}

// --- layered rendering --------------------------------------------------

// TestIsoLayerOrderStacking proves a higher-Order layer draws in front,
// independent of isometric depth: two fully-overlapping nodes at the same cell
// resolve to whichever layer has the greater Order.
func TestIsoLayerOrderStacking(t *testing.T) {
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	green := RGBA{R: 0, G: 220, B: 0, A: 255}
	render := func(aOrder, bOrder int) stdcolor.RGBA {
		d := NewIsoDiagram(nil)
		d.Doc().PutLayer(IsoLayer{ID: "la", Visible: true, Order: aOrder})
		d.Doc().PutLayer(IsoLayer{ID: "lb", Visible: true, Order: bOrder})
		d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3, Color: red, Layer: "la"})
		d.Doc().PutNode(IsoNode{ID: "b", X: 3, Y: 3, Color: green, Layer: "lb"})
		img, err := RenderImage(d, 400, 400, DefaultLight())
		if err != nil {
			t.Fatal(err)
		}
		na, _ := d.Doc().Node("a")
		tx, ty := localOf(d, d.nodeAnchor(na))
		return img.RGBAAt(tx, ty)
	}
	// b on the higher layer -> the top-face centre is green.
	if got := render(0, 1); got.G <= got.R {
		t.Fatalf("higher-layer b not in front: %v", got)
	}
	// a on the higher layer -> the same pixel is now red.
	if got := render(1, 0); got.R <= got.G {
		t.Fatalf("higher-layer a not in front: %v", got)
	}
}

// TestIsoHiddenLayerNotRendered proves a hidden layer's node leaves the surface
// untouched, while the same node on a visible layer paints its colour.
func TestIsoHiddenLayerNotRendered(t *testing.T) {
	theme := DefaultLight()
	red := RGBA{R: 220, G: 0, B: 0, A: 255}
	probe := func(visible bool) stdcolor.RGBA {
		d := NewIsoDiagram(nil)
		d.Doc().PutLayer(IsoLayer{ID: "L", Visible: visible, Order: 1})
		d.Doc().PutNode(IsoNode{ID: "a", X: 3, Y: 3, Color: red, Layer: "L"})
		img, err := RenderImage(d, 400, 400, theme)
		if err != nil {
			t.Fatal(err)
		}
		na, _ := d.Doc().Node("a")
		tx, ty := localOf(d, d.nodeAnchor(na))
		return img.RGBAAt(tx, ty)
	}
	if got := probe(true); got != stdcolor.RGBA(stdColor(red)) {
		t.Fatalf("visible-layer node pixel = %v, want red", got)
	}
	if got := probe(false); got != stdcolor.RGBA(stdColor(theme.Surface)) {
		t.Fatalf("hidden-layer node pixel = %v, want surface (node must not draw)", got)
	}
}

// TestIsoLayeredZoneAndSpritePasses drives the multi-order slow path through
// zones, sprite icons and connectors so every per-layer branch renders.
func TestIsoLayeredZoneAndSpritePasses(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutLayer(IsoLayer{ID: "low", Visible: true, Order: 0})
	d.Doc().PutLayer(IsoLayer{ID: "high", Visible: true, Order: 2})
	d.Doc().PutLayer(IsoLayer{ID: "hiddenHi", Visible: false, Order: 5})
	// zone + node + connector + sprite node spread across orders (and a hidden
	// node to force blitHidden / skip a hidden entity in a pass).
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 3, H: 3, Layer: "low"})
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1, Layer: "low"})
	d.Doc().PutNode(IsoNode{ID: "b", X: 2, Y: 2, Layer: "high"})
	d.Doc().PutNode(IsoNode{ID: "s", X: 4, Y: 4, Icon: "user", Layer: "high"})
	d.Doc().PutNode(IsoNode{ID: "ghost", X: 6, Y: 6, Layer: "hiddenHi"})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Layer: "high"})
	if len(d.contentOrders()) < 2 {
		t.Fatalf("expected several orders, got %v", d.contentOrders())
	}
	if !d.blitHidden() {
		t.Fatal("a hidden node should make blitHidden true")
	}
	if _, err := RenderImage(d, 500, 500, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

// TestIsoRetroCompatByteIdentical is the control run: a document built with only
// Vague-A/B/C features (NO layers) renders byte-for-byte identically to the same
// document with every entity moved onto one explicit default-order visible
// layer — proving the Layer field and the layer machinery are inert when unused.
func TestIsoRetroCompatByteIdentical(t *testing.T) {
	theme := DefaultLight()
	build := func(layer string, withLayerRecord bool) *IsoDiagram {
		d := NewIsoDiagram(nil)
		if withLayerRecord {
			d.Doc().PutLayer(IsoLayer{ID: layer, Name: "base", Visible: true, Order: 0})
		}
		d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2, Label: "Web", Color: RGBA{R: 200, G: 30, B: 30, A: 255}, Layer: layer})
		d.Doc().PutNode(IsoNode{ID: "b", X: 5, Y: 4, Shape: IsoBox, Icon: "server", Layer: layer})
		d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Arrow: IsoArrowSingle, Label: "http", Layer: layer})
		d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 3, H: 2, Label: "vpc", Layer: layer})
		d.Doc().PutText(IsoText{ID: "t", X: 6, Y: 1, Text: "note", Layer: layer})
		return d
	}
	plain := build("", false)
	// The unlayered document must take the fast (byte-identical) render path.
	if len(plain.contentOrders()) != 1 || plain.blitHidden() {
		t.Fatalf("unlayered doc did not take the legacy fast path: orders=%v hidden=%v",
			plain.contentOrders(), plain.blitHidden())
	}
	a, err := RenderImage(plain, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	layered := build("base", true)
	b, err := RenderImage(layered, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("adding a default-order visible layer changed the rendered pixels")
	}
}

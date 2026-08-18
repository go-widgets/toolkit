// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestIsoDocPutAndGetNode(t *testing.T) {
	d := NewIsoDoc()
	if _, ok := d.Node("x"); ok {
		t.Fatal("empty doc reported a node")
	}
	d.PutNode(IsoNode{ID: "a", X: 1, Y: 2, Label: "A"})
	d.PutNode(IsoNode{ID: "b", X: 3, Y: 4})
	if got := len(d.Nodes()); got != 2 {
		t.Fatalf("Nodes len = %d, want 2", got)
	}
	n, ok := d.Node("a")
	if !ok || n.X != 1 || n.Y != 2 || n.Label != "A" {
		t.Fatalf("Node(a) = %+v ok=%v", n, ok)
	}
	// Upsert replaces in place (no new element).
	d.PutNode(IsoNode{ID: "a", X: 9, Y: 9, Label: "A2"})
	if got := len(d.Nodes()); got != 2 {
		t.Fatalf("after upsert Nodes len = %d, want 2", got)
	}
	if n, _ := d.Node("a"); n.X != 9 || n.Label != "A2" {
		t.Fatalf("upsert did not replace: %+v", n)
	}
}

func TestIsoDocNodesIsCopy(t *testing.T) {
	d := NewIsoDoc()
	d.PutNode(IsoNode{ID: "a"})
	s := d.Nodes()
	s[0].X = 100
	if n, _ := d.Node("a"); n.X != 0 {
		t.Fatalf("Nodes() returned a live alias: %+v", n)
	}
}

func TestIsoDocRemoveNodeDropsConnectors(t *testing.T) {
	d := NewIsoDoc()
	d.PutNode(IsoNode{ID: "a"})
	d.PutNode(IsoNode{ID: "b"})
	d.PutNode(IsoNode{ID: "c"})
	d.PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.PutConnector(IsoConnector{ID: "bc", From: "b", To: "c"})
	d.RemoveNode("missing") // no-op branch
	d.RemoveNode("b")       // drops both connectors touching b
	if _, ok := d.Node("b"); ok {
		t.Fatal("b still present")
	}
	if got := len(d.Connectors()); got != 0 {
		t.Fatalf("connectors len = %d, want 0", got)
	}
}

func TestIsoDocConnectorUpsertAndRemove(t *testing.T) {
	d := NewIsoDoc()
	d.PutConnector(IsoConnector{ID: "x", From: "a", To: "b"})
	d.PutConnector(IsoConnector{ID: "x", From: "a", To: "c"}) // upsert
	if got := len(d.Connectors()); got != 1 {
		t.Fatalf("connectors len = %d, want 1", got)
	}
	if c := d.Connectors()[0]; c.To != "c" {
		t.Fatalf("upsert did not replace: %+v", c)
	}
	d.RemoveConnector("nope") // no-op branch
	d.RemoveConnector("x")
	if got := len(d.Connectors()); got != 0 {
		t.Fatalf("after remove connectors len = %d, want 0", got)
	}
}

func TestIsoDocSubscribeFires(t *testing.T) {
	d := NewIsoDoc()
	n := 0
	unsub := d.Subscribe(func() { n++ })
	d.PutNode(IsoNode{ID: "a"})                               // node change
	d.PutConnector(IsoConnector{ID: "c", From: "a", To: "a"}) // connector change
	if n != 2 {
		t.Fatalf("subscriber fired %d times, want 2", n)
	}
	unsub()
	d.PutNode(IsoNode{ID: "b"})
	if n != 2 {
		t.Fatalf("subscriber fired after unsubscribe: %d", n)
	}
}

func TestIsoDocObservableAccessors(t *testing.T) {
	d := NewIsoDoc()
	if d.NodeList() == nil || d.ConnectorList() == nil {
		t.Fatal("observable list accessors returned nil")
	}
	d.NodeList().Append(IsoNode{ID: "a"})
	if _, ok := d.Node("a"); !ok {
		t.Fatal("append through NodeList not visible via Node")
	}
}

// --- Wave C: zones ------------------------------------------------------

func TestIsoDocZoneUpsertGetRemove(t *testing.T) {
	d := NewIsoDoc()
	if _, ok := d.Zone("z"); ok {
		t.Fatal("empty doc reported a zone")
	}
	d.PutZone(IsoZone{ID: "z1", X: 1, Y: 2, W: 3, H: 4, Label: "Z"})
	d.PutZone(IsoZone{ID: "z2", X: 0, Y: 0, W: 1, H: 1})
	if got := len(d.Zones()); got != 2 {
		t.Fatalf("Zones len = %d, want 2", got)
	}
	z, ok := d.Zone("z1")
	if !ok || z.X != 1 || z.W != 3 || z.Label != "Z" {
		t.Fatalf("Zone(z1) = %+v ok=%v", z, ok)
	}
	// Upsert replaces in place.
	d.PutZone(IsoZone{ID: "z1", X: 9, Y: 9, W: 2, H: 2})
	if got := len(d.Zones()); got != 2 {
		t.Fatalf("after upsert Zones len = %d, want 2", got)
	}
	if z, _ := d.Zone("z1"); z.X != 9 || z.W != 2 {
		t.Fatalf("zone upsert did not replace: %+v", z)
	}
	// Zones() is a copy.
	s := d.Zones()
	s[0].X = 123
	if z, _ := d.Zone("z1"); z.X != 9 {
		t.Fatalf("Zones() returned a live alias: %+v", z)
	}
	d.RemoveZone("nope") // missing -> no-op
	if len(d.Zones()) != 2 {
		t.Fatal("removing a missing zone changed the set")
	}
	d.RemoveZone("z1")
	if _, ok := d.Zone("z1"); ok {
		t.Fatal("RemoveZone did not remove z1")
	}
}

// --- Wave C: texts ------------------------------------------------------

func TestIsoDocTextUpsertGetRemove(t *testing.T) {
	d := NewIsoDoc()
	if _, ok := d.Text("t"); ok {
		t.Fatal("empty doc reported a text")
	}
	d.PutText(IsoText{ID: "t1", X: 2, Y: 3, Text: "hi"})
	d.PutText(IsoText{ID: "t2", X: 0, Y: 0})
	if got := len(d.Texts()); got != 2 {
		t.Fatalf("Texts len = %d, want 2", got)
	}
	tx, ok := d.Text("t1")
	if !ok || tx.X != 2 || tx.Text != "hi" {
		t.Fatalf("Text(t1) = %+v ok=%v", tx, ok)
	}
	d.PutText(IsoText{ID: "t1", X: 5, Y: 5, Text: "bye"})
	if got := len(d.Texts()); got != 2 {
		t.Fatalf("after upsert Texts len = %d, want 2", got)
	}
	if tx, _ := d.Text("t1"); tx.X != 5 || tx.Text != "bye" {
		t.Fatalf("text upsert did not replace: %+v", tx)
	}
	s := d.Texts()
	s[0].X = 321
	if tx, _ := d.Text("t1"); tx.X != 5 {
		t.Fatalf("Texts() returned a live alias: %+v", tx)
	}
	d.RemoveText("nope") // missing -> no-op
	if len(d.Texts()) != 2 {
		t.Fatal("removing a missing text changed the set")
	}
	d.RemoveText("t1")
	if _, ok := d.Text("t1"); ok {
		t.Fatal("RemoveText did not remove t1")
	}
}

func TestIsoDocSubscribeFiresForZonesAndTexts(t *testing.T) {
	d := NewIsoDoc()
	n := 0
	unsub := d.Subscribe(func() { n++ })
	d.PutZone(IsoZone{ID: "z"}) // zone change
	d.PutText(IsoText{ID: "t"}) // text change
	if n != 2 {
		t.Fatalf("subscriber fired %d times, want 2", n)
	}
	unsub()
	d.PutZone(IsoZone{ID: "z2"})
	d.PutText(IsoText{ID: "t2"})
	if n != 2 {
		t.Fatalf("subscriber fired after unsubscribe: %d", n)
	}
}

func TestIsoDocZoneTextListAccessors(t *testing.T) {
	d := NewIsoDoc()
	if d.ZoneList() == nil || d.TextList() == nil {
		t.Fatal("zone/text list accessors returned nil")
	}
	d.ZoneList().Append(IsoZone{ID: "z"})
	if _, ok := d.Zone("z"); !ok {
		t.Fatal("append through ZoneList not visible via Zone")
	}
	d.TextList().Append(IsoText{ID: "t"})
	if _, ok := d.Text("t"); !ok {
		t.Fatal("append through TextList not visible via Text")
	}
}

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

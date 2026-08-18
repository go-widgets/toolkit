// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestIsoCopyPasteInternalConnectors proves copy→paste recreates the selected
// nodes and their INTERNAL connectors with fresh ids and the exact tile offset,
// while a connector with an unselected endpoint is not carried.
func TestIsoCopyPasteInternalConnectors(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "b", X: 3, Y: 1})
	d.Doc().PutNode(IsoNode{ID: "out", X: 8, Y: 8}) // not selected
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Label: "wire"})
	d.Doc().PutConnector(IsoConnector{ID: "aout", From: "a", To: "out"}) // external -> not copied
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"}, IsoEntityRef{IsoEntityNode, "b"})
	d.Copy()
	d.Paste()

	// 3 original nodes + 2 pasted = 5; original 2 connectors + 1 internal = 3.
	if len(d.Doc().Nodes()) != 5 {
		t.Fatalf("nodes after paste = %d, want 5", len(d.Doc().Nodes()))
	}
	conns := d.Doc().Connectors()
	if len(conns) != 3 {
		t.Fatalf("connectors after paste = %d, want 3 (only the internal one duplicated)", len(conns))
	}
	// The pasted entities are exactly the new selection, all offset by (+1,+1).
	sel := d.Selection()
	if len(sel) != 3 { // 2 nodes + 1 connector
		t.Fatalf("paste selected %d, want 3: %v", len(sel), sel)
	}
	var pastedNodes []IsoNode
	for _, r := range sel {
		if r.Kind == IsoEntityNode {
			n, _ := d.Doc().Node(r.ID)
			pastedNodes = append(pastedNodes, n)
			if r.ID == "a" || r.ID == "b" {
				t.Fatalf("pasted node reused an original id %q", r.ID)
			}
		}
	}
	// Positions: originals at (1,1)/(3,1), pastes must be at (2,2)/(4,2).
	want := map[[2]int]bool{{2, 2}: true, {4, 2}: true}
	for _, n := range pastedNodes {
		if !want[[2]int{n.X, n.Y}] {
			t.Fatalf("pasted node at (%d,%d) not offset by (+1,+1)", n.X, n.Y)
		}
	}
	// The pasted connector links the two pasted nodes (new ids), not the originals.
	pastedIDs := map[string]bool{}
	for _, n := range pastedNodes {
		pastedIDs[n.ID] = true
	}
	var pastedConn IsoConnector
	for _, c := range conns {
		if c.ID != "ab" && c.ID != "aout" {
			pastedConn = c
		}
	}
	if !pastedIDs[pastedConn.From] || !pastedIDs[pastedConn.To] {
		t.Fatalf("pasted connector %+v not rewired onto the pasted nodes", pastedConn)
	}
	if pastedConn.Label != "wire" {
		t.Fatalf("pasted connector lost its label: %q", pastedConn.Label)
	}
	// Paste is ONE undoable op.
	d.Undo()
	if len(d.Doc().Nodes()) != 3 || len(d.Doc().Connectors()) != 2 {
		t.Fatal("undo did not remove the whole paste in one step")
	}
}

func TestIsoCopyZonesAndTexts(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 2, H: 2, Label: "grp"})
	d.Doc().PutText(IsoText{ID: "t", X: 4, Y: 4, Text: "note"})
	d.selReplace(IsoEntityRef{IsoEntityZone, "z"}, IsoEntityRef{IsoEntityText, "t"})
	d.Copy()
	d.Paste()
	if len(d.Doc().Zones()) != 2 || len(d.Doc().Texts()) != 2 {
		t.Fatalf("paste made %d zones %d texts, want 2 each", len(d.Doc().Zones()), len(d.Doc().Texts()))
	}
	// Find the pasted zone / text (the ones not at the original cell).
	var pz IsoZone
	for _, z := range d.Doc().Zones() {
		if z.ID != "z" {
			pz = z
		}
	}
	if pz.X != 1 || pz.Y != 1 || pz.W != 2 || pz.H != 2 || pz.Label != "grp" {
		t.Fatalf("pasted zone = %+v, want offset (1,1) 2x2 grp", pz)
	}
}

func TestIsoCopyEmptySelectionKeepsClipboard(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"})
	d.Copy()
	// Copy with nothing selected must not wipe the clipboard.
	d.selClear()
	d.Copy()
	d.Paste()
	if len(d.Doc().Nodes()) != 2 {
		t.Fatal("empty Copy wiped the clipboard (paste produced nothing)")
	}
}

func TestIsoPasteEmptyClipboardNoop(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Paste() // empty clipboard -> no-op, no undo entry
	if d.CanUndo() {
		t.Fatal("pasting an empty clipboard pushed an undo entry")
	}
}

func TestIsoCut(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"})
	d.Cut()
	if len(d.Doc().Nodes()) != 0 {
		t.Fatal("Cut did not remove the selection")
	}
	// The cut entity is on the clipboard and pastes back.
	d.Paste()
	if len(d.Doc().Nodes()) != 1 {
		t.Fatal("Cut did not leave the entity on the clipboard")
	}
}

func TestIsoDuplicate(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2})
	// Ctrl-D with nothing selected is a no-op.
	d.OnEvent(Event{Kind: EventKeyDown, Code: "d", Ctrl: true})
	if len(d.Doc().Nodes()) != 1 {
		t.Fatal("Ctrl-D with no selection duplicated something")
	}
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"})
	d.OnEvent(Event{Kind: EventKeyDown, Code: "D", Ctrl: true})
	if len(d.Doc().Nodes()) != 2 {
		t.Fatalf("Ctrl-D did not duplicate: %d nodes", len(d.Doc().Nodes()))
	}
	// The duplicate is selected and offset.
	if len(d.Selection()) != 1 || d.IsSelected(IsoEntityRef{IsoEntityNode, "a"}) {
		t.Fatalf("duplicate did not leave the copy selected: %v", d.Selection())
	}
}

func TestIsoClipboardKeyboardCopyPaste(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 1})
	d.selReplace(IsoEntityRef{IsoEntityNode, "a"})
	d.OnEvent(Event{Kind: EventKeyDown, Code: "c"}) // no Ctrl -> ignored
	d.OnEvent(Event{Kind: EventKeyDown, Code: "v"}) // no Ctrl -> ignored
	if len(d.Doc().Nodes()) != 1 {
		t.Fatal("plain c/v mutated the document")
	}
	d.OnEvent(Event{Kind: EventKeyDown, Code: "c", Ctrl: true}) // copy
	d.OnEvent(Event{Kind: EventKeyDown, Code: "v", Ctrl: true}) // paste
	if len(d.Doc().Nodes()) != 2 {
		t.Fatal("Ctrl-C then Ctrl-V did not paste")
	}
	// Ctrl-X cuts the current (pasted) selection.
	d.OnEvent(Event{Kind: EventKeyDown, Code: "x", Ctrl: true})
	if len(d.Doc().Nodes()) != 1 {
		t.Fatal("Ctrl-X did not cut the selection")
	}
}

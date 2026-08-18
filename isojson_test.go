// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// kitchenSinkDoc builds a document exercising every entity family, every enum
// value (cube/box/pyramid, solid/dashed/dotted, none/single/double), coloured
// and uncoloured entities, icons, labels, widths, routing, sizes and layers —
// inserted deliberately OUT of id order so a round-trip proves the marshaller
// sorts.
func kitchenSinkDoc() *IsoDoc {
	d := NewIsoDoc()
	// layers, reverse order
	d.PutLayer(IsoLayer{ID: "L2", Name: "back", Visible: true, Order: 2})
	d.PutLayer(IsoLayer{ID: "L1", Name: "front", Visible: true, Locked: true, Order: 5})
	// nodes, reverse order, mixed shapes / colours / icons / layers
	d.PutNode(IsoNode{ID: "n3", X: 4, Y: 1, Shape: IsoPyramid, Label: "pyr", Layer: "L2"})
	d.PutNode(IsoNode{ID: "n2", X: 2, Y: 2, Shape: IsoBox, Icon: "server", Color: RGBA{R: 10, G: 20, B: 30, A: 255}})
	d.PutNode(IsoNode{ID: "n1", X: 0, Y: 0, Shape: IsoCube})
	// connectors, mixed styles / arrows / colour / width / routed / layer
	d.PutConnector(IsoConnector{ID: "c2", From: "n2", To: "n3", Style: IsoDotted, Arrow: IsoArrowDouble, Width: 3, Routed: true, Layer: "L1"})
	d.PutConnector(IsoConnector{ID: "c1", From: "n1", To: "n2", Style: IsoDashed, Arrow: IsoArrowSingle, Color: RGBA{R: 200, A: 255}, Label: "link"})
	// c3 is a plain connector: default (solid) style and no arrow head, so the
	// default arm of every enum encoder is exercised too.
	d.PutConnector(IsoConnector{ID: "c3", From: "n1", To: "n3"})
	// zones
	d.PutZone(IsoZone{ID: "z2", X: 3, Y: 3, W: 2, H: 2, Label: "grp", Layer: "L2"})
	d.PutZone(IsoZone{ID: "z1", X: 0, Y: 0, W: 1, H: 1, Color: RGBA{G: 128, A: 64}})
	// texts
	d.PutText(IsoText{ID: "t2", X: 5, Y: 5, Text: "note", Size: 2, Color: RGBA{B: 200, A: 255}, Layer: "L1"})
	d.PutText(IsoText{ID: "t1", X: 1, Y: 1})
	return d
}

// sortedSnapshot returns every family of doc sorted by id, so two documents that
// hold the same entities in different insertion orders compare equal.
func sortedSnapshot(doc IsoDocument) isoSnapshot {
	s := isoSnapshot{
		nodes:  doc.Nodes(),
		conns:  doc.Connectors(),
		zones:  doc.Zones(),
		texts:  doc.Texts(),
		layers: doc.Layers(),
	}
	sort.Slice(s.nodes, func(i, j int) bool { return s.nodes[i].ID < s.nodes[j].ID })
	sort.Slice(s.conns, func(i, j int) bool { return s.conns[i].ID < s.conns[j].ID })
	sort.Slice(s.zones, func(i, j int) bool { return s.zones[i].ID < s.zones[j].ID })
	sort.Slice(s.texts, func(i, j int) bool { return s.texts[i].ID < s.texts[j].ID })
	sort.Slice(s.layers, func(i, j int) bool { return s.layers[i].ID < s.layers[j].ID })
	return s
}

// TestIsoJSONRoundTrip proves the core guarantees: a marshal is deterministic,
// import reconstructs a structurally equal document, and Marshal → Unmarshal →
// Marshal is byte-for-byte identical.
func TestIsoJSONRoundTrip(t *testing.T) {
	doc := kitchenSinkDoc()

	b1, err := MarshalIsoDocument(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Deterministic: a second marshal of the same document is identical.
	b1b, _ := MarshalIsoDocument(doc)
	if !bytes.Equal(b1, b1b) {
		t.Fatal("marshal is not deterministic")
	}

	doc2, err := UnmarshalIsoDocument(b1)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Structural equality (order-independent).
	if !reflect.DeepEqual(sortedSnapshot(doc), sortedSnapshot(doc2)) {
		t.Fatalf("reconstructed document differs:\n got %+v\nwant %+v",
			sortedSnapshot(doc2), sortedSnapshot(doc))
	}

	// Byte-identical re-serialisation.
	b2, err := MarshalIsoDocument(doc2)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("round-trip not byte-identical:\n%s\n---\n%s", b1, b2)
	}

	// The bare-document marshal carries no viewport.
	if strings.Contains(string(b1), "viewport") {
		t.Fatal("MarshalIsoDocument must not emit a viewport")
	}
	// Enum words and a hex colour are present (schema is ours, human-legible).
	for _, want := range []string{"pyramid", "\"box\"", "dashed", "dotted", "single", "double", "#c80000ff", "\"schemaVersion\": 1"} {
		if !strings.Contains(string(b1), want) {
			t.Fatalf("expected %q in output:\n%s", want, b1)
		}
	}
}

// TestIsoJSONEmptyDoc round-trips a document with nothing in it.
func TestIsoJSONEmptyDoc(t *testing.T) {
	b, err := MarshalIsoDocument(NewIsoDoc())
	if err != nil {
		t.Fatal(err)
	}
	doc, err := UnmarshalIsoDocument(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Nodes())+len(doc.Connectors())+len(doc.Zones())+len(doc.Texts())+len(doc.Layers()) != 0 {
		t.Fatal("empty round-trip produced entities")
	}
}

// TestIsoJSONLegacyLoad proves a document that omits newer fields (no layer, no
// colour, no shape, no coordinates) loads with those fields at their zero
// values — the forward/backward tolerance the schema promises.
func TestIsoJSONLegacyLoad(t *testing.T) {
	legacy := []byte(`{
		"schemaVersion": 1,
		"nodes": [{"id": "n1"}],
		"connectors": [{"id": "c1", "from": "n1", "to": "n1"}],
		"zones": [{"id": "z1"}],
		"texts": [{"id": "t1"}]
	}`)
	doc, err := UnmarshalIsoDocument(legacy)
	if err != nil {
		t.Fatalf("legacy load: %v", err)
	}
	n, _ := doc.Node("n1")
	if n.X != 0 || n.Y != 0 || n.Shape != IsoCube || n.Color != (RGBA{}) || n.Layer != "" {
		t.Fatalf("missing fields did not load as zero values: %+v", n)
	}
	z, _ := doc.Zone("z1")
	if z.Layer != "" || z.Color != (RGBA{}) {
		t.Fatalf("zone missing fields not zero: %+v", z)
	}
	tx, _ := doc.Text("t1")
	if tx.Size != 0 || tx.Layer != "" {
		t.Fatalf("text missing fields not zero: %+v", tx)
	}
}

// TestIsoWidgetExportImport round-trips through the widget API: the viewport is
// recorded on export and applied on import, and a widget round-trip is
// byte-identical.
func TestIsoWidgetExportImport(t *testing.T) {
	d := NewIsoDiagram(kitchenSinkDoc())
	d.Cols, d.Rows = 17, 23

	b1, err := d.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b1), "\"cols\": 17") || !strings.Contains(string(b1), "\"rows\": 23") {
		t.Fatalf("viewport not exported:\n%s", b1)
	}

	d2 := NewIsoDiagram(nil)
	if err := d2.ImportJSON(b1); err != nil {
		t.Fatal(err)
	}
	if d2.Cols != 17 || d2.Rows != 23 {
		t.Fatalf("viewport not applied on import: Cols=%d Rows=%d", d2.Cols, d2.Rows)
	}
	b2, err := d2.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("widget round-trip not byte-identical:\n%s\n---\n%s", b1, b2)
	}
}

// TestIsoImportUndoable proves ImportJSON is one undoable command: undo restores
// the exact prior document.
func TestIsoImportUndoable(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a1", X: 1, Y: 1, Label: "before"})

	newDoc := NewIsoDoc()
	newDoc.PutNode(IsoNode{ID: "b1", X: 9, Y: 9, Label: "after"})
	data, _ := MarshalIsoDocument(newDoc)

	if err := d.ImportJSON(data); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.Doc().Node("b1"); !ok {
		t.Fatal("import did not add the new node")
	}
	if _, ok := d.Doc().Node("a1"); ok {
		t.Fatal("import did not replace the previous document")
	}
	if !d.CanUndo() {
		t.Fatal("import left no undo entry")
	}
	d.Undo()
	if _, ok := d.Doc().Node("a1"); !ok {
		t.Fatal("undo did not restore the previous document")
	}
	if _, ok := d.Doc().Node("b1"); ok {
		t.Fatal("undo did not remove the imported node")
	}
}

// TestIsoImportDecodeErrorLeavesDocUntouched proves a decode failure returns the
// error and mutates nothing (no undo entry, no partial load).
func TestIsoImportDecodeErrorLeavesDocUntouched(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "keep", X: 1, Y: 1})
	if err := d.ImportJSON([]byte("{ not json")); err == nil {
		t.Fatal("expected a decode error")
	}
	if _, ok := d.Doc().Node("keep"); !ok {
		t.Fatal("failed import disturbed the document")
	}
	if d.CanUndo() {
		t.Fatal("failed import pushed an undo entry")
	}
}

// TestIsoUnmarshalErrors sweeps every decode error branch: malformed JSON, an
// unsupported version, blank / duplicate ids in every family, malformed colours
// (both the shape-check and the hex-decode arms), unknown enum words and dangling
// connector endpoints.
func TestIsoUnmarshalErrors(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string // substring the error must contain
	}{
		{"malformed", `{`, "iso import"},
		{"missing-version", `{}`, "unsupported schemaVersion"},
		{"future-version", `{"schemaVersion": 999}`, "unsupported schemaVersion"},
		{"empty-node-id", `{"schemaVersion":1,"nodes":[{}]}`, "empty id"},
		{"dup-node-id", `{"schemaVersion":1,"nodes":[{"id":"n1"},{"id":"n1"}]}`, "duplicate node"},
		{"node-bad-color", `{"schemaVersion":1,"nodes":[{"id":"n1","color":"nothex"}]}`, "malformed colour"},
		{"node-bad-hex", `{"schemaVersion":1,"nodes":[{"id":"n1","color":"#gggggggg"}]}`, "malformed colour"},
		{"node-bad-shape", `{"schemaVersion":1,"nodes":[{"id":"n1","shape":"sphere"}]}`, "unknown node shape"},
		{"dup-layer-id", `{"schemaVersion":1,"layers":[{"id":"L1"},{"id":"L1"}]}`, "duplicate layer"},
		{"empty-layer-id", `{"schemaVersion":1,"layers":[{"id":""}]}`, "empty id"},
		{"conn-bad-color", `{"schemaVersion":1,"nodes":[{"id":"n1"}],"connectors":[{"id":"c1","from":"n1","to":"n1","color":"#zzzzzzzz"}]}`, "malformed colour"},
		{"conn-bad-style", `{"schemaVersion":1,"nodes":[{"id":"n1"}],"connectors":[{"id":"c1","from":"n1","to":"n1","style":"wavy"}]}`, "unknown connector style"},
		{"conn-bad-arrow", `{"schemaVersion":1,"nodes":[{"id":"n1"}],"connectors":[{"id":"c1","from":"n1","to":"n1","arrow":"triple"}]}`, "unknown connector arrow"},
		{"dup-conn-id", `{"schemaVersion":1,"nodes":[{"id":"n1"}],"connectors":[{"id":"c1","from":"n1","to":"n1"},{"id":"c1","from":"n1","to":"n1"}]}`, "duplicate connector"},
		{"conn-missing-from", `{"schemaVersion":1,"connectors":[{"id":"c1","from":"ghost","to":"ghost"}]}`, "references missing node \"ghost\""},
		{"conn-missing-to", `{"schemaVersion":1,"nodes":[{"id":"n1"}],"connectors":[{"id":"c1","from":"n1","to":"ghost"}]}`, "references missing node \"ghost\""},
		{"zone-bad-color", `{"schemaVersion":1,"zones":[{"id":"z1","color":"nothex"}]}`, "malformed colour"},
		{"dup-zone-id", `{"schemaVersion":1,"zones":[{"id":"z1"},{"id":"z1"}]}`, "duplicate zone"},
		{"text-bad-color", `{"schemaVersion":1,"texts":[{"id":"t1","color":"nothex"}]}`, "malformed colour"},
		{"dup-text-id", `{"schemaVersion":1,"texts":[{"id":"t1"},{"id":"t1"}]}`, "duplicate text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := UnmarshalIsoDocument([]byte(tc.json))
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			if doc != nil {
				t.Fatalf("a failed import returned a non-nil document")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestIsoColorCodec covers the colour codec directly, including the zero-colour
// (omit) path both ways.
func TestIsoColorCodec(t *testing.T) {
	if got := isoColorToHex(RGBA{}); got != "" {
		t.Fatalf("zero colour should render empty, got %q", got)
	}
	if got := isoColorToHex(RGBA{R: 1, G: 2, B: 3, A: 4}); got != "#01020304" {
		t.Fatalf("hex render wrong: %q", got)
	}
	got, err := isoColorFromHex("#01020304")
	if err != nil {
		t.Fatal(err)
	}
	if got != (RGBA{R: 1, G: 2, B: 3, A: 4}) {
		t.Fatalf("hex parse wrong: %+v", got)
	}
	if c, err := isoColorFromHex(""); err != nil || c != (RGBA{}) {
		t.Fatalf("empty colour should parse to zero, got %+v err=%v", c, err)
	}
}

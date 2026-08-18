// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// IsoSchemaVersion is the version stamped into every document this build writes
// and the only version it reads. It is OUR native format — designed here, not
// borrowed from any other isometric editor — so the number tracks this schema
// alone. A file carrying any other version is rejected by [UnmarshalIsoDocument]
// with a clear error rather than silently mis-read. Additive field changes stay
// within one version: a reader loads a missing field as its zero value (an older
// file that predates, say, layers has no "layer" keys and every entity lands on
// the implicit default layer), so the version only bumps on a breaking change.
const IsoSchemaVersion = 1

// --- wire structs -------------------------------------------------------
//
// The JSON shape is OURS. Every enum is a lowercase word (not a magic integer),
// every colour an "#rrggbbaa" hex string, and every field whose zero value is
// the entity's default is `omitempty` — so the file is compact, legible in a
// diff and forward/backward tolerant (a dropped default reads back as the zero
// value). Key order is the struct field order (deterministic) and the entity
// arrays are sorted by id on write, so re-serialising an unchanged document is
// byte-for-byte stable — the property the collaborative layer builds diffs on.

// isoDocJSON is the whole-document envelope. schemaVersion comes first so a
// reader can gate on it before touching the payload; viewport is an optional
// widget hint (grid extent) a bare-document marshal omits entirely.
type isoDocJSON struct {
	SchemaVersion int                `json:"schemaVersion"`
	Viewport      *isoViewportJSON   `json:"viewport,omitempty"`
	Layers        []isoLayerJSON     `json:"layers,omitempty"`
	Nodes         []isoNodeJSON      `json:"nodes,omitempty"`
	Connectors    []isoConnectorJSON `json:"connectors,omitempty"`
	Zones         []isoZoneJSON      `json:"zones,omitempty"`
	Texts         []isoTextJSON      `json:"texts,omitempty"`
}

// isoViewportJSON is a default view hint: the ground grid's extent in cells. It
// is NOT part of the document model (pan/zoom and grid size live on the widget,
// not the store), so [MarshalIsoDocument] leaves it nil; only the widget's
// [IsoDiagram.ExportJSON] fills it in and [IsoDiagram.ImportJSON] applies it.
type isoViewportJSON struct {
	Cols int `json:"cols,omitempty"`
	Rows int `json:"rows,omitempty"`
}

// isoNodeJSON is the wire form of an [IsoNode].
type isoNodeJSON struct {
	ID    string `json:"id"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	Shape string `json:"shape,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Label string `json:"label,omitempty"`
	Color string `json:"color,omitempty"`
	Layer string `json:"layer,omitempty"`
}

// isoConnectorJSON is the wire form of an [IsoConnector]. from/to are always
// written (a connector without endpoints is meaningless), the rest are
// omitempty enrichments.
type isoConnectorJSON struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label,omitempty"`
	Style  string `json:"style,omitempty"`
	Arrow  string `json:"arrow,omitempty"`
	Color  string `json:"color,omitempty"`
	Width  int    `json:"width,omitempty"`
	Routed bool   `json:"routed,omitempty"`
	Layer  string `json:"layer,omitempty"`
}

// isoZoneJSON is the wire form of an [IsoZone].
type isoZoneJSON struct {
	ID    string `json:"id"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	W     int    `json:"w,omitempty"`
	H     int    `json:"h,omitempty"`
	Color string `json:"color,omitempty"`
	Label string `json:"label,omitempty"`
	Layer string `json:"layer,omitempty"`
}

// isoTextJSON is the wire form of an [IsoText].
type isoTextJSON struct {
	ID    string `json:"id"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	Text  string `json:"text,omitempty"`
	Color string `json:"color,omitempty"`
	Size  int    `json:"size,omitempty"`
	Layer string `json:"layer,omitempty"`
}

// isoLayerJSON is the wire form of an [IsoLayer].
type isoLayerJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Visible bool   `json:"visible,omitempty"`
	Locked  bool   `json:"locked,omitempty"`
	Order   int    `json:"order,omitempty"`
}

// --- colour codec -------------------------------------------------------

// isoColorToHex renders a colour as a lowercase "#rrggbbaa" string, or the empty
// string for the zero colour (A==0) so an entity that inherits its theme colour
// writes no "color" key at all.
func isoColorToHex(c RGBA) string {
	if c.A == 0 {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// isoColorFromHex parses an "#rrggbbaa" string back to a colour. The empty
// string (a missing "color" key) is the zero colour. Anything that is not a
// "#" followed by exactly eight hex digits is a clear error, never a panic.
func isoColorFromHex(s string) (RGBA, error) {
	if s == "" {
		return RGBA{}, nil
	}
	if len(s) != 9 || s[0] != '#' {
		return RGBA{}, fmt.Errorf("iso import: malformed colour %q (want #rrggbbaa)", s)
	}
	b, err := hex.DecodeString(s[1:])
	if err != nil {
		return RGBA{}, fmt.Errorf("iso import: malformed colour %q: %w", s, err)
	}
	return RGBA{R: b[0], G: b[1], B: b[2], A: b[3]}, nil
}

// --- enum codecs --------------------------------------------------------

// isoShapeToStr names a node shape; the default cube writes the empty string
// (omitted).
func isoShapeToStr(s IsoShape) string {
	switch s {
	case IsoBox:
		return "box"
	case IsoPyramid:
		return "pyramid"
	default:
		return ""
	}
}

// isoShapeFromStr parses a node-shape word; the empty string is the default
// cube, and an unrecognised word is a clear error.
func isoShapeFromStr(s string) (IsoShape, error) {
	switch s {
	case "":
		return IsoCube, nil
	case "box":
		return IsoBox, nil
	case "pyramid":
		return IsoPyramid, nil
	default:
		return 0, fmt.Errorf("iso import: unknown node shape %q", s)
	}
}

// isoStyleToStr names a connector stroke style; the default solid writes the
// empty string (omitted).
func isoStyleToStr(s IsoConnectorStyle) string {
	switch s {
	case IsoDashed:
		return "dashed"
	case IsoDotted:
		return "dotted"
	default:
		return ""
	}
}

// isoStyleFromStr parses a stroke-style word; the empty string is the default
// solid, and an unrecognised word is a clear error.
func isoStyleFromStr(s string) (IsoConnectorStyle, error) {
	switch s {
	case "":
		return IsoSolid, nil
	case "dashed":
		return IsoDashed, nil
	case "dotted":
		return IsoDotted, nil
	default:
		return 0, fmt.Errorf("iso import: unknown connector style %q", s)
	}
}

// isoArrowToStr names a connector arrow head; the default (no head) writes the
// empty string (omitted).
func isoArrowToStr(a IsoArrow) string {
	switch a {
	case IsoArrowSingle:
		return "single"
	case IsoArrowDouble:
		return "double"
	default:
		return ""
	}
}

// isoArrowFromStr parses an arrow-head word; the empty string is the default
// (no head), and an unrecognised word is a clear error.
func isoArrowFromStr(s string) (IsoArrow, error) {
	switch s {
	case "":
		return IsoArrowNone, nil
	case "single":
		return IsoArrowSingle, nil
	case "double":
		return IsoArrowDouble, nil
	default:
		return 0, fmt.Errorf("iso import: unknown connector arrow %q", s)
	}
}

// --- entity <-> wire -----------------------------------------------------

// nodeToJSON projects a node onto its wire form.
func nodeToJSON(n IsoNode) isoNodeJSON {
	return isoNodeJSON{
		ID: n.ID, X: n.X, Y: n.Y,
		Shape: isoShapeToStr(n.Shape),
		Icon:  n.Icon, Label: n.Label,
		Color: isoColorToHex(n.Color), Layer: n.Layer,
	}
}

// jsonToNode reconstructs a node from its wire form, validating its shape and
// colour.
func jsonToNode(j isoNodeJSON) (IsoNode, error) {
	shape, err := isoShapeFromStr(j.Shape)
	if err != nil {
		return IsoNode{}, err
	}
	col, err := isoColorFromHex(j.Color)
	if err != nil {
		return IsoNode{}, err
	}
	return IsoNode{
		ID: j.ID, X: j.X, Y: j.Y, Shape: shape,
		Icon: j.Icon, Label: j.Label, Color: col, Layer: j.Layer,
	}, nil
}

// connectorToJSON projects a connector onto its wire form.
func connectorToJSON(c IsoConnector) isoConnectorJSON {
	return isoConnectorJSON{
		ID: c.ID, From: c.From, To: c.To, Label: c.Label,
		Style: isoStyleToStr(c.Style), Arrow: isoArrowToStr(c.Arrow),
		Color: isoColorToHex(c.Color), Width: c.Width,
		Routed: c.Routed, Layer: c.Layer,
	}
}

// jsonToConnector reconstructs a connector from its wire form, validating its
// style, arrow and colour. Endpoint existence is validated later, once the whole
// node set is known.
func jsonToConnector(j isoConnectorJSON) (IsoConnector, error) {
	style, err := isoStyleFromStr(j.Style)
	if err != nil {
		return IsoConnector{}, err
	}
	arrow, err := isoArrowFromStr(j.Arrow)
	if err != nil {
		return IsoConnector{}, err
	}
	col, err := isoColorFromHex(j.Color)
	if err != nil {
		return IsoConnector{}, err
	}
	return IsoConnector{
		ID: j.ID, From: j.From, To: j.To, Label: j.Label,
		Style: style, Arrow: arrow, Color: col, Width: j.Width,
		Routed: j.Routed, Layer: j.Layer,
	}, nil
}

// zoneToJSON projects a zone onto its wire form.
func zoneToJSON(z IsoZone) isoZoneJSON {
	return isoZoneJSON{
		ID: z.ID, X: z.X, Y: z.Y, W: z.W, H: z.H,
		Color: isoColorToHex(z.Color), Label: z.Label, Layer: z.Layer,
	}
}

// jsonToZone reconstructs a zone from its wire form, validating its colour.
func jsonToZone(j isoZoneJSON) (IsoZone, error) {
	col, err := isoColorFromHex(j.Color)
	if err != nil {
		return IsoZone{}, err
	}
	return IsoZone{
		ID: j.ID, X: j.X, Y: j.Y, W: j.W, H: j.H,
		Color: col, Label: j.Label, Layer: j.Layer,
	}, nil
}

// textToJSON projects a text annotation onto its wire form.
func textToJSON(t IsoText) isoTextJSON {
	return isoTextJSON{
		ID: t.ID, X: t.X, Y: t.Y, Text: t.Text,
		Color: isoColorToHex(t.Color), Size: t.Size, Layer: t.Layer,
	}
}

// jsonToText reconstructs a text annotation from its wire form, validating its
// colour.
func jsonToText(j isoTextJSON) (IsoText, error) {
	col, err := isoColorFromHex(j.Color)
	if err != nil {
		return IsoText{}, err
	}
	return IsoText{
		ID: j.ID, X: j.X, Y: j.Y, Text: j.Text,
		Color: col, Size: j.Size, Layer: j.Layer,
	}, nil
}

// layerToJSON projects a layer onto its wire form (no codec-validated fields).
func layerToJSON(l IsoLayer) isoLayerJSON {
	return isoLayerJSON{
		ID: l.ID, Name: l.Name, Visible: l.Visible,
		Locked: l.Locked, Order: l.Order,
	}
}

// jsonToLayer reconstructs a layer from its wire form.
func jsonToLayer(j isoLayerJSON) IsoLayer {
	return IsoLayer{
		ID: j.ID, Name: j.Name, Visible: j.Visible,
		Locked: j.Locked, Order: j.Order,
	}
}

// --- marshal ------------------------------------------------------------

// buildIsoFile assembles the wire envelope for a document: every entity family,
// each sorted by id for a stable, diff-friendly byte stream, plus the optional
// viewport hint.
func buildIsoFile(doc IsoDocument, vp *isoViewportJSON) isoDocJSON {
	layers := doc.Layers()
	sort.Slice(layers, func(i, j int) bool { return layers[i].ID < layers[j].ID })
	nodes := doc.Nodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	conns := doc.Connectors()
	sort.Slice(conns, func(i, j int) bool { return conns[i].ID < conns[j].ID })
	zones := doc.Zones()
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	texts := doc.Texts()
	sort.Slice(texts, func(i, j int) bool { return texts[i].ID < texts[j].ID })

	f := isoDocJSON{SchemaVersion: IsoSchemaVersion, Viewport: vp}
	for _, l := range layers {
		f.Layers = append(f.Layers, layerToJSON(l))
	}
	for _, n := range nodes {
		f.Nodes = append(f.Nodes, nodeToJSON(n))
	}
	for _, c := range conns {
		f.Connectors = append(f.Connectors, connectorToJSON(c))
	}
	for _, z := range zones {
		f.Zones = append(f.Zones, zoneToJSON(z))
	}
	for _, t := range texts {
		f.Texts = append(f.Texts, textToJSON(t))
	}
	return f
}

// encodeIsoFile serialises the envelope with a two-space indent and no HTML
// escaping (a native format, not a web payload), yielding a legible, stable,
// newline-terminated document.
func encodeIsoFile(doc IsoDocument, vp *isoViewportJSON) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	// The envelope holds only strings, ints, bools and slices of the same, so the
	// encoder cannot fail; the error is threaded through (rather than checked in an
	// unreachable branch) for API symmetry and to honour the [MarshalIsoDocument]
	// signature.
	err := enc.Encode(buildIsoFile(doc, vp))
	return buf.Bytes(), err
}

// MarshalIsoDocument serialises a whole [IsoDocument] — every node, connector,
// zone, text annotation and layer — to our native, versioned JSON schema. The
// output is deterministic (entities sorted by id, keys in a fixed order), so two
// runs on the same document produce identical bytes and a re-serialised import
// round-trips byte-for-byte. It carries no viewport: the grid extent and
// pan/zoom are view state, not part of the shared document.
func MarshalIsoDocument(doc IsoDocument) ([]byte, error) {
	return encodeIsoFile(doc, nil)
}

// ExportJSON serialises the widget's document to the native JSON schema (see
// [MarshalIsoDocument]) and additionally records a viewport hint — the widget's
// current grid extent — so a subsequent [IsoDiagram.ImportJSON] can restore the
// same canvas size.
func (d *IsoDiagram) ExportJSON() ([]byte, error) {
	return encodeIsoFile(d.doc, &isoViewportJSON{Cols: d.Cols, Rows: d.Rows})
}

// --- unmarshal ----------------------------------------------------------

// isoCheckID enforces that an entity id is present and unique within its family
// (an OR-map keyed by id cannot hold a blank or duplicated key). It records id
// in seen and returns a clear error otherwise.
func isoCheckID(seen map[string]struct{}, id, kind string) error {
	if id == "" {
		return fmt.Errorf("iso import: %s has an empty id", kind)
	}
	if _, dup := seen[id]; dup {
		return fmt.Errorf("iso import: duplicate %s id %q", kind, id)
	}
	seen[id] = struct{}{}
	return nil
}

// decodeIsoSnapshot parses a native JSON document into a whole-document snapshot
// (the same value type the undo stack uses) plus its optional viewport hint. It
// rejects, with a clear error and never a panic: malformed JSON, a schemaVersion
// this build does not read, a blank or duplicated id in any family, a
// malformed colour or unknown enum word, and a connector whose endpoint is not a
// node in the same document. A field a file omits (an older document with no
// "layer" keys, say) loads as its zero value.
func decodeIsoSnapshot(data []byte) (isoSnapshot, *isoViewportJSON, error) {
	var doc isoDocJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return isoSnapshot{}, nil, fmt.Errorf("iso import: %w", err)
	}
	if doc.SchemaVersion != IsoSchemaVersion {
		return isoSnapshot{}, nil, fmt.Errorf(
			"iso import: unsupported schemaVersion %d (this build reads %d)",
			doc.SchemaVersion, IsoSchemaVersion)
	}

	var s isoSnapshot
	nodeIDs := map[string]struct{}{}

	layerIDs := map[string]struct{}{}
	for _, lj := range doc.Layers {
		if err := isoCheckID(layerIDs, lj.ID, "layer"); err != nil {
			return isoSnapshot{}, nil, err
		}
		s.layers = append(s.layers, jsonToLayer(lj))
	}
	for _, nj := range doc.Nodes {
		if err := isoCheckID(nodeIDs, nj.ID, "node"); err != nil {
			return isoSnapshot{}, nil, err
		}
		n, err := jsonToNode(nj)
		if err != nil {
			return isoSnapshot{}, nil, err
		}
		s.nodes = append(s.nodes, n)
	}
	connIDs := map[string]struct{}{}
	for _, cj := range doc.Connectors {
		if err := isoCheckID(connIDs, cj.ID, "connector"); err != nil {
			return isoSnapshot{}, nil, err
		}
		c, err := jsonToConnector(cj)
		if err != nil {
			return isoSnapshot{}, nil, err
		}
		if _, ok := nodeIDs[c.From]; !ok {
			return isoSnapshot{}, nil, fmt.Errorf(
				"iso import: connector %q references missing node %q", c.ID, c.From)
		}
		if _, ok := nodeIDs[c.To]; !ok {
			return isoSnapshot{}, nil, fmt.Errorf(
				"iso import: connector %q references missing node %q", c.ID, c.To)
		}
		s.conns = append(s.conns, c)
	}
	zoneIDs := map[string]struct{}{}
	for _, zj := range doc.Zones {
		if err := isoCheckID(zoneIDs, zj.ID, "zone"); err != nil {
			return isoSnapshot{}, nil, err
		}
		z, err := jsonToZone(zj)
		if err != nil {
			return isoSnapshot{}, nil, err
		}
		s.zones = append(s.zones, z)
	}
	textIDs := map[string]struct{}{}
	for _, tj := range doc.Texts {
		if err := isoCheckID(textIDs, tj.ID, "text"); err != nil {
			return isoSnapshot{}, nil, err
		}
		t, err := jsonToText(tj)
		if err != nil {
			return isoSnapshot{}, nil, err
		}
		s.texts = append(s.texts, t)
	}
	return s, doc.Viewport, nil
}

// UnmarshalIsoDocument parses a native JSON document (see [MarshalIsoDocument])
// into a fresh [IsoDoc]. It validates the schema version, every id and every
// connector endpoint (see [decodeIsoSnapshot]); on any error it returns nil and
// the error, leaving no half-built document. The result is structurally equal to
// the document that produced the JSON, so Marshal → Unmarshal → Marshal is
// byte-identical.
func UnmarshalIsoDocument(data []byte) (*IsoDoc, error) {
	s, _, err := decodeIsoSnapshot(data)
	if err != nil {
		return nil, err
	}
	doc := NewIsoDoc()
	putSnapshot(doc, s)
	return doc, nil
}

// ImportJSON replaces the widget's document contents with the entities decoded
// from data, as ONE undoable command: a following [IsoDiagram.Undo] restores the
// document exactly as it was. The swap flows through the document's observable
// lists, so every bound view repaints. A viewport hint in the file, when
// present, also updates the widget's grid extent. On a decode error the document
// is left untouched (no undo entry, no repaint) and the error is returned.
func (d *IsoDiagram) ImportJSON(data []byte) error {
	s, vp, err := decodeIsoSnapshot(data)
	if err != nil {
		return err
	}
	d.beginEdit()
	d.restore(s)
	if vp != nil {
		d.Cols, d.Rows = vp.Cols, vp.Rows
	}
	d.invalidate()
	return nil
}

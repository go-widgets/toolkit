// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package isointerop

import (
	"strings"
	"testing"

	"github.com/go-widgets/toolkit"
)

// countNotes returns how many report notes contain sub.
func countNotes(rep ImportReport, sub string) int {
	n := 0
	for _, note := range rep.Notes {
		if strings.Contains(note, sub) {
			n++
		}
	}
	return n
}

// hasNote reports whether any report note contains sub.
func hasNote(rep ImportReport, sub string) bool { return countNotes(rep, sub) > 0 }

// --- unit: parseIsoflowColor ------------------------------------------------

func TestParseIsoflowColor(t *testing.T) {
	cases := []struct {
		in   string
		want toolkit.RGBA
		ok   bool
	}{
		{"", toolkit.RGBA{}, false},
		{"3366ff", toolkit.RGBA{}, false},  // no leading '#'
		{"#123", toolkit.RGBA{}, false},    // wrong length
		{"#zzzzzz", toolkit.RGBA{}, false}, // not hex
		{"#3366ff", toolkit.RGBA{R: 0x33, G: 0x66, B: 0xff, A: 0xff}, true},
		{"#11223344", toolkit.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 0x44}, true},
	}
	for _, c := range cases {
		got, ok := parseIsoflowColor(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseIsoflowColor(%q) = %v,%v; want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// --- unit: resolveIcon ------------------------------------------------------

func TestResolveIcon(t *testing.T) {
	icons := map[string]isoflowIcon{
		"srv":   {ID: "srv", Name: "Server"},
		"weird": {ID: "weird", Name: "Zztop"},
	}
	our, name, ok := resolveIcon("", icons)
	if our != "" || name != "" || !ok {
		t.Errorf("empty icon id: got %q,%q,%v", our, name, ok)
	}
	if our, name, ok = resolveIcon("missing", icons); our != "" || name != "" || ok {
		t.Errorf("dangling icon id: got %q,%q,%v", our, name, ok)
	}
	if our, name, ok = resolveIcon("srv", icons); our != "server" || name != "Server" || !ok {
		t.Errorf("named icon: got %q,%q,%v", our, name, ok)
	}
	if our, name, ok = resolveIcon("weird", icons); our != "" || name != "Zztop" || ok {
		t.Errorf("unmatched name: got %q,%q,%v", our, name, ok)
	}
}

// --- unit: helpers ----------------------------------------------------------

func TestQualifyAndAbs(t *testing.T) {
	if got := qualify("v1", "n", false); got != "n" {
		t.Errorf("qualify single = %q", got)
	}
	if got := qualify("v1", "n", true); got != "v1/n" {
		t.Errorf("qualify multi = %q", got)
	}
	if abs(-4) != 4 || abs(4) != 4 || abs(0) != 0 {
		t.Errorf("abs broken")
	}
	if roundInt(1.6) != 2 || roundInt(-1.6) != -2 {
		t.Errorf("roundInt broken")
	}
}

// --- unit: endpointNode -----------------------------------------------------

func TestEndpointNode(t *testing.T) {
	nodeID := map[string]string{"a": "a"}
	item := "a"
	if id, ok := endpointNode(isoflowAnchor{Ref: isoflowAnchorRef{Item: &item}}, nodeID); !ok || id != "a" {
		t.Errorf("item anchor: got %q,%v", id, ok)
	}
	nope := "nope"
	if _, ok := endpointNode(isoflowAnchor{Ref: isoflowAnchorRef{Item: &nope}}, nodeID); ok {
		t.Errorf("unknown item anchor should fail")
	}
	tile := isoflowTile{X: 1, Y: 1}
	if _, ok := endpointNode(isoflowAnchor{Ref: isoflowAnchorRef{Tile: &tile}}, nodeID); ok {
		t.Errorf("free-tile anchor should fail")
	}
}

// --- happy path + native round-trip ----------------------------------------

// wantNativeJSON is the exact native document [toolkit.MarshalIsoDocument] must
// produce from the clean single-view import below — the import -> native-export
// proof. Entities are sorted by id and default fields omitted by our codec.
const wantNativeJSON = `{
  "schemaVersion": 1,
  "nodes": [
    {
      "id": "it-db",
      "x": 3,
      "y": 4,
      "icon": "database",
      "label": "DB"
    },
    {
      "id": "it-web",
      "x": 1,
      "y": 2,
      "icon": "server",
      "label": "Web"
    }
  ],
  "connectors": [
    {
      "id": "cn1",
      "from": "it-web",
      "to": "it-db",
      "style": "dashed",
      "color": "#3366ffff",
      "width": 2
    }
  ],
  "zones": [
    {
      "id": "rc1",
      "x": 6,
      "y": 6,
      "w": 2,
      "h": 3,
      "color": "#00aa0055"
    }
  ],
  "texts": [
    {
      "id": "tb1",
      "x": 2,
      "y": 7,
      "text": "Note",
      "size": 2
    }
  ]
}
`

const cleanExport = `{
  "version": "1.0",
  "fitToScreen": true,
  "icons": [
    {"id": "web-icon", "name": "Server", "url": "x"},
    {"id": "db-icon", "name": "Database", "url": "y"}
  ],
  "colors": [
    {"id": "cblue", "value": "#3366ff"},
    {"id": "cgreen", "value": "#00aa00"}
  ],
  "items": [
    {"id": "it-web", "name": "Web", "icon": "web-icon"},
    {"id": "it-db", "name": "DB", "icon": "db-icon"}
  ],
  "views": [
    {
      "id": "main",
      "name": "Main",
      "items": [
        {"id": "it-web", "tile": {"x": 1, "y": 2}},
        {"id": "it-db", "tile": {"x": 3, "y": 4}}
      ],
      "connectors": [
        {
          "id": "cn1",
          "color": "cblue",
          "width": 2,
          "style": "DASHED",
          "anchors": [
            {"id": "a1", "ref": {"item": "it-web"}},
            {"id": "a2", "ref": {"item": "it-db"}}
          ]
        }
      ],
      "rectangles": [
        {"id": "rc1", "color": "cgreen", "from": {"x": 6, "y": 6}, "to": {"x": 8, "y": 9}}
      ],
      "textBoxes": [
        {"id": "tb1", "tile": {"x": 2, "y": 7}, "content": "Note", "fontSize": 1.6}
      ]
    }
  ]
}`

func TestImportCleanRoundTrip(t *testing.T) {
	doc, rep, err := ImportIsoflowJSON([]byte(cleanExport))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	// Report accounting: exactly what was imported, nothing dropped.
	if rep.Nodes != 2 || rep.Connectors != 1 || rep.Zones != 1 || rep.Texts != 1 || rep.Layers != 0 {
		t.Errorf("counts = %+v", rep)
	}
	if len(rep.Unmapped) != 0 || len(rep.UnresolvedIcons) != 0 || len(rep.Notes) != 0 {
		t.Errorf("clean import should be noiseless: %+v", rep)
	}

	// Toothed per-entity assertions.
	web, _ := doc.Node("it-web")
	if web != (toolkit.IsoNode{ID: "it-web", X: 1, Y: 2, Icon: "server", Label: "Web"}) {
		t.Errorf("web node = %+v", web)
	}
	db, _ := doc.Node("it-db")
	if db != (toolkit.IsoNode{ID: "it-db", X: 3, Y: 4, Icon: "database", Label: "DB"}) {
		t.Errorf("db node = %+v", db)
	}
	conns := doc.Connectors()
	if len(conns) != 1 {
		t.Fatalf("connectors = %d", len(conns))
	}
	if want := (toolkit.IsoConnector{ID: "cn1", From: "it-web", To: "it-db", Style: toolkit.IsoDashed, Color: toolkit.RGBA{R: 0x33, G: 0x66, B: 0xff, A: 0xff}, Width: 2}); conns[0] != want {
		t.Errorf("connector = %+v; want %+v", conns[0], want)
	}
	zones := doc.Zones()
	if want := (toolkit.IsoZone{ID: "rc1", X: 6, Y: 6, W: 2, H: 3, Color: toolkit.RGBA{R: 0x00, G: 0xaa, B: 0x00, A: defaultZoneAlpha}}); len(zones) != 1 || zones[0] != want {
		t.Errorf("zone = %+v; want %+v", zones, want)
	}
	texts := doc.Texts()
	if want := (toolkit.IsoText{ID: "tb1", X: 2, Y: 7, Text: "Note", Size: 2}); len(texts) != 1 || texts[0] != want {
		t.Errorf("text = %+v; want %+v", texts, want)
	}

	// import -> native-export proof: re-serialise through OUR native codec.
	out, err := toolkit.MarshalIsoDocument(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != wantNativeJSON {
		t.Errorf("native export mismatch:\n got: %q\nwant: %q", out, wantNativeJSON)
	}
}

// --- messy path: every report branch ---------------------------------------

const messyExport = `{
  "title": "My Diagram",
  "description": "notes here",
  "icons": [
    {"id": "rtr", "name": "Router"},
    {"id": "weird", "name": "Zztop"}
  ],
  "colors": [
    {"id": "cblue", "value": "#3366ff"},
    {"id": "cbad", "value": "nothex"}
  ],
  "items": [
    {"id": "known", "name": "K", "icon": "rtr"},
    {"id": "noicon", "name": "N", "icon": ""},
    {"id": "badicon", "name": "B", "icon": "missingicon"},
    {"id": "namedicon", "name": "Z", "icon": "weird"}
  ],
  "views": [
    {
      "id": "m",
      "name": "Messy",
      "items": [
        {"id": "known", "tile": {"x": 0, "y": 0}},
        {"id": "noicon", "tile": {"x": 1, "y": 1}},
        {"id": "badicon", "tile": {"x": 2, "y": 2}},
        {"id": "namedicon", "tile": {"x": 3, "y": 3}},
        {"id": "ghost", "tile": {"x": 4, "y": 4}}
      ],
      "connectors": [
        {"id": "c-ok", "color": "cblue", "style": "SOLID",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "noicon"}}]},
        {"id": "c-few", "anchors": [{"id": "1", "ref": {"item": "known"}}]},
        {"id": "c-tile",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"tile": {"x": 9, "y": 9}}}]},
        {"id": "c-unknownitem",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "nope"}}]},
        {"id": "c-wp",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"tile": {"x": 5, "y": 5}}}, {"id": "3", "ref": {"item": "noicon"}}]},
        {"id": "c-badcolor1", "color": "badcolorid",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "noicon"}}]},
        {"id": "c-badcolor2", "color": "badcolorid",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "noicon"}}]},
        {"id": "c-malformedcolor", "color": "cbad",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "noicon"}}]},
        {"id": "c-badstyle", "style": "WAVY",
         "anchors": [{"id": "1", "ref": {"item": "known"}}, {"id": "2", "ref": {"item": "noicon"}}]}
      ],
      "rectangles": [
        {"id": "r-normal", "color": "cblue", "from": {"x": 0, "y": 0}, "to": {"x": 2, "y": 2}},
        {"id": "r-inverted", "from": {"x": 5, "y": 5}, "to": {"x": 3, "y": 3}},
        {"id": "r-8col", "color": "calpha", "from": {"x": 0, "y": 0}, "to": {"x": 0, "y": 0}}
      ],
      "textBoxes": [
        {"id": "t-small", "tile": {"x": 0, "y": 0}, "content": "s", "fontSize": 0.5},
        {"id": "t-big", "tile": {"x": 1, "y": 1}, "content": "b", "fontSize": 2, "orientation": "X"}
      ]
    }
  ]
}`

func TestImportMessyReport(t *testing.T) {
	// Add the 8-digit palette entry referenced by r-8col.
	src := strings.Replace(messyExport,
		`{"id": "cbad", "value": "nothex"}`,
		`{"id": "cbad", "value": "nothex"},
    {"id": "calpha", "value": "#11223344"}`, 1)

	doc, rep, err := ImportIsoflowJSON([]byte(src))
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	if rep.Nodes != 5 || rep.Zones != 3 || rep.Texts != 2 || rep.Layers != 0 {
		t.Errorf("counts = %+v", rep)
	}
	// Six connectors mapped (c-few, c-tile, c-unknownitem dropped).
	if rep.Connectors != 6 {
		t.Errorf("connectors = %d, want 6", rep.Connectors)
	}
	if len(rep.Unmapped) != 3 {
		t.Errorf("unmapped = %+v", rep.Unmapped)
	}
	// Kinds + reasons of the unmapped connectors.
	byID := map[string]UnmappedElement{}
	for _, u := range rep.Unmapped {
		if u.Kind != "connector" {
			t.Errorf("unexpected unmapped kind %q", u.Kind)
		}
		byID[u.ID] = u
	}
	if !strings.Contains(byID["c-few"].Reason, "fewer than two") {
		t.Errorf("c-few reason = %q", byID["c-few"].Reason)
	}
	if !strings.Contains(byID["c-tile"].Reason, "free tile") {
		t.Errorf("c-tile reason = %q", byID["c-tile"].Reason)
	}
	if !strings.Contains(byID["c-unknownitem"].Reason, "free tile") {
		t.Errorf("c-unknownitem reason = %q", byID["c-unknownitem"].Reason)
	}

	// Unresolved icons: badicon (dangling id, no name) + namedicon (name, no synonym).
	if len(rep.UnresolvedIcons) != 2 {
		t.Fatalf("unresolved icons = %+v", rep.UnresolvedIcons)
	}
	ui := map[string]UnresolvedIcon{}
	for _, u := range rep.UnresolvedIcons {
		ui[u.NodeID] = u
	}
	if ui["badicon"].IconID != "missingicon" || ui["badicon"].IconName != "" {
		t.Errorf("badicon unresolved = %+v", ui["badicon"])
	}
	if ui["namedicon"].IconID != "weird" || ui["namedicon"].IconName != "Zztop" {
		t.Errorf("namedicon unresolved = %+v", ui["namedicon"])
	}

	// Notes: metadata, unknown model item, waypoint, colour issues (deduped),
	// unknown style, dropped orientation.
	for _, sub := range []string{
		"title", "description", "unknown model item", "waypoint",
		"unknown style", "orientation",
	} {
		if !hasNote(rep, sub) {
			t.Errorf("missing note containing %q; notes=%v", sub, rep.Notes)
		}
	}
	// The same bad colour id is referenced twice but noted once (seen-dedup).
	if n := countNotes(rep, "badcolorid"); n != 1 {
		t.Errorf("badcolorid noted %d times, want 1", n)
	}
	if n := countNotes(rep, `"cbad"`); n != 1 {
		t.Errorf("malformed colour cbad noted %d times, want 1", n)
	}

	// Toothed checks on a few native results.
	known, _ := doc.Node("known")
	if known.Icon != "router" || known.Label != "K" {
		t.Errorf("known node = %+v", known)
	}
	bad, _ := doc.Node("badicon")
	if bad.Icon != "" { // fell back
		t.Errorf("badicon should have empty icon, got %q", bad.Icon)
	}
	ghost, _ := doc.Node("ghost")
	if ghost.Label != "" || ghost.Icon != "" {
		t.Errorf("ghost (no model item) = %+v", ghost)
	}
	cok := connByID(doc, "c-ok")
	if cok.Style != toolkit.IsoSolid || cok.Color.A != 0xff {
		t.Errorf("c-ok = %+v", cok)
	}
	cwp := connByID(doc, "c-wp")
	if cwp.From != "known" || cwp.To != "noicon" {
		t.Errorf("c-wp endpoints = %+v", cwp)
	}
	// r-inverted: corners swapped, so min corner (3,3), size 2x2.
	rinv, _ := doc.Zone("r-inverted")
	if rinv.X != 3 || rinv.Y != 3 || rinv.W != 2 || rinv.H != 2 {
		t.Errorf("r-inverted = %+v", rinv)
	}
	// r-8col: zero-size rectangle clamps to 1x1, explicit alpha kept.
	r8, _ := doc.Zone("r-8col")
	if r8.W != 1 || r8.H != 1 || r8.Color.A != 0x44 {
		t.Errorf("r-8col = %+v", r8)
	}
	// r-normal: opaque fill dimmed to the translucent zone tint.
	rn, _ := doc.Zone("r-normal")
	if rn.Color.A != defaultZoneAlpha {
		t.Errorf("r-normal alpha = %#x", rn.Color.A)
	}
	tsmall, _ := doc.Text("t-small")
	if tsmall.Size != 0 { // fontSize < 1 -> default
		t.Errorf("t-small size = %d", tsmall.Size)
	}
	tbig, _ := doc.Text("t-big")
	if tbig.Size != 2 {
		t.Errorf("t-big size = %d", tbig.Size)
	}
}

// connByID returns the connector with id from doc (test helper).
func connByID(doc *toolkit.IsoDoc, id string) toolkit.IsoConnector {
	for _, c := range doc.Connectors() {
		if c.ID == id {
			return c
		}
	}
	return toolkit.IsoConnector{}
}

// --- multi-view: layers + id namespacing ------------------------------------

const multiViewExport = `{
  "items": [
    {"id": "a", "name": "A", "icon": ""},
    {"id": "b", "name": "B", "icon": ""}
  ],
  "views": [
    {
      "id": "v1", "name": "First",
      "items": [
        {"id": "a", "tile": {"x": 0, "y": 0}},
        {"id": "b", "tile": {"x": 1, "y": 1}}
      ],
      "connectors": [
        {"id": "cn", "anchors": [{"id": "1", "ref": {"item": "a"}}, {"id": "2", "ref": {"item": "b"}}]}
      ]
    },
    {
      "id": "v2", "name": "Second",
      "items": [
        {"id": "a", "tile": {"x": 5, "y": 5}}
      ]
    }
  ]
}`

func TestImportMultiViewLayers(t *testing.T) {
	doc, rep, err := ImportIsoflowJSON([]byte(multiViewExport))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if rep.Layers != 2 || rep.Nodes != 3 || rep.Connectors != 1 {
		t.Errorf("counts = %+v", rep)
	}
	// Two layers, one per view, ordered by view index.
	l1, ok1 := doc.Layer("v1")
	l2, ok2 := doc.Layer("v2")
	if !ok1 || !ok2 {
		t.Fatalf("layers missing: %v %v", ok1, ok2)
	}
	if l1.Name != "First" || !l1.Visible || l1.Order != 0 {
		t.Errorf("layer v1 = %+v", l1)
	}
	if l2.Order != 1 {
		t.Errorf("layer v2 order = %d", l2.Order)
	}
	// Node ids are namespaced by view so the shared item id "a" does not collide.
	na, okA := doc.Node("v1/a")
	if !okA || na.Layer != "v1" || na.X != 0 {
		t.Errorf("v1/a = %+v (ok=%v)", na, okA)
	}
	nb, okB := doc.Node("v2/a")
	if !okB || nb.Layer != "v2" || nb.X != 5 {
		t.Errorf("v2/a = %+v (ok=%v)", nb, okB)
	}
	// Connector id + endpoints are namespaced to its view.
	c := connByID(doc, "v1/cn")
	if c.From != "v1/a" || c.To != "v1/b" || c.Layer != "v1" {
		t.Errorf("connector = %+v", c)
	}
}

// --- empty / no-view input --------------------------------------------------

func TestImportNoViews(t *testing.T) {
	doc, rep, err := ImportIsoflowJSON([]byte(`{"title": "T"}`))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(doc.Nodes()) != 0 || rep.Nodes != 0 || rep.Layers != 0 {
		t.Errorf("expected empty doc, got %+v", rep)
	}
	if !hasNote(rep, "title") {
		t.Errorf("title should be noted, notes=%v", rep.Notes)
	}
}

// --- error branches ---------------------------------------------------------

func TestImportErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"malformed json", `{`, "isoflow import"},
		{"wrong type", `{"views": "not-an-array"}`, "isoflow import"},
		{"empty view id", `{"views": [{"id": ""}]}`, "empty id"},
		{"empty view-item id", `{"views": [{"id": "v", "items": [{"id": ""}]}]}`, "view item with an empty id"},
		{"empty connector id", `{"views": [{"id": "v", "connectors": [{"id": ""}]}]}`, "connector with an empty id"},
		{"empty rectangle id", `{"views": [{"id": "v", "rectangles": [{"id": ""}]}]}`, "rectangle with an empty id"},
		{"empty textbox id", `{"views": [{"id": "v", "textBoxes": [{"id": ""}]}]}`, "text box with an empty id"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc, _, err := ImportIsoflowJSON([]byte(c.in))
			if err == nil {
				t.Fatalf("expected error")
			}
			if doc != nil {
				t.Errorf("doc should be nil on error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err, c.want)
			}
		})
	}
}

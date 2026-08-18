// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package isointerop is an OPTIONAL, SEPARATE interoperability layer for the
// isometric diagram widget ([toolkit.IsoDiagram]). It reads the PUBLIC JSON
// export of the third-party isoflow / FossFLOW isometric editor and TRANSLATES
// it into our own native document model ([toolkit.IsoDoc]).
//
// It is deliberately not part of the toolkit's native schema and never alters
// it: importing the core toolkit pulls in nothing from here, and this package
// only ever CONSTRUCTS native entities and hands them to the root package's own
// codec ([toolkit.MarshalIsoDocument]) — it does not reimplement the native
// "#rrggbbaa"/enum encoding, and it copies no isoflow code. The structs under
// "external isoflow export format" below exist solely to DECODE isoflow's
// documented JSON; they mirror that public format, not our model, and are
// clearly fenced off from our native types.
//
// The importer never panics: malformed JSON, a wrong-typed field or a missing
// required id is a returned error; anything that has no faithful native
// equivalent (a connector anchored to a free tile, a dangling reference, an
// icon with no counterpart in our registry, dropped metadata) is recorded in
// the returned [ImportReport] rather than silently lost.
package isointerop

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/go-widgets/toolkit"
)

// --- external isoflow / FossFLOW export format (NOT our schema) --------------
//
// The following types decode the public isoflow "InitialData" / model export
// (see https://isoflow.io/docs/api/initialData). Field names and nesting mirror
// THAT format exactly so encoding/json can populate them; none of it is our
// native model. isoflow separates reusable model "items" (definitions) from
// per-"view" placements, references colours and icons by id, and stores numeric
// coordinates as JSON numbers — so coordinates decode as float64 and are
// rounded to our integer grid.

// isoflowModel is the top-level isoflow export envelope.
type isoflowModel struct {
	Version     string         `json:"version"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Icons       []isoflowIcon  `json:"icons"`
	Colors      []isoflowColor `json:"colors"`
	Items       []isoflowItem  `json:"items"`
	Views       []isoflowView  `json:"views"`
	FitToScreen bool           `json:"fitToScreen"`
}

// isoflowIcon is a reusable icon definition referenced by a model item's Icon.
type isoflowIcon struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Collection  string `json:"collection"`
	IsIsometric bool   `json:"isIsometric"`
}

// isoflowColor is a palette entry; connectors and rectangles reference one by id.
type isoflowColor struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// isoflowItem is a model item (a node definition): a label plus an icon id. Its
// placement (grid position) lives in a view's items, not here.
type isoflowItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// isoflowView is one diagram page: placed items plus connectors, rectangles and
// text boxes drawn between/around them.
type isoflowView struct {
	ID         string             `json:"id"`
	Name       string             `json:"name"`
	Items      []isoflowViewItem  `json:"items"`
	Connectors []isoflowConnector `json:"connectors"`
	Rectangles []isoflowRectangle `json:"rectangles"`
	TextBoxes  []isoflowTextBox   `json:"textBoxes"`
}

// isoflowTile is an integer-ish grid coordinate (decoded loosely as a number).
type isoflowTile struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// isoflowViewItem places a model item (referenced by the shared id) on the grid.
type isoflowViewItem struct {
	ID          string      `json:"id"`
	LabelHeight float64     `json:"labelHeight"`
	Tile        isoflowTile `json:"tile"`
}

// isoflowConnector links anchors; the first and last anchor are its endpoints.
type isoflowConnector struct {
	ID      string          `json:"id"`
	Color   string          `json:"color"`
	Width   float64         `json:"width"`
	Style   string          `json:"style"`
	Anchors []isoflowAnchor `json:"anchors"`
}

// isoflowAnchor is one waypoint of a connector; Ref points at an item or a tile.
type isoflowAnchor struct {
	ID  string           `json:"id"`
	Ref isoflowAnchorRef `json:"ref"`
}

// isoflowAnchorRef is the isoflow anchor union: exactly one of Item / Tile is
// set. An item anchor binds to a placed node; a tile anchor floats free.
type isoflowAnchorRef struct {
	Item *string      `json:"item"`
	Tile *isoflowTile `json:"tile"`
}

// isoflowRectangle is a coloured area given by two opposite grid corners.
type isoflowRectangle struct {
	ID    string      `json:"id"`
	Color string      `json:"color"`
	From  isoflowTile `json:"from"`
	To    isoflowTile `json:"to"`
}

// isoflowTextBox is a floating text annotation at a tile.
type isoflowTextBox struct {
	ID          string      `json:"id"`
	Tile        isoflowTile `json:"tile"`
	Content     string      `json:"content"`
	FontSize    float64     `json:"fontSize"`
	Orientation string      `json:"orientation"`
}

// --- import report ----------------------------------------------------------

// ImportReport is the accounting of one import: how many native entities of each
// family were produced, plus every isoflow element that could not be mapped
// faithfully, so nothing is lost silently.
type ImportReport struct {
	// Nodes, Connectors, Zones, Texts and Layers count the native entities the
	// import produced.
	Nodes, Connectors, Zones, Texts, Layers int
	// Unmapped lists isoflow elements dropped because our model has no faithful
	// equivalent (e.g. a connector anchored to a free tile), each with a reason.
	Unmapped []UnmappedElement
	// UnresolvedIcons lists nodes whose isoflow icon had no counterpart in our
	// icon registry; each such node imported with the fallback shape.
	UnresolvedIcons []UnresolvedIcon
	// Notes records lossy-but-acceptable mappings and ignored metadata (dropped
	// connector waypoints, approximate text sizes, ignored orientation, top-level
	// title/description, and so on).
	Notes []string
}

// UnmappedElement is one isoflow element the import could not represent.
type UnmappedElement struct {
	// Kind is the isoflow element family ("connector", "view-item", ...).
	Kind string
	// ID is the element's isoflow id.
	ID string
	// Reason explains why it was dropped.
	Reason string
}

// UnresolvedIcon records a node whose isoflow icon had no registry counterpart.
type UnresolvedIcon struct {
	// NodeID is the native node that fell back to the default shape.
	NodeID string
	// IconID is the isoflow icon id the model item referenced.
	IconID string
	// IconName is the isoflow icon's human name, or empty when the icon id itself
	// was dangling (not present in the export's icons list).
	IconName string
}

// --- icon name mapping ------------------------------------------------------

// iconSynonyms maps a lowercased isoflow icon NAME to the id of the closest
// built-in icon in our registry ([toolkit.IsoBuiltinIconIDs]). It is a
// best-effort human-name match, not a copy of any isoflow icon pack: only the
// names are compared, never any artwork. An unmatched name resolves to the
// fallback shape and is reported.
var iconSynonyms = map[string]string{
	"server":    "server",
	"servers":   "server",
	"compute":   "server",
	"vm":        "server",
	"host":      "server",
	"rack":      "rack",
	"cloud":     "cloud",
	"internet":  "cloud",
	"database":  "database",
	"db":        "database",
	"sql":       "database",
	"datastore": "database",
	"router":    "router",
	"gateway":   "router",
	"switch":    "switch",
	"storage":   "storage",
	"disk":      "storage",
	"volume":    "storage",
	"bucket":    "storage",
	"user":      "user",
	"person":    "user",
	"actor":     "user",
	"client":    "user",
	"box":       "box",
	"cube":      "box",
	"block":     "box",
	"generic":   "box",
}

// resolveIcon maps a model item's icon id to one of our registry icon ids.
//
//   - An empty icon id means the item has no icon: nothing to resolve, the node
//     keeps the default shape (ok=true, our="").
//   - A dangling icon id (absent from icons) resolves to nothing and is reported
//     (ok=false, name="").
//   - A known icon whose name has no synonym resolves to nothing and is reported
//     (ok=false, name=<the isoflow name>).
//   - Otherwise our is the matched registry id (ok=true).
func resolveIcon(iconID string, icons map[string]isoflowIcon) (our, name string, ok bool) {
	if iconID == "" {
		return "", "", true
	}
	ic, found := icons[iconID]
	if !found {
		return "", "", false
	}
	if id, has := iconSynonyms[strings.ToLower(ic.Name)]; has {
		return id, ic.Name, true
	}
	return "", ic.Name, false
}

// --- colour codec (isoflow's own hex format, NOT our native codec) ----------

// parseIsoflowColor parses an isoflow palette value ("#rrggbb" or "#rrggbbaa")
// into a native colour. A 6-digit value is taken as fully opaque. It reports ok
// = false for anything malformed, so a bad palette entry becomes a report note
// rather than a failed import. This decodes ISOFLOW's format; the native
// "#rrggbbaa" codec is reused untouched on the way back out via
// [toolkit.MarshalIsoDocument].
func parseIsoflowColor(s string) (c toolkit.RGBA, ok bool) {
	if len(s) == 0 || s[0] != '#' {
		return toolkit.RGBA{}, false
	}
	body := s[1:]
	if len(body) != 6 && len(body) != 8 {
		return toolkit.RGBA{}, false
	}
	b, err := hex.DecodeString(body)
	if err != nil {
		return toolkit.RGBA{}, false
	}
	c = toolkit.RGBA{R: b[0], G: b[1], B: b[2], A: 0xff}
	if len(b) == 4 {
		c.A = b[3]
	}
	return c, true
}

// --- style mapping ----------------------------------------------------------

// connectorStyles maps an isoflow connector style word to our native style enum.
var connectorStyles = map[string]toolkit.IsoConnectorStyle{
	"SOLID":  toolkit.IsoSolid,
	"DASHED": toolkit.IsoDashed,
	"DOTTED": toolkit.IsoDotted,
}

// --- helpers ----------------------------------------------------------------

// roundInt rounds an isoflow numeric coordinate to our integer grid.
func roundInt(f float64) int { return int(math.Round(f)) }

// qualify namespaces a per-view element id so ids from different views cannot
// collide in the single native OR-map. With one view (multi=false) the id is
// left bare, keeping the common single-view import clean.
func qualify(viewID, id string, multi bool) string {
	if !multi {
		return id
	}
	return viewID + "/" + id
}

// defaultZoneAlpha is the fill opacity applied to a rectangle colour that isoflow
// gave as an opaque "#rrggbb": our zones tint the ground (alpha is opacity), so
// an opaque fill would hide the grid. An explicit 8-digit alpha is kept as-is.
const defaultZoneAlpha = 0x55

// --- import -----------------------------------------------------------------

// ImportIsoflowJSON parses the public isoflow / FossFLOW JSON export in data and
// builds an equivalent native [toolkit.IsoDoc], returning it alongside an
// [ImportReport] that accounts for every imported entity and every element that
// could not be mapped faithfully.
//
// Mapping (family by family):
//
//   - view item  -> IsoNode   (tile -> X,Y; model item name -> Label; model
//     item icon -> Icon via the registry, unresolved ones reported)
//   - connector  -> IsoConnector (first/last anchor items -> From/To; style,
//     width and palette colour carried over; intermediate waypoints dropped
//     with a note; a free-tile or dangling anchor drops the connector)
//   - rectangle  -> IsoZone   (from/to corners -> X,Y,W,H; palette colour, an
//     opaque one made translucent so it tints rather than hides the grid)
//   - text box   -> IsoText   (tile -> X,Y; content -> Text; fontSize -> Size
//     approximately; orientation noted as dropped)
//   - view       -> IsoLayer  only when the export has more than one view; a
//     single-view export lands on the implicit default layer
//
// Top-level title/description/version/fitToScreen have no document equivalent
// and are noted as ignored. On malformed JSON, a wrong-typed field or a missing
// required id it returns a nil document and a clear error (never a panic).
func ImportIsoflowJSON(data []byte) (*toolkit.IsoDoc, ImportReport, error) {
	var model isoflowModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, ImportReport{}, fmt.Errorf("isoflow import: %w", err)
	}

	var rep ImportReport

	// Index the top-level definition tables once (shared across every view).
	icons := make(map[string]isoflowIcon, len(model.Icons))
	for _, ic := range model.Icons {
		icons[ic.ID] = ic
	}
	colors := make(map[string]isoflowColor, len(model.Colors))
	for _, cl := range model.Colors {
		colors[cl.ID] = cl
	}
	items := make(map[string]isoflowItem, len(model.Items))
	for _, it := range model.Items {
		items[it.ID] = it
	}

	if t := strings.TrimSpace(model.Title); t != "" {
		rep.Notes = append(rep.Notes, fmt.Sprintf("ignored top-level title %q (no document equivalent)", t))
	}
	if model.Description != "" {
		rep.Notes = append(rep.Notes, "ignored top-level description (no document equivalent)")
	}

	doc := toolkit.NewIsoDoc()
	multi := len(model.Views) > 1

	for vi, view := range model.Views {
		if view.ID == "" {
			return nil, ImportReport{}, fmt.Errorf("isoflow import: view #%d has an empty id", vi)
		}
		if multi {
			doc.PutLayer(toolkit.IsoLayer{
				ID:      view.ID,
				Name:    view.Name,
				Visible: true,
				Order:   vi,
			})
			rep.Layers++
		}
		layer := ""
		if multi {
			layer = view.ID
		}

		// The colour resolver, shared by this view's connectors and rectangles,
		// records a note the first time a given palette id fails to resolve.
		noted := map[string]struct{}{}
		resolveColor := func(id string) (toolkit.RGBA, bool) {
			if id == "" {
				return toolkit.RGBA{}, false
			}
			cl, found := colors[id]
			if found {
				if c, ok := parseIsoflowColor(cl.Value); ok {
					return c, true
				}
			}
			if _, seen := noted[id]; !seen {
				noted[id] = struct{}{}
				rep.Notes = append(rep.Notes, fmt.Sprintf("view %q: unresolved colour id %q (left theme default)", view.ID, id))
			}
			return toolkit.RGBA{}, false
		}

		// Place items first: connectors resolve their endpoints against this map
		// (isoflow item ids -> the native node id we minted).
		nodeID := map[string]string{}
		for _, it := range view.Items {
			if it.ID == "" {
				return nil, ImportReport{}, fmt.Errorf("isoflow import: view %q has a view item with an empty id", view.ID)
			}
			nid := qualify(view.ID, it.ID, multi)
			nodeID[it.ID] = nid

			n := toolkit.IsoNode{ID: nid, X: roundInt(it.Tile.X), Y: roundInt(it.Tile.Y), Layer: layer}
			if def, ok := items[it.ID]; ok {
				n.Label = def.Name
				our, name, resolved := resolveIcon(def.Icon, icons)
				if resolved {
					n.Icon = our
				} else {
					rep.UnresolvedIcons = append(rep.UnresolvedIcons, UnresolvedIcon{
						NodeID: nid, IconID: def.Icon, IconName: name,
					})
				}
			} else {
				rep.Notes = append(rep.Notes, fmt.Sprintf("view item %q references unknown model item; imported without label/icon", it.ID))
			}
			doc.PutNode(n)
			rep.Nodes++
		}

		for _, cn := range view.Connectors {
			if cn.ID == "" {
				return nil, ImportReport{}, fmt.Errorf("isoflow import: view %q has a connector with an empty id", view.ID)
			}
			cid := qualify(view.ID, cn.ID, multi)
			if len(cn.Anchors) < 2 {
				rep.Unmapped = append(rep.Unmapped, UnmappedElement{
					Kind: "connector", ID: cid,
					Reason: "connector has fewer than two anchors",
				})
				continue
			}
			from, okFrom := endpointNode(cn.Anchors[0], nodeID)
			to, okTo := endpointNode(cn.Anchors[len(cn.Anchors)-1], nodeID)
			if !okFrom || !okTo {
				rep.Unmapped = append(rep.Unmapped, UnmappedElement{
					Kind: "connector", ID: cid,
					Reason: "an endpoint anchor is a free tile or an unknown item (our connectors link nodes)",
				})
				continue
			}
			if len(cn.Anchors) > 2 {
				rep.Notes = append(rep.Notes, fmt.Sprintf("connector %q: %d intermediate waypoint(s) dropped (native route is computed)", cid, len(cn.Anchors)-2))
			}
			c := toolkit.IsoConnector{ID: cid, From: from, To: to, Width: roundInt(cn.Width), Layer: layer}
			if st, ok := connectorStyles[cn.Style]; ok {
				c.Style = st
			} else if cn.Style != "" {
				rep.Notes = append(rep.Notes, fmt.Sprintf("connector %q: unknown style %q (drawn solid)", cid, cn.Style))
			}
			if col, ok := resolveColor(cn.Color); ok {
				c.Color = col
			}
			doc.PutConnector(c)
			rep.Connectors++
		}

		for _, rc := range view.Rectangles {
			if rc.ID == "" {
				return nil, ImportReport{}, fmt.Errorf("isoflow import: view %q has a rectangle with an empty id", view.ID)
			}
			rid := qualify(view.ID, rc.ID, multi)
			x0, y0 := roundInt(rc.From.X), roundInt(rc.From.Y)
			x1, y1 := roundInt(rc.To.X), roundInt(rc.To.Y)
			z := toolkit.IsoZone{
				ID:    rid,
				X:     min(x0, x1),
				Y:     min(y0, y1),
				W:     max(1, abs(x1-x0)),
				H:     max(1, abs(y1-y0)),
				Layer: layer,
			}
			if col, ok := resolveColor(rc.Color); ok {
				// isoflow rectangle fills are opaque; our zones tint the ground
				// (alpha is opacity), so an opaque fill would hide the grid. Dim a
				// fully opaque colour to a translucent tint; an explicit non-opaque
				// palette alpha is kept as authored.
				if col.A == 0xff {
					col.A = defaultZoneAlpha
				}
				z.Color = col
			}
			doc.PutZone(z)
			rep.Zones++
		}

		for _, tb := range view.TextBoxes {
			if tb.ID == "" {
				return nil, ImportReport{}, fmt.Errorf("isoflow import: view %q has a text box with an empty id", view.ID)
			}
			tid := qualify(view.ID, tb.ID, multi)
			t := toolkit.IsoText{ID: tid, X: roundInt(tb.Tile.X), Y: roundInt(tb.Tile.Y), Text: tb.Content, Layer: layer}
			if tb.FontSize >= 1 {
				t.Size = roundInt(tb.FontSize)
			}
			if tb.Orientation != "" {
				rep.Notes = append(rep.Notes, fmt.Sprintf("text box %q: dropped orientation %q (no native equivalent)", tid, tb.Orientation))
			}
			doc.PutText(t)
			rep.Texts++
		}
	}

	return doc, rep, nil
}

// endpointNode resolves a connector endpoint anchor to a native node id. It
// succeeds only for an item anchor that names a placed node; a free-tile anchor,
// an anchor with no ref, or an item anchor to an unknown node all fail (our
// connectors link nodes, not points).
func endpointNode(a isoflowAnchor, nodeID map[string]string) (string, bool) {
	if a.Ref.Item == nil {
		return "", false
	}
	nid, ok := nodeID[*a.Ref.Item]
	return nid, ok
}

// abs is the integer absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

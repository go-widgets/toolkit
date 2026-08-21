// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-richdoc/richdoc"
)

// block0Line returns block 0's caret line and the editor laid out at w*h.
func block0Line(t *testing.T, doc *richdoc.Document, w, h int) (*RichEditor, reLine) {
	t.Helper()
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	lay := e.buildLayout(DefaultLight())
	for _, l := range lay.lines {
		if l.blockIdx == 0 && l.hasStops {
			return e, l
		}
	}
	t.Fatal("no caret line for block 0")
	return e, reLine{}
}

// findRun returns the first run on ln whose text equals want.
func findRun(ln reLine, want string) (reRun, bool) {
	for _, r := range ln.runs {
		if r.text == want {
			return r, true
		}
	}
	return reRun{}, false
}

// anchorIDs / crossRefs collect the reference nodes of a document by a walk.
func anchorIDs(d *richdoc.Document) []string {
	var ids []string
	v := &refCollector{onAnchor: func(a richdoc.Anchor) { ids = append(ids, a.ID) }}
	richdoc.Walk(d, v)
	return ids
}

func crossRefs(d *richdoc.Document) []richdoc.CrossRef {
	var xs []richdoc.CrossRef
	v := &refCollector{onXRef: func(x richdoc.CrossRef) { xs = append(xs, x) }}
	richdoc.Walk(d, v)
	return xs
}

func footnotes(d *richdoc.Document) []richdoc.Footnote {
	var fs []richdoc.Footnote
	v := &refCollector{onFoot: func(f richdoc.Footnote) { fs = append(fs, f) }}
	richdoc.Walk(d, v)
	return fs
}

type refCollector struct {
	onAnchor func(richdoc.Anchor)
	onXRef   func(richdoc.CrossRef)
	onFoot   func(richdoc.Footnote)
}

func (c *refCollector) Enter(n any) bool {
	switch v := n.(type) {
	case richdoc.Anchor:
		if c.onAnchor != nil {
			c.onAnchor(v)
		}
	case richdoc.CrossRef:
		if c.onXRef != nil {
			c.onXRef(v)
		}
	case richdoc.Footnote:
		if c.onFoot != nil {
			c.onFoot(v)
		}
	}
	return true
}
func (c *refCollector) Leave(any) {}

// --- Footnote -------------------------------------------------------------

func TestFootnoteSuperscriptMarker(t *testing.T) {
	doc := richdoc.New().P(
		richdoc.Txt("ab"),
		richdoc.Note(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("first note"), richdoc.Txt(" tail")}}),
		richdoc.Txt("cd"),
	).Doc()
	e, ln := block0Line(t, doc, 200, 100)

	// Cells are a b [footnote] c d — the footnote is exactly one caret cell.
	if ln.nCells() != 5 {
		t.Fatalf("cells = %d, want 5 (a b footnote c d)", ln.nCells())
	}
	markW := e.superscriptFont().Measure("1")
	if got := ln.cellX[3] - ln.cellX[2]; got != markW {
		t.Errorf("footnote cell width = %d, want superscript '1' width %d", got, markW)
	}

	// The marker paints as a raised, accent, superscript-face "1" at the cell x.
	mk, ok := findRun(ln, "1")
	if !ok {
		t.Fatal("no footnote marker run")
	}
	if mk.dy >= 0 {
		t.Errorf("marker dy = %d, want raised (< 0)", mk.dy)
	}
	if mk.dy != -e.footnoteRise() {
		t.Errorf("marker dy = %d, want -footnoteRise %d", mk.dy, -e.footnoteRise())
	}
	if mk.font != e.superscriptFont() {
		t.Errorf("marker font = %T, want the superscript face", mk.font)
	}
	if mk.ink != DefaultLight().Accent {
		t.Errorf("marker ink = %v, want accent", mk.ink)
	}
	if mk.x != ln.cellX[2] {
		t.Errorf("marker x = %d, want cell-2 gap %d", mk.x, ln.cellX[2])
	}

	// The caret steps over the marker as a single cell.
	e.Caret().Set(DocPos{0, 2})
	e.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if e.Caret().Get() != (DocPos{0, 3}) {
		t.Errorf("ArrowRight over footnote = %+v, want {0,3}", e.Caret().Get())
	}

	// Painting the doc exercises drawRun's dy path; the marker must not panic.
	_ = reRender(e, 200, 100, DefaultLight())

	// The footnote (multi-paragraph-capable body) survives an edit round-trip: an
	// edit to the same paragraph rebuilds it unchanged.
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	fs := footnotes(e.Document())
	if len(fs) != 1 || richdoc.PlainText(&richdoc.Document{Blocks: fs[0].Blocks}) != "first note tail" {
		t.Errorf("footnote after edit = %+v, want body %q", fs, "first note tail")
	}
}

func TestFootnotesNumberInDocumentOrder(t *testing.T) {
	doc := richdoc.New().P(
		richdoc.Txt("x"),
		richdoc.Note(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("n1")}}),
		richdoc.Txt("y"),
		richdoc.Note(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("n2")}}),
	).Doc()
	_, ln := block0Line(t, doc, 200, 100)
	if _, ok := findRun(ln, "1"); !ok {
		t.Error("first footnote not marked 1")
	}
	if _, ok := findRun(ln, "2"); !ok {
		t.Error("second footnote not marked 2")
	}
}

func TestSuperscriptFontShrinksTTFAndCaches(t *testing.T) {
	e := &RichEditor{}
	ttf, err := NewTrueTypeFont(testFontTTF, 12)
	if err != nil {
		t.Fatal(err)
	}
	e.Font = ttf
	sup := e.superscriptFont()
	if sup.Height() >= ttf.Height() {
		t.Errorf("superscript height %d not smaller than base %d", sup.Height(), ttf.Height())
	}
	if e.superscriptFont() != sup { // cache hit: same face object
		t.Error("superscript face was not memoised")
	}
}

// --- Anchor ---------------------------------------------------------------

func TestAnchorInlinesRenderNormallyIDInvisible(t *testing.T) {
	doc := richdoc.New().P(
		richdoc.Txt("go "),
		richdoc.Mark("sec:intro", richdoc.Txt("here")),
		richdoc.Txt(" now"),
	).Doc()
	e, ln := block0Line(t, doc, 300, 100)

	// "go here now" = 11 caret cells, all at ordinary advance-aligned metrics.
	if ln.nCells() != 11 {
		t.Fatalf("cells = %d, want 11", ln.nCells())
	}
	adv := e.baseFont().Advance()
	for k := 0; k <= 11; k++ {
		if ln.cellX[k] != rePadX()+k*adv {
			t.Errorf("cellX[%d] = %d, want %d (anchor text at normal metrics)", k, ln.cellX[k], rePadX()+k*adv)
		}
	}

	// The ID is invisible chrome: an anchored span paints pixel-identical to the
	// same text with no anchor.
	plain := NewRichEditor(richdoc.New().P(richdoc.Txt("go here now")).Doc())
	if d := diffCount(reRender(plain, 300, 100, DefaultLight()), reRender(e, 300, 100, DefaultLight())); d != 0 {
		t.Errorf("anchor ID painted %d visible pixels, want 0", d)
	}

	// The caret enters the anchor's inlines and typing there keeps the Anchor+ID.
	e.Caret().Set(DocPos{0, 4}) // between 'h' and 'ere' of "here"
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	if blockPlain(e, 0) != "go hXere now" {
		t.Errorf("edit inside anchor = %q, want %q", blockPlain(e, 0), "go hXere now")
	}
	if ids := anchorIDs(e.Document()); len(ids) != 1 || ids[0] != "sec:intro" {
		t.Errorf("anchor IDs after edit = %v, want [sec:intro]", ids)
	}
}

func TestPointAnchorFaintTickAtom(t *testing.T) {
	doc := richdoc.New().P(richdoc.Txt("x"), richdoc.Mark("target"), richdoc.Txt("y")).Doc()
	e, ln := block0Line(t, doc, 200, 100)

	// x [anchor] y — the point anchor is one caret cell of its own fixed width.
	if ln.nCells() != 3 {
		t.Fatalf("cells = %d, want 3", ln.nCells())
	}
	if got := ln.cellX[2] - ln.cellX[1]; got != strokeWidth()+scaled(2) {
		t.Errorf("point-anchor cell width = %d, want %d", got, strokeWidth()+scaled(2))
	}

	// A faint (dim) tick is painted at the anchor cell so a reader sees a target.
	lay := e.buildLayout(DefaultLight())
	dim := dimInk(DefaultLight())
	tick := false
	for _, c := range lay.chrome {
		if !c.stroke && c.text == "" && c.r.X == ln.cellX[1] && c.c == dim {
			tick = true
		}
	}
	if !tick {
		t.Error("no faint target tick painted for the point anchor")
	}

	// The point anchor and its ID survive an edit round-trip.
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	if ids := anchorIDs(e.Document()); len(ids) != 1 || ids[0] != "target" {
		t.Errorf("point-anchor IDs after edit = %v, want [target]", ids)
	}
}

// --- CrossRef -------------------------------------------------------------

func TestCrossRefInlinesAccentAndRoundTrip(t *testing.T) {
	doc := richdoc.New().P(
		richdoc.Txt("see "),
		richdoc.Ref("fig1", richdoc.Txt("Fig 1")),
		richdoc.Txt(", "),
		richdoc.Cite("knuth74", richdoc.Txt("Knuth")),
		richdoc.Txt("."),
	).Doc()
	e, ln := block0Line(t, doc, 400, 100)

	// The reference text lays out at normal metrics but paints in the accent ink,
	// while the surrounding prose stays the surface ink.
	adv := e.baseFont().Advance()
	if ln.cellX[1]-ln.cellX[0] != adv {
		t.Errorf("crossref-adjacent cell width = %d, want %d", ln.cellX[1]-ln.cellX[0], adv)
	}
	fig, ok := findRun(ln, "Fig 1")
	if !ok {
		t.Fatal("no 'Fig 1' cross-reference run")
	}
	if fig.ink != DefaultLight().Accent {
		t.Errorf("cross-reference ink = %v, want accent", fig.ink)
	}
	if fig.underline {
		t.Error("cross-reference must not underline like a link")
	}
	if prose, ok := findRun(ln, "see "); ok && prose.ink == DefaultLight().Accent {
		t.Error("surrounding prose painted in the accent ink")
	}

	// The caret enters the reference inlines; typing there preserves the CrossRef
	// with its Target and Kind (label vs cite).
	e.Caret().Set(DocPos{0, 4}) // start of "Fig 1"
	e.OnEvent(Event{Kind: EventChar, Code: "X"})
	if blockPlain(e, 0) != "see XFig 1, Knuth." {
		t.Errorf("edit inside crossref = %q", blockPlain(e, 0))
	}
	xs := crossRefs(e.Document())
	if len(xs) != 2 {
		t.Fatalf("crossrefs after edit = %d, want 2", len(xs))
	}
	if xs[0].Target != "fig1" || xs[0].Kind != richdoc.RefLabel {
		t.Errorf("label crossref = %+v, want fig1/RefLabel", xs[0])
	}
	if xs[1].Target != "knuth74" || xs[1].Kind != richdoc.RefCite {
		t.Errorf("cite crossref = %+v, want knuth74/RefCite", xs[1])
	}
}

func TestEmptyCrossRefPlaceholders(t *testing.T) {
	for _, tc := range []struct {
		name string
		node richdoc.Inline
		want string
	}{
		{"label", richdoc.Ref("fig1"), "fig1"},
		{"cite", richdoc.Cite("knuth74"), "[knuth74]"},
		{"empty-target-label", richdoc.CrossRef{Kind: richdoc.RefLabel}, "?"},
		{"empty-target-cite", richdoc.CrossRef{Kind: richdoc.RefCite}, "[?]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := richdoc.New().P(tc.node).Doc()
			e, ln := block0Line(t, doc, 300, 100)
			if ln.nCells() != 1 {
				t.Fatalf("cells = %d, want 1 (atomic crossref)", ln.nCells())
			}
			run, ok := findRun(ln, tc.want)
			if !ok {
				t.Fatalf("no placeholder run %q; runs = %+v", tc.want, ln.runs)
			}
			if run.ink != DefaultLight().Accent {
				t.Errorf("placeholder ink = %v, want accent", run.ink)
			}
			if got := ln.cellX[1] - ln.cellX[0]; got != e.baseFont().Measure(tc.want) {
				t.Errorf("placeholder cell width = %d, want %d", got, e.baseFont().Measure(tc.want))
			}
			// The inline-less CrossRef survives an edit round-trip unchanged.
			_ = reRender(e, 300, 100, DefaultLight())
		})
	}
}

func TestEmptyCrossRefSurvivesEdit(t *testing.T) {
	doc := richdoc.New().P(richdoc.Txt("a"), richdoc.Cite("k"), richdoc.Txt("b")).Doc()
	e := NewRichEditor(doc)
	e.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	e.Caret().Set(DocPos{0, 0})
	e.OnEvent(Event{Kind: EventChar, Code: "Z"})
	xs := crossRefs(e.Document())
	if len(xs) != 1 || xs[0].Target != "k" || xs[0].Kind != richdoc.RefCite || len(xs[0].Inlines) != 0 {
		t.Errorf("inline-less cite after edit = %+v, want empty knuth-style cite k", xs)
	}
}

// --- direct model unit tests (helpers + rebuild grouping) -----------------

func TestXrefTextBranches(t *testing.T) {
	cases := []struct {
		x    richdoc.CrossRef
		want string
	}{
		{richdoc.CrossRef{Target: "t", Kind: richdoc.RefLabel}, "t"},
		{richdoc.CrossRef{Target: "t", Kind: richdoc.RefCite}, "[t]"},
		{richdoc.CrossRef{Kind: richdoc.RefLabel}, "?"},
		{richdoc.CrossRef{Kind: richdoc.RefCite}, "[?]"},
	}
	for _, c := range cases {
		if got := xrefText(c.x); got != c.want {
			t.Errorf("xrefText(%+v) = %q, want %q", c.x, got, c.want)
		}
	}
}

func TestRefFlattenRebuildRoundTrip(t *testing.T) {
	inlines := []richdoc.Inline{
		richdoc.Txt("a "),
		richdoc.Mark("id", richdoc.Bold(richdoc.Txt("anchored"))),
		richdoc.Ref("fig", richdoc.Txt("F")),
		richdoc.Cite("key", richdoc.Txt("C")),
		richdoc.Mark("pt"), // point anchor
		richdoc.Ref("solo"),
		richdoc.Note(richdoc.Paragraph{Inlines: []richdoc.Inline{richdoc.Txt("body")}}),
	}
	got := rebuildInlines(flattenInlines(inlines))
	d := &richdoc.Document{Blocks: []richdoc.Block{richdoc.Paragraph{Inlines: got}}}

	if ids := anchorIDs(d); len(ids) != 2 {
		t.Errorf("anchor IDs = %v, want two (wrapping + point)", ids)
	}
	if xs := crossRefs(d); len(xs) != 3 {
		t.Errorf("crossrefs = %d, want 3", len(xs))
	}
	if fs := footnotes(d); len(fs) != 1 {
		t.Errorf("footnotes = %d, want 1", len(fs))
	}
	// The wrapping anchor keeps its bold child intact.
	if !walkHas(d, func(n any) bool { _, ok := n.(richdoc.Strong); return ok }) {
		t.Error("bold inside the anchor was lost")
	}
}

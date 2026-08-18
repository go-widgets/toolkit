// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-gfx/gfx/iso"
)

// textDiagram builds a 400x400 diagram with a single text annotation "t".
func textDiagram(x, y int, s string) *IsoDiagram {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutText(IsoText{ID: "t", X: x, Y: y, Text: s})
	return d
}

// --- geometry -----------------------------------------------------------

func TestIsoTextBoxExact(t *testing.T) {
	d := textDiagram(3, 4, "hello")
	tx, _ := d.Doc().Text("t")
	f := d.textFont(tx)
	anchor := d.proj.Project(iso.V(3.5, 4.5, 0))
	tw := f.Measure("hello")
	th := f.Height()
	want := Rect{X: iround(anchor.X) - tw/2, Y: iround(anchor.Y) - th/2, W: tw, H: th}
	if got := d.textBox(tx); got != want {
		t.Fatalf("textBox = %+v, want %+v (centred on exact projected anchor)", got, want)
	}
}

func TestIsoTextEmptyBoxMinWidth(t *testing.T) {
	d := textDiagram(3, 4, "")
	tx, _ := d.Doc().Text("t")
	f := d.textFont(tx)
	if got := d.textBox(tx); got.W != f.Advance() || got.H != f.Height() {
		t.Fatalf("empty text box = %+v, want one glyph (%d x %d)", got, f.Advance(), f.Height())
	}
}

func TestIsoTextFontDefaultAndSized(t *testing.T) {
	d := NewIsoDiagram(nil)
	if d.textFont(IsoText{}).Advance() != d.EffectiveFont().Advance() {
		t.Fatal("unsized text did not use the effective font")
	}
	want := NewBitmapFont(scaled(3)).Advance()
	if got := d.textFont(IsoText{Size: 3}).Advance(); got != want {
		t.Fatalf("sized text advance = %d, want %d", got, want)
	}
}

func TestIsoTextInk(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	if got := d.textInk(IsoText{}, theme); got != theme.OnSurface {
		t.Fatalf("unset ink = %v, want OnSurface", got)
	}
	c := RGBA{R: 4, G: 5, B: 6, A: 255}
	if got := d.textInk(IsoText{Color: c}, theme); got != c {
		t.Fatalf("explicit ink = %v, want %v", got, c)
	}
}

// --- rendering: text is the topmost layer -------------------------------

func TestIsoTextRendersOverNode(t *testing.T) {
	theme := DefaultLight()
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "n", X: 3, Y: 3, Color: RGBA{R: 200, G: 0, B: 0, A: 255}})
	ink := RGBA{R: 0, G: 40, B: 220, A: 255}
	d.Doc().PutText(IsoText{ID: "t", X: 3, Y: 3, Text: "HI", Color: ink})
	img, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// The annotation, drawn last, paints its ink over the node beneath it.
	if !hasInk(img.Pix, ink) {
		t.Fatal("text ink not present: annotation did not draw on the top layer")
	}
}

func TestIsoTextEmptySkipsDrawButOutlinesSelection(t *testing.T) {
	theme := DefaultLight()
	d := textDiagram(3, 3, "") // empty caption
	d.SelectText("t")
	img, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// Empty text draws no glyphs, but the selection box is stroked in accent.
	if !hasInk(img.Pix, theme.Accent) {
		t.Fatal("selected empty-text outline not painted")
	}
}

// --- hit testing --------------------------------------------------------

func TestIsoTextAtLocalTopmost(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if _, ok := d.textAtLocal(10, 10); ok {
		t.Fatal("empty diagram hit a text")
	}
	d.Doc().PutText(IsoText{ID: "low", X: 3, Y: 3, Text: "AAAA"})
	d.Doc().PutText(IsoText{ID: "high", X: 3, Y: 3, Text: "AAAA"}) // same spot, drawn later
	tx, _ := d.Doc().Text("high")
	box := d.textBox(tx)
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	if id, ok := d.textAtLocal(cx, cy); !ok || id != "high" {
		t.Fatalf("overlap text pick = %q ok=%v, want high", id, ok)
	}
	if _, ok := d.textAtLocal(399, 1); ok {
		t.Fatal("far point hit a text")
	}
}

// --- creation gesture ---------------------------------------------------

func TestIsoTextTapAdds(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeText
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sx, sy := localOf(d, iso.V(4.5, 5.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseUp, X: sx, Y: sy})
	ts := d.Doc().Texts()
	if len(ts) != 1 || ts[0].X != 4 || ts[0].Y != 5 || ts[0].Text != "" {
		t.Fatalf("tap added %+v, want one empty text at (4,5)", ts)
	}
	if d.SelectedText() != ts[0].ID {
		t.Fatalf("added text not selected: %q", d.SelectedText())
	}
	// The host fills the caption in through the selected-text setter.
	if !d.SetSelectedTextContent("Note") {
		t.Fatal("SetSelectedTextContent reported no change")
	}
	if tx, _ := d.Doc().Text(ts[0].ID); tx.Text != "Note" {
		t.Fatalf("caption = %q, want Note", tx.Text)
	}
}

func TestIsoTextDragAddsNothing(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeText
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sx, sy := localOf(d, iso.V(4.5, 5.5, 0))
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: sx + 40, Y: sy + 40}) // a drag, not a tap
	d.OnEvent(Event{Kind: EventMouseUp, X: sx + 40, Y: sy + 40})
	if len(d.Doc().Texts()) != 0 {
		t.Fatal("a drag in text mode created a text")
	}
}

func TestIsoTextNextIDSkipsCollision(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	d.Doc().PutText(IsoText{ID: "t1", X: 0, Y: 0}) // occupies the first id
	id := d.commitPlaceText(2, 2)
	if id == "t1" {
		t.Fatal("nextTextID collided with existing t1")
	}
}

// --- move gesture -------------------------------------------------------

func TestIsoTextMoveDrag(t *testing.T) {
	d := textDiagram(3, 3, "note")
	tx, _ := d.Doc().Text("t")
	box := d.textBox(tx)
	px, py := box.X+box.W/2, box.Y+box.H/2 // press on the text box
	// grab cell under the press, then drop three cells further.
	gx, gy := d.cellAtLocal(px, py)
	dropx, dropy := groundCenterLocal(d, gx+3, gy+2)
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	if d.SelectedText() != "t" {
		t.Fatalf("press on text did not select it: %q", d.SelectedText())
	}
	d.OnEvent(Event{Kind: EventMouseDrag, X: dropx, Y: dropy})
	d.OnEvent(Event{Kind: EventMouseUp, X: dropx, Y: dropy})
	moved, _ := d.Doc().Text("t")
	if moved.X != 3+3 || moved.Y != 3+2 {
		t.Fatalf("text moved to (%d,%d), want (6,5)", moved.X, moved.Y)
	}
	d.Undo()
	if u, _ := d.Doc().Text("t"); u.X != 3 || u.Y != 3 {
		t.Fatalf("undo left text at (%d,%d), want (3,3)", u.X, u.Y)
	}
}

func TestIsoTextMoveSameCellNoop(t *testing.T) {
	d := textDiagram(3, 3, "note")
	tx, _ := d.Doc().Text("t")
	box := d.textBox(tx)
	px, py := box.X+box.W/2, box.Y+box.H/2
	d.OnEvent(Event{Kind: EventClick, X: px, Y: py})
	d.OnEvent(Event{Kind: EventMouseDrag, X: px, Y: py}) // same cell
	d.OnEvent(Event{Kind: EventMouseUp, X: px, Y: py})
	if m, _ := d.Doc().Text("t"); m.X != 3 || m.Y != 3 {
		t.Fatalf("no-op move shifted text to (%d,%d)", m.X, m.Y)
	}
}

func TestIsoTextMoveMissingNoPanic(t *testing.T) {
	d := textDiagram(3, 3, "note")
	d.dragText = "ghost"
	d.moveTextDragTo(10, 10) // absent -> early return
}

// --- selection ----------------------------------------------------------

func TestIsoTextSelectionObservable(t *testing.T) {
	d := textDiagram(3, 3, "n")
	d.Doc().PutNode(IsoNode{ID: "nd", X: 7, Y: 7})
	d.Doc().PutConnector(IsoConnector{ID: "c", From: "nd", To: "nd"})
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 0, W: 1, H: 1})
	var seen []string
	d.OnSelectText = func(id string) { seen = append(seen, id) }
	if d.SelectedText() != "" {
		t.Fatal("fresh widget has a selected text")
	}
	if d.SelectedTextObservable() != d.selText {
		t.Fatal("observable accessor mismatch")
	}
	d.SelectText("t")
	if d.SelectedText() != "t" {
		t.Fatalf("SelectedText = %q", d.SelectedText())
	}
	d.setSelected("nd")
	if d.SelectedText() != "" {
		t.Fatal("node selection did not clear the text selection")
	}
	d.SelectText("t")
	d.SelectConnector("c")
	if d.SelectedText() != "" {
		t.Fatal("connector selection did not clear the text selection")
	}
	d.SelectText("t")
	d.SelectZone("z")
	if d.SelectedText() != "" {
		t.Fatal("zone selection did not clear the text selection")
	}
	if len(seen) < 2 || seen[0] != "t" {
		t.Fatalf("OnSelectText calls = %v", seen)
	}
}

// --- setters ------------------------------------------------------------

func TestIsoTextSetters(t *testing.T) {
	d := textDiagram(1, 1, "old")
	if !d.SetTextContent("t", "new") {
		t.Fatal("content change no-op")
	}
	if d.SetTextContent("t", "new") {
		t.Fatal("redundant content set reported a change")
	}
	if d.SetTextContent("nope", "x") {
		t.Fatal("editing a missing text reported a change")
	}
	col := RGBA{R: 1, G: 2, B: 3, A: 255}
	if !d.SetTextColor("t", col) {
		t.Fatal("colour change no-op")
	}
	if d.SetTextColor("t", col) {
		t.Fatal("redundant colour set reported a change")
	}
	if !d.SetTextSize("t", 2) {
		t.Fatal("size change no-op")
	}
	if d.SetTextSize("t", 2) {
		t.Fatal("redundant size set reported a change")
	}
	if !d.SetTextPos("t", 5, 6) {
		t.Fatal("pos change no-op")
	}
	if d.SetTextPos("t", 5, 6) {
		t.Fatal("redundant pos set reported a change")
	}
	tx, _ := d.Doc().Text("t")
	if tx.Text != "new" || tx.Color != col || tx.Size != 2 || tx.X != 5 || tx.Y != 6 {
		t.Fatalf("text after edits = %+v", tx)
	}
	d.Undo() // undo the pos edit
	if u, _ := d.Doc().Text("t"); u.X != 1 || u.Y != 1 {
		t.Fatalf("undo left pos (%d,%d), want (1,1)", u.X, u.Y)
	}
}

func TestIsoSetSelectedText(t *testing.T) {
	d := textDiagram(1, 1, "n")
	// Nothing selected -> no-ops.
	if d.SetSelectedTextContent("x") {
		t.Fatal("content set with no selection")
	}
	if d.SetSelectedTextColor(RGBA{A: 255}) {
		t.Fatal("colour set with no selection")
	}
	if d.SetSelectedTextSize(4) {
		t.Fatal("size set with no selection")
	}
	d.SelectText("t")
	if !d.SetSelectedTextContent("caption") {
		t.Fatal("content-selected no-op")
	}
	if !d.SetSelectedTextColor(RGBA{R: 7, G: 7, B: 7, A: 255}) {
		t.Fatal("colour-selected no-op")
	}
	if !d.SetSelectedTextSize(3) {
		t.Fatal("size-selected no-op")
	}
	tx, _ := d.Doc().Text("t")
	if tx.Text != "caption" || tx.Size != 3 || tx.Color.A != 255 {
		t.Fatalf("selected text edits not applied: %+v", tx)
	}
}

// --- delete: key + context menu -----------------------------------------

func TestIsoTextDeleteKey(t *testing.T) {
	d := textDiagram(1, 1, "n")
	d.SelectText("t")
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if _, ok := d.Doc().Text("t"); ok {
		t.Fatal("Backspace did not remove the selected text")
	}
	if d.SelectedText() != "" {
		t.Fatal("text selection not cleared after delete")
	}
}

func TestIsoTextCommitDeleteMissing(t *testing.T) {
	d := textDiagram(1, 1, "n")
	before := d.CanUndo()
	d.commitDeleteText("nope") // absent -> no undo pushed
	if d.CanUndo() != before {
		t.Fatal("deleting a missing text pushed an undo entry")
	}
}

func TestIsoTextContextMenu(t *testing.T) {
	d := textDiagram(3, 3, "note")
	tx, _ := d.Doc().Text("t")
	box := d.textBox(tx)
	cx, cy := box.X+box.W/2, box.Y+box.H/2
	d.OnEvent(Event{Kind: EventSecondaryClick, X: cx, Y: cy})
	if d.SelectedText() != "t" {
		t.Fatal("secondary click did not select the text")
	}
	if len(d.menu.Menu.Items) != 1 || d.menu.Menu.Items[0].Label != "Delete" {
		t.Fatalf("text menu = %+v, want [Delete]", d.menu.Menu.Items)
	}
	d.menu.Menu.Items[0].Action()
	if _, ok := d.Doc().Text("t"); ok {
		t.Fatal("Delete action did not remove the text")
	}
}

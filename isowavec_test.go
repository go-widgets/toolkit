// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-gfx/gfx/iso"
)

// TestIsoWaveCControlRunByteIdentical is the control run for Wave C: a document
// with NO zones and NO text annotations must render byte-for-byte identically to
// the same document with the zone/text layers absent — proving the two new
// layers are strictly additive and inert when empty. A zone and a text visibly
// change the render (so the layers really do draw); removing them again returns
// the pixels to the exact original bytes.
func TestIsoWaveCControlRunByteIdentical(t *testing.T) {
	theme := DefaultLight()

	build := func() *IsoDiagram {
		d := NewIsoDiagram(nil)
		d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 400})
		d.Doc().PutNode(IsoNode{ID: "a", X: 1, Y: 4, Color: RGBA{R: 200, G: 20, B: 20, A: 255}})
		d.Doc().PutNode(IsoNode{ID: "b", X: 6, Y: 4, Shape: IsoBox})
		d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b", Arrow: IsoArrowSingle, Label: "link"})
		return d
	}

	base, err := RenderImage(build(), 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}

	// The same diagram, plus a zone and a text: the render must differ.
	d := build()
	d.Doc().PutZone(IsoZone{ID: "z", X: 0, Y: 3, W: 8, H: 3, Label: "grp"})
	d.Doc().PutText(IsoText{ID: "t", X: 3, Y: 1, Text: "note"})
	withExtras, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	same := true
	for i := range base.Pix {
		if base.Pix[i] != withExtras.Pix[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("a zone and a text did not change the render at all")
	}

	// Remove them again: the pixels return to the exact original bytes.
	d.Doc().RemoveZone("z")
	d.Doc().RemoveText("t")
	cleared, err := RenderImage(d, 500, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	for i := range base.Pix {
		if base.Pix[i] != cleared.Pix[i] {
			t.Fatalf("byte %d differs after clearing zone+text: base=%d cleared=%d", i, base.Pix[i], cleared.Pix[i])
		}
	}
}

// TestIsoZoneCreateReverseDrag drags a zone rectangle from a high cell to a low
// one: the committed zone is normalised to the min corner and positive size
// (exercising the negative-delta path of iabs and the reversed rubber-band
// preview).
func TestIsoZoneCreateReverseDrag(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.Mode = IsoModeZone
	d.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	sx, sy := localOf(d, iso.V(6.5, 6.5, 0)) // press high
	ex, ey := localOf(d, iso.V(2.5, 3.5, 0)) // drag to low
	d.OnEvent(Event{Kind: EventClick, X: sx, Y: sy})
	d.OnEvent(Event{Kind: EventMouseDrag, X: ex, Y: ey})
	d.Draw(painterPixel(make([]byte, 4*400*400), 400, 400), DefaultLight()) // reversed preview
	d.OnEvent(Event{Kind: EventMouseUp, X: ex, Y: ey})
	zs := d.Doc().Zones()
	if len(zs) != 1 || zs[0].X != 2 || zs[0].Y != 3 || zs[0].W != 5 || zs[0].H != 4 {
		t.Fatalf("reverse-drag zone = %+v, want (2,3) 5x4", zs)
	}
}

// TestIsoWaveCA11yTree walks the accessibility tree of a diagram carrying a
// node, a connector, two zones (one labelled+selected, one unlabelled) and two
// texts (one captioned, one empty+selected), covering the zone/text proxy loops,
// both name-fallback branches and both selected-state branches.
func TestIsoWaveCA11yTree(t *testing.T) {
	d := NewIsoDiagram(nil)
	d.SetBounds(Rect{X: 5, Y: 7, W: 400, H: 400})
	d.Doc().PutNode(IsoNode{ID: "a", X: 2, Y: 2, Label: "Web"})
	d.Doc().PutNode(IsoNode{ID: "b", X: 4, Y: 4})
	d.Doc().PutConnector(IsoConnector{ID: "ab", From: "a", To: "b"})
	d.Doc().PutZone(IsoZone{ID: "z1", X: 0, Y: 0, W: 3, H: 3, Label: "Cluster"})
	d.Doc().PutZone(IsoZone{ID: "z2", X: 5, Y: 5, W: 2, H: 2}) // unlabelled -> name is id
	d.Doc().PutText(IsoText{ID: "t1", X: 6, Y: 1, Text: "hi"})
	d.Doc().PutText(IsoText{ID: "t2", X: 7, Y: 2}) // empty -> name is id
	walk := func() map[string]A11yInfo {
		out := map[string]A11yInfo{}
		for _, e := range WalkA11y(d) {
			out[e.Name] = e.A11yInfo
		}
		return out
	}

	// With the zone selected: it reports selected, both texts do not.
	d.SelectZone("z1")
	byName := walk()
	if len(byName) != 8 { // group + 2 nodes + 1 connector + 2 zones + 2 texts (unique names)
		t.Fatalf("a11y tree has %d unique names, want 8", len(byName))
	}
	if info, ok := byName["Cluster"]; !ok || info.Role != RoleGroup || info.Value != "selected" {
		t.Fatalf("selected labelled zone proxy = %+v ok=%v", info, ok)
	}
	if byName["z2"].Value != "" {
		t.Fatalf("unselected zone value = %q, want empty", byName["z2"].Value)
	}
	if byName["hi"].Value != "" {
		t.Fatalf("unselected text value = %q, want empty", byName["hi"].Value)
	}

	// Switch the selection to the empty text: now it reports selected, no zone
	// does (the two selections are mutually exclusive).
	d.SelectText("t2")
	byName = walk()
	if byName["t2"].Value != "selected" {
		t.Fatalf("selected empty text value = %q, want selected", byName["t2"].Value)
	}
	if byName["Cluster"].Value != "" {
		t.Fatalf("zone value = %q, want empty after the text was selected", byName["Cluster"].Value)
	}
}

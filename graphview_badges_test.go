// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func badgeView(withBadge bool) (*GraphView, *GraphViewModel) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	a := GraphNode{ID: "a", Label: "", Cluster: "c", Category: "k",
		Toggles: []GraphToggle{{ID: "mute", Glyph: "S", Label: "sound", On: true}, {ID: "pin", Glyph: "P"}}}
	b := GraphNode{ID: "b", Label: "bee", Cluster: "c", Category: "k"}
	if withBadge {
		b.Badges = []GraphBadge{{Glyph: "!", Text: "muted"}, {Text: "no egress"}}
	}
	m.Nodes.Append(a, b)
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 400, H: 200})
	return g, m
}

func TestGraphViewBadgesMakeEveryNodeOneChipRowTaller(t *testing.T) {
	plain, _ := badgeView(false)
	plain.Draw(newP(makeSurface(400, 200), 400), DefaultLight())
	pa, _ := plain.NodeBox("a")
	if pa.H != gNodeH() {
		t.Fatalf("without badges a node is %d high, want %d", pa.H, gNodeH())
	}
	g, _ := badgeView(true)
	l := &textLog{Painter: newP(makeSurface(400, 200), 400)}
	g.Draw(l, DefaultLight())
	a, _ := g.NodeBox("a")
	b, _ := g.NodeBox("b")
	if a.H != gNodeH()+scaled(graphBadgeH) || b.H != a.H {
		t.Fatalf("node heights %d and %d, want both %d", a.H, b.H, gNodeH()+scaled(graphBadgeH))
	}
	seen := map[string]bool{}
	for _, s := range l.texts {
		seen[s] = true
	}
	for _, want := range []string{"bee", "! muted", "no egress"} {
		if !seen[want] {
			t.Errorf("%q was not drawn; drew %v", want, l.texts)
		}
	}
}

func TestGraphViewToggleClickCallsOnToggleAndLeavesTheSelection(t *testing.T) {
	g, m := badgeView(false)
	g.Draw(newP(makeSurface(400, 200), 400), DefaultLight())
	a, _ := g.NodeBox("a")
	rects := g.toggleRects(a, 2)
	var got [][2]string
	g.OnToggle = func(n, tg string) { got = append(got, [2]string{n, tg}) }
	click := func(r Rect) { g.OnEvent(Event{Kind: EventClick, X: r.X + r.W/2, Y: r.Y + r.H/2}) }
	click(rects[1])
	click(rects[0])
	if len(got) != 2 || got[0] != [2]string{"a", "pin"} || got[1] != [2]string{"a", "mute"} {
		t.Fatalf("OnToggle calls = %v", got)
	}
	if m.Selected.Get() != "" {
		t.Errorf("a toggle click selected %q", m.Selected.Get())
	}
	// The label part of the node still selects it, and fires nothing.
	g.OnEvent(Event{Kind: EventClick, X: a.X + 2, Y: a.Y + 2})
	if m.Selected.Get() != "node:a" || len(got) != 2 {
		t.Errorf("selected %q after %d toggle calls", m.Selected.Get(), len(got))
	}
	// A click on a toggle with no callback is harmless.
	g.OnToggle = nil
	click(rects[0])
}

// A node in an unknown cluster has no box, so none of its toggles can be hit.
func TestGraphViewToggleOfAnUnplacedNodeIsNotHittable(t *testing.T) {
	g, m := badgeView(false)
	m.Nodes.Append(GraphNode{ID: "lost", Cluster: "nowhere", Toggles: []GraphToggle{{ID: "t", Glyph: "T"}}})
	g.Draw(newP(makeSurface(400, 200), 400), DefaultLight())
	a, _ := g.NodeBox("a")
	rects := g.toggleRects(a, 2)
	if n, tg, ok := g.toggleAt(rects[0].X+1, rects[0].Y+1); !ok || n != "a" || tg != "mute" {
		t.Fatalf("toggleAt on a's first toggle = %q %q %v", n, tg, ok)
	}
	if n, _, ok := g.toggleAt(0, 0); ok {
		t.Errorf("toggleAt on empty space hit %q", n)
	}
}

func TestGraphViewSpaceTogglesTheFirstToggleOfTheSelectedNode(t *testing.T) {
	g, m := badgeView(false)
	g.Draw(newP(makeSurface(400, 200), 400), DefaultLight())
	var got []string
	g.OnToggle = func(n, tg string) { got = append(got, n+"/"+tg) }
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Space"}) // nothing selected
	m.Selected.Set("node:b")                            // no toggles
	g.OnEvent(Event{Kind: EventKeyDown, Code: " "})
	if len(got) != 0 {
		t.Fatalf("fired %v with nothing to toggle", got)
	}
	m.Selected.Set("node:a")
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Space"})
	if len(got) != 1 || got[0] != "a/mute" {
		t.Fatalf("Space fired %v, want a/mute", got)
	}
	g.OnToggle = nil
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Space"})
}

func TestGraphViewNodeColorOverridesTheCategory(t *testing.T) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	m.Nodes.Append(GraphNode{ID: "a", Cluster: "c", Category: "k"}, GraphNode{ID: "b", Cluster: "c", Category: "k"})
	red := RGB(0xC8, 0x10, 0x10)
	g := NewGraphView(m)
	g.NodeColor = func(n GraphNode) (RGBA, bool) { return red, n.ID == "a" }
	g.SetBounds(Rect{W: 300, H: 150})
	surf := makeSurface(300, 150)
	g.Draw(newP(surf, 300), DefaultLight())
	corner := func(id string) RGBA {
		b, _ := g.NodeBox(id)
		return pixelAt(surf, 300, b.X+1, b.Y+b.H-2)
	}
	if corner("a") != red {
		t.Errorf("overridden node is %v, want %v", corner("a"), red)
	}
	if corner("b") != g.categoryColor("k") {
		t.Errorf("other node is %v, want its category colour %v", corner("b"), g.categoryColor("k"))
	}
}

// A hub and a gateway take their shapes, and a toggle on a node placed in a
// narrow box does not escape it.
func TestGraphViewDrawsShapesWithToggles(t *testing.T) {
	m := NewGraphViewModel()
	m.Nodes.Append(
		GraphNode{ID: "h", Label: "net", Shape: GraphShapeHub, Toggles: []GraphToggle{{ID: "t", Glyph: "x"}}},
		GraphNode{ID: "g", Label: "out", Shape: GraphShapeGateway, Badges: []GraphBadge{{Text: "b"}}},
	)
	g := NewGraphView(m)
	g.NodeWidth = 20
	g.Layout = GraphLayoutBands
	g.SetBounds(Rect{W: 200, H: 100})
	g.Draw(newP(makeSurface(200, 100), 200), DefaultLight())
	h, ok := g.NodeBox("h")
	if !ok {
		t.Fatal("hub has no box")
	}
	var fired string
	g.OnToggle = func(n, tg string) { fired = n + "/" + tg }
	g.OnEvent(Event{Kind: EventClick, X: h.X + h.W - 3, Y: h.Y + 3})
	if fired != "h/t" {
		t.Errorf("toggle on a hub fired %q", fired)
	}
}

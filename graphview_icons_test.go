// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

var iconMark = RGB(0x12, 0x34, 0x56)

// markIcons resolves every name but "none" to a drawer that floods its rect
// with iconMark and records the name and ink it was called with.
type markIcons struct {
	names []string
	inks  []RGBA
}

func (m *markIcons) resolve(name string) func(painter.Painter, Rect, RGBA) {
	if name == "none" {
		return nil
	}
	return func(p painter.Painter, r Rect, ink RGBA) {
		m.names = append(m.names, name)
		m.inks = append(m.inks, ink)
		fillRect(p, r.X, r.Y, r.W, r.H, iconMark)
	}
}

func countMark(buf []byte, w, h int) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == iconMark {
				n++
			}
		}
	}
	return n
}

func iconView(n GraphNode) (*GraphView, *GraphViewModel) {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "c", Label: "c"})
	n.Cluster, n.Category = "c", "k"
	m.Nodes.Append(n)
	g := NewGraphView(m)
	g.SetBounds(Rect{W: 400, H: 200})
	return g, m
}

func drawIconView(g *GraphView) []byte {
	buf := makeSurface(400, 200)
	g.Draw(newP(buf, 400), DefaultLight())
	return buf
}

func TestGraphViewNodeIconIsDrawnInsideTheNodeBox(t *testing.T) {
	mi := &markIcons{}
	g, _ := iconView(GraphNode{ID: "a", Label: "hub", Icon: "router"})
	g.Icons = mi.resolve
	buf := drawIconView(g)
	if len(mi.names) != 1 || mi.names[0] != "router" {
		t.Fatalf("resolved icons %v, want [router]", mi.names)
	}
	box, _ := g.NodeBox("a")
	inside := 0
	for y := box.Y; y < box.Y+box.H; y++ {
		for x := box.X; x < box.X+box.W; x++ {
			if pixelAt(buf, 400, x, y) == iconMark {
				inside++
			}
		}
	}
	if inside < 16 {
		t.Fatalf("only %d icon pixels inside the node box %v", inside, box)
	}
	if all := countMark(buf, 400, 200); all != inside {
		t.Errorf("%d icon pixels, %d inside the node box: the icon spilled out", all, inside)
	}
}

func TestGraphViewNodeWithoutIconOrResolverDrawsNone(t *testing.T) {
	mi := &markIcons{}
	g, _ := iconView(GraphNode{ID: "a", Label: "x"})
	g.Icons = mi.resolve
	if countMark(drawIconView(g), 400, 200) != 0 || len(mi.names) != 0 {
		t.Errorf("a node with no Icon drew one: %v", mi.names)
	}
	g2, _ := iconView(GraphNode{ID: "a", Label: "x", Icon: "router"})
	if countMark(drawIconView(g2), 400, 200) != 0 {
		t.Error("an Icon with no resolver drew something")
	}
}

func TestGraphViewNodeIconPushesTheLabelRight(t *testing.T) {
	labelX := func(icon string) int {
		g, _ := iconView(GraphNode{ID: "a", Label: "hub", Icon: icon})
		g.Icons = (&markIcons{}).resolve
		l := &xLog{Painter: newP(makeSurface(400, 200), 400)}
		g.Draw(l, DefaultLight())
		return l.x["hub"]
	}
	if with, without := labelX("router"), labelX(""); with <= without {
		t.Fatalf("label x with icon %d, without %d: the label must move right", with, without)
	}
}

// xLog remembers the x each string was drawn at.
type xLog struct {
	painter.Painter
	x map[string]int
}

func (l *xLog) Text(x, y int, s string, ink painter.RGBA) {
	if l.x == nil {
		l.x = map[string]int{}
	}
	l.x[s] = x
	l.Painter.Text(x, y, s, ink)
}

func TestGraphViewToggleIconReplacesTheGlyph(t *testing.T) {
	for _, on := range []bool{true, false} {
		mi := &markIcons{}
		g, _ := iconView(GraphNode{ID: "a", Label: "x",
			Toggles: []GraphToggle{{ID: "mute", Glyph: "S", Icon: "sound-high", On: on}}})
		g.Icons = mi.resolve
		l := &textLog{Painter: newP(makeSurface(400, 200), 400)}
		g.Draw(l, DefaultLight())
		if len(mi.names) != 1 || mi.names[0] != "sound-high" {
			t.Fatalf("on=%v: resolved %v", on, mi.names)
		}
		for _, s := range l.texts {
			if s == "S" {
				t.Errorf("on=%v: the text glyph was drawn beside the icon", on)
			}
		}
		if countMark(drawIconView(g), 400, 200) < 9 {
			t.Errorf("on=%v: the toggle icon left no pixels", on)
		}
	}
}

func TestGraphViewToggleIconFollowsTheState(t *testing.T) {
	// On is a solid chip with the icon in the node colour; Off an outline with
	// the icon in the label ink: the two inks differ.
	ink := func(on bool) RGBA {
		mi := &markIcons{}
		g, _ := iconView(GraphNode{ID: "a", Label: "x",
			Toggles: []GraphToggle{{ID: "m", Icon: "i", On: on}}})
		g.Icons = mi.resolve
		drawIconView(g)
		return mi.inks[0]
	}
	if ink(true) == ink(false) {
		t.Error("an On and an Off toggle icon are drawn in the same ink")
	}
}

func TestGraphViewToggleFallsBackToGlyphWhenIconIsUnresolved(t *testing.T) {
	for name, g := range map[string]*GraphView{
		"unknown name": func() *GraphView {
			g, _ := iconView(GraphNode{ID: "a", Label: "x", Toggles: []GraphToggle{{ID: "m", Glyph: "S", Icon: "none"}}})
			g.Icons = (&markIcons{}).resolve
			return g
		}(),
		"no resolver": func() *GraphView {
			g, _ := iconView(GraphNode{ID: "a", Label: "x", Toggles: []GraphToggle{{ID: "m", Glyph: "S", Icon: "i"}}})
			return g
		}(),
	} {
		l := &textLog{Painter: newP(makeSurface(400, 200), 400)}
		g.Draw(l, DefaultLight())
		found := false
		for _, s := range l.texts {
			found = found || s == "S"
		}
		if !found {
			t.Errorf("%s: the text glyph fallback was not drawn; drew %v", name, l.texts)
		}
	}
}

func TestGraphViewBadgeIconIsDrawnInTheChip(t *testing.T) {
	mi := &markIcons{}
	g, _ := iconView(GraphNode{ID: "a", Label: "x",
		Badges: []GraphBadge{{Glyph: "!", Icon: "sound-off", Text: "muted"}, {Text: "plain"}}})
	g.Icons = mi.resolve
	l := &textLog{Painter: newP(makeSurface(400, 200), 400)}
	g.Draw(l, DefaultLight())
	if len(mi.names) != 1 || mi.names[0] != "sound-off" {
		t.Fatalf("resolved %v", mi.names)
	}
	seen := map[string]bool{}
	for _, s := range l.texts {
		seen[s] = true
	}
	if !seen["muted"] || seen["! muted"] || !seen["plain"] {
		t.Errorf("badge texts %v: the icon replaces the glyph, the text stays", l.texts)
	}
	if countMark(drawIconView(g), 400, 200) < 9 {
		t.Error("the badge icon left no pixels")
	}
}

func TestGraphViewIconIsClippedToTheViewUnderZoom(t *testing.T) {
	mi := &markIcons{}
	g, _ := iconView(GraphNode{ID: "a", Label: "x", Icon: "router"})
	g.Icons = mi.resolve
	g.SetZoom(200, 0, 0)
	buf := drawIconView(g)
	box, _ := g.NodeBox("a")
	if countMark(buf, 400, 200) == 0 || box.W == 0 {
		t.Fatal("no icon under zoom")
	}
	// At 200 % the icon is twice as large as at 100 %.
	g1, _ := iconView(GraphNode{ID: "a", Label: "x", Icon: "router"})
	g1.Icons = mi.resolve
	one := countMark(drawIconView(g1), 400, 200)
	if two := countMark(buf, 400, 200); two < 3*one {
		t.Errorf("icon pixels %d at 200 %%, %d at 100 %%: the icon must scale with the zoom", two, one)
	}
}

func TestGraphViewIconDrawerGuards(t *testing.T) {
	g, _ := iconView(GraphNode{ID: "a"})
	// No resolver: a named icon is simply unresolved, not a nil call.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("iconDrawer with no resolver panicked: %v", r)
			}
		}()
		if g.iconDrawer("router") != nil {
			t.Error("a drawer came out of no resolver")
		}
	}()
	// An empty name never reaches the resolver.
	called := false
	g.Icons = func(string) func(painter.Painter, Rect, RGBA) { called = true; return nil }
	if g.iconDrawer("") != nil || called {
		t.Errorf("an empty icon name reached the resolver (called=%v)", called)
	}
}

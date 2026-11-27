package toolkit

import (
	"sort"
	"strconv"
	"strings"
)

// RoleTreeItem is a node of the graph as a screen reader meets it: an item of
// the graph's tree, whose children are its toggles (WAI-ARIA treeitem).
const RoleTreeItem Role = "treeitem"

func (*GraphView) a11yDisablesSubtree() bool { return true }

// graphProxy is a non-visual widget that carries one node's description and,
// as children, its toggles.
type graphProxy struct {
	isoProxy
	kids []Widget
}

// Children returns the node's toggle proxies.
func (p *graphProxy) Children() []Widget { return p.kids }

// joinAnd writes ["a" "b" "c"] as "a, b and c".
func joinAnd(s []string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	}
	return strings.Join(s[:len(s)-1], ", ") + " and " + s[len(s)-1]
}

// nodeName is what a screen reader says for a node: its label, the hubs it
// sits on, its badges and the edge labels of the gateways that reach it.
func (g *GraphView) nodeName(n GraphNode, byID map[string]GraphNode) string {
	name := n.Label
	if name == "" {
		name = n.ID
	}
	var on, publishes []string
	for _, e := range g.Model.Edges.Slice() {
		other := e.To
		if e.To == n.ID {
			other = e.From
		} else if e.From != n.ID {
			continue
		}
		o, ok := byID[other]
		switch {
		case ok && o.Shape == GraphShapeHub && n.Shape == GraphShapeBox:
			on = append(on, labelOrID(o))
		case ok && o.Shape == GraphShapeGateway && e.Label != "":
			publishes = append(publishes, e.Label)
		}
	}
	sort.Strings(on)
	sort.Strings(publishes)
	if len(on) > 0 {
		name += ", on " + joinAnd(on)
	}
	if len(publishes) > 0 {
		name += ", publishes " + joinAnd(publishes)
	}
	for _, b := range n.Badges {
		name += ", " + b.Text
	}
	return name
}

func labelOrID(n GraphNode) string {
	if n.Label != "" {
		return n.Label
	}
	return n.ID
}

// Children exposes the nodes, in reading order, as tree items with their
// toggles as switches. The proxies are synthetic: laid over the drawn boxes,
// never drawn.
func (g *GraphView) Children() []Widget {
	g.layout()
	byID := map[string]GraphNode{}
	for _, n := range g.Model.Nodes.Slice() {
		byID[n.ID] = n
	}
	sel := g.Model.Selected.Get()
	var out []Widget
	for _, id := range g.order {
		n := byID[id]
		box, shown := graphClip(g.screen(g.nodeBox[id]), g.Bounds())
		if !shown { // scrolled or panned out of the view: nothing to point at
			continue
		}
		state := ""
		if sel == "node:"+id {
			state = "selected"
		}
		p := &graphProxy{isoProxy: isoProxy{info: A11yInfo{Role: RoleTreeItem, Name: g.nodeName(n, byID), Value: state}, id: "node:" + id}}
		p.SetBounds(box)
		for i, r := range g.toggleRects(g.nodeBox[id], len(n.Toggles)) {
			t := n.Toggles[i]
			name := t.Label
			if name == "" {
				name = t.ID
			}
			v := "off"
			if t.On {
				v = "on"
			}
			tp := &isoProxy{info: A11yInfo{Role: RoleSwitch, Name: name, Value: v}, id: "toggle:" + id + "/" + t.ID}
			if tb, ok := graphClip(g.screen(r), g.Bounds()); ok {
				tp.SetBounds(tb)
				p.kids = append(p.kids, tp)
			}
		}
		out = append(out, p)
	}
	return out
}

// graphClip is r cut to the bounds b; false when nothing of r is inside.
func graphClip(r, b Rect) (Rect, bool) {
	x0, y0 := max(r.X, b.X), max(r.Y, b.Y)
	x1, y1 := min(r.X+r.W, b.X+b.W), min(r.Y+r.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{}, false
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, true
}

// zoomNote is appended to the graph's value when the view is not at 100 %.
func (g *GraphView) zoomNote() string {
	if z := g.zoomPct(); z != 100 {
		return "; zoom " + strconv.Itoa(z) + "%"
	}
	return ""
}

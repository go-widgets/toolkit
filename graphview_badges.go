package toolkit

import "github.com/go-widgets/painter"

// GraphBadge is a small chip under a node's label: a glyph and a text, for a
// fact worth a glance ("muted", "no egress", "exposes 3000/tcp").
//
// Icon, when set and resolved by GraphView.Icons, is drawn instead of Glyph.
type GraphBadge struct {
	Glyph, Text string
	Icon        string
}

// GraphToggle is a glyph button at the right end of a node, such as a speaker.
// The view does not own the state: On is the host's, flipped in the model
// from the GraphView.OnToggle callback.
//
// Icon, when set and resolved by GraphView.Icons, is drawn instead of Glyph; a
// speaker on or off is two names chosen from On by the host.
type GraphToggle struct {
	ID, Glyph, Label string
	On               bool
	Icon             string
}

// iconDrawer resolves an icon name through GraphView.Icons; nil when the name
// is empty, no resolver is set or the resolver does not know the name, so the
// caller falls back to the text Glyph.
func (g *GraphView) iconDrawer(name string) func(p painter.Painter, r Rect, ink RGBA) {
	if name == "" || g.Icons == nil {
		return nil
	}
	return g.Icons(name)
}

// graphBadgeH is the height of the chip row, in painter units.
const graphBadgeH = 12

// nodeH is the height of a node box: the label row, plus one chip row when any
// node in the model has a badge.
func (g *GraphView) nodeH() int {
	if g.badged {
		return gNodeH() + scaled(graphBadgeH)
	}
	return gNodeH()
}

// topStrip is the height of the label row of a node drawn in box.
func (g *GraphView) topStrip(box Rect) int {
	h := box.H
	if g.badged {
		h = box.H * gNodeH() / g.nodeH()
	}
	return h
}

// toggleRects places n toggles at the right end of the node box cb (content
// coordinates), the first leftmost.
func (g *GraphView) toggleRects(cb Rect, n int) []Rect {
	side := gNodeH() - 2
	out := make([]Rect, n)
	for i := range out {
		out[i] = Rect{X: cb.X + cb.W - (n-i)*(side+1), Y: cb.Y + 1, W: side, H: side}
	}
	return out
}

// toggleAt returns the toggle under the widget-space point (x, y).
func (g *GraphView) toggleAt(x, y int) (nodeID, toggleID string, ok bool) {
	g.layout()
	x, y = g.cx(x), g.cy(y)
	for _, n := range g.Model.Nodes.Slice() {
		cb, placed := g.nodeBox[n.ID]
		if !placed {
			continue
		}
		for i, r := range g.toggleRects(cb, len(n.Toggles)) {
			if r.Contains(x, y) {
				return n.ID, n.Toggles[i].ID, true
			}
		}
	}
	return "", "", false
}

// drawToggles paints the toggles of n in tr (content rects): an On toggle is a
// solid chip in the label ink, an Off one an outline.
func (g *GraphView) drawToggles(p painter.Painter, n GraphNode, tr []Rect, col, ink RGBA) {
	f := g.zoomedFont()
	for i, t := range n.Toggles {
		r := g.screen(tr[i])
		if t.On {
			g.fillBox(p, r, g.CornerRadius, ink)
		} else {
			g.strokeBox(p, r, g.CornerRadius, ink)
		}
		glyph := col
		if !t.On {
			glyph = ink
		}
		if d := g.iconDrawer(t.Icon); d != nil {
			in := r.W / 6
			d(p, Rect{X: r.X + in, Y: r.Y + in, W: r.W - 2*in, H: r.H - 2*in}, glyph)
			continue
		}
		f.Draw(p, r.X+(r.W-f.Measure(t.Glyph))/2, r.Y+(r.H-f.Height())/2, t.Glyph, glyph)
	}
}

// drawBadges paints the chip row of n under the label row (top px high).
func (g *GraphView) drawBadges(p painter.Painter, n GraphNode, box Rect, top int, ink RGBA) {
	if len(n.Badges) > 0 {
		f := g.zoomedFont()
		x := box.X + 3
		withClip(p, box, func() {
			for _, b := range n.Badges {
				s := b.Text
				d := g.iconDrawer(b.Icon)
				if b.Glyph != "" && d == nil {
					s = b.Glyph + " " + s
				}
				h := box.H - top - 1
				tx, w := x+2, f.Measure(s)+4
				if d != nil {
					// The icon is a square the height of the chip, left of the text.
					side := max(h-2, 0)
					d(p, Rect{X: x + 2, Y: box.Y + top + (h-side)/2, W: side, H: side}, ink)
					tx, w = x+2+side+2, w+side+2
				}
				g.strokeBox(p, Rect{X: x, Y: box.Y + top, W: w, H: h}, 0, ink)
				f.Draw(p, tx, box.Y+top+(box.H-top-f.Height())/2, s, ink)
				x += w + 2
			}
		})
	}
}

// toggleSelected fires the first toggle of the selected node (Space).
func (g *GraphView) toggleSelected() {
	sel := g.Model.Selected.Get()
	if g.OnToggle == nil {
		return
	}
	for _, n := range g.Model.Nodes.Slice() {
		if "node:"+n.ID == sel && len(n.Toggles) > 0 {
			g.OnToggle(n.ID, n.Toggles[0].ID)
			return
		}
	}
}

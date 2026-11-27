package toolkit

import "github.com/go-widgets/painter"

// GraphBadge is a small chip under a node's label: a glyph and a text, for a
// fact worth a glance ("muted", "no egress", "exposes 3000/tcp").
type GraphBadge struct {
	Glyph, Text string
}

// GraphToggle is a glyph button at the right end of a node, such as a speaker.
// The view does not own the state: On is the host's, flipped in the model
// from the GraphView.OnToggle callback.
type GraphToggle struct {
	ID, Glyph, Label string
	On               bool
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
				if b.Glyph != "" {
					s = b.Glyph + " " + s
				}
				w := f.Measure(s) + 4
				g.strokeBox(p, Rect{X: x, Y: box.Y + top, W: w, H: box.H - top - 1}, 0, ink)
				f.Draw(p, x+2, box.Y+top+(box.H-top-f.Height())/2, s, ink)
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

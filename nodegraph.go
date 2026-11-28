// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// Node geometry, in graph units (scaled by the zoom, not by the metric scale:
// the zoom is the user's scale).
const (
	ngNodeW   = 140.0
	ngHeaderH = 22.0
	ngRowH    = 18.0
	ngPad     = 6.0
	ngPortR   = 5.0
	ngSegs    = 16 // a bezier edge is flattened into this many segments
	// ngWheelStep is the factor one wheel notch zooms by.
	ngWheelStep = 1.1
	// ngPanStep is the pan, in pixels, of one wheel row.
	ngPanStep = 24
	// ngEdgeSlack and ngPortSlack are the hit radii, in pixels.
	ngEdgeSlack = 5.0
	ngPortSlack = 8.0
)

// NodeGraphHitKind says what a hit is.
type NodeGraphHitKind int

const (
	// NodeGraphHitNone is the background.
	NodeGraphHitNone NodeGraphHitKind = iota
	// NodeGraphHitNode is a node's box.
	NodeGraphHitNode
	// NodeGraphHitPort is a port glyph: Node, Port and Output say which.
	NodeGraphHitPort
	// NodeGraphHitEdge is an edge's curve.
	NodeGraphHitEdge
)

// NodeGraphHit is the result of NodeGraph.Hit. Node is set for a node and a
// port, Edge for an edge; Output is true for an output port.
type NodeGraphHit struct {
	Kind             NodeGraphHitKind
	Node, Port, Edge string
	Output           bool
}

type ngMode int

const (
	ngIdle ngMode = iota
	ngMoving
	ngConnecting
	ngPanning
)

// NodeGraph is an editable node graph: boxes with typed ports on their sides,
// joined by bezier edges.
//
//   - Drag a node by its box to move it; drag the background to pan; Ctrl or
//     Cmd + wheel zooms around the pointer, the plain wheel pans.
//   - Drag from an output port and release over an input port to connect them;
//     the model decides whether the pair is allowed (NodeGraphModel.Compatible)
//     and ports that would be accepted are ringed while dragging.
//   - Click a node or an edge to select it; Delete or Backspace removes it;
//     the arrow keys move the selection between nodes, Escape cancels a
//     connection or clears the selection; + - 0 zoom in, out and reset.
//
// All hit-testing is in the widget (Hit). Every edit goes through the model, so
// a host serialises the graph from NodeGraphModel.Subscribe's stream.
type NodeGraph struct {
	Base
	focusState
	// Model is the data (set-once composition; its contents are reactive).
	Model *NodeGraphModel
	// OnReject is called with the model's reason when a drop is refused (an
	// incompatible or duplicate connection). Optional.
	OnReject func(err error)

	mode     ngMode
	from     NodeGraphHit // the output port a connection starts at
	node     string       // the node being moved
	moved    bool
	grabX    float64 // the pointer's offset inside the moved node, graph units
	grabY    float64
	curX     int // the pointer, surface coordinates, while connecting
	curY     int
	panStart NodeGraphViewport
	panFromX int
	panFromY int

	curve    [ngSegs + 1][2]float64
	zoomFont Font
	zoomBase Font
	zoomAt   float64
}

// NewNodeGraph returns a NodeGraph over m (a new empty model when nil).
func NewNodeGraph(m *NodeGraphModel) *NodeGraph {
	if m == nil {
		m = NewNodeGraphModel()
	}
	return &NodeGraph{Model: m}
}

// viewport is the model's view made safe: a zoom in range and a bounded pan,
// whatever a host stored.
func (g *NodeGraph) viewport() NodeGraphViewport {
	v := g.Model.View.Get()
	if math.IsNaN(v.Zoom) || v.Zoom < NodeGraphZoomMin {
		v.Zoom = NodeGraphZoomMin
	}
	v.Zoom = math.Min(v.Zoom, NodeGraphZoomMax)
	v.PanX, v.PanY = clampCoord(v.PanX), clampCoord(v.PanY)
	return v
}

// ZoomBy multiplies the zoom by f (clamped), keeping the graph point under the
// widget-local (ax, ay) where it is.
func (g *NodeGraph) ZoomBy(f float64, ax, ay int) {
	v := g.viewport()
	z := math.Max(NodeGraphZoomMin, math.Min(NodeGraphZoomMax, v.Zoom*f))
	k := z / v.Zoom
	g.Model.View.Set(NodeGraphViewport{Zoom: z, PanX: float64(ax) - (float64(ax)-v.PanX)*k, PanY: float64(ay) - (float64(ay)-v.PanY)*k})
}

// Reset shows the graph at zoom 1 with no pan.
func (g *NodeGraph) Reset() { g.Model.View.Set(NodeGraphViewport{Zoom: 1}) }

// toScreen maps a graph point to surface coordinates.
func (g *NodeGraph) toScreen(v NodeGraphViewport, x, y float64) (float64, float64) {
	b := g.Bounds()
	return float64(b.X) + x*v.Zoom + v.PanX, float64(b.Y) + y*v.Zoom + v.PanY
}

// toGraph maps surface coordinates to a graph point.
func (g *NodeGraph) toGraph(v NodeGraphViewport, x, y int) (float64, float64) {
	b := g.Bounds()
	return (float64(x-b.X) - v.PanX) / v.Zoom, (float64(y-b.Y) - v.PanY) / v.Zoom
}

// nodeSize is the node's box in graph units: a header and one row per port of
// its taller side.
func nodeSize(n NodeGraphNode) (w, h float64) {
	return ngNodeW, ngHeaderH + float64(max(len(n.Inputs), len(n.Outputs)))*ngRowH + ngPad
}

// portPoint is the centre of port i of n, in graph units.
func portPoint(n NodeGraphNode, output bool, i int) (x, y float64) {
	x = n.X
	if output {
		x += ngNodeW
	}
	return x, n.Y + ngHeaderH + (float64(i)+0.5)*ngRowH
}

// portIndex returns the index of port id in a side's ports, or -1.
func portIndex(ports []NodeGraphPort, id string) int {
	for i, p := range ports {
		if p.ID == id {
			return i
		}
	}
	return -1
}

// flatten fills g.curve with the bezier from (x0, y0) to (x1, y1): horizontal
// tangents, as a data-flow edge leaves an output and enters an input.
func (g *NodeGraph) flatten(x0, y0, x1, y1 float64) {
	d := math.Max(40, math.Abs(x1-x0)/2)
	ax, bx := x0+d, x1-d
	for i := 0; i <= ngSegs; i++ {
		t := float64(i) / ngSegs
		u := 1 - t
		a, b, c, e := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
		g.curve[i] = [2]float64{a*x0 + b*ax + c*bx + e*x1, a*y0 + b*y0 + c*y1 + e*y1}
	}
}

// edgeEnds returns an edge's endpoints in surface coordinates. The model keeps
// both of its nodes and ports alive for as long as the edge exists.
func (g *NodeGraph) edgeEnds(v NodeGraphViewport, e NodeGraphEdge) (x0, y0, x1, y1 float64) {
	m := g.Model
	from, to := m.nodes[m.nodeIndex(e.FromNode)], m.nodes[m.nodeIndex(e.ToNode)]
	gx0, gy0 := portPoint(from, true, portIndex(from.Outputs, e.FromPort))
	gx1, gy1 := portPoint(to, false, portIndex(to.Inputs, e.ToPort))
	x0, y0 = g.toScreen(v, gx0, gy0)
	x1, y1 = g.toScreen(v, gx1, gy1)
	return
}

// Hit returns what is under the surface point (x, y): a port first, then the
// topmost node, then an edge. It is the widget's own hit-test: no other part
// of the host needs to know the geometry.
func (g *NodeGraph) Hit(x, y int) NodeGraphHit {
	v := g.viewport()
	m := g.Model
	px, py := float64(x), float64(y)
	slack := math.Max(ngPortSlack, ngPortR*v.Zoom+3)
	for i := len(m.nodes) - 1; i >= 0; i-- {
		n := m.nodes[i]
		for side := 0; side < 2; side++ {
			ports := n.Inputs
			if side == 1 {
				ports = n.Outputs
			}
			for j, p := range ports {
				gx, gy := portPoint(n, side == 1, j)
				sx, sy := g.toScreen(v, gx, gy)
				if math.Hypot(px-sx, py-sy) <= slack {
					return NodeGraphHit{Kind: NodeGraphHitPort, Node: n.ID, Port: p.ID, Output: side == 1}
				}
			}
		}
	}
	gx, gy := g.toGraph(v, x, y)
	for i := len(m.nodes) - 1; i >= 0; i-- {
		n := m.nodes[i]
		w, h := nodeSize(n)
		if gx >= n.X && gx < n.X+w && gy >= n.Y && gy < n.Y+h {
			return NodeGraphHit{Kind: NodeGraphHitNode, Node: n.ID}
		}
	}
	for i := len(m.edges) - 1; i >= 0; i-- {
		e := m.edges[i]
		x0, y0, x1, y1 := g.edgeEnds(v, e)
		g.flatten(x0, y0, x1, y1)
		for k := 0; k < ngSegs; k++ {
			if segDist(px, py, g.curve[k][0], g.curve[k][1], g.curve[k+1][0], g.curve[k+1][1]) <= ngEdgeSlack {
				return NodeGraphHit{Kind: NodeGraphHitEdge, Edge: e.ID}
			}
		}
	}
	return NodeGraphHit{}
}

// ngColor is the colour of a port type: the same type always gets the same
// colour from the toolkit's colour-blind-safe palette.
func ngColor(t string) RGBA {
	h := uint32(2166136261)
	for i := 0; i < len(t); i++ {
		h = (h ^ uint32(t[i])) * 16777619
	}
	return graphDefaultPalette[h%uint32(len(graphDefaultPalette))]
}

// font is the widget's font at the zoom, kept until either changes.
func (g *NodeGraph) font(zoom float64) Font {
	base := g.EffectiveFont()
	if g.zoomFont == nil || g.zoomBase != base || g.zoomAt != zoom {
		g.zoomBase, g.zoomAt, g.zoomFont = base, zoom, ScaleFont(base, zoom)
	}
	return g.zoomFont
}

// strokeCurve draws g.curve: one anti-aliased path on a pixel back-end, line
// segments elsewhere. Selected edges are drawn twice, a pixel apart.
func (g *NodeGraph) strokeCurve(p painter.Painter, c RGBA, width float64) {
	if pp, ok := p.(painter.PathPainter); ok {
		path := painter.NewPath().MoveTo(g.curve[0][0], g.curve[0][1])
		for _, q := range g.curve[1:] {
			path.LineTo(q[0], q[1])
		}
		pp.StrokePath(path, c, width)
		return
	}
	for k := 0; k < ngSegs; k++ {
		a, b := g.curve[k], g.curve[k+1]
		drawLine(p, int(a[0]), int(a[1]), int(b[0]), int(b[1]), c)
	}
}

// Draw paints the edges, the connection being dragged, then the nodes.
func (g *NodeGraph) Draw(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Background)
	v := g.viewport()
	m := g.Model
	sel := m.Selected.Get()
	withClip(p, r, func() {
		for _, e := range m.edges {
			x0, y0, x1, y1 := g.edgeEnds(v, e)
			g.flatten(x0, y0, x1, y1)
			if sel == "edge:"+e.ID {
				g.strokeCurve(p, theme.Accent, 3)
			} else {
				g.strokeCurve(p, theme.OnSurface, 1.5)
			}
		}
		if g.mode == ngConnecting {
			if n, ok := m.Node(g.from.Node); ok {
				gx, gy := portPoint(n, true, portIndex(n.Outputs, g.from.Port))
				x0, y0 := g.toScreen(v, gx, gy)
				g.flatten(x0, y0, float64(g.curX), float64(g.curY))
				g.strokeCurve(p, theme.Accent, 2)
			}
		}
		for _, n := range m.nodes {
			g.drawNode(p, theme, v, n, sel == "node:"+n.ID)
		}
	})
	g.drawFocusRing(p, theme, r)
}

func (g *NodeGraph) drawNode(p painter.Painter, theme *Theme, v NodeGraphViewport, n NodeGraphNode, selected bool) {
	x, y := g.toScreen(v, n.X, n.Y)
	w, h := nodeSize(n)
	box := Rect{X: int(x), Y: int(y), W: int(w * v.Zoom), H: int(h * v.Zoom)}
	if r := g.Bounds(); box.X >= r.X+r.W || box.Y >= r.Y+r.H || box.X+box.W <= r.X || box.Y+box.H <= r.Y {
		return // off screen
	}
	rad := int(4 * v.Zoom)
	fillRoundRect(p, box.X, box.Y, box.W, box.H, rad, theme.Surface)
	hdr := Rect{X: box.X, Y: box.Y, W: box.W, H: int(ngHeaderH * v.Zoom)}
	fillRect(p, hdr.X+1, hdr.Y+1, hdr.W-2, hdr.H-1, blendRGBA(theme.Accent, theme.Surface, 0.3))
	border := theme.Border
	if selected {
		border = theme.Accent
	}
	strokeRoundRect(p, box.X, box.Y, box.W, box.H, rad, border)
	text := v.Zoom >= 0.5
	f := g.font(v.Zoom)
	if text {
		withClip(p, hdr, func() { f.Draw(p, hdr.X+int(ngPad*v.Zoom), hdr.Y+(hdr.H-f.Height())/2, n.Label, theme.OnSurface) })
	}
	connecting := g.mode == ngConnecting
	for side := 0; side < 2; side++ {
		ports := n.Inputs
		if side == 1 {
			ports = n.Outputs
		}
		for i, pt := range ports {
			gx, gy := portPoint(n, side == 1, i)
			cx, cy := g.toScreen(v, gx, gy)
			pr := max(2, int(ngPortR*v.Zoom))
			ring := connecting && side == 0 && g.Model.CanConnect(g.from.Node, g.from.Port, n.ID, pt.ID) == nil
			if ring {
				fillRoundRect(p, int(cx)-pr-2, int(cy)-pr-2, 2*pr+4, 2*pr+4, pr+2, theme.Accent)
			}
			fillRoundRect(p, int(cx)-pr, int(cy)-pr, 2*pr, 2*pr, pr, ngColor(pt.Type))
			if text {
				lx := int(cx) + pr + 3
				if side == 1 {
					lx = int(cx) - pr - 3 - f.Measure(pt.Label)
				}
				f.Draw(p, lx, int(cy)-f.Height()/2, pt.Label, theme.OnSurface)
			}
		}
	}
}

// OnEvent: see the type's documentation. Coordinates are widget-local.
func (g *NodeGraph) OnEvent(ev Event) {
	if g.isDisabled() {
		return
	}
	r := g.Bounds()
	switch ev.Kind {
	case EventClick:
		g.press(ev, r)
	case EventMouseDrag:
		g.drag(ev, r)
	case EventMouseUp:
		g.release(ev, r)
	case EventScroll:
		switch {
		case ev.Ctrl || ev.Meta:
			g.ZoomBy(math.Pow(ngWheelStep, -float64(ev.Delta)), ev.X, ev.Y)
		default:
			v := g.viewport()
			v.PanX -= float64(ev.DeltaX * ngPanStep)
			v.PanY -= float64(ev.Delta * ngPanStep)
			g.Model.View.Set(v)
		}
	case EventKeyDown:
		g.key(ev.Code)
	}
}

func (g *NodeGraph) press(ev Event, r Rect) {
	h := g.Hit(r.X+ev.X, r.Y+ev.Y)
	g.curX, g.curY = r.X+ev.X, r.Y+ev.Y
	g.moved = false
	switch {
	case h.Kind == NodeGraphHitPort && h.Output:
		g.mode, g.from = ngConnecting, h
	case h.Node != "":
		g.mode, g.node = ngMoving, h.Node
		n, _ := g.Model.Node(h.Node)
		gx, gy := g.toGraph(g.viewport(), g.curX, g.curY)
		g.grabX, g.grabY = gx-n.X, gy-n.Y
		g.Model.Selected.Set("node:" + h.Node)
	case h.Kind == NodeGraphHitEdge:
		g.mode = ngIdle
		g.Model.Selected.Set("edge:" + h.Edge)
	default:
		g.mode = ngPanning
		g.panStart, g.panFromX, g.panFromY = g.viewport(), ev.X, ev.Y
		g.Model.Selected.Set("")
	}
}

func (g *NodeGraph) drag(ev Event, r Rect) {
	g.curX, g.curY = r.X+ev.X, r.Y+ev.Y
	switch g.mode {
	case ngMoving:
		gx, gy := g.toGraph(g.viewport(), g.curX, g.curY)
		g.moved = g.Model.setPos(g.node, gx-g.grabX, gy-g.grabY) || g.moved
	case ngPanning:
		v := g.panStart
		v.PanX += float64(ev.X - g.panFromX)
		v.PanY += float64(ev.Y - g.panFromY)
		g.Model.View.Set(v)
	}
}

func (g *NodeGraph) release(ev Event, r Rect) {
	mode := g.mode
	g.mode = ngIdle
	switch {
	case mode == ngMoving && g.moved:
		g.Model.emit(NodeMoved, g.node)
	case mode == ngConnecting:
		h := g.Hit(r.X+ev.X, r.Y+ev.Y)
		if h.Kind != NodeGraphHitPort || h.Output {
			return // released on nothing: the connection is dropped
		}
		err := g.Model.Connect(NodeGraphEdge{FromNode: g.from.Node, FromPort: g.from.Port, ToNode: h.Node, ToPort: h.Port})
		if err != nil && g.OnReject != nil {
			g.OnReject(err)
		}
	}
}

func (g *NodeGraph) key(code string) {
	m := g.Model
	switch code {
	case "Delete", "Backspace":
		m.DeleteSelected()
	case "Escape":
		g.mode = ngIdle
		m.Selected.Set("")
	case "ArrowRight", "ArrowDown", "ArrowLeft", "ArrowUp":
		g.step(code == "ArrowRight" || code == "ArrowDown")
	case "+", "=", "Equal", "NumpadAdd":
		g.ZoomBy(ngWheelStep, g.Bounds().W/2, g.Bounds().H/2)
	case "-", "_", "Minus", "NumpadSubtract":
		g.ZoomBy(1/ngWheelStep, g.Bounds().W/2, g.Bounds().H/2)
	case "0", "Digit0", "Numpad0":
		g.Reset()
	}
}

// step moves the selection to the next or previous node, in model order.
func (g *NodeGraph) step(forward bool) {
	nodes := g.Model.nodes
	if len(nodes) == 0 {
		return
	}
	cur := -1
	if id, ok := strings.CutPrefix(g.Model.Selected.Get(), "node:"); ok {
		cur = g.Model.nodeIndex(id)
	}
	next := cur - 1
	if forward {
		next = cur + 1
	}
	g.Model.Selected.Set("node:" + nodes[clampInt(next, 0, len(nodes)-1)].ID)
}

// A11y describes the graph: how many nodes and edges, and the selection.
func (g *NodeGraph) A11y() A11yInfo {
	v := strconv.Itoa(len(g.Model.nodes)) + " nodes, " + strconv.Itoa(len(g.Model.edges)) + " edges"
	if s := g.Model.Selected.Get(); s != "" {
		v += "; selected " + s
	}
	return A11yInfo{Role: RoleGroup, Name: "node graph editor", Value: v, Disabled: g.isDisabled()}
}

func (*NodeGraph) a11yDisablesSubtree() bool { return true }

func portNames(ports []NodeGraphPort) string {
	names := make([]string, len(ports))
	for i, p := range ports {
		names[i] = p.Label
		if p.Label == "" {
			names[i] = p.ID
		}
	}
	return strings.Join(names, ", ")
}

func ngLabel(n NodeGraphNode) string {
	if n.Label != "" {
		return n.Label
	}
	return n.ID
}

// Children exposes the nodes as tree items and the edges as list items, over
// their drawn boxes; the proxies are never drawn.
func (g *NodeGraph) Children() []Widget {
	v, m := g.viewport(), g.Model
	sel := m.Selected.Get()
	var out []Widget
	for _, n := range m.nodes {
		x, y := g.toScreen(v, n.X, n.Y)
		w, h := nodeSize(n)
		box, ok := graphClip(Rect{X: int(x), Y: int(y), W: int(w * v.Zoom), H: int(h * v.Zoom)}, g.Bounds())
		if !ok {
			continue
		}
		name := ngLabel(n) + ", inputs: " + portNames(n.Inputs) + "; outputs: " + portNames(n.Outputs)
		state := ""
		if sel == "node:"+n.ID {
			state = "selected"
		}
		pr := &isoProxy{info: A11yInfo{Role: RoleTreeItem, Name: name, Value: state}, id: "node:" + n.ID}
		pr.SetBounds(box)
		out = append(out, pr)
	}
	for _, e := range m.edges {
		x0, y0, x1, y1 := g.edgeEnds(v, e)
		box, ok := graphClip(Rect{X: int(math.Min(x0, x1)), Y: int(math.Min(y0, y1)), W: int(math.Abs(x1-x0)) + 1, H: int(math.Abs(y1-y0)) + 1}, g.Bounds())
		if !ok {
			continue
		}
		from, _ := m.Node(e.FromNode)
		to, _ := m.Node(e.ToNode)
		state := ""
		if sel == "edge:"+e.ID {
			state = "selected"
		}
		pr := &isoProxy{info: A11yInfo{Role: RoleListItem, Name: "edge from " + ngLabel(from) + " to " + ngLabel(to), Value: state}, id: "edge:" + e.ID}
		pr.SetBounds(box)
		out = append(out, pr)
	}
	return out
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"strconv"
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// GraphCluster is a box that groups nodes: a host, a machine, a student's
// session. Clusters are laid out in the order of the view model's list.
type GraphCluster struct {
	ID, Label string
}

// GraphNode is one node of a GraphView. Cluster names the GraphCluster it
// sits in (a node whose cluster is unknown is not drawn). Category is a colour
// key: nodes of one category share a colour, assigned in order of first
// appearance from GraphView.Palette.
type GraphNode struct {
	ID, Label, Cluster, Category string
}

// GraphEdgeState says how an edge is drawn.
type GraphEdgeState int

const (
	// GraphEdgeNormal is a live edge, in the theme's ink.
	GraphEdgeNormal GraphEdgeState = iota
	// GraphEdgeDenied is a refused edge: drawn dashed in the error colour, the
	// thing a viewer must notice.
	GraphEdgeDenied
	// GraphEdgeIdle is an edge with no recent traffic, drawn dimmed.
	GraphEdgeIdle
)

// GraphEdge is one directed edge From -> To. Weight (bytes per second, or any
// non-negative magnitude) sets the stroke width on a logarithmic scale;
// Label is shown when the edge is selected.
type GraphEdge struct {
	ID, From, To, Label string
	Weight              float64
	State               GraphEdgeState
}

// GraphViewModel is everything a GraphView shows, as MVVM handles: a host
// (a relay's flow map, a dashboard's network page) fills the lists from its
// own data source and binds Selected and Filter. The widget never knows where
// the data comes from.
type GraphViewModel struct {
	Clusters *mvvm.ObservableList[GraphCluster]
	Nodes    *mvvm.ObservableList[GraphNode]
	Edges    *mvvm.ObservableList[GraphEdge]
	// Selected is "node:<id>", "edge:<id>" or "" (nothing).
	Selected *mvvm.Observable[string]
	// Filter dims every node whose label, cluster label and category do not
	// contain it (case-insensitive), and every edge touching only dimmed
	// nodes. Empty: nothing is dimmed.
	Filter *mvvm.Observable[string]
}

// NewGraphViewModel returns an empty view model.
func NewGraphViewModel() *GraphViewModel {
	return &GraphViewModel{
		Clusters: mvvm.NewObservableList[GraphCluster](),
		Nodes:    mvvm.NewObservableList[GraphNode](),
		Edges:    mvvm.NewObservableList[GraphEdge](),
		Selected: mvvm.NewObservable(""),
		Filter:   mvvm.NewObservable(""),
	}
}

// GraphView draws a node-link graph whose nodes sit in cluster boxes laid out
// on a stable grid: a node does not move when edges change, so a viewer can
// watch traffic come and go without the picture rearranging itself. The
// layout is recomputed only when clusters or nodes are added or removed, or
// the bounds change — never on an edge update.
//
// Click a node or an edge to select it (Selected on the model); the arrow
// keys move the selection between nodes in layout order, Escape clears it,
// Enter calls OnActivate. A legend (categories, and the denied style) is
// drawn along the bottom when ShowLegend is set.
type GraphView struct {
	Base
	focusState
	// Model is the data (set-once composition; its contents are reactive).
	Model *GraphViewModel
	// Palette colours the categories; nil uses a colour-blind-safe default.
	Palette []RGBA
	// MaxEdgeWidth caps an edge's stroke (default 6).
	MaxEdgeWidth int
	// NodeWidth is the width of a node box in painter units (default 72). A
	// host whose labels are longer than a few characters sets it once, before
	// the first draw; a node's label is clipped to its box.
	NodeWidth int
	// CornerRadius rounds the node pills and, by half as much again, the
	// cluster boxes, in logical pixels (default 0: square, as before). A
	// set-once appearance config, like NodeWidth.
	CornerRadius int
	// ShowLegend draws the category legend along the bottom edge.
	ShowLegend bool
	// OnActivate is called with the selection on Enter or a double selection.
	OnActivate func(selected string)

	layoutValid bool
	layoutFor   Rect
	clusterBox  map[string]Rect
	nodeBox     map[string]Rect
	order       []string // node ids in layout order
	categories  map[string]int
	unsub       []func()
	bound       *GraphViewModel

	// Vertical scrolling: when the clusters need more height than the widget
	// has, scrollY (px, >= 0) is how far the content is moved up, contentH its
	// full height and viewH the height it shows in (the bounds minus the legend).
	laidNodeW                int // the node width the layout was computed for
	scrollY, contentH, viewH int
	hasBar                   bool
	sbDrag                   scrollDrag

	// Zoom and pan (graphview_zoom.go). panX is the horizontal pan and, with
	// scrollY, the offset published on pan; contentW is the clusters' width.
	zoom      *mvvm.Observable[int]
	pan       *mvvm.Observable[GraphPoint]
	pushed    GraphPoint
	panX      int
	contentW  int
	laidZoom  int
	bgDrag    bool // a drag that started on the background pans
	dragFrom  GraphPoint
	dragPan   GraphPoint
	mt        *MultiTouchRecognizer
	pinchBase int

	// The labels' font at the zoom (zoomedFont): the font it was made from and
	// the zoom, so it is built again only when one of them changes.
	zoomFont    Font
	zoomBase    Font
	zoomFontPct int
}

// NewGraphView returns a GraphView over m (a new empty model when nil).
func NewGraphView(m *GraphViewModel) *GraphView {
	if m == nil {
		m = NewGraphViewModel()
	}
	g := &GraphView{Model: m}
	g.bind()
	return g
}

// graphDefaultPalette is the Okabe-Ito palette, distinguishable with the
// common colour-vision deficiencies.
var graphDefaultPalette = []RGBA{
	RGB(0xE6, 0x9F, 0x00), RGB(0x56, 0xB4, 0xE9), RGB(0x00, 0x9E, 0x73), RGB(0xF0, 0xE4, 0x42),
	RGB(0x00, 0x72, 0xB2), RGB(0xD5, 0x5E, 0x00), RGB(0xCC, 0x79, 0xA7), RGB(0x99, 0x99, 0x99),
}

// bind subscribes to the model's structure: adding or removing a cluster or
// a node invalidates the layout. Edges are read at draw time and never do.
func (g *GraphView) bind() {
	if g.bound == g.Model {
		return
	}
	for _, u := range g.unsub {
		u()
	}
	g.unsub = nil
	g.bound = g.Model
	g.layoutValid = false
	inv := func() { g.layoutValid = false }
	g.unsub = append(g.unsub, g.Model.Clusters.SubscribeChanged(inv), g.Model.Nodes.SubscribeChanged(inv))
}

// SetBounds places the widget; a new size invalidates the layout.
func (g *GraphView) SetBounds(r Rect) {
	g.Base.SetBounds(r)
	if r != g.layoutFor {
		g.layoutValid = false
	}
}

// Layout geometry, in painter units.
const (
	graphNodeW    = 72
	graphNodeH    = 18
	graphGap      = 6
	graphTitleH   = 14
	graphHitSlack = 3
)

// clusterRadius is the rounding of a cluster box: half again the node pill's.
func (g *GraphView) clusterRadius() int { return g.CornerRadius * 3 / 2 }

// fillBox fills r, rounded by radius when radius > 0, square otherwise.
func (g *GraphView) fillBox(p painter.Painter, r Rect, radius int, c RGBA) {
	if radius > 0 {
		fillRoundRect(p, r.X, r.Y, r.W, r.H, scaled(radius), c)
		return
	}
	fillRect(p, r.X, r.Y, r.W, r.H, c)
}

// strokeBox outlines r, rounded by radius when radius > 0, square otherwise.
func (g *GraphView) strokeBox(p painter.Painter, r Rect, radius int, c RGBA) {
	if radius > 0 {
		strokeRoundRect(p, r.X, r.Y, r.W, r.H, scaled(radius), c)
		return
	}
	strokeRect(p, r.X, r.Y, r.W, r.H, c)
}

// nodeW is the width of a node box.
func (g *GraphView) nodeW() int {
	if g.NodeWidth > 0 {
		return scaled(g.NodeWidth)
	}
	return scaled(graphNodeW)
}

// The layout metrics, scaled by the global metric scale ([SetMetricScale]) so a
// graph drawn on a HiDPI surface grows with its text.
func gGap() int    { return scaled(graphGap) }
func gNodeH() int  { return scaled(graphNodeH) }
func gTitleH() int { return scaled(graphTitleH) }

// layout places clusters on a grid filling the bounds and nodes on a sub-grid
// inside each cluster. Deterministic: the same lists and bounds give the
// same boxes.
func (g *GraphView) layout() {
	g.bind()
	nw := g.nodeW()
	g.pullPan()
	g.zoomPct()
	// A change of node width (NodeWidth set late, or the metric scale changed)
	// lays out again; an edge update never does.
	if g.layoutValid && g.laidNodeW == nw {
		return
	}
	r := g.Bounds()
	g.layoutFor = r
	g.layoutValid = true
	g.laidNodeW = nw
	g.clusterBox = map[string]Rect{}
	g.nodeBox = map[string]Rect{}
	g.order = g.order[:0]
	clusters := g.Model.Clusters.Slice()
	nodes := g.Model.Nodes.Slice()
	members := map[string][]GraphNode{}
	for _, n := range nodes {
		members[n.Cluster] = append(members[n.Cluster], n)
	}
	g.contentH, g.viewH, g.hasBar = 0, 0, false
	if len(clusters) == 0 {
		g.scrollY, g.panX, g.contentW = 0, 0, 0
		g.syncPan()
		return
	}
	area := r
	for _, n := range nodes { // the legend's entries, in the order the nodes name them
		g.categoryColor(n.Category)
	}
	if g.ShowLegend {
		area.H -= g.legendHeight()
	}
	g.viewH = area.H
	// Each cluster is sized by its OWN members and the clusters flow left to
	// right, wrapping to a new row. When the rows need more height than the
	// widget has, a scrollbar takes its column and the rows are flowed again in
	// what is left of the width.
	// A cluster is at least as wide as its title, so a long label never
	// spills over the next cluster.
	titleW := func(c GraphCluster) int { return g.textWidth(c.Label) + 2*gGap() }
	boxes, perRows, h := flowClusters(clusters, members, area.W, nw, titleW)
	if h > area.H {
		g.hasBar = true
		boxes, perRows, h = flowClusters(clusters, members, area.W-scrollbarTrack(), nw, titleW)
	}
	g.contentH = h
	g.contentW = 0
	for _, b := range boxes {
		g.contentW = maxInt(g.contentW, b.X+b.W+gGap())
	}
	g.scrollY = clampInt(g.scrollY, 0, g.maxScrollY())
	g.panX = clampInt(g.panX, 0, g.maxPanX())
	g.syncPan()
	for i, c := range clusters {
		b := boxes[i]
		box := Rect{X: area.X + b.X, Y: area.Y + b.Y - g.scrollY, W: b.W, H: b.H}
		g.clusterBox[c.ID] = box
		for j, n := range members[c.ID] {
			nc, nr := j%perRows[i], j/perRows[i]
			g.nodeBox[n.ID] = Rect{X: box.X + gGap() + nc*(nw+gGap()),
				Y: box.Y + gTitleH() + gGap() + nr*(gNodeH()+gGap()), W: min(nw, max(box.W-2*gGap(), 1)), H: gNodeH()}
			g.order = append(g.order, n.ID)
		}
	}
}

// flowClusters sizes each cluster by its own member count and flows them left
// to right in the width, wrapping when the next one does not fit. It returns
// the boxes relative to the content origin, each cluster's nodes per row, and
// the content height.
func flowClusters(clusters []GraphCluster, members map[string][]GraphNode, width, nodeW int, titleW func(GraphCluster) int) (boxes []Rect, perRows []int, height int) {
	gap := gGap()
	x, y, rowH := gap, gap, 0
	for _, c := range clusters {
		n := maxInt(1, len(members[c.ID]))
		perRow := int(math.Ceil(math.Sqrt(float64(n))))
		// A cluster never grows wider than the view: past that its nodes wrap into
		// more rows (the box grows taller) and its title is clipped to the box.
		capW := width - 2*gap
		if fit := (capW - gap) / (nodeW + gap); capW > 0 && fit < perRow {
			perRow = maxInt(1, fit)
		}
		rowsIn := (n + perRow - 1) / perRow
		cw := maxInt(perRow*(nodeW+gap)+gap, titleW(c))
		if capW > 0 {
			cw = min(cw, capW)
		}
		ch := gTitleH() + rowsIn*(gNodeH()+gap) + gap
		if x > gap && x+cw+gap > width {
			x, y, rowH = gap, y+rowH+gap, 0
		}
		boxes = append(boxes, Rect{X: x, Y: y, W: cw, H: ch})
		perRows = append(perRows, perRow)
		x += cw + gap
		rowH = maxInt(rowH, ch)
	}
	return boxes, perRows, y + rowH + gap
}

// graphScrollStep is the pixels one wheel row moves the content.
func graphScrollStep() int { return gNodeH() + gGap() }

// graphThumbMin is the smallest scrollbar thumb, in pixels.
const graphThumbMin = 12

// scrollbarGeom is the vertical scrollbar's widget-local geometry, and whether
// it is live (the clusters overflow the view).
func (g *GraphView) scrollbarGeom() (sbGeom, bool) {
	g.layout()
	// Live when the content, at the zoom, is taller than the view (at 100 % that
	// is the layout's own overflow test).
	if g.contentH*g.zoomPct() <= g.viewH*100 {
		return sbGeom{}, false
	}
	r := g.Bounds()
	maxScroll := g.maxScrollY()
	thumb := maxInt(graphThumbMin, g.viewH*g.viewH*100/(g.contentH*g.zoomPct()))
	return sbGeom{
		cross0:     r.W - scrollbarTrack(),
		trackStart: 0,
		trackLen:   g.viewH,
		thumbStart: g.scrollY * (g.viewH - thumb) / maxScroll,
		thumbLen:   thumb,
		travelNum:  g.viewH - thumb,
		travelDen:  maxScroll,
		maxScroll:  maxScroll,
	}, true
}

// ScrollTo scrolls the content so px pixels of it are above the view, clamped.
func (g *GraphView) ScrollTo(px int) {
	g.layout()
	px = clampInt(px, 0, g.maxScrollY())
	if px != g.scrollY {
		g.scrollY = px
		g.layoutValid = false
		g.syncPan()
	}
}

// ScrollBy scrolls by delta pixels.
func (g *GraphView) ScrollBy(delta int) {
	g.layout()
	g.ScrollTo(g.scrollY + delta)
}

// ScrollY is how far the content is scrolled, in pixels.
func (g *GraphView) ScrollY() int { return g.scrollY }

// revealNode scrolls just enough to bring the node into the view.
func (g *GraphView) revealNode(id string) {
	b, ok := g.NodeBox(id)
	if !ok {
		return
	}
	// b is in surface pixels; the scroll and the pan are in content pixels.
	top, left := g.Bounds().Y, g.Bounds().X
	z := g.zoomPct()
	inContent := func(px int) int { return rnd(float64(px) * 100 / float64(z)) }
	switch {
	case b.Y < top:
		g.ScrollBy(inContent(b.Y-top) - gGap())
	case b.Y+b.H > top+g.viewH:
		g.ScrollBy(inContent(b.Y+b.H-(top+g.viewH)) + gGap())
	}
	b, _ = g.NodeBox(id)
	switch {
	case b.X < left:
		g.PanBy(inContent(b.X-left) - gGap())
	case b.X+b.W > left+g.Bounds().W:
		g.PanBy(inContent(b.X+b.W-(left+g.Bounds().W)) + gGap())
	}
}

// NodeBox is the box a node is drawn in (ok=false when it is not drawn), for
// hosts and tests.
func (g *GraphView) NodeBox(id string) (Rect, bool) {
	g.layout()
	b, ok := g.nodeBox[id]
	return g.screen(b), ok
}

func (g *GraphView) palette() []RGBA {
	if len(g.Palette) > 0 {
		return g.Palette
	}
	return graphDefaultPalette
}

// categoryColor gives each category a stable colour by first appearance.
func (g *GraphView) categoryColor(cat string) RGBA {
	if g.categories == nil {
		g.categories = map[string]int{}
	}
	i, ok := g.categories[cat]
	if !ok {
		i = len(g.categories)
		g.categories[cat] = i
	}
	pal := g.palette()
	return pal[i%len(pal)]
}

// graphDeniedInk is the error colour: the theme's error_color when it has
// one, else a red chosen for the background's lightness.
func graphDeniedInk(theme *Theme) RGBA {
	if c, ok := theme.Extra["error_color"]; ok {
		return c
	}
	if luminance(theme.Background) > 0.5 {
		return RGB(0xC6, 0x28, 0x28)
	}
	return RGB(0xFF, 0x6B, 0x6B)
}

// edgeWidth maps a weight to a stroke width: 1 for idle, then one more per
// order of magnitude, capped.
func (g *GraphView) edgeWidth(w float64) int {
	mx := g.MaxEdgeWidth
	if mx <= 0 {
		mx = 6
	}
	if w <= 0 {
		return 1
	}
	return clampInt(1+int(math.Log10(1+w)/2), 1, mx)
}

func graphCenter(r Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// matches reports whether a node passes the filter.
func (g *GraphView) matches(n GraphNode, clusterLabel string) bool {
	f := strings.ToLower(g.Model.Filter.Get())
	if f == "" {
		return true
	}
	return strings.Contains(strings.ToLower(n.Label), f) || strings.Contains(strings.ToLower(clusterLabel), f) ||
		strings.Contains(strings.ToLower(n.Category), f)
}

// Draw paints clusters, edges, nodes, the selection, the legend and the
// focus ring.
func (g *GraphView) Draw(p painter.Painter, theme *Theme) {
	g.layout()
	r := g.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Background)
	sel := g.Model.Selected.Get()
	labels := map[string]string{}
	clusters := g.Model.Clusters.Slice()
	for _, c := range clusters {
		labels[c.ID] = c.Label
	}
	visible := map[string]bool{}
	withClip(p, r, func() {
		// In the model's order, not a map's: the same model draws the same
		// operations in the same sequence, so a recording is reproducible.
		for _, c := range clusters {
			id := c.ID
			box := g.screen(g.clusterBox[id]) // layout() boxed every cluster just above
			g.fillBox(p, box, g.clusterRadius(), theme.Surface)
			g.strokeBox(p, box, g.clusterRadius(), theme.Border)
			// The title is clipped to its box: a long one never runs out of it.
			withClip(p, box, func() {
				g.zoomedFont().Draw(p, box.X+gGap(), box.Y+scaled(2), labels[id], theme.OnSurface)
			})
		}
		nodes := g.Model.Nodes.Slice()
		for _, n := range nodes {
			visible[n.ID] = g.matches(n, labels[n.Cluster])
		}
		edges := g.Model.Edges.Slice()
		lanes := edgeLanes(edges)
		for _, e := range edges {
			g.drawEdge(p, theme, e, lanes[e.ID], sel == "edge:"+e.ID, visible[e.From] || visible[e.To])
		}
		for _, n := range nodes {
			cb, ok := g.nodeBox[n.ID]
			if !ok {
				continue
			}
			box := g.screen(cb)
			col := g.categoryColor(n.Category)
			if !visible[n.ID] {
				col = blendRGBA(col, theme.Background, 0.25)
			}
			// Nothing of a node is drawn outside its cluster's box.
			limit := g.screen(g.clusterBox[n.Cluster])
			withClip(p, limit, func() {
				g.fillBox(p, box, g.CornerRadius, col)
				// Label ink by the swatch's lightness, so it reads on every colour.
				ink := RGB(0x10, 0x10, 0x10)
				if luminance(col) < 0.4 {
					ink = RGB(0xFF, 0xFF, 0xFF)
				}
				// The label is clipped to its box: a long one must not run into the
				// node beside it.
				withClip(p, box, func() {
					f := g.zoomedFont()
					f.Draw(p, box.X+3, box.Y+(box.H-f.Height())/2, n.Label, ink)
				})
				if sel == "node:"+n.ID {
					g.strokeBox(p, Rect{X: box.X - 2, Y: box.Y - 2, W: box.W + 4, H: box.H + 4}, g.CornerRadius+2, theme.Accent)
				}
			})
		}
		if g.ShowLegend {
			// Clusters scrolled under the legend must not show through it.
			h := g.legendHeight()
			fillRect(p, r.X, r.Y+r.H-h, r.W, h, theme.Background)
			g.drawLegend(p, theme)
		}
		if sg, ok := g.scrollbarGeom(); ok {
			paintScrollTrack(p, theme, r.X+sg.cross0, r.Y+sg.trackStart, scrollbarTrack(), sg.trackLen)
			paintScrollThumb(p, theme, r.X+sg.cross0, r.Y+sg.thumbStart, scrollbarTrack(), sg.thumbLen)
		}
	})
	g.drawFocusRing(p, theme, r)
}

// graphLaneGap separates edges that join the same two nodes.
const graphLaneGap = 5

// edgeLanes numbers the edges that join the same pair of nodes (in either
// direction) 0, 1, 2, ... so that each is drawn in its own lane: two flows
// between the same containers, say one allowed and one refused, never hide
// each other.
func edgeLanes(edges []GraphEdge) map[string]int {
	seen := map[[2]string]int{}
	lanes := make(map[string]int, len(edges))
	for _, e := range edges {
		k := [2]string{e.From, e.To}
		if e.To < e.From {
			k = [2]string{e.To, e.From}
		}
		lanes[e.ID] = seen[k]
		seen[k]++
	}
	return lanes
}

// laneOffset is lane 0 -> 0, 1 -> +gap, 2 -> -gap, 3 -> +2gap, ...
func laneOffset(lane int) int {
	n := (lane + 1) / 2 * graphLaneGap
	if lane%2 == 0 {
		return -n
	}
	return n
}

// edgeEnds is an edge's segment in its lane, perpendicular to its main axis.
func (g *GraphView) edgeEnds(e GraphEdge, lane int) (x0, y0, x1, y1 int, ok bool) {
	a, ok1 := g.nodeBox[e.From]
	b, ok2 := g.nodeBox[e.To]
	if !ok1 || !ok2 {
		return 0, 0, 0, 0, false
	}
	x0, y0 = graphCenter(a)
	x1, y1 = graphCenter(b)
	off := laneOffset(lane)
	if absInt(x1-x0) >= absInt(y1-y0) {
		y0, y1 = y0+off, y1+off
	} else {
		x0, x1 = x0+off, x1+off
	}
	return x0, y0, x1, y1, true
}

// drawEdge strokes one edge between node centres, in its lane; an edge with
// an end that is not drawn is skipped.
func (g *GraphView) drawEdge(p painter.Painter, theme *Theme, e GraphEdge, lane int, selected, visible bool) {
	x0, y0, x1, y1, ok := g.edgeEnds(e, lane)
	if !ok {
		return
	}
	x0, y0, x1, y1 = g.sx(x0), g.sy(y0), g.sx(x1), g.sy(y1)
	col := theme.OnBackground
	switch e.State {
	case GraphEdgeDenied:
		col = graphDeniedInk(theme)
	case GraphEdgeIdle:
		col = blendRGBA(theme.OnBackground, theme.Background, 0.35)
	}
	if !visible {
		col = blendRGBA(col, theme.Background, 0.2)
	}
	if selected {
		col = theme.Accent
	}
	w := g.edgeWidth(e.Weight)
	dx, dy := x1-x0, y1-y0
	horizontal := absInt(dx) >= absInt(dy)
	for k := 0; k < w; k++ {
		off := k - w/2
		ox, oy := 0, off
		if !horizontal {
			ox, oy = off, 0
		}
		if e.State == GraphEdgeDenied {
			drawDashed(p, x0+ox, y0+oy, x1+ox, y1+oy, col)
		} else {
			drawLine(p, x0+ox, y0+oy, x1+ox, y1+oy, col)
		}
	}
	// The arrow head: a small square at the target end.
	fillRect(p, x1-2+(x0-x1)/8, y1-2+(y0-y1)/8, 4, 4, col)
	if selected && e.Label != "" && g.zoomPct() >= graphZoomBelowLabels {
		f := g.zoomedFont()
		f.Draw(p, (x0+x1)/2+4, (y0+y1)/2-f.Height()-2, e.Label, theme.OnBackground)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// drawDashed draws a 4-on, 3-off dashed line.
func drawDashed(p painter.Painter, x0, y0, x1, y1 int, c RGBA) {
	dx, dy := float64(x1-x0), float64(y1-y0)
	n := math.Hypot(dx, dy)
	for s := 0.0; s < n; s += 7 {
		e := math.Min(s+4, n)
		drawLine(p, x0+int(dx*s/n), y0+int(dy*s/n), x0+int(dx*e/n), y0+int(dy*e/n), c)
	}
}

// graphLegendPad is the space above and below the legend's line, and
// graphSwatch the side of a category swatch.
const (
	graphLegendPad = 3
	graphSwatch    = 10
)

// legendHeight is the legend strip's height, from the widget's font: one
// line of text (or a swatch, if the font is smaller) with its padding per row.
// The legend wraps to as many rows as its entries need in the widget's width, so
// it never runs off the right edge.
func (g *GraphView) legendHeight() int {
	rows := len(g.legendRows())
	return rows*max(g.glyphHeight(), graphSwatch) + (rows+1)*graphLegendPad
}

// legendItem is one entry of the legend: a swatch (or the dashed denied style)
// and its caption, and the width the pair takes.
type legendItem struct {
	cat    string
	denied bool
	w      int
}

// legendRows flows the legend's entries (one per category, then "denied") into
// rows no wider than the widget less its margins.
func (g *GraphView) legendRows() [][]legendItem {
	names := make([]string, len(g.categories))
	for cat, i := range g.categories {
		names[i] = cat
	}
	items := make([]legendItem, 0, len(names)+1)
	for _, cat := range names {
		items = append(items, legendItem{cat: cat, w: 13 + g.textWidth(cat) + 10})
	}
	items = append(items, legendItem{cat: "denied", denied: true, w: 20 + g.textWidth("denied")})
	avail := g.Bounds().W - 2*gGap()
	rows := [][]legendItem{nil}
	x := 0
	for _, it := range items {
		if x > 0 && x+it.w > avail {
			rows = append(rows, nil)
			x = 0
		}
		rows[len(rows)-1] = append(rows[len(rows)-1], it)
		x += it.w
	}
	return rows
}

// drawLegend draws one swatch per category seen, and the denied style, on lines
// centred in the legend strip.
func (g *GraphView) drawLegend(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	h := g.legendHeight()
	line := max(g.glyphHeight(), graphSwatch)
	for i, row := range g.legendRows() {
		top := r.Y + r.H - h + graphLegendPad + i*(line+graphLegendPad)
		ty := top + (line-g.glyphHeight())/2 // text
		sy := top + (line-graphSwatch)/2     // swatch
		x := r.X + gGap()
		for _, it := range row {
			if it.denied {
				drawDashed(p, x, sy+graphSwatch/2, x+16, sy+graphSwatch/2, graphDeniedInk(theme))
				g.drawText(p, x+20, ty, it.cat, theme.OnBackground)
			} else {
				fillRect(p, x, sy, graphSwatch, graphSwatch, g.categoryColor(it.cat))
				g.drawText(p, x+13, ty, it.cat, theme.OnBackground)
			}
			x += it.w
		}
	}
}

// edgeAt returns the edge whose segment passes within graphHitSlack of (x,y).
func (g *GraphView) edgeAt(x, y int) (string, bool) {
	// (x, y) is in content pixels; the slack is in surface pixels.
	best, bestD := "", float64(graphHitSlack)*100/float64(g.zoomPct())+0.5
	edges := g.Model.Edges.Slice()
	lanes := edgeLanes(edges)
	for _, e := range edges {
		x0, y0, x1, y1, ok := g.edgeEnds(e, lanes[e.ID])
		if !ok {
			continue
		}
		if d := segDist(float64(x), float64(y), float64(x0), float64(y0), float64(x1), float64(y1)); d < bestD {
			best, bestD = e.ID, d
		}
	}
	return best, best != ""
}

// segDist is the distance from a point to a segment.
func segDist(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	t := 0.0
	if l2 > 0 {
		t = math.Max(0, math.Min(1, ((px-x0)*dx+(py-y0)*dy)/l2))
	}
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

// Hit returns what is under a surface point: "node:<id>", "edge:<id>" or "".
func (g *GraphView) Hit(x, y int) string {
	g.layout()
	x, y = g.cx(x), g.cy(y) // the point, in content pixels
	for _, id := range g.order {
		if g.nodeBox[id].Contains(x, y) {
			return "node:" + id
		}
	}
	if id, ok := g.edgeAt(x, y); ok {
		return "edge:" + id
	}
	return ""
}

// OnEvent: a click selects what is under it (a second click on the same
// thing activates it); arrows move between nodes; Escape clears; Enter
// activates.
func (g *GraphView) OnEvent(ev Event) {
	if g.isDisabled() {
		return
	}
	r := g.Bounds()
	switch ev.Kind {
	case EventScroll:
		switch {
		case g.zoomWheel(ev):
		case ev.Shift:
			g.PanBy((ev.Delta + ev.DeltaX) * graphScrollStep())
		default:
			if ev.DeltaX != 0 {
				g.PanBy(ev.DeltaX * graphScrollStep())
			}
			g.ScrollBy(ev.Delta * graphScrollStep())
		}
	case EventMouseDrag:
		if g.bgDrag {
			z := g.zoomPct()
			g.PanTo(g.dragPan.X + rnd(float64(g.dragFrom.X-ev.X)*100/float64(z)))
			g.ScrollTo(g.dragPan.Y + rnd(float64(g.dragFrom.Y-ev.Y)*100/float64(z)))
			return
		}
		sg, ok := g.scrollbarGeom()
		g.sbDrag.drag(sg, ok, ev, g.ScrollTo)
	case EventMouseUp:
		g.bgDrag = false
		g.sbDrag.release()
	case EventTouchStart, EventTouchMove, EventTouchEnd:
		g.pinchFeed(ev)
	case EventClick:
		if sg, ok := g.scrollbarGeom(); g.sbDrag.press(sg, ok, ev, g.viewH*100/g.zoomPct(), g.ScrollBy) {
			return
		}
		hit := g.Hit(r.X+ev.X, r.Y+ev.Y)
		if hit != "" && hit == g.Model.Selected.Get() {
			g.activate()
			return
		}
		// A press on the background may become a pan.
		g.bgDrag = hit == ""
		g.dragFrom, g.dragPan = GraphPoint{ev.X, ev.Y}, GraphPoint{g.panX, g.scrollY}
		g.Model.Selected.Set(hit)
	case EventKeyDown:
		if !g.zoomKey(ev.Code) {
			g.key(ev.Code)
		}
	}
}

func (g *GraphView) activate() {
	if g.OnActivate != nil && g.Model.Selected.Get() != "" {
		g.OnActivate(g.Model.Selected.Get())
	}
}

// key moves the node selection in layout order.
func (g *GraphView) key(code string) {
	g.layout()
	switch code {
	case "Escape":
		g.Model.Selected.Set("")
		return
	case "Enter":
		g.activate()
		return
	}
	step := 0
	switch code {
	case "ArrowRight", "ArrowDown":
		step = 1
	case "ArrowLeft", "ArrowUp":
		step = -1
	default:
		return
	}
	if len(g.order) == 0 {
		return
	}
	cur := -1
	for i, id := range g.order {
		if "node:"+id == g.Model.Selected.Get() {
			cur = i
		}
	}
	next := cur + step
	if cur < 0 {
		next = 0
	}
	id := g.order[clampInt(next, 0, len(g.order)-1)]
	g.Model.Selected.Set("node:" + id)
	g.revealNode(id)
}

// A11y describes the graph: how many nodes and edges, how many denied, and
// the selection.
func (g *GraphView) A11y() A11yInfo {
	denied := 0
	edges := g.Model.Edges.Slice()
	for _, e := range edges {
		if e.State == GraphEdgeDenied {
			denied++
		}
	}
	v := strconv.Itoa(g.Model.Nodes.Len()) + " nodes, " + strconv.Itoa(len(edges)) + " edges, " + strconv.Itoa(denied) + " denied"
	if s := g.Model.Selected.Get(); s != "" {
		v += "; selected " + s
	}
	return A11yInfo{Role: RoleGroup, Name: "network graph", Value: v, Disabled: g.isDisabled()}
}

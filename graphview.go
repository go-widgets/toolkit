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
	graphLegendH  = 16
	graphHitSlack = 3
)

// layout places clusters on a grid filling the bounds and nodes on a sub-grid
// inside each cluster. Deterministic: the same lists and bounds give the
// same boxes.
func (g *GraphView) layout() {
	g.bind()
	if g.layoutValid {
		return
	}
	r := g.Bounds()
	g.layoutFor = r
	g.layoutValid = true
	g.clusterBox = map[string]Rect{}
	g.nodeBox = map[string]Rect{}
	g.order = g.order[:0]
	clusters := g.Model.Clusters.Slice()
	nodes := g.Model.Nodes.Slice()
	members := map[string][]GraphNode{}
	most := 1
	for _, n := range nodes {
		members[n.Cluster] = append(members[n.Cluster], n)
		if k := len(members[n.Cluster]); k > most {
			most = k
		}
	}
	if len(clusters) == 0 {
		return
	}
	area := r
	if g.ShowLegend {
		area.H -= graphLegendH
	}
	// Nodes per row inside a cluster, then cluster columns to fill the width.
	perRow := int(math.Ceil(math.Sqrt(float64(most))))
	rowsIn := (most + perRow - 1) / perRow
	cw := perRow*(graphNodeW+graphGap) + graphGap
	ch := graphTitleH + rowsIn*(graphNodeH+graphGap) + graphGap
	cols := maxInt(1, (area.W-graphGap)/(cw+graphGap))
	if cols > len(clusters) {
		cols = len(clusters)
	}
	for i, c := range clusters {
		col, row := i%cols, i/cols
		box := Rect{X: area.X + graphGap + col*(cw+graphGap), Y: area.Y + graphGap + row*(ch+graphGap), W: cw, H: ch}
		g.clusterBox[c.ID] = box
		for j, n := range members[c.ID] {
			nc, nr := j%perRow, j/perRow
			g.nodeBox[n.ID] = Rect{X: box.X + graphGap + nc*(graphNodeW+graphGap),
				Y: box.Y + graphTitleH + graphGap + nr*(graphNodeH+graphGap), W: graphNodeW, H: graphNodeH}
			g.order = append(g.order, n.ID)
		}
	}
}

// NodeBox is the box a node is drawn in (ok=false when it is not drawn), for
// hosts and tests.
func (g *GraphView) NodeBox(id string) (Rect, bool) {
	g.layout()
	b, ok := g.nodeBox[id]
	return b, ok
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
	for _, c := range g.Model.Clusters.Slice() {
		labels[c.ID] = c.Label
	}
	visible := map[string]bool{}
	withClip(p, r, func() {
		for id, box := range g.clusterBox {
			fillRect(p, box.X, box.Y, box.W, box.H, theme.Surface)
			strokeRect(p, box.X, box.Y, box.W, box.H, theme.Border)
			g.drawText(p, box.X+graphGap, box.Y+2, labels[id], theme.OnSurface)
		}
		nodes := g.Model.Nodes.Slice()
		for _, n := range nodes {
			visible[n.ID] = g.matches(n, labels[n.Cluster])
		}
		for _, e := range g.Model.Edges.Slice() {
			g.drawEdge(p, theme, e, sel == "edge:"+e.ID, visible[e.From] || visible[e.To])
		}
		for _, n := range nodes {
			box, ok := g.nodeBox[n.ID]
			if !ok {
				continue
			}
			col := g.categoryColor(n.Category)
			if !visible[n.ID] {
				col = blendRGBA(col, theme.Background, 0.25)
			}
			fillRect(p, box.X, box.Y, box.W, box.H, col)
			// Label ink by the swatch's lightness, so it reads on every colour.
			ink := RGB(0x10, 0x10, 0x10)
			if luminance(col) < 0.4 {
				ink = RGB(0xFF, 0xFF, 0xFF)
			}
			g.drawText(p, box.X+3, box.Y+(box.H-g.glyphHeight())/2, n.Label, ink)
			if sel == "node:"+n.ID {
				strokeRect(p, box.X-2, box.Y-2, box.W+4, box.H+4, theme.Accent)
			}
		}
		if g.ShowLegend {
			g.drawLegend(p, theme)
		}
	})
	g.drawFocusRing(p, theme, r)
}

// drawEdge strokes one edge between node centres; an edge with an end that is
// not drawn is skipped.
func (g *GraphView) drawEdge(p painter.Painter, theme *Theme, e GraphEdge, selected, visible bool) {
	a, ok1 := g.nodeBox[e.From]
	b, ok2 := g.nodeBox[e.To]
	if !ok1 || !ok2 {
		return
	}
	x0, y0 := graphCenter(a)
	x1, y1 := graphCenter(b)
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
	if selected && e.Label != "" {
		g.drawText(p, (x0+x1)/2+4, (y0+y1)/2-g.glyphHeight()-2, e.Label, theme.OnBackground)
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

// drawLegend draws one swatch per category seen, and the denied style.
func (g *GraphView) drawLegend(p painter.Painter, theme *Theme) {
	r := g.Bounds()
	y := r.Y + r.H - graphLegendH + 3
	x := r.X + graphGap
	names := make([]string, len(g.categories))
	for cat, i := range g.categories {
		names[i] = cat
	}
	for _, cat := range names {
		fillRect(p, x, y, 10, 10, g.categoryColor(cat))
		g.drawText(p, x+13, y+1, cat, theme.OnBackground)
		x += 13 + g.textWidth(cat) + 10
	}
	drawDashed(p, x, y+5, x+16, y+5, graphDeniedInk(theme))
	g.drawText(p, x+20, y+1, "denied", theme.OnBackground)
}

// edgeAt returns the edge whose segment passes within graphHitSlack of (x,y).
func (g *GraphView) edgeAt(x, y int) (string, bool) {
	best, bestD := "", float64(graphHitSlack)+0.5
	for _, e := range g.Model.Edges.Slice() {
		a, ok1 := g.nodeBox[e.From]
		b, ok2 := g.nodeBox[e.To]
		if !ok1 || !ok2 {
			continue
		}
		x0, y0 := graphCenter(a)
		x1, y1 := graphCenter(b)
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
	case EventClick:
		hit := g.Hit(r.X+ev.X, r.Y+ev.Y)
		if hit != "" && hit == g.Model.Selected.Get() {
			g.activate()
			return
		}
		g.Model.Selected.Set(hit)
	case EventKeyDown:
		g.key(ev.Code)
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
	g.Model.Selected.Set("node:" + g.order[clampInt(next, 0, len(g.order)-1)])
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
	return A11yInfo{Role: RoleGroup, Name: "network graph", Value: v}
}

package toolkit

import "sort"

// GraphNodeShape says what a GraphNode stands for, and so how a layout places
// and the view draws it.
type GraphNodeShape int

const (
	// GraphShapeBox is an ordinary node: a box inside its cluster (default).
	GraphShapeBox GraphNodeShape = iota
	// GraphShapeHub is a node that joins others, such as a network: drawn as a
	// pill, placed by the Bands layout in its middle band, in no cluster.
	GraphShapeHub
	// GraphShapeGateway is the outside world (the host tab, the Internet):
	// drawn with an accent outline, placed by Bands in its top band.
	GraphShapeGateway
)

// GraphMetrics are the sizes a layout works in, in painter units already
// scaled by the metric scale, so a custom layout draws what the view measures.
type GraphMetrics struct {
	NodeW, NodeH, Gap, TitleH int
	// TitleW is the width a cluster's title needs; a cluster is at least that wide.
	TitleW func(GraphCluster) int
}

// GraphPlacement is a layout's answer: boxes relative to the content origin
// (the view adds its own offset, scroll and zoom).
type GraphPlacement struct {
	// Clusters and Nodes hold a box per placed cluster and node; a node absent
	// from Nodes is not drawn.
	Clusters, Nodes map[string]Rect
	// Order lists the placed node ids in reading order, the arrow keys' order.
	Order []string
	// Width and Height are the content size.
	Width, Height int
	// HasBar says the content needed a vertical scrollbar, whose column the
	// layout already left free.
	HasBar bool
}

// GraphLayout places a model's clusters and nodes in a viewport. It must be
// deterministic: the same model and viewport give the same placement, so the
// picture does not move when only edges change.
type GraphLayout interface {
	Place(m *GraphViewModel, viewport Size, mt GraphMetrics) GraphPlacement
}

type flowGridLayout struct{}
type bandsLayout struct{}

// GraphLayoutFlowGrid is the default: clusters flow left to right and wrap,
// each sized by its own members, in the model's order.
var GraphLayoutFlowGrid GraphLayout = flowGridLayout{}

// GraphLayoutBands is for graphs where one node joins several clusters, as a
// container on several networks: gateways sit in a top band, hubs in a middle
// band, and the boxes below, clustered, ordered to reduce crossings. Everything
// is sorted by id (ties included), so the placement does not depend on the
// order the model lists things in.
var GraphLayoutBands GraphLayout = bandsLayout{}

// SetLayout changes the layout and lays the graph out again; assigning the
// Layout field before the first draw is the same.
func (g *GraphView) SetLayout(l GraphLayout) {
	g.Layout = l
	g.layoutValid = false
}

func (g *GraphView) placer() GraphLayout {
	if g.Layout == nil {
		return GraphLayoutFlowGrid
	}
	return g.Layout
}

func (g *GraphView) metrics() GraphMetrics {
	return GraphMetrics{NodeW: g.nodeW(), NodeH: gNodeH(), Gap: gGap(), TitleH: gTitleH(),
		TitleW: func(c GraphCluster) int { return g.textWidth(c.Label) + 2*gGap() }}
}

// gridMembers groups the boxes of the model's nodes by cluster, in model order.
func gridMembers(nodes []GraphNode) map[string][]GraphNode {
	members := map[string][]GraphNode{}
	for _, n := range nodes {
		members[n.Cluster] = append(members[n.Cluster], n)
	}
	return members
}

// fillCluster puts the nodes of cluster c in its box b (rows of perRow).
func fillCluster(pl *GraphPlacement, c GraphCluster, b Rect, perRow int, nodes []GraphNode, mt GraphMetrics) {
	pl.Clusters[c.ID] = b
	for j, n := range nodes {
		nc, nr := j%perRow, j/perRow
		pl.Nodes[n.ID] = Rect{X: b.X + mt.Gap + nc*(mt.NodeW+mt.Gap),
			Y: b.Y + mt.TitleH + mt.Gap + nr*(mt.NodeH+mt.Gap), W: min(mt.NodeW, max(b.W-2*mt.Gap, 1)), H: mt.NodeH}
		pl.Order = append(pl.Order, n.ID)
	}
}

func newPlacement() GraphPlacement {
	return GraphPlacement{Clusters: map[string]Rect{}, Nodes: map[string]Rect{}}
}

// Place implements GraphLayout. When the rows need more height than the
// viewport has, a scrollbar takes its column and the rows are flowed again in
// what is left of the width.
func (flowGridLayout) Place(m *GraphViewModel, viewport Size, mt GraphMetrics) GraphPlacement {
	clusters := m.Clusters.Slice()
	pl := newPlacement()
	members := gridMembers(m.Nodes.Slice())
	boxes, perRows, h := flowClusters(clusters, members, viewport.W, mt.NodeW, mt.TitleW)
	if h > viewport.H {
		pl.HasBar = true
		boxes, perRows, h = flowClusters(clusters, members, viewport.W-scrollbarTrack(), mt.NodeW, mt.TitleW)
	}
	pl.Height = h
	for i, c := range clusters {
		pl.Width = maxInt(pl.Width, boxes[i].X+boxes[i].W+mt.Gap)
		fillCluster(&pl, c, boxes[i], perRows[i], members[c.ID], mt)
	}
	return pl
}

// Place implements GraphLayout.
func (bandsLayout) Place(m *GraphViewModel, viewport Size, mt GraphMetrics) GraphPlacement {
	pl := placeBands(m, viewport.W, mt)
	if pl.Height > viewport.H {
		pl = placeBands(m, viewport.W-scrollbarTrack(), mt)
		pl.HasBar = true
	}
	return pl
}

// bandsInput is the model sorted by id, with its adjacency.
type bandsInput struct {
	gateways, hubs []GraphNode
	clusters       []GraphCluster
	members        map[string][]GraphNode
	hubsOf         map[string][]string // container id -> hub ids, from the edges
}

func readBands(m *GraphViewModel) bandsInput {
	in := bandsInput{members: map[string][]GraphNode{}, hubsOf: map[string][]string{}}
	known := map[string]bool{}
	in.clusters = append(in.clusters, m.Clusters.Slice()...)
	sort.Slice(in.clusters, func(i, j int) bool { return in.clusters[i].ID < in.clusters[j].ID })
	for _, c := range in.clusters {
		known[c.ID] = true
	}
	hub := map[string]bool{}
	for _, n := range m.Nodes.Slice() {
		switch n.Shape {
		case GraphShapeGateway:
			in.gateways = append(in.gateways, n)
		case GraphShapeHub:
			in.hubs = append(in.hubs, n)
			hub[n.ID] = true
		default:
			if known[n.Cluster] {
				in.members[n.Cluster] = append(in.members[n.Cluster], n)
			}
		}
	}
	byID := func(s []GraphNode) {
		sort.Slice(s, func(i, j int) bool { return s[i].ID < s[j].ID })
	}
	byID(in.gateways)
	byID(in.hubs)
	for _, ms := range in.members {
		byID(ms)
	}
	for _, e := range m.Edges.Slice() {
		if hub[e.To] {
			in.hubsOf[e.From] = append(in.hubsOf[e.From], e.To)
		}
		if hub[e.From] {
			in.hubsOf[e.To] = append(in.hubsOf[e.To], e.From)
		}
	}
	return in
}

// noBary sorts a node with nothing to be near after those that have.
const noBary = 1 << 30

// flowRow places nodes left to right in rows of the width, from y; it returns
// the y below the last row.
func flowRow(pl *GraphPlacement, ns []GraphNode, width, y int, mt GraphMetrics) int {
	x, rowH := mt.Gap, 0
	for _, n := range ns {
		if x > mt.Gap && x+mt.NodeW+mt.Gap > width {
			x, y, rowH = mt.Gap, y+rowH+mt.Gap, 0
		}
		pl.Nodes[n.ID] = Rect{X: x, Y: y, W: mt.NodeW, H: mt.NodeH}
		pl.Order = append(pl.Order, n.ID)
		x += mt.NodeW + mt.Gap
		rowH = mt.NodeH
		pl.Width = maxInt(pl.Width, x)
	}
	return y + rowH
}

// mean returns the mean of vs, or noBary for none. The sum of integers is
// exact, so the order of vs never changes the answer.
func mean(vs []int) int {
	if len(vs) == 0 {
		return noBary
	}
	s := 0
	for _, v := range vs {
		s += v
	}
	return s / len(vs)
}

func placeBands(m *GraphViewModel, width int, mt GraphMetrics) GraphPlacement {
	in := readBands(m)
	var pl GraphPlacement
	for pass := 0; pass < 2; pass++ {
		pl = newPlacement()
		y := mt.Gap
		if len(in.gateways) > 0 {
			y = flowRow(&pl, in.gateways, width, y, mt) + 2*mt.Gap
		}
		if len(in.hubs) > 0 {
			y = flowRow(&pl, in.hubs, width, y, mt) + 2*mt.Gap
		}
		hubX := map[string]int{}
		for _, h := range in.hubs {
			hubX[h.ID] = pl.Nodes[h.ID].X + mt.NodeW/2
		}
		// Containers ordered by the mean x of their hubs, then the clusters by
		// the mean of their containers'.
		bary := map[string]int{}
		for _, ms := range in.members {
			for _, n := range ms {
				var xs []int
				for _, h := range in.hubsOf[n.ID] {
					xs = append(xs, hubX[h])
				}
				bary[n.ID] = mean(xs)
			}
		}
		cb := map[string]int{}
		for _, c := range in.clusters {
			ms := in.members[c.ID]
			sort.SliceStable(ms, func(i, j int) bool {
				if bary[ms[i].ID] != bary[ms[j].ID] {
					return bary[ms[i].ID] < bary[ms[j].ID]
				}
				return ms[i].ID < ms[j].ID
			})
			var vs []int
			for _, n := range ms {
				if bary[n.ID] != noBary {
					vs = append(vs, bary[n.ID])
				}
			}
			cb[c.ID] = mean(vs)
		}
		sort.SliceStable(in.clusters, func(i, j int) bool {
			a, b := in.clusters[i], in.clusters[j]
			if cb[a.ID] != cb[b.ID] {
				return cb[a.ID] < cb[b.ID]
			}
			return a.ID < b.ID
		})
		boxes, perRows, h := flowClusters(in.clusters, in.members, width, mt.NodeW, mt.TitleW)
		for i, c := range in.clusters {
			b := boxes[i]
			b.Y += y - mt.Gap // flowClusters starts one gap down
			pl.Width = maxInt(pl.Width, b.X+b.W+mt.Gap)
			fillCluster(&pl, c, b, perRows[i], in.members[c.ID], mt)
		}
		pl.Height = y - mt.Gap + h
		if len(in.clusters) == 0 {
			pl.Height = y
		}
		if pass == 0 {
			// Second pass: hubs by the mean x of the containers on them.
			cx := map[string][]int{}
			for id, hs := range in.hubsOf {
				r, ok := pl.Nodes[id]
				if !ok {
					continue
				}
				for _, h := range hs {
					cx[h] = append(cx[h], r.X+r.W/2)
				}
			}
			hb := map[string]int{}
			for _, h := range in.hubs {
				hb[h.ID] = mean(cx[h.ID])
			}
			sort.SliceStable(in.hubs, func(i, j int) bool {
				a, b := in.hubs[i], in.hubs[j]
				if hb[a.ID] != hb[b.ID] {
					return hb[a.ID] < hb[b.ID]
				}
				return a.ID < b.ID
			})
		}
	}
	return pl
}

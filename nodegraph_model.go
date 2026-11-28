// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"math"
	"strings"

	"github.com/go-widgets/mvvm"
)

// Bounds on a NodeGraphModel. The graph is typically built from untrusted JSON,
// so the model refuses what exceeds them instead of trusting its caller.
const (
	// NodeGraphMaxNodes and NodeGraphMaxEdges cap the graph's size.
	NodeGraphMaxNodes = 2000
	NodeGraphMaxEdges = 8000
	// NodeGraphMaxPorts caps the inputs, and separately the outputs, of a node.
	NodeGraphMaxPorts = 32
	// NodeGraphMaxCoord is the largest absolute node coordinate; NaN becomes 0
	// and anything beyond (infinities included) is clamped to it.
	NodeGraphMaxCoord = 1e6
	// NodeGraphMaxText caps an id (a longer one is refused) and a label or type
	// (a longer one is cut), in bytes.
	NodeGraphMaxText = 256
)

// The reasons the model refuses an edit. Connect and AddNode wrap one of them.
var (
	ErrNodeGraphLimit     = errors.New("nodegraph: size limit reached")
	ErrNodeGraphInvalid   = errors.New("nodegraph: invalid id or port")
	ErrNodeGraphDuplicate = errors.New("nodegraph: duplicate id or edge")
	ErrNodeGraphUnknown   = errors.New("nodegraph: unknown node or port")
	ErrNodeGraphIncompat  = errors.New("nodegraph: incompatible ports")
)

// NodeGraphPort is a typed connection point. Type is what Compatible compares.
type NodeGraphPort struct {
	ID, Label, Type string
}

// NodeGraphNode is a box at (X, Y) in graph coordinates, with inputs on its
// left edge and outputs on its right edge. Data the host needs (a tool's
// settings, say) stays in the host, keyed by ID: the widget carries none.
type NodeGraphNode struct {
	ID, Label       string
	X, Y            float64
	Inputs, Outputs []NodeGraphPort
}

// NodeGraphEdge joins an output port to an input port.
type NodeGraphEdge struct {
	ID, FromNode, FromPort, ToNode, ToPort string
}

// NodeGraphChangeKind says what a NodeGraphChange reports.
type NodeGraphChangeKind int

const (
	NodeAdded NodeGraphChangeKind = iota
	NodeRemoved
	NodeMoved
	EdgeAdded
	EdgeRemoved
)

// NodeGraphChange is one event of the change stream: ID is the node's or the
// edge's. Removing a node reports each edge it took with it (EdgeRemoved)
// before the NodeRemoved.
type NodeGraphChange struct {
	Kind NodeGraphChangeKind
	ID   string
}

// NodeGraphViewport is the pan and zoom: a graph point (x, y) is drawn at
// (x*Zoom+PanX, y*Zoom+PanY) from the widget's origin.
type NodeGraphViewport struct {
	Zoom, PanX, PanY float64
}

// NodeGraph zoom limits.
const (
	NodeGraphZoomMin = 0.25
	NodeGraphZoomMax = 4.0
)

// NodeGraphModel is the data of a NodeGraph. Every edit goes through its
// methods, which keep the graph consistent: an edge always joins an existing
// output port to an existing input port, so removing a node removes its edges
// and no sequence of calls leaves one dangling.
type NodeGraphModel struct {
	// Compatible says whether an output port may feed an input port; the model
	// consults it on every Connect. nil: the two ports' Type must be equal.
	Compatible func(from, to NodeGraphPort) bool
	// Selected is "node:<id>", "edge:<id>" or "".
	Selected *mvvm.Observable[string]
	// View is the pan and zoom; a host may persist or set it.
	View *mvvm.Observable[NodeGraphViewport]

	nodes []NodeGraphNode
	edges []NodeGraphEdge
	subs  []*func(NodeGraphChange)
}

// NewNodeGraphModel returns an empty model at zoom 1 with no pan.
func NewNodeGraphModel() *NodeGraphModel {
	return &NodeGraphModel{
		Selected: mvvm.NewObservable(""),
		View:     mvvm.NewObservable(NodeGraphViewport{Zoom: 1}),
	}
}

// Subscribe registers fn for the change stream and returns its unsubscribe. A
// caller serialises the graph by re-reading Nodes and Edges on each event.
func (m *NodeGraphModel) Subscribe(fn func(NodeGraphChange)) (unsubscribe func()) {
	p := &fn
	m.subs = append(m.subs, p)
	return func() {
		for i, s := range m.subs {
			if s == p {
				m.subs = append(m.subs[:i:i], m.subs[i+1:]...)
				return
			}
		}
	}
}

func (m *NodeGraphModel) emit(k NodeGraphChangeKind, id string) {
	subs := make([]*func(NodeGraphChange), len(m.subs))
	copy(subs, m.subs) // a subscriber may unsubscribe while it is called
	for _, s := range subs {
		(*s)(NodeGraphChange{Kind: k, ID: id})
	}
}

// Nodes and Edges return the graph in insertion order. The slices are the
// model's own: read them, never write them or keep them across an edit.
func (m *NodeGraphModel) Nodes() []NodeGraphNode { return m.nodes }
func (m *NodeGraphModel) Edges() []NodeGraphEdge { return m.edges }

func (m *NodeGraphModel) nodeIndex(id string) int {
	for i := range m.nodes {
		if m.nodes[i].ID == id {
			return i
		}
	}
	return -1
}

// Node returns the node with the id.
func (m *NodeGraphModel) Node(id string) (NodeGraphNode, bool) {
	if i := m.nodeIndex(id); i >= 0 {
		return m.nodes[i], true
	}
	return NodeGraphNode{}, false
}

// Edge returns the edge with the id.
func (m *NodeGraphModel) Edge(id string) (NodeGraphEdge, bool) {
	for _, e := range m.edges {
		if e.ID == id {
			return e, true
		}
	}
	return NodeGraphEdge{}, false
}

// clampCoord makes a coordinate finite and bounded.
func clampCoord(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	return math.Max(-NodeGraphMaxCoord, math.Min(NodeGraphMaxCoord, v))
}

// cutText bounds a label to NodeGraphMaxText bytes, on a rune boundary.
func cutText(s string) string {
	if len(s) <= NodeGraphMaxText {
		return s
	}
	return strings.ToValidUTF8(s[:NodeGraphMaxText], "")
}

// cleanPorts copies ports, bounded, refusing an empty, long or repeated id.
func cleanPorts(in []NodeGraphPort) ([]NodeGraphPort, error) {
	if len(in) > NodeGraphMaxPorts {
		return nil, ErrNodeGraphLimit
	}
	out := make([]NodeGraphPort, len(in))
	for i, p := range in {
		if p.ID == "" || len(p.ID) > NodeGraphMaxText {
			return nil, ErrNodeGraphInvalid
		}
		for _, q := range out[:i] {
			if q.ID == p.ID {
				return nil, ErrNodeGraphDuplicate
			}
		}
		out[i] = NodeGraphPort{ID: p.ID, Label: cutText(p.Label), Type: cutText(p.Type)}
	}
	return out, nil
}

// AddNode adds a copy of n, its coordinates made finite and bounded. It is
// refused when the id is empty, too long or taken, when a port id is empty or
// repeated within its side, or when a limit is reached.
func (m *NodeGraphModel) AddNode(n NodeGraphNode) error {
	if n.ID == "" || len(n.ID) > NodeGraphMaxText {
		return ErrNodeGraphInvalid
	}
	if len(m.nodes) >= NodeGraphMaxNodes {
		return ErrNodeGraphLimit
	}
	if m.nodeIndex(n.ID) >= 0 {
		return ErrNodeGraphDuplicate
	}
	in, err := cleanPorts(n.Inputs)
	if err != nil {
		return err
	}
	out, err := cleanPorts(n.Outputs)
	if err != nil {
		return err
	}
	m.nodes = append(m.nodes, NodeGraphNode{ID: n.ID, Label: cutText(n.Label), X: clampCoord(n.X), Y: clampCoord(n.Y), Inputs: in, Outputs: out})
	m.emit(NodeAdded, n.ID)
	return nil
}

// RemoveNode removes the node and every edge touching it; false when unknown.
func (m *NodeGraphModel) RemoveNode(id string) bool {
	i := m.nodeIndex(id)
	if i < 0 {
		return false
	}
	var gone []string
	kept := m.edges[:0:0]
	for _, e := range m.edges {
		if e.FromNode == id || e.ToNode == id {
			gone = append(gone, e.ID)
		} else {
			kept = append(kept, e)
		}
	}
	m.edges = kept
	m.nodes = append(m.nodes[:i:i], m.nodes[i+1:]...)
	m.dropSelection("node:"+id, gone)
	for _, e := range gone {
		m.emit(EdgeRemoved, e)
	}
	m.emit(NodeRemoved, id)
	return true
}

// dropSelection clears the selection when it names the removed node or one of
// the removed edges.
func (m *NodeGraphModel) dropSelection(node string, edges []string) {
	s := m.Selected.Get()
	for _, e := range edges {
		if s == "edge:"+e {
			s = node
		}
	}
	if s == node {
		m.Selected.Set("")
	}
}

// MoveNode places the node at (x, y), made finite and bounded.
func (m *NodeGraphModel) MoveNode(id string, x, y float64) bool {
	if !m.setPos(id, x, y) {
		return false
	}
	m.emit(NodeMoved, id)
	return true
}

// setPos is MoveNode without the event: a drag moves a node many times and
// reports it once, at the release.
func (m *NodeGraphModel) setPos(id string, x, y float64) bool {
	i := m.nodeIndex(id)
	if i < 0 {
		return false
	}
	m.nodes[i].X, m.nodes[i].Y = clampCoord(x), clampCoord(y)
	return true
}

func findPort(ports []NodeGraphPort, id string) (NodeGraphPort, bool) {
	for _, p := range ports {
		if p.ID == id {
			return p, true
		}
	}
	return NodeGraphPort{}, false
}

// compatible applies the callback, or the default: equal types.
func (m *NodeGraphModel) compatible(from, to NodeGraphPort) bool {
	if m.Compatible != nil {
		return m.Compatible(from, to)
	}
	return from.Type == to.Type
}

// CanConnect reports why an edge from (fromNode, fromPort) to (toNode, toPort)
// would be refused, or nil. An edge joins two different nodes, an output to an
// input, once, and the Compatible callback must agree.
func (m *NodeGraphModel) CanConnect(fromNode, fromPort, toNode, toPort string) error {
	fi, ti := m.nodeIndex(fromNode), m.nodeIndex(toNode)
	if fi < 0 || ti < 0 {
		return ErrNodeGraphUnknown
	}
	from, okF := findPort(m.nodes[fi].Outputs, fromPort)
	to, okT := findPort(m.nodes[ti].Inputs, toPort)
	if !okF || !okT {
		return ErrNodeGraphUnknown
	}
	if fi == ti {
		return ErrNodeGraphInvalid
	}
	for _, e := range m.edges {
		if e.FromNode == fromNode && e.FromPort == fromPort && e.ToNode == toNode && e.ToPort == toPort {
			return ErrNodeGraphDuplicate
		}
	}
	if !m.compatible(from, to) {
		return ErrNodeGraphIncompat
	}
	return nil
}

// Connect adds the edge after CanConnect and the limit and id checks. An empty
// e.ID is derived from the four endpoints; a taken one is refused.
func (m *NodeGraphModel) Connect(e NodeGraphEdge) error {
	if err := m.CanConnect(e.FromNode, e.FromPort, e.ToNode, e.ToPort); err != nil {
		return err
	}
	if len(m.edges) >= NodeGraphMaxEdges {
		return ErrNodeGraphLimit
	}
	if len(e.ID) > NodeGraphMaxText {
		return ErrNodeGraphInvalid
	}
	if e.ID == "" {
		e.ID = e.FromNode + "." + e.FromPort + ">" + e.ToNode + "." + e.ToPort
	}
	if _, taken := m.Edge(e.ID); taken {
		return ErrNodeGraphDuplicate
	}
	m.edges = append(m.edges, e)
	m.emit(EdgeAdded, e.ID)
	return nil
}

// RemoveEdge removes the edge; false when unknown.
func (m *NodeGraphModel) RemoveEdge(id string) bool {
	for i, e := range m.edges {
		if e.ID == id {
			m.edges = append(m.edges[:i:i], m.edges[i+1:]...)
			m.dropSelection("", []string{id})
			m.emit(EdgeRemoved, id)
			return true
		}
	}
	return false
}

// DeleteSelected removes the selected node or edge; false when nothing is.
func (m *NodeGraphModel) DeleteSelected() bool {
	s := m.Selected.Get()
	switch {
	case strings.HasPrefix(s, "node:"):
		return m.RemoveNode(s[5:])
	case strings.HasPrefix(s, "edge:"):
		return m.RemoveEdge(s[5:])
	}
	return false
}

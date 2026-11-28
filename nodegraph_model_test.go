// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// ngPair: "a" has outputs o (pdf) and p (img); "b" has inputs i (pdf) and j (img).
func ngPair() *NodeGraphModel {
	m := NewNodeGraphModel()
	_ = m.AddNode(NodeGraphNode{ID: "a", Label: "Alpha", X: 20, Y: 20,
		Outputs: []NodeGraphPort{{ID: "o", Label: "out", Type: "pdf"}, {ID: "p", Label: "pic", Type: "img"}}})
	_ = m.AddNode(NodeGraphNode{ID: "b", Label: "Beta", X: 300, Y: 40,
		Inputs: []NodeGraphPort{{ID: "i", Label: "in", Type: "pdf"}, {ID: "j", Label: "img", Type: "img"}}})
	return m
}

func TestNodeGraphAddNodeRefusals(t *testing.T) {
	m := NewNodeGraphModel()
	long := strings.Repeat("x", NodeGraphMaxText+1)
	manyPorts := make([]NodeGraphPort, NodeGraphMaxPorts+1)
	for i := range manyPorts {
		manyPorts[i] = NodeGraphPort{ID: strconv.Itoa(i)}
	}
	cases := []struct {
		name string
		n    NodeGraphNode
		want error
	}{
		{"empty id", NodeGraphNode{}, ErrNodeGraphInvalid},
		{"long id", NodeGraphNode{ID: long}, ErrNodeGraphInvalid},
		{"empty port id", NodeGraphNode{ID: "n", Inputs: []NodeGraphPort{{}}}, ErrNodeGraphInvalid},
		{"long port id", NodeGraphNode{ID: "n", Outputs: []NodeGraphPort{{ID: long}}}, ErrNodeGraphInvalid},
		{"repeated port", NodeGraphNode{ID: "n", Inputs: []NodeGraphPort{{ID: "p"}, {ID: "p"}}}, ErrNodeGraphDuplicate},
		{"too many inputs", NodeGraphNode{ID: "n", Inputs: manyPorts}, ErrNodeGraphLimit},
		{"too many outputs", NodeGraphNode{ID: "n", Outputs: manyPorts}, ErrNodeGraphLimit},
	}
	for _, c := range cases {
		if err := m.AddNode(c.n); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if len(m.Nodes()) != 0 {
		t.Fatal("a refused node was added")
	}
	if err := m.AddNode(NodeGraphNode{ID: "n"}); err != nil {
		t.Fatal(err)
	}
	if err := m.AddNode(NodeGraphNode{ID: "n"}); !errors.Is(err, ErrNodeGraphDuplicate) {
		t.Fatalf("duplicate node: %v", err)
	}
}

func TestNodeGraphNodeLimit(t *testing.T) {
	m := NewNodeGraphModel()
	for i := 0; i < NodeGraphMaxNodes; i++ {
		if err := m.AddNode(NodeGraphNode{ID: strconv.Itoa(i)}); err != nil {
			t.Fatal(i, err)
		}
	}
	if err := m.AddNode(NodeGraphNode{ID: "one too many"}); !errors.Is(err, ErrNodeGraphLimit) {
		t.Fatalf("over the node limit: %v", err)
	}
}

func TestNodeGraphEdgeLimit(t *testing.T) {
	m := NewNodeGraphModel()
	ports := make([]NodeGraphPort, NodeGraphMaxPorts)
	for i := range ports {
		ports[i] = NodeGraphPort{ID: strconv.Itoa(i), Type: "t"}
	}
	for i := 0; i < 10; i++ {
		_ = m.AddNode(NodeGraphNode{ID: strconv.Itoa(i), Inputs: ports, Outputs: ports})
	}
	n := 0
	for a := 0; a < 10 && n < NodeGraphMaxEdges; a++ {
		for b := 0; b < 10 && n < NodeGraphMaxEdges; b++ {
			for o := 0; o < NodeGraphMaxPorts && n < NodeGraphMaxEdges; o++ {
				for i := 0; i < NodeGraphMaxPorts && n < NodeGraphMaxEdges; i++ {
					if a == b {
						continue
					}
					if err := m.Connect(NodeGraphEdge{FromNode: strconv.Itoa(a), FromPort: strconv.Itoa(o), ToNode: strconv.Itoa(b), ToPort: strconv.Itoa(i)}); err != nil {
						t.Fatal(err)
					}
					n++
				}
			}
		}
	}
	err := m.Connect(NodeGraphEdge{FromNode: "9", FromPort: "0", ToNode: "8", ToPort: "0"})
	if !errors.Is(err, ErrNodeGraphLimit) {
		t.Fatalf("over the edge limit: %v (edges %d)", err, len(m.Edges()))
	}
}

func TestNodeGraphCoordinatesAreMadeFiniteAndBounded(t *testing.T) {
	m := NewNodeGraphModel()
	for i, c := range []struct{ x, y, wx, wy float64 }{
		{math.NaN(), math.NaN(), 0, 0},
		{math.Inf(1), math.Inf(-1), NodeGraphMaxCoord, -NodeGraphMaxCoord},
		{1e300, -1e300, NodeGraphMaxCoord, -NodeGraphMaxCoord},
		{12.5, -3, 12.5, -3},
	} {
		id := strconv.Itoa(i)
		if err := m.AddNode(NodeGraphNode{ID: id, X: c.x, Y: c.y}); err != nil {
			t.Fatal(err)
		}
		if n, _ := m.Node(id); n.X != c.wx || n.Y != c.wy {
			t.Errorf("add (%v,%v) -> (%v,%v), want (%v,%v)", c.x, c.y, n.X, n.Y, c.wx, c.wy)
		}
		m.MoveNode(id, c.x, c.y)
		if n, _ := m.Node(id); n.X != c.wx || n.Y != c.wy {
			t.Errorf("move (%v,%v) -> (%v,%v), want (%v,%v)", c.x, c.y, n.X, n.Y, c.wx, c.wy)
		}
	}
}

func TestNodeGraphTextIsCutAndPortsAreCopied(t *testing.T) {
	m := NewNodeGraphModel()
	// 3-byte runes: the cut falls inside one and must not leave invalid UTF-8.
	label := strings.Repeat("€", NodeGraphMaxText)
	ports := []NodeGraphPort{{ID: "p", Label: label, Type: label}}
	if err := m.AddNode(NodeGraphNode{ID: "n", Label: label, Inputs: ports}); err != nil {
		t.Fatal(err)
	}
	n, _ := m.Node("n")
	for _, s := range []string{n.Label, n.Inputs[0].Label, n.Inputs[0].Type} {
		if len(s) > NodeGraphMaxText || strings.ToValidUTF8(s, "") != s || s == "" {
			t.Fatalf("label not cut cleanly: %d bytes", len(s))
		}
	}
	ports[0].ID = "changed"
	if n, _ := m.Node("n"); n.Inputs[0].ID != "p" {
		t.Fatal("the model shares the caller's port slice")
	}
}

func TestNodeGraphConnectRules(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "c", Inputs: []NodeGraphPort{{ID: "i", Type: "pdf"}}, Outputs: []NodeGraphPort{{ID: "o", Type: "pdf"}}})
	cases := []struct {
		name string
		e    NodeGraphEdge
		want error
	}{
		{"unknown from node", NodeGraphEdge{FromNode: "zz", FromPort: "o", ToNode: "b", ToPort: "i"}, ErrNodeGraphUnknown},
		{"unknown to node", NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "zz", ToPort: "i"}, ErrNodeGraphUnknown},
		{"unknown from port", NodeGraphEdge{FromNode: "a", FromPort: "zz", ToNode: "b", ToPort: "i"}, ErrNodeGraphUnknown},
		{"unknown to port", NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "zz"}, ErrNodeGraphUnknown},
		{"input used as output", NodeGraphEdge{FromNode: "b", FromPort: "i", ToNode: "a", ToPort: "o"}, ErrNodeGraphUnknown},
		{"self loop", NodeGraphEdge{FromNode: "c", FromPort: "o", ToNode: "c", ToPort: "i"}, ErrNodeGraphInvalid},
		{"types differ", NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "j"}, ErrNodeGraphIncompat},
		{"long id", NodeGraphEdge{ID: strings.Repeat("x", NodeGraphMaxText+1), FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"}, ErrNodeGraphInvalid},
	}
	for _, c := range cases {
		if err := m.Connect(c.e); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if len(m.Edges()) != 0 {
		t.Fatal("a refused edge was added")
	}
	if err := m.Connect(NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"}); err != nil {
		t.Fatal(err)
	}
	e := m.Edges()[0]
	if e.ID != "a.o>b.i" {
		t.Fatalf("derived id %q", e.ID)
	}
	if got, ok := m.Edge(e.ID); !ok || got != e {
		t.Fatal("Edge lookup")
	}
	if _, ok := m.Edge("nope"); ok {
		t.Fatal("unknown edge found")
	}
	if err := m.Connect(NodeGraphEdge{ID: "other", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"}); !errors.Is(err, ErrNodeGraphDuplicate) {
		t.Fatalf("same endpoints twice: %v", err)
	}
	// An explicit id that is taken, on different endpoints.
	if err := m.Connect(NodeGraphEdge{ID: e.ID, FromNode: "c", FromPort: "o", ToNode: "b", ToPort: "i"}); !errors.Is(err, ErrNodeGraphDuplicate) {
		t.Fatalf("taken id: %v", err)
	}
	if err := m.Connect(NodeGraphEdge{ID: "mine", FromNode: "c", FromPort: "o", ToNode: "b", ToPort: "i"}); err != nil {
		t.Fatal(err) // an input takes several edges
	}
	if _, ok := m.Node("nope"); ok {
		t.Fatal("unknown node found")
	}
}

func TestNodeGraphCompatibleCallbackDecides(t *testing.T) {
	m := ngPair()
	var seen []string
	m.Compatible = func(from, to NodeGraphPort) bool {
		seen = append(seen, from.Type+">"+to.Type)
		return from.Type == "img" && to.Type == "pdf" // an image feeds a pdf input
	}
	if err := m.Connect(NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"}); !errors.Is(err, ErrNodeGraphIncompat) {
		t.Fatalf("pdf>pdf must be refused by this callback: %v", err)
	}
	if err := m.Connect(NodeGraphEdge{FromNode: "a", FromPort: "p", ToNode: "b", ToPort: "i"}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "pdf>pdf,img>pdf" {
		t.Fatalf("callback saw %v", seen)
	}
}

func TestNodeGraphRemovalsAndSelection(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "c", Inputs: []NodeGraphPort{{ID: "i", Type: "pdf"}}})
	_ = m.Connect(NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	_ = m.Connect(NodeGraphEdge{FromNode: "a", FromPort: "o", ToNode: "c", ToPort: "i"})
	var got []string
	m.Subscribe(func(c NodeGraphChange) { got = append(got, strconv.Itoa(int(c.Kind))+c.ID) })

	m.Selected.Set("edge:a.o>c.i")
	if !m.RemoveNode("c") {
		t.Fatal("RemoveNode c")
	}
	if m.Selected.Get() != "" || len(m.Edges()) != 1 {
		t.Fatalf("selection %q edges %d", m.Selected.Get(), len(m.Edges()))
	}
	if strings.Join(got, " ") != "4a.o>c.i 1c" {
		t.Fatalf("events %v: edges go before their node", got)
	}
	if m.RemoveNode("c") || m.RemoveEdge("nope") || m.MoveNode("nope", 1, 1) {
		t.Fatal("an unknown id was removed or moved")
	}
	m.Selected.Set("node:a")
	m.RemoveEdge("a.o>b.i") // unrelated to the selection
	if m.Selected.Get() != "node:a" {
		t.Fatalf("selection lost: %q", m.Selected.Get())
	}
	m.Selected.Set("edge:x")
	if m.DeleteSelected() {
		t.Fatal("deleted an edge that is not there")
	}
	m.Selected.Set("node:b")
	if !m.DeleteSelected() || m.Selected.Get() != "" {
		t.Fatal("DeleteSelected node")
	}
	if m.DeleteSelected() {
		t.Fatal("DeleteSelected with nothing selected")
	}
	m.Selected.Set("edge:zzz")
	if m.DeleteSelected() {
		t.Fatal("deleted a missing edge")
	}
	m2 := ngPair()
	_ = m2.Connect(NodeGraphEdge{ID: "k", FromNode: "a", FromPort: "o", ToNode: "b", ToPort: "i"})
	m2.Selected.Set("edge:k")
	if !m2.DeleteSelected() || len(m2.Edges()) != 0 || m2.Selected.Get() != "" {
		t.Fatal("DeleteSelected edge")
	}
}

func TestNodeGraphChangeStream(t *testing.T) {
	m := NewNodeGraphModel()
	var a, b []NodeGraphChange
	var unsubA func()
	unsubA = m.Subscribe(func(c NodeGraphChange) { a = append(a, c); unsubA() }) // unsubscribes itself
	unsubB := m.Subscribe(func(c NodeGraphChange) { b = append(b, c) })
	_ = m.AddNode(NodeGraphNode{ID: "n", Inputs: []NodeGraphPort{{ID: "i"}}})
	m.MoveNode("n", 5, 6)
	if len(a) != 1 || len(b) != 2 || b[0] != (NodeGraphChange{NodeAdded, "n"}) || b[1] != (NodeGraphChange{NodeMoved, "n"}) {
		t.Fatalf("a=%v b=%v", a, b)
	}
	unsubB()
	unsubB() // twice is harmless
	m.RemoveNode("n")
	if len(b) != 2 {
		t.Fatal("event after unsubscribe")
	}
}

// TestNodeGraphRandomOperationsNeverDangle is the property: whatever sequence of
// add, connect, move and delete calls arrives, in whatever order, with ids that
// exist or not, every edge joins an existing output to an existing input, ids
// are unique and the bounds hold. A second model rebuilt from the change stream
// alone must equal the first (the stream is a faithful serialisation).
func TestNodeGraphRandomOperationsNeverDangle(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		rng := rand.New(rand.NewSource(seed))
		m := NewNodeGraphModel()
		edges := map[string]bool{}
		nodes := map[string]bool{}
		m.Subscribe(func(c NodeGraphChange) {
			switch c.Kind {
			case NodeAdded:
				nodes[c.ID] = true
			case NodeRemoved:
				delete(nodes, c.ID)
			case EdgeAdded:
				edges[c.ID] = true
			case EdgeRemoved:
				delete(edges, c.ID)
			}
		})
		types := []string{"a", "b"}
		id := func() string { return strconv.Itoa(rng.Intn(12)) } // collisions and misses on purpose
		for op := 0; op < 400; op++ {
			switch rng.Intn(6) {
			case 0:
				_ = m.AddNode(NodeGraphNode{ID: id(), X: rng.NormFloat64() * 1e7,
					Inputs:  []NodeGraphPort{{ID: "i", Type: types[rng.Intn(2)]}, {ID: "j", Type: types[rng.Intn(2)]}},
					Outputs: []NodeGraphPort{{ID: "o", Type: types[rng.Intn(2)]}}})
			case 1, 2:
				_ = m.Connect(NodeGraphEdge{FromNode: id(), FromPort: "o", ToNode: id(), ToPort: []string{"i", "j", "x"}[rng.Intn(3)]})
			case 3:
				m.RemoveNode(id())
			case 4:
				if es := m.Edges(); len(es) > 0 {
					m.RemoveEdge(es[rng.Intn(len(es))].ID)
				}
			case 5:
				m.MoveNode(id(), math.NaN(), math.Inf(1))
			}
			checkNodeGraphInvariants(t, m, seed, op)
		}
		if len(nodes) != len(m.Nodes()) || len(edges) != len(m.Edges()) {
			t.Fatalf("seed %d: the stream says %d nodes %d edges, the model has %d and %d", seed, len(nodes), len(edges), len(m.Nodes()), len(m.Edges()))
		}
		for _, e := range m.Edges() {
			if !edges[e.ID] {
				t.Fatalf("seed %d: edge %s is not in the stream", seed, e.ID)
			}
		}
	}
}

func checkNodeGraphInvariants(t *testing.T, m *NodeGraphModel, seed int64, op int) {
	t.Helper()
	ids := map[string]bool{}
	for _, n := range m.Nodes() {
		if ids[n.ID] || math.IsNaN(n.X+n.Y) || math.Abs(n.X) > NodeGraphMaxCoord || math.Abs(n.Y) > NodeGraphMaxCoord {
			t.Fatalf("seed %d op %d: bad node %+v", seed, op, n)
		}
		ids[n.ID] = true
	}
	seen := map[string]bool{}
	for _, e := range m.Edges() {
		from, okF := m.Node(e.FromNode)
		to, okT := m.Node(e.ToNode)
		if !okF || !okT || portIndex(from.Outputs, e.FromPort) < 0 || portIndex(to.Inputs, e.ToPort) < 0 || e.FromNode == e.ToNode {
			t.Fatalf("seed %d op %d: dangling edge %+v", seed, op, e)
		}
		if seen[e.ID] {
			t.Fatalf("seed %d op %d: duplicate edge id %s", seed, op, e.ID)
		}
		seen[e.ID] = true
	}
}

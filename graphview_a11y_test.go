// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

func a11yGraph() *GraphView {
	m := NewGraphViewModel()
	m.Clusters.Append(GraphCluster{ID: "p", Label: "shop"})
	m.Nodes.Append(
		GraphNode{ID: "host", Label: "this tab", Shape: GraphShapeGateway},
		GraphNode{ID: "front", Label: "frontend", Shape: GraphShapeHub},
		GraphNode{ID: "back", Label: "backend", Shape: GraphShapeHub},
		GraphNode{ID: "web", Label: "web", Cluster: "p",
			Badges:  []GraphBadge{{Text: "muted"}},
			Toggles: []GraphToggle{{ID: "snd", Label: "sound", On: true}, {ID: "x", Label: "", On: false}}},
		GraphNode{ID: "bare", Cluster: "p"},
	)
	m.Edges.Append(
		GraphEdge{ID: "1", From: "web", To: "front"},
		GraphEdge{ID: "2", From: "back", To: "web"},
		GraphEdge{ID: "3", From: "host", To: "web", Label: "127.0.0.1:8080 -> 80/tcp"},
		GraphEdge{ID: "4", From: "host", To: "bare"}, // an edge with no label says nothing
		GraphEdge{ID: "5", From: "bare", To: "web"},  // between two boxes: not a hub
	)
	g := NewGraphView(m)
	g.Layout = GraphLayoutBands
	g.SetBounds(Rect{W: 500, H: 300})
	return g
}

// A hub with no label is named by its id.
func TestGraphViewA11yNamesAnUnlabelledHubByItsID(t *testing.T) {
	g := a11yGraph()
	g.Model.Nodes.Append(GraphNode{ID: "net-x", Shape: GraphShapeHub})
	g.Model.Edges.Append(GraphEdge{ID: "6", From: "bare", To: "net-x"})
	var got string
	for _, n := range WalkA11y(g) {
		if n.Key == "GraphView#0/@node:bare" {
			got = n.Name
		}
	}
	if got != "bare, on net-x" {
		t.Fatalf("name = %q, want %q", got, "bare, on net-x")
	}
}

func TestGraphViewA11yIsATreeOfNodesWithSwitches(t *testing.T) {
	g := a11yGraph()
	g.Model.Selected.Set("node:web")
	type row struct {
		Role        Role
		Name, Value string
		Key         string
	}
	var got []row
	for _, n := range WalkA11y(g) {
		got = append(got, row{n.Role, n.Name, n.Value, n.Key})
	}
	want := []row{
		{RoleGroup, "network graph", "5 nodes, 5 edges, 0 denied; selected node:web", "GraphView#0"},
		{RoleTreeItem, "this tab", "", "GraphView#0/@node:host"},
		{RoleTreeItem, "backend", "", "GraphView#0/@node:back"}, // hubs by mean x, then id
		{RoleTreeItem, "frontend", "", "GraphView#0/@node:front"},
		{RoleTreeItem, "web, on backend and frontend, publishes 127.0.0.1:8080 -> 80/tcp, muted", "selected", "GraphView#0/@node:web"},
		{RoleSwitch, "sound", "on", "GraphView#0/@node:web/@toggle:web%2Fsnd"},
		{RoleSwitch, "x", "off", "GraphView#0/@node:web/@toggle:web%2Fx"},
		{RoleTreeItem, "bare", "", "GraphView#0/@node:bare"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("a11y tree:\n got %v\nwant %v", got, want)
	}
	for _, n := range WalkA11y(g)[1:] {
		if n.Rect.W <= 0 || n.Rect.H <= 0 {
			t.Errorf("%s has no area: %v", n.Name, n.Rect)
		}
	}
}

func TestGraphViewA11yAnnouncesZoomOffHundred(t *testing.T) {
	g := a11yGraph()
	if v := g.A11y().Value; v != "5 nodes, 5 edges, 0 denied" {
		t.Errorf("at 100%% the value is %q", v)
	}
	g.SetZoom(150, 0, 0)
	if v := g.A11y().Value; v != "5 nodes, 5 edges, 0 denied; zoom 150%" {
		t.Errorf("at 150%% the value is %q", v)
	}
}

// Nodes scrolled out of the view, and toggles of a node cut by its edge, are
// not pointed at: every proxy lies inside the widget.
func TestGraphViewA11yProxiesStayInsideTheView(t *testing.T) {
	g := a11yGraph()
	g.SetBounds(Rect{X: 3, Y: 4, W: 500, H: 60})
	g.Draw(newP(makeSurface(520, 80), 520), DefaultLight())
	all := g.Model.Nodes.Len()
	nodes := 0
	for _, n := range WalkA11y(g)[1:] {
		if n.Role == RoleTreeItem {
			nodes++
		}
		if r := g.Bounds(); n.Rect.X < r.X || n.Rect.Y < r.Y || n.Rect.X+n.Rect.W > r.X+r.W || n.Rect.Y+n.Rect.H > r.Y+r.H {
			t.Errorf("%s at %v is outside %v", n.Name, n.Rect, r)
		}
	}
	if nodes == 0 || nodes >= all {
		t.Errorf("%d of %d nodes in a view that shows only some", nodes, all)
	}
	if _, ok := graphClip(Rect{X: 0, Y: 0, W: 5, H: 5}, Rect{X: 5, Y: 0, W: 5, H: 5}); ok {
		t.Error("a rect touching the view's edge counts as inside")
	}
	if r, ok := graphClip(Rect{X: 8, Y: 1, W: 5, H: 2}, Rect{X: 5, Y: 0, W: 5, H: 5}); !ok || r != (Rect{X: 8, Y: 1, W: 2, H: 2}) {
		t.Errorf("clip = %v %v", r, ok)
	}
}

func TestGraphViewA11yNameListsOfThree(t *testing.T) {
	if got := joinAnd([]string{"a", "b", "c"}); got != "a, b and c" {
		t.Errorf("joinAnd = %q", got)
	}
	if joinAnd(nil) != "" || joinAnd([]string{"a"}) != "a" || joinAnd([]string{"a", "b"}) != "a and b" {
		t.Error("joinAnd of 0, 1, 2")
	}
}

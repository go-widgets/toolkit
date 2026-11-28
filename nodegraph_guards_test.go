// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

// ngSpy records the round rects a Draw fills; it is a plain Painter, so the
// widget takes its fallback path.
type ngSpy struct{ rounds []painter.Rect }

func (s *ngSpy) FillRect(painter.Rect, RGBA)                 {}
func (s *ngSpy) StrokeRect(painter.Rect, RGBA, int)          {}
func (s *ngSpy) FillRoundRect(r painter.Rect, _ int, _ RGBA) { s.rounds = append(s.rounds, r) }
func (s *ngSpy) StrokeRoundRect(painter.Rect, int, RGBA, int) {
}
func (s *ngSpy) PutPixel(int, int, RGBA)     {}
func (s *ngSpy) Text(int, int, string, RGBA) {}
func (s *ngSpy) Size() (int, int)            { return 700, 500 }

func TestNodeGraphDrawSkipsOffscreenNodesEntirely(t *testing.T) {
	m := ngPair()
	_ = m.AddNode(NodeGraphNode{ID: "far", X: 5000, Y: 5000})
	_ = m.AddNode(NodeGraphNode{ID: "left", X: -5000, Y: 40})
	g := ngView(m)
	s := &ngSpy{}
	g.Draw(s, DefaultLight())
	bodies := 0
	for _, r := range s.rounds {
		if r.W >= 100 { // a node body is 140 wide; port dots are small
			bodies++
		}
		if r.X > 700 || r.X < -100 {
			t.Fatalf("a shape was filled off screen at %v", r)
		}
	}
	if bodies != 2 {
		t.Fatalf("%d node bodies filled, want 2 (the two on-screen nodes)", bodies)
	}
}

func TestNodeGraphGuardsOnUnknownIDs(t *testing.T) {
	m := ngPair()
	if m.RemoveNode("zz") {
		t.Fatal("removed an unknown node")
	}
	if m.setPos("zz", 1, 2) || len(m.Nodes()) != 2 {
		t.Fatal("moved an unknown node")
	}
	if err := m.CanConnect("zz", "o", "b", "i"); !errors.Is(err, ErrNodeGraphUnknown) {
		t.Fatalf("unknown source: %v", err)
	}
	if err := m.CanConnect("a", "o", "zz", "i"); !errors.Is(err, ErrNodeGraphUnknown) {
		t.Fatalf("unknown target: %v", err)
	}
}

func TestNodeGraphCutTextBoundary(t *testing.T) {
	exact := strings.Repeat("a", NodeGraphMaxText)
	if cutText(exact) != exact {
		t.Fatal("a text of exactly the limit was cut")
	}
	if got := cutText(exact + "b"); got != exact {
		t.Fatalf("one over the limit: len %d", len(got))
	}
}

func TestNodeGraphStepOnAnEmptyGraphSelectsNothing(t *testing.T) {
	g := NewNodeGraph(nil)
	g.step(true)
	g.step(false)
	if g.Model.Selected.Get() != "" {
		t.Fatal("selected something in an empty graph")
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// The ribbon reports a toolbar whose children are a tablist (one tab each, the
// active one "selected") and the controls of the shown tab, with their rects.
func TestRibbonAccessibilityTree(t *testing.T) {
	r, _ := rtRibbon(2000)
	r.SetBounds(Rect{X: 10, Y: 20, W: 2000, H: r.Height()})
	r.Tabs[0].Groups[1].Controls[3].check().Set(true)
	nodes := WalkA11y(r)
	if nodes[0].Role != RoleToolbar || nodes[0].Name != "Ribbon" || nodes[0].Value != "Home" {
		t.Fatalf("root %+v", nodes[0].A11yInfo)
	}
	if nodes[1].Role != RoleTablist {
		t.Fatalf("second node %+v", nodes[1].A11yInfo)
	}
	var tabs []A11yNode
	for _, n := range nodes {
		if n.Role == RoleTab {
			tabs = append(tabs, n)
		}
	}
	if len(tabs) != 3 || tabs[0].Value != "selected" || tabs[1].Value != "" || tabs[0].Name != "Home" {
		t.Errorf("tabs %+v", tabs)
	}
	if tabs[0].Rect.X < 10 || tabs[0].Rect.Y != 20 || tabs[0].Rect.H != r.metrics().tabH {
		t.Errorf("a tab's rect is where it is drawn, got %+v", tabs[0].Rect)
	}
	byName := map[string]A11yNode{}
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if n := byName["Word wrap"]; n.Role != RoleCheckbox || n.Value != "checked" {
		t.Errorf("a checked toggle %+v", n.A11yInfo)
	}
	if n := byName["Duplicate"]; n.Role != RoleButton || n.Value != "" {
		t.Errorf("a plain button %+v", n.A11yInfo)
	}
	if n := byName["New slide"]; n.Role != RoleButton || n.Value != "collapsed" {
		t.Errorf("a split button %+v", n.A11yInfo)
	}
	// Open its menu: aria-expanded.
	r.activate(rtCell(r, "New slide"), partArrow)
	if n := nodeNamed(WalkA11y(r), "New slide"); n.Value != "expanded" {
		t.Errorf("an open split button %+v", n.A11yInfo)
	}
}

func nodeNamed(nodes []A11yNode, name string) A11yNode {
	for _, n := range nodes {
		if n.Name == name {
			return n
		}
	}
	return A11yNode{}
}

func TestRibbonAccessibilityOfFoldedGroupsAndCollapse(t *testing.T) {
	r, _ := rtRibbon(360)
	if n := nodeNamed(WalkA11y(r), "Theme"); n.Role != RoleButton || n.Value != "collapsed" {
		t.Errorf("a folded group is a collapsed button: %+v", n.A11yInfo)
	}
	r.activate(rtCell(r, "Theme"), partMain)
	if n := nodeNamed(WalkA11y(r), "Kinds"); n.Role != RoleButton || n.Value != "collapsed" {
		t.Errorf("an open folded group lists its own controls: %+v", n.A11yInfo)
	}
	r.Dismiss()
	r.Collapsed().Set(true)
	for _, n := range WalkA11y(r) {
		if n.Name == "Duplicate" {
			t.Error("a collapsed ribbon announces no controls")
		}
	}
}

func TestRibbonIsAccessible(t *testing.T) {
	var w Widget = &Ribbon{}
	if _, ok := w.(Accessible); !ok {
		t.Fatal("Ribbon must implement Accessible")
	}
	if got := CollectA11y([]Widget{NewRibbon(&RibbonTab{Label: "T"})}); len(got) != 1 || got[0].Role != RoleToolbar {
		t.Errorf("CollectA11y = %+v", got)
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func a11yToolbar(vertical bool) *Toolbar {
	tb := NewToolbar([]ToolbarItem{
		{Label: "Open"}, {Separator: true}, {Label: "Save", Disabled: true}, {Label: "Quit"},
	})
	if vertical {
		tb.Orientation = Vertical
	}
	tb.SetBounds(Rect{X: 10, Y: 20, W: 200, H: 200})
	return tb
}

// Every button is a node with role, name and its own Disabled; the separator is
// decoration and has none.
func TestToolbarItemsAreA11yNodes(t *testing.T) {
	nodes := WalkA11y(a11yToolbar(false))
	if len(nodes) != 4 || nodes[0].Role != RoleToolbar {
		t.Fatalf("nodes = %+v, want the toolbar and 3 buttons", nodes)
	}
	want := []struct {
		name     string
		disabled bool
	}{{"Open", false}, {"Save", true}, {"Quit", false}}
	for i, w := range want {
		n := nodes[i+1]
		if n.Role != RoleButton || n.Name != w.name || n.Disabled != w.disabled {
			t.Errorf("node %d = %+v, want button %q disabled=%v", i+1, n.A11yInfo, w.name, w.disabled)
		}
	}
}

// A node's rectangle is the cell Draw paints and hitTest answers for, in both
// orientations, so a screen reader points at the button it names.
func TestToolbarItemNodeRectsMatchTheHitArea(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		tb := a11yToolbar(vertical)
		for i, n := range tb.Children() {
			r := n.Bounds()
			if r.W != tb.buttonW() || r.H != tb.buttonH() {
				t.Errorf("vertical=%v node %d is %dx%d, want the button size", vertical, i, r.W, r.H)
			}
			idx := tb.hitTest(r.X-tb.Bounds().X+1, r.Y-tb.Bounds().Y+1)
			if tb.Items[idx].Label != n.(Accessible).A11y().Name {
				t.Errorf("vertical=%v node %d (%q) sits over item %q", vertical, i, n.(Accessible).A11y().Name, tb.Items[idx].Label)
			}
		}
	}
}

// itemRect of a separator is the divider strip; out of range is empty.
func TestToolbarItemRect(t *testing.T) {
	h := a11yToolbar(false)
	if got, want := h.itemRect(1), (Rect{X: 10 + h.buttonW(), Y: 20, W: h.sepW(), H: h.buttonH()}); got != want {
		t.Errorf("horizontal separator rect = %+v, want %+v", got, want)
	}
	v := a11yToolbar(true)
	if got, want := v.itemRect(1), (Rect{X: 10, Y: 20 + v.buttonH(), W: v.buttonW(), H: v.sepW()}); got != want {
		t.Errorf("vertical separator rect = %+v, want %+v", got, want)
	}
	if h.itemRect(-1) != (Rect{}) || h.itemRect(len(h.Items)) != (Rect{}) {
		t.Error("an index out of range must answer the empty rectangle")
	}
}

// Disabling the toolbar disables every button node while the item's own flag is
// untouched; enabling restores each to its own.
func TestToolbarDisabledReachesItemNodes(t *testing.T) {
	tb := a11yToolbar(false)
	tb.Disabled().Set(true)
	for _, n := range WalkA11y(tb) {
		if !n.Disabled {
			t.Errorf("%s %q reads enabled in a disabled toolbar", n.Role, n.Name)
		}
	}
	tb.Disabled().Set(false)
	if tb.Items[0].Disabled || WalkA11y(tb)[1].Disabled {
		t.Error("re-enabling the toolbar did not restore the item")
	}
}

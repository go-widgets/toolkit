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
	// not square: swapping W and H, or taking the wrong one along an axis, shows
	tb.ButtonW, tb.ButtonH = 30, 18
	tb.SetBounds(Rect{X: 10, Y: 20, W: 150, H: 150})
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
			if idx < 0 {
				t.Errorf("vertical=%v node %d (%q) sits over no button", vertical, i, n.(Accessible).A11y().Name)
				continue
			}
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

// A node's rectangle is the cell Draw actually paints: the cell's frame is
// stroked in theme.Border, so its four corners carry that colour and the pixel
// past the far edge of the cross axis does not. hitTest and itemRect share
// itemStep, so agreeing with hitTest alone would not catch a wrong step.
func TestToolbarItemNodeRectsMatchWhatDrawPaints(t *testing.T) {
	theme := DefaultLight()
	for _, vertical := range []bool{false, true} {
		tb := a11yToolbar(vertical)
		buf := make([]byte, 4*v04SurfW*v04SurfH)
		tb.Draw(newP(buf, v04SurfW), theme)
		for i, n := range tb.Children() {
			r := n.Bounds()
			for _, c := range [][2]int{{r.X, r.Y}, {r.X + r.W - 1, r.Y}, {r.X, r.Y + r.H - 1}, {r.X + r.W - 1, r.Y + r.H - 1}} {
				if got := pixelAt(buf, v04SurfW, c[0], c[1]); got != theme.Border {
					t.Errorf("vertical=%v node %d %+v: corner (%d,%d) is %+v, want the stroked frame %+v", vertical, i, r, c[0], c[1], got, theme.Border)
				}
			}
			// just past the cell on the cross axis is the strip's ground
			px, py := r.X+1, r.Y+r.H // below the cell
			if vertical {
				px, py = r.X+r.W, r.Y+1 // right of the cell
			}
			if got := pixelAt(buf, v04SurfW, px, py); got == theme.Border {
				t.Errorf("vertical=%v node %d %+v: pixel (%d,%d) past the cell is stroked, the rect is too small", vertical, i, r, px, py)
			}
		}
	}
}

// The cells advance by the button size along the axis and a separator by sepW,
// in both orientations, with a button that is not square.
func TestToolbarItemStep(t *testing.T) {
	h, v := a11yToolbar(false), a11yToolbar(true)
	bw, bh, sw := h.buttonW(), h.buttonH(), h.sepW()
	if bw == bh {
		t.Fatal("the fixture must not be square")
	}
	if got := h.itemStep(h.Items[0], bw, bh, sw); got != bw {
		t.Errorf("horizontal button step = %d, want %d", got, bw)
	}
	if got := v.itemStep(v.Items[0], bw, bh, sw); got != bh {
		t.Errorf("vertical button step = %d, want %d", got, bh)
	}
	for _, tb := range []*Toolbar{h, v} {
		if got := tb.itemStep(tb.Items[1], bw, bh, sw); got != sw {
			t.Errorf("separator step = %d, want %d", got, sw)
		}
	}
	// the third item sits after a button and a separator
	if got, want := h.itemRect(2).X, h.Bounds().X+bw+sw; got != want {
		t.Errorf("horizontal item 2 at x=%d, want %d", got, want)
	}
	if got, want := v.itemRect(2).Y, v.Bounds().Y+bh+sw; got != want {
		t.Errorf("vertical item 2 at y=%d, want %d", got, want)
	}
}

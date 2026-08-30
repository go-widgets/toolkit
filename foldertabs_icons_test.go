// Copyright (c) the go-widgets authors.
// SPDX-License-Identifier: BSD-3-Clause

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// A FolderTabs tab with an optional leading Icon paints that glyph before its
// caption and widens to fit it; tabs without an icon (a nil entry, or a short
// Icons slice) are unchanged.
func TestFolderTabsIcons(t *testing.T) {
	const W, H = 260, 34
	theme := DefaultLight()
	mark := RGB(0xC0, 0x20, 0x40)

	// Two tabs; only the first carries an icon (the second is a nil entry).
	ft := &FolderTabs{
		Labels: []string{"Rendered", "Log"},
		Icons: []IconFunc{
			func(p painter.Painter, r Rect, ink RGBA) { fillRect(p, r.X, r.Y, r.W, r.H, mark) },
			nil,
		},
	}
	ft.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})

	// The iconed tab is wider than it would be from its label alone.
	plain := &FolderTabs{Labels: []string{"Rendered", "Log"}}
	plain.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	if ft.TabRect(0).W <= plain.TabRect(0).W {
		t.Errorf("iconed tab width %d should exceed the plain width %d", ft.TabRect(0).W, plain.TabRect(0).W)
	}
	// A nil icon entry (tab 1) leaves that tab unchanged.
	if ft.tabIcon(1) != nil {
		t.Error("tab 1 has a nil Icons entry; tabIcon should report nil")
	}
	// An index past the Icons slice is nil (short-slice case).
	shorter := &FolderTabs{Labels: []string{"A", "B", "C"}, Icons: []IconFunc{nil}}
	if shorter.tabIcon(2) != nil {
		t.Error("an index past the Icons slice must report no icon")
	}

	// The icon paints inside tab 0.
	buf := makeSurface(W, H)
	ft.Draw(newP(buf, W), theme)
	tr := ft.TabRect(0)
	found := false
	for y := tr.Y; y < tr.Y+tr.H; y++ {
		for x := tr.X; x < tr.X+tr.W; x++ {
			if pixelAt(buf, W, x, y) == mark {
				found = true
			}
		}
	}
	if !found {
		t.Error("the leading tab icon did not paint")
	}
}

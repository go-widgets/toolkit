// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"testing"
)

// --- TreeView vertical scrolling + windowed rendering ----------------------

// manyLeaves builds an expanded root with n leaf children labeled n0..n(n-1),
// for tests that need a tree taller than any reasonable viewport.
func manyLeaves(n int) (root *TreeNode, children []*TreeNode) {
	children = make([]*TreeNode, n)
	for i := range children {
		children[i] = &TreeNode{Label: fmt.Sprintf("n%d", i)}
	}
	root = &TreeNode{Label: "root", Expanded: true, Children: children}
	return root, children
}

// --- regression: small (fully-fitting) tree is byte-identical --------------

func TestTreeViewScrollSmallTreeByteIdenticalToUnvirtualized(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetSelection(root)
	// Non-zero bounds offset + a surface bigger than Bounds to also
	// confirm nothing paints outside Bounds either way.
	tv.SetBounds(Rect{X: 3, Y: 2, W: 150, H: 200})

	const w, h = 200, 220
	theme := DefaultLight()

	got := makeSurface(w, h)
	tv.Draw(newP(got, w), theme)

	// want is painted by the pre-virtualization algorithm, replicated
	// verbatim: full row width, no clip, no scrollbar, rows 0..N drawn
	// at y = r.Y + i*rh.
	want := makeSurface(w, h)
	p := newP(want, w)
	tv.flatten()
	r := tv.Bounds()
	rh := 18
	for i, row := range tv.rows {
		y := r.Y + i*rh
		bg := theme.Surface
		ink := theme.OnSurface
		if tv.IsSelected(row.node) {
			bg = theme.Accent
			ink = theme.Background
		}
		fillRect(p, r.X, y, r.W, rh, bg)
		indent := r.X + row.depth*TreeIndentW
		if len(row.node.Children) > 0 {
			cx := indent + 4
			cy := y + rh/2
			if row.node.Expanded {
				for q := 0; q < 4; q++ {
					fillRect(p, cx-q, cy+2-q, 1+2*q, 1, ink)
				}
			} else {
				for q := 0; q < 4; q++ {
					fillRect(p, cx+2-q, cy-q, 1, 1+2*q, ink)
				}
			}
		}
		textY := y + (rh-tv.glyphHeight())/2
		tv.drawText(p, indent+TreeChevronW, textY, row.node.Label, ink)
	}

	if !bytes.Equal(got, want) {
		t.Fatal("Draw of a fully-fitting tree is not byte-identical to the pre-virtualization algorithm")
	}
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow = %d, want 0 (whole tree fits)", tv.ScrollRow().Get())
	}
}

func TestTreeViewScrollSmallTreeRegressionExpandCollapseAndMultiSelect(t *testing.T) {
	root, a, b, _, c, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.MultiSelect = true
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200}) // windowRows=11, total=7: fits

	clickLabel(tv, 1, false, false) // a
	clickLabel(tv, 4, true, false)  // Ctrl-click c
	if !tv.IsSelected(a) || !tv.IsSelected(c) {
		t.Fatal("multi-select broken by adding virtualization")
	}

	// b's chevron: depth 1 → chevronX = 20.
	tv.OnEvent(Event{Kind: EventClick, X: 20, Y: 2 * 18})
	if b.Expanded {
		t.Fatal("expand/collapse broken by adding virtualization")
	}

	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow = %d, want 0 (whole tree still fits after collapse)", tv.ScrollRow().Get())
	}
}

// --- windowing over a large/expanded tree -----------------------------------

func TestTreeViewScrollWindowsLargeTree(t *testing.T) {
	root, children := manyLeaves(20) // flattened: root(0), n0..n19(1..20)
	tv := NewTreeView(root)
	tv.Selected().Set(children[15]) // flattened idx 16
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90})

	theme := DefaultLight()
	contentW := 200 - scrollbarWidth
	hasAccentInContent := func(buf []byte) bool {
		for y := 0; y < 90; y++ {
			for x := 0; x < contentW; x++ {
				if pixelAt(buf, 200, x, y) == theme.Accent {
					return true
				}
			}
		}
		return false
	}

	// Default ScrollRow=0: window [0,5) = root,n0..n3. idx16 not in it.
	buf1 := makeSurface(200, 90)
	tv.Draw(newP(buf1, 200), theme)
	if hasAccentInContent(buf1) {
		t.Fatal("selected node outside the scroll window was painted")
	}

	// Scroll so idx16 enters the window: max = total(21) - window(5) = 16.
	tv.ScrollTo(16)
	if tv.ScrollRow().Get() != 16 {
		t.Fatalf("ScrollRow = %d, want 16", tv.ScrollRow().Get())
	}
	buf2 := makeSurface(200, 90)
	tv.Draw(newP(buf2, 200), theme)
	if !hasAccentInContent(buf2) {
		t.Fatal("selected node inside the scroll window was not painted")
	}
}

func TestTreeViewZeroHeightBoundsSkipsVirtualization(t *testing.T) {
	// Bounds().H <= 0 → windowRows() is 0, which both Draw and OnEvent
	// must treat as "don't virtualize" (paint / hit-test every row),
	// matching a pre-virtualization TreeView.
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 0})

	tv.Draw(newP(makeSurface(200, 200), 200), DefaultLight())
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 18}) // row 1: a
	if tv.Selected().Get() == nil || tv.Selected().Get().Label != "a" {
		t.Fatalf("Selected = %+v, want a despite H=0 bounds", tv.Selected().Get())
	}
}

// --- collapsing shrinks the flattened count + re-clamps ScrollRow ----------

func TestTreeViewScrollCollapseViaHostReClampsScrollRow(t *testing.T) {
	leaves := make([]*TreeNode, 20)
	for i := range leaves {
		leaves[i] = &TreeNode{Label: fmt.Sprintf("s%d", i)}
	}
	subA := &TreeNode{Label: "subA", Expanded: true, Children: leaves}
	subB := &TreeNode{Label: "subB"}
	root := &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{subA, subB}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5

	// Flattened total = root+subA+20 leaves+subB = 23; max ScrollRow=18.
	tv.ScrollTo(18)
	if tv.ScrollRow().Get() != 18 {
		t.Fatalf("ScrollRow = %d, want 18", tv.ScrollRow().Get())
	}

	subA.Expanded = false // flattened count drops to 3 (root, subA, subB)
	tv.Draw(newP(makeSurface(200, 90), 200), DefaultLight())
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow after collapse = %d, want re-clamped to 0", tv.ScrollRow().Get())
	}
}

func TestTreeViewScrollClickCollapseReClampsScrollRow(t *testing.T) {
	bLeaves := make([]*TreeNode, 20)
	for i := range bLeaves {
		bLeaves[i] = &TreeNode{Label: fmt.Sprintf("b%d", i)}
	}
	A := &TreeNode{Label: "A"}
	B := &TreeNode{Label: "B", Expanded: true, Children: bLeaves}
	C := &TreeNode{Label: "C"}
	root := &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{A, B, C}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5

	tv.ScrollTo(2) // window [2,7): B, b0, b1, b2, b3
	if tv.ScrollRow().Get() != 2 {
		t.Fatalf("ScrollRow = %d, want 2", tv.ScrollRow().Get())
	}

	// B (depth 1) sits at local row 0 of the window: chevron at X=20.
	tv.OnEvent(Event{Kind: EventClick, X: 20, Y: 0})
	if B.Expanded {
		t.Fatal("chevron click should have collapsed B")
	}
	// Post-collapse flattened total = root,A,B,C = 4 <= window(5): re-clamp to 0.
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollRow after click-collapse = %d, want re-clamped to 0", tv.ScrollRow().Get())
	}
}

// --- click hit-testing through a non-zero ScrollRow -------------------------

func TestTreeViewScrollClickWithNonZeroScrollRowSelectsCorrectNode(t *testing.T) {
	root, children := manyLeaves(20)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5

	tv.ScrollTo(8) // window [8,13): n7..n11
	// Local row 2 of the window → flattened idx 10 → children[9] (n9).
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 2 * 18})
	if tv.Selected().Get() != children[9] {
		t.Fatalf("Selected = %v, want n9 (children[9])", tv.Selected().Get())
	}
}

func TestTreeViewClickBelowWindowIgnored(t *testing.T) {
	root, _ := manyLeaves(20)
	tv := NewTreeView(root)
	// H=100, rh=18 → windowRows=5 (90px used), leaving a 10px dead zone.
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.ScrollTo(2)

	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 5 * 18}) // local row 5 == windowRows: past the window
	if tv.Selected().Get() != nil {
		t.Fatal("click below the painted window should not select anything")
	}
}

// --- ScrollTo / ScrollBy clamping -------------------------------------------

func TestTreeViewScrollToAndScrollByClamp(t *testing.T) {
	root, _ := manyLeaves(20)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5, total=21, max=16

	tv.ScrollTo(-5)
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollTo(-5) = %d, want clamped to 0", tv.ScrollRow().Get())
	}
	tv.ScrollTo(999)
	if tv.ScrollRow().Get() != 16 {
		t.Fatalf("ScrollTo(999) = %d, want clamped to 16", tv.ScrollRow().Get())
	}
	tv.ScrollTo(10)
	tv.ScrollBy(3)
	if tv.ScrollRow().Get() != 13 {
		t.Fatalf("ScrollBy(3) from 10 = %d, want 13", tv.ScrollRow().Get())
	}
	tv.ScrollBy(-100)
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("ScrollBy(-100) = %d, want clamped to 0", tv.ScrollRow().Get())
	}
}

// --- scrollToSelected ---------------------------------------------------

func TestTreeViewScrollToSelectedNilIsNoop(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 18}) // windowRows=1
	tv.ScrollRow().Set(3)

	tv.scrollToSelected()
	if tv.ScrollRow().Get() != 3 {
		t.Fatalf("scrollToSelected with nil Selected changed ScrollRow to %d, want unchanged 3", tv.ScrollRow().Get())
	}
}

func TestTreeViewScrollToSelectedNotVisibleIsNoop(t *testing.T) {
	// d is collapsed, so its child d1 isn't in the flattened order.
	root, _, _, _, _, d, d1, _ := newMultiSelectTree()
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	tv.Selected().Set(d1)
	tv.ScrollRow().Set(0)
	_ = d

	tv.scrollToSelected()
	if tv.ScrollRow().Get() != 0 {
		t.Fatalf("scrollToSelected with a hidden Selected changed ScrollRow to %d, want unchanged 0", tv.ScrollRow().Get())
	}
}

func TestTreeViewScrollToSelectedAdjustsMinimally(t *testing.T) {
	root, children := manyLeaves(20)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5, total=21

	// Selected below the window: scroll down just enough to reveal it.
	tv.Selected().Set(children[15]) // flattened idx 16
	tv.scrollToSelected()
	if want := 16 - 5 + 1; tv.ScrollRow().Get() != want {
		t.Fatalf("ScrollRow = %d, want %d (minimal downward scroll)", tv.ScrollRow().Get(), want)
	}

	// Selected above the window: scroll up to put it at the top.
	tv.Selected().Set(children[0]) // flattened idx 1
	tv.scrollToSelected()
	if tv.ScrollRow().Get() != 1 {
		t.Fatalf("ScrollRow = %d, want 1 (minimal upward scroll)", tv.ScrollRow().Get())
	}
}

// --- right-edge scrollbar ----------------------------------------------

func TestTreeViewScrollbarPaintedOnOverflow(t *testing.T) {
	root, _ := manyLeaves(20)
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90}) // windowRows=5, total=21: overflow

	theme := DefaultLight()
	trackX := 200 - scrollbarWidth

	scan := func(scrollRow int) (foundAlt, foundAccent bool) {
		tv.ScrollTo(scrollRow)
		buf := makeSurface(200, 90)
		tv.Draw(newP(buf, 200), theme)
		for y := 0; y < 90; y++ {
			for x := trackX; x < 200; x++ {
				switch pixelAt(buf, 200, x, y) {
				case scrollbarTrackColor(theme):
					foundAlt = true
				case scrollbarThumbColor(theme):
					foundAccent = true
				}
			}
		}
		return
	}

	alt0, accent0 := scan(0)
	if !alt0 {
		t.Fatal("scrollbar track (SurfaceAlt) not painted")
	}
	if !accent0 {
		t.Fatal("scrollbar thumb (Accent) not painted")
	}

	// Scrolling should move the thumb: with ScrollRow at the max, the
	// bottom of the track (not the top) must show the thumb colour.
	tv.ScrollTo(16) // max
	buf := makeSurface(200, 90)
	tv.Draw(newP(buf, 200), theme)
	// Sample the track's centre column: the rounded thumb's cap tapers at the
	// track edge, so the leftmost column can be bare even where the thumb sits.
	if pixelAt(buf, 200, trackX+scrollbarWidth/2, 87) != scrollbarThumbColor(theme) {
		t.Fatal("thumb did not move to the bottom of the track at max ScrollRow")
	}
}

func TestTreeViewScrollbarThumbMinimumSize(t *testing.T) {
	root, _ := manyLeaves(500) // 501 flattened rows against a 5-row window
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 90})

	// With 501 rows against a 5-row window the proportional thumb is a sliver;
	// the floor clamps it to a grabbable minimum. Assert the clamped length from
	// geometry — a slim track's rounded caps taper the exact-colour centre column,
	// so counting pixels there under-reports a thumb that is in fact at the floor.
	g, ok := tv.scrollbarGeom()
	if !ok {
		t.Fatal("expected an overflow scrollbar")
	}
	if g.thumbLen < scaled(8) {
		t.Fatalf("thumb length = %d px, want >= %d (clamped minimum)", g.thumbLen, scaled(8))
	}
}

func TestTreeViewNoScrollbarWhenTreeFits(t *testing.T) {
	root, _, _, _, _, _, _, _ := newMultiSelectTree() // 7 rows
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200}) // windowRows=11: fits

	theme := DefaultLight()
	buf := makeSurface(200, 200)
	tv.Draw(newP(buf, 200), theme)

	trackX := 200 - scrollbarWidth
	for y := 0; y < 200; y++ {
		for x := trackX; x < 200; x++ {
			if got := pixelAt(buf, 200, x, y); got == theme.SurfaceAlt {
				t.Fatalf("scrollbar track painted at (%d,%d) though the tree fits", x, y)
			}
		}
	}
}

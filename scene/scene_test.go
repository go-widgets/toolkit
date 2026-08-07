// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Date: 2026-08-07
package scene

import (
	"testing"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// ---------------------------------------------------------------------------
// test widgets
// ---------------------------------------------------------------------------

// cell is an opaque leaf: it fills its whole bounds with a solid colour and
// records how many times its Draw ran (so occlusion + pruning can be asserted
// at the "was Draw called" level). It implements Opaque over its full bounds.
type cell struct {
	toolkit.Base
	col   toolkit.RGBA
	draws int
}

func newCell(r Rect, col toolkit.RGBA) *cell {
	c := &cell{col: col}
	c.SetBounds(r)
	return c
}

func (c *cell) Draw(p painter.Painter, _ *toolkit.Theme) {
	c.draws++
	p.FillRect(c.Bounds(), c.col)
}

func (c *cell) OpaqueRect() (Rect, bool) { return c.Bounds(), true }

// ghost is a NON-opaque leaf: it paints a single pixel (so it is not a full
// cover) and never claims opacity. Used to prove a transparent/partial occluder
// culls nothing.
type ghost struct {
	toolkit.Base
	col   toolkit.RGBA
	draws int
}

func newGhost(r Rect, col toolkit.RGBA) *ghost {
	g := &ghost{col: col}
	g.SetBounds(r)
	return g
}

func (g *ghost) Draw(p painter.Painter, _ *toolkit.Theme) {
	g.draws++
	b := g.Bounds()
	p.PutPixel(b.X, b.Y, g.col)
}

// group is a scene-aware pass-through container: DrawSelf fills its own bounds
// with bg (its chrome), and the immediate-mode Draw paints that same chrome and
// then recurses its children in order — so root.Draw is a faithful full-repaint
// baseline that produces the SAME pixels as DrawSelf + the scene's own child
// traversal. children are held as concrete widgets and exposed via Children().
type group struct {
	toolkit.Base
	bg       toolkit.RGBA
	kids     []toolkit.Widget
	selfDraw int
}

func newGroup(r Rect, bg toolkit.RGBA, kids ...toolkit.Widget) *group {
	g := &group{bg: bg, kids: kids}
	g.SetBounds(r)
	return g
}

func (g *group) Children() []toolkit.Widget { return g.kids }

func (g *group) DrawSelf(p painter.Painter, _ *toolkit.Theme) {
	g.selfDraw++
	if g.bg.A != 0 {
		p.FillRect(g.Bounds(), g.bg)
	}
}

func (g *group) Draw(p painter.Painter, th *toolkit.Theme) {
	g.DrawSelf(p, th)
	for _, k := range g.kids {
		if k != nil {
			k.Draw(p, th)
		}
	}
}

// plainBox is a container WITHOUT a SelfDrawer seam, exercising the wholesale
// fallback: the scene must draw it via its ordinary (recursing) Draw.
type plainBox struct {
	toolkit.Base
	kids  []toolkit.Widget
	draws int
}

func newPlainBox(r Rect, kids ...toolkit.Widget) *plainBox {
	b := &plainBox{kids: kids}
	b.SetBounds(r)
	return b
}

func (b *plainBox) Children() []toolkit.Widget { return b.kids }

func (b *plainBox) Draw(p painter.Painter, th *toolkit.Theme) {
	b.draws++
	for _, k := range b.kids {
		if k != nil {
			k.Draw(p, th)
		}
	}
}

// ---------------------------------------------------------------------------
// test painters
// ---------------------------------------------------------------------------

// newBuf makes a zeroed RGBA buffer + PixelPainter of the given size.
func newBuf(w, h int) ([]byte, *painter.PixelPainter) {
	buf := make([]byte, 4*w*h)
	return buf, painter.NewPixelPainter(buf, w, h)
}

// fill paints every byte of buf to v (a sentinel).
func fill(buf []byte, v byte) {
	for i := range buf {
		buf[i] = v
	}
}

// plainPainter implements painter.Painter but NOT painter.Clipper, to exercise
// the "back-end cannot clip" path in Render.
type plainPainter struct{ w, h int }

func (plainPainter) FillRect(Rect, painter.RGBA)                  {}
func (plainPainter) StrokeRect(Rect, painter.RGBA, int)           {}
func (plainPainter) FillRoundRect(Rect, int, painter.RGBA)        {}
func (plainPainter) StrokeRoundRect(Rect, int, painter.RGBA, int) {}
func (plainPainter) PutPixel(int, int, painter.RGBA)              {}
func (plainPainter) Text(int, int, string, painter.RGBA)          {}
func (p plainPainter) Size() (int, int)                           { return p.w, p.h }

var (
	_ painter.Painter = plainPainter{}
	_ toolkit.Widget  = (*cell)(nil)
	_ toolkit.Widget  = (*group)(nil)
	_ Opaque          = (*cell)(nil)
	_ SelfDrawer      = (*group)(nil)
	_ childProvider   = (*group)(nil)
)

var red = toolkit.RGB(0xC0, 0x10, 0x10)
var green = toolkit.RGB(0x10, 0xC0, 0x10)
var blue = toolkit.RGB(0x10, 0x10, 0xC0)
var grey = toolkit.RGB(0x40, 0x40, 0x40)

func th() *toolkit.Theme { return toolkit.DefaultLight() }

// ---------------------------------------------------------------------------
// CORRECTNESS GATE: damage render is pixel-identical to a full repaint
// ---------------------------------------------------------------------------

// buildDense builds a WxH surface root (grey background) holding rows x cols
// opaque cells, plus a handle slice of the cells so a scripted event log can
// mutate them. Returns the root and the flat cell list in row-major order.
func buildDense(surfW, surfH, rows, cols int) (*group, []*cell) {
	cellW, cellH := surfW/cols, surfH/rows
	var cells []*cell
	var rowGroups []toolkit.Widget
	for r := 0; r < rows; r++ {
		var rowKids []toolkit.Widget
		for c := 0; c < cols; c++ {
			cl := newCell(Rect{X: c * cellW, Y: r * cellH, W: cellW - 1, H: cellH - 1}, blue)
			cells = append(cells, cl)
			rowKids = append(rowKids, cl)
		}
		rg := newGroup(Rect{X: 0, Y: r * cellH, W: surfW, H: cellH}, toolkit.RGBA{}, rowKids...)
		rowGroups = append(rowGroups, rg)
	}
	root := newGroup(Rect{X: 0, Y: 0, W: surfW, H: surfH}, grey, rowGroups...)
	return root, cells
}

// fnv1a hashes a buffer (order-sensitive, whole-image) — the "screenshot hash".
func fnv1a(b []byte) uint64 {
	const off = 1469598103934665603
	const prime = 1099511628211
	h := uint64(off)
	for _, c := range b {
		h ^= uint64(c)
		h *= prime
	}
	return h
}

// TestPixelIdentityOverEventLog is the critical correctness gate: over a
// scripted log of appearance changes on a dense scene, the PERSISTED buffer the
// scene updates via damage-only Render must stay byte-identical (screenshot-hash
// equal) to a FRESH full immediate-mode repaint of the same tree.
func TestPixelIdentityOverEventLog(t *testing.T) {
	const W, H, rows, cols = 160, 120, 6, 8
	root, cells := buildDense(W, H, rows, cols)

	// A = persisted, updated by scene damage-render. B = fresh full repaint.
	bufA, pa := newBuf(W, H)
	bufB, pb := newBuf(W, H)

	s := New(root)
	// Initial full frame into A.
	s.Render(pa, th())
	// Initial full immediate repaint into B.
	root.Draw(pb, th())
	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("initial frame: damage-render buffer != full repaint")
	}

	// Scripted event log: (cellIndex, colour). Each step recolours one cell.
	palette := []toolkit.RGBA{red, green, blue, grey, red, green}
	log := []int{0, 7, 47, 47, 23, 11, 0, 46, 47, 30, 15, 8, 8, 40}
	for step, idx := range log {
		c := cells[idx]
		c.col = palette[(step+idx)%len(palette)]
		s.Invalidate(c)
		s.Render(pa, th()) // A: only the damaged cell repainted

		// B: full repaint from scratch (clear + recurse whole tree).
		fill(bufB, 0)
		root.Draw(pb, th())

		if fnv1a(bufA) != fnv1a(bufB) {
			t.Fatalf("step %d (cell %d): damage-render buffer diverged from full repaint", step, idx)
		}
	}
}

// TestPixelIdentityWithOcclusionAndWholesale proves identity ALSO holds when the
// tree contains (a) an opaque occluder over a lower node — the scene skips the
// occludee, the full repaint draws-then-overdraws it, same pixels — and (b) a
// wholesale (non-SelfDrawer) container.
func TestPixelIdentityWithOcclusionAndWholesale(t *testing.T) {
	const W, H = 80, 60
	lower := newCell(Rect{X: 10, Y: 10, W: 30, H: 30}, red)
	cover := newCell(Rect{X: 5, Y: 5, W: 40, H: 40}, green) // fully covers lower
	wholeChild := newCell(Rect{X: 50, Y: 20, W: 20, H: 20}, blue)
	whole := newPlainBox(Rect{X: 50, Y: 20, W: 20, H: 20}, wholeChild)
	// draw order: lower (bottom), cover (on top of lower), whole (separate).
	root := newGroup(Rect{X: 0, Y: 0, W: W, H: H}, grey, lower, cover, whole)

	bufA, pa := newBuf(W, H)
	bufB, pb := newBuf(W, H)
	s := New(root)
	s.Render(pa, th())
	root.Draw(pb, th())
	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("occlusion+wholesale: initial damage-render != full repaint")
	}

	// Recolour the covered lower cell + the wholesale child; identity must hold.
	lower.col = blue
	wholeChild.col = red
	s.Invalidate(lower)
	s.Invalidate(wholeChild)
	s.Render(pa, th())
	fill(bufB, 0)
	root.Draw(pb, th())
	if fnv1a(bufA) != fnv1a(bufB) {
		t.Fatalf("occlusion+wholesale: damage-render diverged from full repaint")
	}
}

// ---------------------------------------------------------------------------
// EXACT damage region = union(old, new)
// ---------------------------------------------------------------------------

func TestDamageExactUnionSamePosition(t *testing.T) {
	c := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 100, H: 100}, grey, c)
	s := New(root)
	_, pa := newBuf(100, 100)
	s.Render(pa, th()) // consume the initial full-surface damage

	// Recolour in place: old == new, damage must be EXACTLY the one cell rect.
	c.col = green
	s.Invalidate(c)
	got := s.damage.Rects()
	if len(got) != 1 || got[0] != (Rect{X: 10, Y: 10, W: 20, H: 20}) {
		t.Fatalf("in-place damage = %v, want exactly [{10 10 20 20}]", got)
	}
}

func TestDamageExactUnionAfterMove(t *testing.T) {
	c := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 200, H: 200}, grey, c)
	s := New(root)
	_, pa := newBuf(200, 200)
	s.Render(pa, th()) // lastRect stamped to {10,10,20,20}

	// Move disjointly: old {10,10,20,20}, new {100,100,20,20}. Damage must be
	// EXACTLY those two rects (disjoint → not coalesced into one).
	old := Rect{X: 10, Y: 10, W: 20, H: 20}
	nw := Rect{X: 100, Y: 100, W: 20, H: 20}
	c.SetBounds(nw)
	s.Invalidate(c)
	got := s.damage.Rects()
	if len(got) != 2 {
		t.Fatalf("moved damage = %v, want exactly 2 disjoint rects", got)
	}
	seen := map[Rect]bool{got[0]: true, got[1]: true}
	if !seen[old] || !seen[nw] {
		t.Fatalf("moved damage = %v, want exactly {%v, %v}", got, old, nw)
	}
}

// ---------------------------------------------------------------------------
// OCCLUSION: fully-covered opaque node's Draw NOT called; partial/transparent IS
// ---------------------------------------------------------------------------

func TestOcclusionOpaqueCoverSkipsDraw(t *testing.T) {
	lower := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, red)
	cover := newCell(Rect{X: 5, Y: 5, W: 40, H: 40}, green) // opaque, fully covers
	root := newGroup(Rect{X: 0, Y: 0, W: 80, H: 80}, grey, lower, cover)
	s := New(root)
	_, pa := newBuf(80, 80)

	lower.draws, cover.draws = 0, 0
	s.Render(pa, th()) // initial full-surface damage covers both

	if lower.draws != 0 {
		t.Fatalf("fully-occluded lower.Draw called %d times, want 0", lower.draws)
	}
	if cover.draws != 1 {
		t.Fatalf("occluder cover.Draw called %d times, want 1", cover.draws)
	}
}

func TestOcclusionPartialAndTransparentDoNotCull(t *testing.T) {
	// Partial opaque cover (does NOT fully contain lower) must NOT cull.
	lower := newCell(Rect{X: 10, Y: 10, W: 40, H: 40}, red)
	partial := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, green) // covers only part
	root := newGroup(Rect{X: 0, Y: 0, W: 80, H: 80}, grey, lower, partial)
	s := New(root)
	_, pa := newBuf(80, 80)
	lower.draws = 0
	s.Render(pa, th())
	if lower.draws != 1 {
		t.Fatalf("partially-covered lower.Draw = %d, want 1 (not culled)", lower.draws)
	}

	// Transparent (non-Opaque) cover over a lower cell must NOT cull.
	low2 := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, red)
	g := newGhost(Rect{X: 0, Y: 0, W: 80, H: 80}, blue) // full-size but NOT Opaque
	root2 := newGroup(Rect{X: 0, Y: 0, W: 80, H: 80}, grey, low2, g)
	s2 := New(root2)
	_, pb := newBuf(80, 80)
	low2.draws = 0
	s2.Render(pb, th())
	if low2.draws != 1 {
		t.Fatalf("cell under transparent sibling: Draw = %d, want 1 (not culled)", low2.draws)
	}
}

// notOpaque is a leaf that IMPLEMENTS Opaque but declines to promise opacity
// (returns false) — the defensive "declared but not opaque" path.
type notOpaque struct {
	toolkit.Base
	draws int
}

func (n *notOpaque) Draw(p painter.Painter, _ *toolkit.Theme) {
	n.draws++
	p.FillRect(n.Bounds(), red)
}
func (n *notOpaque) OpaqueRect() (Rect, bool) { return n.Bounds(), false }

// ---------------------------------------------------------------------------
// BACKGROUND/CHROME occlusion: a container's DrawSelf is skipped when a child
// subtree fully covers the damage, and drawn otherwise. This is the
// optimisation behind the dense-scene speedup, so assert it precisely.
// ---------------------------------------------------------------------------

func TestChromeSkippedWhenCovered(t *testing.T) {
	// A nested tree: root(grey) → mid(transparent) → opaque cell that covers
	// the whole damaged area. The opaque cell is a GRANDCHILD, so coverage must
	// be detected through the subtree, not just direct children.
	cover := newCell(Rect{X: 0, Y: 0, W: 40, H: 40}, blue)
	mid := newGroup(Rect{X: 0, Y: 0, W: 40, H: 40}, toolkit.RGBA{}, cover)
	root := newGroup(Rect{X: 0, Y: 0, W: 40, H: 40}, grey, mid)
	s := New(root)
	_, p := newBuf(40, 40)

	root.selfDraw = 0
	s.Render(p, th()) // initial full-surface damage == the covered area
	if root.selfDraw != 0 {
		t.Fatalf("root chrome DrawSelf ran %d times though a child fully covers it, want 0", root.selfDraw)
	}
	if cover.draws != 1 {
		t.Fatalf("covering cell drawn %d times, want 1", cover.draws)
	}
}

func TestChromeDrawnWhenNotCovered(t *testing.T) {
	// The child does NOT cover the whole surface, so the background chrome must
	// be painted. Also exercises an Opaque widget that returns false.
	small := &notOpaque{}
	small.SetBounds(Rect{X: 0, Y: 0, W: 5, H: 5})
	root := newGroup(Rect{X: 0, Y: 0, W: 40, H: 40}, grey, small)
	s := New(root)
	_, p := newBuf(40, 40)
	root.selfDraw = 0
	s.Render(p, th())
	if root.selfDraw != 1 {
		t.Fatalf("root chrome DrawSelf ran %d times with a non-covering child, want 1", root.selfDraw)
	}
	if small.draws != 1 {
		t.Fatalf("non-opaque child drawn %d times, want 1", small.draws)
	}
}

// ---------------------------------------------------------------------------
// PRECISE BOUNDS: an incremental Render paints ONLY inside the damage region
// ---------------------------------------------------------------------------

func TestRenderPaintsOnlyInsideDamage(t *testing.T) {
	const W, H = 100, 100
	c0 := newCell(Rect{X: 10, Y: 10, W: 20, H: 20}, red)
	c1 := newCell(Rect{X: 60, Y: 60, W: 20, H: 20}, green)
	root := newGroup(Rect{X: 0, Y: 0, W: W, H: H}, grey, c0, c1)
	s := New(root)

	buf, p := newBuf(W, H)
	s.Render(p, th()) // full first frame

	// Re-fill with a sentinel, invalidate ONLY c1, render. Every byte that is
	// no longer the sentinel MUST fall inside the returned damage region.
	const sentinel = 0x7F
	fill(buf, sentinel)
	c1.col = blue
	s.Invalidate(c1)
	region := s.Render(p, th())

	if region.Rects() == nil || len(region.Rects()) != 1 {
		t.Fatalf("expected exactly one damage rect, got %v", region.Rects())
	}
	dmg := region.Rects()[0]
	want := Rect{X: 60, Y: 60, W: 20, H: 20}
	if dmg != want {
		t.Fatalf("damage rect = %v, want %v", dmg, want)
	}
	// Scan the whole surface: any painted (non-sentinel) pixel must be inside dmg.
	painted := 0
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			off := (y*W + x) * 4
			changed := buf[off] != sentinel || buf[off+1] != sentinel ||
				buf[off+2] != sentinel || buf[off+3] != sentinel
			if changed {
				painted++
				if !dmg.Contains(x, y) {
					t.Fatalf("painted pixel (%d,%d) OUTSIDE damage %v", x, y, dmg)
				}
			}
		}
	}
	if painted != dmg.W*dmg.H {
		t.Fatalf("painted %d pixels, want exactly %d (the full damage rect)", painted, dmg.W*dmg.H)
	}
	// c0 (undamaged) must NOT have been repainted this frame.
	if c0.draws != 1 { // 1 from the initial full frame only
		t.Fatalf("undamaged c0.Draw called %d times this incremental frame, want it untouched", c0.draws)
	}
}

// ---------------------------------------------------------------------------
// ZERO allocations in steady-state Render
// ---------------------------------------------------------------------------

func TestRenderZeroAllocSteadyState(t *testing.T) {
	const W, H = 200, 200
	root, cells := buildDense(W, H, 5, 5)
	s := New(root)
	_, p := newBuf(W, H)
	theme := th()      // hoisted: DefaultLight allocates; not part of Render
	s.Render(p, theme) // warm up: grow all backing slices to steady-state size

	toggle := true
	got := testing.AllocsPerRun(200, func() {
		if toggle {
			cells[12].col = red
		} else {
			cells[12].col = green
		}
		toggle = !toggle
		s.Invalidate(cells[12])
		s.Render(p, theme)
	})
	if got != 0 {
		t.Fatalf("steady-state Invalidate+Render allocated %.1f objs/op, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// misc branches: no-damage frame, unknown widget, non-Clipper painter, tree
// structure change, root invalidation
// ---------------------------------------------------------------------------

func TestRenderNoDamageDrawsNothing(t *testing.T) {
	c := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 20, H: 20}, grey, c)
	s := New(root)
	_, p := newBuf(20, 20)
	s.Render(p, th()) // consume initial damage
	c.draws = 0
	region := s.Render(p, th()) // nothing invalidated
	if len(region.Rects()) != 0 {
		t.Fatalf("no-damage frame returned %v, want empty", region.Rects())
	}
	if c.draws != 0 {
		t.Fatalf("no-damage frame drew cell %d times, want 0", c.draws)
	}
}

func TestInvalidateUnknownWidgetIsNoop(t *testing.T) {
	c := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 20, H: 20}, grey, c)
	s := New(root)
	_, p := newBuf(20, 20)
	s.Render(p, th())
	stranger := newCell(Rect{X: 0, Y: 0, W: 5, H: 5}, blue)
	s.Invalidate(stranger) // not in the tree
	if len(s.damage.Rects()) != 0 {
		t.Fatalf("invalidating a stranger produced damage %v, want none", s.damage.Rects())
	}
}

func TestRenderNonClipperPainter(t *testing.T) {
	c := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 20, H: 20}, grey, c)
	s := New(root)
	// A painter that is NOT a Clipper: Render must still draw (unclipped) and
	// return the damage region without panicking.
	region := s.Render(plainPainter{20, 20}, th())
	if region.Bounds() != (Rect{X: 0, Y: 0, W: 20, H: 20}) {
		t.Fatalf("non-clipper initial region = %v, want full surface", region.Bounds())
	}
}

func TestRootInvalidateWalksToNilParent(t *testing.T) {
	c := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 20, H: 20}, grey, c)
	s := New(root)
	_, p := newBuf(20, 20)
	s.Render(p, th())
	s.Invalidate(root) // parentOf(root) == nil → loop terminates
	if !s.Root().Dirty() {
		t.Fatalf("root should be dirty after Invalidate(root)")
	}
	if s.damage.Bounds() != (Rect{X: 0, Y: 0, W: 20, H: 20}) {
		t.Fatalf("root damage = %v, want full surface", s.damage.Bounds())
	}
}

func TestReconcileStructureChanges(t *testing.T) {
	c0 := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	c1 := newCell(Rect{X: 10, Y: 0, W: 10, H: 10}, green)
	root := newGroup(Rect{X: 0, Y: 0, W: 40, H: 20}, grey, c0, c1)
	s := New(root)
	_, p := newBuf(40, 20)
	s.Render(p, th())
	if len(s.Root().Children()) != 2 {
		t.Fatalf("expected 2 children initially, got %d", len(s.Root().Children()))
	}

	// Remove one child (len change → rebuild path).
	root.kids = []toolkit.Widget{c0}
	s.Invalidate(c0)
	s.Render(p, th())
	if len(s.Root().Children()) != 1 {
		t.Fatalf("after removal expected 1 child, got %d", len(s.Root().Children()))
	}

	// Swap the remaining child for a different widget, same length (same-len
	// but different-widget → rebuild path), and include a nil entry (skipped).
	c2 := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, blue)
	root.kids = []toolkit.Widget{c2, nil}
	s.Invalidate(c2)
	s.Render(p, th())
	kids := s.Root().Children()
	if len(kids) != 1 || kids[0].Widget() != toolkit.Widget(c2) {
		t.Fatalf("after swap expected [c2], got %v", kids)
	}

	// Re-adding a previously-seen widget reuses its indexed node (cn != nil).
	root.kids = []toolkit.Widget{c2, c0}
	s.Invalidate(c0)
	s.Render(p, th())
	if len(s.Root().Children()) != 2 {
		t.Fatalf("after re-add expected 2 children, got %d", len(s.Root().Children()))
	}

	// Same LENGTH but a different first widget: the fast-path equality loop
	// must detect the mismatch (break) and fall through to a rebuild.
	c3 := newCell(Rect{X: 20, Y: 0, W: 10, H: 10}, red)
	root.kids = []toolkit.Widget{c3, c0}
	s.Invalidate(c3)
	s.Render(p, th())
	kids2 := s.Root().Children()
	if len(kids2) != 2 || kids2[0].Widget() != toolkit.Widget(c3) {
		t.Fatalf("after same-len swap expected [c3, c0], got %v", kids2)
	}
}

func TestContainerReportingZeroChildren(t *testing.T) {
	c := newCell(Rect{X: 0, Y: 0, W: 10, H: 10}, red)
	root := newGroup(Rect{X: 0, Y: 0, W: 20, H: 20}, grey, c)
	s := New(root)
	_, p := newBuf(20, 20)
	s.Render(p, th())
	root.kids = nil // container now reports zero children
	s.Invalidate(root)
	s.Render(p, th())
	if len(s.Root().Children()) != 0 {
		t.Fatalf("expected 0 children after clearing, got %d", len(s.Root().Children()))
	}
}

// ---------------------------------------------------------------------------
// RegionSet + rectangle-helper unit coverage (exact)
// ---------------------------------------------------------------------------

func TestRegionSetCoalescing(t *testing.T) {
	var rs RegionSet
	rs.Add(Rect{}) // empty ignored
	if len(rs.Rects()) != 0 {
		t.Fatalf("empty Add produced %v", rs.Rects())
	}
	if rs.Bounds() != (Rect{}) {
		t.Fatalf("empty Bounds = %v, want zero", rs.Bounds())
	}
	if rs.Area() != 0 {
		t.Fatalf("empty Area = %d, want 0", rs.Area())
	}

	big := Rect{X: 0, Y: 0, W: 40, H: 40}
	small := Rect{X: 5, Y: 5, W: 10, H: 10} // ⊂ big
	rs.Add(small)
	rs.Add(big) // subsumes small → small removed, only big remains
	if len(rs.Rects()) != 1 || rs.Rects()[0] != big {
		t.Fatalf("subsume: got %v, want [big]", rs.Rects())
	}
	rs.Add(small) // already covered by big → dropped
	if len(rs.Rects()) != 1 {
		t.Fatalf("already-covered Add grew set to %v", rs.Rects())
	}

	disjoint := Rect{X: 100, Y: 100, W: 10, H: 10}
	rs.Add(disjoint)
	if len(rs.Rects()) != 2 {
		t.Fatalf("disjoint Add → %v, want 2 rects", rs.Rects())
	}
	if rs.Area() != 40*40+10*10 {
		t.Fatalf("Area = %d, want %d", rs.Area(), 40*40+100)
	}
	wantB := Rect{X: 0, Y: 0, W: 110, H: 110}
	if rs.Bounds() != wantB {
		t.Fatalf("Bounds = %v, want %v", rs.Bounds(), wantB)
	}
}

func TestRectHelpers(t *testing.T) {
	if !isEmpty(Rect{W: 0, H: 5}) || !isEmpty(Rect{W: 5, H: 0}) {
		t.Fatal("isEmpty misclassified a zero-extent rect")
	}
	if isEmpty(Rect{W: 1, H: 1}) {
		t.Fatal("isEmpty misclassified a 1x1 rect")
	}
	a := Rect{X: 0, Y: 0, W: 10, H: 10}
	b := Rect{X: 5, Y: 5, W: 10, H: 10}
	if !intersects(a, b) {
		t.Fatal("intersects(a,b) = false, want true")
	}
	if intersects(a, Rect{X: 100, Y: 0, W: 10, H: 10}) {
		t.Fatal("intersects with disjoint = true")
	}
	if intersects(a, Rect{}) {
		t.Fatal("intersects with empty = true")
	}
	if got := rectIntersect(a, b); got != (Rect{X: 5, Y: 5, W: 5, H: 5}) {
		t.Fatalf("rectIntersect = %v, want {5 5 5 5}", got)
	}
	if got := rectIntersect(a, Rect{X: 100, Y: 0, W: 1, H: 1}); got != (Rect{}) {
		t.Fatalf("disjoint rectIntersect = %v, want zero", got)
	}
	if got := union(a, Rect{}); got != a {
		t.Fatalf("union(a, empty) = %v, want a", got)
	}
	if got := union(Rect{}, b); got != b {
		t.Fatalf("union(empty, b) = %v, want b", got)
	}
	if got := union(a, b); got != (Rect{X: 0, Y: 0, W: 15, H: 15}) {
		t.Fatalf("union(a,b) = %v, want {0 0 15 15}", got)
	}
	if !contains(a, Rect{}) {
		t.Fatal("contains(a, empty) = false, want true")
	}
	if contains(Rect{}, Rect{X: 0, Y: 0, W: 1, H: 1}) {
		t.Fatal("contains(empty, r) = true, want false")
	}
	if !contains(a, Rect{X: 2, Y: 2, W: 3, H: 3}) {
		t.Fatal("contains(a, inner) = false, want true")
	}
	if contains(a, b) {
		t.Fatal("contains(a, overlapping-but-outside) = true, want false")
	}
}

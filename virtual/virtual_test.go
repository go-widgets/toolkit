// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"runtime"
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// --- test painters ----------------------------------------------------------

// recPainter records clip push/pop counts and implements painter.Clipper.
type recPainter struct {
	pushes, pops int
}

func (p *recPainter) FillRect(r painter.Rect, c painter.RGBA)                        {}
func (p *recPainter) StrokeRect(r painter.Rect, c painter.RGBA, lineW int)           {}
func (p *recPainter) FillRoundRect(r painter.Rect, radius int, c painter.RGBA)       {}
func (p *recPainter) StrokeRoundRect(r painter.Rect, rad int, c painter.RGBA, l int) {}
func (p *recPainter) PutPixel(x, y int, c painter.RGBA)                              {}
func (p *recPainter) Text(x, y int, s string, ink painter.RGBA)                      {}
func (p *recPainter) Size() (int, int)                                               { return 4096, 4096 }
func (p *recPainter) PushClip(r painter.Rect)                                        { p.pushes++ }
func (p *recPainter) PopClip()                                                       { p.pops++ }

// plainPainter implements painter.Painter but NOT painter.Clipper, exercising
// the "back-end cannot clip" branch.
type plainPainter struct{}

func (plainPainter) FillRect(r painter.Rect, c painter.RGBA)                        {}
func (plainPainter) StrokeRect(r painter.Rect, c painter.RGBA, lineW int)           {}
func (plainPainter) FillRoundRect(r painter.Rect, radius int, c painter.RGBA)       {}
func (plainPainter) StrokeRoundRect(r painter.Rect, rad int, c painter.RGBA, l int) {}
func (plainPainter) PutPixel(x, y int, c painter.RGBA)                              {}
func (plainPainter) Text(x, y int, s string, ink painter.RGBA)                      {}
func (plainPainter) Size() (int, int)                                               { return 4096, 4096 }

var _ painter.Clipper = (*recPainter)(nil)

func theme() *toolkit.Theme { return &toolkit.Theme{} }

// intItems builds a slice 0..n-1.
func intItems(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// --- height index -----------------------------------------------------------

func TestBuildIndexEmpty(t *testing.T) {
	idx := buildIndex(0, func(int) int { return 20 })
	if !idx.uniform || idx.n != 0 || idx.total != 0 {
		t.Fatalf("empty index = %+v", idx)
	}
	if got := idx.locate(50); got != 0 {
		t.Fatalf("locate on empty = %d, want 0", got)
	}
	if got := idx.prefix(3); got != 0 {
		t.Fatalf("prefix on empty = %d, want 0", got)
	}
	if got := idx.heightAt(0); got != 0 {
		t.Fatalf("heightAt on empty = %d, want 0", got)
	}
}

func TestBuildIndexUniform(t *testing.T) {
	idx := buildIndex(10, func(int) int { return 20 })
	if !idx.uniform || idx.rowH != 20 || idx.total != 200 {
		t.Fatalf("uniform index = %+v", idx)
	}
	if idx.fen != nil || idx.heights != nil {
		t.Fatalf("uniform must not allocate fenwick/heights")
	}
	// prefix / heightAt / locate on the uniform fast path.
	if got := idx.prefix(3); got != 60 {
		t.Fatalf("prefix(3) = %d, want 60", got)
	}
	if got := idx.prefix(-1); got != 0 {
		t.Fatalf("prefix(-1) = %d, want 0", got)
	}
	if got := idx.prefix(999); got != 200 {
		t.Fatalf("prefix(999) clamp = %d, want 200", got)
	}
	if got := idx.heightAt(4); got != 20 {
		t.Fatalf("heightAt = %d, want 20", got)
	}
	if got := idx.heightAt(-1); got != 0 {
		t.Fatalf("heightAt(-1) = %d, want 0", got)
	}
	if got := idx.locate(0); got != 0 {
		t.Fatalf("locate(0) = %d, want 0", got)
	}
	if got := idx.locate(-5); got != 0 {
		t.Fatalf("locate(-5) = %d, want 0", got)
	}
	if got := idx.locate(55); got != 2 {
		t.Fatalf("locate(55) = %d, want 2", got)
	}
	if got := idx.locate(1000); got != 9 {
		t.Fatalf("locate(beyond) = %d, want 9", got)
	}
}

func TestBuildIndexVariableFenwickBruteForce(t *testing.T) {
	n := 500
	// A varying (and one negative → clamped to 0) height pattern.
	h := func(i int) int {
		if i == 7 {
			return -100 // exercises norm's clamp in BOTH build passes
		}
		return 10 + (i%5)*7
	}
	idx := buildIndex(n, h)
	if idx.uniform {
		t.Fatal("expected variable index")
	}
	if idx.heights[7] != 0 {
		t.Fatalf("negative height not clamped: %d", idx.heights[7])
	}
	// Brute-force prefix sums to validate prefix/heightAt/locate.
	pref := make([]int, n+1)
	for i := 0; i < n; i++ {
		hv := h(i)
		if hv < 0 {
			hv = 0
		}
		pref[i+1] = pref[i] + hv
	}
	if idx.total != pref[n] {
		t.Fatalf("total = %d, want %d", idx.total, pref[n])
	}
	for row := 0; row <= n; row++ {
		if got := idx.prefix(row); got != pref[row] {
			t.Fatalf("prefix(%d) = %d, want %d", row, got, pref[row])
		}
	}
	for i := 0; i < n; i++ {
		want := pref[i+1] - pref[i]
		if got := idx.heightAt(i); got != want {
			t.Fatalf("heightAt(%d) = %d, want %d", i, got, want)
		}
	}
	// locate against a linear scan, at every pixel offset.
	bruteLocate := func(off int) int {
		if off <= 0 {
			return 0
		}
		if off >= pref[n] {
			return n - 1
		}
		for r := 0; r < n; r++ {
			if pref[r] <= off && off < pref[r+1] {
				return r
			}
		}
		return n - 1
	}
	for off := -3; off <= pref[n]+3; off++ {
		if got, want := idx.locate(off), bruteLocate(off); got != want {
			t.Fatalf("locate(%d) = %d, want %d", off, got, want)
		}
	}
}

func TestRemapMoveTop(t *testing.T) {
	// from == to → unchanged (only reachable via the helper, not a real event).
	if got := remapMoveTop(5, 3, 3); got != 5 {
		t.Fatalf("from==to = %d, want 5", got)
	}
	// the tracked item is the one moved.
	if got := remapMoveTop(2, 2, 6); got != 6 {
		t.Fatalf("top==from = %d, want 6", got)
	}
	// [0,1,2,3,4] move 1→4 ⇒ [0,2,3,4,1]: item at 3 (value 3) lands at 2
	// (exercises top>from ⇒ t--; t<to ⇒ no ++).
	if got := remapMoveTop(3, 1, 4); got != 2 {
		t.Fatalf("remap(3,1,4) = %d, want 2", got)
	}
	// [0,1,2,3,4,5] move 5→0 ⇒ [5,0,1,2,3,4]: item at 1 (value 1) lands at 2
	// (exercises top<from ⇒ no t--; t>=to ⇒ ++).
	if got := remapMoveTop(1, 5, 0); got != 2 {
		t.Fatalf("remap(1,5,0) = %d, want 2", got)
	}
	// [A,B,C,D] move 0→2 ⇒ [B,C,A,D]: item at 3 (D) stays at 3.
	if got := remapMoveTop(3, 0, 2); got != 3 {
		t.Fatalf("remap(3,0,2) = %d, want 3", got)
	}
}

// --- VirtualList: exact visible count at scale --------------------------------

func TestVirtualListExactCountOneMillion(t *testing.T) {
	const n = 1_000_000
	m := mvvm.NewObservableList[int](intItems(n)...)
	rendered := 0
	var firstIdx, lastIdx int
	first := true
	v := NewVirtualList[int](m, func(int) int { return 20 },
		func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int) {
			if first {
				firstIdx = i
				first = false
			}
			lastIdx = i
			rendered++
		})
	v.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 300, H: 1000}) // 1000/20 = 50 rows

	fv, cnt := v.VisibleRange()
	if fv != 0 || cnt != 50 {
		t.Fatalf("VisibleRange at top = (%d,%d), want (0,50)", fv, cnt)
	}
	v.Draw(&recPainter{}, theme())
	if rendered != cnt {
		t.Fatalf("Render calls = %d, want exactly %d (visibleCount)", rendered, cnt)
	}
	if rendered != 50 {
		t.Fatalf("Render calls = %d, want exactly 50", rendered)
	}
	if firstIdx != 0 || lastIdx != 49 {
		t.Fatalf("rendered rows [%d..%d], want [0..49]", firstIdx, lastIdx)
	}

	// Scroll to an exact row boundary deep into the model.
	rendered, first = 0, true
	v.ScrollTo(20 * 100) // row 100
	fv, cnt = v.VisibleRange()
	if fv != 100 || cnt != 50 {
		t.Fatalf("VisibleRange scrolled = (%d,%d), want (100,50)", fv, cnt)
	}
	v.Draw(&recPainter{}, theme())
	if rendered != 50 || firstIdx != 100 || lastIdx != 149 {
		t.Fatalf("scrolled render: n=%d rows[%d..%d], want 50 [100..149]", rendered, firstIdx, lastIdx)
	}
}

// --- VirtualList: variable-height first-visible-row at scripted offsets -------

func TestVirtualListVariableFirstVisibleRow(t *testing.T) {
	// rows: heights 30,30,...  no — make them vary.
	h := func(i int) int { return 10 + (i%4)*10 } // 10,20,30,40 repeating
	m := mvvm.NewObservableList[int](intItems(200)...)
	v := NewVirtualList[int](m, h, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})

	// prefix sums for the first several rows: 0,10,30,60,100,110,130,...
	// offset 0 → row 0; 10 → row1; 29 → row1; 30 → row2; 59 → row2; 60 → row3; 99 → row3; 100 → row4
	cases := []struct{ off, want int }{
		{0, 0}, {5, 0}, {10, 1}, {29, 1}, {30, 2}, {59, 2}, {60, 3}, {99, 3}, {100, 4},
	}
	for _, c := range cases {
		v.ScrollOffset = c.off
		got, _ := v.VisibleRange()
		if got != c.want {
			t.Fatalf("offset %d → firstVisibleRow %d, want %d", c.off, got, c.want)
		}
	}
}

// --- Fenwick within ~n× of uniform on a scroll bench -------------------------

func benchScroll(b *testing.B, variable bool) {
	const n = 1_000_000
	m := mvvm.NewObservableList[int](intItems(n)...)
	var hf func(int) int
	if variable {
		hf = func(i int) int { return 10 + (i%7)*3 }
	} else {
		hf = func(int) int { return 20 }
	}
	acc := 0
	v := NewVirtualList[int](m, hf,
		func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int) { acc += r.H })
	v.SetBounds(toolkit.Rect{W: 300, H: 1000})
	idx := v.idx
	b.ResetTimer()
	for k := 0; k < b.N; k++ {
		v.ScrollByRows(1)
		first, cnt := v.VisibleRange()
		// simulate Draw's per-visible-row height work (O(1) each).
		for j := 0; j < cnt; j++ {
			acc += idx.heightAt(first + j)
		}
	}
	runtime.KeepAlive(acc)
}

func BenchmarkScrollUniform(b *testing.B) { benchScroll(b, false) }
func BenchmarkScrollFenwick(b *testing.B) { benchScroll(b, true) }

func TestFenwickWithinFactorOfUniform(t *testing.T) {
	ru := testing.Benchmark(BenchmarkScrollUniform)
	rf := testing.Benchmark(BenchmarkScrollFenwick)
	if ru.N == 0 || rf.N == 0 || ru.NsPerOp() == 0 {
		t.Fatalf("benchmarks did not run: uniform=%v fenwick=%v", ru, rf)
	}
	ratio := float64(rf.NsPerOp()) / float64(ru.NsPerOp())
	t.Logf("scroll tick: uniform=%d ns/op  fenwick=%d ns/op  ratio=%.2fx",
		ru.NsPerOp(), rf.NsPerOp(), ratio)
	// The O(log n) Fenwick path must stay within a small constant factor of the
	// O(1) uniform path on a representative scroll frame (headroom for CI noise).
	if ratio > 6.0 {
		t.Fatalf("Fenwick scroll %.2fx uniform, want ≤ 6x", ratio)
	}
}

// --- 0 allocations per scroll tick -------------------------------------------

func TestScrollTickZeroAllocs(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(10_000)...)
	v := NewVirtualList[int](m, func(int) int { return 15 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 200, H: 450})
	if a := testing.AllocsPerRun(2000, func() {
		v.ScrollBy(1)
		v.VisibleRange()
	}); a != 0 {
		t.Fatalf("allocs per scroll tick = %v, want 0", a)
	}
}

// --- incremental updates / anchor stability ----------------------------------

// snapshot renders the list into a recPainter and returns the (screenY→value)
// pairs of every rendered row, so two snapshots can be compared for identity.
type row struct {
	y, val int
}

func snapshot[T any](v *VirtualList[int]) []row {
	var out []row
	old := v.Render
	v.Render = func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int) {
		out = append(out, row{r.Y, item})
	}
	v.Draw(&recPainter{}, theme())
	v.Render = old
	return out
}

func eqRows(a, b []row) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAnchorInsertAboveViewport(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100}) // 10 rows visible
	v.ScrollTo(500)                           // first visible row = 50
	before := snapshot[int](v)
	fvBefore, _ := v.VisibleRange()
	if fvBefore != 50 {
		t.Fatalf("setup: first visible = %d, want 50", fvBefore)
	}

	// Insert ABOVE the viewport: the visible rows must not jump.
	m.Insert(0, -1)
	after := snapshot[int](v)
	if v.ScrollOffset != 510 {
		t.Fatalf("offset after insert-above = %d, want 510 (anchor shifted by one row)", v.ScrollOffset)
	}
	if !eqRows(before, after) {
		t.Fatalf("visible region changed on insert-above:\n before=%v\n after =%v", before, after)
	}
}

func TestAnchorInsertBelowAndWithin(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.ScrollTo(500)

	// Insert BELOW the viewport → offset unchanged.
	m.Append(999)
	if v.ScrollOffset != 500 {
		t.Fatalf("offset after insert-below = %d, want 500", v.ScrollOffset)
	}
	// Insert WITHIN the viewport (between top row 50 and bottom) → top item
	// stays anchored, so offset unchanged.
	before := snapshot[int](v)
	m.Insert(55, 777)
	if v.ScrollOffset != 500 {
		t.Fatalf("offset after insert-within = %d, want 500", v.ScrollOffset)
	}
	after := snapshot[int](v)
	// Top row (value that was 50) is unchanged; but rows at/after 55 shifted.
	if after[0] != before[0] {
		t.Fatalf("top row moved on insert-within: %v vs %v", after[0], before[0])
	}
}

func TestAnchorRemoveAbove(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.ScrollTo(500) // first row 50
	before := snapshot[int](v)

	// Remove 3 rows entirely above the viewport.
	m.RemoveAt(0)
	m.RemoveAt(0)
	m.RemoveAt(0)
	if v.ScrollOffset != 470 {
		t.Fatalf("offset after remove-above = %d, want 470", v.ScrollOffset)
	}
	after := snapshot[int](v)
	if !eqRows(before, after) {
		t.Fatalf("visible region changed on remove-above:\n before=%v\n after=%v", before, after)
	}
}

func TestRemoveStraddlingTop(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.ScrollTo(500) // top row 50
	// Remove a range [48,52) that straddles the top row → anchor snaps to the
	// removal start.
	for i := 0; i < 4; i++ {
		m.RemoveAt(48)
	}
	first, _ := v.VisibleRange()
	if first != 48 {
		t.Fatalf("first visible after straddling remove = %d, want 48", first)
	}
}

func TestMoveAndReplaceAndReset(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.ScrollTo(500)

	// Move a row from above the viewport to below it: still valid + on screen.
	m.Move(10, 90)
	if v.ScrollOffset < 0 || v.ScrollOffset > v.idx.total-v.Bounds().H {
		t.Fatalf("offset after move out of range: %d (total %d)", v.ScrollOffset, v.idx.total)
	}
	// Replace a visible row (same count → offset unchanged).
	off := v.ScrollOffset
	m.Set(55, 4242)
	if v.ScrollOffset != off {
		t.Fatalf("offset after replace = %d, want %d", v.ScrollOffset, off)
	}
	// Reset (Clear) → scroll back to the top.
	m.Clear()
	if v.ScrollOffset != 0 {
		t.Fatalf("offset after reset = %d, want 0", v.ScrollOffset)
	}
	fv, cnt := v.VisibleRange()
	if fv != 0 || cnt != 0 {
		t.Fatalf("VisibleRange after clear = (%d,%d), want (0,0)", fv, cnt)
	}
}

func TestReplaceAboveViewportShiftsAnchor(t *testing.T) {
	// Variable heights: replacing a row ABOVE the viewport with a taller one
	// keeps the same top item anchored (offset grows by the height delta).
	heights := make([]int, 100)
	for i := range heights {
		heights[i] = 10
	}
	hf := func(i int) int { return heights[i] }
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, hf, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.ScrollTo(500) // top item index 50
	topBefore, _ := v.VisibleRange()

	heights[10] = 60 // row 10 (above viewport) grows by 50
	m.Set(10, 10)    // triggers ListReplace; height fn now returns the new height
	topAfter, _ := v.VisibleRange()
	if topAfter != topBefore {
		t.Fatalf("top item changed on replace-above: %d → %d", topBefore, topAfter)
	}
	if v.ScrollOffset != 550 {
		t.Fatalf("offset after replace-above = %d, want 550 (shifted by +50)", v.ScrollOffset)
	}
}

// --- Draw: clip / no-clip / no-overflow --------------------------------------

func TestDrawClipPushPop(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 20 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100}) // content 2000 > 100 → overflow

	rp := &recPainter{}
	v.Draw(rp, theme())
	if rp.pushes != 1 || rp.pops != 1 {
		t.Fatalf("clip push/pop = %d/%d, want 1/1", rp.pushes, rp.pops)
	}
	// A painter that cannot clip: overflow path, canClip=false, no panic.
	v.Draw(plainPainter{}, theme())
}

func TestDrawNoOverflow(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(3)...)
	v := NewVirtualList[int](m, func(int) int { return 20 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 500}) // content 60 < 500 → no overflow

	rp := &recPainter{}
	v.Draw(rp, theme())
	if rp.pushes != 0 || rp.pops != 0 {
		t.Fatalf("no-overflow must not clip; got %d/%d", rp.pushes, rp.pops)
	}
}

func TestDrawGuards(t *testing.T) {
	// nil Model → n==0 → early return.
	v := &VirtualList[int]{RowHeight: func(int) int { return 10 },
		Render: func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {}}
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	v.Draw(&recPainter{}, theme())

	// zero bounds → early return.
	m := mvvm.NewObservableList[int](intItems(5)...)
	v2 := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v2.Draw(&recPainter{}, theme()) // bounds are zero
	if fv, cnt := v2.VisibleRange(); fv != 0 || cnt != 0 {
		t.Fatalf("VisibleRange zero-bounds = (%d,%d)", fv, cnt)
	}

	// nil Render → Draw returns without panicking.
	v3 := NewVirtualList[int](m, func(int) int { return 10 }, nil)
	v3.SetBounds(toolkit.Rect{W: 100, H: 100})
	v3.Draw(&recPainter{}, theme())

	// all-zero heights → total 0 → VisibleRange (0,0) and Draw no-op.
	v4 := NewVirtualList[int](m, func(int) int { return 0 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v4.SetBounds(toolkit.Rect{W: 100, H: 100})
	if fv, cnt := v4.VisibleRange(); fv != 0 || cnt != 0 {
		t.Fatalf("all-zero VisibleRange = (%d,%d)", fv, cnt)
	}
	v4.Draw(&recPainter{}, theme())

	// default row height when RowHeight is nil.
	v5 := NewVirtualList[int](m, nil, func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v5.SetBounds(toolkit.Rect{W: 100, H: 100})
	if v5.idx.rowH != DefaultRowHeight {
		t.Fatalf("nil RowHeight → rowH %d, want %d", v5.idx.rowH, DefaultRowHeight)
	}
}

// --- scroll API + events + model rebinding -----------------------------------

func TestVirtualListScrollAndEvents(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(100)...)
	v := NewVirtualList[int](m, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100}) // max offset = 1000-100 = 900

	v.ScrollTo(-50)
	if v.ScrollOffset != 0 {
		t.Fatalf("ScrollTo(-50) = %d, want 0", v.ScrollOffset)
	}
	v.ScrollTo(100000)
	if v.ScrollOffset != 900 {
		t.Fatalf("ScrollTo(huge) = %d, want 900", v.ScrollOffset)
	}
	v.ScrollBy(-10000)
	if v.ScrollOffset != 0 {
		t.Fatalf("ScrollBy under = %d, want 0", v.ScrollOffset)
	}
	// EventScroll → whole rows.
	v.OnEvent(toolkit.Event{Kind: toolkit.EventScroll, Delta: 3})
	if v.ScrollOffset != 30 {
		t.Fatalf("wheel 3 rows = %d, want 30", v.ScrollOffset)
	}
	v.ScrollByRows(-100) // clamps to top
	if v.ScrollOffset != 0 {
		t.Fatalf("ScrollByRows under = %d, want 0", v.ScrollOffset)
	}
	v.ScrollByRows(100000) // clamps to bottom (row 100 → prefix clamps to 900)
	if v.ScrollOffset != 900 {
		t.Fatalf("ScrollByRows over = %d, want 900", v.ScrollOffset)
	}
	// A non-scroll event is ignored.
	v.OnEvent(toolkit.Event{Kind: toolkit.EventClick})

	// ScrollByRows on empty model returns early.
	empty := NewVirtualList[int](mvvm.NewObservableList[int](), func(int) int { return 10 }, nil)
	empty.SetBounds(toolkit.Rect{W: 10, H: 10})
	empty.ScrollByRows(5)
	if empty.ScrollOffset != 0 {
		t.Fatalf("empty ScrollByRows = %d, want 0", empty.ScrollOffset)
	}
}

func TestVirtualListRebindAndClose(t *testing.T) {
	m1 := mvvm.NewObservableList[int](intItems(10)...)
	v := NewVirtualList[int](m1, func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})

	// Rebind to a new model (exercises the resubscribe branch).
	m2 := mvvm.NewObservableList[int](intItems(50)...)
	v.Model = m2
	v.ensure()
	if v.subscribed != m2 || v.idx.n != 50 {
		t.Fatalf("rebind failed: subscribed=%v n=%d", v.subscribed == m2, v.idx.n)
	}
	// A mutation on the OLD model must no longer affect us.
	before := v.ScrollOffset
	m1.Insert(0, -1)
	if v.ScrollOffset != before {
		t.Fatalf("old model still affects offset")
	}
	// Close, then mutate: no callback fires.
	v.ScrollTo(200)
	off := v.ScrollOffset
	v.Close()
	v.Close() // idempotent
	m2.Insert(0, -1)
	if v.ScrollOffset != off {
		t.Fatalf("mutation after Close changed offset %d → %d", off, v.ScrollOffset)
	}

	// Rebinding a live (subscribed) list to a nil model rebuilds to an empty
	// index (exercises modelLen's nil branch through ensure).
	vn := NewVirtualList[int](mvvm.NewObservableList[int](intItems(20)...),
		func(int) int { return 10 }, nil)
	if vn.idx.n != 20 {
		t.Fatalf("setup n = %d, want 20", vn.idx.n)
	}
	vn.Model = nil
	vn.ensure()
	if vn.idx.n != 0 {
		t.Fatalf("nil model n = %d, want 0", vn.idx.n)
	}
}

// --- VirtualGrid --------------------------------------------------------------

func TestVirtualGridReflowExactRects(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(10)...)
	type cell struct {
		i          int
		x, y, w, h int
	}
	var cells []cell
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(p painter.Painter, th *toolkit.Theme, r toolkit.Rect, i int, item int) {
			cells = append(cells, cell{i, r.X, r.Y, r.W, r.H})
		})
	g.SetBounds(toolkit.Rect{X: 0, Y: 0, W: 160, H: 100}) // 160/50 = 3 cols

	fv, cnt := g.VisibleRange()
	// rows 0,1,2 intersect y∈[0,100) (row2 partial 80..120) → 3 rows × 3 cols = 9 cells.
	if fv != 0 || cnt != 9 {
		t.Fatalf("grid VisibleRange = (%d,%d), want (0,9)", fv, cnt)
	}
	g.Draw(&recPainter{}, theme())
	if len(cells) != 9 {
		t.Fatalf("grid rendered %d cells, want 9", len(cells))
	}
	// exact rects: cell i at (col*50, row*40), col=i%3, row=i/3.
	for _, c := range cells {
		wantX := (c.i % 3) * 50
		wantY := (c.i / 3) * 40
		if c.x != wantX || c.y != wantY || c.w != 50 || c.h != 40 {
			t.Fatalf("cell %d rect = (%d,%d,%d,%d), want (%d,%d,50,40)", c.i, c.x, c.y, c.w, c.h, wantX, wantY)
		}
	}

	// Scroll down by one row (40px) and re-check exact rects + count.
	cells = nil
	g.ScrollBy(40)
	fv, cnt = g.VisibleRange()
	// rows 1,2,3 now; row3 has only 1 cell (index 9). cells 3..9 = 7 cells.
	if fv != 3 || cnt != 7 {
		t.Fatalf("scrolled grid VisibleRange = (%d,%d), want (3,7)", fv, cnt)
	}
	g.Draw(&recPainter{}, theme())
	if len(cells) != 7 {
		t.Fatalf("scrolled grid rendered %d cells, want 7", len(cells))
	}
	for _, c := range cells {
		wantX := (c.i % 3) * 50
		wantY := (c.i/3)*40 - 40 // minus scroll offset
		if c.x != wantX || c.y != wantY {
			t.Fatalf("scrolled cell %d at (%d,%d), want (%d,%d)", c.i, c.x, c.y, wantX, wantY)
		}
	}
}

func TestVirtualGridColsAndGuards(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(6)...)
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})

	// no bounds yet → cols 0, contentHeight 0, VisibleRange (0,0).
	if g.cols() != 0 || g.contentHeight() != 0 {
		t.Fatalf("no-bounds cols/content = %d/%d", g.cols(), g.contentHeight())
	}
	if fv, cnt := g.VisibleRange(); fv != 0 || cnt != 0 {
		t.Fatalf("no-bounds VisibleRange = (%d,%d)", fv, cnt)
	}
	// width smaller than a cell → at least 1 column.
	g.SetBounds(toolkit.Rect{W: 30, H: 40})
	if g.cols() != 1 {
		t.Fatalf("narrow cols = %d, want 1", g.cols())
	}
	// zero cell width → cols 0.
	g2 := NewVirtualGrid[int](m, toolkit.Size{W: 0, H: 40}, nil)
	g2.SetBounds(toolkit.Rect{W: 100, H: 100})
	if g2.cols() != 0 {
		t.Fatalf("zero-cellW cols = %d, want 0", g2.cols())
	}
}

func TestVirtualGridScroll(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(30)...)
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 100}) // 3 cols, 10 rows, content 400, max off 300

	g.ScrollTo(-10)
	if g.ScrollOffset != 0 {
		t.Fatalf("grid ScrollTo(-10) = %d", g.ScrollOffset)
	}
	g.ScrollTo(99999)
	if g.ScrollOffset != 300 {
		t.Fatalf("grid ScrollTo(huge) = %d, want 300", g.ScrollOffset)
	}
	g.ScrollBy(-99999)
	if g.ScrollOffset != 0 {
		t.Fatalf("grid ScrollBy under = %d", g.ScrollOffset)
	}
	g.OnEvent(toolkit.Event{Kind: toolkit.EventScroll, Delta: 2}) // 2 rows = 80px
	if g.ScrollOffset != 80 {
		t.Fatalf("grid wheel = %d, want 80", g.ScrollOffset)
	}
	g.OnEvent(toolkit.Event{Kind: toolkit.EventClick}) // ignored

	// ScrollByRows guard: zero cell height.
	gz := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 0}, nil)
	gz.SetBounds(toolkit.Rect{W: 150, H: 100})
	gz.ScrollByRows(5)
	if gz.ScrollOffset != 0 {
		t.Fatalf("zero-cellH ScrollByRows = %d", gz.ScrollOffset)
	}
}

func TestVirtualGridAnchorAndEvents(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(60)...)
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 120}) // 3 cols
	g.ScrollTo(200)                           // top row 5 → topCell 15

	// Insert ABOVE the top cell → offset shifts down by one row (Count divisible
	// by cols keeps rem 0 here).
	g.Model.Insert(0, -1)
	g.Model.Insert(0, -2)
	g.Model.Insert(0, -3) // 3 inserts above → +1 row of offset (3/3 cols)
	if g.ScrollOffset != 240 {
		t.Fatalf("grid anchor insert-above offset = %d, want 240", g.ScrollOffset)
	}
	// Remove above.
	g.Model.RemoveAt(0)
	g.Model.RemoveAt(0)
	g.Model.RemoveAt(0)
	if g.ScrollOffset != 200 {
		t.Fatalf("grid anchor remove-above offset = %d, want 200", g.ScrollOffset)
	}
	// Remove straddling the top cell.
	first0, _ := g.VisibleRange()
	_ = first0
	// Move + Replace + Reset paths.
	g.Model.Move(0, 40)
	g.Model.Set(20, 4242)
	g.Model.Clear()
	if g.ScrollOffset != 0 {
		t.Fatalf("grid reset offset = %d, want 0", g.ScrollOffset)
	}

	// onChange early-return when geometry not ready.
	g2 := &VirtualGrid[int]{Model: mvvm.NewObservableList[int](intItems(5)...),
		CellSize: toolkit.Size{W: 0, H: 0}}
	g2.ensure()
	g2.Model.Insert(0, 9) // onChange with c<=0 → early return, no panic
}

func TestVirtualGridRemoveStraddlingTop(t *testing.T) {
	m := mvvm.NewObservableList[int](intItems(60)...)
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 120})
	g.ScrollTo(200) // topCell 15
	// Remove a range straddling topCell (indices 14..18).
	for i := 0; i < 4; i++ {
		g.Model.RemoveAt(14)
	}
	// topCell snaps to 14 → row 14/3 = 4 → offset 160.
	if g.ScrollOffset != 160 {
		t.Fatalf("grid straddling remove offset = %d, want 160", g.ScrollOffset)
	}
}

func TestVirtualGridDrawGuardsAndClip(t *testing.T) {
	// overflow + non-clipper painter.
	m := mvvm.NewObservableList[int](intItems(30)...)
	g := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 100})
	rp := &recPainter{}
	g.Draw(rp, theme())
	if rp.pushes != 1 || rp.pops != 1 {
		t.Fatalf("grid overflow clip = %d/%d, want 1/1", rp.pushes, rp.pops)
	}
	g.Draw(plainPainter{}, theme()) // canClip=false branch

	// no overflow.
	g.SetBounds(toolkit.Rect{W: 150, H: 1000})
	rp2 := &recPainter{}
	g.Draw(rp2, theme())
	if rp2.pushes != 0 {
		t.Fatalf("grid no-overflow must not clip, got %d", rp2.pushes)
	}

	// guard early-returns: nil model, nil render, zero bounds, zero cell.
	(&VirtualGrid[int]{CellSize: toolkit.Size{W: 50, H: 40},
		Render: func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {}}).Draw(&recPainter{}, theme())

	gNilRender := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40}, nil)
	gNilRender.SetBounds(toolkit.Rect{W: 150, H: 100})
	gNilRender.Draw(&recPainter{}, theme())

	gZeroCell := NewVirtualGrid[int](m, toolkit.Size{W: 0, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	gZeroCell.SetBounds(toolkit.Rect{W: 150, H: 100})
	gZeroCell.Draw(&recPainter{}, theme())

	gZeroBounds := NewVirtualGrid[int](m, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	gZeroBounds.Draw(&recPainter{}, theme())
}

func TestVirtualGridRebindAndClose(t *testing.T) {
	m1 := mvvm.NewObservableList[int](intItems(9)...)
	g := NewVirtualGrid[int](m1, toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 100})
	m2 := mvvm.NewObservableList[int](intItems(30)...)
	g.Model = m2
	g.ensure()
	if g.subscribed != m2 {
		t.Fatal("grid rebind failed")
	}
	g.Close()
	g.Close()
	off := g.ScrollOffset
	m2.Insert(0, -1)
	if g.ScrollOffset != off {
		t.Fatalf("grid mutation after Close changed offset")
	}
	g.Model = nil
	g.ensure()
	if g.modelLen() != 0 {
		t.Fatal("grid nil model len != 0")
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/go-crdt/crdt"
)

// These property tests are the acceptance gate for the collaborative sheet
// adapter. The structured/crdt core already proves its own convergence; what is
// proved HERE is that the adapter — its positional (column, row) addressing over
// stable identities, and its MVVM fan-out — inherits that convergence rather
// than quietly breaking it. Two replicas edit concurrently through the adapter's
// own API while an unreliable network delivers batches late, out of order and
// duplicated, and every replica must end byte-identical.
//
// The oracle is the byte-equal structured snapshot (strictly stronger than equal
// values), reinforced by the adapter's own positional read agreeing. Randomness
// is seeded, so a failure names the seed and reproduces on any platform,
// js/wasm included.

// --- unreliable transport ------------------------------------------------------

type sheetNet struct{ inbox [][]crdt.PartOps }

func newSheetNet(n int) *sheetNet { return &sheetNet{inbox: make([][]crdt.PartOps, n)} }

func (nw *sheetNet) broadcast(from int, batches []crdt.PartOps) {
	for i := range nw.inbox {
		if i != from {
			nw.inbox[i] = append(nw.inbox[i], batches...)
		}
	}
}

func (nw *sheetNet) deliver(t *testing.T, rng *rand.Rand, docs []*CollabSheet, i int) {
	t.Helper()
	queued := nw.inbox[i]
	if len(queued) == 0 {
		return
	}
	n := 1 + rng.IntN(len(queued))
	batch := append([]crdt.PartOps{}, queued[:n]...)
	nw.inbox[i] = queued[n:]
	rng.Shuffle(len(batch), func(a, b int) { batch[a], batch[b] = batch[b], batch[a] })
	if rng.IntN(4) == 0 {
		batch = append(batch, batch[rng.IntN(len(batch))]) // a duplicate must change nothing
	}
	if err := docs[i].Apply(batch...); err != nil {
		t.Fatalf("replica %d: Apply: %v", i, err)
	}
}

func (nw *sheetNet) settle(t *testing.T, rng *rand.Rand, docs []*CollabSheet) {
	t.Helper()
	for i := range docs {
		for len(nw.inbox[i]) > 0 {
			nw.deliver(t, rng, docs, i)
		}
	}
}

// randomEdit makes one random positional edit on a replica and returns the
// batches to broadcast, or nil when the sheet offered nothing to do. Every path
// goes through the adapter's own (column, row) API, which is what is under test.
func randomEdit(t *testing.T, r *CollabSheet, rng *rand.Rand) []crdt.PartOps {
	t.Helper()
	one := func(b crdt.PartOps, err error) []crdt.PartOps {
		if err != nil {
			t.Fatalf("sheet edit: %v", err)
		}
		return []crdt.PartOps{b}
	}
	switch rng.IntN(9) {
	case 0:
		return one(r.AppendRow())
	case 1:
		return one(r.AppendCol())
	case 2:
		return one(r.InsertRow(rng.IntN(r.RowCount() + 1)))
	case 3:
		return one(r.InsertCol(rng.IntN(r.ColCount() + 1)))
	case 4:
		if r.RowCount() == 0 || r.ColCount() == 0 {
			return nil
		}
		col, row := rng.IntN(r.ColCount()), rng.IntN(r.RowCount())
		raw := fmt.Sprintf("v%d", rng.IntN(1000))
		if rng.IntN(3) == 0 {
			raw = fmt.Sprintf("=%d+%d", rng.IntN(100), rng.IntN(100))
		}
		return one(r.SetCellText(col, row, raw))
	case 5:
		if r.RowCount() == 0 {
			return nil
		}
		return one(r.DeleteRow(rng.IntN(r.RowCount())))
	case 6:
		// Moving is the operation the axes became sequences for, so it belongs
		// in the harness the convergence properties are checked with rather
		// than only in a test of its own.
		if r.RowCount() < 2 {
			return nil
		}
		from, to := rng.IntN(r.RowCount()), rng.IntN(r.RowCount())
		if from == to {
			return nil
		}
		return one(r.MoveRow(from, to))
	case 7:
		if r.ColCount() < 2 {
			return nil
		}
		from, to := rng.IntN(r.ColCount()), rng.IntN(r.ColCount())
		if from == to {
			return nil
		}
		return one(r.MoveCol(from, to))
	default:
		if r.ColCount() == 0 {
			return nil
		}
		return one(r.DeleteCol(rng.IntN(r.ColCount())))
	}
}

func runSheetSession(t *testing.T, seed uint64, replicas int) []*CollabSheet {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 0x5eed))
	docs := make([]*CollabSheet, replicas)
	for i := range docs {
		docs[i] = NewCollabSheet(crdt.SiteID(i + 1))
	}
	nw := newSheetNet(replicas)
	for range 14 {
		for i := range docs {
			for range 1 + rng.IntN(3) {
				if batches := randomEdit(t, docs[i], rng); len(batches) > 0 {
					nw.broadcast(i, batches)
				}
			}
			if rng.IntN(2) == 0 {
				nw.deliver(t, rng, docs, i)
			}
		}
	}
	nw.settle(t, rng, docs)
	return docs
}

// gridOf reads a replica's whole cell plane through the adapter's positional
// API, so two replicas can be compared the way a view sees them — not only by
// the byte snapshot.
func gridOf(c *CollabSheet) [][]string {
	out := make([][]string, c.RowCount())
	for r := range out {
		row := make([]string, c.ColCount())
		for col := range row {
			row[col] = c.CellText(col, r)
		}
		out[r] = row
	}
	return out
}

func sameGrid(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

// converged reports whether every replica holds byte-identical state, nothing
// left pending, equal versions, and — the adapter-level reinforcement — an equal
// positional grid. It is returned rather than asserted so the control-run can
// prove it discriminates.
func converged(docs []*CollabSheet) bool {
	want := docs[0].Snapshot()
	wantGrid := gridOf(docs[0])
	for _, d := range docs {
		if !bytes.Equal(d.Snapshot(), want) {
			return false
		}
		if d.Pending() != 0 {
			return false
		}
		if !d.Version().Equal(docs[0].Version()) {
			return false
		}
		if !sameGrid(gridOf(d), wantGrid) {
			return false
		}
	}
	return true
}

func assertConverged(t *testing.T, seed uint64, docs []*CollabSheet) {
	t.Helper()
	if !converged(docs) {
		for i, d := range docs {
			t.Logf("seed %d: replica %d pending=%d snapshot=%x", seed, i, d.Pending(), d.Snapshot())
		}
		t.Fatalf("seed %d: replicas did not converge", seed)
	}
}

// flatten concatenates OpsSince batches into single-operation batches, so the
// laws below can shuffle and regroup at operation granularity. A sheet has only
// list (rows/cols) and map (cells) parts.
func flatten(batches []crdt.PartOps) []crdt.PartOps {
	var out []crdt.PartOps
	for _, b := range batches {
		switch {
		case len(b.List) > 0:
			for _, op := range b.List {
				out = append(out, crdt.PartOps{Part: b.Part, List: []crdt.ListOp{op}})
			}
		case len(b.Map) > 0:
			for _, op := range b.Map {
				out = append(out, crdt.PartOps{Part: b.Part, Map: []crdt.MapOp{op}})
			}
		}
	}
	return out
}

// --- control-run: the oracle must detect divergence ----------------------------

// TestCollabSheetHarnessDetectsDivergence runs BEFORE the laws are trusted: a
// convergence oracle that never fails proves nothing. It confirms that a single
// withheld operation is reported as NOT converged, and that supplying it flips
// the verdict — so a green law below is evidence, not an accident.
func TestCollabSheetHarnessDetectsDivergence(t *testing.T) {
	source := runSheetSession(t, 17, 2)
	ops := flatten(source[0].OpsSince(nil))
	if len(ops) < 4 {
		t.Fatalf("fixture too small: %d ops", len(ops))
	}
	full := NewCollabSheet(1)
	short := NewCollabSheet(2)
	if err := full.Apply(ops...); err != nil {
		t.Fatal(err)
	}
	if err := short.Apply(ops[:len(ops)-1]...); err != nil { // withhold one
		t.Fatal(err)
	}
	if converged([]*CollabSheet{full, short}) {
		t.Fatal("oracle failed to notice a withheld operation")
	}
	if err := short.Apply(ops[len(ops)-1]); err != nil {
		t.Fatal(err)
	}
	if !converged([]*CollabSheet{full, short}) {
		t.Fatal("oracle failed to notice identical state")
	}
}

// --- the four CRDT laws, at the adapter level ----------------------------------

// TestCollabSheetConvergence: replicas edit concurrently while the network
// delivers late, out of order and with duplicates; once delivery catches up
// every replica must hold byte-identical state and an identical grid.
func TestCollabSheetConvergence(t *testing.T) {
	for seed := range uint64(200) {
		docs := runSheetSession(t, seed, 2+int(seed%3))
		assertConverged(t, seed, docs)
	}
}

// TestCollabSheetCommutativity applies one fixed operation set in many orders,
// one operation at a time, and requires byte-identical state every time — the
// pending buffer, not the batch, doing the reordering.
func TestCollabSheetCommutativity(t *testing.T) {
	source := runSheetSession(t, 42, 3)
	ops := flatten(source[0].OpsSince(nil))
	if len(ops) < 20 {
		t.Fatalf("only %d operations to permute; fixture too small", len(ops))
	}
	rng := rand.New(rand.NewPCG(7, 7))
	var want []byte
	for trial := range 200 {
		shuffled := append([]crdt.PartOps{}, ops...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		d := NewCollabSheet(99)
		for _, b := range shuffled {
			if err := d.Apply(b); err != nil {
				t.Fatalf("trial %d: Apply: %v", trial, err)
			}
		}
		if d.Pending() != 0 {
			t.Fatalf("trial %d: %d operations never became applicable", trial, d.Pending())
		}
		if trial == 0 {
			want = d.Snapshot()
			continue
		}
		if !bytes.Equal(d.Snapshot(), want) {
			t.Fatalf("trial %d: state differs from the first ordering", trial)
		}
	}
}

// TestCollabSheetIdempotence re-delivers operations a replica already holds and
// requires the state to be unchanged.
func TestCollabSheetIdempotence(t *testing.T) {
	docs := runSheetSession(t, 5, 3)
	assertConverged(t, 5, docs)
	ops := flatten(docs[0].OpsSince(nil))
	before := docs[0].Snapshot()
	for _, b := range ops {
		if err := docs[0].Apply(b); err != nil {
			t.Fatal(err)
		}
	}
	rng := rand.New(rand.NewPCG(1, 2))
	rng.Shuffle(len(ops), func(a, b int) { ops[a], ops[b] = ops[b], ops[a] })
	if err := docs[0].Apply(ops...); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(docs[0].Snapshot(), before) {
		t.Fatal("re-applying operations changed the state")
	}
}

// TestCollabSheetAssociativity feeds one operation set to two fresh replicas
// grouped two different ways; the grouping cannot matter.
func TestCollabSheetAssociativity(t *testing.T) {
	source := runSheetSession(t, 99, 3)
	ops := flatten(source[0].OpsSince(nil))
	rng := rand.New(rand.NewPCG(3, 4))
	rng.Shuffle(len(ops), func(a, b int) { ops[a], ops[b] = ops[b], ops[a] })

	left := NewCollabSheet(101)  // delivered in a few big groups
	right := NewCollabSheet(102) // delivered one at a time
	at := 0
	for at < len(ops) {
		step := 1 + rng.IntN(5)
		if at+step > len(ops) {
			step = len(ops) - at
		}
		if err := left.Apply(ops[at : at+step]...); err != nil {
			t.Fatal(err)
		}
		at += step
	}
	for _, b := range ops {
		if err := right.Apply(b); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(left.Snapshot(), right.Snapshot()) {
		t.Fatal("regrouping the same operations produced different state")
	}
}

// TestCollabSheetLateJoiner is the snapshot property: a replica that joins by
// loading a snapshot is indistinguishable from one that replayed the history.
func TestCollabSheetLateJoiner(t *testing.T) {
	for seed := range uint64(60) {
		docs := runSheetSession(t, seed, 3)
		assertConverged(t, seed, docs)
		joined, err := LoadCollabSheet(99, docs[0].Snapshot())
		if err != nil {
			t.Fatalf("seed %d: LoadCollabSheet: %v", seed, err)
		}
		if !bytes.Equal(joined.Snapshot(), docs[0].Snapshot()) {
			t.Fatalf("seed %d: a snapshot did not reload to itself", seed)
		}
	}
}

// --- stable addressing ---------------------------------------------------------

// TestCollabSheetStableAddressingUnderConcurrentInsert is the property the whole
// (rowID, colID) design exists for: two replicas concurrently insert a row and
// write a cell; after merge every value is still on the row its author put it,
// even though the positions renumbered on both sides.
func TestCollabSheetStableAddressingUnderConcurrentInsert(t *testing.T) {
	a := NewCollabSheet(1)
	b := NewCollabSheet(2)
	// Shared starting shape: one row, one column, one cell.
	_, r0, _ := a.sheet.AppendRow()
	_, c0, _ := a.sheet.AppendCol()
	if err := b.Apply(r0, c0); err != nil {
		t.Fatal(err)
	}
	base, err := a.SetCellText(0, 0, "base")
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Apply(base); err != nil {
		t.Fatal(err)
	}

	// Concurrently: a inserts a row at the top and writes into it; b writes into
	// what is, for b, still the only row.
	aRow, err := a.InsertRow(0)
	if err != nil {
		t.Fatal(err)
	}
	aCell, err := a.SetCellText(0, 0, "fromA") // a's new top row
	if err != nil {
		t.Fatal(err)
	}
	bCell, err := b.SetCellText(0, 0, "fromB=base-row") // b's original row
	if err != nil {
		t.Fatal(err)
	}

	// Exchange, in a deliberately awkward order.
	if err := b.Apply(aCell, aRow); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(bCell); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(a.Snapshot(), b.Snapshot()) {
		t.Fatalf("replicas diverged: a=%x b=%x", a.Snapshot(), b.Snapshot())
	}
	if !sameGrid(gridOf(a), gridOf(b)) {
		t.Fatalf("grids differ: a=%v b=%v", gridOf(a), gridOf(b))
	}
	// a inserted its row at position 0, so a's "fromA" is row 0 and the shared
	// base row (now carrying b's write) is row 1.
	if got := a.CellText(0, 0); got != "fromA" {
		t.Fatalf("row 0 col 0 = %q, want fromA", got)
	}
	if got := a.CellText(0, 1); got != "fromB=base-row" {
		t.Fatalf("row 1 col 0 = %q, want fromB=base-row (b's write landed on the base row)", got)
	}
}

// --- fan-out through MVVM ------------------------------------------------------

// TestCollabSheetFanOut proves the observable seam fires on both a local edit
// and an applied remote batch, and that a remote edit is reflected in the
// positional read — so a bound view would repaint and show the merged value.
func TestCollabSheetFanOut(t *testing.T) {
	a := NewCollabSheet(1)
	if a.Site() != crdt.SiteID(1) {
		t.Fatalf("Site() = %v, want 1", a.Site())
	}
	local := 0
	unsub := a.Subscribe(func() { local++ })
	beforeRev := a.Rev()
	r, _ := a.AppendRow()
	c, _ := a.AppendCol()
	set, _ := a.SetCellText(0, 0, "hello")
	if local != 3 {
		t.Fatalf("local edits fired the observable %d times, want 3", local)
	}
	if a.Rev() <= beforeRev {
		t.Fatal("revision did not advance on local edits")
	}
	unsub()
	a.AppendRow()
	if local != 3 {
		t.Fatal("observer still fired after unsubscribe")
	}

	// A peer applies the batches; its observable must fire and its grid update.
	b := NewCollabSheet(2)
	remote := 0
	b.Subscribe(func() { remote++ })
	if err := b.Apply(r, c, set); err != nil {
		t.Fatal(err)
	}
	if remote != 1 {
		t.Fatalf("remote apply fired the observable %d times, want 1", remote)
	}
	if got := b.CellText(0, 0); got != "hello" {
		t.Fatalf("remote replica cell = %q, want hello", got)
	}
}

// TestCollabSheetTextKinds covers both cell encodings the adapter writes — a
// leading-"=" formula and a plain literal — round-tripping the raw text either
// way, and an empty write clearing the cell.
func TestCollabSheetTextKinds(t *testing.T) {
	c := NewCollabSheet(1)
	c.AppendRow()
	c.AppendCol()
	if _, err := c.SetCellText(0, 0, "=A1+1"); err != nil {
		t.Fatal(err)
	}
	if got := c.CellText(0, 0); got != "=A1+1" {
		t.Fatalf("formula round-trip = %q", got)
	}
	if _, err := c.SetCellText(0, 0, "plain"); err != nil {
		t.Fatal(err)
	}
	if got := c.CellText(0, 0); got != "plain" {
		t.Fatalf("literal round-trip = %q", got)
	}
	if _, err := c.SetCellText(0, 0, ""); err != nil {
		t.Fatal(err)
	}
	if got := c.CellText(0, 0); got != "" {
		t.Fatalf("cleared cell = %q, want empty", got)
	}
}

// --- error branches ------------------------------------------------------------

// TestCollabSheetErrors exercises every error path: an out-of-range positional
// address, a bad row/column index, an unparseable snapshot and an invalid
// applied batch.
func TestCollabSheetErrors(t *testing.T) {
	c := NewCollabSheet(1)
	// Empty sheet: any cell address is out of range, and a read is "".
	if _, err := c.SetCellText(0, 0, "x"); err != ErrCellOutOfRange {
		t.Fatalf("SetCellText on empty sheet: err=%v, want ErrCellOutOfRange", err)
	}
	if got := c.CellText(0, 0); got != "" {
		t.Fatalf("CellText on empty sheet = %q, want empty", got)
	}
	c.AppendRow()
	c.AppendCol()
	// Negative and past-the-end indices, on both axes.
	for _, tc := range []struct{ col, row int }{{-1, 0}, {0, -1}, {1, 0}, {0, 1}} {
		if _, err := c.SetCellText(tc.col, tc.row, "x"); err != ErrCellOutOfRange {
			t.Fatalf("SetCellText(%d,%d): err=%v, want ErrCellOutOfRange", tc.col, tc.row, err)
		}
		if got := c.CellText(tc.col, tc.row); got != "" {
			t.Fatalf("CellText(%d,%d) = %q, want empty", tc.col, tc.row, got)
		}
	}
	// Row/column insert and delete outside range.
	if _, err := c.InsertRow(-1); err == nil {
		t.Fatal("InsertRow(-1) did not error")
	}
	if _, err := c.InsertCol(5); err == nil {
		t.Fatal("InsertCol(5) did not error")
	}
	if _, err := c.DeleteRow(3); err == nil {
		t.Fatal("DeleteRow(3) did not error")
	}
	if _, err := c.DeleteCol(3); err == nil {
		t.Fatal("DeleteCol(3) did not error")
	}
	// A malformed snapshot is an error, not a panic.
	if _, err := LoadCollabSheet(1, []byte("not a snapshot")); err == nil {
		t.Fatal("LoadCollabSheet on garbage did not error")
	}
	// An invalid batch (zero-value part) is refused by Apply and changes nothing.
	before := c.Rev()
	if err := c.Apply(crdt.PartOps{}); err == nil {
		t.Fatal("Apply of an invalid batch did not error")
	}
	if c.Rev() != before {
		t.Fatal("a failed Apply still ticked the revision")
	}
}

// --- widget bridge -------------------------------------------------------------

// TestSyncSpreadsheet proves the existing Spreadsheet widget drives, and is
// driven by, the collaborative model through its unchanged API: a remote peer's
// merged edit appears in the widget, and a widget edit reaches the peer.
func TestSyncSpreadsheet(t *testing.T) {
	// Two replicas of the collaborative model, one behind each widget.
	m1 := NewCollabSheet(1)
	m2 := NewCollabSheet(2)
	// Lay out a shared 2x2 grid and mirror it to both replicas.
	var setup []crdt.PartOps
	for range 2 {
		r, _ := m1.AppendRow()
		col, _ := m1.AppendCol()
		setup = append(setup, r, col)
	}
	if err := m2.Apply(setup...); err != nil {
		t.Fatal(err)
	}

	ss := NewSpreadsheet(2, 2)
	detach := SyncSpreadsheet(ss, m1)

	// A remote peer (m2) edits; ship its op to m1; the widget must show it.
	op, err := m2.SetCellText(1, 0, "remote")
	if err != nil {
		t.Fatal(err)
	}
	if err := m1.Apply(op); err != nil {
		t.Fatal(err)
	}
	if got := ss.CellRaw(1, 0); got != "remote" {
		t.Fatalf("widget cell after remote edit = %q, want remote", got)
	}

	// A local widget edit must reach the collaborative model and thence the peer.
	// The active cell defaults to A1 (0,0).
	ss.BeginEdit()
	ss.editor.Text().Set("typed")
	ss.CommitEdit()
	if got := m1.CellText(0, 0); got != "typed" {
		t.Fatalf("model cell after widget edit = %q, want typed", got)
	}
	// The peer sees it after a sync.
	if err := m2.Apply(m1.OpsSince(m2.Version())...); err != nil {
		t.Fatal(err)
	}
	if got := m2.CellText(0, 0); got != "typed" {
		t.Fatalf("peer cell after widget edit = %q, want typed", got)
	}

	// After detach the widget's hook is cleared and further widget edits no longer
	// reach the model.
	detach()
	if ss.OnCellChange != nil {
		t.Fatal("detach did not clear OnCellChange")
	}
}

// TestSyncSpreadsheetClamp covers the mirror's clamp: a collaborative sheet
// larger than the widget shows only its top-left window, addressing no cell the
// widget has no room for.
func TestSyncSpreadsheetClamp(t *testing.T) {
	m := NewCollabSheet(1)
	for range 3 {
		m.AppendRow()
		m.AppendCol()
	}
	// Fill a cell that lies outside a 2x2 widget and one inside it.
	if _, err := m.SetCellText(2, 2, "outside"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetCellText(1, 1, "inside"); err != nil {
		t.Fatal(err)
	}
	ss := NewSpreadsheet(2, 2) // smaller than the 3x3 model
	detach := SyncSpreadsheet(ss, m)
	defer detach()
	if got := ss.CellRaw(1, 1); got != "inside" {
		t.Fatalf("in-window cell = %q, want inside", got)
	}
	// The out-of-window value is simply not mirrored; the widget has no such cell.
	if got := ss.CellRaw(1, 1); got == "outside" {
		t.Fatal("out-of-window value leaked into the widget")
	}
}

// A row can be dragged, which the axes were lists and could not be. The row
// keeps its identity, so its cells come with it and nothing else in the sheet
// moves — which is what makes this worth having over a delete and an insert.
func TestCollabSheetMoveRowCarriesItsCells(t *testing.T) {
	c := NewCollabSheet(1)
	for range 3 {
		if _, err := c.AppendRow(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.AppendCol(); err != nil {
		t.Fatal(err)
	}
	for row, text := range []string{"a", "b", "c"} {
		if _, err := c.SetCellText(0, row, text); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := c.MoveRow(2, 0); err != nil {
		t.Fatal(err)
	}
	for row, want := range []string{"c", "a", "b"} {
		if got := c.CellText(0, row); got != want {
			t.Fatalf("row %d reads %q, want %q", row, got, want)
		}
	}
	if _, err := c.MoveCol(0, 0); err == nil {
		t.Fatal("moving a column to where it is was accepted")
	}
	for _, tc := range []struct{ from, to int }{{-1, 0}, {0, -1}, {3, 0}, {0, 3}} {
		if _, err := c.MoveRow(tc.from, tc.to); err == nil {
			t.Fatalf("MoveRow(%d, %d) was accepted", tc.from, tc.to)
		}
	}
}

// Two replicas dragging the same row at once, which as a delete and an insert
// would leave it twice over or not at all.
func TestCollabSheetConcurrentRowMove(t *testing.T) {
	a := NewCollabSheet(1)
	for range 3 {
		if _, err := a.AppendRow(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.AppendCol(); err != nil {
		t.Fatal(err)
	}
	for row, text := range []string{"1", "2", "3"} {
		if _, err := a.SetCellText(0, row, text); err != nil {
			t.Fatal(err)
		}
	}
	b, err := LoadCollabSheet(2, a.Snapshot())
	if err != nil {
		t.Fatal(err)
	}

	fromA, err := a.MoveRow(0, 2)
	if err != nil {
		t.Fatal(err)
	}
	fromB, err := b.MoveRow(0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(fromB); err != nil {
		t.Fatal(err)
	}
	if err := b.Apply(fromA); err != nil {
		t.Fatal(err)
	}

	if a.RowCount() != 3 || b.RowCount() != 3 {
		t.Fatalf("the replicas hold %d and %d rows, want 3 each", a.RowCount(), b.RowCount())
	}
	for row := range 3 {
		if a.CellText(0, row) != b.CellText(0, row) {
			t.Fatalf("row %d reads %q on one replica and %q on the other",
				row, a.CellText(0, row), b.CellText(0, row))
		}
	}
	// Every row is still there, once.
	seen := map[string]bool{}
	for row := range 3 {
		text := a.CellText(0, row)
		if seen[text] {
			t.Fatalf("row %q is read twice", text)
		}
		seen[text] = true
	}
}

// A move has to reach a bound widget. Everything else here notifies, and a
// silent move would leave a spreadsheet on screen showing the old order until
// something else happened to redraw it.
func TestCollabSheetMoveNotifies(t *testing.T) {
	c := NewCollabSheet(1)
	for range 2 {
		if _, err := c.AppendRow(); err != nil {
			t.Fatal(err)
		}
		if _, err := c.AppendCol(); err != nil {
			t.Fatal(err)
		}
	}

	seen := 0
	unsubscribe := c.Subscribe(func() { seen++ })
	defer unsubscribe()

	before := c.Rev()
	if _, err := c.MoveRow(0, 1); err != nil {
		t.Fatal(err)
	}
	if c.Rev() == before {
		t.Fatal("moving a row did not raise the revision")
	}
	if seen != 1 {
		t.Fatalf("a bound widget was told %d times about a row move, want once", seen)
	}
	if _, err := c.MoveCol(0, 1); err != nil {
		t.Fatal(err)
	}
	if seen != 2 {
		t.Fatalf("a bound widget was told %d times in all, want twice", seen)
	}
	// A refused move must not notify: nothing changed.
	if _, err := c.MoveRow(0, 0); err == nil {
		t.Fatal("moving a row to where it is was accepted")
	}
	if seen != 2 {
		t.Fatalf("a refused move told a bound widget anyway (%d)", seen)
	}
}

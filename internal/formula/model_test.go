// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "testing"

// ref is a terse Ref constructor for the tests.
func ref(col, row int) Ref { return Ref{Col: col, Row: row} }

// setDisplay sets a cell and returns nothing; display reads it back.
func display(m *Model, col, row int) string { return m.Display(ref(col, row)) }

func TestNewModelClampsAndReports(t *testing.T) {
	m := NewModel(-1, -2)
	if m.Cols() != 0 || m.Rows() != 0 {
		t.Errorf("NewModel(-1,-2) dims = %dx%d, want 0x0", m.Cols(), m.Rows())
	}
	m = NewModel(3, 4)
	if m.Cols() != 3 || m.Rows() != 4 {
		t.Errorf("NewModel(3,4) dims = %dx%d, want 3x4", m.Cols(), m.Rows())
	}
}

func TestModelInBounds(t *testing.T) {
	m := NewModel(2, 2)
	cases := []struct {
		r    Ref
		want bool
	}{
		{ref(0, 0), true},
		{ref(-1, 0), false}, // col < 0
		{ref(2, 0), false},  // col >= cols
		{ref(0, -1), false}, // row < 0
		{ref(0, 2), false},  // row >= rows
	}
	for _, c := range cases {
		if got := m.InBounds(c.r); got != c.want {
			t.Errorf("InBounds(%+v) = %v, want %v", c.r, got, c.want)
		}
	}
}

func TestLiteralCells(t *testing.T) {
	m := NewModel(3, 3)
	m.SetCell(ref(0, 0), "42")
	m.SetCell(ref(0, 1), "hello")
	m.SetCell(ref(0, 2), " 3.5 ") // spaces trimmed for numeric detection
	m.SetCell(ref(1, 0), "\t7\t") // tabs trimmed too

	if got := display(m, 0, 0); got != "42" {
		t.Errorf("A1 = %q, want 42", got)
	}
	if v := m.Get(ref(0, 0)); v.Kind != KindNumber || v.Num != 42 {
		t.Errorf("A1 value = %+v, want number 42", v)
	}
	if got := display(m, 0, 1); got != "hello" {
		t.Errorf("A2 = %q, want hello", got)
	}
	if v := m.Get(ref(0, 1)); v.Kind != KindText {
		t.Errorf("A2 value = %+v, want text", v)
	}
	if got := display(m, 0, 2); got != "3.5" {
		t.Errorf("A3 = %q, want 3.5", got)
	}
	if got := display(m, 1, 0); got != "7" {
		t.Errorf("B1 = %q, want 7", got)
	}
	if got := m.Raw(ref(0, 0)); got != "42" {
		t.Errorf("Raw(A1) = %q, want 42", got)
	}
}

func TestEmptyAndOutOfBoundsCells(t *testing.T) {
	m := NewModel(2, 2)
	// A never-written cell is blank.
	if got := display(m, 1, 1); got != "" {
		t.Errorf("blank cell display = %q, want empty", got)
	}
	if got := m.Raw(ref(1, 1)); got != "" {
		t.Errorf("blank cell Raw = %q, want empty", got)
	}
	// Setting out of bounds is a no-op.
	m.SetCell(ref(9, 9), "99")
	if got := display(m, 9, 9); got != "" {
		t.Errorf("OOB set leaked: %q", got)
	}
	// Clearing removes the cell.
	m.SetCell(ref(0, 0), "5")
	m.SetCell(ref(0, 0), "")
	if got := display(m, 0, 0); got != "" {
		t.Errorf("cleared cell display = %q, want empty", got)
	}
	if got := m.Raw(ref(0, 0)); got != "" {
		t.Errorf("cleared cell Raw = %q, want empty", got)
	}
}

func TestFormulaRecalcOnEdit(t *testing.T) {
	m := NewModel(3, 3)
	m.SetCell(ref(0, 0), "10")    // A1 literal
	m.SetCell(ref(1, 0), "=A1*2") // B1 formula reads a literal
	if got := display(m, 1, 0); got != "20" {
		t.Fatalf("B1 = %q, want 20", got)
	}
	m.SetCell(ref(0, 0), "20") // editing A1 recomputes B1
	if got := display(m, 1, 0); got != "40" {
		t.Fatalf("B1 after edit = %q, want 40", got)
	}
}

func TestFormulaChainPropagation(t *testing.T) {
	m := NewModel(3, 3)
	m.SetCell(ref(0, 0), "1")     // A1 literal
	m.SetCell(ref(1, 0), "=A1+1") // B1 = 2
	m.SetCell(ref(2, 0), "=B1+1") // C1 = 3 (depends on a FORMULA cell)
	if got := display(m, 1, 0); got != "2" {
		t.Errorf("B1 = %q, want 2", got)
	}
	if got := display(m, 2, 0); got != "3" {
		t.Errorf("C1 = %q, want 3", got)
	}
	m.SetCell(ref(0, 0), "10") // ripples the whole chain
	if got := display(m, 1, 0); got != "11" {
		t.Errorf("B1 after edit = %q, want 11", got)
	}
	if got := display(m, 2, 0); got != "12" {
		t.Errorf("C1 after edit = %q, want 12", got)
	}
}

// TestTwoFormulaPrecedents drives the partial in-degree decrement (C1 waits on
// two formula precedents).
func TestTwoFormulaPrecedents(t *testing.T) {
	m := NewModel(3, 3)
	m.SetCell(ref(0, 0), "=1")     // A1 formula
	m.SetCell(ref(1, 0), "=2")     // B1 formula
	m.SetCell(ref(2, 0), "=A1+B1") // C1 depends on both
	if got := display(m, 2, 0); got != "3" {
		t.Errorf("C1 = %q, want 3", got)
	}
}

// TestDuplicatePrecedentDeduped drives the `seen` de-dup: a formula precedent
// referenced twice must not double its in-degree (else it would never resolve).
func TestDuplicatePrecedentDeduped(t *testing.T) {
	m := NewModel(2, 2)
	m.SetCell(ref(0, 0), "=1")     // A1 formula
	m.SetCell(ref(1, 0), "=A1+A1") // B1 references A1 twice
	if got := display(m, 1, 0); got != "2" {
		t.Errorf("B1 = %q, want 2", got)
	}
}

func TestSelfReferenceIsCircular(t *testing.T) {
	m := NewModel(2, 2)
	m.SetCell(ref(0, 0), "=A1+1")
	if got := display(m, 0, 0); got != "#CIRC!" {
		t.Errorf("A1 self-ref = %q, want #CIRC!", got)
	}
}

func TestMutualReferenceIsCircular(t *testing.T) {
	m := NewModel(2, 2)
	m.SetCell(ref(0, 0), "=B1")
	m.SetCell(ref(1, 0), "=A1")
	if got := display(m, 0, 0); got != "#CIRC!" {
		t.Errorf("A1 = %q, want #CIRC!", got)
	}
	if got := display(m, 1, 0); got != "#CIRC!" {
		t.Errorf("B1 = %q, want #CIRC!", got)
	}
}

// TestCycleWithHealthyCellsCoexist covers the leftover loop's both arms: a
// resolved (done) formula cell alongside cells trapped in a cycle.
func TestCycleWithHealthyCellsCoexist(t *testing.T) {
	m := NewModel(4, 4)
	m.SetCell(ref(0, 0), "1")     // A1 literal
	m.SetCell(ref(1, 0), "=A1+1") // B1 resolves to 2 (done)
	m.SetCell(ref(2, 0), "=D1")   // C1 <-> D1 cycle
	m.SetCell(ref(3, 0), "=C1")
	if got := display(m, 1, 0); got != "2" {
		t.Errorf("B1 = %q, want 2", got)
	}
	if got := display(m, 2, 0); got != "#CIRC!" {
		t.Errorf("C1 = %q, want #CIRC!", got)
	}
	if got := display(m, 3, 0); got != "#CIRC!" {
		t.Errorf("D1 = %q, want #CIRC!", got)
	}
}

// TestDownstreamOfCycleIsCircular covers a cell that is not itself in a cycle
// but depends on one.
func TestDownstreamOfCycleIsCircular(t *testing.T) {
	m := NewModel(4, 4)
	m.SetCell(ref(0, 0), "=B1")
	m.SetCell(ref(1, 0), "=A1")
	m.SetCell(ref(2, 0), "=A1+1") // C1 depends on the cyclic A1
	if got := display(m, 2, 0); got != "#CIRC!" {
		t.Errorf("C1 downstream of cycle = %q, want #CIRC!", got)
	}
}

func TestUnparseableFormulaIsName(t *testing.T) {
	m := NewModel(2, 2)
	m.SetCell(ref(0, 0), "=1+")   // A1 does not parse
	m.SetCell(ref(1, 0), "=A1+1") // B1 propagates the #NAME?
	if got := display(m, 0, 0); got != "#NAME?" {
		t.Errorf("A1 = %q, want #NAME?", got)
	}
	if got := display(m, 1, 0); got != "#NAME?" {
		t.Errorf("B1 = %q, want #NAME?", got)
	}
	// Raw is preserved so the editor can re-open the bad formula.
	if got := m.Raw(ref(0, 0)); got != "=1+" {
		t.Errorf("Raw(A1) = %q, want =1+", got)
	}
}

// TestRefsOfNodeKinds drives every AST node kind through refsOf during
// recompute: single ref, range (in- and out-of-bounds), unary, call, and the
// bare-name / literal leaves.
func TestRefsOfNodeKinds(t *testing.T) {
	m := NewModel(3, 3)
	m.SetCell(ref(0, 0), "2") // A1
	m.SetCell(ref(1, 0), "3") // B1
	m.SetCell(ref(0, 1), "4") // A2
	m.SetCell(ref(1, 1), "5") // B2

	m.SetCell(ref(2, 0), "=SUM(A1:B2)") // C1 range in-bounds -> 14
	m.SetCell(ref(2, 1), "=-A1")        // C2 unary over a ref -> -2
	m.SetCell(ref(2, 2), `="x"`)        // C3 string leaf
	m.SetCell(ref(0, 2), "=FOO")        // A3 bare name leaf -> #NAME?
	m.SetCell(ref(1, 2), "=SUM(B1:Z1)") // B3 range clipped by bounds -> #REF! (excludes B3)

	if got := display(m, 2, 0); got != "14" {
		t.Errorf("C1 = %q, want 14", got)
	}
	if got := display(m, 2, 1); got != "-2" {
		t.Errorf("C2 = %q, want -2", got)
	}
	if got := display(m, 2, 2); got != "x" {
		t.Errorf("C3 = %q, want x", got)
	}
	if got := display(m, 0, 2); got != "#NAME?" {
		t.Errorf("A3 = %q, want #NAME?", got)
	}
	if got := display(m, 1, 2); got != "#REF!" {
		t.Errorf("B3 = %q, want #REF!", got)
	}
}

// TestRefsOfReversedRange drives refsOf's corner-normalisation arms (c1>c2 and
// r1>r2) via the Model, whose recompute is the only caller of refsOf. The
// formula sits in C1 so the reversed range B2:A1 never includes it.
func TestRefsOfReversedRange(t *testing.T) {
	m := NewModel(3, 2)
	m.SetCell(ref(0, 0), "1")           // A1
	m.SetCell(ref(1, 0), "2")           // B1
	m.SetCell(ref(0, 1), "3")           // A2
	m.SetCell(ref(1, 1), "4")           // B2
	m.SetCell(ref(2, 0), "=SUM(B2:A1)") // C1: reversed both corners -> 1+2+3+4
	if got := display(m, 2, 0); got != "10" {
		t.Errorf("reversed-range sum = %q, want 10", got)
	}
}

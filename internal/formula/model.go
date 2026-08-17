// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "strconv"

// Model is an A1-addressed grid of cells and the formula engine over it. A cell
// holds raw user input — a literal (number or text) or a leading-"=" formula —
// and a computed Value. SetCell recomputes every formula cell in dependency
// order (with #CIRC! for cycles); Get and Display read the results back.
//
// The zero Model is not usable; build one with NewModel.
type Model struct {
	cols, rows int
	cells      map[Ref]*cell
}

// cell is one stored grid entry.
type cell struct {
	raw       string // exactly what the user typed
	isFormula bool   // raw began with '='
	ast       *node  // parsed formula, nil for a literal or an unparseable formula
	val       Value  // computed result (== the literal value for a literal cell)
}

// NewModel builds an empty cols x rows sheet. Negative dimensions clamp to 0.
func NewModel(cols, rows int) *Model {
	if cols < 0 {
		cols = 0
	}
	if rows < 0 {
		rows = 0
	}
	return &Model{cols: cols, rows: rows, cells: map[Ref]*cell{}}
}

// Cols reports the sheet's column count.
func (m *Model) Cols() int { return m.cols }

// Rows reports the sheet's row count.
func (m *Model) Rows() int { return m.rows }

// InBounds reports whether r lies inside the sheet.
func (m *Model) InBounds(r Ref) bool {
	return r.Col >= 0 && r.Col < m.cols && r.Row >= 0 && r.Row < m.rows
}

// Get returns the computed value of r: the cell's value, or a blank for an
// empty (or out-of-bounds) cell.
func (m *Model) Get(r Ref) Value {
	if c := m.cells[r]; c != nil {
		return c.val
	}
	return Blank()
}

// Display is the string cell r renders — the computed value's Display form.
func (m *Model) Display(r Ref) string { return m.Get(r).Display() }

// Raw returns the raw text stored in r (what an editor re-opens), or "" when
// the cell is empty.
func (m *Model) Raw(r Ref) string {
	if c := m.cells[r]; c != nil {
		return c.raw
	}
	return ""
}

// SetCell stores raw as the contents of cell r and recomputes the sheet. An
// empty raw clears the cell. A leading '=' marks a formula (parsed now, with a
// parse failure stored as a #NAME? value); anything else is a literal, kept as
// a number when it parses as one and as text otherwise. Setting an
// out-of-bounds cell is a no-op.
func (m *Model) SetCell(r Ref, raw string) {
	if !m.InBounds(r) {
		return
	}
	if raw == "" {
		delete(m.cells, r)
		m.recompute()
		return
	}
	c := &cell{raw: raw}
	if raw[0] == '=' {
		c.isFormula = true
		ast, err := Parse(raw[1:])
		if err != nil {
			c.val = Error(ErrName)
		} else {
			c.ast = ast
		}
	} else {
		c.val = literalValue(raw)
	}
	m.cells[r] = c
	m.recompute()
}

// literalValue interprets a non-formula cell: a number when the whole text
// parses as one, otherwise text.
func literalValue(raw string) Value {
	if f, err := strconv.ParseFloat(trimSpace(raw), 64); err == nil {
		return Number(f)
	}
	return TextValue(raw)
}

// trimSpace trims ASCII spaces and tabs from both ends — enough for numeric
// literal detection without pulling in unicode handling.
func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// recompute re-evaluates every formula cell in dependency order. It builds the
// dependency graph over the formula cells (edges precedent -> dependent),
// topologically sorts them with Kahn's algorithm, evaluates in that order (so
// every precedent is ready), and marks whatever the sort cannot reach — the
// cells in a cycle and everything downstream of one — as #CIRC!.
//
// The whole sheet is recomputed on each edit. The grid a widget drives is
// small, so a full pass is both correct and trivially free of stale reads; the
// dependency graph earns its keep as the ordering and the cycle detector.
func (m *Model) recompute() {
	// The formula cells with a parseable AST are the graph's nodes; literal,
	// blank and parse-error cells are always-ready leaves read via Get.
	inGraph := map[Ref]bool{}
	for ref, c := range m.cells {
		if c.isFormula && c.ast != nil {
			inGraph[ref] = true
		}
	}
	indeg := map[Ref]int{}
	dependents := map[Ref][]Ref{}
	for ref := range inGraph {
		seen := map[Ref]bool{}
		for _, pr := range refsOf(m.cells[ref].ast, m) {
			if inGraph[pr] && !seen[pr] {
				seen[pr] = true
				indeg[ref]++
				dependents[pr] = append(dependents[pr], ref)
			}
		}
	}
	var queue []Ref
	for ref := range inGraph {
		if indeg[ref] == 0 {
			queue = append(queue, ref)
		}
	}
	done := map[Ref]bool{}
	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		done[ref] = true
		m.cells[ref].val = eval(m.cells[ref].ast, m)
		for _, d := range dependents[ref] {
			indeg[d]--
			if indeg[d] == 0 {
				queue = append(queue, d)
			}
		}
	}
	for ref := range inGraph {
		if !done[ref] {
			m.cells[ref].val = Error(ErrCirc)
		}
	}
}

// refsOf lists the in-bounds cells an AST reads: single refs directly, ranges
// expanded (normalised, clipped to the sheet). A self-reference is included, so
// the graph carries the self-edge that makes A1=A1+1 a cycle. Duplicates are
// fine; recompute de-dupes per dependent.
func refsOf(n *node, m *Model) []Ref {
	var out []Ref
	var walk func(*node)
	walk = func(nd *node) {
		switch nd.kind {
		case nRef:
			out = append(out, nd.ref)
		case nRange:
			c1, c2 := nd.ref.Col, nd.ref2.Col
			r1, r2 := nd.ref.Row, nd.ref2.Row
			if c1 > c2 {
				c1, c2 = c2, c1
			}
			if r1 > r2 {
				r1, r2 = r2, r1
			}
			for rr := r1; rr <= r2; rr++ {
				for cc := c1; cc <= c2; cc++ {
					if ref := (Ref{Col: cc, Row: rr}); m.InBounds(ref) {
						out = append(out, ref)
					}
				}
			}
		case nUnary:
			walk(nd.x)
		case nBin:
			walk(nd.l)
			walk(nd.r)
		case nCall:
			for _, a := range nd.args {
				walk(a)
			}
		}
	}
	walk(n)
	return out
}

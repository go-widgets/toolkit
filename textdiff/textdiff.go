// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package textdiff compares two texts line by line: the shortest edit
// script (Myers, "An O(ND) Difference Algorithm and Its Variations", 1986),
// grouped into hunks with context, as a unified diff, or as rows for a
// side-by-side view.
//
// The search is Myers' linear-space refinement: it finds the middle snake of
// the edit graph from both ends at once and recurses on the two halves, so
// memory is O(N+M) whatever the distance, and time O((N+M)·D) for D edits.
// Lines are interned to integers first, so a comparison is one integer test,
// and the common prefix and suffix are taken off before the search.
//
// Two texts that share almost nothing can make D close to N+M, and the time
// quadratic. A budget bounds the work: when a sub-problem exceeds it, that
// part is reported as deleted and inserted as a whole. The script is then
// still correct (applying it to a gives b) but no longer minimal there, as
// GNU diff does past its "too expensive" heuristic. Options.Budget sets it;
// the default keeps two 10 000-line texts with nothing in common well under
// a second.
package textdiff

import (
	"fmt"
	"strings"
)

// Op is what an edit does to a line.
type Op uint8

const (
	// Equal keeps line A of a, which is line B of b.
	Equal Op = iota
	// Delete removes line A of a.
	Delete
	// Insert adds line B of b.
	Insert
)

func (o Op) String() string {
	switch o {
	case Equal:
		return "="
	case Delete:
		return "-"
	case Insert:
		return "+"
	}
	return fmt.Sprintf("Op(%d)", uint8(o))
}

// Edit is one step of the script, in order: A indexes a (Equal, Delete),
// B indexes b (Equal, Insert). The index an edit does not use is the
// position in that text where it happens, so a script can be walked with
// both cursors.
type Edit struct {
	Op   Op
	A, B int
}

// Options tune the comparison.
type Options struct {
	// Budget bounds the work of the search, in steps of the edit graph
	// (0: DefaultBudget; negative: no bound).
	Budget int
}

// DefaultBudget is the work allowed when Options.Budget is 0.
const DefaultBudget = 20_000_000

// Lines returns the shortest edit script turning a into b.
func Lines(a, b []string) []Edit { return LinesWith(a, b, Options{}) }

// LinesWith is Lines with options.
func LinesWith(a, b []string, o Options) []Edit {
	ids := make(map[string]int, len(a)+len(b))
	intern := func(s []string) []int {
		out := make([]int, len(s))
		for i, l := range s {
			id, ok := ids[l]
			if !ok {
				id = len(ids)
				ids[l] = id
			}
			out[i] = id
		}
		return out
	}
	d := &differ{a: intern(a), b: intern(b), budget: o.Budget}
	if d.budget == 0 {
		d.budget = DefaultBudget
	}
	d.edits = make([]Edit, 0, max(len(a), len(b)))
	d.seq(0, len(a), 0, len(b))
	return deletesFirst(d.edits)
}

// deletesFirst puts, in each run of changes, the deletions before the
// insertions, as diff -u prints them (the search may find them in either
// order), and fixes the position each edit gives in the text it does not
// index: a deletion sits where the run starts in b, an insertion where it
// ends in a.
func deletesFirst(edits []Edit) []Edit {
	for i := 0; i < len(edits); {
		if edits[i].Op == Equal {
			i++
			continue
		}
		j := i
		for j < len(edits) && edits[j].Op != Equal {
			j++
		}
		var dels, ins []Edit
		for _, e := range edits[i:j] {
			if e.Op == Delete {
				dels = append(dels, e)
			} else {
				ins = append(ins, e)
			}
		}
		aEnd, bStart := edits[i].A, edits[i].B
		if len(dels) > 0 {
			aEnd = dels[len(dels)-1].A + 1
		}
		if len(ins) > 0 {
			bStart = ins[0].B
		}
		k := i
		for _, e := range dels {
			edits[k] = Edit{Delete, e.A, bStart}
			k++
		}
		for _, e := range ins {
			edits[k] = Edit{Insert, aEnd, e.B}
			k++
		}
		i = j
	}
	return edits
}

// Split cuts a text into lines, without their "\n". A final newline does not
// make an empty last line, and an empty text has no lines.
func Split(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.TrimSuffix(s, "\n")
	return strings.Split(s, "\n")
}

type differ struct {
	a, b   []int
	edits  []Edit
	budget int // < 0: unbounded
	vf, vb []int
}

// seq emits the script for a[a0:a1] against b[b0:b1].
func (d *differ) seq(a0, a1, b0, b1 int) {
	// the common prefix
	for a0 < a1 && b0 < b1 && d.a[a0] == d.b[b0] {
		d.edits = append(d.edits, Edit{Equal, a0, b0})
		a0++
		b0++
	}
	// the common suffix, emitted last
	s := 0
	for a1-s > a0 && b1-s > b0 && d.a[a1-s-1] == d.b[b1-s-1] {
		s++
	}
	a1, b1 = a1-s, b1-s
	switch {
	case a0 == a1:
		for j := b0; j < b1; j++ {
			d.edits = append(d.edits, Edit{Insert, a0, j})
		}
	case b0 == b1:
		for i := a0; i < a1; i++ {
			d.edits = append(d.edits, Edit{Delete, i, b0})
		}
	case !d.share(a0, a1, b0, b1):
		// nothing in common: no search can do better than replacing
		for i := a0; i < a1; i++ {
			d.edits = append(d.edits, Edit{Delete, i, b0})
		}
		for j := b0; j < b1; j++ {
			d.edits = append(d.edits, Edit{Insert, a1, j})
		}
	default:
		x, y, ok := d.middle(a0, a1, b0, b1)
		if !ok || (x == a0 && y == b0) || (x == a1 && y == b1) {
			// too expensive, or no progress: replace the block whole
			for i := a0; i < a1; i++ {
				d.edits = append(d.edits, Edit{Delete, i, b0})
			}
			for j := b0; j < b1; j++ {
				d.edits = append(d.edits, Edit{Insert, a1, j})
			}
		} else {
			d.seq(a0, x, b0, y)
			d.seq(x, a1, y, b1)
		}
	}
	for i := 0; i < s; i++ {
		d.edits = append(d.edits, Edit{Equal, a1 + i, b1 + i})
	}
}

// share reports whether a[a0:a1] and b[b0:b1] have a line in common. It
// costs O(N+M), against the search's O((N+M)·D) that two blocks with
// nothing in common would pay in full (D = N+M).
func (d *differ) share(a0, a1, b0, b1 int) bool {
	if a1-a0 < 64 && b1-b0 < 64 {
		return true // small: the search is cheaper than the set
	}
	seen := make(map[int]struct{}, a1-a0)
	for _, id := range d.a[a0:a1] {
		seen[id] = struct{}{}
	}
	for _, id := range d.b[b0:b1] {
		if _, ok := seen[id]; ok {
			return true
		}
	}
	return false
}

// middle finds where a shortest path through the edit graph of a[a0:a1]
// against b[b0:b1] crosses from the forward search into the backward one:
// the point (x, y) splits the problem in two. ok is false when the budget
// ran out first.
func (d *differ) middle(a0, a1, b0, b1 int) (x, y int, ok bool) {
	n, m := a1-a0, b1-b0
	maxD := (n + m + 1) / 2
	off := maxD + 1
	size := 2*off + 1
	if cap(d.vf) < size {
		d.vf, d.vb = make([]int, size), make([]int, size)
	}
	vf, vb := d.vf[:size], d.vb[:size]
	for i := range vf {
		vf[i], vb[i] = -1, -1
	}
	vf[off+1], vb[off+1] = 0, 0
	delta := n - m
	odd := delta%2 != 0
	for e := 0; e <= maxD; e++ {
		if d.budget >= 0 {
			d.budget -= 2*e + 1
			if d.budget < 0 {
				return 0, 0, false
			}
		}
		// forward: vf[k] is the furthest x on diagonal k = x - y
		for k := -e; k <= e; k += 2 {
			var px int
			if k == -e || (k != e && vf[off+k-1] < vf[off+k+1]) {
				px = vf[off+k+1] // down: an insertion
			} else {
				px = vf[off+k-1] + 1 // right: a deletion
			}
			py := px - k
			for px < n && py < m && d.a[a0+px] == d.b[b0+py] {
				px++
				py++
			}
			vf[off+k] = px
			if odd {
				// the backward search on the same diagonal
				kb := delta - k
				if kb >= -(e-1) && kb <= e-1 && vb[off+kb] >= 0 && px+vb[off+kb] >= n {
					return a0 + px, b0 + py, true
				}
			}
		}
		// backward: vb[k] is how far from the end, on diagonal k counted
		// from the bottom-right corner
		for k := -e; k <= e; k += 2 {
			var px int
			if k == -e || (k != e && vb[off+k-1] < vb[off+k+1]) {
				px = vb[off+k+1]
			} else {
				px = vb[off+k-1] + 1
			}
			py := px - k
			for px < n && py < m && d.a[a1-px-1] == d.b[b1-py-1] {
				px++
				py++
			}
			vb[off+k] = px
			if !odd {
				kf := delta - k
				if kf >= -e && kf <= e && vf[off+kf] >= 0 && vf[off+kf]+px >= n {
					fx := vf[off+kf]
					return a0 + fx, b0 + fx - kf, true
				}
			}
		}
	}
	return 0, 0, false
}

// Stats counts the lines a script adds and removes.
func Stats(edits []Edit) (added, removed int) {
	for _, e := range edits {
		switch e.Op {
		case Insert:
			added++
		case Delete:
			removed++
		}
	}
	return added, removed
}

// Hunk is a run of changes with the lines of context around it: lines
// AStart..AStart+ALen of a against BStart..BStart+BLen of b (0-based).
type Hunk struct {
	AStart, ALen, BStart, BLen int
	Edits                      []Edit
}

// Hunks groups a script into hunks, each change with up to context equal
// lines around it; changes closer than 2·context lines share a hunk.
func Hunks(edits []Edit, context int) []Hunk {
	context = max(context, 0)
	var changes []int
	for i, e := range edits {
		if e.Op != Equal {
			changes = append(changes, i)
		}
	}
	var out []Hunk
	prevEnd := 0
	for c := 0; c < len(changes); {
		first, last := changes[c], changes[c]
		c++
		// changes separated by at most 2·context equal lines share a hunk
		for c < len(changes) && changes[c]-last-1 <= 2*context {
			last = changes[c]
			c++
		}
		start := max(first-context, prevEnd)
		end := min(last+1+context, len(edits))
		prevEnd = end
		h := Hunk{Edits: edits[start:end]}
		h.AStart, h.BStart = edits[start].A, edits[start].B
		for _, e := range h.Edits {
			if e.Op != Insert {
				h.ALen++
			}
			if e.Op != Delete {
				h.BLen++
			}
		}
		out = append(out, h)
	}
	return out
}

// Unified writes the script as a unified diff (diff -u) with context lines;
// the empty string when a and b are the same.
func Unified(aName, bName string, a, b []string, context int) string {
	hs := Hunks(Lines(a, b), context)
	if len(hs) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", aName, bName)
	for _, h := range hs {
		fmt.Fprintf(&sb, "@@ -%s +%s @@\n", rangeOf(h.AStart, h.ALen), rangeOf(h.BStart, h.BLen))
		for _, e := range h.Edits {
			line := ""
			switch e.Op {
			case Equal:
				sb.WriteByte(' ')
				line = a[e.A]
			case Delete:
				sb.WriteByte('-')
				line = a[e.A]
			case Insert:
				sb.WriteByte('+')
				line = b[e.B]
			}
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// rangeOf is a unified diff's range: 1-based start, and the length when it
// is not 1 (an empty range names the line before it).
func rangeOf(start, n int) string {
	switch n {
	case 0:
		return fmt.Sprintf("%d,0", start)
	case 1:
		return fmt.Sprintf("%d", start+1)
	}
	return fmt.Sprintf("%d,%d", start+1, n)
}

// Row is one line of a side-by-side view: A is the line of a on the left,
// B the line of b on the right, -1 for a blank side. Op is Equal for an
// unchanged line, Delete for a line only on the left, Insert for a line
// only on the right; a changed line has both sides and Op Delete.
type Row struct {
	Op   Op
	A, B int
}

// Changed reports whether the row shows a line that was replaced (both
// sides, different).
func (r Row) Changed() bool { return r.Op != Equal && r.A >= 0 && r.B >= 0 }

// SideBySide lays a script out in rows: equal lines face each other, and in
// each block of changes the deleted lines face the inserted ones in order,
// the longer side running on against blanks.
func SideBySide(edits []Edit) []Row {
	out := make([]Row, 0, len(edits))
	for i := 0; i < len(edits); {
		if edits[i].Op == Equal {
			out = append(out, Row{Equal, edits[i].A, edits[i].B})
			i++
			continue
		}
		var dels, ins []int
		for i < len(edits) && edits[i].Op != Equal {
			if edits[i].Op == Delete {
				dels = append(dels, edits[i].A)
			} else {
				ins = append(ins, edits[i].B)
			}
			i++
		}
		for k := 0; k < max(len(dels), len(ins)); k++ {
			r := Row{Op: Delete, A: -1, B: -1}
			if k < len(dels) {
				r.A = dels[k]
			}
			if k < len(ins) {
				r.B = ins[k]
				if k >= len(dels) {
					r.Op = Insert
				}
			}
			out = append(out, r)
		}
	}
	return out
}

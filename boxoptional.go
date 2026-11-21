// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// AddOptional appends w with a fixed height, to be left out when the box is too
// short for it. w is laid out only while the box's flexible children keep at
// least keep pixels between them (their share, gaps excluded); otherwise it, and
// every optional child added after it, gets an empty Rect and takes no gap.
//
// It is what a host needs when a column holds rows that are nice to have above a
// list: the rows leave as the column shrinks and the list keeps its room, without
// the host working out how many rows fit. Optional children are judged in the
// order they were added, so an earlier one outranks a later one; a different keep
// per child lets some rows outlast others.
func (v *VBox) AddOptional(w Widget, size, keep int) {
	v.children = append(v.children, boxChild{w: w, size: max(size, 0), optional: true, keep: keep})
	v.SetBounds(v.Bounds())
}

// AddOptional is [VBox.AddOptional] for a fixed width.
func (h *HBox) AddOptional(w Widget, size, keep int) {
	h.children = append(h.children, boxChild{w: w, size: max(size, 0), optional: true, keep: keep})
	h.SetBounds(h.Bounds())
}

// boxShed drops the optional children the main-axis extent total has no room for
// and returns the others, in order. A dropped child is given an empty Rect, so it
// neither draws over nor takes a click meant for its neighbour.
//
// A child stays while, with it and every earlier optional child, the flexible
// children still keep their keep pixels; the first that fails ends the run, so
// the rows that remain are always the first ones.
func boxShed(children []boxChild, total, spacing int) []boxChild {
	has := false
	for _, c := range children {
		if c.optional {
			has = true
			break
		}
	}
	if !has {
		return children
	}
	n, fixed := 0, 0 // the children that stay for certain, and their fixed sizes
	for _, c := range children {
		if !c.optional {
			n++
			if c.flex == 0 {
				fixed += c.size
			}
		}
	}
	stay := make(map[int]bool)
	stopped := false
	for i, c := range children {
		if !c.optional || stopped {
			continue
		}
		if total-spacing*n-(fixed+c.size) < c.keep { // n is the count before c: c adds one gap
			stopped = true
			continue
		}
		n++
		fixed += c.size
		stay[i] = true
	}
	out := make([]boxChild, 0, len(children))
	for i, c := range children {
		if c.optional && !stay[i] {
			c.w.SetBounds(Rect{})
			continue
		}
		out = append(out, c)
	}
	return out
}

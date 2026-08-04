// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// scrollExtreme overshoots far past either end of any list. Because a clamped
// pixel/row scroll pins to its valid range, adding or subtracting it lands
// exactly on the last or first position — the Home / End behaviour, without
// the caller having to report its length. Used by ScrollView's key handling.
const scrollExtreme = 1 << 30

// rovingIndex computes where a data widget's keyboard cursor moves for a
// roving-cursor navigation key, over a visible list of n rows with the given
// page step, starting from cur (-1 = no cursor yet). It reports the new index
// and whether code was one of the navigation keys it handles:
//
//	ArrowUp / ArrowDown   one row up / down (clamped, no wrap)
//	PageUp  / PageDown    one page up / down (page = visible rows)
//	Home    / End         jump to the first / last row
//
// A first ArrowUp/ArrowDown with no cursor lands on the first row; the Page
// keys treat a missing cursor as the first row too. A key that isn't a
// navigation key returns (cur, false), leaving the caller free to handle it
// (Enter/Space activation, tree Left/Right, …). n <= 0 always returns
// (cur, false): an empty list has nowhere for the cursor to go.
func rovingIndex(cur, n, page int, code string) (int, bool) {
	if n <= 0 {
		return cur, false
	}
	base := cur
	if base < 0 {
		base = 0
	}
	switch code {
	case "ArrowDown":
		if cur < 0 {
			return 0, true
		}
		return min(cur+1, n-1), true
	case "ArrowUp":
		if cur < 0 {
			return 0, true
		}
		return max(cur-1, 0), true
	case "PageDown":
		return min(base+page, n-1), true
	case "PageUp":
		return max(base-page, 0), true
	case "Home":
		return 0, true
	case "End":
		return n - 1, true
	default:
		return cur, false
	}
}

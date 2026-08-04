// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// rowScroller is implemented by every widget that scrolls its content by
// whole rows through a clamped ScrollBy (ListBox, Table, TreeTable,
// TreeView). The native wheel + keyboard scroll handlers drive them through
// this single interface so the scroll-input policy lives in one place
// instead of being re-derived in each widget's OnEvent.
type rowScroller interface {
	// ScrollBy shifts the top visible row by delta (positive scrolls down /
	// forward, negative up / back), clamped to the valid range at both ends.
	ScrollBy(delta int)
}

// scrollExtreme overshoots far past either end of any list. Because ScrollBy
// clamps, adding or subtracting it lands exactly on the last or first row —
// the Home / End behaviour, without each widget having to report its length.
const scrollExtreme = 1 << 30

// handleScrollKey applies the standard scroll-navigation keybindings to a
// row-scrolling widget and reports whether code was one of them:
//
//	ArrowUp / ArrowDown   one row up / down
//	PageUp  / PageDown    one page up / down (page = visible rows)
//	Home    / End         jump to the first / last row
//
// page is the widget's current visible-row count (the page step). A key that
// isn't a scroll key returns false, leaving the caller free to fall through
// to its own key handling.
func handleScrollKey(s rowScroller, code string, page int) bool {
	switch code {
	case "ArrowUp":
		s.ScrollBy(-1)
	case "ArrowDown":
		s.ScrollBy(1)
	case "PageUp":
		s.ScrollBy(-page)
	case "PageDown":
		s.ScrollBy(page)
	case "Home":
		s.ScrollBy(-scrollExtreme)
	case "End":
		s.ScrollBy(scrollExtreme)
	default:
		return false
	}
	return true
}

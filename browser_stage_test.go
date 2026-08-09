// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestBrowserDeliverStageProgressive covers the staged-delivery contract: an
// intermediate frame updates content but keeps loading on and preserves the
// scroll position; the final frame clears loading; a new navigation resets the
// scroll.
func TestBrowserDeliverStageProgressive(t *testing.T) {
	b, _, _, _ := newTestBrowser()
	b.Open("http://a", "A") // startLoad: loading on, scroll reset
	cr := b.contentRect()
	tab := b.activeTab()
	if !tab.loading {
		t.Fatal("navigation should mark the tab loading")
	}

	tall := make([]byte, cr.W*(cr.H*3)*4)

	// Intermediate frame: content lands, loading stays on.
	b.DeliverStage("http://a", tall, cr.W, cr.H*3, cr.W, nil, "A", false)
	if !tab.loading {
		t.Error("an intermediate (final=false) frame must keep loading on")
	}
	if tab.imgH != cr.H*3 {
		t.Error("intermediate frame content was not stored")
	}

	// The user scrolls during the staged load; a further intermediate frame must
	// preserve that scroll (refine in place, not snap to top).
	tab.scroll = 30
	b.DeliverStage("http://a", tall, cr.W, cr.H*3, cr.W, nil, "A", false)
	if tab.scroll != 30 {
		t.Errorf("intermediate frame reset scroll to %d, want preserved 30", tab.scroll)
	}

	// The final frame clears loading and still preserves the scroll.
	b.DeliverStage("http://a", tall, cr.W, cr.H*3, cr.W, nil, "A", true)
	if tab.loading {
		t.Error("a final (final=true) frame must clear loading")
	}
	if tab.scroll != 30 {
		t.Errorf("final frame reset scroll to %d, want preserved 30", tab.scroll)
	}

	// Deliver (the convenience final form) also preserves scroll.
	b.Deliver("http://a", tall, cr.W, cr.H*3, cr.W, nil, "A")
	if tab.scroll != 30 || tab.loading {
		t.Errorf("Deliver: scroll=%d loading=%v, want 30/false", tab.scroll, tab.loading)
	}

	// A NEW navigation resets both scroll offsets back to the top.
	tab.scrollX = 12
	b.Navigate("http://a/next")
	if tab.scroll != 0 || tab.scrollX != 0 {
		t.Errorf("navigation left scroll=%d scrollX=%d, want 0/0", tab.scroll, tab.scrollX)
	}
}

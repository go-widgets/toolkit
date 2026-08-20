// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// ChildOffset must describe where the content is PAINTED, rubber band included:
// an accessibility bridge reads it between frames, and telling it the content
// sits at the bound while it is visibly past would put a control's announced
// position away from its drawn one.
func TestChildOffsetIncludesTheRubberBand(t *testing.T) {
	sv := newPanScrollView()
	if dx, dy := sv.ChildOffset(); dx != 0 || dy != 0 {
		t.Fatalf("at rest ChildOffset = (%d,%d), want (0,0)", dx, dy)
	}

	// Drag well past the start so the band stretches.
	sv.OnEvent(Event{Kind: EventClick, X: 40, Y: 60})
	sv.OnEvent(Event{Kind: EventMouseDrag, X: 40, Y: 4000})
	_, over := sv.Overscroll()
	if over >= 0 {
		t.Fatalf("overscroll=%d, want a negative stretch past the start", over)
	}
	_, dy := sv.ChildOffset()
	if want := -(sv.OffsetY().Get() + over); dy != want {
		t.Fatalf("ChildOffset y = %d, want %d (offset %d + overscroll %d)",
			dy, want, sv.OffsetY().Get(), over)
	}
	// And it is not merely the offset, which is what the bug would look like.
	if dy == -sv.OffsetY().Get() {
		t.Fatal("ChildOffset ignored the rubber band")
	}
}

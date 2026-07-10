// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestSplitDropPayload(t *testing.T) {
	// Empty payload yields no items (nil).
	if got := SplitDropPayload(""); got != nil {
		t.Errorf("SplitDropPayload(\"\") = %v, want nil", got)
	}
	// Single item.
	if got := SplitDropPayload("/a"); len(got) != 1 || got[0] != "/a" {
		t.Errorf("single = %v, want [/a]", got)
	}
	// Multiple items, with a trailing separator and a blank line that must be
	// dropped rather than becoming a phantom "" item.
	got := SplitDropPayload("/a\n/b\n\n/c\n")
	want := []string{"/a", "/b", "/c"}
	if len(got) != len(want) {
		t.Fatalf("multi = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("multi[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestJoinDropPayloadRoundTrips(t *testing.T) {
	items := []string{"/x", "/y", "/z"}
	if got := SplitDropPayload(JoinDropPayload(items)); len(got) != 3 ||
		got[0] != "/x" || got[2] != "/z" {
		t.Errorf("round-trip = %v, want %v", got, items)
	}
}

// dragThing is a minimal DragSource used only to prove the interface is
// implementable and its DragData is reachable through the interface.
type dragThing struct {
	Base
	data string
}

func (d *dragThing) DragData() string { return d.data }

var _ DragSource = (*dragThing)(nil)

func TestDragSourceContract(t *testing.T) {
	var src DragSource = &dragThing{data: "/payload"}
	if src.DragData() != "/payload" {
		t.Errorf("DragData() = %q, want /payload", src.DragData())
	}
	// It is a Widget too (Base supplies Bounds), so a container can lay it out.
	src.SetBounds(Rect{X: 1, Y: 2, W: 3, H: 4})
	if src.Bounds().W != 3 {
		t.Errorf("DragSource is not a usable Widget: %+v", src.Bounds())
	}
}

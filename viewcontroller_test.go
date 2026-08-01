// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// otherProbe is a second widget type, to exercise LookupAs's wrong-type path.
type otherProbe struct{ Base }

// TestViewController checks a ViewController collects Ref'd widgets while building,
// exposes the root, and looks widgets up by name (untyped and typed), including
// the missing-name and wrong-type paths. The tree mixes Ref'd and un-Ref'd nodes.
func TestViewController(t *testing.T) {
	list, save := &dockProbe{}, &dockProbe{}
	vc := NewViewController(VBoxNode(
		Leaf(list).Ref("list").Flexed(1),
		Leaf(save).Ref("save").Sized(32),
		Leaf(&dockProbe{}), // un-Ref'd: exercises the ref=="" branch
	))

	// Root builds and lays out.
	vc.Root().SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	if vc.Root() == nil {
		t.Fatal("Root is nil")
	}
	if list.Bounds().W == 0 {
		t.Fatalf("list not laid out: %+v", list.Bounds())
	}

	// Untyped lookup: present and missing.
	if vc.Lookup("list") != Widget(list) {
		t.Fatalf("Lookup(list) = %v, want the list widget", vc.Lookup("list"))
	}
	if vc.Lookup("nope") != nil {
		t.Fatal("Lookup of a missing name should be nil")
	}

	// Typed lookup: correct type.
	if got, ok := LookupAs[*dockProbe](vc, "save"); !ok || got != save {
		t.Fatalf("LookupAs[*dockProbe](save) = %v,%v", got, ok)
	}
	// Typed lookup: missing name.
	if _, ok := LookupAs[*dockProbe](vc, "nope"); ok {
		t.Fatal("LookupAs of a missing name should report ok=false")
	}
	// Typed lookup: wrong type (the ref holds a *dockProbe, not *otherProbe).
	if _, ok := LookupAs[*otherProbe](vc, "list"); ok {
		t.Fatal("LookupAs with the wrong type should report ok=false")
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// ViewController builds a
// declarative Node tree once and then lets logic reach the widgets that matter by
// name, instead of threading pointers through the construction code. Tag nodes
// with Node.Ref("name"); the controller collects those widgets while building and
// exposes them via Lookup / the typed LookupAs. Event handlers are wired the
// Go-idiomatic way — look a widget up and assign its callback — rather than by
// resolving handler-name strings, so the compiler checks every wire.
//
//	vc := NewViewController(VBoxNode(
//		Leaf(list).Ref("list").Flexed(1),
//		Leaf(saveBtn).Ref("save").Sized(32),
//	))
//	if b, ok := LookupAs[*Button](vc, "save"); ok {
//		b.OnClick = onSave
//	}
//	vc.Root().SetBounds(screen)
type ViewController struct {
	root  Widget
	refs  map[string]Widget
	order []string // ref names in build order, for deterministic RefAt
}

// NewViewController builds root and collects every Ref-tagged widget in the tree.
func NewViewController(root Node) *ViewController {
	vc := &ViewController{refs: map[string]Widget{}}
	vc.root = root.build(func(name string, w Widget) {
		if _, dup := vc.refs[name]; !dup {
			vc.order = append(vc.order, name)
		}
		vc.refs[name] = w
	})
	return vc
}

// Root is the built root widget — call SetBounds/Draw/OnEvent on it.
func (vc *ViewController) Root() Widget { return vc.root }

// Lookup returns the widget tagged with Ref(name), or nil if there is none.
func (vc *ViewController) Lookup(name string) Widget { return vc.refs[name] }

// LookupAs returns the widget tagged with Ref(name) as type T. ok is false when
// the name is absent or the widget is not a T — the typed lookup by name.
func LookupAs[T Widget](vc *ViewController, name string) (val T, ok bool) {
	val, ok = vc.refs[name].(T)
	return val, ok
}

// RefAt returns the name of the Ref-tagged widget whose Bounds contains the
// surface point (px,py) — hit-testing by reference. Refs are scanned in reverse
// build order, so a later (typically deeper or on-top) ref shadows an earlier one
// when they overlap. ok is false when no ref-tagged widget covers the point.
func (vc *ViewController) RefAt(px, py int) (name string, ok bool) {
	for i := len(vc.order) - 1; i >= 0; i-- {
		if vc.refs[vc.order[i]].Bounds().Contains(px, py) {
			return vc.order[i], true
		}
	}
	return "", false
}

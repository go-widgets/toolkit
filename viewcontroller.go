// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// ViewController is the Go-native analog of Ext.app.ViewController: it builds a
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
	root Widget
	refs map[string]Widget
}

// NewViewController builds root and collects every Ref-tagged widget in the tree.
func NewViewController(root Node) *ViewController {
	refs := map[string]Widget{}
	w := root.build(refs)
	return &ViewController{root: w, refs: refs}
}

// Root is the built root widget — call SetBounds/Draw/OnEvent on it.
func (vc *ViewController) Root() Widget { return vc.root }

// Lookup returns the widget tagged with Ref(name), or nil if there is none.
func (vc *ViewController) Lookup(name string) Widget { return vc.refs[name] }

// LookupAs returns the widget tagged with Ref(name) as type T. ok is false when
// the name is absent or the widget is not a T — the typed lookupReference.
func LookupAs[T Widget](vc *ViewController, name string) (val T, ok bool) {
	val, ok = vc.refs[name].(T)
	return val, ok
}

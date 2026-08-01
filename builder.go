// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// This file is the Go-native declarative view builder: describe a UI as a tree of
// Node values — a type-safe component-config tree ({widget,
// layout, children:[…]}) — and Build() instantiates the Container/widget tree. No
// JSON, no reflection, no string xtypes: the tree is ordinary Go values, so it is
// checked by the compiler and you keep direct references to the leaf widgets you
// pass in.
//
//	ui := BorderNode(
//		Leaf(topbar).At(RegionNorth).Sized(44),
//		Leaf(sidebar).At(RegionWest).Sized(200),
//		HBoxNode(
//			Leaf(list).Sized(240),
//			Leaf(detail).Flexed(1),
//		).At(RegionCenter),
//	).Build()

// Node describes one piece of a UI tree. It is either a leaf (Widget set) or a
// container (Layout + Children). Flex/Size/Region are the layout configuration
// this node contributes to ITS PARENT's layout (the parent reads them when it
// adds this node as an Item); they are ignored on the root.
type Node struct {
	Widget   Widget // leaf content; when set, Layout/Children are ignored
	Layout   Layout // container content: how Children are arranged
	Children []Node

	Flex   int    // parent box layout: proportional weight
	Size   int    // parent box layout: fixed main-axis size; border: band thickness
	Region Region // parent border layout: which region this node occupies

	ref string // lookup name for a ViewController (see Ref)
}

// Build instantiates the node into a Widget: a leaf returns its Widget as-is; a
// non-leaf builds a *Container with the node's Layout and its recursively-built
// children (each added with its parent-layout config).
func (n Node) Build() Widget { return n.build(nil) }

// build is Build with an optional reference sink: when collect is non-nil, every
// node carrying a Ref name reports its built widget through it (used by
// ViewController, which records both the widget and the ref order).
func (n Node) build(collect func(name string, w Widget)) Widget {
	var w Widget
	if n.Widget != nil {
		w = n.Widget
	} else {
		c := NewContainer(n.Layout)
		for _, ch := range n.Children {
			c.Add(Item{Widget: ch.build(collect), Flex: ch.Flex, Size: ch.Size, Region: ch.Region})
		}
		w = c
	}
	if collect != nil && n.ref != "" {
		collect(n.ref, w)
	}
	return w
}

// Flexed sets this node's parent-box flex weight and returns it (fluent).
func (n Node) Flexed(flex int) Node { n.Flex = flex; return n }

// Sized sets this node's fixed size (box main-axis extent, or border band
// thickness) and returns it (fluent).
func (n Node) Sized(size int) Node { n.Size = size; return n }

// At sets this node's border region and returns it (fluent).
func (n Node) At(region Region) Node { n.Region = region; return n }

// Ref tags this node with a lookup name so a ViewController built from the tree
// can retrieve its widget by name (a named reference). Empty names
// are ignored.
func (n Node) Ref(name string) Node { n.ref = name; return n }

// Leaf wraps a widget as a leaf node.
func Leaf(w Widget) Node { return Node{Widget: w} }

// FitNode builds a container node whose children each fill it (FitLayout).
func FitNode(children ...Node) Node { return Node{Layout: FitLayout{}, Children: children} }

// HBoxNode builds a horizontal box container node. Like NewHBox it seeds the
// DefaultBoxSpacing gutter (via NewBoxLayout).
func HBoxNode(children ...Node) Node { return Node{Layout: NewBoxLayout(), Children: children} }

// VBoxNode builds a vertical box container node. Like NewVBox it seeds the
// DefaultBoxSpacing gutter.
func VBoxNode(children ...Node) Node {
	l := NewBoxLayout()
	l.Vertical = true
	return Node{Layout: l, Children: children}
}

// BorderNode builds a border container node; children carry their Region via At.
func BorderNode(children ...Node) Node { return Node{Layout: BorderLayout{}, Children: children} }

// CardNode builds a card container node showing the child at active.
func CardNode(active int, children ...Node) Node {
	return Node{Layout: &CardLayout{Active: active}, Children: children}
}

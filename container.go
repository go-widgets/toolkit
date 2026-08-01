// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// This file brings Sencha's container/layout operating model to the toolkit: a
// Container holds a list of Items and delegates their placement to a swappable
// Layout, so the same children can be arranged as a fit, a box, a border, or a
// card stack by changing one field — the way Ext JS separates Ext.container.
// Container from its layout config. The existing HBox/VBox/Border/Dock stay; this
// is the composable, config-driven alternative built on the same primitives.

// Region names a Border layout edge (or the centre) an Item occupies. The zero
// value RegionCenter fills what the edge regions leave.
type Region int

const (
	RegionCenter Region = iota
	RegionNorth
	RegionSouth
	RegionWest
	RegionEast
)

// Item wraps a widget with its per-layout configuration — the analog of an Ext
// child component's layout config. Box layouts read Flex/Size (a positive Flex is
// a proportional weight; else a positive Size is a fixed main-axis extent; both
// zero means an equal flex share, like HBox.Append). Border layouts read Region
// (with Size the edge band's thickness). Fit/Card ignore the config.
type Item struct {
	Widget Widget
	Flex   int
	Size   int
	Region Region
}

// boxSpec resolves an Item's flex weight and fixed size for a box layout: a
// positive Flex wins, else a positive Size is fixed, else an equal flex share.
func (it Item) boxSpec() (flex, size int) {
	if it.Flex > 0 {
		return it.Flex, 0
	}
	if it.Size > 0 {
		return 0, it.Size
	}
	return 1, 0
}

// Layout positions a container's items within its content rectangle. Swapping the
// Layout re-arranges the same items — the heart of the Ext operating model.
type Layout interface {
	Arrange(r Rect, items []Item)
}

// FitLayout sizes every item to fill the container (typically one item, e.g. a
// card body). Ext's fit layout.
type FitLayout struct{}

// Arrange fills each item to the container bounds.
func (FitLayout) Arrange(r Rect, items []Item) {
	for _, it := range items {
		it.Widget.SetBounds(r)
	}
}

// BoxLayout stacks items along one axis (horizontal by default; set Vertical for
// a column), honouring per-item Flex/Size and the Align/Pack options — Ext's
// hbox/vbox. It reuses the same sizing/alignment primitives as HBox/VBox.
type BoxLayout struct {
	Vertical bool
	Spacing  int
	Align    BoxAlign
	Pack     BoxPack
}

// Arrange lays the items out along the box axis.
func (l *BoxLayout) Arrange(r Rect, items []Item) {
	if len(items) == 0 {
		return
	}
	children := make([]boxChild, len(items))
	for i, it := range items {
		flex, size := it.boxSpec()
		children[i] = boxChild{w: it.Widget, flex: flex, size: size}
	}
	total := r.W
	if l.Vertical {
		total = r.H
	}
	sp := boxSpacing(l.Spacing)
	sizes := boxCells(total, sp, children)
	pos := r.X
	if l.Vertical {
		pos = r.Y
	}
	pos += boxLead(l.Pack, boxSlack(total, sp, sizes))
	for i, it := range items {
		if l.Vertical {
			x, cw := boxCross(it.Widget, l.Align, r.X, r.W, sizes[i], false)
			it.Widget.SetBounds(Rect{X: x, Y: pos, W: cw, H: sizes[i]})
		} else {
			y, ch := boxCross(it.Widget, l.Align, r.Y, r.H, sizes[i], true)
			it.Widget.SetBounds(Rect{X: pos, Y: y, W: sizes[i], H: ch})
		}
		pos += sizes[i] + sp
	}
}

// BorderLayout arranges items by Region: North/South span the full width, then
// West/East span the height between them, then Center fills the rest. Item.Size
// is the edge band's thickness. Ext's border layout, over the shared dockCarve.
type BorderLayout struct{}

// regionSide maps a border Region to the Dock edge dockCarve understands.
func regionSide(r Region) DockSide {
	switch r {
	case RegionSouth:
		return DockBottom
	case RegionWest:
		return DockLeft
	case RegionEast:
		return DockRight
	default: // RegionNorth
		return DockTop
	}
}

// Arrange carves the edge regions off in N,S,W,E order, then fills Center.
func (BorderLayout) Arrange(r Rect, items []Item) {
	avail := r
	for _, side := range []Region{RegionNorth, RegionSouth, RegionWest, RegionEast} {
		for _, it := range items {
			if it.Region != side {
				continue
			}
			size := it.Size
			if size < 0 {
				size = 0
			}
			var bar Rect
			bar, avail = dockCarve(regionSide(side), size, avail)
			it.Widget.SetBounds(bar)
		}
	}
	for _, it := range items {
		if it.Region == RegionCenter {
			it.Widget.SetBounds(avail)
		}
	}
}

// CardLayout shows exactly one item — the one at Active — filling the container,
// and collapses the rest to an empty rectangle so the Container skips them. Ext's
// card layout (wizards, tab bodies, view switching).
type CardLayout struct {
	Active int
}

// Arrange fills the active item and empties the others.
func (l *CardLayout) Arrange(r Rect, items []Item) {
	for i, it := range items {
		if i == l.Active {
			it.Widget.SetBounds(r)
		} else {
			it.Widget.SetBounds(Rect{})
		}
	}
}

// Container holds a list of Items and positions them via its Layout. It is a
// Widget: Draw paints every item with a non-empty rectangle (so a CardLayout's
// inactive cards and collapsed box cells are skipped), and OnEvent routes by
// Bounds into the matched item's local space.
type Container struct {
	Base
	Layout Layout
	items  []Item
}

// NewContainer builds a Container with the given layout (nil = items keep the
// bounds they are given). Add items with Add/AddWidget.
func NewContainer(layout Layout) *Container { return &Container{Layout: layout} }

// Add appends a configured item and re-arranges. Returns the container for
// fluent, declarative construction.
func (c *Container) Add(it Item) *Container {
	c.items = append(c.items, it)
	c.relayout()
	return c
}

// AddWidget appends a plain widget with the zero item config (equal flex share in
// a box, centre in a border, a fit/card cell otherwise).
func (c *Container) AddWidget(w Widget) *Container { return c.Add(Item{Widget: w}) }

// Items returns the container's items in insertion order.
func (c *Container) Items() []Item { return c.items }

// SetItems replaces every item and re-arranges — the seam a data-driven view uses
// to rebuild its children (e.g. an mvvm ObservableList binding). SetItems() with
// no arguments clears the container. Returns the container for chaining.
func (c *Container) SetItems(items ...Item) *Container {
	c.items = items
	c.relayout()
	return c
}

// relayout re-runs the layout over the current bounds (a no-op with no layout).
func (c *Container) relayout() {
	if c.Layout != nil {
		c.Layout.Arrange(c.Bounds(), c.items)
	}
}

// SetBounds positions the container and re-arranges its items.
func (c *Container) SetBounds(r Rect) {
	c.Base.SetBounds(r)
	c.relayout()
}

// Draw paints every item that has a non-empty rectangle.
func (c *Container) Draw(p painter.Painter, theme *Theme) {
	for _, it := range c.items {
		if b := it.Widget.Bounds(); b.W > 0 && b.H > 0 {
			it.Widget.Draw(p, theme)
		}
	}
}

// OnEvent forwards to the first non-empty item whose Bounds contains the point,
// translated into that item's local space.
func (c *Container) OnEvent(ev Event) {
	pr := c.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, it := range c.items {
		b := it.Widget.Bounds()
		if b.W > 0 && b.H > 0 && b.Contains(sx, sy) {
			it.Widget.OnEvent(translateEvent(ev, pr, b))
			return
		}
	}
}

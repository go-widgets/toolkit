// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// #578: a Disabled list, grid or card list takes no input and announces itself
// as disabled.

func TestVirtualListDisabledIgnoresWheel(t *testing.T) {
	v := NewVirtualList[int](mvvm.NewObservableList[int](intItems(100)...), func(int) int { return 10 },
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	v.SetBounds(toolkit.Rect{W: 100, H: 100})
	wheel := toolkit.Event{Kind: toolkit.EventScroll, Delta: 3}

	v.Disabled().Set(true)
	v.OnEvent(wheel)
	if v.ScrollOffset != 0 {
		t.Fatalf("disabled list scrolled to %d", v.ScrollOffset)
	}
	if info := v.A11y(); !info.Disabled || info.Role != toolkit.RoleList {
		t.Fatalf("disabled list A11y = %+v", info)
	}
	v.Disabled().Set(false)
	v.OnEvent(wheel)
	if v.ScrollOffset != 30 {
		t.Fatalf("re-enabled list: offset %d, want 30", v.ScrollOffset)
	}
	if v.A11y().Disabled {
		t.Fatal("re-enabled list still announces Disabled")
	}
}

func TestVirtualGridDisabledIgnoresWheel(t *testing.T) {
	g := NewVirtualGrid[int](mvvm.NewObservableList[int](intItems(30)...), toolkit.Size{W: 50, H: 40},
		func(painter.Painter, *toolkit.Theme, toolkit.Rect, int, int) {})
	g.SetBounds(toolkit.Rect{W: 150, H: 100})
	wheel := toolkit.Event{Kind: toolkit.EventScroll, Delta: 2}

	g.Disabled().Set(true)
	g.OnEvent(wheel)
	if g.ScrollOffset != 0 {
		t.Fatalf("disabled grid scrolled to %d", g.ScrollOffset)
	}
	if info := g.A11y(); !info.Disabled || info.Role != toolkit.RoleGrid {
		t.Fatalf("disabled grid A11y = %+v", info)
	}
	g.Disabled().Set(false)
	g.OnEvent(wheel)
	if g.ScrollOffset != 80 {
		t.Fatalf("re-enabled grid: offset %d, want 80", g.ScrollOffset)
	}
	if g.A11y().Disabled {
		t.Fatal("re-enabled grid still announces Disabled")
	}
}

// Keyboard path: CardList has its own OnEvent, past VirtualList's, and moves a
// selection cursor from arrow, page and Enter keys. Disabled, none of it runs.
func TestCardListDisabledTakesNoKeysClicksOrWheel(t *testing.T) {
	c := newList(100, 100, 100)
	var selected, activated int
	c.OnSelect = func(int) { selected++ }
	c.OnActivate = func(int) { activated++ }
	events := []toolkit.Event{
		{Kind: toolkit.EventKeyDown, Code: "ArrowDown"},
		{Kind: toolkit.EventKeyDown, Code: "PageDown"},
		{Kind: toolkit.EventKeyDown, Code: "End"},
		{Kind: toolkit.EventKeyDown, Code: "Enter"},
		{Kind: toolkit.EventClick, X: 10, Y: 30},
		{Kind: toolkit.EventScroll, Delta: 3},
	}

	c.Disabled().Set(true)
	for _, ev := range events {
		c.OnEvent(ev)
	}
	if c.Selected != -1 || c.ScrollOffset != 0 || selected != 0 || activated != 0 {
		t.Fatalf("disabled card list moved: Selected=%d offset=%d select=%d activate=%d",
			c.Selected, c.ScrollOffset, selected, activated)
	}
	if !c.A11y().Disabled {
		t.Fatal("disabled card list does not announce Disabled")
	}

	c.Disabled().Set(false)
	for _, ev := range events {
		c.OnEvent(ev)
	}
	if c.Selected == -1 || selected == 0 || activated == 0 {
		t.Fatalf("re-enabled card list ignored its events: Selected=%d select=%d activate=%d", c.Selected, selected, activated)
	}
}

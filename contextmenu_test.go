// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// newTestContextMenu returns a context menu over a 200x200 surface with three
// items (Cut, a separator, Paste-with-shortcut) plus one submenu row.
func newTestContextMenu() (*ContextMenu, *[]string) {
	fired := &[]string{}
	menu := NewMenu([]MenuItem{
		{Label: "Cut", Action: func() { *fired = append(*fired, "Cut") }},
		{Separator: true},
		{Label: "Paste", Shortcut: "Ctrl+V", Action: func() { *fired = append(*fired, "Paste") }},
		{Label: "More", Submenu: NewMenu(nil)},
	})
	cm := NewContextMenu(menu)
	cm.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	return cm, fired
}

func TestContextMenuPopupOpens(t *testing.T) {
	cm, _ := newTestContextMenu()
	if cm.Open {
		t.Fatal("should start closed")
	}
	cm.Popup(30, 40)
	if !cm.Open || cm.AnchorX != 30 || cm.AnchorY != 40 {
		t.Fatalf("Popup did not open at anchor: open=%v (%d,%d)", cm.Open, cm.AnchorX, cm.AnchorY)
	}
	cm.Close()
	if cm.Open {
		t.Fatal("Close did not hide")
	}
}

func TestContextMenuSizeFloorsAndGrows(t *testing.T) {
	// A menu of tiny labels floors at ContextMenuMinW.
	small := NewContextMenu(NewMenu([]MenuItem{{Label: "a", Action: func() {}}}))
	if w, _ := small.menuSize(); w != ContextMenuMinW {
		t.Errorf("small width = %d, want floor %d", w, ContextMenuMinW)
	}
	// A genuinely wide label grows the width past the floor (the rowW > w arm).
	wide := NewContextMenu(NewMenu([]MenuItem{
		{Label: "Paste Special As Plain Text", Shortcut: "Ctrl+Shift+V", Action: func() {}},
	}))
	if w, _ := wide.menuSize(); w <= ContextMenuMinW {
		t.Errorf("wide menu width = %d, want > floor %d", w, ContextMenuMinW)
	}
	// Height: 3 rows * MenuRowH + 1 separator + 4 inset (the standard fixture).
	cm, _ := newTestContextMenu()
	if _, h := cm.menuSize(); h != 3*MenuRowH+MenuSeparatorH+4 {
		t.Errorf("height = %d, want %d", h, 3*MenuRowH+MenuSeparatorH+4)
	}
}

func TestContextMenuClampsInsideSurface(t *testing.T) {
	cm, _ := newTestContextMenu()
	// Anchor in the far bottom-right corner: the menu must shift up+left to stay
	// fully inside the 200x200 surface.
	cm.Popup(199, 199)
	mb := cm.MenuBounds()
	if mb.X+mb.W > 200 || mb.Y+mb.H > 200 {
		t.Errorf("menu spilled off bottom-right: %+v", mb)
	}
	// Anchor past the top-left origin clamps back to the surface origin.
	cm.AnchorX, cm.AnchorY = -50, -50
	mb = cm.MenuBounds()
	if mb.X < 0 || mb.Y < 0 {
		t.Errorf("menu spilled off top-left: %+v", mb)
	}
}

func TestContextMenuDrawOnlyWhenOpen(t *testing.T) {
	cm, _ := newTestContextMenu()
	surf := makeSurface(200, 200)
	// Closed: nothing painted (corner stays sentinel).
	cm.Draw(newP(surf, 200), DefaultLight())
	sentinel := RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 255}
	if got := pixelAt(surf, 200, 5, 5); got != sentinel {
		t.Errorf("closed menu painted at (5,5): %+v", got)
	}
	// Open: the menu body paints its Border frame somewhere.
	cm.Popup(10, 10)
	cm.Draw(newP(surf, 200), DefaultLight())
	if got := countInk(surf, 200, 200, DefaultLight().Border); got == 0 {
		t.Error("open menu drew no border")
	}
}

func TestContextMenuClickInsideFiresAndCloses(t *testing.T) {
	cm, fired := newTestContextMenu()
	cm.Popup(10, 10)
	mb := cm.MenuBounds()
	// Click the first row ("Cut"): its local Y is inside the first MenuRowH band
	// (body inset 2 + a few px). Convert to surface coords.
	localY := 2 + MenuRowH/2
	cm.OnEvent(Event{Kind: EventClick, X: mb.X + 10, Y: mb.Y + localY})
	if len(*fired) != 1 || (*fired)[0] != "Cut" {
		t.Fatalf("expected Cut to fire, got %v", *fired)
	}
	if cm.Open {
		t.Error("activating an item should close the menu (via OnClose)")
	}
}

func TestContextMenuClickOutsideDismisses(t *testing.T) {
	cm, fired := newTestContextMenu()
	cm.Popup(10, 10)
	mb := cm.MenuBounds()
	// Click well outside the menu rect → dismiss, no action.
	cm.OnEvent(Event{Kind: EventClick, X: mb.X + mb.W + 20, Y: mb.Y + mb.H + 20})
	if cm.Open {
		t.Error("outside click should dismiss")
	}
	if len(*fired) != 0 {
		t.Errorf("outside click fired an action: %v", *fired)
	}
}

func TestContextMenuIgnoresWhenClosedOrNonClick(t *testing.T) {
	cm, _ := newTestContextMenu()
	// Closed: any event is a no-op.
	cm.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	if cm.Open {
		t.Error("event on a closed menu should not open it")
	}
	// Open but non-click: ignored (menu stays open).
	cm.Popup(10, 10)
	cm.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if !cm.Open {
		t.Error("non-click event should not dismiss")
	}
}

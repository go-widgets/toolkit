// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// --- TextView ------------------------------------------------------------

func TestTextViewNewEmpty(t *testing.T) {
	v := NewTextView("")
	if len(v.Lines) != 1 || v.Lines[0] != "" {
		t.Fatalf("empty new: Lines = %v", v.Lines)
	}
}

func TestTextViewNewWithText(t *testing.T) {
	v := NewTextView("a\nb\nc")
	if len(v.Lines) != 3 || v.Lines[1] != "b" {
		t.Fatalf("new: Lines = %v", v.Lines)
	}
}

func TestTextViewTextRoundTrip(t *testing.T) {
	v := NewTextView("hello\nworld")
	if v.Text() != "hello\nworld" {
		t.Fatalf("Text() = %q", v.Text())
	}
}

func TestTextViewSetTextEmpty(t *testing.T) {
	v := NewTextView("abc\ndef")
	v.SetText("")
	if len(v.Lines) != 1 || v.Lines[0] != "" || v.CursorLine != 0 || v.CursorCol != 0 {
		t.Fatalf("SetText(\"\") didn't reset")
	}
}

func TestTextViewSetTextNonEmpty(t *testing.T) {
	v := NewTextView("abc")
	v.SetText("one\ntwo")
	if len(v.Lines) != 2 || v.Lines[1] != "two" {
		t.Fatalf("SetText: %v", v.Lines)
	}
}

func TestTextViewClickFocuses(t *testing.T) {
	v := NewTextView("a")
	v.OnEvent(Event{Kind: EventClick})
	if !v.Focused {
		t.Fatal("click should focus")
	}
}

func TestTextViewCharInsertsAndFiresOnChange(t *testing.T) {
	changes := 0
	v := NewTextView("ab")
	v.OnChange = func() { changes++ }
	v.CursorCol = 1
	v.OnEvent(Event{Kind: EventChar, Code: "X"})
	if v.Lines[0] != "aXb" || v.CursorCol != 2 || changes != 1 {
		t.Fatalf("char insert: %v cursor=%d changes=%d", v.Lines, v.CursorCol, changes)
	}
}

func TestTextViewCharWithNewlineSplitsLine(t *testing.T) {
	v := NewTextView("abc")
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventChar, Code: "x\ny"})
	if len(v.Lines) != 2 || v.Lines[0] != "abx" || v.Lines[1] != "yc" {
		t.Fatalf("split-on-newline: %v", v.Lines)
	}
}

func TestTextViewEmptyCharNoOp(t *testing.T) {
	v := NewTextView("ab")
	v.OnEvent(Event{Kind: EventChar, Code: ""})
	if v.Lines[0] != "ab" {
		t.Fatal("empty char should not mutate")
	}
}

func TestTextViewEnterSplitsLine(t *testing.T) {
	v := NewTextView("abcdef")
	v.CursorCol = 3
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(v.Lines) != 2 || v.Lines[0] != "abc" || v.Lines[1] != "def" {
		t.Fatalf("Enter split: %v", v.Lines)
	}
	if v.CursorLine != 1 || v.CursorCol != 0 {
		t.Fatalf("cursor after Enter: line=%d col=%d", v.CursorLine, v.CursorCol)
	}
}

func TestTextViewBackspaceMidLine(t *testing.T) {
	v := NewTextView("abc")
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if v.Lines[0] != "ac" || v.CursorCol != 1 {
		t.Fatalf("backspace: %v cursor=%d", v.Lines, v.CursorCol)
	}
}

func TestTextViewBackspaceAtLineStartMerges(t *testing.T) {
	v := NewTextView("ab\ncd")
	v.CursorLine = 1
	v.CursorCol = 0
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(v.Lines) != 1 || v.Lines[0] != "abcd" || v.CursorLine != 0 || v.CursorCol != 2 {
		t.Fatalf("merge: %v line=%d col=%d", v.Lines, v.CursorLine, v.CursorCol)
	}
}

func TestTextViewBackspaceAtBufferStartNoOp(t *testing.T) {
	v := NewTextView("ab")
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if v.Lines[0] != "ab" {
		t.Fatal("backspace at buffer start should be no-op")
	}
}

func TestTextViewArrowLeftRightAndWrap(t *testing.T) {
	v := NewTextView("ab\ncd")
	v.CursorCol = 0
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	// at (0,0): nowhere to go
	if v.CursorLine != 0 || v.CursorCol != 0 {
		t.Fatal("ArrowLeft at start should pin")
	}
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if v.CursorLine != 1 || v.CursorCol != 0 {
		t.Fatalf("ArrowRight wrap: line=%d col=%d", v.CursorLine, v.CursorCol)
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if v.CursorLine != 0 || v.CursorCol != 2 {
		t.Fatalf("ArrowLeft wrap back: line=%d col=%d", v.CursorLine, v.CursorCol)
	}
	// ArrowRight at end of last line should pin.
	v.CursorLine = 1
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if v.CursorLine != 1 || v.CursorCol != 2 {
		t.Fatal("ArrowRight at buffer end should pin")
	}
}

func TestTextViewArrowUpDownClampsCol(t *testing.T) {
	v := NewTextView("longer\nshort\nxx")
	v.CursorLine = 0
	v.CursorCol = 6
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if v.CursorLine != 1 || v.CursorCol != 5 {
		t.Fatalf("after down: line=%d col=%d, want 1/5", v.CursorLine, v.CursorCol)
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if v.CursorLine != 2 || v.CursorCol != 2 {
		t.Fatalf("after second down: line=%d col=%d, want 2/2", v.CursorLine, v.CursorCol)
	}
	// ArrowDown at last line should pin.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowDown"})
	if v.CursorLine != 2 {
		t.Fatal("ArrowDown at last line should pin")
	}
	// ArrowUp back up.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if v.CursorLine != 1 {
		t.Fatalf("ArrowUp: line=%d", v.CursorLine)
	}
	// ArrowUp at first line should pin.
	v.CursorLine = 0
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if v.CursorLine != 0 {
		t.Fatal("ArrowUp at first line should pin")
	}
}

func TestTextViewHomeEnd(t *testing.T) {
	v := NewTextView("abcdef")
	v.CursorCol = 3
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if v.CursorCol != 0 {
		t.Fatal("Home")
	}
	v.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if v.CursorCol != 6 {
		t.Fatalf("End: col=%d", v.CursorCol)
	}
}

func TestTextViewUnknownKeyNoOp(t *testing.T) {
	v := NewTextView("a")
	v.OnEvent(Event{Kind: EventKeyDown, Code: "F1"})
	if v.Lines[0] != "a" {
		t.Fatal("F1 should not mutate")
	}
}

func TestTextViewIgnoresKeyUp(t *testing.T) {
	v := NewTextView("a")
	v.OnEvent(Event{Kind: EventKeyUp, Code: "x"})
	if v.Lines[0] != "a" {
		t.Fatal("KeyUp should not mutate")
	}
}

func TestTextViewDrawFocusedAndUnfocused(t *testing.T) {
	const w, h = 100, 60
	theme := DefaultLight()
	v := NewTextView("hello\nworld")
	v.Focused = true
	v.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	buf := makeSurface(w, h)
	v.Draw(buf, w, theme)
	// Focused border at top-left = Accent.
	if pixelAt(buf, w, 0, 0) != theme.Accent {
		t.Fatalf("focused border = %+v, want Accent", pixelAt(buf, w, 0, 0))
	}
	v.Focused = false
	buf2 := makeSurface(w, h)
	v.Draw(buf2, w, theme)
	if pixelAt(buf2, w, 0, 0) != theme.Border {
		t.Fatalf("unfocused border = %+v, want Border", pixelAt(buf2, w, 0, 0))
	}
}

func TestTextViewNilOnChangeNoPanic(t *testing.T) {
	v := NewTextView("a")
	v.OnEvent(Event{Kind: EventChar, Code: "b"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
}

// --- Menu + MenuBar ------------------------------------------------------

func TestMenuClickFiresAndCloses(t *testing.T) {
	fired := false
	closed := false
	m := NewMenu([]MenuItem{
		{Label: "Open", Action: func() { fired = true }},
		{Separator: true},
		{Label: "Quit", Action: func() {}},
	})
	m.OnClose = func() { closed = true }
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	m.OnEvent(Event{Kind: EventClick, X: 30, Y: 10}) // row 0 "Open"
	if !fired || !closed {
		t.Fatalf("fired=%v closed=%v", fired, closed)
	}
}

func TestMenuSeparatorAndDisabledIgnored(t *testing.T) {
	m := NewMenu([]MenuItem{
		{Separator: true},
		{Label: "Disabled" /* Action nil */},
	})
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	// Click separator row.
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: 4})
	// Click disabled row.
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: 12})
	// Nothing to assert other than no panic; coverage hits the
	// Action==nil + Separator branches.
}

func TestMenuClickOutOfRangeIgnored(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "X", Action: func() {}}})
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 80})
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: 500})
}

func TestMenuSetHover(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "A"}, {Label: "B"}})
	m.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	m.SetHover(28) // second row
	if m.Hover != 1 {
		t.Fatalf("Hover = %d, want 1", m.Hover)
	}
	m.SetHover(-1)
	if m.Hover != -1 {
		t.Fatal("SetHover(-1) should reset")
	}
}

func TestMenuNilOnCloseNoPanic(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "X", Action: func() {}}})
	m.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 60})
	m.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
}

func TestMenuIgnoresNonClick(t *testing.T) {
	fired := false
	m := NewMenu([]MenuItem{{Label: "X", Action: func() { fired = true }}})
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fired {
		t.Fatal("KeyDown should not fire menu action")
	}
}

func TestMenuDrawsHoveredSubmenuSeparator(t *testing.T) {
	const w, h = 128, 80
	theme := DefaultLight()
	sub := NewMenu(nil)
	m := NewMenu([]MenuItem{
		{Label: "Hovered", Action: func() {}, Submenu: sub},
		{Separator: true},
		{Label: "Disabled"},
	})
	m.Hover = 0
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 70})
	m.Draw(makeSurface(w, h), w, theme)
}

func TestMenuBarClickToggles(t *testing.T) {
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	b.AddMenu("Edit", NewMenu(nil))
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: MenuBarH + 200})
	b.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // open File
	if b.Active != 0 {
		t.Fatalf("after click File: Active = %d", b.Active)
	}
	b.OnEvent(Event{Kind: EventClick, X: 5, Y: 5}) // toggle off
	if b.Active != -1 {
		t.Fatal("second click should toggle off")
	}
	b.OnEvent(Event{Kind: EventClick, X: 70, Y: 5}) // Edit (idx 1)
	if b.Active != 1 {
		t.Fatalf("after click Edit: Active = %d", b.Active)
	}
}

func TestMenuBarClickBelowBarIgnored(t *testing.T) {
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	b.OnEvent(Event{Kind: EventClick, X: 5, Y: MenuBarH + 5})
	if b.Active != -1 {
		t.Fatal("click below bar must not open a menu")
	}
}

func TestMenuBarClickOutOfRangeIgnored(t *testing.T) {
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	b.OnEvent(Event{Kind: EventClick, X: 500, Y: 5})
	if b.Active != -1 {
		t.Fatal("out-of-range click must not open a menu")
	}
}

func TestMenuBarIgnoresNonClick(t *testing.T) {
	b := NewMenuBar()
	b.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if b.Active != -1 {
		t.Fatal("KeyDown should not open a menu")
	}
}

func TestMenuBarDrawHighlightsActive(t *testing.T) {
	const w, h = 200, MenuBarH * 2
	theme := DefaultLight()
	b := NewMenuBar()
	b.AddMenu("File", NewMenu(nil))
	b.AddMenu("Edit", NewMenu(nil))
	b.Active = 0
	b.SetBounds(Rect{X: 0, Y: 0, W: 200, H: MenuBarH})
	b.Draw(makeSurface(w, h), w, theme)
}

// --- Dialog --------------------------------------------------------------

func TestDialogButtonClickFires(t *testing.T) {
	clicked := false
	ok := NewButton("OK", func() { clicked = true })
	d := NewDialog("Confirm", NewLabel("Are you sure?"), ok)
	d.SetBounds(Rect{X: 100, Y: 100, W: 300, H: 200})
	// Click center of OK button. SetBounds laid it out at the bottom-
	// right; compute (x, y) inside its bounds.
	bb := ok.Bounds()
	d.OnEvent(Event{Kind: EventClick, X: bb.X + bb.W/2 - 100, Y: bb.Y + bb.H/2 - 100})
	if !clicked {
		t.Fatal("OK button click didn't fire")
	}
}

func TestDialogClickFallsThroughToContent(t *testing.T) {
	body := &recordingWidget{}
	d := NewDialog("X", body)
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	d.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
	if len(body.events) != 1 {
		t.Fatalf("content event count = %d", len(body.events))
	}
}

func TestDialogNilContentNoPanic(t *testing.T) {
	d := NewDialog("X", nil)
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	d.Draw(makeSurface(400, 300), 400, DefaultLight())
	d.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
}

func TestDialogDraw(t *testing.T) {
	d := NewDialog("Title", NewLabel("body"), NewButton("OK", nil))
	d.SetBounds(Rect{X: 0, Y: 0, W: 300, H: 200})
	d.Draw(makeSurface(400, 300), 400, DefaultLight())
}

func TestNewMessageDialogShape(t *testing.T) {
	d := NewMessageDialog("Info", "All good", nil)
	if d.Title != "Info" || d.Content == nil || len(d.Buttons) != 1 {
		t.Fatalf("shape = %+v", d)
	}
}

// --- Tooltip -------------------------------------------------------------

func TestTooltipShowAndHide(t *testing.T) {
	tt := NewTooltip("Hi")
	tt.Show(Rect{X: 10, Y: 10, W: 40, H: 20})
	if !tt.Visible {
		t.Fatal("after Show: not Visible")
	}
	b := tt.Bounds()
	if b.X != 10 || b.Y != 32 {
		t.Fatalf("tooltip bounds = %+v", b)
	}
	tt.Hide()
	if tt.Visible {
		t.Fatal("after Hide: still Visible")
	}
}

func TestTooltipDrawWhenHiddenNoOp(t *testing.T) {
	tt := NewTooltip("Hi")
	buf := makeSurface(64, 64)
	tt.Draw(buf, 64, DefaultLight())
	if pixelAt(buf, 64, 10, 10) != (RGBA{0xC8, 0xC8, 0xC8, 0xFF}) {
		t.Fatal("hidden tooltip painted")
	}
}

func TestTooltipDrawVisible(t *testing.T) {
	const w, h = 200, 100
	theme := DefaultLight()
	tt := NewTooltip("Hello")
	tt.Show(Rect{X: 10, Y: 10, W: 40, H: 20})
	buf := makeSurface(w, h)
	tt.Draw(buf, w, theme)
	// Bubble background = OnSurface ink colour.
	if pixelAt(buf, w, 20, 35) != theme.OnSurface {
		t.Fatalf("bubble fill = %+v, want OnSurface", pixelAt(buf, w, 20, 35))
	}
}

// --- DropDown ------------------------------------------------------------

func TestDropDownNewClampsSelected(t *testing.T) {
	d := NewDropDown([]string{"a", "b"}, 99)
	if d.Selected != 0 {
		t.Fatalf("Selected = %d, want 0", d.Selected)
	}
}

func TestDropDownNewEmpty(t *testing.T) {
	d := NewDropDown(nil, 0)
	if d.Current() != "" {
		t.Fatal("empty Current should be \"\"")
	}
}

func TestDropDownCurrent(t *testing.T) {
	d := NewDropDown([]string{"a", "b", "c"}, 1)
	if d.Current() != "b" {
		t.Fatalf("Current = %q", d.Current())
	}
	d.Selected = 99
	if d.Current() != "" {
		t.Fatalf("Current with invalid Selected = %q", d.Current())
	}
}

func TestDropDownToggleOpen(t *testing.T) {
	d := NewDropDown([]string{"a"}, 0)
	d.OnEvent(Event{Kind: EventClick})
	if !d.Open {
		t.Fatal("after first click: Open")
	}
	d.OnEvent(Event{Kind: EventClick})
	if d.Open {
		t.Fatal("after second click: closed")
	}
}

func TestDropDownIgnoresNonClick(t *testing.T) {
	d := NewDropDown([]string{"a"}, 0)
	d.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if d.Open {
		t.Fatal("KeyDown should not toggle")
	}
}

func TestDropDownSelectValid(t *testing.T) {
	got := -1
	d := NewDropDown([]string{"a", "b", "c"}, 0)
	d.Open = true
	d.OnSelect = func(i int) { got = i }
	d.Select(2)
	if d.Selected != 2 || d.Open || got != 2 {
		t.Fatalf("Select(2): Selected=%d Open=%v got=%d", d.Selected, d.Open, got)
	}
}

func TestDropDownSelectInvalid(t *testing.T) {
	d := NewDropDown([]string{"a"}, 0)
	d.Select(-1)
	d.Select(99)
	if d.Selected != 0 {
		t.Fatal("invalid Select should not change Selected")
	}
}

func TestDropDownSelectNilCallback(t *testing.T) {
	d := NewDropDown([]string{"a"}, 0)
	d.Select(0)
}

func TestDropDownPopoverBounds(t *testing.T) {
	d := NewDropDown([]string{"a", "b", "c"}, 0)
	d.SetBounds(Rect{X: 5, Y: 5, W: 100, H: 20})
	b := d.PopoverBounds()
	if b.X != 5 || b.Y != 25 || b.W != 100 || b.H != 3*18 {
		t.Fatalf("PopoverBounds = %+v", b)
	}
}

func TestDropDownPopoverBoundsClampsRows(t *testing.T) {
	opts := make([]string, 20)
	for i := range opts {
		opts[i] = "x"
	}
	d := NewDropDown(opts, 0)
	d.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 20})
	b := d.PopoverBounds()
	if b.H != PopoverMaxRows*18 {
		t.Fatalf("H = %d, want %d", b.H, PopoverMaxRows*18)
	}
}

func TestDropDownDraw(t *testing.T) {
	d := NewDropDown([]string{"a", "b"}, 0)
	d.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 24})
	d.Draw(makeSurface(120, 32), 120, DefaultLight())
}

// --- TreeView ------------------------------------------------------------

func TestTreeViewExpandToggle(t *testing.T) {
	leaf := &TreeNode{Label: "leaf"}
	root := &TreeNode{Label: "root", Children: []*TreeNode{leaf}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	// Click chevron on root row (y in [0, 18), x around the chevron).
	tv.OnEvent(Event{Kind: EventClick, X: 4, Y: 5})
	if !root.Expanded {
		t.Fatal("after chevron click: root.Expanded false")
	}
	tv.OnEvent(Event{Kind: EventClick, X: 4, Y: 5})
	if root.Expanded {
		t.Fatal("second chevron click should collapse")
	}
}

func TestTreeViewSelectFiresOnActivate(t *testing.T) {
	var picked *TreeNode
	root := &TreeNode{Label: "root", Expanded: true,
		Children: []*TreeNode{{Label: "child"}}}
	tv := NewTreeView(root)
	tv.OnActivate = func(n *TreeNode) { picked = n }
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	// Click label area of child (row idx 1, x past chevron).
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 25})
	if picked == nil || picked.Label != "child" {
		t.Fatalf("picked = %+v", picked)
	}
	if tv.Selected != picked {
		t.Fatal("Selected != picked")
	}
}

func TestTreeViewClickOutOfRangeIgnored(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.OnEvent(Event{Kind: EventClick, X: 50, Y: 500})
	if tv.Selected != nil {
		t.Fatal("out-of-range click selected something")
	}
}

func TestTreeViewClickNegativeYIgnored(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.OnEvent(Event{Kind: EventClick, X: 50, Y: -10})
	if tv.Selected != nil {
		t.Fatal("negative Y selected something")
	}
}

func TestTreeViewIgnoresNonClick(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if tv.Selected != nil {
		t.Fatal("KeyDown should not select")
	}
}

func TestTreeViewNilRoot(t *testing.T) {
	tv := NewTreeView(nil)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.Draw(makeSurface(200, 100), 200, DefaultLight())
	tv.OnEvent(Event{Kind: EventClick, X: 50, Y: 5})
}

func TestTreeViewNilOnActivateNoPanic(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 5})
}

func TestTreeViewZeroRowHeightFallback(t *testing.T) {
	root := &TreeNode{Label: "root"}
	tv := NewTreeView(root)
	tv.RowHeight = 0
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.Draw(makeSurface(200, 100), 200, DefaultLight())
	tv.OnEvent(Event{Kind: EventClick, X: 80, Y: 5})
}

func TestTreeViewDrawCollapsedChevron(t *testing.T) {
	// Cover the ▶ (collapsed) chevron paint branch.
	root := &TreeNode{Label: "root", Expanded: false,
		Children: []*TreeNode{{Label: "child"}}}
	tv := NewTreeView(root)
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.Draw(makeSurface(200, 100), 200, DefaultLight())
}

// --- Extra branch coverage for TextView ---------------------------------

func TestTextViewSplitLineFiresOnChange(t *testing.T) {
	c := 0
	v := NewTextView("abc")
	v.OnChange = func() { c++ }
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if c != 1 {
		t.Fatalf("OnChange fired %d times after Enter, want 1", c)
	}
}

func TestTextViewBackspaceMidLineFiresOnChange(t *testing.T) {
	c := 0
	v := NewTextView("abc")
	v.OnChange = func() { c++ }
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c != 1 {
		t.Fatalf("OnChange fired %d times after Backspace mid-line, want 1", c)
	}
}

func TestTextViewBackspaceMergeFiresOnChange(t *testing.T) {
	c := 0
	v := NewTextView("ab\ncd")
	v.OnChange = func() { c++ }
	v.CursorLine = 1
	v.CursorCol = 0
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if c != 1 {
		t.Fatalf("OnChange fired %d times after merge backspace, want 1", c)
	}
}

func TestTextViewSplitLineNoOnChangeNoPanic(t *testing.T) {
	v := NewTextView("abc")
	v.OnChange = nil
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if len(v.Lines) != 2 {
		t.Fatal("Enter should still split with nil OnChange")
	}
}

func TestTextViewBackspaceNoOnChangeNoPanic(t *testing.T) {
	v := NewTextView("ab")
	v.OnChange = nil
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if v.Lines[0] != "a" {
		t.Fatal("Backspace should still delete with nil OnChange")
	}
}

func TestTextViewBackspaceMergeNoOnChangeNoPanic(t *testing.T) {
	v := NewTextView("ab\ncd")
	v.OnChange = nil
	v.CursorLine = 1
	v.CursorCol = 0
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(v.Lines) != 1 {
		t.Fatal("merge should work with nil OnChange")
	}
}

func TestTextViewCursorLeftInLine(t *testing.T) {
	v := NewTextView("abc")
	v.CursorCol = 2
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if v.CursorCol != 1 {
		t.Fatalf("ArrowLeft in-line: col=%d, want 1", v.CursorCol)
	}
}

func TestTextViewCursorRightInLine(t *testing.T) {
	v := NewTextView("abc")
	v.CursorCol = 1
	v.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if v.CursorCol != 2 {
		t.Fatalf("ArrowRight in-line: col=%d, want 2", v.CursorCol)
	}
}

func TestTreeViewDrawExpandedHierarchy(t *testing.T) {
	root := &TreeNode{Label: "root", Expanded: true,
		Children: []*TreeNode{
			{Label: "a"},
			{Label: "b", Expanded: true, Children: []*TreeNode{{Label: "b1"}}},
		}}
	tv := NewTreeView(root)
	tv.Selected = root
	tv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	tv.Draw(makeSurface(200, 100), 200, DefaultLight())
}

// --- v0.5: MenuItem.Shortcut + MenuBar Alt+letter -----------------------

func TestMenuItemShortcutHintPainted(t *testing.T) {
	// A shortcut hint on a row exercises the "right-aligned hint" branch
	// in Menu.Draw. Also exercise the hovered-row inverse ink path.
	m := NewMenu([]MenuItem{
		{Label: "New", Action: func() {}, Shortcut: "Ctrl+N"},
	})
	m.SetBounds(Rect{X: 0, Y: 0, W: 160, H: MenuRowH + 4})
	m.Draw(makeSurface(160, 60), 160, DefaultLight())
	m.Hover = 0
	m.Draw(makeSurface(160, 60), 160, DefaultLight())
}

func TestMenuBarAltLetter(t *testing.T) {
	bar := NewMenuBar()
	bar.Names = []string{"File", "Edit", "View"}
	bar.Menus = []*Menu{NewMenu(nil), NewMenu(nil), NewMenu(nil)}
	bar.SetBounds(Rect{X: 0, Y: 0, W: 200, H: MenuBarH})

	// Alt+F opens File (index 0).
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Alt+F"})
	if bar.Active != 0 {
		t.Fatalf("Alt+F active=%d, want 0", bar.Active)
	}
	// Alt+e (lower-case) opens Edit (case-insensitive match).
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Alt+e"})
	if bar.Active != 1 {
		t.Fatalf("Alt+e active=%d, want 1", bar.Active)
	}
	// Escape closes.
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if bar.Active != -1 {
		t.Fatalf("Escape active=%d, want -1", bar.Active)
	}
	// Alt+X (no match) leaves things alone.
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Alt+X"})
	if bar.Active != -1 {
		t.Fatalf("Alt+X (miss) should not open a menu; got %d", bar.Active)
	}
	// Malformed Code (not "Alt+X") is dropped.
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+N"})
	if bar.Active != -1 {
		t.Fatalf("non-Alt code should not open; got %d", bar.Active)
	}
	// Empty name in Names is skipped (defensive branch).
	bar.Names = []string{"", "File"}
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Alt+F"})
	if bar.Active != 1 {
		t.Fatalf("Alt+F should skip empty name; got active=%d", bar.Active)
	}
	// Lower-case first letter in Names (case-insensitive match on that side too).
	bar.Names = []string{"file"}
	bar.Active = -1
	bar.OnEvent(Event{Kind: EventKeyDown, Code: "Alt+F"})
	if bar.Active != 0 {
		t.Fatalf("Alt+F should match lower-case 'file'; got active=%d", bar.Active)
	}
}

func TestMenuBarMnemonic(t *testing.T) {
	bar := NewMenuBar()
	bar.Names = []string{"File", "edit", ""}
	if bar.Mnemonic(0) != 'F' {
		t.Fatalf("Mnemonic(0) = %c want F", bar.Mnemonic(0))
	}
	if bar.Mnemonic(1) != 'E' {
		t.Fatalf("Mnemonic(1) = %c want E (case-insensitive)", bar.Mnemonic(1))
	}
	if bar.Mnemonic(2) != 0 {
		t.Fatalf("Mnemonic(2) empty name should be 0")
	}
	if bar.Mnemonic(-1) != 0 {
		t.Fatalf("Mnemonic(-1) out-of-range should be 0")
	}
	if bar.Mnemonic(99) != 0 {
		t.Fatalf("Mnemonic(99) out-of-range should be 0")
	}
}

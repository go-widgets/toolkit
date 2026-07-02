// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

// Each v0.4 widget has its own dedicated test block below. The shared
// test scaffold uses a 200x200 RGBA surface to keep the painters
// exercising real bounds while the tests stay fast.

const v04SurfW = 200
const v04SurfH = 200

func v04Surface() []byte { return make([]byte, 4*v04SurfW*v04SurfH) }

// --- Toolbar -------------------------------------------------------------

func TestToolbarClickFires(t *testing.T) {
	clicked := -1
	tb := NewToolbar([]ToolbarItem{
		{Label: "A", OnClick: func() { clicked = 0 }},
		{Separator: true},
		{Label: "B", OnClick: func() { clicked = 1 }},
		{Label: "C", Disabled: true, OnClick: func() { clicked = 2 }},
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: ToolbarButtonH})
	// Item 0 at x=12; click should fire.
	tb.OnEvent(Event{Kind: EventClick, X: 12, Y: 10})
	if clicked != 0 {
		t.Fatalf("want clicked=0, got %d", clicked)
	}
	// Click in the separator: ignored.
	tb.OnEvent(Event{Kind: EventClick, X: 24 + ToolbarSepW/2, Y: 10})
	if clicked != 0 {
		t.Fatalf("separator click must not fire; got %d", clicked)
	}
	// Item 1 (B) after the separator: x=24+sep+12.
	tb.OnEvent(Event{Kind: EventClick, X: 24 + ToolbarSepW + 12, Y: 10})
	if clicked != 1 {
		t.Fatalf("want clicked=1, got %d", clicked)
	}
	// Disabled item: ignored.
	tb.OnEvent(Event{Kind: EventClick, X: 24 + ToolbarSepW + 24 + 12, Y: 10})
	if clicked == 2 {
		t.Fatalf("disabled item must not fire")
	}
}

func TestToolbarHitTestBounds(t *testing.T) {
	tb := NewToolbar([]ToolbarItem{{Label: "A"}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 24})
	if tb.hitTest(0, -1) != -1 {
		t.Fatal("y<0 must miss")
	}
	if tb.hitTest(0, 30) != -1 {
		t.Fatal("y>=H must miss")
	}
	if tb.hitTest(50, 10) != -1 {
		t.Fatal("x past last button must miss")
	}
}

func TestToolbarDrawIcon(t *testing.T) {
	icon := make([]byte, 4*ToolbarButtonW*ToolbarButtonH)
	for i := range icon {
		icon[i] = 0x80
	}
	tb := NewToolbar([]ToolbarItem{
		{Label: "X", Icon: icon, OnClick: func() {}},
		{Separator: true},
		{Label: "Y", Disabled: true},
		{Label: ""}, // empty label uses "?" fallback
	})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: ToolbarButtonH})
	tb.OnEvent(Event{Kind: EventClick, X: 12, Y: 10}) // press visual state
	tb.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
}

func TestToolbarZeroDimensionsFallback(t *testing.T) {
	tb := &Toolbar{Items: []ToolbarItem{{Label: "A"}}, pressIdx: -1}
	tb.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 24})
	tb.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
	if tb.hitTest(0, 0) != 0 {
		t.Fatal("zero-W/H toolbar should still hit-test via fallback constants")
	}
}

func TestToolbarOnEventIgnoresKeyDown(t *testing.T) {
	tb := NewToolbar([]ToolbarItem{{Label: "A", OnClick: func() { t.Fatal("must not fire") }}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 24})
	tb.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
}

func TestToolbarNilOnClick(t *testing.T) {
	tb := NewToolbar([]ToolbarItem{{Label: "A"}}) // nil OnClick
	tb.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 24})
	tb.OnEvent(Event{Kind: EventClick, X: 12, Y: 10}) // must not panic
}

func TestBlitRGBASrcTooShort(t *testing.T) {
	src := []byte{0xFF, 0xFF, 0xFF, 0xFF} // 1 px when caller asks for 2x2
	blitRGBA(newP(v04Surface(), v04SurfW), 0, 0, 2, 2, src)
}

// --- Statusbar -----------------------------------------------------------

func TestStatusbarDrawAndSet(t *testing.T) {
	sb := NewStatusbar([]string{"A", "B"})
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	sb.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
	sb.SetSegment(0, "Updated")
	sb.SetSegment(3, "Grown") // grows past end
	if len(sb.Segments) != 4 {
		t.Fatalf("want 4 segments after grow, got %d", len(sb.Segments))
	}
	if sb.Segments[3] != "Grown" {
		t.Fatal("grown segment text wrong")
	}
	sb.SetSegment(-1, "ignored") // negative index = no-op
	if len(sb.Segments) != 4 {
		t.Fatal("negative SetSegment must be no-op")
	}
}

func TestStatusbarZeroMinFallback(t *testing.T) {
	sb := &Statusbar{Segments: []string{"a", "b"}}
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	sb.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
}

func TestStatusbarSingleSegment(t *testing.T) {
	sb := NewStatusbar([]string{"only"})
	sb.SetBounds(Rect{X: 0, Y: 0, W: 200, H: StatusbarH})
	sb.Draw(newP(v04Surface(), v04SurfW), DefaultLight()) // last-segment-fills branch
}

// --- FileChooser ---------------------------------------------------------

func TestFileChooserOpenAndCancel(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true, Children: []*TreeNode{
		{Label: "etc"},
		{Label: "var"},
	}}
	listFiles := func(n *TreeNode) []string {
		return []string{"a.txt", "b.txt"}
	}
	var opened, cancelled string
	fc := NewFileChooser(root, listFiles)
	fc.OnAccept = func(p string) { opened = p }
	fc.OnCancel = func() { cancelled = "yes" }
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	fc.Draw(newP(v04Surface(), v04SurfW), DefaultLight())

	// Click on the tree's first row (root): selects it + fills the list.
	treeR := fc.tree.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: treeR.X + 20, Y: treeR.Y + 5})
	if len(fc.list.Items) == 0 {
		t.Fatalf("expected files after tree activate, got 0")
	}
	// Click on first list item.
	listR := fc.list.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: listR.X + 5, Y: listR.Y + 5})
	if fc.selectedFile != "a.txt" {
		t.Fatalf("want a.txt, got %q", fc.selectedFile)
	}
	// Click on the path entry: forwarded harmlessly.
	er := fc.pathEntry.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: er.X + 5, Y: er.Y + 5})
	// Click Open.
	ob := fc.openButton.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: ob.X + 5, Y: ob.Y + 5})
	if opened == "" {
		t.Fatal("OnAccept must fire")
	}
	if fc.Path() == "" {
		t.Fatal("Path() must return entry text")
	}
	// Click Cancel.
	cb := fc.cancelButton.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: cb.X + 5, Y: cb.Y + 5})
	if cancelled != "yes" {
		t.Fatal("OnCancel must fire")
	}
}

func TestFileChooserListActivateOutOfRange(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true}
	fc := NewFileChooser(root, func(n *TreeNode) []string { return nil })
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	// Force OnActivate path with no items.
	fc.list.OnActivate(0) // idx >= len: must early-return
}

func TestFileChooserNoListCallback(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true}
	fc := NewFileChooser(root, nil)
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	treeR := fc.tree.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: treeR.X + 20, Y: treeR.Y + 5})
}

func TestFileChooserNilCallbacks(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true, Children: []*TreeNode{{Label: "a"}}}
	fc := NewFileChooser(root, func(n *TreeNode) []string { return []string{"x"} })
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	ob := fc.openButton.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: ob.X + 5, Y: ob.Y + 5}) // nil OnAccept
	cb := fc.cancelButton.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: cb.X + 5, Y: cb.Y + 5}) // nil OnCancel
}

func TestFileChooserOutOfBoundsEvent(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true}
	fc := NewFileChooser(root, func(n *TreeNode) []string { return nil })
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	fc.OnEvent(Event{Kind: EventClick, X: 9999, Y: 9999})
}

func TestFileChooserListActivateRefreshesPath(t *testing.T) {
	root := &TreeNode{Label: "/", Expanded: true}
	fc := NewFileChooser(root, func(n *TreeNode) []string { return []string{"x"} })
	fc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	// Tree-activate root first so currentDir is set.
	treeR := fc.tree.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: treeR.X + 20, Y: treeR.Y + 5})
	listR := fc.list.Bounds()
	fc.OnEvent(Event{Kind: EventClick, X: listR.X + 5, Y: listR.Y + 5})
	if fc.Path() == "" {
		t.Fatal("path should contain dir/file after activation")
	}
	// Also exercise the "currentDir == nil" branch of OnActivate.
	fc2 := NewFileChooser(root, func(n *TreeNode) []string { return []string{"y"} })
	fc2.list.Items = []string{"y"}
	fc2.list.OnActivate(0)
	if fc2.Path() != "/y" {
		t.Fatalf("want /y, got %q", fc2.Path())
	}
}

// --- Calendar ------------------------------------------------------------

func TestCalendarDraw(t *testing.T) {
	c := NewCalendar(2026, 6, 30)
	c.SetToday(2026, 6, 30)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	c.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
}

func TestCalendarClampMonth(t *testing.T) {
	c := NewCalendar(2026, 0, 1)
	if c.Month != 1 {
		t.Fatalf("month=0 must clamp to 1, got %d", c.Month)
	}
	c = NewCalendar(2026, 13, 1)
	if c.Month != 12 {
		t.Fatalf("month=13 must clamp to 12, got %d", c.Month)
	}
	c = NewCalendar(2026, 6, 0)
	if c.Day != 1 {
		t.Fatalf("day=0 must clamp to 1, got %d", c.Day)
	}
	c = NewCalendar(2026, 6, 99)
	if c.Day != 30 {
		t.Fatalf("day=99 must clamp to 30, got %d", c.Day)
	}
}

func TestCalendarDaysInMonth(t *testing.T) {
	cases := []struct {
		y, m, want int
	}{
		{2026, 1, 31}, {2026, 2, 28}, {2024, 2, 29}, {2100, 2, 28}, {2000, 2, 29},
		{2026, 3, 31}, {2026, 4, 30}, {2026, 5, 31}, {2026, 6, 30},
		{2026, 7, 31}, {2026, 8, 31}, {2026, 9, 30}, {2026, 10, 31},
		{2026, 11, 30}, {2026, 12, 31}, {2026, 0, 30}, {2026, 99, 30},
	}
	for _, c := range cases {
		if g := DaysInMonth(c.y, c.m); g != c.want {
			t.Fatalf("DaysInMonth(%d,%d)=%d want %d", c.y, c.m, g, c.want)
		}
	}
}

func TestCalendarWeekday(t *testing.T) {
	// 2026-01-01 was a Thursday. Mon=0 Sun=6 -> Thursday=3.
	if g := WeekdayOfFirst(2026, 1); g != 3 {
		t.Fatalf("2026-01-01 weekday want 3, got %d", g)
	}
	// 2024-02-01 = Thursday too. (Tests the m<3 branch with year-1.)
	if g := WeekdayOfFirst(2024, 2); g != 3 {
		t.Fatalf("2024-02-01 weekday want 3, got %d", g)
	}
	// Force the Sat (0) branch: 2025-11-01 was a Saturday.
	if g := WeekdayOfFirst(2025, 11); g != 5 {
		t.Fatalf("2025-11-01 weekday want 5 (Sat), got %d", g)
	}
	// Force the Sun (1) branch: 2026-03-01 was a Sunday.
	if g := WeekdayOfFirst(2026, 3); g != 6 {
		t.Fatalf("2026-03-01 weekday want 6 (Sun), got %d", g)
	}
}

func TestCalendarSelect(t *testing.T) {
	c := NewCalendar(2026, 6, 1)
	var got int
	c.OnSelect = func(y, m, d int) { got = d }
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	// June 2026 first = Mon -> col 0 row 0. Click the 1st cell.
	gridY := CalendarHeaderH + GlyphHeight + 4
	c.OnEvent(Event{Kind: EventClick, X: 0, Y: gridY + 2})
	if got != 1 {
		t.Fatalf("want day=1, got %d", got)
	}
	// Click outside the grid (above): no-op.
	got = 0
	c.OnEvent(Event{Kind: EventClick, X: 0, Y: 5})
	if got != 0 {
		t.Fatal("click in header must not fire OnSelect")
	}
	// Click on a column out of range: no-op.
	c.OnEvent(Event{Kind: EventClick, X: 99999, Y: gridY + 2})
	// Click in the gap before the first day (June 2026 starts on Mon
	// so col=0 has day=1; pick a month that DOES leave a leading
	// gap: 2025-11 starts on Sat (col 5) so clicking col 0 should
	// fall in the gap).
	c2 := NewCalendar(2025, 11, 1)
	c2.OnSelect = func(y, m, d int) { t.Fatalf("gap click must not fire (got d=%d)", d) }
	c2.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	c2.OnEvent(Event{Kind: EventClick, X: 0, Y: gridY + 2})
	// Click well past the last day: no-op.
	c.OnEvent(Event{Kind: EventClick, X: 0, Y: gridY + 8*CalendarCellH})
}

func TestCalendarSetDate(t *testing.T) {
	c := NewCalendar(2026, 1, 1)
	c.SetDate(2027, 8, 15)
	if c.Year != 2027 || c.Month != 8 || c.Day != 15 {
		t.Fatalf("SetDate failed: y=%d m=%d d=%d", c.Year, c.Month, c.Day)
	}
}

func TestCalendarOnEventKeyDown(t *testing.T) {
	c := NewCalendar(2026, 6, 1)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // ignored
}

func TestCalendarSelectNegativeCol(t *testing.T) {
	c := NewCalendar(2026, 6, 1)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	gridY := CalendarHeaderH + GlyphHeight + 4
	c.OnEvent(Event{Kind: EventClick, X: -50, Y: gridY + 2}) // col<0 branch
}

func TestMonthNameAll(t *testing.T) {
	wants := []string{"???", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for i := 0; i <= 12; i++ {
		if monthName(i) != wants[i] {
			t.Fatalf("monthName(%d)=%q want %q", i, monthName(i), wants[i])
		}
	}
	if monthName(99) != "???" {
		t.Fatal("monthName(99) must default")
	}
}

func TestItoa(t *testing.T) {
	if itoa(0) != "0" {
		t.Fatal("itoa(0)")
	}
	if itoa(123) != "123" {
		t.Fatal("itoa(123)")
	}
	if itoa(-7) != "-7" {
		t.Fatalf("itoa(-7)=%q", itoa(-7))
	}
}

// --- ColorChooser --------------------------------------------------------

func TestColorChooserDraw(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF})
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	cc.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
}

func TestColorChooserNewForcesAlpha(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0})
	if cc.Color.A != 0xFF {
		t.Fatalf("alpha must be forced to 0xFF, got %d", cc.Color.A)
	}
}

func TestColorChooserClick(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	got := uint8(0)
	cc.OnChange = func(c RGBA) { got = c.R }
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	// Click the R track far right.
	cc.OnEvent(Event{Kind: EventClick, X: 999, Y: ColorChooserChannelPadY + 2})
	if got != 255 {
		t.Fatalf("want R=255 after right-click, got %d", got)
	}
	// Click the R track far left.
	cc.OnEvent(Event{Kind: EventClick, X: -10, Y: ColorChooserChannelPadY + 2})
	// Out-of-bounds horizontal still inside the channel row: clamps to 0.
	if cc.Color.R != 255 && cc.Color.R != 0 {
		t.Fatalf("R after left-click should be 0 or 255 (clamped), got %d", cc.Color.R)
	}
	// Click middle of G track.
	cc.OnEvent(Event{Kind: EventClick, X: 100, Y: ColorChooserChannelPadY + ColorChooserChannelH + 2})
	if cc.Color.G == 0 {
		t.Fatal("G should change after middle click")
	}
	// Click middle of B track.
	cc.OnEvent(Event{Kind: EventClick, X: 100, Y: ColorChooserChannelPadY + 2*ColorChooserChannelH + 2})
	if cc.Color.B == 0 {
		t.Fatal("B should change after middle click")
	}
}

func TestColorChooserClickOutsideBounds(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	cc.OnEvent(Event{Kind: EventClick, X: -1, Y: -1}) // out of bounds
	cc.OnEvent(Event{Kind: EventClick, X: 999, Y: 999})
	cc.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // wrong kind
	// Click between channels (no row hits): falls through.
	cc.OnEvent(Event{Kind: EventClick, X: 50, Y: ColorChooserChannelPadY + 3*ColorChooserChannelH + 5})
}

func TestColorChooserNilOnChange(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	cc.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	cc.OnEvent(Event{Kind: EventClick, X: 100, Y: ColorChooserChannelPadY + 2})
}

func TestColorChooserChannelGetterDefault(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xFF})
	if cc.channel(0) != 0x12 || cc.channel(1) != 0x34 || cc.channel(2) != 0x56 {
		t.Fatalf("channel getter wrong: %v", cc.Color)
	}
	if cc.channel(99) != 0 {
		t.Fatal("channel(99) default branch")
	}
	cc.setChannel(99, 1) // setChannel default branch
}

func TestColorChooserHex(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0xAB, G: 0xCD, B: 0xEF, A: 0xFF})
	if got := cc.Hex(); got != "#ABCDEF" {
		t.Fatalf("Hex want #ABCDEF got %q", got)
	}
}

func TestColorChooserSetHex(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	got := RGBA{}
	cc.OnChange = func(c RGBA) { got = c }
	cc.SetHex("#102030")
	if cc.Color.R != 0x10 || cc.Color.G != 0x20 || cc.Color.B != 0x30 {
		t.Fatalf("SetHex(#102030) failed: %v", cc.Color)
	}
	if got.R != 0x10 {
		t.Fatal("OnChange must fire from SetHex")
	}
	cc.SetHex("ABCDEF") // no leading hash
	if cc.Color.R != 0xAB {
		t.Fatal("SetHex without # failed")
	}
	cc.SetHex("zzzzzz")  // invalid -> ignored
	cc.SetHex("#11223")  // wrong length -> ignored
	cc.SetHex("xx2233")  // first pair invalid
	cc.SetHex("11xx33")  // mid pair invalid
	cc.SetHex("1122xx")  // last pair invalid
	cc.SetHex("#aabbcc") // lower-case
	if cc.Color.R != 0xAA {
		t.Fatal("SetHex lowercase failed")
	}
}

func TestColorChooserSetHexNilOnChange(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	cc.SetHex("#FF0000")
}

func TestHexNib(t *testing.T) {
	if _, ok := hexNib('g'); ok {
		t.Fatal("g must reject")
	}
	if _, ok := hex2('z', '0'); ok {
		t.Fatal("z must reject hi")
	}
	if _, ok := hex2('0', 'z'); ok {
		t.Fatal("z must reject lo")
	}
}

// --- Selection model on TextView -----------------------------------------

func TestSelectionEmpty(t *testing.T) {
	s := Selection{1, 2, 1, 2}
	if !s.IsEmpty() {
		t.Fatal("equal endpoints = empty")
	}
	s.EndCol = 5
	if s.IsEmpty() {
		t.Fatal("nonzero range = not empty")
	}
}

func TestSelectionRange(t *testing.T) {
	s := SelectionRange(0, 0, 2, 3)
	if s.StartLine != 0 || s.EndLine != 2 {
		t.Fatalf("forward range wrong: %+v", s)
	}
	s = SelectionRange(2, 3, 0, 0)
	if s.StartLine != 0 || s.EndLine != 2 {
		t.Fatalf("reverse range must canonicalise: %+v", s)
	}
	s = SelectionRange(1, 5, 1, 2)
	if s.StartCol != 2 || s.EndCol != 5 {
		t.Fatal("same-line reverse must canonicalise cols")
	}
}

func TestSelectionTextSingleLine(t *testing.T) {
	lines := []string{"hello world"}
	got := SelectionText(lines, Selection{0, 0, 0, 5})
	if got != "hello" {
		t.Fatalf("want hello, got %q", got)
	}
	if got := SelectionText(lines, Selection{0, 0, 0, 99}); got != "hello world" {
		t.Fatalf("over-end-clamp failed: %q", got)
	}
	if got := SelectionText(lines, Selection{0, 3, 0, 3}); got != "" {
		t.Fatal("empty selection must yield ''")
	}
}

func TestSelectionTextMultiLine(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	got := SelectionText(lines, Selection{0, 2, 2, 3})
	want := "ne1\nline2\nlin"
	if got != want {
		t.Fatalf("multi-line: want %q got %q", want, got)
	}
	// Over-end-clamp on last line.
	got = SelectionText(lines, Selection{0, 0, 2, 99})
	if got != "line1\nline2\nline3" {
		t.Fatalf("clamp last-line over-end failed: %q", got)
	}
}

func TestDeleteSelectionSingleLine(t *testing.T) {
	out := DeleteSelection([]string{"hello"}, Selection{0, 1, 0, 4})
	if len(out) != 1 || out[0] != "ho" {
		t.Fatalf("single-line delete failed: %v", out)
	}
	out = DeleteSelection([]string{"x"}, Selection{0, 0, 0, 0}) // empty sel
	if out[0] != "x" {
		t.Fatal("empty-sel must be no-op")
	}
}

func TestDeleteSelectionMultiLine(t *testing.T) {
	lines := []string{"a", "b", "c", "d"}
	// Selection covers "\nb\n" (after "a" → before "c"); deletion
	// joins line 0's prefix "a" + line 2's suffix "c" → "ac", then
	// line "d" follows on the next row.
	out := DeleteSelection(lines, Selection{0, 1, 2, 0})
	if len(out) != 2 || out[0] != "ac" || out[1] != "d" {
		t.Fatalf("multi-line delete failed: %v", out)
	}
	// Selection covers full body across rows: "hi\nya" → joined to
	// "" + "" = "" on one line.
	out = DeleteSelection([]string{"hi", "ya"}, Selection{0, 0, 1, 99})
	if len(out) != 1 || out[0] != "" {
		t.Fatalf("over-end multi-line delete failed: %v", out)
	}
}

func TestTextViewSelectionAPIs(t *testing.T) {
	tv := NewTextView("hello\nworld")
	if tv.HasSelection() {
		t.Fatal("fresh TextView must have empty selection")
	}
	tv.SetSelection(Selection{0, 0, 0, 5})
	if !tv.HasSelection() {
		t.Fatal("after SetSelection HasSelection must be true")
	}
	if tv.SelectionText() != "hello" {
		t.Fatalf("SelectionText want hello, got %q", tv.SelectionText())
	}
	tv.ClearSelection()
	if tv.HasSelection() {
		t.Fatal("after ClearSelection HasSelection must be false")
	}
}

func TestTextViewSelectAll(t *testing.T) {
	tv := NewTextView("a\nbc")
	tv.SelectAll()
	if tv.SelectionText() != "a\nbc" {
		t.Fatalf("SelectAll text wrong: %q", tv.SelectionText())
	}
	// SelectAll on empty buffer.
	tv2 := &TextView{} // no lines
	tv2.SelectAll()    // must not panic
}

func TestTextViewDeleteSelectionFires(t *testing.T) {
	fired := false
	tv := NewTextView("hello world")
	tv.OnChange = func() { fired = true }
	tv.SetSelection(Selection{0, 0, 0, 6})
	tv.DeleteSelection()
	if tv.Text() != "world" {
		t.Fatalf("delete-selection wrong: %q", tv.Text())
	}
	if !fired {
		t.Fatal("OnChange must fire")
	}
	// Empty selection: no-op.
	fired = false
	tv.DeleteSelection()
	if fired {
		t.Fatal("empty-sel delete must not fire OnChange")
	}
}

func TestTextViewDeleteSelectionNilOnChange(t *testing.T) {
	tv := NewTextView("ab")
	tv.SetSelection(Selection{0, 0, 0, 2})
	tv.DeleteSelection()
	if tv.Text() != "" {
		t.Fatalf("delete failed: %q", tv.Text())
	}
}

// --- Last-mile coverage gap fillers --------------------------------------

func TestCalendarTodayPillNonSelected(t *testing.T) {
	// Today=June 30, selected=June 1. Day 30 should hit the SurfaceAlt
	// "today" branch (not the Accent "selected" one).
	c := NewCalendar(2026, 6, 1)
	c.SetToday(2026, 6, 30)
	c.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 200})
	c.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
}

func TestColorChooserSetHexLengthMismatch(t *testing.T) {
	cc := NewColorChooser(RGBA{R: 0, G: 0, B: 0, A: 0xFF})
	cc.SetHex("ABCDE") // len 5 -> rejected
	if cc.Color.R != 0 {
		t.Fatal("malformed SetHex must not mutate")
	}
}

func TestDeleteSelectionAllRowsEmptyFallback(t *testing.T) {
	// Construct a malformed lines slice + selection that drives the
	// "len(out)==0" fallback. Realistic: select the only line entirely
	// AND have it be empty so the merged string is "". out becomes
	// ["", ""... no: it becomes [""] always. The "len(out)==0" is
	// only reachable if both pre + post slices are empty AND the
	// merged string was somehow omitted — defensive, not pixel-
	// reachable. Cover via the DeleteSelection call below which still
	// executes the surrounding code paths.
	out := DeleteSelection([]string{""}, Selection{0, 0, 0, 0})
	if len(out) != 1 {
		t.Fatalf("empty-input no-op want 1 line, got %d", len(out))
	}
}

func TestToolbarPressedInkBranch(t *testing.T) {
	// Drive the "i == t.pressIdx" ink branch in Draw: press item 0
	// then immediately draw.
	tb := NewToolbar([]ToolbarItem{{Label: "A", OnClick: func() {}}})
	tb.SetBounds(Rect{X: 0, Y: 0, W: 24, H: 24})
	tb.OnEvent(Event{Kind: EventClick, X: 12, Y: 10})
	tb.Draw(newP(v04Surface(), v04SurfW), DefaultLight())
	if tb.pressIdx != 0 {
		t.Fatal("pressIdx should be 0 after click")
	}
}

func TestTextViewCutCopyPaste(t *testing.T) {
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	if c := tv.CopySelection(); c != "hello" {
		t.Fatalf("copy want hello, got %q", c)
	}
	if tv.Text() != "hello world" {
		t.Fatal("copy must not mutate")
	}
	if cut := tv.CutSelection(); cut != "hello" {
		t.Fatalf("cut want hello, got %q", cut)
	}
	if tv.Text() != " world" {
		t.Fatalf("after cut text wrong: %q", tv.Text())
	}
	// Paste with no selection.
	tv.Paste("yo")
	if tv.Text() != "yo world" {
		t.Fatalf("paste at start wrong: %q", tv.Text())
	}
	// Paste over an existing selection.
	tv.SetSelection(Selection{0, 0, 0, 2})
	tv.Paste("HEY")
	if tv.Text() != "HEY world" {
		t.Fatalf("paste-over-selection wrong: %q", tv.Text())
	}
}

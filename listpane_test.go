// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// listPaneFixture builds a pane over a ListBox with the three counters a test
// needs, laid out in a 220x140 box.
type listPaneFixture struct {
	pane                  *ListPane
	list                  *ListBox
	adds, removes, extras int
}

func newListPaneFixture(style ListBarStyle, withExtra bool) *listPaneFixture {
	f := &listPaneFixture{list: NewListBox([]string{"alpha", "beta", "gamma"})}
	f.pane = NewListPane(f.list, style)
	f.pane.Add.OnClick = func() { f.adds++ }
	f.pane.Remove.OnClick = func() { f.removes++ }
	if withExtra {
		x := NewButton("Export", func() { f.extras++ })
		x.Flat = true
		f.pane.Extra = []*Button{x}
	}
	f.pane.SetBounds(Rect{X: 10, Y: 20, W: 220, H: 140})
	return f
}

func (f *listPaneFixture) click(sx, sy int) {
	r := f.pane.Bounds()
	f.pane.OnEvent(Event{Kind: EventClick, X: sx - r.X, Y: sy - r.Y})
}

func centreOf(r Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

func TestListBarStyleFor(t *testing.T) {
	for goos, want := range map[string]ListBarStyle{
		"darwin": ListBarMac, "windows": ListBarWindows, "linux": ListBarGNOME,
		"freebsd": ListBarGNOME, "": ListBarGNOME,
	} {
		if got := ListBarStyleFor(goos); got != want {
			t.Errorf("ListBarStyleFor(%q) = %v, want %v", goos, got, want)
		}
	}
}

func TestListPaneMacLayout(t *testing.T) {
	f := newListPaneFixture(ListBarMac, false)
	p := f.pane
	fr := Rect{X: 10, Y: 20, W: 220, H: 140}
	if p.FrameRect() != fr {
		t.Fatalf("frame = %+v, want %+v", p.FrameRect(), fr)
	}
	// smallSquare: a 24-px bar flush with the bottom of the frame.
	if want := (Rect{X: 10, Y: 136, W: 220, H: 24}); p.BarRect() != want {
		t.Errorf("bar = %+v, want %+v", p.BarRect(), want)
	}
	// The list takes what is left inside the 1-px border; the bar's top edge is
	// the list's bottom border.
	if want := (Rect{X: 11, Y: 21, W: 218, H: 115}); p.ContentRect() != want {
		t.Errorf("content = %+v, want %+v", p.ContentRect(), want)
	}
	if f.list.Bounds() != p.ContentRect() {
		t.Errorf("the list was not given the content rect")
	}
	// Square buttons of 26, joined by one shared 1-px divider each.
	if want := (Rect{X: 11, Y: 137, W: 26, H: 22}); p.Add.Bounds() != want {
		t.Errorf("Add = %+v, want %+v", p.Add.Bounds(), want)
	}
	if want := (Rect{X: 38, Y: 137, W: 26, H: 22}); p.Remove.Bounds() != want {
		t.Errorf("Remove = %+v, want %+v", p.Remove.Bounds(), want)
	}
	// An inert filler covers the rest of the bar, up to the right border.
	if want := (Rect{X: 65, Y: 137, W: 164, H: 22}); p.FillerRect() != want {
		t.Errorf("filler = %+v, want %+v", p.FillerRect(), want)
	}
}

func TestListPaneWindowsLayout(t *testing.T) {
	f := newListPaneFixture(ListBarWindows, true)
	p := f.pane
	if p.BarRect().H != 28 {
		t.Errorf("bar height = %d, want 28", p.BarRect().H)
	}
	add, rm, ex := p.Add.Bounds(), p.Remove.Bounds(), p.Extra[0].Bounds()
	if add.W != 32 || rm.X != add.X+add.W {
		t.Errorf("flat buttons are not butted at 32: add=%+v remove=%+v", add, rm)
	}
	// A separator, not a join, sits between the two groups.
	if ex.X <= rm.X+rm.W {
		t.Errorf("Extra %+v does not leave room for a separator after %+v", ex, rm)
	}
	if len(p.lay.seps) != 1 {
		t.Fatalf("separators = %d, want 1", len(p.lay.seps))
	}
	if s := p.lay.seps[0]; s.X < rm.X+rm.W || s.X+s.W > ex.X {
		t.Errorf("separator %+v is not between %+v and %+v", s, rm, ex)
	}
}

func TestListPaneWindowsAddMayCarryText(t *testing.T) {
	f := newListPaneFixture(ListBarWindows, false)
	f.pane.SetAddText("Add Tunnel")
	f.pane.SetBounds(f.pane.Bounds())
	if w := f.pane.Add.Bounds().W; w <= 32 {
		t.Errorf("Add with a caption is %d wide, want wider than an icon button", w)
	}
	if got := f.pane.Add.A11y().Name; got != "Add Tunnel" {
		t.Errorf("Add name = %q", got)
	}
}

func TestListPaneGNOMELayout(t *testing.T) {
	f := newListPaneFixture(ListBarGNOME, true)
	p := f.pane
	bar := p.BarRect()
	add := p.Add.Bounds()
	// 3-px padding inside the 1-px border, all round.
	if add.X != bar.X+1+3 || add.Y != bar.Y+1+3 {
		t.Errorf("Add at %+v, want 3 px of padding inside the bar %+v", add, bar)
	}
	if bar.H != 26+2*3+2 {
		t.Errorf("bar height = %d, want %d", bar.H, 26+2*3+2)
	}
	rm, ex := p.Remove.Bounds(), p.Extra[0].Bounds()
	if rm.X != add.X+add.W+1 {
		t.Errorf("linked buttons: Remove at %d, want one divider after Add's %d", rm.X, add.X+add.W)
	}
	if ex.X <= rm.X+rm.W+1 {
		t.Errorf("the Extra group is not spaced from the linked pair")
	}
}

func TestListPaneRoutesClicksByPosition(t *testing.T) {
	for _, st := range []ListBarStyle{ListBarMac, ListBarWindows, ListBarGNOME} {
		f := newListPaneFixture(st, true)
		p := f.pane

		x, y := centreOf(p.Add.Bounds())
		f.click(x, y)
		x, y = centreOf(p.Remove.Bounds())
		f.click(x, y)
		f.click(x, y)
		x, y = centreOf(p.Extra[0].Bounds())
		f.click(x, y)
		if f.adds != 1 || f.removes != 2 || f.extras != 1 {
			t.Errorf("style %d: add/remove/extra = %d/%d/%d, want 1/2/1", st, f.adds, f.removes, f.extras)
		}

		// The filler, the bar's padding and its separators react to nothing.
		before := [3]int{f.adds, f.removes, f.extras}
		if fl := p.FillerRect(); fl.W > 0 {
			f.click(centreOf(fl))
		}
		b := p.BarRect()
		f.click(b.X+b.W-1, b.Y+b.H-1)
		for _, s := range p.lay.seps {
			f.click(centreOf(s))
		}
		if after := [3]int{f.adds, f.removes, f.extras}; after != before {
			t.Errorf("style %d: the inert part of the bar fired a button: %v -> %v", st, before, after)
		}

		// A click on the list reaches the list, which selects the row.
		c := p.ContentRect()
		f.click(c.X+10, c.Y+4)
		if got := f.list.Selected().Get(); got != 0 {
			t.Errorf("style %d: list selection = %d after a click on its first row", st, got)
		}
	}
}

func TestListPaneDeleteFiresRemove(t *testing.T) {
	f := newListPaneFixture(ListBarWindows, false)
	f.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if f.removes != 1 {
		t.Errorf("Delete fired Remove %d times, want 1", f.removes)
	}
	// Backspace is the Mac's delete key, and only the Mac's.
	f.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if f.removes != 1 {
		t.Errorf("Backspace removed on a non-Mac pane")
	}
	m := newListPaneFixture(ListBarMac, false)
	m.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	m.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if m.removes != 2 {
		t.Errorf("Mac removes = %d, want 2", m.removes)
	}
	// Remove disabled: the key does nothing.
	m.pane.Remove.Disabled().Set(true)
	m.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	if m.removes != 2 {
		t.Errorf("Delete fired a disabled Remove")
	}
}

func TestListPaneDisabledIsInert(t *testing.T) {
	f := newListPaneFixture(ListBarMac, true)
	f.pane.Disabled().Set(true)
	x, y := centreOf(f.pane.Add.Bounds())
	f.click(x, y)
	f.pane.OnEvent(Event{Kind: EventKeyDown, Code: "Delete"})
	c := f.pane.ContentRect()
	f.click(c.X+10, c.Y+4)
	if f.adds+f.removes != 0 || f.list.Selected().Get() != -1 {
		t.Errorf("a disabled pane reacted: adds=%d removes=%d selected=%d", f.adds, f.removes, f.list.Selected().Get())
	}
}

func TestListPaneA11yTree(t *testing.T) {
	f := newListPaneFixture(ListBarMac, true)
	f.pane.AccessibleName = "Tunnels"
	nodes := WalkA11y(f.pane)
	var bar *A11yNode
	var names []string
	for i, n := range nodes {
		if n.Role == RoleToolbar {
			bar = &nodes[i]
		}
		if n.Role == RoleButton {
			names = append(names, n.Name)
		}
	}
	if nodes[0].Role != RoleGroup || nodes[0].Name != "Tunnels" {
		t.Errorf("root node = %+v, want the pane as a named group", nodes[0].A11yInfo)
	}
	if bar == nil || bar.Rect != f.pane.BarRect() {
		t.Fatalf("no toolbar node covering the bar: %+v", bar)
	}
	if want := []string{"Add", "Remove", "Export"}; len(names) != 3 || names[0] != want[0] || names[1] != want[1] || names[2] != want[2] {
		t.Errorf("button names = %v, want %v", names, want)
	}
	// Disabling the pane disables the bar, its buttons and the list.
	f.pane.Disabled().Set(true)
	for _, n := range WalkA11y(f.pane) {
		if !n.Disabled {
			t.Errorf("node %q (%s) is not announced disabled with its pane", n.Name, n.Role)
		}
	}
}

func TestListPaneFocusWalkReachesTheBarButtons(t *testing.T) {
	f := newListPaneFixture(ListBarMac, true)
	f.pane.Content = NewButton("c", nil) // a focusable content
	f.pane.SetBounds(f.pane.Bounds())
	if got := len(focusListOf(f.pane)); got != 4 {
		t.Fatalf("focusable descendants = %d, want 4 (content + Add, Remove, Export)", got)
	}
	f.pane.Disabled().Set(true)
	root := NewContainer(nil)
	root.AddWidget(f.pane)
	root.SetBounds(Rect{W: 300, H: 200})
	if got := len(focusListOf(root)); got != 0 {
		t.Errorf("a disabled pane offers %d focus stops", got)
	}
}

func TestListPaneWalkNativeReachesANativeContent(t *testing.T) {
	li := NewNativeList([]string{"un", "deux"}, 0)
	p := NewListPane(li, ListBarMac)
	p.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	got := WalkNative(p)
	if len(got) != 1 || got[0].Kind != NativeList {
		t.Fatalf("WalkNative found %+v, want the one native list", got)
	}
	if got[0].Rect != p.ContentRect() {
		t.Errorf("native list rect = %+v, want the content rect %+v", got[0].Rect, p.ContentRect())
	}
}

func TestListPaneNilContentAndZeroSize(t *testing.T) {
	p := NewListPane(nil, ListBarGNOME)
	p.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 5}) // smaller than its own bar
	buf := makeSurface(20, 20)
	p.Draw(newP(buf, 20), DefaultLight())
	p.SetBounds(Rect{})
	p.Draw(newP(buf, 20), DefaultLight())
	if n := len(p.Children()); n != 1 {
		t.Errorf("children = %d, want the bar only", n)
	}
	(&ListPane{}).Draw(newP(buf, 20), DefaultLight()) // a literal, never laid out
}

func TestListPaneSetAddTextWithoutAddIsANoOp(t *testing.T) {
	p := NewListPane(nil, ListBarWindows)
	p.Add = nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("SetAddText without an Add panicked: %v", r)
		}
	}()
	p.SetAddText("Add Tunnel")
}

func TestListPaneButtonAtFindsTheButtonUnderAPaneLocalPoint(t *testing.T) {
	f := newListPaneFixture(ListBarMac, true)
	r := f.pane.Bounds()
	for _, b := range []*Button{f.pane.Add, f.pane.Remove, f.pane.Extra[0]} {
		sx, sy := centreOf(b.Bounds())
		if got := f.pane.ButtonAt(sx-r.X, sy-r.Y); got != b {
			t.Errorf("ButtonAt(%d,%d) = %p, want %p", sx-r.X, sy-r.Y, got, b)
		}
	}
	if got := f.pane.ButtonAt(5, 5); got != nil {
		t.Errorf("ButtonAt over the list = %v, want nil", got)
	}
}

func TestListPaneRelaysOutWhenButtonsWereAddedAfterTheLastLayout(t *testing.T) {
	f := newListPaneFixture(ListBarMac, false)
	f.pane.Extra = []*Button{NewButton("x", nil)}
	buf := makeSurface(240, 170)
	f.pane.Draw(newP(buf, 240), DefaultLight())
	if got := len(f.pane.lay.buttons); got != 3 {
		t.Errorf("laid-out buttons = %d, want 3 after Draw", got)
	}
}

func TestListPaneDrawsEveryButtonState(t *testing.T) {
	th := DefaultLight()
	for _, st := range []ListBarStyle{ListBarMac, ListBarWindows, ListBarGNOME} {
		face := func(setup func(f *listPaneFixture)) RGBA {
			f := newListPaneFixture(st, false)
			setup(f)
			const w, h = 240, 170
			buf := makeSurface(w, h)
			f.pane.Draw(newP(buf, w), th)
			b := f.pane.Add.Bounds() // a pixel at the top edge, off the glyph
			return pixelAt(buf, w, b.X+b.W/2, b.Y+2)
		}
		rest := face(func(*listPaneFixture) {})
		sel := face(func(f *listPaneFixture) { f.pane.Add.Selected().Set(true) })
		prs := face(func(f *listPaneFixture) { f.pane.Add.pressed = true })
		hov := face(func(f *listPaneFixture) { f.pane.Add.hovered = true })
		dis := face(func(f *listPaneFixture) { f.pane.Add.Disabled().Set(true) })
		if sel != th.Accent || prs != th.Accent {
			t.Errorf("style %d: selected %v / pressed %v, want Accent %v", st, sel, prs, th.Accent)
		}
		if hov == rest {
			t.Errorf("style %d: hover face equals the resting face %v", st, rest)
		}
		if dis != th.SurfaceAlt {
			t.Errorf("style %d: disabled face %v, want SurfaceAlt %v", st, dis, th.SurfaceAlt)
		}
	}
}

func TestListPanePointerAndCharEventsReachTheirTargets(t *testing.T) {
	f := newListPaneFixture(ListBarMac, true)
	r := f.pane.Bounds()
	sx, sy := centreOf(f.pane.Add.Bounds())
	lx, ly := sx-r.X, sy-r.Y
	f.pane.OnEvent(Event{Kind: EventMouseMove, X: lx, Y: ly})
	if !f.pane.Add.hovered {
		t.Error("a move over Add did not hover it")
	}
	f.pane.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})
	if f.pane.Add.hovered {
		t.Error("a move away did not clear Add's hover")
	}
	f.pane.OnEvent(Event{Kind: EventMouseUp, X: lx, Y: ly})
	f.pane.OnEvent(Event{Kind: EventMouseDrag, X: 12, Y: 12})
	f.pane.OnEvent(Event{Kind: EventChar, Code: "a"})
}

func TestDrawIconMinusFitsAWideBoxToItsHeight(t *testing.T) {
	ink := RGB(0xFF, 0, 0)
	const w, h = 40, 14
	buf := makeSurface(w, h)
	DrawIconMinus(newP(buf, w), Rect{W: w, H: h}, ink)
	minX, maxX := w, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if pixelAt(buf, w, x, y) == ink {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	if maxX < 0 {
		t.Fatal("nothing drawn")
	}
	if got := maxX - minX + 1; got > h {
		t.Errorf("bar is %d px wide in a %d px high box, want at most the height", got, h)
	}
}

func TestListPaneEmptyBoundsPaintNothing(t *testing.T) {
	f := newListPaneFixture(ListBarGNOME, true)
	f.pane.SetBounds(Rect{X: 5, Y: 5})
	buf := makeSurface(40, 40)
	before := append([]byte(nil), buf...)
	f.pane.Draw(newP(buf, 40), DefaultLight())
	if string(before) != string(buf) {
		t.Error("a pane with empty bounds painted pixels")
	}
}

func TestListPaneDrawsADistinctBarInsideAFrame(t *testing.T) {
	th := DefaultLight()
	if th.SurfaceAlt == th.Surface {
		t.Skip("theme has no distinct bar colour")
	}
	for _, st := range []ListBarStyle{ListBarMac, ListBarWindows, ListBarGNOME} {
		f := newListPaneFixture(st, false)
		const w, h = 240, 170
		buf := makeSurface(w, h)
		f.pane.Draw(newP(buf, w), th)
		p := f.pane
		// The frame's hairline on the left edge, mid-list.
		if got := pixelAt(buf, w, 10, 60); got != th.Border {
			t.Errorf("style %d: frame edge = %v, want Border %v", st, got, th.Border)
		}
		// Outside the pane nothing is painted (the sentinel survives).
		if got := pixelAt(buf, w, 5, 60); got != (RGBA{R: 0xC8, G: 0xC8, B: 0xC8, A: 0xFF}) {
			t.Errorf("style %d: painted outside the pane: %v", st, got)
		}
		// The bar is SurfaceAlt, where the filler is (or the bar's right end).
		fl := p.FillerRect()
		if got := pixelAt(buf, w, fl.X+fl.W/2, fl.Y+fl.H/2); got != th.SurfaceAlt {
			t.Errorf("style %d: bar face = %v, want SurfaceAlt %v", st, got, th.SurfaceAlt)
		}
		// The bar's top edge is a Border line shared with the list.
		if got := pixelAt(buf, w, fl.X+fl.W/2, p.BarRect().Y); got != th.Border {
			t.Errorf("style %d: bar top edge = %v, want Border", st, got)
		}
		// The empty area of the list is Surface, a colour the bar does not share.
		c := p.ContentRect()
		if got := pixelAt(buf, w, c.X+c.W-3, c.Y+c.H-3); got != th.Surface {
			t.Errorf("style %d: list face = %v, want Surface %v", st, got, th.Surface)
		}
	}
	// Mac: the shared divider between Add and Remove is a Border column.
	f := newListPaneFixture(ListBarMac, false)
	buf := makeSurface(240, 170)
	f.pane.Draw(newP(buf, 240), th)
	if got := pixelAt(buf, 240, 37, 148); got != th.Border {
		t.Errorf("Mac divider = %v, want Border", got)
	}
	// GNOME: the bottom corners are rounded, so the outermost pixel is not the bar.
	g := newListPaneFixture(ListBarGNOME, false)
	buf = makeSurface(240, 170)
	g.pane.Draw(newP(buf, 240), th)
	if got := pixelAt(buf, 240, 10, 159); got == th.SurfaceAlt || got == th.Border {
		t.Errorf("GNOME bottom-left corner pixel = %v, want it cut away by the radius", got)
	}
}

func TestDrawIconMinusMatchesThePlusBar(t *testing.T) {
	ink := RGB(0xFF, 0, 0)
	for _, side := range []int{12, 13, 18, 24, 25} {
		pl, mi := makeSurface(side, side), makeSurface(side, side)
		box := Rect{W: side, H: side}
		DrawIconPlus(newP(pl, side), box, ink)
		DrawIconMinus(newP(mi, side), box, ink)
		// Every minus pixel is a plus pixel, and it is the plus's horizontal bar.
		n := 0
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				m := pixelAt(mi, side, x, y) == ink
				if m {
					n++
					if pixelAt(pl, side, x, y) != ink {
						t.Fatalf("side %d: minus pixel (%d,%d) is not in the plus", side, x, y)
					}
				}
			}
		}
		if n == 0 {
			t.Fatalf("side %d: the minus painted nothing", side)
		}
		// Symmetric about both axes.
		for y := 0; y < side; y++ {
			for x := 0; x < side; x++ {
				if (pixelAt(mi, side, x, y) == ink) != (pixelAt(mi, side, side-1-x, side-1-y) == ink) {
					t.Fatalf("side %d: the minus is not centrally symmetric at (%d,%d)", side, x, y)
				}
			}
		}
	}
	DrawIconMinus(newP(makeSurface(4, 4), 4), Rect{W: 1, H: 1}, ink) // degenerate: no panic
}

// TestListPaneWritesPNGs renders the three styles to LISTPANE_PNG_DIR when it is
// set (a directory OUTSIDE any repository), for a person to look at.
func TestListPaneWritesPNGs(t *testing.T) {
	dir := os.Getenv("LISTPANE_PNG_DIR")
	if dir == "" {
		t.Skip("LISTPANE_PNG_DIR not set")
	}
	names := map[ListBarStyle]string{ListBarMac: "mac", ListBarWindows: "windows", ListBarGNOME: "gnome"}
	for st, name := range names {
		f := newListPaneFixture(st, st == ListBarWindows)
		f.list.Selected().Set(1)
		if st == ListBarWindows {
			f.pane.SetAddText("Add Tunnel")
			f.pane.SetBounds(f.pane.Bounds())
		}
		const w, h = 240, 170
		buf := makeSurface(w, h)
		f.pane.Draw(newP(buf, w), DefaultLight())
		img := &image.NRGBA{Pix: buf, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
		fh, err := os.Create(filepath.Join(dir, "listpane-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(fh, img); err != nil {
			t.Fatal(err)
		}
		fh.Close()
	}
}

// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ide builds the shape the playground uses: Files (fixed, collapsible), an
// editor and a preview (flexible, 6:5), Help (fixed), in a 1200x400 splitter.
type ide struct {
	s                    *Splitter
	files, help          *mvvm.Observable[int]
	editor, preview      *mvvm.Observable[int]
	filesOn, helpOn      *mvvm.Observable[bool]
	resized              [][]int
	fw, ew, pw, hw       Widget
	now                  time.Time
	filesPane, helpPane  *SplitPane
	editorPane, prevPane *SplitPane
}

func newIDE(t *testing.T, w int) *ide {
	t.Helper()
	d := &ide{
		files: mvvm.NewObservable(190), help: mvvm.NewObservable(280),
		editor: mvvm.NewObservable(600), preview: mvvm.NewObservable(500),
		filesOn: mvvm.NewObservable(true), helpOn: mvvm.NewObservable(true),
		fw: NewLabel("files"), ew: NewLabel("editor"), pw: NewLabel("preview"), hw: NewLabel("help"),
		now: time.Unix(1000, 0),
	}
	d.filesPane = &SplitPane{Widget: d.fw, Name: "Files", Size: d.files, Min: 120, Max: 480, Collapsible: true, Visible: d.filesOn}
	d.editorPane = &SplitPane{Widget: d.ew, Name: "Editor", Weight: d.editor, Min: 200}
	d.prevPane = &SplitPane{Widget: d.pw, Name: "Preview", Weight: d.preview, Min: 200}
	d.helpPane = &SplitPane{Widget: d.hw, Name: "Help", Size: d.help, Min: 180, Max: 560, Visible: d.helpOn}
	d.s = NewSplitter(false, d.filesPane, d.editorPane, d.prevPane, d.helpPane)
	d.s.Now = func() time.Time { return d.now }
	d.s.OnResized = func(ch []int) { d.resized = append(d.resized, ch) }
	d.s.SetBounds(Rect{W: w, H: 400})
	return d
}

func (d *ide) sizes() [4]int {
	return [4]int{d.s.PaneSize(0), d.s.PaneSize(1), d.s.PaneSize(2), d.s.PaneSize(3)}
}

// drag presses divider j, moves by dx and releases.
func (d *ide) drag(t *testing.T, j, dx int) {
	t.Helper()
	r, ok := d.s.HandleRect(j)
	if !ok {
		t.Fatalf("no divider %d", j)
	}
	x := r.X + r.W/2
	d.s.OnEvent(Event{Kind: EventClick, X: x, Y: 50})
	d.s.OnEvent(Event{Kind: EventMouseDrag, X: x + dx/2, Y: 50})
	d.s.OnEvent(Event{Kind: EventMouseDrag, X: x + dx, Y: 50})
	d.s.OnEvent(Event{Kind: EventMouseUp, X: x + dx, Y: 50})
}

func TestSplitterDefaultLayout(t *testing.T) {
	d := newIDE(t, 1200)
	got := d.sizes()
	if got[0] != 190 || got[3] != 280 {
		t.Fatalf("fixed panes %v, want 190 and 280", got)
	}
	if sum := got[0] + got[1] + got[2] + got[3] + 3*SplitterHandleW; sum != 1200 {
		t.Fatalf("panes and dividers fill %d of 1200", sum)
	}
	if r := float64(got[1]) / float64(got[2]); r < 1.19 || r > 1.21 {
		t.Fatalf("editor:preview is %v, want 6:5", r)
	}
	// the panes sit left to right with a divider between
	x := 0
	for i, w := range []Widget{d.fw, d.ew, d.pw, d.hw} {
		if b := w.Bounds(); b.X != x || b.W != got[i] || b.H != 400 {
			t.Fatalf("pane %d at %+v, want x=%d w=%d", i, b, x, got[i])
		}
		x += got[i] + SplitterHandleW
	}
	if len(d.resized) != 0 {
		t.Fatalf("a layout is not a gesture: OnResized got %v", d.resized)
	}
}

func TestSplitterDragMovesTheFixedPaneAndTheNeighbourAbsorbs(t *testing.T) {
	d := newIDE(t, 1200)
	before := d.sizes()
	d.drag(t, 0, 50)
	got := d.sizes()
	if got[0] != before[0]+50 || got[1] != before[1]-50 || got[2] != before[2] || got[3] != before[3] {
		t.Fatalf("after dragging the Files divider by 50: %v, was %v", got, before)
	}
	if d.files.Get() != 240 {
		t.Fatalf("Files size observable is %d, want 240", d.files.Get())
	}
	if len(d.resized) != 1 || len(d.resized[0]) != 2 || d.resized[0][0] != 0 || d.resized[0][1] != 1 {
		t.Fatalf("OnResized should report panes 0 and 1 once, got %v", d.resized)
	}
	// the divider after the preview moves Help, taking from the preview
	d.drag(t, 2, -40)
	got2 := d.sizes()
	if got2[3] != got[3]+40 || got2[2] != got[2]-40 || got2[1] != got[1] {
		t.Fatalf("after dragging the Help divider by -40: %v, was %v", got2, got)
	}
	if d.help.Get() != 320 {
		t.Fatalf("Help size observable is %d, want 320", d.help.Get())
	}
}

func TestSplitterFlexiblePairChangesTheRatioNotTheTotal(t *testing.T) {
	d := newIDE(t, 1200)
	before := d.sizes()
	d.drag(t, 1, -100)
	got := d.sizes()
	if got[1] != before[1]-100 || got[2] != before[2]+100 {
		t.Fatalf("editor/preview %v then %v after -100", before, got)
	}
	if got[0] != before[0] || got[3] != before[3] {
		t.Fatalf("a flexible pair must leave the fixed panes alone: %v then %v", before, got)
	}
	// the ratio, not the pixels, is kept when the window grows
	d.s.SetBounds(Rect{W: 1500, H: 400})
	g2 := d.sizes()
	r1 := float64(got[1]) / float64(got[2])
	r2 := float64(g2[1]) / float64(g2[2])
	if r2 < r1-0.02 || r2 > r1+0.02 {
		t.Fatalf("the editor:preview ratio %v became %v when the window grew", r1, r2)
	}
	if g2[0] != 190 || g2[3] != 280 {
		t.Fatalf("fixed panes changed with the window: %v", g2)
	}
	if len(d.resized) != 1 {
		t.Fatalf("a window resize is not a gesture; OnResized called %d times", len(d.resized))
	}
}

func TestSplitterBoundsStopTheDivider(t *testing.T) {
	d := newIDE(t, 1200)
	d.drag(t, 0, 2000)
	// the editor next to it gives way down to its Min 200 and no further: the
	// divider stops there, under Files' Max
	if got := d.sizes(); got[1] != 200 || got[0] != 190+382-200 || got[2] != 318 {
		t.Fatalf("Files must stop where the editor reaches its Min: %v", got)
	}
	d = newIDE(t, 2000)
	d.drag(t, 0, 2000)
	if got := d.sizes(); got[0] != 480 {
		t.Fatalf("Files must stop at its Max 480 when there is room: %v", got)
	}
	d = newIDE(t, 1200)
	d.drag(t, 2, 2000) // Help shrinks to its Min
	if got := d.sizes(); got[3] != 180 {
		t.Fatalf("Help must stop at its Min 180: %v", got)
	}
	d = newIDE(t, 1200)
	d.drag(t, 1, -2000) // the editor is a flexible pane with a Min
	if got := d.sizes(); got[1] != 200 {
		t.Fatalf("the editor must stop at its Min 200: %v", got)
	}
}

func TestSplitterCollapseByDragAndByDoubleClick(t *testing.T) {
	d := newIDE(t, 1200)
	before := d.sizes()
	d.drag(t, 0, -150) // below half the Min: collapses
	if !d.s.Collapsed(0) || d.files.Get() != 0 || d.s.PaneSize(0) != 0 {
		t.Fatalf("Files should be collapsed: size %d, observable %d", d.s.PaneSize(0), d.files.Get())
	}
	if b := d.fw.Bounds(); b.W != 0 {
		t.Fatalf("a collapsed pane has no room: %+v", b)
	}
	if got := d.sizes(); got[1] != before[1]+before[0] {
		t.Fatalf("the editor should take the Files room: %v, was %v", got, before)
	}
	if _, ok := d.s.HandleRect(0); !ok {
		t.Fatal("the divider of a collapsed pane stays, to bring it back")
	}
	// a double-click on it brings the pane back to what it was
	d.now = d.now.Add(time.Second)
	r, _ := d.s.HandleRect(0)
	x := r.X + r.W/2
	d.s.OnEvent(Event{Kind: EventClick, X: x, Y: 10})
	d.s.OnEvent(Event{Kind: EventMouseUp, X: x, Y: 10})
	d.now = d.now.Add(100 * time.Millisecond)
	d.s.OnEvent(Event{Kind: EventClick, X: x, Y: 10})
	d.s.OnEvent(Event{Kind: EventMouseUp, X: x, Y: 10})
	if d.s.Collapsed(0) || d.files.Get() != 190 {
		t.Fatalf("a double-click should restore Files to 190: collapsed=%v size=%d", d.s.Collapsed(0), d.files.Get())
	}
	// and a double-click again collapses it
	r, _ = d.s.HandleRect(0)
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 1, Y: 10, Code: "double"})
	if !d.s.Collapsed(0) {
		t.Fatal("an event tagged double should collapse Files")
	}
	// dragging out of the collapsed state restores a size within the bounds
	d.drag(t, 0, 160)
	if d.s.Collapsed(0) || d.files.Get() < 120 {
		t.Fatalf("dragging a collapsed pane out gives it at least its Min: %d", d.files.Get())
	}
}

func TestSplitterNonCollapsiblePaneStopsAtItsMin(t *testing.T) {
	d := newIDE(t, 1200)
	d.drag(t, 2, 1000)
	if d.s.Collapsed(3) || d.help.Get() != 180 {
		t.Fatalf("Help is not collapsible: %d", d.help.Get())
	}
	r, _ := d.s.HandleRect(2)
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 1, Y: 10, Code: "double"})
	if d.s.Collapsed(3) || d.help.Get() != 180 {
		t.Fatal("a double-click does nothing to a pane that cannot collapse")
	}
}

func TestSplitterKeyboard(t *testing.T) {
	d := newIDE(t, 1200)
	if !d.s.FocusHandle(0) || d.s.FocusedHandle() != 0 {
		t.Fatal("divider 0 should take the focus")
	}
	key := func(code string, shift bool) {
		d.s.OnEvent(Event{Kind: EventKeyDown, Code: code, Shift: shift})
	}
	key("ArrowRight", false)
	if d.files.Get() != 190+SplitterStep {
		t.Fatalf("ArrowRight: Files is %d, want %d", d.files.Get(), 190+SplitterStep)
	}
	key("ArrowLeft", false)
	key("ArrowLeft", true)
	if d.files.Get() != 190-4*SplitterStep {
		t.Fatalf("Shift+ArrowLeft moves four steps: Files is %d", d.files.Get())
	}
	key("End", false)
	if d.files.Get() != 190+382-200 || d.s.PaneSize(1) != 200 {
		t.Fatalf("End: Files is %d, want the most the editor can give (372)", d.files.Get())
	}
	key("Home", false)
	if !d.s.Collapsed(0) {
		t.Fatalf("Home takes a collapsible pane to nothing: %d", d.files.Get())
	}
	key("Enter", false)
	if d.s.Collapsed(0) || d.files.Get() != 372 {
		t.Fatalf("Enter brings it back to the size it had: %d", d.files.Get())
	}
	key("Enter", false)
	if !d.s.Collapsed(0) {
		t.Fatal("Enter collapses an open pane")
	}
	if n := len(d.resized); n != 7 {
		t.Fatalf("every key that moved something is a gesture: %d, want 7", n)
	}
	before := len(d.resized)
	key("a", false)
	key("ArrowUp", false) // not this splitter's axis
	if len(d.resized) != before {
		t.Fatal("other keys are not for the divider")
	}
}

func TestSplitterTabMovesBetweenDividersAndPanes(t *testing.T) {
	d := newIDE(t, 1200)
	d.s.FocusHandle(0)
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if d.s.FocusedHandle() != 1 {
		t.Fatalf("Tab should go to the next divider, got %d", d.s.FocusedHandle())
	}
	d.s.OnEvent(Event{Kind: EventKeyDown, Code: "Tab", Shift: true})
	if d.s.FocusedHandle() != 0 {
		t.Fatalf("Shift+Tab should come back, got %d", d.s.FocusedHandle())
	}
	d.s.Blur()
	if d.s.FocusedHandle() != -1 {
		t.Fatal("Blur takes the focus off")
	}
	r, _ := d.s.HandleRect(1)
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 2, Y: 20})
	d.s.OnEvent(Event{Kind: EventMouseUp, X: r.X + 2, Y: 20})
	if d.s.FocusedHandle() != 1 {
		t.Fatalf("a press on a divider focuses it, got %d", d.s.FocusedHandle())
	}
}

func TestSplitterPaneSwitchedOffTakesNoRoomAndNoDivider(t *testing.T) {
	d := newIDE(t, 1200)
	d.filesOn.Set(false)
	if b := d.fw.Bounds(); b.W != 0 || b.H != 0 {
		t.Fatalf("a hidden pane has empty bounds: %+v", b)
	}
	if got := d.sizes(); got[0] != 0 || got[1]+got[2]+got[3]+2*SplitterHandleW != 1200 {
		t.Fatalf("the other panes fill the width: %v", got)
	}
	if _, ok := d.s.HandleRect(2); ok {
		t.Fatal("with Files off there are two dividers fewer... only 0..1 are in use")
	}
	if r, ok := d.s.HandleRect(0); !ok || r.X != d.ew.Bounds().X+d.ew.Bounds().W {
		t.Fatalf("divider 0 should now sit after the editor: %+v %v", r, ok)
	}
	if n := len(d.s.Children()); n != 3 {
		t.Fatalf("Children are the panes that are on: %d", n)
	}
	d.filesOn.Set(true)
	if d.files.Get() != 190 || d.s.PaneSize(0) != 190 {
		t.Fatal("switching a pane back on gives it the size it had")
	}
	// a hidden pane hears nothing
	d.helpOn.Set(false)
	if d.s.PaneSize(3) != 0 {
		t.Fatal("Help is off")
	}
}

func TestSplitterObservablesAreTheModel(t *testing.T) {
	d := newIDE(t, 1200)
	d.files.Set(300)
	if d.s.PaneSize(0) != 300 || d.fw.Bounds().W != 300 {
		t.Fatalf("setting the Size observable relays out: %d", d.s.PaneSize(0))
	}
	d.files.Set(9999) // out of range: held at the Max
	if d.s.PaneSize(0) != 480 {
		t.Fatalf("a size beyond Max is held at it: %d", d.s.PaneSize(0))
	}
	d.files.Set(190)
	d.editor.Set(1)
	d.preview.Set(1)
	if a, b := d.s.PaneSize(1), d.s.PaneSize(2); a-b > 1 || b-a > 1 {
		t.Fatalf("equal weights share equally: %d and %d", a, b)
	}
	if len(d.resized) != 0 {
		t.Fatal("writing the model is not a gesture")
	}
}

func TestSplitterSmallWindowSqueezesFixedPanesFirst(t *testing.T) {
	d := newIDE(t, 760)
	got := d.sizes()
	if got[1] < 200 || got[2] < 200 {
		t.Fatalf("the flexible panes keep their Min while the fixed ones give way: %v", got)
	}
	if got[0] < 120 || got[3] < 180 {
		t.Fatalf("the fixed panes are squeezed to their Min at most: %v", got)
	}
	if sum := got[0] + got[1] + got[2] + got[3] + 3*SplitterHandleW; sum != 760 {
		t.Fatalf("the panes fill 760: %d", sum)
	}
	if d.files.Get() != 190 || d.help.Get() != 280 {
		t.Fatal("a squeeze is not stored: the sizes come back with the room")
	}
	d.s.SetBounds(Rect{W: 1200, H: 400})
	if got := d.sizes(); got[0] != 190 || got[3] != 280 {
		t.Fatalf("the room came back: %v", got)
	}
	// even a window too small for the Mins is filled, not overrun
	d.s.SetBounds(Rect{W: 300, H: 400})
	if sum := d.sizes(); sum[0]+sum[1]+sum[2]+sum[3]+3*SplitterHandleW > 300 {
		t.Fatalf("a tiny window must not be overrun: %v", sum)
	}
}

func TestSplitterVertical(t *testing.T) {
	top, bottom := NewLabel("top"), NewLabel("bottom")
	h := mvvm.NewObservable(100)
	s := NewSplitter(true,
		&SplitPane{Widget: top, Name: "Top", Weight: mvvm.NewObservable(1), Min: 50},
		&SplitPane{Widget: bottom, Name: "Bottom", Size: h, Min: 40, Max: 300})
	s.SetBounds(Rect{X: 5, Y: 7, W: 300, H: 500})
	if b := bottom.Bounds(); b.H != 100 || b.W != 300 || b.Y != 7+500-100 {
		t.Fatalf("bottom at %+v", b)
	}
	r, ok := s.HandleRect(0)
	if !ok || r.W != 300 || r.H != SplitterHandleW || r.Y != top.Bounds().Y+top.Bounds().H {
		t.Fatalf("horizontal bar at %+v", r)
	}
	if c := s.Cursor(10, r.Y-7+2); c != "row-resize" {
		t.Fatalf("cursor over a horizontal bar is %q", c)
	}
	s.FocusHandle(0)
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowUp"})
	if h.Get() != 100+SplitterStep {
		t.Fatalf("ArrowUp grows the pane below: %d", h.Get())
	}
	s.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if h.Get() != 100+SplitterStep {
		t.Fatal("a vertical splitter ignores the horizontal arrows")
	}
	// a drag in the splitter's own coordinates, whatever its origin
	r, _ = s.HandleRect(0) // the keys moved it
	s.OnEvent(Event{Kind: EventClick, X: 10, Y: r.Y - 7 + 2})
	s.OnEvent(Event{Kind: EventMouseDrag, X: 10, Y: r.Y - 7 + 2 + 30})
	s.OnEvent(Event{Kind: EventMouseUp, X: 10, Y: r.Y - 7 + 2 + 30})
	if h.Get() != 100+SplitterStep-30 {
		t.Fatalf("dragging down by 30 shrinks the pane below: %d", h.Get())
	}
}

func TestSplitterCursorAndHover(t *testing.T) {
	d := newIDE(t, 1200)
	r, _ := d.s.HandleRect(0)
	if c := d.s.Cursor(r.X+2, 30); c != "col-resize" {
		t.Fatalf("cursor over a divider: %q", c)
	}
	if c := d.s.Cursor(r.X-30, 30); c != "" {
		t.Fatalf("cursor over a pane: %q", c)
	}
	d.s.OnEvent(Event{Kind: EventClick, X: r.X + 2, Y: 30})
	if c := d.s.Cursor(r.X+500, 30); c != "col-resize" {
		t.Fatalf("while a divider is held the cursor stays: %q", c)
	}
	if !d.s.Dragging().Get() {
		t.Fatal("Dragging should be true while held")
	}
	d.s.OnEvent(Event{Kind: EventMouseUp, X: r.X + 2, Y: 30})
	if d.s.Dragging().Get() {
		t.Fatal("Dragging should end on release")
	}
}

func TestSplitterAccessibleSeparator(t *testing.T) {
	d := newIDE(t, 1200)
	a, ok := d.s.Handle(0).(Accessible)
	if !ok {
		t.Fatal("a divider is Accessible")
	}
	info := a.A11y()
	if info.Role != RoleSeparator || info.Name != "Resize Files" || !info.HasRange {
		t.Fatalf("separator info %+v", info)
	}
	if info.Min != 0 || info.Max != 480 || info.Now != 190 {
		t.Fatalf("aria-valuemin/max/now are 0/480/190 for a collapsible Files at 190: %+v", info)
	}
	if f, ok := d.s.Handle(0).(Focusable); !ok || f == nil {
		t.Fatal("a divider is Focusable")
	}
}

func TestSplitterContrastIsAtLeastThreeToOneInBothThemes(t *testing.T) {
	for name, th := range map[string]*Theme{"light": DefaultLight(), "dark": DefaultDark()} {
		ink, active := SplitterColors(th)
		for what, c := range map[string]RGBA{"line": ink, "accent": active} {
			for gname, g := range map[string]RGBA{"Background": th.Background, "Surface": th.Surface} {
				if r := ContrastRatio(c, g); r < 3 {
					t.Errorf("%s theme: the divider's %s is %.2f:1 against %s, want 3:1", name, what, r, gname)
				}
			}
		}
	}
	if r := ContrastRatio(RGB(0, 0, 0), RGB(255, 255, 255)); r < 20.9 || r > 21.1 {
		t.Fatalf("black on white is 21:1, got %v", r)
	}
}

func TestSplitterDrawsDividersAndRing(t *testing.T) {
	d := newIDE(t, 1200)
	th := DefaultLight()
	paint := func() []byte {
		buf := make([]byte, 1200*400*4)
		d.s.Draw(painter.NewPixelPainter(buf, 1200, 400), th)
		return buf
	}
	plain := paint()
	d.s.FocusHandle(0)
	focused := paint()
	same := true
	for i := range plain {
		if plain[i] != focused[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("a focused divider must draw a ring")
	}
	r, _ := d.s.HandleRect(0)
	ink, _ := SplitterColors(th)
	px := (r.H/2*1200 + r.X + r.W/2) * 4
	// the grip is centred in the divider, in the ink colour at rest
	d.s.Blur()
	rest := paint()
	if rest[px] != ink.R || rest[px+1] != ink.G || rest[px+2] != ink.B {
		t.Fatalf("the divider's centre is %v, want the ink %v", rest[px:px+3], ink)
	}
}

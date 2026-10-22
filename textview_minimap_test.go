// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func newMinimapEditor(n int) *CodeEditor {
	c := NewCodeEditor(scrollDoc(n, 0, 0))
	c.SetBounds(Rect{X: 5, Y: 7, W: 400, H: 700})
	c.ShowMinimap = true
	return c
}

func TestMinimapOffByDefaultAndWhenNarrow(t *testing.T) {
	c := NewCodeEditor(scrollDoc(100, 0, 0))
	c.SetBounds(Rect{W: 400, H: 700})
	if c.minimapW() != 0 {
		t.Fatal("CodeEditor must not show a minimap unless asked")
	}
	c.ShowMinimap = true
	if c.minimapW() != scaled(minimapWidth) {
		t.Fatalf("minimapW = %d", c.minimapW())
	}
	c.SetBounds(Rect{W: 3 * scaled(minimapWidth), H: 240})
	if c.minimapW() != 0 {
		t.Fatal("a window narrower than four minimap widths must hide it")
	}
	plain := NewTextView(scrollDoc(100, 0, 0))
	plain.ShowMinimap = true // needs ShowScrollbars
	plain.SetBounds(Rect{W: 400, H: 240})
	if plain.minimapW() != 0 {
		t.Fatal("a minimap needs ShowScrollbars")
	}
}

// The minimap sits left of the vertical bar and takes its columns from the text
// window; the horizontal bar and the text stop at it.
func TestMinimapLayoutTakesItsOwnColumns(t *testing.T) {
	c := newMinimapEditor(1000)
	rc := c.minimapRect()
	vg, _ := c.vbarGeom()
	if rc.X+rc.W != vg.cross0 {
		t.Fatalf("minimap ends at %d, the vertical bar starts at %d", rc.X+rc.W, vg.cross0)
	}
	if got, want := c.textAreaWidth(), rc.X-c.textLeftInset(); got != want {
		t.Fatalf("textAreaWidth = %d, want %d (up to the minimap)", got, want)
	}
	off := newMinimapEditor(1000)
	off.ShowMinimap = false
	if off.textAreaWidth()-c.textAreaWidth() != scaled(minimapWidth) {
		t.Fatal("the minimap must cost exactly its own width")
	}
	// A wide document's horizontal track also ends at the minimap.
	w := NewCodeEditor(scrollDoc(1000, 1, 400))
	w.SetBounds(Rect{W: 400, H: 240})
	w.ShowMinimap = true
	hg, _ := w.hbarGeom()
	if hg.trackStart+hg.trackLen != w.minimapRect().X {
		t.Fatalf("horizontal track ends at %d, minimap at %d", hg.trackStart+hg.trackLen, w.minimapRect().X)
	}
}

func TestMinimapClickCentresAndSliderDragKeepsGrab(t *testing.T) {
	c := newMinimapEditor(150) // fits the minimap without sampling
	rc := c.minimapRect()
	_ = c.syncMinimap()
	rowH := c.minimap.rowH()
	vis := c.visibleLines()
	// Click on the row of line 100: the window centres there.
	c.OnEvent(Event{Kind: EventClick, X: rc.X + 5, Y: rc.Y + 100*rowH + 1})
	if c.CursorLine().Get() != 0 {
		t.Fatal("a minimap click moved the caret")
	}
	if got, want := c.ScrollLine().Get(), 100-vis/2; got != want {
		t.Fatalf("centred click: ScrollLine = %d, want %d", got, want)
	}
	// Press ON the slider 3 rows below its top, drag 20 rows down: the top follows
	// by 20, so the pointer stays on the same spot of the slider.
	top := c.ScrollLine().Get()
	c.OnEvent(Event{Kind: EventClick, X: rc.X + 5, Y: rc.Y + (top+3)*rowH + 1})
	if c.ScrollLine().Get() != top {
		t.Fatal("pressing on the slider must not jump")
	}
	c.OnEvent(Event{Kind: EventMouseDrag, X: rc.X + 5, Y: rc.Y + (top+3+20)*rowH + 1})
	if got := c.ScrollLine().Get(); got != c.maxScrollLine() && got != top+20 {
		t.Fatalf("slider drag: ScrollLine = %d, want %d", got, top+20)
	}
	c.OnEvent(Event{Kind: EventMouseUp})
	if c.mmDrag {
		t.Fatal("release must end the drag")
	}
	// Past the ends it clamps.
	c.OnEvent(Event{Kind: EventClick, X: rc.X + 5, Y: rc.Y + 2})
	if c.ScrollLine().Get() != 0 {
		t.Fatalf("click at the top: ScrollLine = %d", c.ScrollLine().Get())
	}
	c.OnEvent(Event{Kind: EventMouseUp})
	// A click left of the minimap is a text click, not a minimap press.
	c.OnEvent(Event{Kind: EventClick, X: rc.X - 4, Y: 40})
	if c.mmDrag {
		t.Fatal("a click in the text started a minimap drag")
	}
}

func TestMinimapSamplesLongDocumentsAndStaysMonotonic(t *testing.T) {
	c := newMinimapEditor(20000)
	rc := c.minimapRect()
	_ = c.syncMinimap()
	prev := -1
	for y := rc.Y; y < rc.Y+rc.H; y += 2 {
		c.OnEvent(Event{Kind: EventClick, X: rc.X + 5, Y: y})
		got := c.ScrollLine().Get()
		if got < prev {
			t.Fatalf("minimap not monotonic: y=%d -> %d after %d", y, got, prev)
		}
		prev = got
		c.OnEvent(Event{Kind: EventMouseUp})
	}
	if prev < 10000 {
		t.Fatalf("the bottom of the overview reached only line %d of 20000", prev)
	}
}

func TestMinimapMarksAndMatchesDrawn(t *testing.T) {
	for name, th := range popupThemes() {
		c := newMinimapEditor(100)
		c.SetMinimapMarks([]MinimapMark{{Line: 10, Kind: MarkError}, {Line: 20, Kind: MarkWarning}, {Line: 9999}, {Line: -1}})
		if len(c.MinimapMarks()) != 4 {
			t.Fatal("MinimapMarks must return what was set")
		}
		c.SetMatchHighlights([]Selection{{StartLine: 30, StartCol: 0, EndLine: 30, EndCol: 3}})
		c.ScrollLine().Set(1 << 20) // slider at the bottom, clear of the marks
		buf := makeSurface(420, 720)
		c.Draw(newP(buf, 420), th)
		rc := c.minimapRect()
		rowH := c.minimap.rowH()
		tick := func(line int) RGBA {
			return pixelAt(buf, 420, c.Bounds().X+rc.X+rc.W-1, c.Bounds().Y+rc.Y+line*rowH)
		}
		for _, k := range []struct {
			line int
			kind MarkKind
		}{{10, MarkError}, {20, MarkWarning}, {30, MarkMatch}} {
			want := minimapMarkColor(th, k.kind)
			if got := tick(k.line); got != want {
				t.Errorf("%s: tick for line %d = %v, want %v", name, k.line, got, want)
			}
			if r := contrastRatio(want, th.SurfaceAlt); r < 3 {
				t.Errorf("%s: mark kind %d vs the minimap panel = %.2f:1, want >= 3:1", name, k.kind, r)
			}
		}
		// The slider's outline (the accent) is likewise >= 3:1 against the panel.
		if r := contrastRatio(minimapAccent(th), th.SurfaceAlt); r < 3 {
			t.Errorf("%s: slider outline vs panel = %.2f:1", name, r)
		}
	}
}

func TestEnsureContrast(t *testing.T) {
	white, black, grey := RGB(255, 255, 255), RGB(0, 0, 0), RGB(0x80, 0x80, 0x80)
	if got := ensureContrast(black, white, black, 3); got != black {
		t.Fatalf("a colour that already passes is kept, got %v", got)
	}
	light := RGB(0xd0, 0xd0, 0xff)
	if got := ensureContrast(light, white, black, 3); contrastRatio(got, white) < 3 {
		t.Fatalf("pulled colour %v still under 3:1", got)
	}
	if got := ensureContrast(grey, grey, grey, 3); got != grey {
		t.Fatalf("no mix can help: want the toward colour, got %v", got)
	}
}

func TestMinimapDrawsTokenSegmentsInEditorColours(t *testing.T) {
	c := newMinimapEditor(60)
	th := DefaultLight()
	c.Syntax = nil
	buf := makeSurface(420, 260)
	c.Draw(newP(buf, 420), th)
	if c.minimap.segmentCount(th.OnSurface) == 0 {
		t.Fatal("the minimap drew no segments")
	}
}

func BenchmarkEditorMinimapDraw(b *testing.B) {
	for _, mm := range []bool{false, true} {
		name := "minimap-off"
		if mm {
			name = "minimap-on"
		}
		b.Run(name, func(b *testing.B) {
			c := newScrollEditor(5000, 100, 200)
			c.SetBounds(Rect{W: 600, H: 600})
			c.ShowMinimap = mm
			th := DefaultDark()
			buf := makeSurface(600, 600)
			p := newP(buf, 600)
			c.Draw(p, th)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c.ScrollLine().Set(i % 4000)
				c.Draw(p, th)
			}
		})
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// winBounds is the placement reused across the Window tests: a 200x150 panel at
// (10,20). Derived tool / grip / body coordinates in the tests below assume it.
var winBounds = Rect{X: 10, Y: 20, W: 200, H: 150}

func TestWindowDrawWithBodyAndAllTools(t *testing.T) {
	th := DefaultLight()
	body := &recordingWidget{}
	w := NewWindow("Hello", body)
	w.Closable, w.Minimizable, w.Maximizable, w.Resizable = true, true, true, true
	w.SetBounds(winBounds)

	buf := makeSurface(220, 180)
	w.Draw(newP(buf, 220), th)

	// Title-bar band + body area are filled with Surface.
	if got := pixelAt(buf, 220, 15, 25); got != th.Surface {
		t.Fatalf("title-bar fill = %+v, want Surface %+v", got, th.Surface)
	}
	if got := pixelAt(buf, 220, 50, 100); got != th.Surface {
		t.Fatalf("body fill = %+v, want Surface %+v", got, th.Surface)
	}
	// Resize grip is filled with SurfaceAlt (grip square top-left at 198,158).
	if got := pixelAt(buf, 220, 204, 164); got != th.SurfaceAlt {
		t.Fatalf("grip fill = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// Close glyph paints ink on the main diagonal (glyph box top-left 195,28).
	if got := pixelAt(buf, 220, 195, 28); got != th.OnSurface {
		t.Fatalf("close glyph pixel = %+v, want OnSurface %+v", got, th.OnSurface)
	}
	if body.draws != 1 {
		t.Fatalf("Body.Draw calls = %d, want 1", body.draws)
	}
}

func TestWindowDrawWithoutBodyNoToolsNotResizable(t *testing.T) {
	th := DefaultLight()
	w := NewWindow("", nil) // no body, no tools, not resizable
	w.SetBounds(winBounds)

	buf := makeSurface(220, 180)
	w.Draw(newP(buf, 220), th)

	// Body area still fills even with no Body widget.
	if got := pixelAt(buf, 220, 50, 100); got != th.Surface {
		t.Fatalf("body fill (no body) = %+v, want Surface %+v", got, th.Surface)
	}
	// Grip must NOT be painted when not resizable: the grip corner keeps body
	// Surface (it is inside the body area), never SurfaceAlt.
	if got := pixelAt(buf, 220, 204, 164); got == th.SurfaceAlt {
		t.Fatalf("grip painted while not resizable: %+v", got)
	}
}

func TestWindowHitRegionAllRegions(t *testing.T) {
	w := NewWindow("t", &recordingWidget{})
	w.Closable, w.Minimizable, w.Maximizable, w.Resizable = true, true, true, true
	w.SetBounds(winBounds)

	cases := []struct {
		name   string
		px, py int
		want   WindowRegion
	}{
		{"close", 199, 32, WindowClose},
		{"minimize", 177, 32, WindowMinimize},
		{"maximize", 155, 32, WindowMaximize},
		{"titlebar", 20, 25, WindowTitleBar},
		{"resize", 200, 160, WindowResize},
		{"body", 50, 100, WindowBody},
		{"none", 5, 5, WindowNone},
	}
	for _, c := range cases {
		if got := w.HitRegion(c.px, c.py); got != c.want {
			t.Fatalf("%s: HitRegion(%d,%d) = %v, want %v", c.name, c.px, c.py, got, c.want)
		}
	}
}

func TestWindowHitRegionGripFallsThroughWhenNotResizable(t *testing.T) {
	w := NewWindow("t", nil)
	w.Resizable = false
	w.SetBounds(winBounds)
	// The grip corner point resolves to the body when Resizable is off.
	if got := w.HitRegion(200, 160); got != WindowBody {
		t.Fatalf("grip point (not resizable) = %v, want WindowBody", got)
	}
}

func TestWindowMoveBy(t *testing.T) {
	body := &recordingWidget{}
	w := NewWindow("t", body)
	w.SetBounds(winBounds)
	w.MoveBy(5, 7)

	if got := w.Bounds(); got != (Rect{X: 15, Y: 27, W: 200, H: 150}) {
		t.Fatalf("bounds after MoveBy = %+v", got)
	}
	// Body relaid out below the shifted title bar.
	if got := body.Bounds(); got != (Rect{X: 15, Y: 51, W: 200, H: 126}) {
		t.Fatalf("body bounds after MoveBy = %+v", got)
	}
}

func TestWindowResizeToClampedAndExact(t *testing.T) {
	body := &recordingWidget{}
	w := NewWindow("t", body)
	w.SetBounds(winBounds)

	// Below the minimum on both axes -> clamped to windowMinW / windowMinH.
	w.ResizeTo(10, 10)
	if got := w.Bounds(); got.W != windowMinW || got.H != windowMinH {
		t.Fatalf("clamped size = %dx%d, want %dx%d", got.W, got.H, windowMinW, windowMinH)
	}

	// Above the minimum on both axes -> honoured exactly.
	w.ResizeTo(300, 200)
	if got := w.Bounds(); got.W != 300 || got.H != 200 {
		t.Fatalf("exact size = %dx%d, want 300x200", got.W, got.H)
	}
	// Body follows the new size.
	if got := body.Bounds(); got != (Rect{X: 10, Y: 44, W: 300, H: 176}) {
		t.Fatalf("body bounds after ResizeTo = %+v", got)
	}
}

// clickAt returns a Window-local click event whose surface position is (px,py).
func clickAt(w *Window, px, py int) Event {
	r := w.Bounds()
	return Event{Kind: EventClick, X: px - r.X, Y: py - r.Y}
}

func TestWindowOnEventIgnoresNonClick(t *testing.T) {
	fired := false
	w := NewWindow("t", nil)
	w.Closable, w.OnClose = true, func() { fired = true }
	w.SetBounds(winBounds)
	w.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fired {
		t.Fatal("non-click event fired a tool callback")
	}
}

func TestWindowOnEventToolCallbacks(t *testing.T) {
	th := struct{ close, min, max bool }{}
	w := NewWindow("t", nil)
	w.Closable, w.Minimizable, w.Maximizable = true, true, true
	w.OnClose = func() { th.close = true }
	w.OnMinimize = func() { th.min = true }
	w.OnMaximize = func() { th.max = true }
	w.SetBounds(winBounds)

	w.OnEvent(clickAt(w, 199, 32)) // close
	w.OnEvent(clickAt(w, 177, 32)) // minimize
	w.OnEvent(clickAt(w, 155, 32)) // maximize

	if !th.close || !th.min || !th.max {
		t.Fatalf("tool callbacks fired = %+v, want all true", th)
	}
}

func TestWindowOnEventNilCallbacksAreSafe(t *testing.T) {
	w := NewWindow("t", nil)
	w.Closable, w.Minimizable, w.Maximizable = true, true, true
	w.SetBounds(winBounds)
	// All callbacks nil: clicking each tool must not panic.
	w.OnEvent(clickAt(w, 199, 32)) // close
	w.OnEvent(clickAt(w, 177, 32)) // minimize
	w.OnEvent(clickAt(w, 155, 32)) // maximize
}

func TestWindowOnEventForwardsBodyClick(t *testing.T) {
	body := &recordingWidget{}
	w := NewWindow("t", body)
	w.SetBounds(winBounds)

	w.OnEvent(clickAt(w, 50, 100)) // surface (50,100) is inside the body area

	if len(body.events) != 1 {
		t.Fatalf("body received %d events, want 1", len(body.events))
	}
	// Body bounds are (10,44); the event is translated into body-local coords.
	if got := body.events[0]; got.X != 40 || got.Y != 56 {
		t.Fatalf("forwarded body event = (%d,%d), want (40,56)", got.X, got.Y)
	}
}

func TestWindowOnEventBodyClickNilBodyIsSafe(t *testing.T) {
	w := NewWindow("t", nil)
	w.SetBounds(winBounds)
	// A body-region click with no Body must not panic.
	w.OnEvent(clickAt(w, 50, 100))
}

func TestWindowOnEventTitleBarClickIsInert(t *testing.T) {
	fired := false
	w := NewWindow("t", nil)
	w.Closable, w.OnClose = true, func() { fired = true }
	w.SetBounds(winBounds)
	w.OnEvent(clickAt(w, 20, 25)) // title bar, not a tool
	if fired {
		t.Fatal("title-bar click fired a tool callback")
	}
}

func TestNewWindowDefaults(t *testing.T) {
	body := &recordingWidget{}
	w := NewWindow("Title", body)
	if w.Title != "Title" || w.Body != body {
		t.Fatalf("NewWindow fields = (%q, %v)", w.Title, w.Body)
	}
	if w.minW != windowMinW || w.minH != windowMinH {
		t.Fatalf("NewWindow min size = %dx%d, want %dx%d", w.minW, w.minH, windowMinW, windowMinH)
	}
}

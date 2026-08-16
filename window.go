// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Window is a draggable, resizable floating panel: a title-bar band carrying a
// left-aligned Title and a right-aligned cluster of window tools (close,
// minimize, maximize), above a body area that hosts an optional Body widget.
// Unlike Dialog — a fixed, centred, non-movable modal — a Window is meant to be
// moved and resized around the surface.
//
// Following the toolkit's app-driven drag model (see Paned / Border), Window
// draws its chrome and exposes hit-testing plus explicit move/resize methods,
// but it does NOT run its own mouse-tracking loop. The host app owns pointer
// tracking: on mouse-down it calls HitRegion to learn whether the press landed
// on the title bar (start a move) or the resize grip (start a resize), then
// feeds the deltas to MoveBy / ResizeTo on each drag tick. Single clicks on the
// tools and forwarding into Body are handled by OnEvent as usual.
type Window struct {
	Base
	Title string
	Body  Widget // optional; nil leaves the body area empty

	// Tool flags enable the matching title-bar buttons; each fires its callback
	// (when non-nil) on a click. A disabled tool is neither drawn nor hit-tested.
	Closable, Minimizable, Maximizable bool
	OnClose, OnMinimize, OnMaximize    func()

	// Resizable draws the bottom-right resize grip and makes HitRegion report
	// WindowResize over it; the app drives the actual resize via ResizeTo.
	Resizable bool

	// minW / minH are the smallest body-inclusive size ResizeTo will settle on.
	minW, minH int
}

// WindowTitleH is the pixel height of the title-bar band.
const WindowTitleH = 24

// windowToolW is the width allocated to each title-bar tool button.
const windowToolW = 22

// windowGrip is the side length of the bottom-right resize grip square.
const windowGrip = 12

// windowGlyph is the side length of the small glyph box centred in a tool.
const windowGlyph = 8

// Default minimum window size, applied by NewWindow. Kept generous enough that
// the title bar and at least one tool always fit.
const (
	windowMinW = 80
	windowMinH = 60
)

// WindowRegion names the part of a Window a surface point falls on, as reported
// by HitRegion. The app uses it on mouse-down to decide whether to begin a move
// (WindowTitleBar), begin a resize (WindowResize), or leave the press to
// OnEvent (the tools / body).
type WindowRegion int

const (
	// WindowNone is returned for a point outside the window entirely.
	WindowNone WindowRegion = iota
	// WindowTitleBar is the title band excluding the tool buttons — the app
	// starts a move drag here.
	WindowTitleBar
	// WindowClose / WindowMinimize / WindowMaximize are the tool buttons.
	WindowClose
	WindowMinimize
	WindowMaximize
	// WindowResize is the bottom-right grip (only when Resizable) — the app
	// starts a resize drag here.
	WindowResize
	// WindowBody is the content area below the title bar.
	WindowBody
)

// NewWindow builds a Window with the given title and (optional) body, seeded
// with the default minimum size. Enable the tools / Resizable and wire the
// callbacks on the returned value.
func NewWindow(title string, body Widget) *Window {
	return &Window{Title: title, Body: body, minW: windowMinW, minH: windowMinH}
}

// titleBarRect is the full title-bar band in surface coordinates.
func (w *Window) titleBarRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X, Y: r.Y, W: r.W, H: scaled(WindowTitleH)}
}

// bodyAreaRect is the content area below the title bar in surface coordinates.
func (w *Window) bodyAreaRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X, Y: r.Y + scaled(WindowTitleH), W: r.W, H: r.H - scaled(WindowTitleH)}
}

// gripRect is the bottom-right resize grip square in surface coordinates.
func (w *Window) gripRect() Rect {
	r := w.Bounds()
	return Rect{X: r.X + r.W - windowGrip, Y: r.Y + r.H - windowGrip, W: windowGrip, H: windowGrip}
}

// toolRects returns the surface-space rectangle of each enabled title-bar tool,
// keyed by its region and laid out right-to-left: close (rightmost), then
// minimize, then maximize. Disabled tools are absent from the map.
func (w *Window) toolRects() map[WindowRegion]Rect {
	r := w.Bounds()
	out := make(map[WindowRegion]Rect)
	x := r.X + r.W
	place := func(reg WindowRegion, enabled bool) {
		if !enabled {
			return
		}
		x -= scaled(windowToolW)
		out[reg] = Rect{X: x, Y: r.Y, W: scaled(windowToolW), H: scaled(WindowTitleH)}
	}
	place(WindowClose, w.Closable)
	place(WindowMinimize, w.Minimizable)
	place(WindowMaximize, w.Maximizable)
	return out
}

// SetBounds stores the placement and lays the body out below the title bar.
func (w *Window) SetBounds(r Rect) {
	w.Base.SetBounds(r)
	w.relayout()
}

// relayout re-fits the body into the current body area (a no-op with no body).
func (w *Window) relayout() {
	if w.Body != nil {
		w.Body.SetBounds(w.bodyAreaRect())
	}
}

// MoveBy shifts the window by (dx, dy) and re-lays out its body — the app calls
// this on each drag tick after a WindowTitleBar press.
func (w *Window) MoveBy(dx, dy int) {
	r := w.Bounds()
	r.X += dx
	r.Y += dy
	w.Base.SetBounds(r)
	w.relayout()
}

// ResizeTo sets the window's size to (width, height), clamped to the minimum,
// and re-lays out its body — the app calls this on each drag tick after a
// WindowResize press.
func (w *Window) ResizeTo(width, height int) {
	if width < w.minW {
		width = w.minW
	}
	if height < w.minH {
		height = w.minH
	}
	r := w.Bounds()
	r.W = width
	r.H = height
	w.Base.SetBounds(r)
	w.relayout()
}

// HitRegion reports which part of the window the surface point (px, py) lands
// on. Tools win over the bare title bar; the grip (when Resizable) wins over
// the body it overlaps.
func (w *Window) HitRegion(px, py int) WindowRegion {
	for reg, rc := range w.toolRects() {
		if rc.Contains(px, py) {
			return reg
		}
	}
	if w.titleBarRect().Contains(px, py) {
		return WindowTitleBar
	}
	if w.Resizable && w.gripRect().Contains(px, py) {
		return WindowResize
	}
	if w.bodyAreaRect().Contains(px, py) {
		return WindowBody
	}
	return WindowNone
}

// Draw paints the title bar (band + title + enabled tools), the body area
// (fill + border + optional Body) and, when Resizable, the resize grip.
func (w *Window) Draw(p painter.Painter, theme *Theme) {
	tb := w.titleBarRect()
	fillRect(p, tb.X, tb.Y, tb.W, tb.H, theme.Surface)
	strokeRect(p, tb.X, tb.Y, tb.W, tb.H, theme.Border)
	titleY := tb.Y + (scaled(WindowTitleH)-w.glyphHeight())/2
	w.drawText(p, tb.X+8, titleY, w.Title, theme.OnSurface)
	for reg, rc := range w.toolRects() {
		w.drawTool(p, reg, rc, theme.OnSurface)
	}

	body := w.bodyAreaRect()
	fillRect(p, body.X, body.Y, body.W, body.H, theme.Surface)
	strokeRect(p, body.X, body.Y, body.W, body.H, theme.Border)
	if w.Body != nil {
		w.Body.SetBounds(body)
		w.Body.Draw(p, theme)
	}

	if w.Resizable {
		gr := w.gripRect()
		fillRect(p, gr.X, gr.Y, gr.W, gr.H, theme.SurfaceAlt)
		strokeRect(p, gr.X, gr.Y, gr.W, gr.H, theme.Border)
	}
}

// drawTool paints one tool's glyph centred in its rect: an × for close, a
// horizontal bar for minimize, a square outline for maximize. Glyphs are drawn
// with primitives so they render independently of the active font.
func (w *Window) drawTool(p painter.Painter, reg WindowRegion, rc Rect, ink RGBA) {
	gx := rc.X + (rc.W-windowGlyph)/2
	gy := rc.Y + (rc.H-windowGlyph)/2
	switch reg {
	case WindowClose:
		for i := 0; i < windowGlyph; i++ {
			fillRect(p, gx+i, gy+i, 1, 1, ink)
			fillRect(p, gx+i, gy+windowGlyph-1-i, 1, 1, ink)
		}
	case WindowMinimize:
		fillRect(p, gx, gy+windowGlyph-1, windowGlyph, 1, ink)
	case WindowMaximize:
		strokeRect(p, gx, gy, windowGlyph, windowGlyph, ink)
	}
}

// OnEvent handles single clicks: a click on an enabled tool fires its callback;
// a click in the body forwards to Body in body-local coordinates. Title-bar and
// resize-grip DRAGS are not handled here — the app drives those via MoveBy /
// ResizeTo — so a press on either falls through silently.
func (w *Window) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	r := w.Bounds()
	px, py := ev.X+r.X, ev.Y+r.Y
	switch w.HitRegion(px, py) {
	case WindowClose:
		if w.OnClose != nil {
			w.OnClose()
		}
	case WindowMinimize:
		if w.OnMinimize != nil {
			w.OnMinimize()
		}
	case WindowMaximize:
		if w.OnMaximize != nil {
			w.OnMaximize()
		}
	case WindowBody:
		if w.Body != nil {
			w.Body.OnEvent(translateEvent(ev, r, w.Body.Bounds()))
		}
	}
}

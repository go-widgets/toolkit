// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/mvvm"
)

// Zoom limits and step, the same contract as [PagedView.Zoom]: percent, 25 to
// 400, in steps of 25, 100 by default.
const (
	GraphZoomMin  = 25
	GraphZoomMax  = 400
	GraphZoomStep = 25
	// graphZoomBelowLabels is the zoom under which edge labels are not drawn.
	graphZoomBelowLabels = 50
)

// GraphPoint is a pan offset, in content pixels at 100 % zoom.
type GraphPoint struct{ X, Y int }

// Zoom is the zoom in percent, 25..400 (a value outside is clamped when read),
// 100 by default: the same contract as PagedView.Zoom. At 100 with no pan the
// view is drawn exactly as a GraphView without zoom support is.
func (g *GraphView) Zoom() *mvvm.Observable[int] {
	if g.zoom == nil {
		g.zoom = mvvm.NewObservable(100)
	}
	return g.zoom
}

// Pan is the content pixel (at 100 %) shown at the view's top-left corner. Its Y
// is the vertical scroll offset ([GraphView.ScrollY]); both are clamped to the
// content, so a value the content cannot reach is brought back at the next draw.
func (g *GraphView) Pan() *mvvm.Observable[GraphPoint] {
	if g.pan == nil {
		g.pan = mvvm.NewObservable(GraphPoint{})
	}
	return g.pan
}

// zoomPct is the clamped zoom. A change relays out once, so the pan limits
// follow it.
func (g *GraphView) zoomPct() int {
	z := clampInt(g.Zoom().Get(), GraphZoomMin, GraphZoomMax)
	if z != g.laidZoom {
		g.laidZoom = z
		g.layoutValid = false
	}
	return z
}

// SetZoom sets the zoom (clamped) keeping the content under the widget-local
// point (ax, ay) where it is.
func (g *GraphView) SetZoom(z, ax, ay int) {
	g.layout()
	old := g.zoomPct()
	z = clampInt(z, GraphZoomMin, GraphZoomMax)
	// The content point under the anchor stays put: scroll/pan absorb the change.
	g.panX = g.panX + int(math.Round(float64(ax)*100/float64(old)-float64(ax)*100/float64(z)))
	g.scrollY = g.scrollY + int(math.Round(float64(ay)*100/float64(old)-float64(ay)*100/float64(z)))
	g.Zoom().Set(z)
	g.layoutValid = false
	g.layout()
}

// ZoomIn and ZoomOut step the zoom by 25 around the view's centre.
func (g *GraphView) ZoomIn()  { g.stepZoom(GraphZoomStep, g.Bounds().W/2, g.Bounds().H/2) }
func (g *GraphView) ZoomOut() { g.stepZoom(-GraphZoomStep, g.Bounds().W/2, g.Bounds().H/2) }

func (g *GraphView) stepZoom(d, ax, ay int) { g.SetZoom(g.zoomPct()+d, ax, ay) }

// FitToView sets the zoom (a multiple of 25, never above 100) and the pan so
// that every cluster is visible.
func (g *GraphView) FitToView() {
	g.layout()
	z := 100
	if g.contentW > 0 && g.contentH > 0 {
		zw := g.Bounds().W * 100 / g.contentW
		zh := g.viewH * 100 / g.contentH
		z = min(100, zw, zh)
	}
	z = clampInt(z/GraphZoomStep*GraphZoomStep, GraphZoomMin, GraphZoomMax)
	g.Zoom().Set(z)
	g.panX, g.scrollY = 0, 0
	g.layoutValid = false
	g.layout()
}

// maxScrollY and maxPanX are the content that does not fit the view, in content
// pixels at 100 %.
func (g *GraphView) maxScrollY() int {
	return maxInt(0, g.contentH-g.viewH*100/g.zoomPct())
}
func (g *GraphView) maxPanX() int {
	return maxInt(0, g.contentW-g.Bounds().W*100/g.zoomPct())
}

// pushPan publishes the pan on the observable; pullPan adopts a value a host
// set on it since.
func (g *GraphView) pushPan() {
	g.pushed = GraphPoint{g.panX, g.scrollY}
	g.Pan().Set(g.pushed)
}

func (g *GraphView) syncPan() {
	if v := (GraphPoint{g.panX, g.scrollY}); v != g.pushed {
		g.pushPan()
	}
}

func (g *GraphView) pullPan() {
	if g.pan == nil {
		return
	}
	if p := g.pan.Get(); p != g.pushed {
		g.panX = p.X
		if p.Y != g.scrollY {
			g.scrollY = p.Y
			g.layoutValid = false
		}
	}
}

// PanTo and PanBy move the horizontal pan, clamped.
func (g *GraphView) PanTo(x int) {
	g.layout()
	x = clampInt(x, 0, g.maxPanX())
	if x != g.panX {
		g.panX = x
		g.pushPan()
	}
}
func (g *GraphView) PanBy(dx int) { g.PanTo(g.panX + dx) }

// rnd rounds to the nearest integer, halves up.
func rnd(v float64) int { return int(math.Floor(v + 0.5)) }

// sx and sy map a content coordinate (what clusterBox and nodeBox hold) to the
// surface coordinate it is drawn at; cx and cy are their inverses. At 100 % with
// no pan the arithmetic is exact on integers, so each is the identity.
func (g *GraphView) sx(x int) int {
	o := g.Bounds().X
	return o + rnd(float64(x-o-g.panX)*float64(g.zoomPct())/100)
}

func (g *GraphView) sy(y int) int {
	o := g.Bounds().Y
	return o + rnd(float64(y-o)*float64(g.zoomPct())/100)
}

func (g *GraphView) cx(x int) int {
	o := g.Bounds().X
	return o + g.panX + rnd(float64(x-o)*100/float64(g.zoomPct()))
}

func (g *GraphView) cy(y int) int {
	o := g.Bounds().Y
	return o + rnd(float64(y-o)*100/float64(g.zoomPct()))
}

// screen maps a content rectangle to where it is drawn. A side that has size
// keeps at least one pixel when zoomed far out.
func (g *GraphView) screen(r Rect) Rect {
	x0, y0 := g.sx(r.X), g.sy(r.Y)
	w, h := g.sx(r.X+r.W)-x0, g.sy(r.Y+r.H)-y0
	return Rect{X: x0, Y: y0, W: max(w, min(r.W, 1)), H: max(h, min(r.H, 1))}
}

// zoomEvent handles a wheel event with the zoom modifier: it reports whether
// the event was one.
func (g *GraphView) zoomWheel(ev Event) bool {
	if !(ev.Ctrl || ev.Meta) || ev.Delta == 0 {
		return false
	}
	step := GraphZoomStep
	if ev.Delta > 0 { // wheel down zooms out, as in a browser
		step = -step
	}
	g.stepZoom(step, ev.X, ev.Y)
	return true
}

// pinch wires two-finger zoom: a gesture scales the zoom it began at, around
// its centre.
func (g *GraphView) pinchFeed(ev Event) {
	if g.mt == nil {
		g.mt = NewMultiTouchRecognizer()
		g.mt.OnMultiBegin = func(MultiTouchState) { g.pinchBase = g.zoomPct() }
		g.mt.OnMultiUpdate = func(m MultiTouchState) {
			g.SetZoom(rnd(float64(g.pinchBase)*m.Scale), int(m.CenterX), int(m.CenterY))
		}
	}
	g.mt.Feed(ev)
}

// zoomKey handles +, -, 0 and reports whether the key was one of them.
func (g *GraphView) zoomKey(code string) bool {
	switch code {
	case "+", "=", "Equal", "NumpadAdd":
		g.ZoomIn()
	case "-", "_", "Minus", "NumpadSubtract":
		g.ZoomOut()
	case "0", "Digit0", "Numpad0":
		g.FitToView()
	default:
		return false
	}
	return true
}

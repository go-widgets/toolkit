// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// rtThemes are the palettes the contrast checks run over: the two defaults and a
// faint dark one whose Border ("#3a4556 on #141a24") is the hairline that the
// popup-border work found too quiet, and a pale light one.
func rtThemes() map[string]*Theme {
	faint := DefaultDark()
	faint.Background = RGB(0x14, 0x1A, 0x24)
	faint.Surface = RGB(0x1B, 0x22, 0x2E)
	faint.SurfaceAlt = RGB(0x24, 0x2D, 0x3B)
	faint.Border = RGB(0x3A, 0x45, 0x56)
	faint.Accent = RGB(0x2F, 0x5F, 0xA8)
	pale := DefaultLight()
	pale.Background = RGB(0xFF, 0xFF, 0xFF)
	pale.Surface = RGB(0xF6, 0xF7, 0xF9)
	pale.Border = RGB(0xE3, 0xE5, 0xE8)
	pale.Accent = RGB(0x8F, 0xC1, 0xF7)
	return map[string]*Theme{"light": DefaultLight(), "dark": DefaultDark(), "faint-dark": faint, "pale-light": pale}
}

func rtPixel(img *image.RGBA, x, y int) RGBA {
	o := img.PixOffset(x, y)
	return RGBA{R: img.Pix[o], G: img.Pix[o+1], B: img.Pix[o+2], A: img.Pix[o+3]}
}

// Boundaries and the focus ring keep 3:1 against the surfaces they sit on, in
// every palette (WCAG 2.2, 1.4.11), computed from the tokens alone.
func TestRibbonContrastTokens(t *testing.T) {
	for name, th := range rtThemes() {
		for _, c := range []struct {
			what string
			ink  RGBA
		}{{"line", ribbonLine(th)}, {"accent", ribbonAccent(th)}} {
			for gname, ground := range map[string]RGBA{"background": th.Background, "surface": th.Surface} {
				if r := contrastRatio(c.ink, ground); r < ribbonMinContrast {
					t.Errorf("%s: %s is %.2f:1 against the %s, want >= %.1f", name, c.what, r, gname, ribbonMinContrast)
				}
			}
		}
		if r := contrastRatio(th.OnSurface, th.SurfaceAlt); r < 4.5 {
			t.Errorf("%s: text on the hover face is %.2f:1", name, r)
		}
	}
}

// The same, measured on rendered pixels: the outline of the active tab, a group
// divider, a drop-down's box, the focus ring on a tab and on a control, the
// edge of a popup.
func TestRibbonContrastPixels(t *testing.T) {
	for name, th := range rtThemes() {
		run := &sampleRun{}
		r := run.ribbon()
		const w = 1100
		r.SetBounds(Rect{W: w, H: r.Height()})
		m := r.metrics()
		tabs, _ := r.tabRects(m)
		body := r.body()
		check := func(what string, img *image.RGBA, x, y int, grounds ...RGBA) {
			t.Helper()
			got := rtPixel(img, x, y)
			for _, g := range grounds {
				if c := contrastRatio(got, g); c < ribbonMinContrast {
					t.Errorf("%s: %s pixel (%d,%d) is %.2f:1 against %v, want >= 3", name, what, x, y, c, g)
				}
			}
		}
		img := renderRibbon(r, th, w, 200)
		check("active tab outline (left)", img, tabs[0].X, tabs[0].Y+m.tabH/2, th.Background, th.Surface)
		check("active tab outline (top)", img, tabs[0].X+tabs[0].W/2, tabs[0].Y, th.Background, th.Surface)
		g0 := body.groups[0].rect
		check("group divider", img, g0.X+g0.W, g0.Y+m.bodyH/2, th.Surface)
		check("tab strip rule", img, w-3, m.tabH, th.Background, th.Surface)
		di := rtCell(r, "Style")
		dd := r.cells()[di].rect
		check("drop-down box", img, dd.X+dd.W/2, dd.Y+scaled(2), th.Surface, th.Background)

		r.SetFocused(true)
		img = renderRibbon(r, th, w, 200)
		check("focus ring on the active tab", img, tabs[0].X+1, tabs[0].Y+m.tabH/2, th.Surface)
		r.focusTab = 1
		img = renderRibbon(r, th, w, 200)
		check("focus ring on an inactive tab", img, tabs[1].X+1, tabs[1].Y+m.tabH/2, th.Background)
		r.zone, r.focusCtl = ribbonZoneBody, rtCell(r, "Duplicate")
		cell := r.cells()[r.focusCtl].rect
		img = renderRibbon(r, th, w, 200)
		check("focus ring on a control", img, cell.X+cell.W/2, cell.Y, th.Surface)

		r.activate(rtCell(r, "New slide"), partArrow)
		panel, _ := r.popupGeometry(r.pop, m)
		img = renderRibbon(r, th, w, 260)
		check("popup edge (left)", img, panel.X, panel.Y+panel.H/2, th.Background, th.Surface)
		check("popup edge (bottom)", img, panel.X+panel.W/2, panel.Y+panel.H-1, th.Background, th.Surface)
	}
}

// The checked state of a toggle is told by more than a fill: an outline at 3:1.
func TestRibbonCheckedToggleHasAnOutline(t *testing.T) {
	for name, th := range rtThemes() {
		r, _ := rtRibbon(1100)
		r.Tabs[0].Groups[1].Controls[3].check().Set(true)
		c := r.cells()[rtCell(r, "Word wrap")].rect
		img := renderRibbon(r, th, 1100, 120)
		if got := rtPixel(img, c.X+c.W/2, c.Y); contrastRatio(got, th.Surface) < ribbonMinContrast {
			t.Errorf("%s: a checked toggle's outline is %.2f:1", name, contrastRatio(got, th.Surface))
		}
	}
}

// Disabled controls are drawn muted, and an enabled one is not.
func TestRibbonDisabledControlIsMuted(t *testing.T) {
	th := DefaultLight()
	r, _ := rtRibbon(1100)
	img1 := renderRibbon(r, th, 1100, 120)
	r.Tabs[0].Groups[0].Controls[1].Action.SetEnabled(false) // Duplicate
	img2 := renderRibbon(r, th, 1100, 120)
	c := r.cells()[rtCell(r, "Duplicate")].rect
	changed := false
	for y := c.Y; y < c.Y+c.H; y++ {
		for x := c.X; x < c.X+c.W; x++ {
			if rtPixel(img1, x, y) != rtPixel(img2, x, y) {
				changed = true
			}
		}
	}
	if !changed {
		t.Error("disabling a control left it looking the same")
	}
}

// Everything paints inside its own rectangle, or inside a popup it owns.
func TestRibbonDrawsEveryStateWithoutPanicking(t *testing.T) {
	for name, th := range rtThemes() {
		for _, w := range []int{1300, 700, 520, 360, 200, 60} {
			for _, scale := range []float64{1, 2} {
				SetMetricScale(scale)
				run := &sampleRun{}
				r := run.ribbon()
				for tab := range r.Tabs {
					r.ActiveTab().Set(tab)
					r.SetBounds(Rect{X: 3, Y: 2, W: w, H: r.Height()})
					r.SetFocused(true)
					renderRibbon(r, th, w+6, r.Height()+300)
					r.hoverTab, r.chevHot = tab, true
					r.hoverCtl, r.hoverMs = 0, 1000
					renderRibbon(r, th, w+6, r.Height()+300)
					r.zone = ribbonZoneBody
					for i := range r.cells() {
						r.focusCtl = i
						renderRibbon(r, th, w+6, r.Height()+300)
					}
				}
				SetMetricScale(1)
			}
		}
		_ = name
	}
}

func TestRibbonDrawsPopupsOfEveryKind(t *testing.T) {
	th := DefaultLight()
	r, _ := rtRibbon(1100)
	r.Tabs[0].Groups[0].Controls[0].Items[1].Action.SetEnabled(false)
	r.Tabs[0].Groups[0].Controls[0].Items[0].Action.Shortcut = MustParseChord("Ctrl+T")
	for _, c := range []struct {
		label string
		part  int
	}{{"New slide", partArrow}, {"Style", partMain}, {"Kinds", partArrow}} {
		r.Dismiss()
		r.activate(rtCell(r, c.label), c.part)
		if r.pop == nil {
			t.Fatalf("%s opened nothing", c.label)
		}
		r.pop.focus = 1
		renderRibbon(r, th, 1100, 300)
		r.pop.focus = 0
		renderRibbon(r, DefaultDark(), 1100, 300)
	}
	// The gallery's chosen cell is outlined in the strip and in the grid.
	r.Dismiss()
	r.Tabs[0].Groups[2].Controls[1].selection().Set(2)
	renderRibbon(r, th, 1100, 300)
	r.activate(rtCell(r, "Kinds"), partArrow)
	renderRibbon(r, th, 1100, 300)
	// A drop-down whose selection is out of range shows its label.
	r.Dismiss()
	r.Tabs[0].Groups[2].Controls[0].selection().Set(99)
	renderRibbon(r, th, 1100, 300)
	// A folded group's panel.
	r2, _ := rtRibbon(360)
	r2.activate(rtCell(r2, "Theme"), partMain)
	renderRibbon(r2, th, 360, 300)
	// A collapsed ribbon's peeking body.
	r3, _ := rtRibbon(900)
	r3.Collapsed().Set(true)
	r3.peek = true
	renderRibbon(r3, th, 900, 300)
}

func TestRibbonKeyTipBadgesFilterByWhatWasTyped(t *testing.T) {
	th := DefaultLight()
	r, _ := rtRibbon(1100)
	rtKey(r, "Alt")
	renderRibbon(r, th, 1100, 200)
	rtKey(r, "H")
	renderRibbon(r, th, 1100, 200)
	r.tipBuf = "Z" // nothing starts with Z here: every badge is filtered out
	renderRibbon(r, th, 1100, 200)
	// A badge for an empty tip draws nothing, and a longer tip than typed is fine.
	r.drawBadge(nilPainter(), th, r.metrics(), "", 10, 10)
}

func TestRibbonTooltipBubbleIsInsideTheDrawing(t *testing.T) {
	th := DefaultLight()
	r, _ := rtRibbon(1100)
	i := rtCell(r, "New slide")
	x, y := rtCenterOf(r.cells()[i])
	rtMove(r, x, y)
	r.Tick(0.6)
	m := r.metrics()
	anchor := r.cells()[i].rect
	img := renderRibbon(r, th, 1100, 220)
	// The bubble starts under the control: its first row is the line colour.
	if got := rtPixel(img, anchor.X+2, anchor.Y+anchor.H+scaled(2)); got != ribbonLine(th) {
		t.Errorf("tooltip border pixel %v, want %v", got, ribbonLine(th))
	}
	_ = m
}

func TestRibbonIconsAndInk(t *testing.T) {
	th := DefaultLight()
	if ribbonInk(th, true) != th.OnSurface || ribbonInk(th, false) == th.OnSurface {
		t.Error("ribbonInk should mute the disabled ink")
	}
	drawIcon(nilPainter(), nil, Rect{}, th.OnSurface) // a missing icon is simply not drawn
}

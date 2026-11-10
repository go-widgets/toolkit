// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ContrastLevel is the WCAG 2.x conformance level a text/background pair reaches.
type ContrastLevel int

// The levels, in increasing strength. WCAG 2.2 success criteria 1.4.3 (AA) and
// 1.4.6 (AAA): normal text needs 4.5:1 and 7:1; large text (18 pt, or 14 pt
// bold) needs 3:1 and 4.5:1.
const (
	ContrastFail ContrastLevel = iota
	ContrastAA
	ContrastAAA
)

// ContrastAAMin / ContrastAAAMin are the ratios normal text must reach;
// ContrastAALargeMin / ContrastAAALargeMin those large text must.
const (
	ContrastAAMin       = 4.5
	ContrastAAAMin      = 7.0
	ContrastAALargeMin  = 3.0
	ContrastAAALargeMin = 4.5
)

// String is "AAA", "AA" or "Fail" (a code, not prose: the badge's own wording of
// a failure is ContrastBadge.FailText).
func (l ContrastLevel) String() string {
	switch l {
	case ContrastAAA:
		return "AAA"
	case ContrastAA:
		return "AA"
	}
	return "Fail"
}

// Passes reports whether the level is at least AA.
func (l ContrastLevel) Passes() bool { return l >= ContrastAA }

// The ratio itself is [ContrastRatio] (splitter.go): symmetric, alpha ignored.

// ContrastGrade maps a ratio to the level it reaches for normal or large text.
func ContrastGrade(ratio float64, large bool) ContrastLevel {
	aa, aaa := ContrastAAMin, ContrastAAAMin
	if large {
		aa, aaa = ContrastAALargeMin, ContrastAAALargeMin
	}
	switch {
	case ratio >= aaa:
		return ContrastAAA
	case ratio >= aa:
		return ContrastAA
	}
	return ContrastFail
}

// ContrastBadge is a small pill that says how readable a foreground colour is on
// a background: "4.52:1 AA", "2.10:1 Fail". The verdict is in the text, never
// only in the pill's colour, so it reads without telling green from red.
//
// The two colours are Observables (Foreground, Background): a host binds them to
// the pickers that edit them and the badge recomputes on its next Draw. Large
// switches the thresholds to the large-text ones (3:1 / 4.5:1). The pill is a
// passive display, like [Badge].
type ContrastBadge struct {
	Base
	// FailText is the word shown for a failing pair ("Fail" when empty): set it
	// to the interface's language. The level names AA and AAA are the standard's
	// own and are not translated.
	FailText string

	fg, bg *mvvm.Observable[RGBA]
	large  *mvvm.Observable[bool]
}

// NewContrastBadge builds a badge for fg on bg.
func NewContrastBadge(fg, bg RGBA) *ContrastBadge {
	return &ContrastBadge{
		fg:    mvvm.NewObservable(fg),
		bg:    mvvm.NewObservable(bg),
		large: mvvm.NewObservable(false),
	}
}

// Foreground is the ink colour, a shared [mvvm.Observable].
func (c *ContrastBadge) Foreground() *mvvm.Observable[RGBA] {
	if c.fg == nil {
		c.fg = mvvm.NewObservable(RGBA{})
	}
	return c.fg
}

// Background is the ground colour, a shared [mvvm.Observable].
func (c *ContrastBadge) Background() *mvvm.Observable[RGBA] {
	if c.bg == nil {
		c.bg = mvvm.NewObservable(RGBA{})
	}
	return c.bg
}

// Large is whether the text is large (the lower thresholds), a shared
// [mvvm.Observable].
func (c *ContrastBadge) Large() *mvvm.Observable[bool] {
	if c.large == nil {
		c.large = mvvm.NewObservable(false)
	}
	return c.large
}

// Ratio is the current contrast ratio.
func (c *ContrastBadge) Ratio() float64 {
	return ContrastRatio(c.Foreground().Get(), c.Background().Get())
}

// Level is the level the current pair reaches.
func (c *ContrastBadge) Level() ContrastLevel {
	return ContrastGrade(c.Ratio(), c.Large().Get())
}

// Text is what the badge says.
func (c *ContrastBadge) Text() string {
	word := c.Level().String()
	if c.Level() == ContrastFail && c.FailText != "" {
		word = c.FailText
	}
	return fmt.Sprintf("%.2f:1 %s", c.Ratio(), word)
}

// The pill tones: fixed, like the error red of FormField, because pass and fail
// must keep their meaning under any theme.
var (
	contrastPassFill = RGBA{R: 0x1a, G: 0x7f, B: 0x37, A: 0xff}
	contrastFailFill = RGBA{R: 0xb4, G: 0x23, B: 0x18, A: 0xff}
)

// Draw paints the pill. A zero Bounds width auto-sizes to the text, as [Badge].
func (c *ContrastBadge) Draw(p painter.Painter, theme *Theme) {
	txt := c.Text()
	r := c.Bounds()
	if r.W == 0 {
		r.W = c.textWidth(txt) + 2*scaled(BadgePadX)
		if r.H == 0 {
			r.H = c.glyphHeight() + 2*scaled(BadgePadY)
		}
		c.SetBounds(r)
	}
	fill := contrastPassFill
	if !c.Level().Passes() {
		fill = contrastFailFill
	}
	radius := min(r.W, r.H)
	p.FillRoundRect(painter.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, radius/2, fill)
	c.drawText(p, r.X+(r.W-c.textWidth(txt))/2, r.Y+(r.H-c.glyphHeight())/2, txt, RGB(0xff, 0xff, 0xff))
}

// A11y names the pair's verdict for a screen reader.
func (c *ContrastBadge) A11y() A11yInfo { return A11yInfo{Role: RoleText, Name: c.Text()} }

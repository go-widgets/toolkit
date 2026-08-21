// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// CycleButton is a button that steps through a fixed set of Options, showing the
// active one and advancing to the next on each click (wrapping past the end). It
// is the compact alternative to a radio group or dropdown when the choice set is
// small and cycling is natural (e.g. a view mode: List → Grid → Compact).
//
// Options is set-once config. The shown option is identified by an index whose
// reactive state is MVVM-only: it lives in an unexported Observable exposed via
// [CycleButton.Index]. A host binds that handle (Set / Subscribe / two-way) and
// reads Options[Index] itself — there is no settable Index field and no change
// callback.
type CycleButton struct {
	Base
	focusState
	Options []string

	index *mvvm.Observable[int]
}

// Index is the currently-shown option's index as a shared [mvvm.Observable]: a
// host binds it (Set / Subscribe / two-way) — there is no settable Index field.
// A click or a key step Sets it (wrapping within Options); subscribers are
// notified. It lazy-inits to 0 so a bare &CycleButton{} yields a usable handle.
func (c *CycleButton) Index() *mvvm.Observable[int] {
	if c.index == nil {
		c.index = mvvm.NewObservable(0)
	}
	return c.index
}

// NewCycleButton builds a CycleButton over options (the first shown).
func NewCycleButton(options ...string) *CycleButton {
	c := &CycleButton{Options: options}
	c.index = mvvm.NewObservable(0)
	return c
}

// Value returns the currently shown option, or "" when there are none (or Index
// is out of range). It is the value-oriented read over the reactive Index
// handle.
func (c *CycleButton) Value() string {
	i := c.Index().Get()
	if i < 0 || i >= len(c.Options) {
		return ""
	}
	return c.Options[i]
}

// Draw paints the button body + the active option's label, centred, using the
// widget's font.
func (c *CycleButton) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	face, ink, border := theme.Surface, theme.OnSurface, theme.Border
	if c.Disabled().Get() {
		face, ink, border = mutedFace(theme), mutedInk(theme), mutedInk(theme)
	}
	// buttonRadius routes through scaled so the corner grows with HiDPI and touch
	// density; scaled(buttonRadius) == buttonRadius at compact/1x (byte-identical).
	rad := scaled(buttonRadius)
	fillRoundRect(p, r.X, r.Y, r.W, r.H, rad, face)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, rad, border)
	if v := c.Value(); v != "" {
		tx := r.X + (r.W-c.textWidth(v))/2
		ty := r.Y + (r.H-c.glyphHeight())/2
		c.drawText(p, tx, ty, v, ink)
	}
	c.drawFocusRing(p, theme, r)
}

// OnEvent advances to the next option on a click (wrapping), Setting the Index
// Observable. A Disabled cycle button ignores every kind.
func (c *CycleButton) OnEvent(ev Event) {
	if c.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		c.step(+1)
	case EventKeyDown:
		// Space / Enter / ArrowRight advance (matching a click); ArrowLeft steps
		// back. Both directions wrap and Set the Index Observable.
		switch ev.Code {
		case " ", "Space", "Enter", "ArrowRight":
			c.step(+1)
		case "ArrowLeft":
			c.step(-1)
		}
	}
}

// step advances the Index by delta places (wrapping) and Sets the Index
// Observable -- the shared mutate path for a click and a keyboard step.
// Subscribers are notified on change. A no-op when there are no options.
func (c *CycleButton) step(delta int) {
	n := len(c.Options)
	if n == 0 {
		return
	}
	c.Index().Set(((c.Index().Get()+delta)%n + n) % n)
}

// HitRect is the cycle button's interactive rectangle: its drawn Bounds clamped
// up to the density hit-target and centred over them (see [touchHitRect]).
// Byte-identical to Bounds under DensityCompact; a finger-sized target under
// DensityTouch.
func (c *CycleButton) HitRect() Rect { return touchHitRect(c.Bounds()) }

// HitTest reports whether a surface point falls on the cycle button's
// (touch-clamped) hit rect.
func (c *CycleButton) HitTest(px, py int) bool { return c.HitRect().Contains(px, py) }

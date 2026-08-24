// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"time"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// DefaultClockFormat is the reference-time layout a Clock uses when its Format is
// unset: a 24-hour "HH:MM" digital reading (e.g. "15:04").
const DefaultClockFormat = "15:04"

// Clock is a passive digital clock: it renders a single [time.Time] as a centred
// text reading, so a desktop shell's dock, panel or lock screen stops hand-
// composing one (measure, centre, DrawText) every frame.
//
// It deliberately does NOT read the wall clock. time.Now is untestable and is
// banned in some contexts (a deterministic replay, a wasm build), so the HOST
// drives the widget: the current instant lives on an [mvvm.Observable] exposed
// via [Clock.Time], and the app advances it with [Clock.SetTime] — in a browser
// that is one call per second off a JS Date tick. Nothing here calls time.Now.
//
// The reading is formatted by [Clock.Format] (a Go reference-time layout,
// defaulting to [DefaultClockFormat]) unless [Clock.Func] is set, in which case
// that function produces the string — the seam for a localised or otherwise
// custom rendering the reference layout cannot express. Both are set-once config.
//
// Draw is composed, not hand-drawn: a single centred [Label] carries the text, so
// the Clock reuses the toolkit's font, alignment and ellipsis machinery instead
// of rasterising glyphs itself. The Label is an internal rendering detail — the
// Clock is the accessible element and announces the reading itself (RoleText),
// exactly as [GroupCard] / [PostCard] own the labels they compose, so a screen
// reader hears the time once rather than from both the wrapper and its child.
type Clock struct {
	Base
	// Format is the Go reference-time layout the reading is rendered with
	// (e.g. "15:04", "3:04 PM", "Mon 15:04:05"). The zero value ""
	// uses [DefaultClockFormat]. Ignored when Func is set. Set-once config.
	Format string
	// Func, when non-nil, produces the reading from the current time instead of
	// Format — a seam for a localised or otherwise custom string the reference
	// layout cannot express. Set-once config (a func hook).
	Func func(time.Time) string
	// Align is the reading's horizontal alignment within the widget's bounds. The
	// zero value AlignLeft matches Label; a dock/panel clock typically sets
	// AlignCenter. Set-once config.
	Align Align

	// t is the current instant, reactive via the Time() accessor.
	t *mvvm.Observable[time.Time]
	// lbl is the internal Label that draws (and measures) the reading.
	lbl *Label
	// subscribed guards the one-time Time()->label wiring so Time() stays
	// idempotent.
	subscribed bool
}

// NewClock constructs a Clock showing t, centred, with the default 24-hour
// layout. Set Format (or Func) for a different rendering before the first Draw.
func NewClock(t time.Time) *Clock {
	c := &Clock{Align: AlignCenter}
	c.Time().Set(t)
	return c
}

// label returns the internal drawing Label, creating it on first use so a bare
// &Clock{} works without a constructor.
func (c *Clock) label() *Label {
	if c.lbl == nil {
		c.lbl = NewLabel("")
	}
	return c.lbl
}

// Time is the displayed instant as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) or advances it via [Clock.SetTime] — there is no
// settable time field. Created (and wired to refresh the reading) on first
// access, so a bare &Clock{} binds cleanly.
func (c *Clock) Time() *mvvm.Observable[time.Time] {
	if c.t == nil {
		c.t = mvvm.NewObservable(time.Time{})
	}
	if !c.subscribed {
		c.subscribed = true
		c.t.Subscribe(func(time.Time) { c.sync() })
	}
	return c.t
}

// SetTime advances the displayed instant to t (a host's per-tick call) and
// returns the Clock for fluent chaining. Equivalent to c.Time().Set(t).
func (c *Clock) SetTime(t time.Time) *Clock {
	c.Time().Set(t)
	return c
}

// layout is the reference-time layout in effect: Format, or DefaultClockFormat
// when Format is unset.
func (c *Clock) layout() string {
	if c.Format == "" {
		return DefaultClockFormat
	}
	return c.Format
}

// reading is the formatted time string: Func(t) when Func is set, otherwise
// t.Format(layout).
func (c *Clock) reading() string {
	t := c.Time().Get()
	if c.Func != nil {
		return c.Func(t)
	}
	return t.Format(c.layout())
}

// sync pushes the current reading (and alignment) onto the internal Label, so the
// text stays correct between frames whenever Time changes.
func (c *Clock) sync() {
	l := c.label()
	l.Align = c.Align
	l.Text().Set(c.reading())
}

// Draw paints the reading as a centred (per Align) Label filling the Clock's
// bounds.
func (c *Clock) Draw(p painter.Painter, theme *Theme) {
	c.sync()
	l := c.label()
	l.SetBounds(c.Bounds())
	l.Draw(p, theme)
}

// A11y reports the Clock as static text carrying the current reading. The reading
// is exposed both as the accessible Name (what a reader announces for RoleText)
// and as Value (the current, changing reading), so a bridge that surfaces either
// hears the time.
func (c *Clock) A11y() A11yInfo {
	s := c.reading()
	return A11yInfo{Role: RoleText, Name: s, Value: s}
}

// Clock is an accessible leaf.
var _ Accessible = (*Clock)(nil)

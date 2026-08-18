// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// StatTrend selects the semantic direction of a Stat's optional Change
// indicator. StatFlat renders the change text in the theme's dim
// border ink (the same "muted-ink" convention HeaderBar uses for its
// subtitle); StatUp and StatDown paint fixed green and red shades so
// the direction reads at a glance regardless of the app's accent
// choice. Widgets that want a neutral or accent-tinted change value
// use StatFlat and let the theme drive the tone.
type StatTrend int

const (
	// StatFlat is the neutral "no direction" trend. Change ink comes
	// from Theme.Border so the value blends with the surrounding
	// muted labels.
	StatFlat StatTrend = iota
	// StatUp signals a positive change ("+12%", "revenue up"). Ink is
	// a fixed sea-green so up-trends read the same across every theme.
	StatUp
	// StatDown signals a negative change ("-4%", "errors up"). Ink is
	// a fixed brick-red so down-trends read the same across every theme.
	StatDown
)

// Stat is a compact KPI card — a small dim Title on top, a large
// Value in the middle drawn with a one-pixel horizontal thickening
// pass to fake a bold weight, and an optional Change indicator at
// the bottom coloured by Trend. Modelled on DaisyUI's `<div
// class="stat">` block: three vertically stacked text rows on a
// bordered Surface panel.
//
// Stat is passive display only — the parent view supplies any
// interaction (a click-through link, a tooltip) as a separate widget
// on top. HitTest / OnEvent stay as Base defaults.
//
// The reactive metric is MVVM-only: the current Value, Change and Trend
// each live in an unexported [mvvm.Observable] exposed via the
// like-named accessor. A host updates the tile at runtime with
// s.Value().Set("…") / s.Change().Set("…") / s.Trend().Set(StatUp);
// subscribers are notified. Only the static Title stays a plain config
// field.
type Stat struct {
	Base
	// Title is the static caption drawn above the metric (config).
	Title string

	value  *mvvm.Observable[string]
	change *mvvm.Observable[string]
	trend  *mvvm.Observable[StatTrend]
}

// Value is the headline metric as a shared [mvvm.Observable]: a host binds it
// (Set / Subscribe / two-way) — there is no settable Value field. Draw and A11y
// read it via Get.
func (s *Stat) Value() *mvvm.Observable[string] {
	if s.value == nil {
		s.value = mvvm.NewObservable("")
	}
	return s.value
}

// Change is the optional bottom-row indicator ("+12%", "-4%") as a shared
// [mvvm.Observable]. An empty string paints no change row; a host mutates it via
// Set.
func (s *Stat) Change() *mvvm.Observable[string] {
	if s.change == nil {
		s.change = mvvm.NewObservable("")
	}
	return s.change
}

// Trend selects the Change row's semantic direction (StatFlat / StatUp /
// StatDown) as a shared [mvvm.Observable]. A host mutates it via Set; Draw reads
// it via Get to colour the change ink.
func (s *Stat) Trend() *mvvm.Observable[StatTrend] {
	if s.trend == nil {
		s.trend = mvvm.NewObservable(StatFlat)
	}
	return s.trend
}

// Stat sizing constants. Padding matches Alert (12, 8) so a Stat
// composes cleanly next to an Alert banner; the two gap constants
// keep the three text rows readable at 5x7 glyphs without the tall
// vertical footprint of a full Card.
const (
	// StatPadX is the horizontal inset between the border and the
	// left edge of the Title / Value / Change text.
	StatPadX = 12
	// StatPadY is the vertical inset between the top border and the
	// first row of Title text (and between the last row of Change
	// text and the bottom border).
	StatPadY = 8
	// StatTitleGap is the vertical space inserted between the Title
	// row's bottom and the Value row's top.
	StatTitleGap = 4
	// StatValueGap is the vertical space inserted between the Value
	// row's bottom and the Change row's top.
	StatValueGap = 4
)

// NewStat constructs a Stat with the given title + value. Change
// defaults to "" (no change row painted) and Trend defaults to
// StatFlat; a host enables the bottom row via s.Change().Set(…) and
// s.Trend().Set(…).
func NewStat(title, value string) *Stat {
	return &Stat{
		Title:  title,
		value:  mvvm.NewObservable(value),
		change: mvvm.NewObservable(""),
		trend:  mvvm.NewObservable(StatFlat),
	}
}

// statChangeInk maps a Trend to the ink used for the Change row.
// StatFlat defers to Theme.Border so it matches the app's dim-label
// palette; StatUp / StatDown carry fixed shades since the theme
// doesn't (and shouldn't) grow semantic-colour slots for every
// widget that wants one — same rationale as alertFace.
func statChangeInk(trend StatTrend, theme *Theme) RGBA {
	switch trend {
	case StatUp:
		return RGBA{R: 50, G: 150, B: 80, A: 255}
	case StatDown:
		return RGBA{R: 190, G: 60, B: 60, A: 255}
	default: // StatFlat (also any out-of-range Trend values)
		return dimInk(theme)
	}
}

// Draw paints the surface fill, the three text rows and finally the
// outer border stroke. Draw order matches Card (fill, decorations,
// border last) so the 1-px border always sits on top and clips
// overlapping ink.
//
// The Value row is drawn TWICE — once at (x, y) and again at (x+1,
// y) — to fake a bold weight. Since the 5x7 bitmap font ships one
// stroke width, the double-draw thickens each column by one pixel
// so the Value visually outweighs the surrounding Title + Change
// rows without a second glyph table.
func (s *Stat) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	titleX := r.X + StatPadX
	titleY := r.Y + StatPadY
	s.drawText(p, titleX, titleY, s.Title, dimInk(theme))

	value := s.Value().Get()
	valueY := titleY + s.glyphHeight() + StatTitleGap
	s.drawText(p, titleX, valueY, value, theme.OnSurface)
	s.drawText(p, titleX+1, valueY, value, theme.OnSurface)

	if change := s.Change().Get(); change != "" {
		changeY := valueY + s.glyphHeight() + StatValueGap
		s.drawText(p, titleX, changeY, change, statChangeInk(s.Trend().Get(), theme))
	}

	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// TimelineKind selects the semantic colour of a timeline event's
// marker square. TimelineDefault reuses the theme's Accent so a
// neutral event matches the app's palette; the other three carry
// fixed shades — green for success, amber for warning, red for
// error — reusing the exact RGB tuples Alert already ships, so an
// Alert banner and a Timeline row read as the same colour language.
type TimelineKind int

const (
	// TimelineDefault is a neutral event. Marker fill = Theme.Accent.
	TimelineDefault TimelineKind = iota
	// TimelineSuccess flags a completed step ("Deploy OK"). Green.
	TimelineSuccess
	// TimelineWarning flags a non-fatal event ("High latency"). Amber.
	TimelineWarning
	// TimelineError flags a failure ("Build failed"). Red.
	TimelineError
)

// TimelineEvent is one row in a Timeline's Events slice. Title is
// the always-visible headline; Detail is an optional second line
// rendered underneath in the dim Border ink (matching HeaderBar's
// subtitle convention). Kind drives the marker colour.
type TimelineEvent struct {
	Title  string
	Detail string
	Kind   TimelineKind
}

// Timeline is a vertical event log — think a GitHub PR activity
// stream or a Discord message list. The widget draws a 1-px
// vertical rail on the left, one filled square marker per event on
// that rail, and the event's Title (+ optional Detail) rendered to
// the right of the marker.
//
// A vertical Timeline scrolls: the mouse wheel (EventScroll) shifts the event
// window up/down, clamped at both ends, and the events are clipped to Bounds
// so a long log never bleeds past the widget's box. EventAt maps a point to
// the event under it through the same offset, so a caller who wants
// click-to-focus can hit-test the scrolled list without redoing the layout
// math. A horizontal Timeline stays a fixed left-to-right ribbon (no scroll).
type Timeline struct {
	Base
	Events []TimelineEvent
	// Horizontal runs the rail left-to-right (a process ribbon) instead of
	// top-to-bottom. The zero value (false) keeps the original vertical
	// activity-stream layout. A bool rather than the shared Orientation enum
	// because Timeline's natural default is vertical, whereas that enum's
	// zero value is Horizontal — a plain flag keeps the non-breaking default
	// unambiguous.
	Horizontal bool

	// scrollY is the vertical scroll offset in pixels for the vertical layout:
	// the event list is drawn shifted up by this many pixels and hit-tested
	// through it. The wheel (EventScroll) moves it and reads clamp on the fly
	// (clampedScrollY), so a value left stale after the list shrank is
	// harmless; at scrollY == 0 rendering + hit-testing are byte-identical to
	// before scrolling existed.
	scrollY int
}

// Timeline sizing constants. Marker column is 12 px wide, the
// marker itself 6 px so it sits centred on the rail with a 3-px
// gutter either side; event rows are one glyph plus a 4-px vertical
// spacer so successive titles don't touch, and Detail rows sit 2 px
// below their Title with a matching glyph height.
const (
	// TimelineMarkerW is the reserved horizontal column width for
	// the rail + marker before the event's text begins.
	TimelineMarkerW = 12
	// TimelineMarkerSize is the pixel side of each event's filled
	// square marker painted on the rail.
	TimelineMarkerSize = 6
	// TimelineDetailGap is the vertical space inserted between an
	// event's Title row and its Detail row when Detail != "".
	TimelineDetailGap = 2
	// TimelinePadX is the horizontal inset between the widget's
	// left edge and the rail's marker column.
	TimelinePadX = 8
	// TimelinePadY is the vertical inset between the widget's top
	// edge and the first event row (and between the last event row
	// and the bottom edge).
	TimelinePadY = 8
)

// TimelineEventH is the vertical stride from one event's Title row to the next
// when the event has NO Detail — one glyph row plus 4 px of inter-event
// spacing. A function, as it derives from the active font's GlyphHeight.
func TimelineEventH() int { return GlyphHeight() + 4 }

// eventBlockH is the full vertical extent one event occupies in the vertical
// layout: the base stride plus, when the event carries a Detail line, the gap
// and the Detail glyph row beneath it.
func (tl *Timeline) eventBlockH(ev TimelineEvent) int {
	h := TimelineEventH()
	if ev.Detail != "" {
		h += scaled(TimelineDetailGap) + tl.glyphHeight()
	}
	return h
}

// contentH is the total pixel height the vertical event list occupies,
// including the top + bottom padding bands, used to clamp the scroll offset.
func (tl *Timeline) contentH() int {
	h := 2 * scaled(TimelinePadY)
	for _, ev := range tl.Events {
		h += tl.eventBlockH(ev)
	}
	return h
}

// maxScrollY is the highest scrollY that still leaves the list filling the
// widget, floored at 0 so a list that already fits never scrolls.
func (tl *Timeline) maxScrollY() int {
	m := tl.contentH() - tl.Bounds().H
	if m < 0 {
		m = 0
	}
	return m
}

// clampedScrollY returns scrollY clamped to [0, maxScrollY()] without mutating
// the field, so a value left stale after the list shrank never paints or
// hit-tests outside the valid window.
func (tl *Timeline) clampedScrollY() int {
	v := tl.scrollY
	if v < 0 {
		v = 0
	}
	if m := tl.maxScrollY(); v > m {
		v = m
	}
	return v
}

// ScrollBy shifts the vertical scroll offset by delta rows (negative scrolls
// up), converting rows to pixels through TimelineEventH and clamping to
// [0, maxScrollY()]. A no-op for a horizontal timeline, which does not scroll.
func (tl *Timeline) ScrollBy(delta int) {
	if tl.Horizontal {
		return
	}
	tl.scrollY += delta * TimelineEventH()
	tl.scrollY = tl.clampedScrollY()
}

// EventAt maps a widget-local (x, y) to the index of the vertical-timeline
// event under it, accounting for the scroll offset, or -1 for the padding
// bands, a point outside the widget's width, or empty space below the last
// event. A horizontal timeline always returns -1 (its ribbon layout is
// hit-tested by the caller). It is the offset-aware inverse of Draw's row
// walk, so a click after scrolling resolves to the event actually shown.
func (tl *Timeline) EventAt(x, y int) int {
	if tl.Horizontal || x < 0 || x >= tl.Bounds().W {
		return -1
	}
	cy := scaled(TimelinePadY) - tl.clampedScrollY()
	for i, ev := range tl.Events {
		h := tl.eventBlockH(ev)
		if y >= cy && y < cy+h {
			return i
		}
		cy += h
	}
	return -1
}

// OnEvent handles the mouse wheel: a vertical timeline scrolls its event list
// by EventScroll.Delta rows (clamped at both ends by ScrollBy) so a long log
// stays reachable. Every other event -- and any event on a horizontal timeline
// -- is ignored, preserving Timeline's otherwise passive-display contract.
func (tl *Timeline) OnEvent(ev Event) {
	if ev.Kind == EventScroll {
		tl.ScrollBy(ev.Delta)
	}
}

// NewTimeline constructs a Timeline carrying the given events. A
// nil events slice is normalised to a non-nil empty slice so
// downstream code (range loops, len() checks) never has to guard
// for nil separately.
func NewTimeline(events []TimelineEvent) *Timeline {
	if events == nil {
		events = []TimelineEvent{}
	}
	return &Timeline{Events: events}
}

// timelineMarkerInk maps a Kind to the fill colour of its marker
// square. TimelineDefault defers to the theme so a neutral event
// matches the app's accent; the other three carry fixed shades
// reused verbatim from Alert (green / amber / red) so a Timeline
// row and an Alert banner of the same severity read as the same
// colour.
func timelineMarkerInk(kind TimelineKind, theme *Theme) RGBA {
	switch kind {
	case TimelineSuccess:
		return RGB(0x2E, 0x8B, 0x57) // sea green — same as AlertSuccess
	case TimelineWarning:
		return RGB(0xE0, 0xA0, 0x30) // amber — same as AlertWarning
	case TimelineError:
		return RGB(0xC0, 0x30, 0x30) // brick red — same as AlertError
	default: // TimelineDefault (also any out-of-range Kind values)
		return theme.Accent
	}
}

// Draw paints the surface fill, the vertical rail line, one marker
// per event and each event's Title (+ optional Detail). The rail is
// painted BEFORE the markers so a marker overwrites the rail pixel
// where they intersect, giving the marker its full square silhouette
// without a separate clipping pass.
func (tl *Timeline) Draw(p painter.Painter, theme *Theme) {
	if tl.Horizontal {
		tl.drawHorizontal(p, theme)
		return
	}
	r := tl.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	railX := r.X + scaled(TimelinePadX) + scaled(TimelineMarkerW)/2
	railY := r.Y + scaled(TimelinePadY)
	railH := r.H - 2*scaled(TimelinePadY)
	fillRect(p, railX, railY, 1, railH, theme.Border)

	textX := r.X + scaled(TimelinePadX) + scaled(TimelineMarkerW)
	// Shift the event window up by the scroll offset and clip it to Bounds so a
	// long log never bleeds past the widget. At scrollY == 0 the clip contains
	// the whole (fitting) list, leaving the render byte-identical.
	y := r.Y + scaled(TimelinePadY) - tl.clampedScrollY()
	withClip(p, r, func() {
		for _, ev := range tl.Events {
			markerX := railX - scaled(TimelineMarkerSize)/2
			markerY := y + (tl.glyphHeight()-scaled(TimelineMarkerSize))/2
			fillRect(p, markerX, markerY, scaled(TimelineMarkerSize), scaled(TimelineMarkerSize),
				timelineMarkerInk(ev.Kind, theme))
			tl.drawText(p, textX, y, ev.Title, theme.OnSurface)
			blockH := TimelineEventH()
			if ev.Detail != "" {
				tl.drawText(p, textX, y+tl.glyphHeight()+scaled(TimelineDetailGap), ev.Detail, dimInk(theme))
				blockH += scaled(TimelineDetailGap) + tl.glyphHeight()
			}
			y += blockH
		}
	})
}

// drawHorizontal paints the timeline as a left-to-right ribbon: a single
// horizontal rail near the top with one marker per event spread along it, and
// each event's Title (+ optional Detail) rendered below its marker. Column
// widths follow the widest of an event's Title / Detail so captions never
// overlap.
func (tl *Timeline) drawHorizontal(p painter.Painter, theme *Theme) {
	r := tl.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	railY := r.Y + scaled(TimelinePadY) + scaled(TimelineMarkerW)/2
	railX := r.X + scaled(TimelinePadX)
	railW := r.W - 2*scaled(TimelinePadX)
	fillRect(p, railX, railY, railW, 1, theme.Border)

	textY := r.Y + scaled(TimelinePadY) + scaled(TimelineMarkerW)
	x := r.X + scaled(TimelinePadX)
	for _, ev := range tl.Events {
		colW := tl.textWidth(ev.Title)
		if w := tl.textWidth(ev.Detail); w > colW {
			colW = w
		}
		if colW < scaled(TimelineMarkerSize) {
			colW = scaled(TimelineMarkerSize)
		}
		markerX := x + (colW-scaled(TimelineMarkerSize))/2
		markerY := railY - scaled(TimelineMarkerSize)/2
		fillRect(p, markerX, markerY, scaled(TimelineMarkerSize), scaled(TimelineMarkerSize),
			timelineMarkerInk(ev.Kind, theme))
		tl.drawText(p, x, textY, ev.Title, theme.OnSurface)
		if ev.Detail != "" {
			tl.drawText(p, x, textY+tl.glyphHeight()+scaled(TimelineDetailGap), ev.Detail, dimInk(theme))
		}
		x += colW + scaled(TimelinePadX)
	}
}

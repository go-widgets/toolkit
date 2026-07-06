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
// Timeline is a passive display container — hit-testing / event
// routing are not implemented. A caller who wants click-to-focus
// walks the same layout math externally.
type Timeline struct {
	Base
	Events []TimelineEvent
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
	// TimelineEventH is the vertical stride from one event's Title
	// row to the next when the event has NO Detail — one glyph row
	// plus 4 px of inter-event spacing.
	TimelineEventH = GlyphHeight + 4
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
	r := tl.Bounds()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)

	railX := r.X + TimelinePadX + TimelineMarkerW/2
	railY := r.Y + TimelinePadY
	railH := r.H - 2*TimelinePadY
	fillRect(p, railX, railY, 1, railH, theme.Border)

	textX := r.X + TimelinePadX + TimelineMarkerW
	y := r.Y + TimelinePadY
	for _, ev := range tl.Events {
		markerX := railX - TimelineMarkerSize/2
		markerY := y + (GlyphHeight-TimelineMarkerSize)/2
		fillRect(p, markerX, markerY, TimelineMarkerSize, TimelineMarkerSize,
			timelineMarkerInk(ev.Kind, theme))
		DrawText(p, textX, y, ev.Title, theme.OnSurface)
		blockH := TimelineEventH
		if ev.Detail != "" {
			DrawText(p, textX, y+GlyphHeight+TimelineDetailGap, ev.Detail, theme.Border)
			blockH += TimelineDetailGap + GlyphHeight
		}
		y += blockH
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Breadcrumbs is a horizontal navigation path — "Home > Docs >
// Reference" — rendered as a sequence of Segments separated by a
// chevron character. Segment text uses Theme.OnBackground; each
// chevron uses Theme.Border so it reads as a subtle divider rather
// than another clickable label.
//
// A click on a crumb fires OnSelect with that segment's index, so
// "Home > Docs > Reference" navigates up when the user clicks an
// ancestor crumb. OnSelect nil leaves the widget an inert display: the
// same per-segment X layout Draw builds is walked by OnEvent to hit-test
// the clicked crumb, so the click target and the drawn glyph can never
// drift apart.
type Breadcrumbs struct {
	Base
	Segments []string
	// OnSelect, when non-nil, fires with the 0-based index of the crumb the
	// user clicked. Nil (the zero value) keeps the widget passive.
	OnSelect func(i int)
}

// BreadcrumbSep is the character(s) drawn between two segments. Kept as
// a package constant so a caller who wants "/" or "»" replaces one
// symbol without touching Draw.
const BreadcrumbSep = ">"

// BreadcrumbGap is the horizontal pixel gap inserted on either side of
// the separator glyph so the chevron doesn't touch the segment ink.
const BreadcrumbGap = 4

// NewBreadcrumbs constructs a Breadcrumbs with the given segments.
// A nil or empty Segments slice renders as a no-op — Draw exits without
// painting anything.
func NewBreadcrumbs(segments []string) *Breadcrumbs {
	return &Breadcrumbs{Segments: segments}
}

// Draw paints each segment followed by a separator (except after the
// last one). Segments are vertically centred inside Bounds when
// Bounds.H exceeds GlyphHeight(), otherwise they anchor at Bounds.Y.
func (b *Breadcrumbs) Draw(p painter.Painter, theme *Theme) {
	r := b.Bounds()
	ty := r.Y
	if r.H > b.glyphHeight() {
		ty = r.Y + (r.H-b.glyphHeight())/2
	}
	x := r.X
	n := len(b.Segments)
	for i, seg := range b.Segments {
		b.drawText(p, x, ty, seg, theme.OnBackground)
		x += b.textWidth(seg)
		if i < n-1 {
			x += BreadcrumbGap
			b.drawText(p, x, ty, BreadcrumbSep, theme.Border)
			x += b.textWidth(BreadcrumbSep) + BreadcrumbGap
		}
	}
}

// OnEvent fires OnSelect(i) when a click lands on the i-th crumb. It walks the
// exact per-segment X layout Draw builds (textWidth(seg) then, between crumbs,
// gap + separator + gap), so the hit region matches the painted glyphs. Clicks
// in the inter-crumb separator gap, or when OnSelect is nil, are ignored. Event
// coordinates are widget-local, so the first crumb starts at local x == 0.
func (b *Breadcrumbs) OnEvent(ev Event) {
	if ev.Kind != EventClick || b.OnSelect == nil {
		return
	}
	n := len(b.Segments)
	x := 0
	for i, seg := range b.Segments {
		w := b.textWidth(seg)
		if ev.X >= x && ev.X < x+w {
			b.OnSelect(i)
			return
		}
		x += w
		if i < n-1 {
			x += BreadcrumbGap + b.textWidth(BreadcrumbSep) + BreadcrumbGap
		}
	}
}

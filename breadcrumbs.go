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
// The widget is passive display only: it computes per-segment X
// positions from TextWidth so a caller wanting hit-testing (e.g.
// clicking "Docs" to navigate up two levels) can walk the same offset
// table externally. HitTest / OnEvent stay as Base defaults.
type Breadcrumbs struct {
	Base
	Segments []string
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
	if r.H > GlyphHeight() {
		ty = r.Y + (r.H-GlyphHeight())/2
	}
	x := r.X
	n := len(b.Segments)
	for i, seg := range b.Segments {
		DrawText(p, x, ty, seg, theme.OnBackground)
		x += TextWidth(seg)
		if i < n-1 {
			x += BreadcrumbGap
			DrawText(p, x, ty, BreadcrumbSep, theme.Border)
			x += TextWidth(BreadcrumbSep) + BreadcrumbGap
		}
	}
}

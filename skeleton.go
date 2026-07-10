// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// SkeletonKind selects the placeholder shape. Three kinds cover the
// dominant loading-state patterns: SkeletonText for a paragraph or a
// list row, SkeletonAvatar for an identity chip (paired with
// SkeletonText in a message row), SkeletonBlock for a media thumbnail
// or a card body.
type SkeletonKind int

const (
	// SkeletonText draws N horizontal bars stacked vertically. The last
	// bar is 60% width so the shape reads as "wrapped text" rather than
	// a solid block.
	SkeletonText SkeletonKind = iota
	// SkeletonAvatar draws a rounded square in SurfaceAlt matching the
	// Avatar widget's shape — so a Skeleton row lines up pixel-for-pixel
	// with the real Avatar it will be swapped for once the data loads.
	SkeletonAvatar
	// SkeletonBlock draws one filled rectangle covering Bounds() inset
	// by SkeletonLinePad — the "media thumbnail loading" affordance.
	SkeletonBlock
)

// Skeleton is a placeholder rendered while real content is loading.
// Every Skeleton fills in Theme.SurfaceAlt so the shape reads as
// "content coming" without demanding attention; there is no text or
// animation — the visual weight alone signals the pending state.
//
// A caller typically swaps a Skeleton for the real widget once data
// arrives; there is no Visible flag because dropping the widget from
// the tree is cheaper than gating every Draw on a bool.
//
// Skeleton is passive: it displays and does not respond to input.
type Skeleton struct {
	Base
	Kind  SkeletonKind
	Lines int
}

// Skeleton sizing constants. Values chosen to line up with the toolkit's
// GlyphHeight() so a SkeletonText row visually replaces a row of body
// text without shifting the surrounding layout.
const (
	// SkeletonLineH is the pixel height of a single SkeletonText bar.
	SkeletonLineH = 10
	// SkeletonLineGap is the vertical gap between two SkeletonText bars.
	SkeletonLineGap = 6
	// SkeletonLinePad is the inset applied to SkeletonBlock so the fill
	// stops shy of the Bounds edge — matches Card's body pad.
	SkeletonLinePad = 4
)

// NewSkeleton constructs a Skeleton of the given kind + line count. The
// lines argument is honoured only when kind == SkeletonText; if it is
// non-positive in that case it defaults to 3 (a natural stand-in for a
// paragraph). For SkeletonAvatar / SkeletonBlock the value is stored
// verbatim but ignored by Draw.
func NewSkeleton(kind SkeletonKind, lines int) *Skeleton {
	if kind == SkeletonText && lines <= 0 {
		lines = 3
	}
	return &Skeleton{Kind: kind, Lines: lines}
}

// Draw paints the placeholder appropriate for Kind. Nothing else about
// the widget is theme-aware: every fill lands in Theme.SurfaceAlt so
// the placeholder recedes into the panel without demanding attention.
func (s *Skeleton) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	switch s.Kind {
	case SkeletonAvatar:
		// Same three-band pill Avatar draws — so a SkeletonAvatar next
		// to a SkeletonText row previews the future Avatar exactly.
		fillRect(p, r.X+1, r.Y, r.W-2, r.H, theme.SurfaceAlt)
		fillRect(p, r.X, r.Y+1, 1, r.H-2, theme.SurfaceAlt)
		fillRect(p, r.X+r.W-1, r.Y+1, 1, r.H-2, theme.SurfaceAlt)
	case SkeletonBlock:
		fillRect(p, r.X+SkeletonLinePad, r.Y+SkeletonLinePad,
			r.W-2*SkeletonLinePad, r.H-2*SkeletonLinePad, theme.SurfaceAlt)
	default: // SkeletonText (also any out-of-range Kind values)
		y := r.Y
		for i := 0; i < s.Lines; i++ {
			w := r.W
			// The last bar reads as "wrapped text" — 60% width so the
			// row terminates naturally rather than looking like a
			// solid block.
			if i == s.Lines-1 {
				w = (r.W * 3) / 5
			}
			fillRect(p, r.X, y, w, SkeletonLineH, theme.SurfaceAlt)
			y += SkeletonLineH + SkeletonLineGap
		}
	}
}

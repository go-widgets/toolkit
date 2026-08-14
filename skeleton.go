// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"

	"github.com/go-widgets/painter"
)

// SkeletonKind selects the placeholder shape. The kinds cover the
// dominant loading-state patterns:
//
//   - SkeletonText draws N rounded bars stacked vertically (a paragraph
//     or a list row).
//   - SkeletonRect draws one rounded block — the modern "media / card
//     body loading" affordance, corner-radius configurable.
//   - SkeletonCircle draws a true circle — an avatar / status-dot
//     placeholder.
//   - SkeletonAvatar / SkeletonBlock are the original pixel-exact
//     swap-parity variants (a three-band pill matching Avatar, and a
//     square inset fill). They are kept for callers that swap a
//     Skeleton for the Avatar / Block widget pixel-for-pixel.
//
// Every kind fills in Theme.SurfaceAlt (the muted "content coming"
// tone) and, when Animated, is swept by a diagonal shimmer band (a
// lighter tint) whose position is driven by Phase.
type SkeletonKind int

const (
	// SkeletonText draws Lines horizontal bars stacked vertically. The
	// last bar is LastFrac of the width so the shape reads as "wrapped
	// text" rather than a solid block.
	SkeletonText SkeletonKind = iota
	// SkeletonAvatar draws a rounded square in SurfaceAlt matching the
	// Avatar widget's three-band pill — so a Skeleton row lines up
	// pixel-for-pixel with the real Avatar it will be swapped for.
	SkeletonAvatar
	// SkeletonBlock draws one filled square rectangle covering Bounds()
	// inset by SkeletonLinePad — the original media-thumbnail affordance.
	SkeletonBlock
	// SkeletonRect draws one rounded-corner block covering Bounds(). The
	// corner radius is Skeleton.Radius (default SkeletonRectRadius).
	SkeletonRect
	// SkeletonCircle draws a true circle inscribed in (and centred
	// within) Bounds() — the avatar placeholder for the rounded family.
	SkeletonCircle
)

// Skeleton is a placeholder rendered while real content is loading.
// Every Skeleton fills in Theme.SurfaceAlt so the shape reads as
// "content coming" without demanding attention.
//
// When Animated is set, Draw overlays a diagonal shimmer band — a
// lighter tint that sweeps across the base grey. The band position is
// Phase (0..1); the consumer advances Phase every frame (typically via
// SetPhase(elapsed*speed), which wraps for you). A stopped Skeleton
// (Animated == false, the zero value) renders flat grey, so the widget
// is cheap when the host has no animation loop.
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

	// LineH / LineGap / LastFrac tune SkeletonText. Zero (the default)
	// falls through to SkeletonLineH / SkeletonLineGap / SkeletonLastFrac
	// so an untuned SkeletonText renders like a paragraph of body text.
	LineH    int
	LineGap  int
	LastFrac float64

	// Radius is the corner radius for SkeletonRect and for SkeletonText
	// bars. Zero falls through to a shape-appropriate default
	// (SkeletonRectRadius for a rect; a third of the line height for a
	// text bar). SkeletonCircle ignores it (a circle is fully rounded).
	Radius int

	// Animated turns the shimmer band on. Phase (0..1) is the sweep
	// position: 0 parks the band just off the leading edge (flat grey),
	// rising to 1 sweeps it off the trailing edge.
	Animated bool
	Phase    float64
}

// Skeleton sizing + shimmer constants. Line values line up with the
// toolkit's GlyphHeight() so a SkeletonText row visually replaces a row
// of body text without shifting the surrounding layout.
const (
	// SkeletonLineH is the default pixel height of a SkeletonText bar.
	SkeletonLineH = 10
	// SkeletonLineGap is the default vertical gap between two bars.
	SkeletonLineGap = 6
	// SkeletonLinePad is the inset applied to SkeletonBlock so the fill
	// stops shy of the Bounds edge — matches Card's body pad.
	SkeletonLinePad = 4
	// SkeletonLastFrac is the default width fraction of the last text
	// bar (60%), so the paragraph terminates naturally.
	SkeletonLastFrac = 0.6
	// SkeletonRectRadius is the default corner radius for SkeletonRect.
	SkeletonRectRadius = 6

	// skeletonBandFrac is the shimmer band width as a fraction of the
	// swept region's width.
	skeletonBandFrac = 0.35
	// skeletonShimmerSkew tilts the band diagonally: the band centre for
	// a row shifts by skew*(rows-from-bottom), so the highlight reads as
	// a diagonal gleam rather than a vertical wipe.
	skeletonShimmerSkew = 0.6
	// skeletonShimmerAmp is the peak opacity of the highlight at the band
	// centre (0..1).
	skeletonShimmerAmp = 0.6
	// skeletonHighlightMix is how far the band tint sits from the base
	// toward white (0 = base, 1 = white). A lift that reads as "lighter"
	// in BOTH light and dark themes.
	skeletonHighlightMix = 0.6
)

// NewSkeleton constructs a Skeleton of the given kind + line count. The
// lines argument is honoured only when kind == SkeletonText; if it is
// non-positive in that case it defaults to 3 (a natural stand-in for a
// paragraph). For the non-text kinds the value is stored verbatim but
// ignored by Draw.
func NewSkeleton(kind SkeletonKind, lines int) *Skeleton {
	if kind == SkeletonText && lines <= 0 {
		lines = 3
	}
	return &Skeleton{Kind: kind, Lines: lines}
}

// SetPhase sets the shimmer sweep position and switches the shimmer on.
// t may be any float (e.g. elapsedSeconds*speed); it is wrapped into
// [0,1) so the caller can feed a monotonically increasing clock without
// tracking the cycle. Returns the Skeleton so the call chains.
func (s *Skeleton) SetPhase(t float64) *Skeleton {
	t -= math.Floor(t)
	s.Phase = t
	s.Animated = true
	return s
}

// Tick advances the shimmer sweep by deltaSeconds, wrapping Phase modulo 1 so
// it stays bounded. It advances only while Animated (a flat, un-animated
// Skeleton needs no frames), matching what Animating reports. Together they make
// an animated Skeleton an [Animator], driven by [TickTree] / [TreeAnimating] —
// the per-frame counterpart of the absolute-clock SetPhase.
func (s *Skeleton) Tick(deltaSeconds float64) {
	if !s.Animated {
		return
	}
	s.Phase += deltaSeconds
	s.Phase -= math.Floor(s.Phase)
}

// Animating reports whether the skeleton still needs frames: true exactly when
// its shimmer is Animated (a static placeholder needs no repaint).
func (s *Skeleton) Animating() bool { return s.Animated }

// effLineH / effLineGap / effLastFrac / textBarRadius resolve the tuning
// fields to their defaults when left at the zero value.
func (s *Skeleton) effLineH() int {
	if s.LineH > 0 {
		return s.LineH
	}
	return SkeletonLineH
}

func (s *Skeleton) effLineGap() int {
	if s.LineGap > 0 {
		return s.LineGap
	}
	return SkeletonLineGap
}

func (s *Skeleton) effLastFrac() float64 {
	if s.LastFrac > 0 {
		return s.LastFrac
	}
	return SkeletonLastFrac
}

// textBarRadius is the corner radius of a SkeletonText bar: the caller's
// Radius override, else a third of the line height (a gently rounded
// bar). Painter clamps it to half the smaller side.
func (s *Skeleton) textBarRadius(lineH int) int {
	if s.Radius > 0 {
		return s.Radius
	}
	return lineH / 3
}

// rectRadius is the corner radius of a SkeletonRect: the caller's Radius
// override, else SkeletonRectRadius.
func (s *Skeleton) rectRadius() int {
	if s.Radius > 0 {
		return s.Radius
	}
	return SkeletonRectRadius
}

// Draw paints the placeholder appropriate for Kind, then (when
// Animated) sweeps the shimmer band over each filled region.
func (s *Skeleton) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	base := theme.SurfaceAlt
	switch s.Kind {
	case SkeletonAvatar:
		// Three-band pill Avatar draws — so a SkeletonAvatar next to a
		// SkeletonText row previews the future Avatar exactly.
		fillRect(p, r.X+1, r.Y, r.W-2, r.H, base)
		fillRect(p, r.X, r.Y+1, 1, r.H-2, base)
		fillRect(p, r.X+r.W-1, r.Y+1, 1, r.H-2, base)
		s.shimmer(p, r, base)
	case SkeletonBlock:
		in := Rect{X: r.X + SkeletonLinePad, Y: r.Y + SkeletonLinePad,
			W: r.W - 2*SkeletonLinePad, H: r.H - 2*SkeletonLinePad}
		fillRect(p, in.X, in.Y, in.W, in.H, base)
		s.shimmer(p, in, base)
	case SkeletonRect:
		fillRoundRect(p, r.X, r.Y, r.W, r.H, s.rectRadius(), base)
		s.shimmer(p, r, base)
	case SkeletonCircle:
		d := r.W
		if r.H < d {
			d = r.H
		}
		cx := r.X + (r.W-d)/2
		cy := r.Y + (r.H-d)/2
		fillRoundRect(p, cx, cy, d, d, d/2, base)
		s.shimmer(p, Rect{X: cx, Y: cy, W: d, H: d}, base)
	default: // SkeletonText (also any out-of-range Kind values)
		lineH := s.effLineH()
		gap := s.effLineGap()
		frac := s.effLastFrac()
		rad := s.textBarRadius(lineH)
		y := r.Y
		for i := 0; i < s.Lines; i++ {
			w := r.W
			if i == s.Lines-1 {
				w = int(float64(r.W) * frac)
			}
			fillRoundRect(p, r.X, y, w, lineH, rad, base)
			s.shimmer(p, Rect{X: r.X, Y: y, W: w, H: lineH}, base)
			y += lineH + gap
		}
	}
}

// shimmer overlays a diagonal light band over the filled region `area`.
// It is a no-op unless Animated. The band is a triangular highlight
// profile: opacity peaks at the band centre and falls to zero at
// ±half-band-width, so the sweep reads as a soft gleam. Every write
// lands inside `area` (⊆ Bounds), so the widget never paints out of
// bounds.
func (s *Skeleton) shimmer(p painter.Painter, area Rect, base RGBA) {
	if !s.Animated || area.W <= 0 || area.H <= 0 {
		return
	}
	hl := skeletonHighlight(base)
	bandW := int(float64(area.W) * skeletonBandFrac)
	if bandW < 1 {
		bandW = 1
	}
	bf := float64(bandW)
	// Sweep coordinate span: the diagonal reach across the region plus a
	// band-width of runway on each side so Phase 0 / 1 park the band just
	// off either edge.
	span := float64(area.W) + skeletonShimmerSkew*float64(area.H)
	centre := s.Phase*(span+2*bf) - bf
	for yy := 0; yy < area.H; yy++ {
		// Rows nearer the bottom lead the sweep, giving the diagonal tilt.
		skew := skeletonShimmerSkew * float64(area.H-1-yy)
		for xx := 0; xx < area.W; xx++ {
			u := float64(xx) + skew
			d := math.Abs(u - centre)
			if d >= bf {
				continue
			}
			cov := (1 - d/bf) * skeletonShimmerAmp
			a := uint8(cov*255 + 0.5)
			putPixel(p, area.X+xx, area.Y+yy, RGBA{R: hl.R, G: hl.G, B: hl.B, A: a})
		}
	}
}

// skeletonHighlight lifts base toward white by skeletonHighlightMix,
// yielding a tint that reads as "lighter" against the base in both
// light and dark themes.
func skeletonHighlight(base RGBA) RGBA {
	return blendRGBA(RGB(0xFF, 0xFF, 0xFF), base, skeletonHighlightMix)
}

// SkeletonItem is one positioned child of a SkeletonGroup: a primitive
// Skeleton plus its rectangle in the group's LOCAL coordinate system
// (relative to the group's top-left).
type SkeletonItem struct {
	Skel  *Skeleton
	Local Rect
}

// SkeletonGroup composes several primitive Skeletons into one loading
// placeholder — an avatar + text lines + a media block, a whole loading
// page, etc. It is a thin container: Draw positions each child relative
// to the group's Bounds and forwards the group's shimmer Phase so the
// whole composition gleams in sync.
//
// SkeletonGroup is passive and decorative (A11y reports it as a
// presentation element, like the primitive Skeleton).
type SkeletonGroup struct {
	Base
	items []SkeletonItem
	// Animated + Phase mirror Skeleton; SetPhase drives both and they
	// cascade to every child at Draw time.
	Animated bool
	Phase    float64
}

// Add appends a primitive Skeleton at the given LOCAL rectangle and
// returns the group so calls chain.
func (g *SkeletonGroup) Add(s *Skeleton, local Rect) *SkeletonGroup {
	g.items = append(g.items, SkeletonItem{Skel: s, Local: local})
	return g
}

// Items returns the group's children with their local rectangles, for
// inspection / layout tests.
func (g *SkeletonGroup) Items() []SkeletonItem { return g.items }

// SetPhase sets the group's shimmer position (wrapped into [0,1)) and
// switches the shimmer on for every child. Returns the group so calls
// chain. The consumer advances this every frame.
func (g *SkeletonGroup) SetPhase(t float64) *SkeletonGroup {
	t -= math.Floor(t)
	g.Phase = t
	g.Animated = true
	return g
}

// Tick advances the group's shimmer sweep by deltaSeconds, wrapping Phase
// modulo 1. It advances only while Animated and cascades to every child at Draw
// time (Draw copies the group's Phase into each child), so ticking the group is
// enough to animate the whole composition. Together with Animating this makes
// SkeletonGroup an [Animator] driven by [TickTree] / [TreeAnimating].
func (g *SkeletonGroup) Tick(deltaSeconds float64) {
	if !g.Animated {
		return
	}
	g.Phase += deltaSeconds
	g.Phase -= math.Floor(g.Phase)
}

// Animating reports whether the group still needs frames: true exactly when its
// shimmer is Animated.
func (g *SkeletonGroup) Animating() bool { return g.Animated }

// Draw positions each child relative to the group's Bounds, forwards
// the shimmer state, and paints it.
func (g *SkeletonGroup) Draw(p painter.Painter, theme *Theme) {
	o := g.Bounds()
	for i := range g.items {
		it := g.items[i]
		it.Skel.SetBounds(Rect{X: o.X + it.Local.X, Y: o.Y + it.Local.Y,
			W: it.Local.W, H: it.Local.H})
		it.Skel.Animated = g.Animated
		it.Skel.Phase = g.Phase
		it.Skel.Draw(p, theme)
	}
}

// NewSkeletonCard builds a content-card skeleton inside bounds: a circle
// avatar top-left, a two-line text header beside it, and a rounded media
// block filling the rest — the classic "post is loading" placeholder.
// It is a composition of the primitives, so callers can inspect / tweak
// the children via Items().
func NewSkeletonCard(bounds Rect) *SkeletonGroup {
	const (
		pad     = 8
		avatarD = 40
		headerH = 2*SkeletonLineH + SkeletonLineGap
	)
	g := &SkeletonGroup{}
	g.SetBounds(bounds)

	// Avatar circle, top-left.
	g.Add(NewSkeleton(SkeletonCircle, 0), Rect{X: pad, Y: pad, W: avatarD, H: avatarD})

	// Two-line text header to the right of the avatar, vertically
	// centred against it.
	hx := pad + avatarD + pad
	hy := pad + (avatarD-headerH)/2
	header := &Skeleton{Kind: SkeletonText, Lines: 2}
	g.Add(header, Rect{X: hx, Y: hy, W: bounds.W - hx - pad, H: headerH})

	// Media block filling the remainder.
	by := pad + avatarD + pad
	g.Add(NewSkeleton(SkeletonRect, 0),
		Rect{X: pad, Y: by, W: bounds.W - 2*pad, H: bounds.H - by - pad})
	return g
}

// NewPageSkeleton builds a loading web-page placeholder inside bounds: a
// top bar, alternating paragraph line-groups and media blocks. This is
// what a webengine browser client shows while the browserproxy fetches
// a page. It is a preset (a composition of the primitives), not a
// bespoke widget, so it is reusable + inspectable via Items().
func NewPageSkeleton(bounds Rect) *SkeletonGroup {
	const (
		pad    = 12
		gap    = 16
		barH   = 24
		paraH  = 3*SkeletonLineH + 2*SkeletonLineGap
		imageH = 90
	)
	g := &SkeletonGroup{}
	g.SetBounds(bounds)
	innerW := bounds.W - 2*pad

	y := pad
	add := func(k SkeletonKind, lines, h int) {
		g.Add(&Skeleton{Kind: k, Lines: lines}, Rect{X: pad, Y: y, W: innerW, H: h})
		y += h + gap
	}
	// Top navigation bar.
	add(SkeletonRect, 0, barH)
	// A paragraph, a hero image, a second paragraph, a second image —
	// the rhythm of a typical article page.
	add(SkeletonText, 3, paraH)
	add(SkeletonRect, 0, imageH)
	add(SkeletonText, 3, paraH)
	add(SkeletonRect, 0, imageH)
	return g
}

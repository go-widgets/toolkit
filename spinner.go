// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

import "math"

// SpinnerStyle selects an indeterminate-spinner look. The zero value is
// SpinnerHand, so an untouched Spinner keeps the original rotating-hand
// rendering.
type SpinnerStyle int

const (
	// SpinnerHand is a single rotating radial line from the centre (the
	// original, and the zero-value default).
	SpinnerHand SpinnerStyle = iota
	// SpinnerDots is a ring of dots orbiting the centre, the leading dot in
	// Accent and the trail fading toward SurfaceAlt.
	SpinnerDots
	// SpinnerRing is a comet-like arc sweeping around the circle, its head in
	// Accent fading to SurfaceAlt along the tail.
	SpinnerRing
	// SpinnerBars is a row of vertical bars whose heights pulse out of phase,
	// like an audio equalizer.
	SpinnerBars
)

// Spinner is an indeterminate loading indicator. When Active, Draw paints the
// selected Style advanced by Phase in Theme.Accent. The caller drives Phase via
// Tick(dt) so the animation cadence stays tied to the host's frame loop (no
// goroutine, no timer).
type Spinner struct {
	Base
	Active bool
	Phase  float64      // 0..1, full cycle
	Style  SpinnerStyle // look; zero value = SpinnerHand
}

// NewSpinner builds a Spinner stopped at Phase=0 in the default (hand) style.
func NewSpinner() *Spinner { return &Spinner{} }

// Tick advances Phase by deltaSeconds, wrapping modulo 1 so the value stays
// bounded.
func (s *Spinner) Tick(deltaSeconds float64) {
	s.Phase += deltaSeconds
	s.Phase -= math.Floor(s.Phase)
}

// Draw paints the spinner when Active, dispatching on Style. It is a no-op when
// inactive or given empty bounds.
func (s *Spinner) Draw(p painter.Painter, theme *Theme) {
	if !s.Active {
		return
	}
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	switch s.Style {
	case SpinnerDots:
		s.drawDots(p, theme, r)
	case SpinnerRing:
		s.drawRing(p, theme, r)
	case SpinnerBars:
		s.drawBars(p, theme, r)
	default:
		s.drawHand(p, theme, r)
	}
}

// spinnerGeom returns the centre and the smaller side of r, the shared basis for
// every spinner style's sizing.
func spinnerGeom(r Rect) (cx, cy float64, minSide int) {
	cx = float64(r.X) + float64(r.W)/2
	cy = float64(r.Y) + float64(r.H)/2
	minSide = r.W
	if r.H < r.W {
		minSide = r.H
	}
	return
}

// atLeast1 floors v at 1, so a sub-pixel size never collapses a marker to
// nothing (and keeps the size math branch-free at the call sites).
func atLeast1(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

// drawHand paints the original rotating radial line (length = 40% of the
// smaller side) as single pixels from the centre outward.
func (s *Spinner) drawHand(p painter.Painter, theme *Theme, r Rect) {
	cx, cy, minSide := spinnerGeom(r)
	radius := float64(minSide) / 2 * 0.4
	angle := s.Phase * 2 * math.Pi
	dx, dy := math.Cos(angle), math.Sin(angle)
	steps := atLeast1(int(radius))
	for t := 0; t < steps; t++ {
		f := float64(t) / float64(steps) * radius
		fillRect(p, int(cx+dx*f), int(cy+dy*f), 1, 1, theme.Accent)
	}
}

// drawDots paints eight dots evenly spaced on an orbit, the one at the current
// head in Accent and the rest fading toward SurfaceAlt with distance behind it.
func (s *Spinner) drawDots(p painter.Painter, theme *Theme, r Rect) {
	const n = 8
	cx, cy, minSide := spinnerGeom(r)
	orbit := float64(minSide) / 2 * 0.72
	dotR := atLeast1(minSide / 12)
	head := s.Phase * n
	for i := 0; i < n; i++ {
		ang := 2 * math.Pi * float64(i) / n
		behind := math.Mod(head-float64(i), n)
		if behind < 0 {
			behind += n
		}
		col := blendRGBA(theme.Accent, theme.SurfaceAlt, 1-behind/n)
		px := int(cx+orbit*math.Cos(ang)) - dotR
		py := int(cy+orbit*math.Sin(ang)) - dotR
		p.FillRoundRect(painter.Rect{X: px, Y: py, W: dotR * 2, H: dotR * 2}, dotR, col)
	}
}

// drawRing paints a comet arc (three-quarter sweep) around the circle, its head
// (at Phase) in Accent fading to SurfaceAlt along the tail.
func (s *Spinner) drawRing(p painter.Painter, theme *Theme, r Rect) {
	cx, cy, minSide := spinnerGeom(r)
	orbit := float64(minSide) / 2 * 0.8
	const sweep = 1.5 * math.Pi
	dot := atLeast1(minSide / 16)
	steps := atLeast1(minSide * 2) // minSide ≥ 1 (empty bounds are rejected) ⇒ steps ≥ 2
	start := s.Phase * 2 * math.Pi
	for k := 0; k < steps; k++ {
		u := float64(k) / float64(steps-1) // 0 at the tail … exactly 1 at the head
		ang := start + u*sweep
		col := blendRGBA(theme.Accent, theme.SurfaceAlt, u)
		fillRect(p, int(cx+orbit*math.Cos(ang)), int(cy+orbit*math.Sin(ang)), dot, dot, col)
	}
}

// drawBars paints four vertical bars, centred, whose heights pulse out of phase
// so the group reads as an equalizer.
func (s *Spinner) drawBars(p painter.Painter, theme *Theme, r Rect) {
	const n = 4
	cx, cy, minSide := spinnerGeom(r)
	maxH := atLeast1(minSide * 3 / 4)
	barW := atLeast1(minSide / (2 * n))
	gap := atLeast1(barW / 2)
	total := n*barW + (n-1)*gap
	startX := int(cx) - total/2
	for i := 0; i < n; i++ {
		frac := 0.3 + 0.7*math.Abs(math.Sin((s.Phase+float64(i)/(2*n))*2*math.Pi))
		h := atLeast1(int(float64(maxH) * frac))
		fillRect(p, startX+i*(barW+gap), int(cy)-h/2, barW, h, theme.Accent)
	}
}

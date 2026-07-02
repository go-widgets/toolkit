// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

import "math"

// Spinner is an indeterminate loading indicator. When Active, Draw
// paints a "clock hand" from the centre of bounds rotated by Phase *
// 2π in Theme.Accent. Caller drives Phase via Tick(dt) so the
// animation cadence stays tied to the host's frame loop (no
// goroutine, no timer).
type Spinner struct {
	Base
	Active bool
	Phase  float64 // 0..1, full cycle
}

// NewSpinner builds a Spinner stopped at Phase=0.
func NewSpinner() *Spinner { return &Spinner{} }

// Tick advances Phase by deltaSeconds, wrapping modulo 1 so the
// value stays bounded.
func (s *Spinner) Tick(deltaSeconds float64) {
	s.Phase += deltaSeconds
	s.Phase -= math.Floor(s.Phase)
}

// Draw paints the rotating hand when Active. Hand length = 40% of
// the smaller of (W, H); painted as a thin radial line of single
// pixels from centre outward.
func (s *Spinner) Draw(p painter.Painter, theme *Theme) {
	if !s.Active {
		return
	}
	r := s.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	cx := float64(r.X) + float64(r.W)/2
	cy := float64(r.Y) + float64(r.H)/2
	radius := float64(r.W) / 2
	if float64(r.H) < float64(r.W) {
		radius = float64(r.H) / 2
	}
	radius *= 0.4
	angle := s.Phase * 2 * math.Pi
	dx := math.Cos(angle)
	dy := math.Sin(angle)
	steps := int(radius)
	if steps < 1 {
		steps = 1
	}
	for t := 0; t < steps; t++ {
		f := float64(t) / float64(steps) * radius
		px := int(cx + dx*f)
		py := int(cy + dy*f)
		fillRect(p, px, py, 1, 1, theme.Accent)
	}
}

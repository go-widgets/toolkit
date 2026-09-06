// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Density is the toolkit's touch profile — the "visually adapted for touch"
// axis, DISTINCT from HiDPI ([MetricScale]) and composed WITH it. HiDPI answers
// "how many device pixels per logical pixel" so chrome stays crisp; Density
// answers "how much breathing room does a pointer need" so the same widgets read
// comfortably under a fingertip on a phone and compactly under a mouse on a
// desktop. A phone renders at 3x device scale AND a touch density; the two
// multiply.
//
// It is a package global, mirroring [MetricScale]: a process draws one surface
// at a time on the main thread, so a single setting drives every widget without
// threading it through every call. The default is [DensityCompact], whose factor
// is exactly 1.0, so a host that never calls [SetDensity] gets metrics
// byte-for-byte identical to a toolkit that had no density system at all — the
// same guarantee [MetricScale] makes at 1.0.
//
// The level type is [DensityLevel] so the package-global read accessor can keep
// the name [Density], mirroring the [MetricScale] accessor pair.
type DensityLevel int

const (
	// DensityCompact is the desktop default: no extra spacing and no minimum
	// hit-target. Its factor is exactly 1.0, so every metric is unchanged from a
	// density-less toolkit.
	DensityCompact DensityLevel = iota
	// DensityComfortable widens spacing a little and enforces a modest minimum
	// hit-target — a middle ground for hybrids (touchscreen laptops, kiosks).
	DensityComfortable
	// DensityTouch is the phone/tablet profile: generous spacing and a 44
	// logical-pixel minimum hit-target (the Apple HIG / Material touch floor).
	DensityTouch
)

// density is the active touch profile. DensityCompact is the zero value, so the
// default costs nothing and matches the historical (density-less) behaviour.
var density = DensityCompact

// SetDensity sets the global touch profile. An out-of-range value is ignored, so
// the density never lands on an undefined level (mirroring how [SetMetricScale]
// ignores a non-positive scale).
func SetDensity(d DensityLevel) {
	if d < DensityCompact || d > DensityTouch {
		return
	}
	appearanceMu.Lock()
	density = d
	appearanceMu.Unlock()
}

// Density returns the current global touch profile ([DensityCompact] by
// default).
func Density() DensityLevel {
	appearanceMu.RLock()
	defer appearanceMu.RUnlock()
	return density
}

// densityFactor is the spacing multiplier a level applies to every base metric
// on top of [MetricScale]. Compact is exactly 1.0 (byte-identical default);
// comfortable and touch grow the whole chrome uniformly so one flip re-sizes the
// toolkit without any widget changing its own code.
func densityFactor(d DensityLevel) float64 {
	switch d {
	case DensityComfortable:
		return 1.25
	case DensityTouch:
		return 1.5
	default: // DensityCompact and any unreachable value: no growth.
		return 1.0
	}
}

// minHitLogical is the minimum hit-target for a level in LOGICAL pixels, before
// [MetricScale]. Compact imposes none (0); comfortable sits between; touch is the
// 44-logical-pixel finger floor. It is deliberately independent of
// [densityFactor]: the floor is an absolute reachability guarantee, not a scaled
// spacing, so it grows only with device pixels.
func minHitLogical(d DensityLevel) int {
	switch d {
	case DensityComfortable:
		return 36
	case DensityTouch:
		return 44
	default: // DensityCompact and any unreachable value: no floor.
		return 0
	}
}

// MinHitTarget returns the current minimum hit dimension in DEVICE pixels: the
// density's logical floor scaled by [MetricScale] only (NOT by [densityFactor]).
// It is 0 under [DensityCompact], so nothing is clamped on the desktop. Under
// [DensityTouch] it is 44 logical pixels — 44 at MetricScale 1, 88 at MetricScale
// 2 — so a fingertip always lands a large-enough target regardless of panel DPI.
func MinHitTarget() int { return dpiScaled(minHitLogical(density)) }

// TouchTarget clamps an interactive control's computed hit dimension (already in
// device pixels) UP to [MinHitTarget], leaving anything already large enough
// untouched. Under [DensityCompact] the minimum is 0, so it is a pass-through and
// the control's hit rect is byte-identical to its drawn bounds. A widget uses it
// on each axis of its clickable region, e.g.:
//
//	hitW := TouchTarget(r.W)
//	hitH := TouchTarget(r.H)
//
// then centres the (possibly larger) hit rect over its visual bounds — see
// [Switch.HitRect] for a worked example. Clamping the HIT area, not the drawn
// pixels, keeps a small control visually unchanged while giving a finger the
// reach the platform requires.
func TouchTarget(computed int) int {
	if m := MinHitTarget(); computed < m {
		return m
	}
	return computed
}

// HitRect is the worked example of [TouchTarget]: the switch's interactive
// rectangle, clamped up to the density minimum on each axis and centred over the
// drawn [Widget.Bounds]. Under the default [DensityCompact] the minimum is 0, so
// the returned rect equals Bounds byte-for-byte; under [DensityTouch] a small
// switch grows a finger-sized hit area around its unchanged pixels. Only the hit
// region grows — Draw is untouched — so the switch looks the same and is easier
// to press.
func (s *Switch) HitRect() Rect {
	r := s.Bounds()
	w, h := TouchTarget(r.W), TouchTarget(r.H)
	return Rect{
		X: r.X - (w-r.W)/2,
		Y: r.Y - (h-r.H)/2,
		W: w,
		H: h,
	}
}

// touchHitRect is the generic form of the [Switch.HitRect] worked example and
// the single seam every widget's HitRect routes through: it clamps a drawn
// rectangle UP to the current density hit-target on each axis (via
// [TouchTarget]) and centres the (possibly larger) hit rect over the unchanged
// input. Under the default [DensityCompact] the minimum is 0, so TouchTarget is
// a pass-through and the returned rect equals r byte-for-byte; under
// [DensityTouch] a small control grows a finger-sized hit area around its
// unchanged pixels. Sharing this one helper across the CONTROLS/CHROME and
// INPUTS/PICKERS families keeps the clamp+centre rule from ever drifting between
// widgets. Only the hit region grows — Draw is never involved.
func touchHitRect(r Rect) Rect {
	w, h := TouchTarget(r.W), TouchTarget(r.H)
	return Rect{X: r.X - (w-r.W)/2, Y: r.Y - (h-r.H)/2, W: w, H: h}
}

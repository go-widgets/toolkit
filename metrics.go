// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Global metric scale for HiDPI.
//
// Every widget's PIXEL metric — padding, gap, radius, track width, slot size,
// stroke — is a base value in LOGICAL pixels routed through [scaled]. A host that
// renders into a HiDPI framebuffer (a Retina window drawing at the panel's real
// resolution) calls [SetMetricScale] once with its device scale, and the whole
// toolkit lays out and paints crisp at that resolution instead of at a fixed
// logical size. Fonts already carry their own Scale (see [NewBitmapFont]); this
// covers the box metrics around the text so chrome and type scale together.
//
// The scale is a package global, mirroring the package-level active font
// (SetFont / CurrentFont): a process draws one surface at a time on the main
// thread, so one setting drives every widget without threading it through every
// call. 1.0 (the default) is logical pixels — the historical behaviour, so a host
// that never calls SetMetricScale is byte-for-byte unchanged.
var metricScale = 1.0

// SetMetricScale sets the global metric scale. A non-positive value is ignored,
// so the scale never collapses metrics to zero.
func SetMetricScale(f float64) {
	if f <= 0 {
		return
	}
	appearanceMu.Lock()
	changed := f != metricScale
	if changed {
		metricScale = f
	}
	appearanceMu.Unlock()
	if !changed {
		return
	}
	// The text follows. Every other metric here is multiplied by the scale and
	// the one thing a person actually reads was not, so an application on a
	// HiDPI display got labels at half the size it asked for and had to
	// re-render the face itself -- which is per-application work for a
	// toolkit-wide fact.
	rescaleText()
}

// MetricScale returns the current global metric scale (1.0 by default).
func MetricScale() float64 {
	appearanceMu.RLock()
	defer appearanceMu.RUnlock()
	return metricScale
}

// scaled rounds a base (logical-pixel) metric to device pixels at the current
// HiDPI scale AND touch density. It is the single seam every widget's pixel
// constant passes through, so both HiDPI ([MetricScale]) and density
// ([Density]) re-size the whole toolkit from one place, without any widget
// changing its own code. The two compose multiplicatively:
//
//	device = round(base × metricScale × densityFactor(density))
//
// Under the default [DensityCompact] the factor is exactly 1.0, so this reduces
// to the pure HiDPI form and every metric is byte-identical to a density-less
// toolkit.
func scaled(v int) int {
	appearanceMu.RLock()
	s, d := metricScale, density
	appearanceMu.RUnlock()
	return int(float64(v)*s*densityFactor(d) + 0.5)
}

// dpiScaled rounds a base (logical-pixel) length to device pixels at the current
// HiDPI [MetricScale] ONLY — it does NOT apply the touch [Density] factor. It
// backs [MinHitTarget], whose floor is an absolute reachability guarantee in
// logical pixels that must grow with panel DPI but not with spacing density.
func dpiScaled(v int) int {
	appearanceMu.RLock()
	s := metricScale
	appearanceMu.RUnlock()
	return int(float64(v)*s + 0.5)
}

// Scaled is the exported form of [scaled]: it rounds a base (logical-pixel)
// metric to device pixels at the current [MetricScale]. Sibling packages that
// compose these widgets (e.g. the virtual list feed) route their own fixed
// metrics through it so they scale in lockstep with the core widgets instead of
// re-deriving the rounding.
func Scaled(v int) int { return scaled(v) }

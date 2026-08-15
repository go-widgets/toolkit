// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// Does a widget actually honour [SetMetricScale]?
//
// The knob exists and is documented as the thing a HiDPI host sets once, so
// that "the whole toolkit lays out and paints crisp at that resolution". Ten
// files out of a hundred and sixty route their metrics through it. The rest
// carry raw pixel constants -- a 6-pixel radius, a 1-pixel border, an 8-pixel
// pad -- which stay 6, 1 and 8 on a screen with twice the pixels, so the chrome
// shrinks to half its intended size while the window around it is sharp.
//
// This finds them, rather than my reading a hundred files and guessing which
// numbers are pixels.
//
// The instrument is the RUN-LENGTH PROFILE of the middle row and middle column.
// Draw the widget at scale 1 in a w×h box and at scale 2 in a 2w×2h box, then
// walk the middle scanline of each and record the runs of equal colour. A widget
// whose metrics all scale gives the SAME NUMBER of runs with each one twice as
// wide; one with an unscaled constant gives a run that did not double. Neither
// the painted area nor the bounding box can tell the two apart -- both are
// driven by the bounds the caller supplies, which double either way -- and that
// is why this measures the interior and not the outline.
//
// It measures CHROME, not text. The catalogue gives widgets no labels on
// purpose: type carries its own scale (SetFont / NewBitmapFont), a centred
// baseline rounds -- (H-glyphH)/2 is 12 at one scale and 25 at twice it, not 24
// -- and a scanline that lands one glyph row off reports a difference that is
// arithmetic rather than a defect. Whether text should follow the metric scale
// is a real question, and a separate one.

// runLengths walks one row (or column) of an RGBA buffer and returns the length
// of each run of identical pixels.
func runLengths(buf []byte, w, h int, index int, column bool) []int {
	n := w
	at := func(i int) RGBA { return pixelAt(buf, w, i, index) }
	if column {
		n = h
		at = func(i int) RGBA { return pixelAt(buf, w, index, i) }
	}
	var runs []int
	cur := 1
	prev := at(0)
	for i := 1; i < n; i++ {
		c := at(i)
		if c == prev {
			cur++
			continue
		}
		runs = append(runs, cur)
		cur, prev = 1, c
	}
	return append(runs, cur)
}

// withoutSlivers drops runs no wider than one LOGICAL pixel at the scale they
// were measured at.
//
// Such a run is an anti-aliasing sliver, not a metric: the boundary of a filled
// round-rect is one pixel of partial coverage at any size, and at twice the
// scale it may resolve into two steps where it had one. Comparing those reports
// arithmetic as a defect.
//
// In logical units, because a border that DOES scale is one pixel at 1x and two
// at 2x -- dropping "runs of one pixel" would keep the second and not the first,
// and then every framed widget looks broken for having been fixed.
//
// It does not hide a border that failed to scale, which is the defect this whole
// audit exists for: a border that stayed one pixel leaves the interior run two
// pixels too wide, and the interior is nobody's sliver.
func withoutSlivers(runs []int, scale int) []int {
	out := make([]int, 0, len(runs))
	for _, r := range runs {
		if r > scale {
			out = append(out, r)
		}
	}
	return out
}

// scaleReport is what one widget's audit produced.
type scaleReport struct {
	name string
	// mismatch is the first run that did not double, empty when all of them did.
	mismatch string
}

// auditScaling draws build() at scale 1 and at scale 2 and reports a run that
// did not double.
//
// WIDTHS, not positions. A boundary's position moves by a single pixel when a
// one-pixel border fails to scale, which is indistinguishable from the rounding
// of an odd metric -- I tried that instrument first and it declared the very
// defect this was written to find to be within tolerance. A run's WIDTH is not
// ambiguous: a border that stayed one pixel wide while everything around it
// doubled is a border that did not scale.
//
// Three lines per axis, and a widget is reported only when all three disagree.
// A metric that does not scale is wrong on EVERY line -- the border, the pad,
// the box are there whatever row you look along. A diagonal feature crossing an
// exact scanline is not: a checkmark's second stroke may meet row h/2 at one
// scale and row h/2+1 at twice it, purely from integer rounding, and calling
// that a defect would have me contorting a widget to satisfy a probe.
//
// Tolerance is one pixel, for the odd metric that rounds.
func auditScaling(t *testing.T, name string, w, h int, build func() Widget) scaleReport {
	t.Helper()
	draw := func(scale int) ([]byte, int, int) {
		defer SetMetricScale(1)
		defer SetFont(NewBitmapFont(1))
		SetMetricScale(float64(scale))
		SetFont(NewBitmapFont(scale))
		bw, bh := w*scale, h*scale
		buf := makeSurface(bw, bh)
		wd := build()
		wd.SetBounds(Rect{X: 0, Y: 0, W: bw, H: bh})
		wd.Draw(newP(buf, bw), DefaultDark())
		return buf, bw, bh
	}
	one, w1, h1 := draw(1)
	two, w2, h2 := draw(2)

	for _, axis := range []struct {
		name   string
		column bool
	}{{"row", false}, {"column", true}} {
		var failures []string
		for _, frac := range []float64{1.0 / 3, 1.0 / 2, 2.0 / 3} {
			var r1, r2 []int
			if axis.column {
				r1 = runLengths(one, w1, h1, int(float64(w1)*frac), true)
				r2 = runLengths(two, w2, h2, int(float64(w2)*frac), true)
			} else {
				r1 = runLengths(one, w1, h1, int(float64(h1)*frac), false)
				r2 = runLengths(two, w2, h2, int(float64(h2)*frac), false)
			}
			r1, r2 = withoutSlivers(r1, 1), withoutSlivers(r2, 2)
			if len(r1) != len(r2) {
				failures = append(failures, fmt.Sprintf("at %.2f: %d runs at 1x and %d at 2x (%v vs %v)",
					frac, len(r1), len(r2), r1, r2))
				continue
			}
			for i := range r1 {
				want := 2 * r1[i]
				if diff := r2[i] - want; diff < -1 || diff > 1 {
					failures = append(failures, fmt.Sprintf("at %.2f: run %d is %d at 1x and %d at 2x, want ~%d",
						frac, i, r1[i], r2[i], want))
					break
				}
			}
		}
		if len(failures) == 3 {
			return scaleReport{name, axis.name + " " + failures[1]}
		}
	}
	return scaleReport{name: name}
}

// TestMetricScaleAudit is the inventory: it audits the widget catalogue and
// FAILS with the list of widgets whose interior does not scale.
//
// It is written to be read, not merely to be green. Every name it prints is a
// widget that will be half-size on a HiDPI screen, and the list shrinks as they
// are fixed.
func TestMetricScaleAudit(t *testing.T) {
	type entry struct {
		name  string
		w, h  int
		build func() Widget
	}
	catalogue := []entry{
		{"Button", 120, 32, func() Widget { return &Button{} }},
		{"Button/flat", 120, 32, func() Widget { return &Button{Flat: true} }},
		{"CheckButton", 120, 24, func() Widget { return &CheckButton{Checked: true} }},
		{"Switch", 60, 24, func() Widget { return &Switch{On: true} }},
		{"Card", 160, 120, func() Widget { return &Card{} }},
		{"ProgressBar", 120, 16, func() Widget { return &ProgressBar{Fraction: 0.5} }},
		{"Chip", 80, 24, func() Widget { return &Chip{} }},
		{"Kbd", 40, 20, func() Widget { return &Kbd{} }},
		{"Avatar", 40, 40, func() Widget { return &Avatar{} }},
		{"Entry", 160, 28, func() Widget { return &Entry{} }},
		{"IconButton", 32, 32, func() Widget { return &IconButton{} }},
		{"Alert", 200, 60, func() Widget { return &Alert{} }},
		{"Banner", 200, 40, func() Widget { return &Banner{} }},
		{"Frame", 160, 100, func() Widget { return &Frame{} }},
		{"Gauge", 80, 80, func() Widget { return &Gauge{} }},
		{"Switch/off", 60, 24, func() Widget { return &Switch{} }},
		{"ToggleButton", 100, 30, func() Widget { return &ToggleButton{} }},
		{"Expander", 160, 30, func() Widget { return &Expander{} }},
	}

	var broken []scaleReport
	for _, e := range catalogue {
		rep := auditScaling(t, e.name, e.w, e.h, e.build)
		if rep.mismatch != "" {
			broken = append(broken, rep)
		}
	}
	sort.Slice(broken, func(i, j int) bool { return broken[i].name < broken[j].name })
	if len(broken) == 0 {
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d widgets do not honour SetMetricScale:\n", len(broken), len(catalogue))
	for _, r := range broken {
		fmt.Fprintf(&b, "  %-16s %s\n", r.name, r.mismatch)
	}
	t.Error(b.String())
}

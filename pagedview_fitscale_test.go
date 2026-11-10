// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"math"
	"testing"
)

// ⛔ THE FIT ZOOM IS A PROPERTY OF THE WINDOW, NOT OF THE PANEL.
//
// The pane is measured in device pixels and a page's natural size is not, so
// usable*100/natural is a DEVICE percent: one step of it is 1/MetricScale of a
// logical percent. Truncating that lands on a different logical zoom per ratio.
//
// Measured in one headless Chrome at the same CSS viewport
// (go-tex/go-tex.github.io#191): 30 at ratio 1 and 61 at ratio 2 — logical 30%
// against 30.5% — so a reader on a Retina panel saw the page 1.7% larger than a
// reader on a 1× panel with the same window.
//
// The assertion is on the LOGICAL zoom, because that is what a reader sees.
func TestFitZoomIsTheSameLogicalSizeAtEveryScale(t *testing.T) {
	// Pages WIDER than the pane, so the fit lands near 30% — the shape of the
	// reported case, and clear of the [25, 400] clamp at every ratio below.
	// (That clamp is in DEVICE percent and so is itself ratio-dependent: a
	// Retina reader can zoom out only half as far and in only half as far. Same
	// confusion, different symptom; not this change's to fix.)
	//
	// ⛔ SEVERAL WIDTHS, because one is luck. The first version used 1218
	// alone, whose ratio against this pane divides almost exactly — so
	// doubling it never crossed an integer and the old form passed. A width
	// whose fraction sits near a half is where truncation bites, and which
	// width that is depends on the gutter and the margins. Sweeping removes the
	// need to know.
	const paneW = 400
	// Kept inside (25, 400) at every ratio: 1409 against this pane fits at
	// exactly 25, the clamp floor, and the guard below caught it measuring the
	// clamp instead of the fit.
	widths := []int{1100, 1150, 1187, 1218, 1219, 1223, 1237, 1259, 1301}

	logicalAt := func(pageW int, scale float64) float64 {
		t.Helper()
		SetMetricScale(scale)
		defer SetMetricScale(1)
		pv := NewPagedView([]*image.RGBA{solidPage(pageW, 400, pgRed)})
		// The pane is given in device pixels, as a host at this ratio would.
		pv.SetBounds(Rect{X: 0, Y: 0, W: int(float64(paneW) * scale), H: int(300 * scale)})
		pv.fitWidth()
		z := pv.Zoom().Get()
		if z <= pagedZoomMin || z >= pagedZoomMax {
			t.Fatalf("page %d, scale %.2f: zoom %d hit the clamp, so this measures the clamp and not the fit", pageW, scale, z)
		}
		return float64(z) / scale
	}
	// ⛔ EXACT, and integer scales, and that combination is the whole test.
	//
	// A first version of this allowed ±1 logical percent across ratios 1.25 to
	// 3 — and PASSED against the defect it was written for. Dividing Zoom by
	// the scale cancels precisely what was wrong: the old form's device percent
	// is proportional to the ratio, so the logical value it yields differs only
	// by the truncation residue — at most 1/scale of a percent, which is INSIDE
	// a ±1 bound. The reported case was 30 against 30.5: half a percent, and
	// 1.7% of the page.
	//
	// So the bound has to be the quantum itself, not a percent. At an integer
	// scale the fix makes the logical zoom exactly equal; that is the case the
	// defect was measured in and the one most panels have. Fractional scales
	// cannot be exact — Zoom is an int — and are bounded separately below.
	for _, pageW := range widths {
		base := logicalAt(pageW, 1)
		if base <= 0 {
			t.Fatalf("page %d: fit at ratio 1 gave %v", pageW, base)
		}
		for _, scale := range []float64{2, 3, 4} {
			if got := logicalAt(pageW, scale); got != base {
				t.Errorf("page %d, scale %.0f: logical zoom %.4f%%, ratio 1 gives %.4f%% — a reader on this panel sees the page %.2f%% %s",
					pageW, scale, got, base, math.Abs(got-base)/base*100,
					map[bool]string{true: "larger", false: "smaller"}[got > base])
			}
		}
	}
}

// A fractional scale cannot be exact — a whole logical percent times 1.25 is
// not always a whole device percent, and Zoom is an int — so the claim there is
// that the residue is bounded by the quantum the ratio can express, and is not
// the systematic half-percent the old form produced.
func TestFitZoomAtFractionalScalesIsWithinItsOwnQuantum(t *testing.T) {
	const pageW, paneW = 1218, 400
	defer SetMetricScale(1)

	logicalAt := func(scale float64) float64 {
		SetMetricScale(scale)
		pv := NewPagedView([]*image.RGBA{solidPage(pageW, 400, pgRed)})
		pv.SetBounds(Rect{X: 0, Y: 0, W: int(float64(paneW) * scale), H: int(300 * scale)})
		pv.fitWidth()
		return float64(pv.Zoom().Get()) / scale
	}
	base := logicalAt(1)
	for _, scale := range []float64{1.25, 1.5, 1.75} {
		got := logicalAt(scale)
		if diff := math.Abs(got - base); diff > 1/scale {
			t.Errorf("scale %.2f: logical zoom %.4f%% against %.4f%% at ratio 1, off by %.4f — more than the %.4f this ratio can express",
				scale, got, base, diff, 1/scale)
		}
	}
}

// At an INTEGER scale the quantum is exactly a whole logical percent, which is
// the case the defect was measured in (ratio 2) and the one most panels have.
func TestFitPercentIsAWholeLogicalPercentAtIntegerScales(t *testing.T) {
	defer SetMetricScale(1)
	for _, scale := range []float64{2, 3, 4} {
		SetMetricScale(scale)
		for _, c := range [][2]int{{1000, 329}, {400, 1218}, {1234, 567}} {
			usable := int(float64(c[0]) * scale)
			q := fitPercent(usable, c[1])
			if q%int(scale) != 0 {
				t.Errorf("scale %.0f: fitPercent(%d, %d) = %d, not a whole logical percent", scale, usable, c[1], q)
			}
		}
	}
}

// ⛔ AT SCALE 1 NOTHING MOVES. That is what makes this a fix rather than a
// change: the ratio that was already right must be byte-identical to the plain
// integer division it replaces, or every consumer on a 1× panel gets a silent
// re-layout out of a bug fix they did not ask for.
func TestFitPercentIsUnchangedAtScaleOne(t *testing.T) {
	SetMetricScale(1)
	for _, c := range [][2]int{
		{1000, 329}, {400, 100}, {37, 1000}, {1, 1}, {999, 1000}, {100000, 7},
	} {
		usable, natural := c[0], c[1]
		want := usable * 100 / natural
		if want < 1 {
			want = 1
		}
		if got := fitPercent(usable, natural); got != want {
			t.Errorf("fitPercent(%d, %d) = %d, want %d (the old integer division)", usable, natural, got, want)
		}
	}
}

// The guards: a page with no width, and a scale a host has not set yet.
func TestFitPercentGuards(t *testing.T) {
	SetMetricScale(1)
	if got := fitPercent(100, 0); got != 1 {
		t.Errorf("a zero-width page gave %d, want the 1%% floor", got)
	}
	// A pane far narrower than the page still yields a usable zoom rather than
	// 0, which would make the page vanish.
	if got := fitPercent(1, 100000); got != 1 {
		t.Errorf("a pane narrower than the page gave %d, want the 1%% floor", got)
	}
}

// Copyright (c) 2026, the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package toolkit

import (
	"sync"
	"testing"
)

// TestAppearanceSurvivesConcurrentUse covers the toolkit's global appearance
// being set on one goroutine while widgets read it on another.
//
// That is not a contrived pairing: it is a window moving between displays. The
// scale is set from whatever thread the display-change event arrives on, and
// the UI is laying out at the old one. Unguarded it is a data race over four
// globals at once — the scale, the density, the active font and the OpenType
// size — and what a race costs here is not a crash but silence: chrome laid out
// at one scale around type measured at another.
//
// Run with -race, this is the whole test. Without it, it still exercises that
// no ordering deadlocks: SetMetricScale re-renders the font through SetFont,
// which takes the same lock.
func TestAppearanceSurvivesConcurrentUse(t *testing.T) {
	scale, dens, font := MetricScale(), Density(), CurrentFont()
	t.Cleanup(func() {
		SetFont(nil)
		SetMetricScale(scale)
		SetDensity(dens)
		if font != nil {
			SetFont(font)
		}
	})

	const rounds = 200
	var wg sync.WaitGroup
	writers := []func(int){
		func(i int) { SetMetricScale(1 + float64(i%3)) },
		func(i int) { SetDensity(DensityLevel(i % 3)) },
		func(int) { SetFont(nil) },
	}
	readers := []func(){
		func() { _ = scaled(10) },
		func() { _ = dpiScaled(10) },
		func() { _ = MetricScale() },
		func() { _ = Density() },
		func() { _ = CurrentFont() },
		func() { _ = GlyphHeight() },
	}
	for _, w := range writers {
		wg.Add(1)
		go func(w func(int)) {
			defer wg.Done()
			for i := range rounds {
				w(i)
			}
		}(w)
	}
	for _, r := range readers {
		wg.Add(1)
		go func(r func()) {
			defer wg.Done()
			for range rounds {
				r()
			}
		}(r)
	}
	wg.Wait()

	// The state is still coherent: whatever the last writer left, a reader
	// gets a usable answer rather than a torn one.
	if got := scaled(10); got <= 0 {
		t.Errorf("scaled(10) = %d after concurrent use, want a positive metric", got)
	}
	if CurrentFont() == nil {
		t.Error("there is no active font after concurrent use")
	}
}

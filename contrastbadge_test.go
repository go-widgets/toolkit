// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestContrastGradeThresholds(t *testing.T) {
	cases := []struct {
		ratio float64
		large bool
		want  ContrastLevel
	}{
		{4.49, false, ContrastFail}, {4.5, false, ContrastAA}, {6.99, false, ContrastAA}, {7, false, ContrastAAA},
		{2.99, true, ContrastFail}, {3, true, ContrastAA}, {4.5, true, ContrastAAA},
	}
	for _, c := range cases {
		if got := ContrastGrade(c.ratio, c.large); got != c.want {
			t.Errorf("ContrastGrade(%v, large=%v) = %v, want %v", c.ratio, c.large, got, c.want)
		}
	}
	if ContrastFail.Passes() || !ContrastAA.Passes() || !ContrastAAA.Passes() {
		t.Error("Passes must be true from AA up")
	}
	if ContrastFail.String() != "Fail" || ContrastAA.String() != "AA" || ContrastAAA.String() != "AAA" {
		t.Error("String")
	}
}

func TestContrastBadgeFollowsItsObservables(t *testing.T) {
	b := NewContrastBadge(RGB(0, 0, 0), RGB(255, 255, 255))
	if got := b.Text(); got != "21.00:1 AAA" {
		t.Fatalf("black on white = %q", got)
	}
	b.Foreground().Set(RGB(0x77, 0x77, 0x77)) // #777 on white is 4.48:1, the classic near miss
	if b.Level() != ContrastFail {
		t.Fatalf("#777 on white = %v (%.3f), want Fail", b.Level(), b.Ratio())
	}
	b.FailText = "Échec"
	if got := b.Text(); got != "4.48:1 Échec" {
		t.Fatalf("localised failure = %q", got)
	}
	b.Large().Set(true)
	if b.Level() != ContrastAA {
		t.Fatalf("large text at %.2f must pass AA", b.Ratio())
	}
	b.Draw(newP(makeSurface(120, 24), 120), DefaultLight())
	if b.Bounds().W == 0 {
		t.Error("a zero-width badge must size itself to its text")
	}
	if b.A11y().Name != b.Text() {
		t.Error("the screen reader hears the verdict")
	}
	(&ContrastBadge{}).Draw(newP(makeSurface(120, 24), 120), DefaultLight()) // a bare badge does not panic
}

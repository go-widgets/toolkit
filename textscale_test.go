// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// Type follows the one HiDPI knob, unless the host chose a font itself.
//
// The metric audit deliberately measures chrome and not text, and that left a
// gap it could not see: a host that turned SetMetricScale got padding, borders
// and controls at twice the size around type that stayed exactly as it was.
// That is not a partial improvement, it is a worse interface than the one it had
// before it asked.
func TestBuiltInFontFollowsTheMetricScale(t *testing.T) {
	defer func() { SetFont(nil); SetMetricScale(1) }()

	SetFont(nil)
	SetMetricScale(1)
	h1, a1 := GlyphHeight(), GlyphAdvance()

	SetMetricScale(2)
	h2, a2 := GlyphHeight(), GlyphAdvance()
	if h2 != 2*h1 || a2 != 2*a1 {
		t.Errorf("at twice the scale the built-in glyph is %dx%d against %dx%d at one: "+
			"the chrome doubled and the type did not", a2, h2, a1, h1)
	}
	if got, want := TextWidth("abc"), 3*a2; got != want {
		t.Errorf("TextWidth at scale 2 = %d, want %d", got, want)
	}

	// A fractional scale rounds to a whole bitmap multiple: the built-in font is
	// a block rasteriser and half a block is not a glyph.
	SetMetricScale(1.4)
	if got := GlyphHeight(); got != h1 {
		t.Errorf("at scale 1.4 the glyph height is %d, want the unscaled %d", got, h1)
	}
	SetMetricScale(1.6)
	if got := GlyphHeight(); got != 2*h1 {
		t.Errorf("at scale 1.6 the glyph height is %d, want the doubled %d", got, 2*h1)
	}
}

// A host that chose a font chose its size too.
func TestAChosenFontIsLeftAlone(t *testing.T) {
	defer func() { SetFont(nil); SetMetricScale(1) }()

	SetFont(NewBitmapFont(3))
	SetMetricScale(2)
	if got, want := GlyphHeight(), 3*baseGlyphHeight; got != want {
		t.Errorf("a font the host set to 3 reports height %d under a metric scale of 2, want %d "+
			"-- it must not be scaled twice", got, want)
	}

	// Handing back nil returns to the built-in, which does follow the scale.
	SetFont(nil)
	if got, want := GlyphHeight(), 2*baseGlyphHeight; got != want {
		t.Errorf("after SetFont(nil) the height is %d, want the built-in at scale 2 (%d)", got, want)
	}
}

// The scaled built-in is cached: text measurement runs per glyph per frame, and
// allocating a font each time would be a new object sixty times a second.
func TestScaledDefaultFontIsCached(t *testing.T) {
	defer SetMetricScale(1)
	SetMetricScale(2)
	if a, b := CurrentFont(), CurrentFont(); a != b {
		t.Error("two reads of the scaled built-in font returned different objects")
	}
	SetMetricScale(1)
	if got := CurrentFont(); got != Font(defaultFont) {
		t.Error("at scale 1 the built-in is not the plain unscaled font")
	}

	// A scale of zero or less is not a scale, and a font of no pixels is not a
	// font: the toolkit ignores such a value, and this asserts what the font
	// side does with whatever it holds.
	metricScale = 0 // set directly: SetMetricScale rejects it, as it should
	if got := CurrentFont(); got != Font(defaultFont) {
		t.Error("with a nonsense scale the built-in font is not the plain unscaled one")
	}
	metricScale = 1
}

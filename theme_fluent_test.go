// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestFluentLight(t *testing.T) {
	assertPalette(t, "FluentLight", FluentLight(),
		op(0xF3, 0xF3, 0xF3), // Background
		op(0xFF, 0xFF, 0xFF), // Surface
		op(0xEB, 0xEB, 0xEB), // SurfaceAlt
		op(0x20, 0x20, 0x20), // OnBackground
		op(0x20, 0x20, 0x20), // OnSurface
		op(0x00, 0x67, 0xC0), // Accent
		op(0xDF, 0xDF, 0xDF), // Border
		op(0xFF, 0xFF, 0xFF), // OnAccent
	)
}

func TestFluentDark(t *testing.T) {
	assertPalette(t, "FluentDark", FluentDark(),
		op(0x20, 0x20, 0x20), // Background
		op(0x2B, 0x2B, 0x2B), // Surface
		op(0x27, 0x27, 0x27), // SurfaceAlt
		op(0xFF, 0xFF, 0xFF), // OnBackground
		op(0xF0, 0xF0, 0xF0), // OnSurface
		op(0x4C, 0xC2, 0xFF), // Accent
		op(0x1D, 0x1D, 0x1D), // Border
		op(0x00, 0x00, 0x00), // OnAccent (bright cyan accent -> black ink)
	)
}

// TestFluentDarkOnAccentIsBlack pins the one built-in whose on-accent ink is
// black rather than white: the dark cyan accent is light enough to need it.
func TestFluentDarkOnAccentIsBlack(t *testing.T) {
	if FluentDark().Extra["OnAccent"] != op(0, 0, 0) {
		t.Errorf("FluentDark OnAccent must be black, got %+v", FluentDark().Extra["OnAccent"])
	}
	if FluentLight().Extra["OnAccent"] != op(0xFF, 0xFF, 0xFF) {
		t.Errorf("FluentLight OnAccent must be white, got %+v", FluentLight().Extra["OnAccent"])
	}
}

// TestFluentLightDarkDiffer guards against a palette copy/paste regression.
func TestFluentLightDarkDiffer(t *testing.T) {
	l, d := FluentLight(), FluentDark()
	if l.Background == d.Background {
		t.Error("Fluent light/dark Background must differ")
	}
	if l.Accent == d.Accent {
		t.Error("Fluent light/dark Accent must differ")
	}
}

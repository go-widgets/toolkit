// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestAlertConstructor covers NewAlert's field wiring — regression
// guard for accidental struct-field reorderings.
func TestAlertConstructor(t *testing.T) {
	a := NewAlert("hi", AlertWarning)
	if a.Text != "hi" || a.Kind != AlertWarning {
		t.Fatalf("NewAlert: got (%q, %d), want (%q, %d)", a.Text, a.Kind, "hi", AlertWarning)
	}
}

// TestAlertDrawEachKind walks every AlertKind constant, drawing each
// into its own surface, and confirms the border corner + text ink
// pixels land as expected. Covers the Info / Success / Warning / Error
// switch arms in alertFace.
func TestAlertDrawEachKind(t *testing.T) {
	theme := DefaultLight()
	cases := []struct {
		name string
		kind AlertKind
	}{
		{"info", AlertInfo},
		{"success", AlertSuccess},
		{"warning", AlertWarning},
		{"error", AlertError},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			a := NewAlert("Msg", c.kind)
			a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 30})
			surf := makeSurface(120, 40)
			a.Draw(newP(surf, 120), theme)
			// Corner pixel = Border stroke.
			if got := pixelAt(surf, 120, 0, 0); got != theme.Border {
				t.Fatalf("%s: corner = %+v, want Border", c.name, got)
			}
			// Interior body pixel above the text row = the Kind's face colour.
			wantFace := alertFace(c.kind, theme)
			if got := pixelAt(surf, 120, 50, 3); got != wantFace {
				t.Fatalf("%s: body at (50,3) = %+v, want %+v", c.name, got, wantFace)
			}
		})
	}
}

// TestAlertInfoUsesThemeAccent locks in the Info=Accent behaviour: the
// info-kind body pixel must match the theme's Accent so an app that
// swaps its accent palette gets a matching info banner for free.
func TestAlertInfoUsesThemeAccent(t *testing.T) {
	theme := DefaultLight()
	a := NewAlert("hi", AlertInfo)
	a.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 30})
	surf := makeSurface(120, 40)
	a.Draw(newP(surf, 120), theme)
	if got := pixelAt(surf, 120, 50, 5); got != theme.Accent {
		t.Fatalf("info face = %+v, want theme.Accent %+v", got, theme.Accent)
	}
}

// TestAlertFaceKindsAreDistinct proves the four Kind constants map to
// four distinct colours — otherwise the whole semantic-colour promise
// collapses. Covers alertFace as a pure function alongside the Draw
// exercise above.
func TestAlertFaceKindsAreDistinct(t *testing.T) {
	theme := DefaultLight()
	faces := []RGBA{
		alertFace(AlertInfo, theme),
		alertFace(AlertSuccess, theme),
		alertFace(AlertWarning, theme),
		alertFace(AlertError, theme),
	}
	for i := 0; i < len(faces); i++ {
		for j := i + 1; j < len(faces); j++ {
			if faces[i] == faces[j] {
				t.Fatalf("kinds %d and %d share colour %+v", i, j, faces[i])
			}
		}
	}
}

// TestAlertFaceDefaultBranch covers the switch's default arm: an
// out-of-range AlertKind falls back to Info (theme.Accent). Otherwise
// the default-case line stays uncovered.
func TestAlertFaceDefaultBranch(t *testing.T) {
	theme := DefaultLight()
	got := alertFace(AlertKind(999), theme)
	if got != theme.Accent {
		t.Fatalf("default-arm face = %+v, want theme.Accent %+v", got, theme.Accent)
	}
}

// TestAlertDrawText verifies the message text lands in Theme.Background
// ink at the pad offset. "T" column 0 is bit 0 (row 0), so at
// (r.X+AlertPadX, r.Y+AlertPadY) = (12, 8) we expect an inked pixel.
func TestAlertDrawText(t *testing.T) {
	theme := DefaultLight()
	a := NewAlert("T", AlertInfo)
	a.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 24})
	surf := makeSurface(100, 30)
	a.Draw(newP(surf, 100), theme)
	if got := pixelAt(surf, 100, AlertPadX, AlertPadY); got != theme.Background {
		t.Fatalf("alert text pixel = %+v, want Background %+v", got, theme.Background)
	}
}

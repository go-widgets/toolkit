// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// A pressed ToggleButton's label sits on the accent face: it is drawn in the
// accent ink, not OnSurface, which is dark on dark in a light theme.
func TestPressedToggleLabelContrastsWithTheAccentFace(t *testing.T) {
	for name, th := range popupThemes() {
		tb := NewToggleButton("Minimap", true)
		tb.SetBounds(Rect{W: 90, H: 28})
		buf := makeSurface(90, 28)
		tb.Draw(newP(buf, 90), th)
		face := pixelAt(buf, 90, 3, 3)
		if face != th.Accent {
			t.Fatalf("%s: pressed face = %v, want Accent", name, face)
		}
		ink := accentInk(th)
		found := false
		for y := 0; y < 28 && !found; y++ {
			for x := 0; x < 90; x++ {
				if pixelAt(buf, 90, x, y) == ink {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("%s: the label was not drawn in the accent ink %v", name, ink)
		}
		if r := contrastRatio(ink, th.Accent); r < 3 {
			t.Errorf("%s: label vs accent face = %.2f:1, want >= 3:1", name, r)
		}
	}
}

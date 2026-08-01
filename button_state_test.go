// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestButtonSelectedAndDanger covers the sticky Selected (accent fill) and the
// ButtonDanger (red border/ink) draw branches.
func TestButtonSelectedAndDanger(t *testing.T) {
	th := DefaultLight()

	// Selected fills with Accent — assert an accent pixel in the body.
	sel := NewButton("Reddit", nil)
	sel.Selected = true
	sel.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf := makeSurface(60, 20)
	sel.Draw(newP(buf, 60), th)
	// Sample a body pixel left of the centred label so we hit the fill, not text.
	if px := pixelAt(buf, 60, 4, 10); px != th.Accent {
		t.Fatalf("selected body = %+v, want Accent %+v", px, th.Accent)
	}

	// ButtonDanger draws a red border — assert a red pixel on the top edge.
	del := NewButton("Delete", nil)
	del.Style = ButtonDanger
	del.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 20})
	buf2 := makeSurface(60, 20)
	del.Draw(newP(buf2, 60), th)
	found := false
	for x := 0; x < 60 && !found; x++ {
		if pixelAt(buf2, 60, x, 0) == dangerInk {
			found = true
		}
	}
	if !found {
		t.Fatal("ButtonDanger should stroke a red border")
	}
}

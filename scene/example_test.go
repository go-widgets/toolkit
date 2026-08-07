// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Date: 2026-08-07
package scene

import (
	"fmt"

	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Example shows the opt-in damage loop: wrap a laid-out root in a Scene, then on
// each frame Invalidate whatever changed and Render — the returned Region is the
// exact rectangle set the host must blit.
func Example() {
	// A 200x120 surface with one opaque cell the "hover" recolours.
	hovered := newCell(Rect{X: 20, Y: 20, W: 40, H: 30}, blue)
	root := newGroup(Rect{X: 0, Y: 0, W: 200, H: 120}, grey, hovered)

	s := New(root)
	buf := make([]byte, 4*200*120)
	p := painter.NewPixelPainter(buf, 200, 120)
	theme := toolkit.DefaultLight()

	s.Render(p, theme) // first frame paints everything

	// The pointer enters the cell: recolour it and damage just that widget.
	hovered.col = red
	s.Invalidate(hovered)
	region := s.Render(p, theme)

	fmt.Printf("blit %d rect(s), bounds=%v, bytes=%d\n",
		len(region.Rects()), region.Bounds(), region.Area()*4)
	// Output:
	// blit 1 rect(s), bounds={20 20 40 30}, bytes=4800
}

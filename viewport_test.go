// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// vpProbe is a leaf that paints a solid fill across its whole bounds and records
// the last event it received, so a test can assert both exact geometry and event
// routing.
type vpProbe struct {
	Base
	ink    RGBA
	got    bool
	ev     Event
	drawn  bool
	drawAt Rect
}

func (p *vpProbe) Draw(pt painter.Painter, _ *Theme) {
	p.drawn, p.drawAt = true, p.Bounds()
	r := p.Bounds()
	fillRect(pt, r.X, r.Y, r.W, r.H, p.ink)
}
func (p *vpProbe) OnEvent(ev Event) { p.got, p.ev = true, ev }

// The four edge sizes and the surface used across the layout assertions.
const (
	vpW, vpH                         = 100, 60
	vpTop, vpBottom, vpLeft, vpRight = 10, 8, 12, 6
)

// newFullViewport builds a Viewport with a widget in every one of its five slots
// and returns it plus the five probes, laid out over the vpW x vpH surface.
func newFullViewport() (*Viewport, [regionCount]*vpProbe) {
	var probes [regionCount]*vpProbe
	v := NewViewport()
	for i := range probes {
		probes[i] = &vpProbe{ink: RGBA{R: uint8(i * 40), A: 255}}
	}
	v.Set(ViewportTop, probes[ViewportTop], vpTop)
	v.Set(ViewportBottom, probes[ViewportBottom], vpBottom)
	v.Set(ViewportLeft, probes[ViewportLeft], vpLeft)
	v.Set(ViewportRight, probes[ViewportRight], vpRight)
	v.Set(ViewportCenter, probes[ViewportCenter], 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: vpW, H: vpH})
	return v, probes
}

// wantRects are the exact rectangles the five slots must occupy on the vpW x vpH
// surface, in the fixed carve precedence (top and bottom span the width; left and
// right take the band between them; the centre fills the rest).
var wantRects = map[ViewportRegion]Rect{
	ViewportTop:    {X: 0, Y: 0, W: vpW, H: vpTop},
	ViewportBottom: {X: 0, Y: vpH - vpBottom, W: vpW, H: vpBottom},
	ViewportLeft:   {X: 0, Y: vpTop, W: vpLeft, H: vpH - vpTop - vpBottom},
	ViewportRight:  {X: vpW - vpRight, Y: vpTop, W: vpRight, H: vpH - vpTop - vpBottom},
	ViewportCenter: {X: vpLeft, Y: vpTop, W: vpW - vpLeft - vpRight, H: vpH - vpTop - vpBottom},
}

// TestViewportExactRegionBounds asserts every slot lands on its exact rectangle,
// both as reported by RegionRect and as pushed down to each child's Bounds.
func TestViewportExactRegionBounds(t *testing.T) {
	v, probes := newFullViewport()
	for region, want := range wantRects {
		if got := v.RegionRect(region); got != want {
			t.Errorf("RegionRect(%d) = %+v, want %+v", region, got, want)
		}
		if got := probes[region].Bounds(); got != want {
			t.Errorf("region %d child bounds = %+v, want %+v", region, got, want)
		}
	}
}

// TestViewportRegionsTileWithoutGapOrOverlap checks the four edges plus the
// centre exactly tile the surface: every pixel of the surface belongs to exactly
// one slot, and none reaches outside it.
func TestViewportRegionsTileWithoutGapOrOverlap(t *testing.T) {
	v, _ := newFullViewport()
	owner := make([][]int, vpH)
	for y := range owner {
		owner[y] = make([]int, vpW)
		for x := range owner[y] {
			owner[y][x] = -1
		}
	}
	for region := ViewportRegion(0); region < regionCount; region++ {
		r := v.RegionRect(region)
		for y := r.Y; y < r.Y+r.H; y++ {
			for x := r.X; x < r.X+r.W; x++ {
				if x < 0 || y < 0 || x >= vpW || y >= vpH {
					t.Fatalf("region %d rect %+v reaches outside the surface at (%d,%d)", region, r, x, y)
				}
				if owner[y][x] != -1 {
					t.Fatalf("(%d,%d) claimed by region %d and region %d", x, y, owner[y][x], region)
				}
				owner[y][x] = int(region)
			}
		}
	}
	for y := 0; y < vpH; y++ {
		for x := 0; x < vpW; x++ {
			if owner[y][x] == -1 {
				t.Fatalf("(%d,%d) belongs to no region", x, y)
			}
		}
	}
}

// TestViewportRefillsOnResize is the core of the harness: after a resize the
// slots must re-fill the new surface exactly, with the centre absorbing the
// change and the edges keeping their fixed extents.
func TestViewportRefillsOnResize(t *testing.T) {
	v, probes := newFullViewport()
	v.SetBounds(Rect{X: 5, Y: 7, W: 200, H: 120})

	want := map[ViewportRegion]Rect{
		ViewportTop:    {X: 5, Y: 7, W: 200, H: vpTop},
		ViewportBottom: {X: 5, Y: 7 + 120 - vpBottom, W: 200, H: vpBottom},
		ViewportLeft:   {X: 5, Y: 7 + vpTop, W: vpLeft, H: 120 - vpTop - vpBottom},
		ViewportRight:  {X: 5 + 200 - vpRight, Y: 7 + vpTop, W: vpRight, H: 120 - vpTop - vpBottom},
		ViewportCenter: {X: 5 + vpLeft, Y: 7 + vpTop, W: 200 - vpLeft - vpRight, H: 120 - vpTop - vpBottom},
	}
	for region, w := range want {
		if got := v.RegionRect(region); got != w {
			t.Errorf("after resize RegionRect(%d) = %+v, want %+v", region, got, w)
		}
		if got := probes[region].Bounds(); got != w {
			t.Errorf("after resize region %d child = %+v, want %+v", region, got, w)
		}
	}
}

// TestViewportEmptyCenterFillsRemainder checks that an unset centre still governs
// the remaining rectangle (RegionRect(ViewportCenter) is the remainder) and that an
// absent edge folds its space into the centre.
func TestViewportEmptyCenterFillsRemainder(t *testing.T) {
	v := NewViewport()
	top := &vpProbe{ink: RGBA{A: 255}}
	v.Set(ViewportTop, top, 15)
	v.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 40})

	// No centre widget, but the centre rect is the space below the top bar.
	if got := v.RegionRect(ViewportCenter); got != (Rect{X: 0, Y: 15, W: 80, H: 25}) {
		t.Fatalf("empty centre = %+v", got)
	}
	// Absent edges report the zero rect.
	if got := v.RegionRect(ViewportLeft); got != (Rect{}) {
		t.Fatalf("absent left = %+v, want zero", got)
	}
	// A wholly empty Viewport gives the whole surface to the centre.
	blank := NewViewport()
	blank.SetBounds(Rect{X: 0, Y: 0, W: 30, H: 20})
	if got := blank.RegionRect(ViewportCenter); got != (Rect{X: 0, Y: 0, W: 30, H: 20}) {
		t.Fatalf("blank centre = %+v", got)
	}
}

// TestViewportNoPixelOutsideBounds renders a full Viewport into a sentinel-filled
// pixel buffer and asserts NOT ONE painted pixel lands outside the Viewport's
// bounds — the overflow guard from the precise-bounds test discipline.
func TestViewportNoPixelOutsideBounds(t *testing.T) {
	const bw, bh = 60, 40
	sentinel := RGBA{R: 1, G: 2, B: 3, A: 4}
	buf := make([]byte, bw*bh*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = sentinel.R, sentinel.G, sentinel.B, sentinel.A
	}
	p := painter.NewPixelPainter(buf, bw, bh)

	v := NewViewport()
	v.Set(ViewportTop, &vpProbe{ink: RGBA{R: 200, A: 255}}, 5)
	v.Set(ViewportLeft, &vpProbe{ink: RGBA{G: 200, A: 255}}, 8)
	v.Set(ViewportCenter, &vpProbe{ink: RGBA{B: 200, A: 255}}, 0)
	// Bounds deliberately smaller than the buffer, offset from the origin, so a
	// margin of sentinel pixels surrounds it.
	b := Rect{X: 6, Y: 4, W: 40, H: 28}
	v.SetBounds(b)
	v.Draw(p, DefaultLight())

	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			i := (y*bw + x) * 4
			painted := buf[i] != sentinel.R || buf[i+1] != sentinel.G ||
				buf[i+2] != sentinel.B || buf[i+3] != sentinel.A
			inside := x >= b.X && x < b.X+b.W && y >= b.Y && y < b.Y+b.H
			if painted && !inside {
				t.Fatalf("painted pixel outside bounds at (%d,%d)", x, y)
			}
		}
	}
}

// TestViewportCellPrecise renders a Viewport through the cell back-end (the same
// path the terminal/tui host uses) and asserts, cell by cell, that each slot's
// background colour lands in exactly its region and nowhere else — a cell-precise
// render check for a widget that renders.
func TestViewportCellPrecise(t *testing.T) {
	const cw, ch = 20, 10
	p := painter.NewCellPainter(cw, ch)

	topInk := RGBA{R: 111, A: 255}
	botInk := RGBA{R: 222, A: 255}
	ctrInk := RGBA{R: 55, A: 255}
	v := NewViewport()
	v.Set(ViewportTop, &vpProbe{ink: topInk}, 2)
	v.Set(ViewportBottom, &vpProbe{ink: botInk}, 3)
	v.Set(ViewportCenter, &vpProbe{ink: ctrInk}, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: cw, H: ch})
	v.Draw(p, DefaultLight())

	bg := func(x, y int) RGBA { return p.Cells[y*cw+x].Bg }
	for y := 0; y < ch; y++ {
		var want RGBA
		switch {
		case y < 2:
			want = topInk // top bar: rows 0..1
		case y >= ch-3:
			want = botInk // bottom bar: rows 7..9
		default:
			want = ctrInk // centre: rows 2..6
		}
		for x := 0; x < cw; x++ {
			if got := bg(x, y); got != want {
				t.Fatalf("cell (%d,%d) bg = %+v, want %+v", x, y, got, want)
			}
		}
	}
}

// TestViewportEventRouting checks a click in each slot reaches that slot's widget
// in its own local coordinates, and that the edges win the seam over the centre.
func TestViewportEventRouting(t *testing.T) {
	v, probes := newFullViewport()

	// A point inside the top bar (surface (50,3)) → top probe, local (50,3).
	v.OnEvent(Event{Kind: EventClick, X: 50, Y: 3})
	if !probes[ViewportTop].got || probes[ViewportTop].ev.X != 50 || probes[ViewportTop].ev.Y != 3 {
		t.Fatalf("top routing: got=%v ev=%+v", probes[ViewportTop].got, probes[ViewportTop].ev)
	}
	// A point in the centre band (surface (50,30)) → centre probe, local
	// (50-vpLeft, 30-vpTop).
	v.OnEvent(Event{Kind: EventClick, X: 50, Y: 30})
	if !probes[ViewportCenter].got ||
		probes[ViewportCenter].ev.X != 50-vpLeft || probes[ViewportCenter].ev.Y != 30-vpTop {
		t.Fatalf("centre routing: got=%v ev=%+v", probes[ViewportCenter].got, probes[ViewportCenter].ev)
	}
	// A point in the left bar → left probe; the right bar → right probe.
	v.OnEvent(Event{Kind: EventClick, X: 3, Y: 30})
	if !probes[ViewportLeft].got {
		t.Fatalf("left routing missed")
	}
	v.OnEvent(Event{Kind: EventClick, X: vpW - 3, Y: 30})
	if !probes[ViewportRight].got {
		t.Fatalf("right routing missed")
	}

	// A click beyond an offset Viewport reaches no slot (and must not panic):
	// the centre is present but its bounds do not contain the far point.
	off, offProbes := newFullViewport()
	off.SetBounds(Rect{X: 10, Y: 10, W: vpW, H: vpH})
	off.OnEvent(Event{Kind: EventClick, X: 1000, Y: 1000})
	if offProbes[ViewportCenter].got {
		t.Fatalf("far outside click should reach no slot")
	}
}

// TestViewportSetGuards exercises the input guards: an out-of-range region is
// ignored, a negative size clamps to 0, and a nil widget clears a slot.
func TestViewportSetGuards(t *testing.T) {
	v := NewViewport()
	// Out-of-range region (below 0 and at/above the count) is a no-op.
	v.Set(ViewportRegion(-1), &vpProbe{}, 5)
	v.Set(regionCount, &vpProbe{}, 5)
	if got := v.RegionRect(ViewportRegion(-1)); got != (Rect{}) {
		t.Fatalf("out-of-range RegionRect = %+v", got)
	}
	if got := v.RegionRect(regionCount); got != (Rect{}) {
		t.Fatalf("out-of-range RegionRect = %+v", got)
	}

	// Negative size clamps to 0: the top bar collapses to zero height.
	top := &vpProbe{ink: RGBA{A: 255}}
	v.Set(ViewportTop, top, -20)
	v.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	if got := v.RegionRect(ViewportTop); got != (Rect{X: 0, Y: 0, W: 40, H: 0}) {
		t.Fatalf("clamped top = %+v", got)
	}

	// Clearing the slot (nil) drops it back to a zero rect.
	v.Set(ViewportTop, nil, 10)
	v.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	if got := v.RegionRect(ViewportTop); got != (Rect{}) {
		t.Fatalf("cleared top = %+v", got)
	}
}

// TestViewportChildrenReadingOrder checks Children yields exactly the present
// slots, edges clockwise from the top then the centre, skipping empty slots — the
// order a screen-reader walk announces the shell in.
func TestViewportChildrenReadingOrder(t *testing.T) {
	v, probes := newFullViewport()
	want := []Widget{
		probes[ViewportTop], probes[ViewportRight], probes[ViewportBottom],
		probes[ViewportLeft], probes[ViewportCenter],
	}
	got := v.Children()
	if len(got) != len(want) {
		t.Fatalf("Children len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Children[%d] = %p, want %p", i, got[i], want[i])
		}
	}

	// With only two slots set, Children yields just those two, in reading order.
	v2 := NewViewport()
	c := &vpProbe{}
	l := &vpProbe{}
	v2.Set(ViewportCenter, c, 0)
	v2.Set(ViewportLeft, l, 5)
	got2 := v2.Children()
	if len(got2) != 2 || got2[0] != l || got2[1] != c {
		t.Fatalf("sparse Children = %v, want [left centre]", got2)
	}
}

// TestViewportA11yIsPresentational pins the shell's accessibility role: it is a
// presentational container, so CollectA11y skips it but still walks its children.
func TestViewportA11yIsPresentational(t *testing.T) {
	v := NewViewport()
	if got := v.A11y(); got.Role != RolePresentation {
		t.Fatalf("A11y role = %q, want %q", got.Role, RolePresentation)
	}
}

// TestViewportDrawSkipsEmptySlots renders a Viewport with only a centre and
// checks the edge slots never draw (no probe marked drawn) while the centre does.
func TestViewportDrawSkipsEmptySlots(t *testing.T) {
	const bw, bh = 20, 20
	buf := make([]byte, bw*bh*4)
	p := painter.NewPixelPainter(buf, bw, bh)
	c := &vpProbe{ink: RGBA{B: 200, A: 255}}
	v := NewViewport()
	v.Set(ViewportCenter, c, 0)
	v.SetBounds(Rect{X: 0, Y: 0, W: bw, H: bh})
	v.Draw(p, DefaultLight())
	if !c.drawn {
		t.Fatal("centre not drawn")
	}
	if c.drawAt != (Rect{X: 0, Y: 0, W: bw, H: bh}) {
		t.Fatalf("centre drawn at %+v", c.drawAt)
	}
}

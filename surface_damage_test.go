// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

// Without a Damage hook, RenderDamaged blits the whole buffer and reports its
// footprint — pixel-identical to Draw followed by a full present, so a host that
// only knows the incremental path shows exactly what the whole-surface path
// would.
func TestSurfaceRenderDamagedWholeSurfaceWithoutHook(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })

	pDmg := surface(12, 12)
	rects := s.RenderDamaged(pDmg, DefaultDark())

	if want := []Rect{{X: 0, Y: 0, W: 6, H: 5}}; !reflect.DeepEqual(rects, want) {
		t.Fatalf("damage = %v, want the whole footprint %v", rects, want)
	}
	// Same pixels as Draw, to the byte.
	pDraw := surface(12, 12)
	s.Draw(pDraw, DefaultDark())
	for i := range pDraw.Buf {
		if pDmg.Buf[i] != pDraw.Buf[i] {
			t.Fatalf("byte %d: RenderDamaged=%d, Draw=%d", i, pDmg.Buf[i], pDraw.Buf[i])
		}
	}
}

// An empty (non-nil) Damage slice means the same as no hook: assume everything
// changed, blit the whole surface.
func TestSurfaceRenderDamagedEmptySliceIsWholeSurface(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })
	s.Damage = func() []Rect { return []Rect{} }

	p := surface(12, 12)
	rects := s.RenderDamaged(p, DefaultDark())
	if want := []Rect{{X: 0, Y: 0, W: 6, H: 5}}; !reflect.DeepEqual(rects, want) {
		t.Fatalf("damage = %v, want %v", rects, want)
	}
	if pixelAt(p.Buf, p.Width, 0, 0) != pixelAt(src, 6, 0, 0) {
		t.Error("whole surface should have blitted the buffer's 0,0")
	}
}

// With a Damage hook reporting one rectangle, RenderDamaged blits ONLY that
// rectangle: its pixels land, everything else is left untouched, and the
// reported rectangle (in surface coordinates) is returned.
func TestSurfaceRenderDamagedBlitsOnlyReportedRect(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })
	s.Damage = func() []Rect { return []Rect{{X: 2, Y: 1, W: 2, H: 2}} }

	p := surface(12, 12)
	rects := s.RenderDamaged(p, DefaultDark())

	if want := []Rect{{X: 2, Y: 1, W: 2, H: 2}}; !reflect.DeepEqual(rects, want) {
		t.Fatalf("damage = %v, want %v", rects, want)
	}
	// Inside the reported rectangle: the buffer's own pixels.
	if got, want := pixelAt(p.Buf, p.Width, 2, 1), pixelAt(src, 6, 2, 1); got != want {
		t.Errorf("inside damage (2,1) = %v, want %v", got, want)
	}
	if got, want := pixelAt(p.Buf, p.Width, 3, 2), pixelAt(src, 6, 3, 2); got != want {
		t.Errorf("inside damage (3,2) = %v, want %v", got, want)
	}
	// Outside it: never written.
	if pixelAt(p.Buf, p.Width, 0, 0) != (RGBA{}) {
		t.Error("blitted (0,0), which was not in the damage")
	}
	if pixelAt(p.Buf, p.Width, 5, 4) != (RGBA{}) {
		t.Error("blitted (5,4), which was not in the damage")
	}
}

// Damage rectangles are in the buffer's coordinates; RenderDamaged offsets them
// onto the surface where the widget sits, so both the returned damage and the
// blitted pixels are in surface space.
func TestSurfaceRenderDamagedOffsetsOntoBounds(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })
	s.SetBounds(Rect{X: 2, Y: 3, W: 6, H: 5})
	s.Damage = func() []Rect { return []Rect{{X: 1, Y: 1, W: 2, H: 2}} }

	p := surface(12, 12)
	rects := s.RenderDamaged(p, DefaultDark())

	if want := []Rect{{X: 3, Y: 4, W: 2, H: 2}}; !reflect.DeepEqual(rects, want) {
		t.Fatalf("damage = %v, want it offset onto the bounds %v", rects, want)
	}
	if got, want := pixelAt(p.Buf, p.Width, 3, 4), pixelAt(src, 6, 1, 1); got != want {
		t.Errorf("surface (3,4) = %v, want the buffer's (1,1) = %v", got, want)
	}
	// The buffer's own origin (surface 2,3) is outside the damage: untouched.
	if pixelAt(p.Buf, p.Width, 2, 3) != (RGBA{}) {
		t.Error("blitted the buffer origin, which was outside the damage")
	}
}

// A damage rectangle reaching past the buffer is clamped to it; one wholly off
// the buffer contributes nothing and is dropped.
func TestSurfaceRenderDamagedClampsAndDropsOffBufferRects(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })
	s.Damage = func() []Rect {
		return []Rect{
			{X: 4, Y: 4, W: 4, H: 4},   // straddles the 6x5 edge -> clamps to 2x1
			{X: 10, Y: 10, W: 2, H: 2}, // wholly off -> dropped
		}
	}

	p := surface(12, 12)
	rects := s.RenderDamaged(p, DefaultDark())
	if want := []Rect{{X: 4, Y: 4, W: 2, H: 1}}; !reflect.DeepEqual(rects, want) {
		t.Fatalf("damage = %v, want only the clamped rect %v", rects, want)
	}
	if got, want := pixelAt(p.Buf, p.Width, 4, 4), pixelAt(src, 6, 4, 4); got != want {
		t.Errorf("clamped damage (4,4) = %v, want %v", got, want)
	}
}

// The incremental path shares Draw's guards: nothing usable to show paints
// nothing and reports no damage.
func TestSurfaceRenderDamagedNothingWithoutUsableFrame(t *testing.T) {
	for _, tc := range []struct {
		name  string
		s     *Surface
		bound Rect
	}{
		{"no Frame", &Surface{}, Rect{W: 4, H: 4}},
		{"zero-size buffer", NewSurface(func() ([]byte, int, int) { return nil, 0, 0 }), Rect{W: 4, H: 4}},
		{"buffer too short", NewSurface(func() ([]byte, int, int) { return make([]byte, 4), 4, 4 }), Rect{W: 4, H: 4}},
		{"empty bounds", NewSurface(func() ([]byte, int, int) { return gradient(4, 4), 4, 4 }), Rect{W: 0, H: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.s.SetBounds(tc.bound)
			tc.s.Damage = func() []Rect { return []Rect{{X: 0, Y: 0, W: 2, H: 2}} }
			p := surface(8, 8)
			if got := tc.s.RenderDamaged(p, DefaultDark()); got != nil {
				t.Fatalf("damage = %v, want nil", got)
			}
			for i := range p.Buf {
				if p.Buf[i] != 0 {
					t.Fatalf("painted byte %d without a usable frame", i)
				}
			}
		})
	}
}

// intersectRect returns the overlap, or a zero rectangle when the two do not
// meet — on either axis.
func TestIntersectRect(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b Rect
		want Rect
	}{
		{"overlap", Rect{X: 0, Y: 0, W: 4, H: 4}, Rect{X: 2, Y: 2, W: 4, H: 4}, Rect{X: 2, Y: 2, W: 2, H: 2}},
		{"disjoint on x", Rect{X: 0, Y: 0, W: 2, H: 4}, Rect{X: 3, Y: 0, W: 2, H: 4}, Rect{}},
		{"disjoint on y", Rect{X: 0, Y: 0, W: 4, H: 2}, Rect{X: 0, Y: 3, W: 4, H: 2}, Rect{}},
		{"nested", Rect{X: 1, Y: 1, W: 2, H: 2}, Rect{X: 0, Y: 0, W: 6, H: 6}, Rect{X: 1, Y: 1, W: 2, H: 2}},
	} {
		if got := intersectRect(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: intersectRect(%v,%v) = %v, want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

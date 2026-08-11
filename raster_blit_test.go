// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// basePainter forwards the base Painter -- and NOTHING else -- to a real
// PixelPainter. Embedding the PixelPainter would promote its optional
// capabilities and defeat the point, so every method is spelled out. The three
// doubles below add back exactly one capability each, which is how a back-end
// that predates the image primitive, or that clips but cannot blit, is
// reproduced faithfully.
type basePainter struct{ p *painter.PixelPainter }

func (n *basePainter) FillRect(r Rect, c RGBA)               { n.p.FillRect(r, c) }
func (n *basePainter) StrokeRect(r Rect, c RGBA, w int)      { n.p.StrokeRect(r, c, w) }
func (n *basePainter) FillRoundRect(r Rect, rad int, c RGBA) { n.p.FillRoundRect(r, rad, c) }
func (n *basePainter) StrokeRoundRect(r Rect, rad int, c RGBA, w int) {
	n.p.StrokeRoundRect(r, rad, c, w)
}
func (n *basePainter) PutPixel(x, y int, c RGBA)       { n.p.PutPixel(x, y, c) }
func (n *basePainter) Text(x, y int, s string, c RGBA) { n.p.Text(x, y, s, c) }
func (n *basePainter) Size() (int, int)                { return n.p.Size() }

// noImage clips but has no image primitive: the reference every other path is
// compared against, since it is the loop the widgets used to carry themselves.
type noImage struct{ basePainter }

func (n *noImage) PushClip(r Rect) { n.p.PushClip(r) }
func (n *noImage) PopClip()        { n.p.PopClip() }

// noImageNoClip has neither: the most limited back-end the contract allows.
type noImageNoClip struct{ basePainter }

// imageNoClip carries the image primitive but NOT the Clipper: a back-end that
// can blit a block yet cannot restrict where it lands. blitImage may use the
// primitive only when the clip cannot bite, and must fall back otherwise.
type imageNoClip struct{ basePainter }

func (n *imageNoClip) DrawImage(dst Rect, src []byte, srcW, srcH int) {
	n.p.DrawImage(dst, src, srcW, srcH)
}

func surface(w, h int) *painter.PixelPainter {
	return &painter.PixelPainter{Buf: make([]byte, w*h*4), Width: w, Height: h}
}

// gradient builds a w*h RGBA block with a distinct value per pixel, so a
// misplaced sample shows up as a byte difference rather than blending in.
func gradient(w, h int) []byte {
	b := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			b[o], b[o+1], b[o+2], b[o+3] = uint8(x*7), uint8(y*11), uint8(x+y), 0xFF
		}
	}
	return b
}

func sameBytes(t *testing.T, what string, a, b *painter.PixelPainter) {
	t.Helper()
	for i := range a.Buf {
		if a.Buf[i] != b.Buf[i] {
			px := i / 4
			t.Fatalf("%s: pixel %d,%d byte %d differs: primitive %d, per-pixel %d",
				what, px%a.Width, px/a.Width, i%4, a.Buf[i], b.Buf[i])
		}
	}
}

// The claim the helper's doc makes: a back-end with the primitive and one
// without it produce IDENTICAL surfaces. Speed that changes the picture is not
// an optimisation.
func TestBlitImageMatchesThePerPixelLoop(t *testing.T) {
	src := gradient(6, 5)
	cases := []struct {
		name      string
		dst, clip Rect
	}{
		{"1:1", Rect{X: 2, Y: 2, W: 6, H: 5}, Rect{X: 0, Y: 0, W: 20, H: 20}},
		{"upscaled", Rect{X: 0, Y: 0, W: 18, H: 15}, Rect{X: 0, Y: 0, W: 20, H: 20}},
		{"downscaled", Rect{X: 1, Y: 1, W: 3, H: 2}, Rect{X: 0, Y: 0, W: 20, H: 20}},
		{"clipped", Rect{X: 0, Y: 0, W: 18, H: 15}, Rect{X: 4, Y: 3, W: 5, H: 6}},
		{"clip cuts the whole blit", Rect{X: 0, Y: 0, W: 6, H: 5}, Rect{X: 15, Y: 15, W: 2, H: 2}},
		{"off the left and top edges", Rect{X: -3, Y: -2, W: 6, H: 5}, Rect{X: -5, Y: -5, W: 30, H: 30}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fast := surface(20, 20)
			blitImage(fast, tc.dst, tc.clip, src, 6, 5)

			ref := surface(20, 20)
			blitImage(&noImage{basePainter{p: ref}}, tc.dst, tc.clip, src, 6, 5)

			sameBytes(t, tc.name, fast, ref)
		})
	}
}

// A back-end with the primitive but NO Clipper may only take the fast path when
// the clip cannot bite; otherwise it must fall back, or it would paint outside
// the area the caller reserved.
func TestBlitImageWithoutAClipperStillHonoursTheClip(t *testing.T) {
	src := gradient(6, 5)
	dst := Rect{X: 0, Y: 0, W: 6, H: 5}
	clip := Rect{X: 2, Y: 2, W: 2, H: 2}

	ref := surface(20, 20)
	blitImage(&noImage{basePainter{p: ref}}, dst, clip, src, 6, 5)

	got := surface(20, 20)
	blitImage(&noImageNoClip{basePainter{p: got}}, dst, clip, src, 6, 5)

	sameBytes(t, "no clipper", got, ref)
	// And the clip really did bite, or the case would prove nothing.
	if got.Buf[3] != 0 {
		t.Error("painted at 0,0, outside the clip")
	}
	if got.Buf[(2*20+2)*4+3] == 0 {
		t.Error("nothing painted inside the clip")
	}
}

// Nonsense arguments leave the surface alone rather than panicking: the same
// tolerance the painter itself offers.
func TestBlitImageRejectsNonsense(t *testing.T) {
	p := surface(8, 8)
	full := Rect{X: 0, Y: 0, W: 8, H: 8}
	blitImage(p, full, full, gradient(2, 2), 0, 2)   // no source width
	blitImage(p, full, full, gradient(2, 2), 2, 0)   // no source height
	blitImage(p, Rect{}, full, gradient(2, 2), 2, 2) // empty destination
	blitImage(p, full, full, []byte{1, 2, 3}, 2, 2)  // source too short
	for i := range p.Buf {
		if p.Buf[i] != 0 {
			t.Fatal("a malformed blit painted something")
		}
	}
}

// A back-end with the primitive and no Clipper: it takes the fast path when the
// clip encloses the destination, and the per-pixel path when it does not.
// Either way the pixels match the reference.
func TestBlitImageUsesThePrimitiveOnlyWhenTheClipCannotBite(t *testing.T) {
	src := gradient(6, 5)
	dst := Rect{X: 2, Y: 2, W: 6, H: 5}

	for _, tc := range []struct {
		name string
		clip Rect
	}{
		{"clip encloses the destination", Rect{X: 0, Y: 0, W: 20, H: 20}},
		{"clip cuts into the destination", Rect{X: 3, Y: 3, W: 2, H: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := surface(20, 20)
			blitImage(&noImage{basePainter{p: ref}}, dst, tc.clip, src, 6, 5)

			got := surface(20, 20)
			blitImage(&imageNoClip{basePainter{p: got}}, dst, tc.clip, src, 6, 5)

			sameBytes(t, tc.name, got, ref)
		})
	}
}

func TestRectEncloses(t *testing.T) {
	outer := Rect{X: 0, Y: 0, W: 10, H: 10}
	if !rectEncloses(outer, Rect{X: 1, Y: 1, W: 8, H: 8}) {
		t.Error("an enclosed rect was reported outside")
	}
	if rectEncloses(outer, Rect{X: -1, Y: 0, W: 2, H: 2}) {
		t.Error("a rect past the left edge was reported inside")
	}
	if rectEncloses(outer, Rect{X: 0, Y: -1, W: 2, H: 2}) {
		t.Error("a rect past the top edge was reported inside")
	}
	if rectEncloses(outer, Rect{X: 5, Y: 0, W: 6, H: 2}) {
		t.Error("a rect past the right edge was reported inside")
	}
	if rectEncloses(outer, Rect{X: 0, Y: 5, W: 2, H: 6}) {
		t.Error("a rect past the bottom edge was reported inside")
	}
}

// The three widgets that moved onto the primitive must paint what they painted
// before, so each is rendered twice: once on a back-end with the primitive, once
// on one without.
func TestMigratedWidgetsPaintIdentically(t *testing.T) {
	src := gradient(6, 5)

	widgets := map[string]func() Widget{
		"Image fit": func() Widget {
			i := NewImage(src, 6, 5)
			i.Scale = ScaleFit
			i.SetBounds(Rect{X: 1, Y: 1, W: 17, H: 12})
			return i
		},
		"Image stretch": func() Widget {
			i := NewImage(src, 6, 5)
			i.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
			return i
		},
		"Wallpaper fill (cover)": func() Widget {
			w := NewWallpaper(src, 6, 5, WallpaperFill)
			w.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
			return w
		},
		"Wallpaper center": func() Widget {
			w := NewWallpaper(src, 6, 5, WallpaperCenter)
			w.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
			return w
		},
		"Wallpaper fit": func() Widget {
			w := NewWallpaper(src, 6, 5, WallpaperFit)
			w.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
			return w
		},
		"Wallpaper tile": func() Widget {
			w := NewWallpaper(src, 6, 5, WallpaperTile)
			w.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
			return w
		},
	}
	for name, build := range widgets {
		t.Run(name, func(t *testing.T) {
			theme := DefaultDark()

			fast := surface(20, 20)
			build().Draw(fast, theme)

			ref := surface(20, 20)
			build().Draw(&noImage{basePainter{p: ref}}, theme)

			sameBytes(t, name, fast, ref)
			// A blank surface would pass trivially.
			var painted bool
			for i := 3; i < len(fast.Buf); i += 4 {
				if fast.Buf[i] != 0 {
					painted = true
					break
				}
			}
			if !painted {
				t.Fatal("the widget painted nothing, so the comparison proves nothing")
			}
		})
	}
}

// The three Wallpaper paths did not just change back-end: their loops changed
// SHAPE. Cover now paints the scaled image whole and lets the clip crop it,
// and Tile issues one blit per tile instead of walking the destination. Testing
// the new code against itself would prove nothing about that, so the pre-
// migration implementations are reproduced here verbatim and compared against.

func refCover(p painter.Painter, r Rect, pix []byte, iw, ih int) {
	scaledW, scaledH := r.W, ih*r.W/iw
	if scaledH < r.H {
		scaledH, scaledW = r.H, iw*r.H/ih
	}
	ox := r.X + (r.W-scaledW)/2
	oy := r.Y + (r.H-scaledH)/2
	for dy := 0; dy < r.H; dy++ {
		sy := clampInt((r.Y+dy-oy)*ih/scaledH, 0, ih-1)
		for dx := 0; dx < r.W; dx++ {
			sx := clampInt((r.X+dx-ox)*iw/scaledW, 0, iw-1)
			p.PutPixel(r.X+dx, r.Y+dy, samplePixel(pix, iw, sx, sy))
		}
	}
}

func refCenter(p painter.Painter, r Rect, pix []byte, iw, ih int) {
	ox := r.X + (r.W-iw)/2
	oy := r.Y + (r.H-ih)/2
	for sy := 0; sy < ih; sy++ {
		dy := oy + sy
		if dy < r.Y || dy >= r.Y+r.H {
			continue
		}
		for sx := 0; sx < iw; sx++ {
			dx := ox + sx
			if dx < r.X || dx >= r.X+r.W {
				continue
			}
			p.PutPixel(dx, dy, samplePixel(pix, iw, sx, sy))
		}
	}
}

func refTile(p painter.Painter, r Rect, pix []byte, iw, ih int) {
	for dy := 0; dy < r.H; dy++ {
		sy := dy % ih
		for dx := 0; dx < r.W; dx++ {
			sx := dx % iw
			p.PutPixel(r.X+dx, r.Y+dy, samplePixel(pix, iw, sx, sy))
		}
	}
}

func TestWallpaperPaintsWhatItPaintedBeforeTheMigration(t *testing.T) {
	// Sizes chosen so the image divides the bounds evenly in one case and not
	// at all in the others: rounding is where a rewritten loop drifts.
	geoms := []struct {
		iw, ih, bw, bh int
	}{
		{6, 5, 20, 20}, {4, 4, 16, 16}, {7, 3, 20, 11}, {9, 9, 5, 5},
	}
	modes := []struct {
		name string
		mode WallpaperMode
		ref  func(painter.Painter, Rect, []byte, int, int)
	}{
		{"fill (cover)", WallpaperFill, refCover},
		{"center", WallpaperCenter, refCenter},
		{"tile", WallpaperTile, refTile},
	}
	for _, g := range geoms {
		for _, m := range modes {
			t.Run(m.name, func(t *testing.T) {
				src := gradient(g.iw, g.ih)
				r := Rect{X: 0, Y: 0, W: g.bw, H: g.bh}
				theme := DefaultDark()

				got := surface(g.bw, g.bh)
				w := NewWallpaper(src, g.iw, g.ih, m.mode)
				w.SetBounds(r)
				w.Draw(got, theme)

				// The reference covers the image only; the fallback beneath it
				// is untouched code, so it is painted the same way first.
				want := surface(g.bw, g.bh)
				wf := NewWallpaper(nil, 0, 0, m.mode)
				wf.SetBounds(r)
				wf.Draw(want, theme)
				m.ref(want, r, src, g.iw, g.ih)

				sameBytes(t, m.name, got, want)
			})
		}
	}
}

// What the migration is worth where a user sees it: a full-window wallpaper and
// a full-window image, drawn on a back-end with the primitive and on the
// per-pixel back-end they used before.
func benchWidget(b *testing.B, w Widget, primitive bool) {
	r := Rect{X: 0, Y: 0, W: 1000, H: 700}
	w.SetBounds(r)
	theme := DefaultDark()
	s := surface(1000, 700)
	var p painter.Painter = s
	if !primitive {
		p = &noImage{basePainter{p: s}}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Draw(p, theme)
	}
}

func benchImage() Widget     { return NewImage(gradient(500, 350), 500, 350) }
func benchWallpaper() Widget { return NewWallpaper(gradient(500, 350), 500, 350, WallpaperFill) }

func BenchmarkImageWithPrimitive(b *testing.B)     { benchWidget(b, benchImage(), true) }
func BenchmarkImagePerPixel(b *testing.B)          { benchWidget(b, benchImage(), false) }
func BenchmarkWallpaperWithPrimitive(b *testing.B) { benchWidget(b, benchWallpaper(), true) }
func BenchmarkWallpaperPerPixel(b *testing.B)      { benchWidget(b, benchWallpaper(), false) }

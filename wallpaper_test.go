// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// gradBuf builds a w*h RGBA source whose per-pixel colour encodes column/row,
// so a scale/placement can be checked pixel-precisely.
func gradBuf(w, h int) []byte {
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			buf[o], buf[o+1], buf[o+2], buf[o+3] = byte(x+1), byte(y+1), 0x40, 0xFF
		}
	}
	return buf
}

func TestWallpaperEmptyBoundsNoPanic(t *testing.T) {
	buf := make([]byte, 4)
	p := painter.NewPixelPainter(buf, 1, 1)
	w := NewWallpaperGradient(RGB(1, 2, 3), RGBA{})
	w.SetBounds(Rect{}) // zero bounds
	w.Draw(p, DefaultLight())
	if buf[3] != 0 {
		t.Fatal("empty-bounds wallpaper painted")
	}
}

func TestWallpaperSolidFallbackAndThemeDefault(t *testing.T) {
	const W, H = 4, 4
	th := DefaultLight()
	// Top unset (A==0) => theme.Background; Bottom unset => solid (no gradient).
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	w := &Wallpaper{}
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, th)
	r, g, b, a := at(buf, W, 2, 2)
	if a != 0xFF || r != th.Background.R || g != th.Background.G || b != th.Background.B {
		t.Fatalf("solid fallback = %d,%d,%d,%d, want theme background", r, g, b, a)
	}
	// Explicit opaque solid Top honoured.
	buf2 := make([]byte, W*H*4)
	p2 := painter.NewPixelPainter(buf2, W, H)
	w2 := NewWallpaperGradient(RGB(0x10, 0x20, 0x30), RGBA{})
	w2.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w2.Draw(p2, th)
	if r, g, b, _ := at(buf2, W, 0, 0); r != 0x10 || g != 0x20 || b != 0x30 {
		t.Fatalf("solid top = %d,%d,%d", r, g, b)
	}
}

func TestWallpaperVerticalGradient(t *testing.T) {
	const W, H = 4, 4
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	w := NewWallpaperGradient(RGB(0, 0, 0), RGB(0xFF, 0, 0)) // black -> red down
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// Top row is the top stop; bottom row is the bottom stop.
	if r, _, _, _ := at(buf, W, 0, 0); r != 0 {
		t.Fatalf("top row R = %d, want 0", r)
	}
	if r, _, _, _ := at(buf, W, 0, H-1); r != 0xFF {
		t.Fatalf("bottom row R = %d, want 255", r)
	}
	// Monotonic increase down the column.
	var prev byte
	for y := 0; y < H; y++ {
		r, _, _, _ := at(buf, W, 0, y)
		if y > 0 && r < prev {
			t.Fatalf("gradient not monotonic at row %d: %d < %d", y, r, prev)
		}
		prev = r
	}
}

func TestWallpaperFillCoverCropsAndCovers(t *testing.T) {
	// A 2x1 (wide) source covering a 4x4 box: cover scales to 8x4, crops sides.
	const W, H = 4, 4
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	w := NewWallpaper(gradBuf(2, 1), 2, 1, WallpaperFill)
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// Every pixel is covered by the image (alpha 0xFF from the source).
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if _, _, _, a := at(buf, W, x, y); a != 0xFF {
				t.Fatalf("cover left (%d,%d) uncovered", x, y)
			}
		}
	}
}

func TestWallpaperFillTallSource(t *testing.T) {
	// A 1x2 (tall) source into a 4x4 box exercises the scaledH>=H branch's else.
	const W, H = 4, 4
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	w := NewWallpaper(gradBuf(1, 2), 1, 2, WallpaperFill)
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if _, _, _, a := at(buf, W, x, y); a != 0xFF {
				t.Fatalf("tall cover (%d,%d) uncovered", x, y)
			}
		}
	}
}

func TestWallpaperFitLeavesMargins(t *testing.T) {
	const W, H = 10, 10
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 4:1 wide source fitted into 10x10 => drawn 10x2 centred, margins = fallback.
	w := NewWallpaper(gradBuf(4, 1), 4, 1, WallpaperFit)
	w.Top, w.Bottom = RGB(9, 9, 9), RGBA{} // opaque solid fallback so margins are known
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// Centre band painted by the image (source col 1 => R>=1, and it's not 9).
	if _, _, _, a := at(buf, W, 5, 4); a != 0xFF {
		t.Fatal("fit did not paint the centred band")
	}
	// Top margin shows the fallback solid.
	if r, _, _, _ := at(buf, W, 5, 0); r != 9 {
		t.Fatalf("fit top margin R = %d, want fallback 9", r)
	}
}

func TestWallpaperCenterClipsAndMargins(t *testing.T) {
	const W, H = 6, 6
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 2x2 source centred 1:1 in 6x6 => occupies cols/rows 2..3, margins fallback.
	w := NewWallpaper(gradBuf(2, 2), 2, 2, WallpaperCenter)
	w.Top = RGB(7, 7, 7) // opaque solid fallback
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// Centre carries the source (col 0 => R==1 at dst col 2).
	if r, _, _, _ := at(buf, W, 2, 2); r != 1 {
		t.Fatalf("center pixel R = %d, want source 1", r)
	}
	// Corner is fallback.
	if r, _, _, _ := at(buf, W, 0, 0); r != 7 {
		t.Fatalf("center margin R = %d, want fallback 7", r)
	}
}

func TestWallpaperCenterLargerThanBoundsClips(t *testing.T) {
	const W, H = 2, 2
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 4x4 source centred in a 2x2 box => the off-box source pixels are clipped
	// (exercises the dx/dy < r bound continues).
	w := NewWallpaper(gradBuf(4, 4), 4, 4, WallpaperCenter)
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// Every destination pixel is covered (source spans the whole box).
	for i := 3; i < len(buf); i += 4 {
		if buf[i] != 0xFF {
			t.Fatal("center-crop left a hole")
		}
	}
}

func TestWallpaperTileRepeats(t *testing.T) {
	const W, H = 5, 5
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// 2x2 source tiled across 5x5.
	w := NewWallpaper(gradBuf(2, 2), 2, 2, WallpaperTile)
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	// (0,0) and (2,2) are the same source pixel (col 0,row 0) => equal R.
	r00, _, _, _ := at(buf, W, 0, 0)
	r22, _, _, _ := at(buf, W, 2, 2)
	if r00 != r22 {
		t.Fatalf("tile not periodic: (0,0)=%d (2,2)=%d", r00, r22)
	}
	// (1,0) is source col 1 => R==2, distinct from col 0 (R==1).
	if r10, _, _, _ := at(buf, W, 1, 0); r10 == r00 {
		t.Fatalf("tile columns not distinct: %d", r10)
	}
}

func TestWallpaperNoImagePaintsOnlyFallback(t *testing.T) {
	const W, H = 3, 3
	buf := make([]byte, W*H*4)
	p := painter.NewPixelPainter(buf, W, H)
	// Invalid image (short buffer) => hasImage false => only fallback.
	w := &Wallpaper{Pixels: make([]byte, 2), IW: 2, IH: 2, Top: RGB(5, 6, 7)}
	w.SetBounds(Rect{X: 0, Y: 0, W: W, H: H})
	w.Draw(p, DefaultLight())
	if r, g, b, _ := at(buf, W, 1, 1); r != 5 || g != 6 || b != 7 {
		t.Fatalf("invalid image should leave fallback: %d,%d,%d", r, g, b)
	}
}

func TestWallpaperHitTest(t *testing.T) {
	w := NewWallpaperGradient(RGB(1, 1, 1), RGBA{})
	w.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	if w.HitTest(5, 5) {
		t.Fatal("event-transparent by default: HitTest should be false")
	}
	w.Interactive = true
	if !w.HitTest(5, 5) {
		t.Fatal("interactive wallpaper should hit inside bounds")
	}
	if w.HitTest(50, 50) {
		t.Fatal("interactive wallpaper should miss outside bounds")
	}
}

func TestLerpRGBASingleRow(t *testing.T) {
	// den<=1 returns a unchanged (guards a 1px-tall gradient).
	got := lerpRGBA(RGB(10, 20, 30), RGB(0, 0, 0), 0, 1)
	if got != (RGBA{R: 10, G: 20, B: 30, A: 0xFF}) {
		t.Fatalf("lerp den=1 = %+v, want a unchanged", got)
	}
}

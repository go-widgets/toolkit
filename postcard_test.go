// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// PostCard draws in the default bitmap font: glyph height 7, advance 6
// (Measure(s) == 6*len(s)). Card constants: CardPadX=8, CardPadY=6, CardGapX=6,
// CardGapY=4, CardLineSpacing=2, BadgePadX=4, BadgePadY=1. Derived slots:
// badgeRowH = 7+2*1 = 9, titleSlot = 7+2 = 9, metaH = 7. Every Measure below is
// worked out by hand from those so a layout shift is caught.

func TestPostCardThumbSize(t *testing.T) {
	// Unset => defaults on both axes.
	w, h := (&PostCard{}).thumbSize()
	if w != DefaultPostCardThumbW || h != DefaultPostCardThumbH {
		t.Fatalf("default thumb size = %dx%d, want %dx%d", w, h, DefaultPostCardThumbW, DefaultPostCardThumbH)
	}
	// Explicit positive values are honoured; a non-positive axis falls back.
	w, h = (&PostCard{ThumbW: 40, ThumbH: 0}).thumbSize()
	if w != 40 || h != DefaultPostCardThumbH {
		t.Fatalf("mixed thumb size = %dx%d, want 40x%d", w, h, DefaultPostCardThumbH)
	}
}

func TestPostCardMaxLines(t *testing.T) {
	if got := (&PostCard{}).maxLines(); got != DefaultPostCardTitleLines {
		t.Fatalf("unset MaxTitleLines => %d, got %d", DefaultPostCardTitleLines, got)
	}
	if got := (&PostCard{MaxTitleLines: 2}).maxLines(); got != 2 {
		t.Fatalf("explicit MaxTitleLines respected, got %d", got)
	}
}

func TestPostCardMeasure(t *testing.T) {
	// Full row, no thumbnail, width 200 -> cw=184; "Hi"=12<=184 -> 1 line.
	// contentH = badge9 + 1*9 + gap4 + meta7 = 29; +2*CardPadY(6) = 41.
	if got := NewPostCard("SRC", "chan", "Hi", "meta").Measure(200); got != 41 {
		t.Fatalf("full one-line card: want 41, got %d", got)
	}
	// Empty title collapses the title block (n=0): 9 + 0 + 4 + 7 = 20; +12 = 32.
	if got := NewPostCard("SRC", "chan", "", "meta").Measure(200); got != 32 {
		t.Fatalf("title-less card: want 32, got %d", got)
	}
	// Bare title only (no badge row, no meta): 0 + 9 + 4 + 0 = 13; +12 = 25.
	if got := NewPostCard("", "", "Hi", "").Measure(200); got != 25 {
		t.Fatalf("bare title card: want 25, got %d", got)
	}
	// Two-line title at width 40 -> cw=24; "aaaaa"(=30) and "bbbbb"(=30) each own
	// line => n=2. 0 + 2*9 + 4 + 0 = 22; +12 = 34.
	if got := NewPostCard("", "", "aaaaa bbbbb", "").Measure(40); got != 34 {
		t.Fatalf("two-line title card: want 34, got %d", got)
	}
}

func TestPostCardMeasureThumbnail(t *testing.T) {
	// A default 72x72 thumbnail dwarfs the 29-tall natural content, so it
	// dictates the height: contentH = 72; +12 = 84.
	c := NewPostCard("SRC", "chan", "Hi", "meta")
	c.Thumbnail = cardImg(10, 10)
	if got := c.Measure(200); got != 84 {
		t.Fatalf("thumb-dominated card: want 84, got %d", got)
	}
	// A small 20x20 thumbnail is shorter than the 29-tall content, so the content
	// still dictates the height: contentH = 29; +12 = 41.
	c.ThumbW, c.ThumbH = 20, 20
	if got := c.Measure(200); got != 41 {
		t.Fatalf("content-dominated card: want 41, got %d", got)
	}
}

func TestPostCardMeasureNarrowClamps(t *testing.T) {
	// Width narrower than the padding (and the thumbnail column) clamps the
	// content width to 1 rather than going negative; the card still stacks a
	// positive height.
	c := NewPostCard("", "", "word", "")
	if got := c.Measure(10); got <= 12 {
		t.Fatalf("narrow no-thumb card should still stack, got %d", got)
	}
	c.Thumbnail = cardImg(4, 4)
	if got := c.Measure(10); got <= 12 {
		t.Fatalf("narrow thumb card should still stack, got %d", got)
	}
}

func TestPostCardPillInk(t *testing.T) {
	// An explicit PillInk wins outright.
	set := RGBA{R: 1, G: 2, B: 3, A: 0xFF}
	if got := (&PostCard{PillInk: set}).pillInk(); got != set {
		t.Fatalf("explicit PillInk = %+v, want %+v", got, set)
	}
	// No ink but a light pill -> a near-black derived ink.
	light := RGBA{R: 0xEE, G: 0xEE, B: 0xEE, A: 0xFF}
	if got := (&PostCard{PillColor: light}).pillInk(); got != readableInk(light) {
		t.Fatalf("derived ink on a light pill = %+v", got)
	}
	// No ink and no colour -> the zero RGBA (Badge falls back to the theme).
	if got := (&PostCard{}).pillInk(); got.A != 0 {
		t.Fatalf("unset pill ink should be zero, got %+v", got)
	}
}

func TestReadableInk(t *testing.T) {
	if got := readableInk(RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}); got.R != 0x11 {
		t.Fatalf("ink on white should be near-black, got %+v", got)
	}
	if got := readableInk(RGBA{R: 0, G: 0, B: 0, A: 0xFF}); got.R != 0xFF {
		t.Fatalf("ink on black should be near-white, got %+v", got)
	}
}

func TestPostCardChildrenRuns(t *testing.T) {
	c := NewPostCard("SRC", "channel", "Hello world", "▲12 · 3h")
	r := Rect{X: 20, Y: 20, W: 220, H: c.Measure(220)}
	buf := makeSurface(r.X+r.W+20, r.Y+r.H+20)
	c.SetBounds(r)
	c.Draw(newP(buf, r.X+r.W+20), DefaultLight())

	runs := CollectRuns(c)
	want := []string{"channel", "Hello world", "▲12 · 3h"}
	if len(runs) != len(want) {
		t.Fatalf("want %d runs %v, got %d %v", len(want), want, len(runs), runs)
	}
	for i, w := range want {
		if runs[i].Text != w {
			t.Fatalf("run %d = %q, want %q (order: subtitle, title, meta)", i, runs[i].Text, w)
		}
		b := runs[i].Bounds
		if b.X < r.X || b.Y < r.Y || b.X+b.W > r.X+r.W || b.Y+b.H > r.Y+r.H {
			t.Fatalf("run %d (%q) bounds %+v escape the card %+v", i, w, b, r)
		}
	}
}

func TestPostCardChildrenEmpty(t *testing.T) {
	// A pill-only card (no subtitle, no title, no meta) exposes no selectable
	// text, so Children is nil.
	if kids := NewPostCard("SRC", "", "", "").Children(); kids != nil {
		t.Fatalf("pill-only card should expose no children, got %v", kids)
	}
	// A title-only card exposes exactly its title line (subtitle + meta absent).
	kids := NewPostCard("", "", "Solo", "").Children()
	if len(kids) != 1 {
		t.Fatalf("title-only card should expose one child, got %d", len(kids))
	}
}

func TestPostCardA11y(t *testing.T) {
	got := NewPostCard("SRC", "chan", "The Headline", "meta").A11y()
	if got != (A11yInfo{Role: RoleGroup, Name: "The Headline"}) {
		t.Fatalf("A11y = %+v, want a group named by the title", got)
	}
}

func TestPostCardDrawWithinBounds(t *testing.T) {
	blue := RGBA{R: 0x20, G: 0x60, B: 0xC0, A: 0xFF}
	cases := []struct {
		name  string
		card  *PostCard
		width int
	}{
		{"full", func() *PostCard {
			c := NewPostCard("SRC", "a channel", "A wrapped headline that needs several lines to fit here", "▲128 · 3h")
			c.PillColor = blue
			c.Thumbnail = cardImg(16, 9)
			return c
		}(), 200},
		{"pill-no-subtitle-no-thumb", NewPostCard("SRC", "", "A title with no subtitle beside the pill", "meta"), 180},
		{"subtitle-no-pill-no-meta", NewPostCard("", "just a channel", "A subtitle-only header card title", ""), 180},
		{"bare-title", NewPostCard("", "", "A bare title with no badge row and no meta line", ""), 180},
		{"no-title-line", NewPostCard("SRC", "chan", "", "meta"), 180},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertCardWithinBounds(t, tc.card, tc.width)
		})
	}
}

func TestPostCardDrawPaintsPill(t *testing.T) {
	// The pill fill lands as visible, non-frame pixels near the top-left of the
	// content, proving the badge row draws.
	blue := RGBA{R: 0x20, G: 0x60, B: 0xC0, A: 0xFF}
	c := NewPostCard("SRC", "chan", "Hi", "meta")
	c.PillColor = blue
	const pad = 10
	h := c.Measure(160)
	surfW, surfH := 160+2*pad, h+2*pad
	buf := makeSurface(surfW, surfH)
	c.SetBounds(Rect{X: pad, Y: pad, W: 160, H: h})
	c.Draw(newP(buf, surfW), DefaultLight())
	found := false
	for y := 0; y < surfH && !found; y++ {
		for x := 0; x < surfW; x++ {
			if pixelAt(buf, surfW, x, y) == blue {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("the coloured pill was never painted")
	}
}

// hasColor reports whether c appears anywhere in the buffer.
func hasColor(buf []byte, w int, c RGBA) bool {
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] == c.R && buf[i+1] == c.G && buf[i+2] == c.B && buf[i+3] == c.A {
			return true
		}
	}
	return false
}

func TestPostThumbDraw(t *testing.T) {
	img := RGBA{R: 0x30, G: 0x60, B: 0x90, A: 0xFF} // the colour cardImg fills
	th := DefaultLight()

	// An empty box paints nothing.
	empty := &postThumb{img: cardImg(4, 4)}
	buf := makeSurface(20, 20)
	empty.SetBounds(Rect{X: 0, Y: 0, W: 0, H: 10})
	empty.Draw(newP(buf, 20), th)
	if _, _, maxX, _ := nbPaintedBBox(buf, 20, 20); maxX >= 0 {
		t.Fatal("a zero-width thumbnail box must paint nothing")
	}

	// A nil image leaves only the muted SurfaceAlt ground (no image colour).
	buf = makeSurface(40, 40)
	nilT := &postThumb{}
	nilT.SetBounds(Rect{X: 5, Y: 5, W: 30, H: 30})
	nilT.Draw(newP(buf, 40), th)
	if hasColor(buf, 40, img) {
		t.Fatal("a nil thumbnail must not blit an image")
	}
	if minX, _, maxX, _ := nbPaintedBBox(buf, 40, 40); maxX < 0 || minX < 5 {
		t.Fatal("a nil thumbnail must still paint its muted ground")
	}

	// A zero-extent image is skipped after the ground fill.
	buf = makeSurface(40, 40)
	zt := &postThumb{img: image.NewRGBA(image.Rectangle{})}
	zt.SetBounds(Rect{X: 5, Y: 5, W: 30, H: 30})
	zt.Draw(newP(buf, 40), th)
	if hasColor(buf, 40, img) {
		t.Fatal("a zero-extent thumbnail must not blit")
	}

	// A wide (landscape) image is width-bound: it blits its colour inside the box.
	buf = makeSurface(50, 50)
	wide := &postThumb{img: cardImg(20, 10)}
	wide.SetBounds(Rect{X: 5, Y: 5, W: 40, H: 40})
	wide.Draw(newP(buf, 50), th)
	if !hasColor(buf, 50, img) {
		t.Fatal("a landscape thumbnail should blit its pixels")
	}

	// A tall (portrait) image takes the height-bound branch.
	buf = makeSurface(50, 50)
	tall := &postThumb{img: cardImg(10, 20)}
	tall.SetBounds(Rect{X: 5, Y: 5, W: 40, H: 40})
	tall.Draw(newP(buf, 50), th)
	if !hasColor(buf, 50, img) {
		t.Fatal("a portrait thumbnail should blit its pixels")
	}
}

func TestPostCardReservesThumbPlaceholder(t *testing.T) {
	// A placeholder-only card (media declared, image not yet decoded) reserves the
	// thumbnail column, so the default 72-tall thumb box dominates the 29-tall
	// natural content: contentH=72; +12=84 — the same height a real image yields, so
	// the card does not reflow when the image later lands.
	c := NewPostCard("SRC", "chan", "Hi", "meta")
	c.ThumbPlaceholder = "image"
	if !c.reservesThumb() {
		t.Fatal("a placeholder should reserve the thumbnail column")
	}
	if c.hasThumb() {
		t.Fatal("a placeholder-only card has no decoded image")
	}
	if got := c.Measure(200); got != 84 {
		t.Fatalf("placeholder-reserved card: want 84, got %d", got)
	}
	// The decoded image landing keeps the exact same geometry: no reflow.
	c.Thumbnail = cardImg(10, 10)
	if got := c.Measure(200); got != 84 {
		t.Fatalf("image landing must not reflow: want 84, got %d", got)
	}
	// A bare card with neither image nor placeholder reserves nothing.
	if (&PostCard{}).reservesThumb() {
		t.Fatal("an empty card reserves no thumbnail column")
	}
}

func TestPostThumbPlaceholder(t *testing.T) {
	th := DefaultLight()
	font := (&PostCard{}).metaFont() // package default bitmap font (advance 6, height 7)
	ink := dimInk(th)

	// A fitting label is drawn in dim ink over the muted ground.
	buf := makeSurface(60, 40)
	fit := &postThumb{placeholder: "img", phFont: font}
	fit.SetBounds(Rect{X: 5, Y: 5, W: 50, H: 30})
	fit.Draw(newP(buf, 60), th)
	if !hasColor(buf, 60, ink) {
		t.Fatal("a fitting placeholder label should be drawn in dim ink")
	}

	// No font: the label is skipped, leaving only the ground.
	buf = makeSurface(60, 40)
	noFont := &postThumb{placeholder: "img"}
	noFont.SetBounds(Rect{X: 5, Y: 5, W: 50, H: 30})
	noFont.Draw(newP(buf, 60), th)
	if hasColor(buf, 60, ink) {
		t.Fatal("a placeholder with no font must be skipped")
	}

	// A label wider than the box is skipped (Measure 6*17 > 20).
	buf = makeSurface(30, 40)
	wide := &postThumb{placeholder: "a very long label", phFont: font}
	wide.SetBounds(Rect{X: 2, Y: 2, W: 20, H: 30})
	wide.Draw(newP(buf, 30), th)
	if hasColor(buf, 30, ink) {
		t.Fatal("a label wider than the box must be skipped")
	}

	// A box shorter than the label height (7) is skipped.
	buf = makeSurface(60, 20)
	short := &postThumb{placeholder: "x", phFont: font}
	short.SetBounds(Rect{X: 2, Y: 2, W: 50, H: 5})
	short.Draw(newP(buf, 60), th)
	if hasColor(buf, 60, ink) {
		t.Fatal("a label taller than the box must be skipped")
	}
}

func TestPostThumbDegenerateFit(t *testing.T) {
	th := DefaultLight()
	// A very tall image in a short, wide box drives the width-bound dh to 0,
	// which clamps to 1 (still blits a 1px-tall strip).
	buf := makeSurface(60, 60)
	flat := &postThumb{img: cardImg(1000, 1)}
	flat.SetBounds(Rect{X: 2, Y: 2, W: 40, H: 40})
	flat.Draw(newP(buf, 60), th)
	if !hasColor(buf, 60, RGBA{R: 0x30, G: 0x60, B: 0x90, A: 0xFF}) {
		t.Fatal("a degenerate-fit landscape thumbnail should still blit a clamped strip")
	}
	// A very wide image in a tall, narrow box drives the height-bound dw to 0,
	// which clamps to 1.
	buf = makeSurface(60, 60)
	thin := &postThumb{img: cardImg(1, 1000)}
	thin.SetBounds(Rect{X: 2, Y: 2, W: 40, H: 40})
	thin.Draw(newP(buf, 60), th)
	if !hasColor(buf, 60, RGBA{R: 0x30, G: 0x60, B: 0x90, A: 0xFF}) {
		t.Fatal("a degenerate-fit portrait thumbnail should still blit a clamped strip")
	}
}

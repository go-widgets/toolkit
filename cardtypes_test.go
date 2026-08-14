// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"
)

// The content-card family draws in the default bitmap font: glyph height 7,
// advance 6 (Measure(s) == 6*len(s)). With CardPadX=8, CardPadY=6, CardGapY=4,
// CardLineSpacing=2, CardGapX=6, every Measure below is worked out by hand from
// those constants so a layout change that shifts a card's height is caught.

// cardImg builds an opaque w*h RGBA image (a zero origin, tightly packed) so a
// blit lands visible pixels.
func cardImg(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0x30, 0x60, 0x90, 0xFF
	}
	return img
}

// ---- shared frame helpers (cardframe.go) --------------------------------

func TestWrapText(t *testing.T) {
	f := CurrentFont() // advance 6
	if got := wrapText(f, "   ", 100); got != nil {
		t.Fatalf("all-whitespace should wrap to nil, got %q", got)
	}
	if got := wrapText(f, "abc", 100); len(got) != 1 || got[0] != "abc" {
		t.Fatalf("single fitting word: got %q", got)
	}
	// "aaa"=18, "aaa bbb"=42 > 24 -> break; "bbb"=18, "bbb ccc"=42>24 -> break.
	got := wrapText(f, "aaa bbb ccc", 24)
	if len(got) != 3 {
		t.Fatalf("greedy wrap: want 3 lines, got %d (%q)", len(got), got)
	}
	// A single word wider than the width goes on its own line, not split.
	got = wrapText(f, "aaaaaaaa", 24) // 48 > 24
	if len(got) != 1 || got[0] != "aaaaaaaa" {
		t.Fatalf("over-long word: want it on one line, got %q", got)
	}
}

func TestClampLines(t *testing.T) {
	f := CurrentFont()
	in := []string{"a", "b", "c", "d"}
	if got := clampLines(f, in, 0, 100); len(got) != 4 {
		t.Fatalf("max<=0 must pass through, got %d", len(got))
	}
	if got := clampLines(f, in, 9, 100); len(got) != 4 {
		t.Fatalf("already within max must pass through, got %d", len(got))
	}
	got := clampLines(f, in, 2, 100)
	if len(got) != 2 {
		t.Fatalf("truncate to 2, got %d", len(got))
	}
	if last := []rune(got[1]); last[len(last)-1] != '…' {
		t.Fatalf("truncated last line must end in an ellipsis, got %q", got[1])
	}
}

func TestTextBlockHeight(t *testing.T) {
	f := CurrentFont()
	if got := textBlockHeight(f, 0); got != 0 {
		t.Fatalf("zero lines occupy no height, got %d", got)
	}
	if got := textBlockHeight(f, -3); got != 0 {
		t.Fatalf("negative lines occupy no height, got %d", got)
	}
	if got := textBlockHeight(f, 1); got != 7 {
		t.Fatalf("one line = 7, got %d", got)
	}
	if got := textBlockHeight(f, 3); got != 25 { // 3*7 + 2*2
		t.Fatalf("three lines = 25, got %d", got)
	}
}

func TestStackLayout(t *testing.T) {
	// A leading zero block takes no space and inserts no gap; the two non-zero
	// blocks get one CardGapY between them.
	// block0 zero -> skipped; block1 7 -> y7; block2 5: prev true -> +gap4=11
	// (ys=11), y=16.
	ys, total := stackLayout([]int{0, 7, 5})
	if ys[0] != 0 || ys[1] != 0 || ys[2] != 11 {
		t.Fatalf("ys = %v, want [0 0 11]", ys)
	}
	if total != 16 {
		t.Fatalf("total = %d, want 16", total)
	}
	// All-zero stacks to nothing.
	if _, tot := stackLayout([]int{0, 0, 0}); tot != 0 {
		t.Fatalf("all-zero total = %d, want 0", tot)
	}
	// A zero block BETWEEN two non-zero blocks is skipped, one gap total.
	ys2, tot2 := stackLayout([]int{7, 0, 5}) // 7 -> y7; skip; 5 prev true -> +4=11, y=16
	if ys2[0] != 0 || ys2[2] != 11 || tot2 != 16 {
		t.Fatalf("mid-zero stack ys=%v tot=%d, want [0 _ 11] 16", ys2, tot2)
	}
}

func TestRGBAPixelsFastPathAndRepack(t *testing.T) {
	// Contiguous, zero-origin image: the Pix slice is handed back with no copy.
	img := cardImg(3, 2)
	pix, w, h := rgbaPixels(img)
	if w != 3 || h != 2 || len(pix) != 3*2*4 {
		t.Fatalf("fast path: w,h,len = %d,%d,%d", w, h, len(pix))
	}
	if &pix[0] != &img.Pix[0] {
		t.Fatal("contiguous image should be returned without a copy")
	}
	// Sub-image: non-zero origin + wide stride forces a row-by-row repack.
	big := cardImg(4, 4)
	sub := big.SubImage(image.Rect(1, 1, 3, 3)).(*image.RGBA)
	pix2, w2, h2 := rgbaPixels(sub)
	if w2 != 2 || h2 != 2 || len(pix2) != 2*2*4 {
		t.Fatalf("repack: w,h,len = %d,%d,%d", w2, h2, len(pix2))
	}
	if pix2[3] != 0xFF {
		t.Fatal("repacked pixels lost their alpha")
	}
}

// ---- CardMeta (cardmeta.go) ---------------------------------------------

func TestCardMetaSegmentsHideRules(t *testing.T) {
	// Every field present -> four segments.
	m := NewCardMeta("alice", "3h", 12, 4)
	if got := m.segments(); len(got) != 4 {
		t.Fatalf("all fields: want 4 segments, got %d (%v)", len(got), got)
	}
	// Empty strings and negative counts each hide their field.
	m = NewCardMeta("", "", -1, -1)
	if got := m.segments(); len(got) != 0 {
		t.Fatalf("all hidden: want 0 segments, got %v", got)
	}
	if m.line() != "" {
		t.Fatalf("all-hidden line must be empty, got %q", m.line())
	}
	// Zero score / zero comments are shown (only NEGATIVE hides).
	m = NewCardMeta("", "", 0, 0)
	if got := m.segments(); len(got) != 2 {
		t.Fatalf("zero counts are shown: want 2, got %v", got)
	}
}

func TestCardMetaMeasure(t *testing.T) {
	if got := NewCardMeta("", "", -1, -1).Measure(100); got != 0 {
		t.Fatalf("hidden strip measures 0, got %d", got)
	}
	if got := NewCardMeta("a", "", -1, -1).Measure(100); got != 7 {
		t.Fatalf("shown strip measures one glyph row (7), got %d", got)
	}
}

func TestCardMetaDrawHiddenPaintsNothing(t *testing.T) {
	buf := makeSurface(40, 20)
	m := NewCardMeta("", "", -1, -1) // all hidden
	m.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 20})
	m.Draw(newP(buf, 40), DefaultLight())
	if _, _, _, maxY := nbPaintedBBox(buf, 40, 20); maxY >= 0 {
		t.Fatal("a hidden meta strip must paint nothing")
	}
}

func TestCardMetaDrawElidesAndCentres(t *testing.T) {
	// Bounds far narrower than the strip text: it must elide to width and stay
	// inside, and (H > glyph) vertically centre.
	r := Rect{X: 5, Y: 5, W: 60, H: 21}
	buf := makeSurface(80, 40)
	m := NewCardMeta("a-very-long-author-name", "2026-08-14", 999, 999)
	m.SetBounds(r)
	m.Draw(newP(buf, 80), DefaultLight())
	minX, minY, maxX, maxY := nbPaintedBBox(buf, 80, 40)
	if maxX < 0 {
		t.Fatal("meta strip painted nothing")
	}
	if minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H {
		t.Fatalf("meta strip paints outside bounds %+v: X[%d..%d] Y[%d..%d]", r, minX, maxX, minY, maxY)
	}
	// Vertical centring: the top painted row is below r.Y (a 7px row in a 21px
	// box starts at r.Y + (21-7)/2 = r.Y+7).
	if minY <= r.Y {
		t.Fatalf("centred strip should start below the top, minY=%d r.Y=%d", minY, r.Y)
	}
}

func TestCardMetaDrawTightBoundsNotCentred(t *testing.T) {
	// H == glyph height: no vertical centring branch, top row at r.Y.
	r := Rect{X: 2, Y: 3, W: 200, H: 7}
	buf := makeSurface(220, 20)
	m := NewCardMeta("bob", "1d", 5, 2)
	m.SetBounds(r)
	m.Draw(newP(buf, 220), DefaultLight())
	_, minY, _, maxY := nbPaintedBBox(buf, 220, 20)
	if minY < r.Y || maxY >= r.Y+r.H {
		t.Fatalf("tight strip out of bounds: Y[%d..%d] r.Y=%d H=%d", minY, maxY, r.Y, r.H)
	}
}

// ---- MediaCard (mediacard.go) -------------------------------------------

func TestMediaCardThumbHeight(t *testing.T) {
	if got := (&MediaCard{}).thumbHeight(100); got != 0 {
		t.Fatalf("nil thumbnail => 0 band, got %d", got)
	}
	empty := &MediaCard{Thumbnail: image.NewRGBA(image.Rectangle{})}
	if got := empty.thumbHeight(100); got != 0 {
		t.Fatalf("zero-extent thumbnail => 0 band, got %d", got)
	}
	// 12x6 image at content width 24 => 24*6/12 = 12.
	c := &MediaCard{Thumbnail: cardImg(12, 6)}
	if got := c.thumbHeight(24); got != 12 {
		t.Fatalf("aspect band: want 12, got %d", got)
	}
}

func TestMediaCardMeasure(t *testing.T) {
	// W=40 -> cw=24. Title "Hi" (=12<=24) one line=7. No thumb, no meta.
	c := NewMediaCard("Hi", nil, nil)
	if got := c.Measure(40); got != 19 { // 7 + 2*CardPadY(6)
		t.Fatalf("title-only media card: want 19, got %d", got)
	}
	// Add meta: blocks [0,7,7] -> 7, gap 4, 7 => 18; +12 = 30.
	c.Meta = NewCardMeta("a", "", -1, -1)
	if got := c.Measure(40); got != 30 {
		t.Fatalf("title+meta media card: want 30, got %d", got)
	}
	// Add a 12x6 thumbnail: band 12, gap 4, title 7 => 23; +... meta too.
	// blocks [12,7,7]: 12; +4 title=7 ->y23; +4 meta=7 ->y34; total 34; +12=46.
	c.Thumbnail = cardImg(12, 6)
	if got := c.Measure(40); got != 46 {
		t.Fatalf("thumb+title+meta media card: want 46, got %d", got)
	}
}

func TestMediaCardChildren(t *testing.T) {
	if kids := NewMediaCard("t", nil, nil).Children(); kids != nil {
		t.Fatalf("no meta => nil children, got %v", kids)
	}
	m := NewCardMeta("a", "", -1, -1)
	kids := NewMediaCard("t", nil, m).Children()
	if len(kids) != 1 || kids[0] != m {
		t.Fatalf("meta must be the sole child, got %v", kids)
	}
}

func TestMediaCardDrawWithinBounds(t *testing.T) {
	c := NewMediaCard("A wrapped media headline that needs several lines to fit",
		cardImg(16, 9), NewCardMeta("alice", "3h", 128, 12))
	assertCardWithinBounds(t, c, 180)
	// The nil-thumbnail path (no image band) still fills exactly Measure.
	c2 := NewMediaCard("Short title", nil, nil)
	assertCardWithinBounds(t, c2, 180)
}

// ---- ArticleCard (articlecard.go) ---------------------------------------

func TestArticleCardBodyCap(t *testing.T) {
	if got := (&ArticleCard{}).bodyCap(); got != DefaultArticleBodyLines {
		t.Fatalf("unset BodyLines => default %d, got %d", DefaultArticleBodyLines, got)
	}
	if got := (&ArticleCard{BodyLines: 2}).bodyCap(); got != 2 {
		t.Fatalf("explicit BodyLines respected, got %d", got)
	}
}

func TestArticleCardMeasureClampsBody(t *testing.T) {
	// W=40 -> cw=24. Title "Hi"=7. Body "aa bb cc dd" wraps to 4 lines, clamps
	// to 3 (bodyH = 3*7+2*2 = 25). blocks [7,25,0]: 7; +gap4 body=25 -> y36;
	// total 36; +12 = 48.
	c := NewArticleCard("Hi", "aa bb cc dd", nil)
	if got := c.Measure(40); got != 48 {
		t.Fatalf("clamped-body article: want 48, got %d", got)
	}
	// BodyLines=1 clamps the same body to a single elided line (bodyH=7).
	c.BodyLines = 1
	// blocks [7,7,0]: 7; +4 body=7 -> y18; total 18; +12 = 30.
	if got := c.Measure(40); got != 30 {
		t.Fatalf("one-line-body article: want 30, got %d", got)
	}
}

func TestArticleCardChildren(t *testing.T) {
	if kids := NewArticleCard("t", "b", nil).Children(); kids != nil {
		t.Fatalf("no meta => nil children, got %v", kids)
	}
	m := NewCardMeta("a", "", 1, 1)
	kids := NewArticleCard("t", "b", m).Children()
	if len(kids) != 1 || kids[0] != m {
		t.Fatalf("meta must be the sole child, got %v", kids)
	}
}

func TestArticleCardDrawWithinBounds(t *testing.T) {
	c := NewArticleCard("An article headline",
		"A summary paragraph long enough that it wraps over several lines and is "+
			"then cut to the body-line cap with a trailing ellipsis to mark it.",
		NewCardMeta("bob", "1d", 42, 7))
	assertCardWithinBounds(t, c, 200)
}

// ---- LinkCard (linkcard.go) ---------------------------------------------

func TestLinkCardHasFavicon(t *testing.T) {
	if (&LinkCard{}).hasFavicon() {
		t.Fatal("nil favicon => false")
	}
	if (&LinkCard{Favicon: image.NewRGBA(image.Rectangle{})}).hasFavicon() {
		t.Fatal("zero-extent favicon => false")
	}
	if !(&LinkCard{Favicon: cardImg(8, 8)}).hasFavicon() {
		t.Fatal("valid favicon => true")
	}
}

func TestLinkCardMeasure(t *testing.T) {
	// W=40 -> cw=24. No favicon: colW=24. Title "Hi"=1 line(7), domain shown(7):
	// colH stack[7,7] = 18 = headH; no meta -> total 18; +12 = 30.
	c := NewLinkCard(nil, "Hi", "x.com", nil)
	if got := c.Measure(40); got != 30 {
		t.Fatalf("no-favicon link card: want 30, got %d", got)
	}
	// With favicon: fav=7, colW=24-7-6=11. Title "Hi"(=12>11) one over-long line
	// (7). Domain shown (7). colH=18; headH=max(7,18)=18; total 18; +12=30.
	c.Favicon = cardImg(8, 8)
	if got := c.Measure(40); got != 30 {
		t.Fatalf("favicon link card: want 30, got %d", got)
	}
	// No domain: domainH=0. colH stack[7,0]=7; headH=max(fav7,7)=7; +12=19.
	c.Domain = ""
	if got := c.Measure(40); got != 19 {
		t.Fatalf("no-domain link card: want 19, got %d", got)
	}
}

func TestLinkCardMeasureNarrowClampsColumn(t *testing.T) {
	// W=20 -> cw=4; with a favicon colW = 4-7-6 = -9, clamped to 1. Must not
	// panic and must produce a positive height.
	c := NewLinkCard(cardImg(8, 8), "Hi", "x.com", nil)
	if got := c.Measure(20); got <= 12 {
		t.Fatalf("narrow link card should still stack content, got %d", got)
	}
}

func TestLinkCardChildren(t *testing.T) {
	if kids := NewLinkCard(nil, "t", "d", nil).Children(); kids != nil {
		t.Fatalf("no meta => nil children, got %v", kids)
	}
	m := NewCardMeta("a", "", -1, 3)
	kids := NewLinkCard(nil, "t", "d", m).Children()
	if len(kids) != 1 || kids[0] != m {
		t.Fatalf("meta must be the sole child, got %v", kids)
	}
}

func TestLinkCardDrawWithinBounds(t *testing.T) {
	// Favicon + domain + meta: exercises every drawn block.
	c := NewLinkCard(cardImg(16, 16),
		"A shared link with a title that wraps to a couple of lines",
		"example.com", NewCardMeta("carol", "5h", 88, 3))
	assertCardWithinBounds(t, c, 200)
	// No favicon, no domain, no meta: the bare title-only path.
	c2 := NewLinkCard(nil, "Bare link title", "", nil)
	assertCardWithinBounds(t, c2, 200)
}

// ---- shared bounds assertion --------------------------------------------

// assertCardWithinBounds draws a card whose Bounds().H is set to Measure(width)
// — the caller contract — into a padded surface and fails if it paints a single
// pixel outside those bounds.
func assertCardWithinBounds(t *testing.T, c interface {
	Widget
	Measure(int) int
}, width int) {
	t.Helper()
	const pad = 20
	h := c.Measure(width)
	if h <= 0 {
		t.Fatalf("card measured non-positive height %d", h)
	}
	r := Rect{X: pad, Y: pad, W: width, H: h}
	surfW, surfH := width+2*pad, h+2*pad
	buf := makeSurface(surfW, surfH)
	c.SetBounds(r)
	c.Draw(newP(buf, surfW), DefaultLight())
	minX, minY, maxX, maxY := nbPaintedBBox(buf, surfW, surfH)
	if maxX < 0 {
		t.Fatal("card painted nothing")
	}
	if minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H {
		t.Fatalf("card paints outside bounds %+v: X[%d..%d] Y[%d..%d]", r, minX, maxX, minY, maxY)
	}
}

// Draw must position content at the same height Measure reports: draw once at
// Measure(W) height and confirm the painted bottom reaches into the last inset
// row rather than stopping short (which would prove Draw and Measure diverged).
func TestCardMeasureDrawAgree(t *testing.T) {
	c := NewArticleCard("Title here", "body one two three four five six", nil)
	const width = 120
	h := c.Measure(width)
	buf := makeSurface(width, h)
	c.SetBounds(Rect{X: 0, Y: 0, W: width, H: h})
	c.Draw(newP(buf, width), DefaultLight())
	_, _, _, maxY := nbPaintedBBox(buf, width, h)
	// The frame stroke sits on the last row (h-1); content never exceeds it.
	if maxY != h-1 {
		t.Fatalf("painted bottom %d, want the frame's last row %d", maxY, h-1)
	}
}

// A wrapped line that is STILL wider than its column (a single unbreakable word
// longer than the width) must be ellipsised at draw time by drawTextBlock and
// stay inside the card — the branch a normal multi-word paragraph never hits.
func TestCardDrawEllipsisesOverlongWord(t *testing.T) {
	c := NewArticleCard("supercalifragilisticexpialidocious-and-then-some",
		"antidisestablishmentarianism-taken-far-past-any-reasonable-column-width",
		nil)
	assertCardWithinBounds(t, c, 120)
}

// A favicon taller than an empty text column drives the head-row height off the
// favicon side (the fav > colH branch): title and domain both empty.
func TestLinkCardFaviconDrivesHeadHeight(t *testing.T) {
	c := NewLinkCard(cardImg(8, 8), "", "", nil)
	// cw = 40-16 = 24; colH = 0; headH = fav = 7; total 7; +12 = 19.
	if got := c.Measure(40); got != 19 {
		t.Fatalf("favicon-only link card: want 19, got %d", got)
	}
	assertCardWithinBounds(t, c, 40)
}

// Every card in the family answers A11y() with a stable role and its title/text
// so CollectA11y announces it (the TestEveryWidgetIsAccessible gate).
func TestCardFamilyA11y(t *testing.T) {
	if a := NewCardMeta("alice", "3h", 1, 2).A11y(); a.Role != RoleText || a.Name == "" {
		t.Fatalf("CardMeta A11y = %+v, want text with a byline", a)
	}
	if a := NewMediaCard("Clip", nil, nil).A11y(); a.Role != RoleGroup || a.Name != "Clip" {
		t.Fatalf("MediaCard A11y = %+v", a)
	}
	if a := NewArticleCard("Story", "b", nil).A11y(); a.Role != RoleGroup || a.Name != "Story" {
		t.Fatalf("ArticleCard A11y = %+v", a)
	}
	a := NewLinkCard(nil, "Post", "example.com", nil).A11y()
	if a.Role != RoleGroup || a.Name != "Post" || a.Value != "example.com" {
		t.Fatalf("LinkCard A11y = %+v", a)
	}
}

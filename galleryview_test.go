// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"
)

// gvLongLabel is wider than the caption band (400-2*20 = 360px at 6px/glyph),
// so selecting item 2 forces the caption to elide.
const gvLongLabel = "A gallery caption that is far too wide to ever fit inside the preview caption band"

// sampleGallery builds an 8-item gallery: an image item, a raster item with NO
// image (so its light chip fills the preview), a glyph item with a too-wide
// label (glyph preview + caption elision), then five plain glyph items.
func sampleGallery() *GalleryView {
	items := []GalleryItem{
		{Image: solidIcon(RGB(0x40, 0x10, 0x10)), Label: "Zero", Key: "k0"},
		{Label: "One", Key: "k1", Raster: true},
		{Label: gvLongLabel, Key: "k2"},
	}
	for i := 3; i < 8; i++ {
		items = append(items, GalleryItem{Label: fmt.Sprintf("Item %d", i), Key: fmt.Sprintf("k%d", i)})
	}
	return NewGalleryView(items...)
}

// --- construction / normalization ---------------------------------------

func TestGalleryNewSelectsFirst(t *testing.T) {
	g := sampleGallery()
	if g.Selected().Get() != 0 {
		t.Fatalf("new gallery selected %d, want 0", g.Selected().Get())
	}
	if empty := NewGalleryView(); empty.Selected().Get() != -1 {
		t.Fatalf("empty gallery selected %d, want -1", empty.Selected().Get())
	}
}

func TestGallerySetItemsNormalizes(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(5)
	// Fewer items: the out-of-range selection snaps to the last item.
	g.SetItems([]GalleryItem{{Label: "a"}, {Label: "b"}, {Label: "c"}})
	if g.Selected().Get() != 2 {
		t.Fatalf("after shrink selected %d, want 2", g.Selected().Get())
	}
	// Clear then repopulate: an unset selection snaps back to the first item.
	g.SetItems(nil)
	if g.Selected().Get() != -1 {
		t.Fatalf("empty selected %d, want -1", g.Selected().Get())
	}
	g.SetItems([]GalleryItem{{Label: "x"}, {Label: "y"}})
	if g.Selected().Get() != 0 {
		t.Fatalf("repopulated selected %d, want 0", g.Selected().Get())
	}
}

func TestGallerySetSelectedValidation(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(3)
	if g.Selected().Get() != 3 {
		t.Fatalf("valid SetSelected = %d, want 3", g.Selected().Get())
	}
	g.SetSelected(99) // out of range clears
	if g.Selected().Get() != -1 {
		t.Fatalf("invalid SetSelected = %d, want -1", g.Selected().Get())
	}
}

// --- geometry ------------------------------------------------------------

func TestGalleryGeometry(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	if got, want := g.PreviewRect(), (Rect{X: 0, Y: 0, W: 400, H: 280}); got != want {
		t.Fatalf("PreviewRect = %+v, want %+v", got, want)
	}
	if got, want := g.StripRect(), (Rect{X: 0, Y: 280, W: 400, H: 120}); got != want {
		t.Fatalf("StripRect = %+v, want %+v", got, want)
	}
	if got, want := g.captionRect(), (Rect{X: 0, Y: 252, W: 400, H: 28}); got != want {
		t.Fatalf("captionRect = %+v, want %+v", got, want)
	}
	if got, want := g.previewImageRect(), (Rect{X: 20, Y: 20, W: 360, H: 212}); got != want {
		t.Fatalf("previewImageRect = %+v, want %+v", got, want)
	}
	if g.thumbSize() != 100 || g.cellW() != 112 {
		t.Fatalf("thumbSize/cellW = %d/%d, want 100/112", g.thumbSize(), g.cellW())
	}

	// At sel 0 the strip is anchored at 0, so thumb 0 sits at the left pad.
	r0, ok0 := g.ThumbRect(0)
	if !ok0 || r0 != (Rect{X: 10, Y: 290, W: 100, H: 100}) {
		t.Fatalf("ThumbRect(0) = %+v ok=%v, want {10,290,100,100} true", r0, ok0)
	}
	r1, _ := g.ThumbRect(1)
	if r1 != (Rect{X: 122, Y: 290, W: 100, H: 100}) {
		t.Fatalf("ThumbRect(1) = %+v, want {122,290,100,100}", r1)
	}
	if _, ok := g.ThumbRect(-1); ok {
		t.Fatalf("ThumbRect(-1) ok, want false")
	}
	if _, ok := g.ThumbRect(99); ok {
		t.Fatalf("ThumbRect(99) ok, want false")
	}
}

// --- preview rendering ---------------------------------------------------

func TestGalleryPreviewImage(t *testing.T) {
	g := sampleGallery() // sel 0 = image item
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)

	// The 1×1 image fills the fitted preview square {94,20,212,212}; its centre
	// is the image colour.
	imgC := RGB(0x40, 0x10, 0x10)
	if got := pixelAt(buf, 400, 200, 126); got != imgC {
		t.Fatalf("preview image centre = %+v, want %+v", got, imgC)
	}
	// The body above the preview padding is the Surface fill.
	if got := pixelAt(buf, 400, 200, 5); got != th.Surface {
		t.Fatalf("body fill = %+v, want Surface %+v", got, th.Surface)
	}
}

func TestGalleryPreviewRasterChipDark(t *testing.T) {
	g := sampleGallery()
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(1) // raster, no image → chip fills the preview square
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)
	if got := pixelAt(buf, 400, 200, 126); got != chipColor(th) {
		t.Fatalf("dark raster preview centre = %+v, want chip %+v", got, chipColor(th))
	}
}

func TestGalleryPreviewRasterChipLight(t *testing.T) {
	g := sampleGallery()
	th := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(1)
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)
	if got, want := pixelAt(buf, 400, 200, 126), RGB(0xFF, 0xFF, 0xFF); got != want {
		t.Fatalf("light raster preview centre = %+v, want white %+v", got, want)
	}
}

func TestGalleryPreviewGlyph(t *testing.T) {
	g := sampleGallery()
	th := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(2) // glyph item (no image, not raster)
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)

	// The document page fills the square with SurfaceAlt; sample above the first
	// page line (y=73) to avoid the muted ink lines.
	if got := pixelAt(buf, 400, 200, 40); got != th.SurfaceAlt {
		t.Fatalf("glyph page fill = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// The middle page line (y=126) is drawn in muted ink.
	if got := pixelAt(buf, 400, 150, 126); got != mutedInk(th) {
		t.Fatalf("glyph page line = %+v, want mutedInk %+v", got, mutedInk(th))
	}
}

func TestGalleryCaption(t *testing.T) {
	g := sampleGallery() // sel 0 → "Zero"
	th := DefaultLight()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)

	tw := TextWidth("Zero")
	lx := (400 - tw) / 2
	ly := 252 + (28-GlyphHeight())/2
	if !anyInkAround(buf, 400, lx, ly, tw, th.OnSurface) {
		t.Fatalf("caption 'Zero' not painted")
	}

	// A too-wide label elides but still paints (exercises the elide branch).
	g.SetSelected(2)
	buf2 := makeSurface(400, 400)
	g.Draw(newP(buf2, 400), th)
	if !anyInkAround(buf2, 400, 40, ly, 320, th.OnSurface) {
		t.Fatalf("elided caption not painted")
	}
}

func TestGalleryPreviewBlankWhenCleared(t *testing.T) {
	g := sampleGallery()
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(-1) // clear selection: preview goes blank
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th) // must not panic
	// The preview area shows only the Surface fill, no image/chip/glyph.
	if got := pixelAt(buf, 400, 200, 126); got != th.Surface {
		t.Fatalf("blank preview centre = %+v, want Surface %+v", got, th.Surface)
	}
}

// --- filmstrip rendering + selection ring --------------------------------

func TestGalleryStripBandAndSelectionRing(t *testing.T) {
	g := sampleGallery() // sel 0, strip anchored at 0
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)

	// The strip band is filled with SurfaceAlt (sample a spot between thumbs,
	// clear of the selection field around thumb 0).
	if got := pixelAt(buf, 400, 116, 300); got != th.SurfaceAlt {
		t.Fatalf("strip band = %+v, want SurfaceAlt %+v", got, th.SurfaceAlt)
	}
	// The band's top hairline is the Border colour.
	if got := pixelAt(buf, 400, 200, 280); got != th.Border {
		t.Fatalf("strip top hairline = %+v, want Border %+v", got, th.Border)
	}
	// The selected thumb (0) carries an accent ring: its field is thumb 0 grown
	// by gvRing → {7,287,106,106}; sample the straight top edge middle.
	if got := pixelAt(buf, 400, 60, 287); got != th.Accent {
		t.Fatalf("selection ring = %+v, want Accent %+v", got, th.Accent)
	}
}

func TestGalleryThumbnailChipAndImage(t *testing.T) {
	g := sampleGallery()
	th := DefaultDark()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), th)

	// Thumb 0 (image item) centre = image colour. Thumb 0 = {10,290,100,100}.
	imgC := RGB(0x40, 0x10, 0x10)
	if got := pixelAt(buf, 400, 60, 340); got != imgC {
		t.Fatalf("thumb0 image centre = %+v, want %+v", got, imgC)
	}
	// Thumb 1 (raster, no image) centre = chip. Thumb 1 = {122,290,100,100}.
	if got := pixelAt(buf, 400, 172, 340); got != chipColor(th) {
		t.Fatalf("thumb1 chip centre = %+v, want %+v", got, chipColor(th))
	}
}

// --- auto-scroll ---------------------------------------------------------

func TestGalleryAutoScrollCentersSelection(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})

	// Select a middle item: the strip centres it (not clamped).
	g.SetSelected(4)
	if g.stripScroll != 308 {
		t.Fatalf("sel 4 stripScroll = %d, want 308", g.stripScroll)
	}
	r4, ok := g.ThumbRect(4)
	if !ok || r4.X+r4.W/2 != 200 {
		t.Fatalf("thumb4 centre X = %d, want 200 (ok=%v)", r4.X+r4.W/2, ok)
	}

	// Select the last item: the strip clamps at the maximum scroll, but the
	// thumbnail is still fully visible in the band.
	g.SetSelected(7)
	if g.stripScroll != 504 {
		t.Fatalf("sel 7 stripScroll = %d, want 504 (max)", g.stripScroll)
	}
	r7, ok := g.ThumbRect(7)
	sr := g.StripRect()
	if !ok || r7.X < sr.X || r7.X+r7.W > sr.X+sr.W {
		t.Fatalf("thumb7 %+v not fully visible in %+v", r7, sr)
	}

	// Thumb 0 is now scrolled off the left edge (not visible → drawStrip skips).
	if _, ok0 := g.ThumbRect(0); ok0 {
		t.Fatalf("thumb0 visible after scroll, want off-screen")
	}
	buf := makeSurface(400, 400)
	g.Draw(newP(buf, 400), DefaultDark()) // exercises the off-screen skip
}

func TestGalleryClampStripScroll(t *testing.T) {
	g := sampleGallery() // max scroll 504
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if got := g.clampStripScroll(-5); got != 0 {
		t.Fatalf("clamp(-5) = %d, want 0", got)
	}
	if got := g.clampStripScroll(9999); got != 504 {
		t.Fatalf("clamp(9999) = %d, want 504", got)
	}
	if got := g.clampStripScroll(100); got != 100 {
		t.Fatalf("clamp(100) = %d, want 100", got)
	}
	// A gallery whose content fits the band has a zero max scroll.
	small := NewGalleryView(GalleryItem{Label: "solo"})
	small.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if got := small.clampStripScroll(50); got != 0 {
		t.Fatalf("solo clamp(50) = %d, want 0", got)
	}
}

// --- keyboard ------------------------------------------------------------

func TestGalleryKeyboardNavigation(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	var selects []int
	g.Selected().Subscribe(func(i int) { selects = append(selects, i) })

	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // 0 -> 1
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Right"})      // 1 -> 2 (alias)
	if g.Selected().Get() != 2 {
		t.Fatalf("after two rights sel=%d, want 2", g.Selected().Get())
	}
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // 2 -> 1
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Left"})      // 1 -> 0
	if g.Selected().Get() != 0 {
		t.Fatalf("after two lefts sel=%d, want 0", g.Selected().Get())
	}
	// Left at index 0 is a no-op and fires no OnSelect.
	before := len(selects)
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Left"})
	if g.Selected().Get() != 0 || len(selects) != before {
		t.Fatalf("left at 0 changed state: sel=%d selects=%v", g.Selected().Get(), selects)
	}
	// End / Home jump to the ends.
	g.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if g.Selected().Get() != 7 {
		t.Fatalf("End sel=%d, want 7", g.Selected().Get())
	}
	// Right at the last index is a no-op.
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Right"})
	if g.Selected().Get() != 7 {
		t.Fatalf("right at end sel=%d, want 7", g.Selected().Get())
	}
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Home"})
	if g.Selected().Get() != 0 {
		t.Fatalf("Home sel=%d, want 0", g.Selected().Get())
	}
	// An unhandled key does nothing.
	g.OnEvent(Event{Kind: EventKeyDown, Code: "PageDown"})
	if g.Selected().Get() != 0 {
		t.Fatalf("PageDown moved selection to %d", g.Selected().Get())
	}
	if want := []int{1, 2, 1, 0, 7, 0}; fmt.Sprint(selects) != fmt.Sprint(want) {
		t.Fatalf("OnSelect sequence = %v, want %v", selects, want)
	}
}

func TestGalleryKeyboardFromCleared(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.SetSelected(-1)
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"}) // -1 -> 0
	if g.Selected().Get() != 0 {
		t.Fatalf("right from cleared = %d, want 0", g.Selected().Get())
	}
	g.SetSelected(-1)
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"}) // -1 -> 0
	if g.Selected().Get() != 0 {
		t.Fatalf("left from cleared = %d, want 0", g.Selected().Get())
	}
}

func TestGalleryActivate(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	activated := -9
	g.OnActivate = func(i int) { activated = i }
	g.SetSelected(3)
	for _, code := range []string{"Enter", "Return", " ", "Space"} {
		activated = -9
		g.OnEvent(Event{Kind: EventKeyDown, Code: code})
		if activated != 3 {
			t.Fatalf("%q activated %d, want 3", code, activated)
		}
	}
	// Enter with the selection cleared fires nothing.
	g.SetSelected(-1)
	activated = -9
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if activated != -9 {
		t.Fatalf("Enter while cleared activated %d, want none", activated)
	}
}

func TestGalleryActivateNilCallback(t *testing.T) {
	g := sampleGallery() // OnActivate nil
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // must not panic
	if g.Selected().Get() != 0 {
		t.Fatalf("nil-activate changed selection to %d", g.Selected().Get())
	}
}

// --- click ---------------------------------------------------------------

func TestGalleryClickSelectActivate(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	var selected, activated = -9, -9
	g.Selected().Subscribe(func(i int) { selected = i })
	g.OnActivate = func(i int) { activated = i }

	// Thumb 2 centre = {234,290,100,100} → (284,340).
	g.OnEvent(Event{Kind: EventClick, X: 284, Y: 340})
	if g.Selected().Get() != 2 || selected != 2 {
		t.Fatalf("click thumb2: sel=%d cb=%d, want 2/2", g.Selected().Get(), selected)
	}
	// Selecting recenters the strip, so re-click where thumb 2 sits now.
	r2, _ := g.ThumbRect(2)
	selected = -9
	g.OnEvent(Event{Kind: EventClick, X: r2.X + r2.W/2, Y: r2.Y + r2.H/2})
	if activated != 2 || selected != -9 {
		t.Fatalf("re-click: activated=%d selected=%d, want 2/-9", activated, selected)
	}
}

func TestGalleryClickMisses(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	// Click in the preview region (above the strip) selects nothing new.
	g.OnEvent(Event{Kind: EventClick, X: 200, Y: 100})
	if g.Selected().Get() != 0 {
		t.Fatalf("preview click changed selection to %d", g.Selected().Get())
	}
	// Click in the gap between thumb 0 (ends x=110) and thumb 1 (starts 122).
	g.OnEvent(Event{Kind: EventClick, X: 115, Y: 340})
	if g.Selected().Get() != 0 {
		t.Fatalf("gap click changed selection to %d", g.Selected().Get())
	}
}

func TestGalleryThumbAtEdges(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if idx := g.ThumbAt(5, 340); idx != -1 { // left of the leading pad (local < 0)
		t.Fatalf("ThumbAt left pad = %d, want -1", idx)
	}
	if idx := g.ThumbAt(60, 100); idx != -1 { // above the strip band
		t.Fatalf("ThumbAt above strip = %d, want -1", idx)
	}
	// A two-item gallery: a click past the last thumb maps to i >= len.
	small := NewGalleryView(GalleryItem{Label: "a"}, GalleryItem{Label: "b"})
	small.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	if idx := small.ThumbAt(300, 340); idx != -1 { // local 290 -> cell 2 >= len 2
		t.Fatalf("ThumbAt past last = %d, want -1", idx)
	}
	if idx := small.ThumbAt(60, 340); idx != 0 { // thumb 0 body
		t.Fatalf("ThumbAt thumb0 = %d, want 0", idx)
	}
}

func TestGalleryClickNilOnSelect(t *testing.T) {
	g := NewGalleryView(GalleryItem{Label: "a"}, GalleryItem{Label: "b"}) // OnSelect nil
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.OnEvent(Event{Kind: EventClick, X: 172, Y: 340}) // select thumb 1
	if g.Selected().Get() != 1 {
		t.Fatalf("nil-OnSelect click sel=%d, want 1", g.Selected().Get())
	}
}

func TestGalleryClickNilOnActivate(t *testing.T) {
	g := NewGalleryView(GalleryItem{Label: "a"}, GalleryItem{Label: "b"}) // OnActivate nil
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.OnEvent(Event{Kind: EventClick, X: 60, Y: 340}) // select thumb 0
	g.OnEvent(Event{Kind: EventClick, X: 60, Y: 340}) // re-click: activate, nil cb
	if g.Selected().Get() != 0 {
		t.Fatalf("nil-OnActivate re-click sel=%d, want 0", g.Selected().Get())
	}
}

// --- empty / disabled / a11y --------------------------------------------

func TestGalleryEmptyState(t *testing.T) {
	th := DefaultLight()
	w, h := 300, 200

	g := NewGalleryView()
	g.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	g.Draw(newP(buf, w), th)
	def := TextWidth("No items")
	dx, dy := (w-def)/2, h/2-GlyphHeight()/2
	if !anyInkAround(buf, w, dx, dy, def, mutedInk(th)) {
		t.Fatalf("default empty message not painted")
	}

	g2 := NewGalleryView()
	g2.Empty = "Nothing selected"
	g2.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf2 := makeSurface(w, h)
	g2.Draw(newP(buf2, w), th)
	cw := TextWidth("Nothing selected")
	if !anyInkAround(buf2, w, (w-cw)/2, dy, cw, mutedInk(th)) {
		t.Fatalf("custom empty message not painted")
	}

	// An empty gallery ignores clicks and keys.
	g.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	if g.Selected().Get() != -1 {
		t.Fatalf("empty gallery selected %d, want -1", g.Selected().Get())
	}
}

func TestGalleryDisabledInert(t *testing.T) {
	g := sampleGallery()
	g.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 400})
	g.Disabled = true
	g.OnEvent(Event{Kind: EventClick, X: 284, Y: 340})
	g.OnEvent(Event{Kind: EventKeyDown, Code: "End"})
	if g.Selected().Get() != 0 {
		t.Fatalf("disabled gallery selection moved to %d, want 0", g.Selected().Get())
	}
}

func TestGalleryA11y(t *testing.T) {
	g := sampleGallery()
	if got := g.A11y(); got.Role != RoleGrid || got.Value != "Zero" {
		t.Fatalf("A11y = %+v, want grid/Zero", got)
	}
	g.SetSelected(99) // clears
	if got := g.A11y(); got.Role != RoleGrid || got.Value != "" {
		t.Fatalf("cleared A11y = %+v, want grid with empty value", got)
	}
}

// --- degenerate sizes / helpers -----------------------------------------

func TestGalleryTinyBoundsNoPanic(t *testing.T) {
	g := sampleGallery()
	g.SetSelected(2) // glyph preview path with a degenerate area
	g.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 30})
	if g.thumbSize() != 1 {
		t.Fatalf("tiny thumbSize = %d, want 1 (floored)", g.thumbSize())
	}
	g.Draw(newP(makeSurface(40, 30), 40), DefaultDark()) // must not panic
}

func TestGallerySquareIn(t *testing.T) {
	// Wide rect: the square takes the (smaller) height and centres horizontally.
	if got, want := squareIn(Rect{X: 0, Y: 0, W: 200, H: 100}), (Rect{X: 50, Y: 0, W: 100, H: 100}); got != want {
		t.Fatalf("squareIn wide = %+v, want %+v", got, want)
	}
	// Tall rect: the square takes the (smaller) width and centres vertically.
	if got, want := squareIn(Rect{X: 0, Y: 0, W: 100, H: 200}), (Rect{X: 0, Y: 50, W: 100, H: 100}); got != want {
		t.Fatalf("squareIn tall = %+v, want %+v", got, want)
	}
}

func TestGalleryExpandRect(t *testing.T) {
	if got, want := expandRect(Rect{X: 10, Y: 20, W: 30, H: 40}, 3), (Rect{X: 7, Y: 17, W: 36, H: 46}); got != want {
		t.Fatalf("expandRect = %+v, want %+v", got, want)
	}
}

// ExampleGalleryView builds a small gallery, moves the selection with a key and
// reports the current item.
func ExampleGalleryView() {
	g := NewGalleryView(
		GalleryItem{Label: "Sunset.jpg", Key: "sunset", Raster: true},
		GalleryItem{Label: "Notes.txt", Key: "notes"},
	)
	g.SetBounds(Rect{X: 0, Y: 0, W: 320, H: 240})
	g.Draw(newP(makeSurface(320, 240), 320), DefaultLight())
	g.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowRight"})
	fmt.Printf("selected item %d\n", g.Selected().Get())
	// Output: selected item 1
}

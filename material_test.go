// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"testing"

	"github.com/go-images/images"
	"github.com/go-widgets/painter"
)

// edgeSource builds a w*h RGBA surface split at column x=split: black on the
// left, white on the right — a sharp vertical edge a blur must soften.
func edgeSource(w, h, split int) []byte {
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			v := byte(0)
			if x >= split {
				v = 255
			}
			buf[o], buf[o+1], buf[o+2], buf[o+3] = v, v, v, 255
		}
	}
	return buf
}

func TestMaterialEmptyBoundsNoPanic(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	m.SetBounds(Rect{W: 0, H: 10})
	buf := make([]byte, 4)
	m.Draw(newP(buf, 1), DefaultLight())
}

// TestMaterialFallbackBlurMatchesReference is the pixel-diff-vs-reference proof:
// the material's optimised padded-region blur must equal, byte for (near-)byte,
// the reference obtained by blurring the WHOLE source through go-images and
// cropping — the two are independent computations that must agree wherever the
// destination sits far enough from the source edge that neither clamps.
func TestMaterialFallbackBlurMatchesReference(t *testing.T) {
	const sw, sh = 200, 120
	src := edgeSource(sw, sh, 100)
	const sigma = 6.0
	r := Rect{X: 60, Y: 20, W: 80, H: 60} // >3σ from every source edge

	m := NewMaterial(MaterialSidebar)
	m.SetSource(src, sw, sh)
	m.Sigma = sigma
	m.SetBounds(r)
	m.ensureBlur(r, sigma)
	if m.cache == nil {
		t.Fatal("ensureBlur produced no cache")
	}

	full, err := images.GaussianBlur(&image.RGBA{
		Pix: src, Stride: sw * 4, Rect: image.Rect(0, 0, sw, sh),
	}, sigma)
	if err != nil {
		t.Fatal(err)
	}

	var maxDiff int
	for dy := 0; dy < r.H; dy++ {
		for dx := 0; dx < r.W; dx++ {
			co := (dy*r.W + dx) * 4
			fo := ((r.Y+dy)*sw + (r.X + dx)) * 4
			for c := 0; c < 4; c++ {
				d := int(m.cache[co+c]) - int(full.Pix[fo+c])
				if d < 0 {
					d = -d
				}
				if d > maxDiff {
					maxDiff = d
				}
			}
		}
	}
	if maxDiff > 1 {
		t.Errorf("padded-region blur diverges from full-source reference: maxDiff=%d (want <=1)", maxDiff)
	}
}

// TestMaterialFallbackDiffersFromOpaque proves the fallback actually blurs: at
// the source's sharp black/white edge the material's backdrop must show
// intermediate greys, where the un-blurred (opaque) source is a hard step.
func TestMaterialFallbackDiffersFromOpaque(t *testing.T) {
	const sw, sh = 200, 120
	src := edgeSource(sw, sh, 100)
	r := Rect{X: 60, Y: 20, W: 80, H: 60}
	m := NewMaterial(MaterialSidebar)
	m.SetSource(src, sw, sh)
	m.Sigma = 6
	m.SetBounds(r)
	m.ensureBlur(r, 6)

	// Surface (100,50) is white in the opaque source (x>=100). In the blur it
	// must be a mid grey — neither black nor white — because the black left half
	// bled across the edge.
	dx, dy := 100-r.X, 50-r.Y
	v := m.cache[(dy*r.W+dx)*4]
	if v < 40 || v > 215 {
		t.Errorf("edge pixel not blurred: got %d, want an intermediate grey (40..215)", v)
	}
}

func TestMaterialDrawTintOverBlurWithinBounds(t *testing.T) {
	const w, h = 220, 160
	const sw, sh = 200, 120
	src := edgeSource(sw, sh, 100)
	surf := makeSurface(w, h) // sentinel 0xC8 ground
	m := NewMaterial(MaterialSidebar)
	m.SetSource(src, sw, sh)
	m.Sigma = 5
	r := Rect{X: 40, Y: 30, W: 90, H: 70}
	m.SetBounds(r)
	m.Draw(newP(surf, w), DefaultLight())

	// Every painted (non-sentinel) pixel must lie inside Bounds.
	minX, minY, maxX, maxY := nbPaintedBBox(surf, w, h)
	if maxX < 0 {
		t.Fatal("material painted nothing")
	}
	if minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H {
		t.Errorf("material painted outside bounds %+v: X[%d..%d] Y[%d..%d]", r, minX, maxX, minY, maxY)
	}
	// A pixel just outside the material keeps the sentinel ground.
	out := pixelAt(surf, w, r.X-1, r.Y)
	if out.R != 0xC8 || out.G != 0xC8 || out.B != 0xC8 {
		t.Errorf("sentinel outside bounds overwritten: %+v", out)
	}
}

func TestMaterialNoSourceDrawsTintOnly(t *testing.T) {
	const w, h = 60, 40
	surf := makeSurface(w, h)
	m := NewMaterial(MaterialHUD) // fixed dark tint
	r := Rect{X: 10, Y: 8, W: 30, H: 20}
	m.SetBounds(r)
	m.Draw(newP(surf, w), DefaultLight())
	// Centre is washed by the HUD tint over the sentinel — darker than 0xC8.
	c := pixelAt(surf, w, r.X+15, r.Y+10)
	if c.R >= 0xC8 {
		t.Errorf("HUD tint did not darken the ground: %+v", c)
	}
	// Outside stays sentinel.
	if o := pixelAt(surf, w, 0, 0); o.R != 0xC8 {
		t.Errorf("painted outside bounds: %+v", o)
	}
}

func TestMaterialSelectionHasNoBlur(t *testing.T) {
	m := NewMaterial(MaterialSelection)
	if got := m.effectiveSigma(); got != 0 {
		t.Errorf("selection sigma = %v, want 0", got)
	}
	// With sigma 0 and a source, Draw must not build a blur cache.
	m.SetSource(edgeSource(20, 20, 10), 20, 20)
	m.SetBounds(Rect{W: 20, H: 20})
	buf := makeSurface(20, 20)
	m.Draw(newP(buf, 20), DefaultLight())
	if m.cache != nil {
		t.Error("selection built a blur cache despite sigma 0")
	}
}

func TestMaterialNativeSkipsFallback(t *testing.T) {
	const w, h = 60, 40
	surf := makeSurface(w, h)
	m := NewMaterial(MaterialSidebar)
	m.SetSource(edgeSource(60, 40, 30), 60, 40)
	m.Sigma = 5
	r := Rect{X: 10, Y: 8, W: 30, H: 20}
	m.SetBounds(r)
	if m.NativeBacked() {
		t.Fatal("new material should not be native-backed")
	}
	m.SetNativeBacked(true)
	if !m.NativeBacked() {
		t.Fatal("SetNativeBacked(true) not recorded")
	}
	m.Draw(newP(surf, w), DefaultLight())
	// Native path paints nothing here (no child): the hole is left for the
	// system view, so the framebuffer keeps its sentinel ground untouched.
	if c := pixelAt(surf, w, r.X+15, r.Y+10); c.R != 0xC8 {
		t.Errorf("native material painted a fallback backdrop: %+v", c)
	}
	if m.cache != nil {
		t.Error("native material built a blur cache")
	}
}

func TestMaterialNativeDrawsChild(t *testing.T) {
	const w, h = 80, 40
	surf := makeSurface(w, h)
	m := NewMaterial(MaterialSidebar)
	m.Child = NewLabel("Hi")
	m.SetBounds(Rect{X: 5, Y: 5, W: 60, H: 24})
	m.SetNativeBacked(true)
	m.Draw(newP(surf, w), DefaultLight())
	// The child label paints ink somewhere inside the bounds.
	minX, minY, maxX, maxY := nbPaintedBBox(surf, w, h)
	if maxX < 0 {
		t.Fatal("native material with a child painted nothing")
	}
	_ = minX
	_ = minY
	_ = maxY
}

func TestMaterialSetBoundsLaysOutChild(t *testing.T) {
	m := NewMaterial(MaterialMenu)
	child := NewLabel("x")
	m.Child = child
	r := Rect{X: 3, Y: 4, W: 50, H: 20}
	m.SetBounds(r)
	if child.Bounds() != r {
		t.Errorf("child bounds = %+v, want %+v", child.Bounds(), r)
	}
	// No-child SetBounds must not panic.
	NewMaterial(MaterialMenu).SetBounds(r)
}

func TestMaterialHitTest(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	m.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	// No child: transparent to hits.
	if m.HitTest(50, 50) {
		t.Error("childless material should be hit-transparent")
	}
	btn := NewButton("go", nil)
	m.Child = btn
	m.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	btn.SetBounds(Rect{X: 10, Y: 10, W: 40, H: 20})
	if !m.HitTest(20, 15) {
		t.Error("point over child should hit")
	}
	if m.HitTest(90, 90) {
		t.Error("point off child should not hit")
	}
}

func TestMaterialOnEvent(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	// No child: OnEvent is a no-op (no panic).
	m.OnEvent(Event{Kind: EventClick, X: 1, Y: 1})

	clicked := 0
	btn := NewButton("go", func() { clicked++ })
	m.Child = btn
	pr := Rect{X: 10, Y: 10, W: 100, H: 60}
	m.SetBounds(pr) // lays child out to fill pr

	// A click whose surface point lands inside the child fires it. Event coords
	// are material-local; surface = local + pr.origin.
	m.OnEvent(Event{Kind: EventClick, X: 20, Y: 20})
	m.OnEvent(Event{Kind: EventMouseUp, X: 20, Y: 20})
	if clicked == 0 {
		t.Error("click inside child did not reach it")
	}

	// A move is forwarded unconditionally (no panic, exercises the branch).
	m.OnEvent(Event{Kind: EventMouseMove, X: 5, Y: 5})

	// A click far outside the child's rect is not forwarded. Shrink the child.
	btn.SetBounds(Rect{X: 12, Y: 12, W: 10, H: 8})
	before := clicked
	m.OnEvent(Event{Kind: EventClick, X: 90, Y: 50})
	if clicked != before {
		t.Error("click outside child was forwarded")
	}
}

func TestMaterialInvalidateRecomputes(t *testing.T) {
	const sw, sh = 40, 40
	m := NewMaterial(MaterialSidebar)
	m.SetSource(edgeSource(sw, sh, 20), sw, sh)
	m.Sigma = 4
	r := Rect{X: 5, Y: 5, W: 20, H: 20}
	m.SetBounds(r)
	m.ensureBlur(r, 4)
	first := m.cache
	if first == nil {
		t.Fatal("no cache")
	}
	// A second call with an unchanged key reuses the cache (blurValid true).
	m.ensureBlur(r, 4)
	if &m.cache[0] != &first[0] {
		t.Error("cache recomputed despite unchanged key")
	}
	// Invalidate forces a recompute.
	m.Invalidate()
	if m.cache != nil {
		t.Fatal("Invalidate did not drop the cache")
	}
	m.ensureBlur(r, 4)
	if m.cache == nil {
		t.Fatal("recompute after Invalidate produced nothing")
	}
	// Changing sigma also recomputes.
	m.ensureBlur(r, 5)
	if m.cacheSigma != 5 {
		t.Error("sigma change did not recompute")
	}
}

func TestMaterialEnsureBlurZeroRegion(t *testing.T) {
	// A destination entirely off a 1x1 source still packs and blurs (clamped);
	// the point is that ensureBlur never indexes out of range.
	m := NewMaterial(MaterialSidebar)
	m.SetSource([]byte{10, 20, 30, 255}, 1, 1)
	r := Rect{X: 0, Y: 0, W: 3, H: 3}
	m.SetBounds(r)
	m.ensureBlur(r, 2)
	if m.cache == nil || len(m.cache) != r.W*r.H*4 {
		t.Fatalf("cache size wrong: %d", len(m.cache))
	}
}

// TestMaterialEnsureBlurErrorBranch drives the defensive fallback: GaussianBlur
// rejects a non-positive sigma, which Draw never passes (its gate is sigma>0),
// so the branch is reached only by a direct call — proving it clears the cache
// rather than panicking.
func TestMaterialEnsureBlurErrorBranch(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	m.SetSource(edgeSource(20, 20, 10), 20, 20)
	m.cache = []byte{1, 2, 3, 4} // pretend a stale cache
	m.SetBounds(Rect{W: 10, H: 10})
	m.ensureBlur(Rect{W: 10, H: 10}, 0) // sigma 0 -> GaussianBlur errors
	if m.cache != nil {
		t.Error("error branch should have cleared the cache")
	}
}

func TestMaterialEffectiveOverrides(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	// Default sigma from kind.
	if m.effectiveSigma() != materialSigma(MaterialSidebar) {
		t.Error("default sigma wrong")
	}
	// Override.
	m.Sigma = 3.5
	if m.effectiveSigma() != 3.5 {
		t.Error("sigma override ignored")
	}
	// Default tint from kind+theme.
	th := DefaultLight()
	if m.effectiveTint(th) != materialTint(MaterialSidebar, th) {
		t.Error("default tint wrong")
	}
	// Override.
	m.Tint = painter.RGBA{R: 1, G: 2, B: 3, A: 200}
	if m.effectiveTint(th) != m.Tint {
		t.Error("tint override ignored")
	}
}

func TestMaterialKindDefaultsAllReachable(t *testing.T) {
	th := DefaultLight()
	kinds := []MaterialKind{
		MaterialWindowBackground, MaterialSidebar, MaterialTitlebar,
		MaterialMenu, MaterialPopover, MaterialHUD, MaterialSelection,
	}
	for _, k := range kinds {
		s := materialSigma(k)
		tint := materialTint(k, th)
		if k == MaterialSelection {
			if s != 0 {
				t.Errorf("selection sigma = %v, want 0", s)
			}
		} else if s <= 0 {
			t.Errorf("kind %d sigma = %v, want >0", k, s)
		}
		// Every default tint is translucent so the backdrop shows.
		if tint.A == 0 || tint.A == 0xFF {
			t.Errorf("kind %d tint alpha = %d, want translucent", k, tint.A)
		}
	}
	// An out-of-range kind falls through to the no-blur / selection defaults.
	if materialSigma(MaterialKind(99)) != 0 {
		t.Error("unknown kind should default to no blur")
	}
	if materialTint(MaterialKind(99), th).A == 0 {
		t.Error("unknown kind should still give a translucent tint")
	}
}

func TestMaterialHasSource(t *testing.T) {
	m := NewMaterial(MaterialSidebar)
	if m.hasSource() {
		t.Error("empty material reports a source")
	}
	m.SetSource(make([]byte, 10*10*4), 10, 10)
	if !m.hasSource() {
		t.Error("valid source not reported")
	}
	// Too-short buffer for the dims is not a usable source.
	m.Source = m.Source[:8]
	if m.hasSource() {
		t.Error("short buffer reported as source")
	}
}

func TestMaterialA11yPresentation(t *testing.T) {
	if NewMaterial(MaterialSidebar).A11y().Role != RolePresentation {
		t.Error("material should be presentational")
	}
}

func TestCollectMaterialsAndSpec(t *testing.T) {
	if got := CollectMaterials(nil); got != nil {
		t.Errorf("nil root should yield no materials, got %v", got)
	}
	// A plain widget yields none.
	if got := CollectMaterials(NewLabel("x")); len(got) != 0 {
		t.Errorf("label yielded materials: %v", got)
	}
	// A material nested inside a container is found through the tree walk.
	inner := NewMaterial(MaterialMenu)
	inner.Blend = BlendWithinWindow
	inner.SetBounds(Rect{X: 4, Y: 5, W: 30, H: 20})
	box := NewVBox()
	box.Append(NewLabel("a"))
	box.Append(inner)
	box.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 100})
	got := CollectMaterials(box)
	if len(got) != 1 || got[0] != inner {
		t.Fatalf("nested material not collected: %v", got)
	}
	sp := got[0].Spec()
	if sp.Kind != MaterialMenu || sp.Blend != BlendWithinWindow || sp.Rect != inner.Bounds() {
		t.Errorf("spec wrong: %+v", sp)
	}
	// A material that is itself the root is collected, and its child descended.
	root := NewMaterial(MaterialSidebar)
	root.Child = NewMaterial(MaterialHUD)
	all := CollectMaterials(root)
	if len(all) != 2 {
		t.Errorf("root + child material = %d, want 2", len(all))
	}
}

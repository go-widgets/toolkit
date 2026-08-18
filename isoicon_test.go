// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"testing"

	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
)

// solidSprite builds a w x h straight-alpha raster filled with one opaque colour
// — a stand-in for an external icon pack's PNG, with no third-party art bundled.
func solidSprite(w, h int, c stdcolor.RGBA) *raster.Image {
	img := raster.New(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

// --- control-run: validate the pixel-probe instrument on known geometry -------

// TestIsoIconProbeControl is the control run for the icon pixel probe. It renders
// a KNOWN-good geometry — a plain unit cube (the "box" built-in) — and confirms
// the probe reads exactly the base colour at the cube's top centre (z=1) and the
// canvas surface one unit ABOVE it (z=2, empty air). Only once the instrument is
// proven on the cube do the icon tests below trust it to tell a tall icon (which
// fills z=2) apart from the cube fallback (which does not).
func TestIsoIconProbeControl(t *testing.T) {
	theme := DefaultLight()
	base := RGBA{R: 210, G: 40, B: 90, A: 255}
	d := NewIsoDiagram(nil)
	d.Doc().PutNode(IsoNode{ID: "a", X: 4, Y: 4, Icon: "box", Color: base})
	img, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// Top centre of the cube (z=1): the top face at DefaultShading.Top == 1 is the
	// base colour exactly.
	tx, ty := localOf(d, iso.V(4.5, 4.5, 1))
	if got := img.RGBAAt(tx, ty); got != stdcolor.RGBA(stdColor(base)) {
		t.Fatalf("control: cube top pixel = %v, want base %v", got, base)
	}
	// One unit above the cube (z=2) is empty air: the widget's surface colour.
	ax, ay := localOf(d, iso.V(4.5, 4.5, 2))
	if got := img.RGBAAt(ax, ay); got != stdcolor.RGBA(stdColor(theme.Surface)) {
		t.Fatalf("control: air-above-cube pixel = %v, want surface %v", got, theme.Surface)
	}
}

// --- registry resolution ------------------------------------------------------

func TestIsoIconRegistryResolveExact(t *testing.T) {
	r := NewIsoIconRegistry()
	marker := solidSprite(2, 2, stdcolor.RGBA{R: 1, G: 2, B: 3, A: 255})
	want := IsoSpriteIcon{Img: marker}
	r.Register("thing", want)

	got, ok := r.Resolve("thing")
	if !ok {
		t.Fatal("registered id did not resolve")
	}
	sp, isSprite := got.(IsoSpriteIcon)
	if !isSprite || sp.Img != marker {
		t.Fatalf("Resolve returned %#v, want the exact registered sprite", got)
	}
}

func TestIsoIconRegistryUnknownIsFallbackCube(t *testing.T) {
	r := NewIsoIconRegistry()
	icon, ok := r.Resolve("no-such-icon")
	if ok {
		t.Fatal("unknown id reported found")
	}
	// The fallback renders exactly one shape, a unit iso.Cube.
	shapes := icon.Render(2, 3, stdcolor.RGBA{R: 10, G: 20, B: 30, A: 255}).Shapes
	if len(shapes) != 1 {
		t.Fatalf("fallback produced %d shapes, want 1", len(shapes))
	}
	cube, isCube := shapes[0].(iso.Cube)
	if !isCube {
		t.Fatalf("fallback shape is %T, want iso.Cube", shapes[0])
	}
	if cube.Pos != iso.V(2, 3, 0) || cube.Size != 1 {
		t.Fatalf("fallback cube = %+v, want unit cube at (2,3,0)", cube)
	}
	// The empty id resolves to the same fallback.
	if _, ok := r.Resolve(""); ok {
		t.Fatal("empty id reported found")
	}
}

func TestIsoIconRegisterPack(t *testing.T) {
	r := NewIsoIconRegistry()
	hub := IsoSpriteIcon{Img: solidSprite(2, 2, stdcolor.RGBA{A: 255})}
	// A named pack namespaces its ids under the pack name.
	r.RegisterPack(IsoIconPack{Name: "net", Icons: map[string]IsoIcon{"hub": hub}})
	if _, ok := r.Resolve("net/hub"); !ok {
		t.Fatal("named-pack icon not found under net/hub")
	}
	if _, ok := r.Resolve("hub"); ok {
		t.Fatal("named-pack icon leaked under bare id")
	}
	// An unnamed pack registers its ids bare.
	r.RegisterPack(IsoIconPack{Icons: map[string]IsoIcon{"bare": hub}})
	if _, ok := r.Resolve("bare"); !ok {
		t.Fatal("unnamed-pack icon not found under bare id")
	}
}

func TestIsoIconRegistryIDs(t *testing.T) {
	r := NewIsoIconRegistry()
	r.Register("a", IsoFallbackIcon)
	r.Register("b", IsoFallbackIcon)
	ids := r.IDs()
	if len(ids) != 2 {
		t.Fatalf("IDs = %v, want 2 entries", ids)
	}
	set := map[string]bool{ids[0]: true, ids[1]: true}
	if !set["a"] || !set["b"] {
		t.Fatalf("IDs = %v, want {a,b}", ids)
	}
}

func TestIsoIconPackageLevelRegistration(t *testing.T) {
	// Package-level helpers mutate the shared default registry. Use ids unlikely
	// to collide with the built-ins.
	sprite := IsoSpriteIcon{Img: solidSprite(2, 2, stdcolor.RGBA{A: 255})}
	RegisterIcon("test.custom", sprite)
	if _, ok := IsoDefaultIcons().Resolve("test.custom"); !ok {
		t.Fatal("RegisterIcon did not reach the default registry")
	}
	RegisterIconPack(IsoIconPack{Name: "testpack", Icons: map[string]IsoIcon{"x": sprite}})
	if _, ok := IsoDefaultIcons().Resolve("testpack/x"); !ok {
		t.Fatal("RegisterIconPack did not reach the default registry")
	}
}

// --- built-in icon set --------------------------------------------------------

func TestIsoBuiltinIconsAllRegisteredAndDraw(t *testing.T) {
	theme := DefaultLight()
	base := RGBA{R: 60, G: 160, B: 220, A: 255}
	for _, id := range IsoBuiltinIconIDs {
		icon, ok := IsoDefaultIcons().Resolve(id)
		if !ok {
			t.Fatalf("built-in %q not registered in the default registry", id)
		}
		shapes := icon.Render(3, 3, stdColor(base)).Shapes
		if len(shapes) == 0 {
			t.Fatalf("built-in %q rendered no shapes", id)
		}
		// Rendering a node that uses the icon must paint opaque pixels somewhere in
		// its cell (it is not a no-op / all-transparent).
		d := NewIsoDiagram(nil)
		d.Doc().PutNode(IsoNode{ID: "n", X: 3, Y: 3, Icon: id, Color: base})
		img, err := RenderImage(d, 400, 400, theme)
		if err != nil {
			t.Fatal(err)
		}
		cx, cy := localOf(d, iso.V(3.5, 3.5, 0.2))
		if got := img.RGBAAt(cx, cy); got.A != 255 {
			t.Fatalf("built-in %q left its cell centre unpainted: %v", id, got)
		}
	}
}

// TestIsoIconServerVsCubePixel is the headline exact-pixel test: a node with
// Icon="server" renders the server icon (a two-unit-tall tower) whose top FACE
// fills the air one unit above where a cube fallback ends, so probing the same
// screen point tells them apart — server paints its base colour there, the cube
// fallback leaves the surface.
func TestIsoIconServerVsCubePixel(t *testing.T) {
	theme := DefaultLight()
	base := RGBA{R: 230, G: 120, B: 20, A: 255}

	server := NewIsoDiagram(nil)
	server.Doc().PutNode(IsoNode{ID: "s", X: 4, Y: 4, Icon: "server", Color: base})
	simg, err := RenderImage(server, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	// z=2 is the server tower's top-face centre: base colour exactly (Top factor 1).
	sx, sy := localOf(server, iso.V(4.5, 4.5, 2))
	if got := simg.RGBAAt(sx, sy); got != stdcolor.RGBA(stdColor(base)) {
		t.Fatalf("server top pixel = %v, want base %v", got, base)
	}

	// An unknown id at the same cell falls back to the cube, which does NOT reach
	// z=2 — the same probe point is empty surface.
	cube := NewIsoDiagram(nil)
	cube.Doc().PutNode(IsoNode{ID: "c", X: 4, Y: 4, Icon: "totally-unknown", Color: base})
	cimg, err := RenderImage(cube, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	cx, cy := localOf(cube, iso.V(4.5, 4.5, 2))
	if got := cimg.RGBAAt(cx, cy); got != stdcolor.RGBA(stdColor(theme.Surface)) {
		t.Fatalf("cube-fallback pixel at z=2 = %v, want surface %v (fallback drew a tall icon?)", got, theme.Surface)
	}
}

// --- sprite icons -------------------------------------------------------------

func TestIsoSpriteIconBlitsAtCellRect(t *testing.T) {
	theme := DefaultLight()
	magenta := stdcolor.RGBA{R: 255, G: 0, B: 255, A: 255}
	reg := NewIsoIconRegistry()
	reg.Register("pic", IsoSpriteIcon{Img: solidSprite(8, 8, magenta)})

	d := NewIsoDiagram(nil)
	d.Icons = reg // per-widget override library
	d.Doc().PutNode(IsoNode{ID: "s", X: 4, Y: 4, Icon: "pic"})
	img, err := RenderImage(d, 400, 400, theme)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := d.Doc().Node("s")
	rect := d.spriteRect(n)
	// The rect centre must be the opaque sprite colour.
	ccx, ccy := rect.X+rect.W/2, rect.Y+rect.H/2
	if got := img.RGBAAt(ccx, ccy); got != magenta {
		t.Fatalf("sprite centre pixel = %v, want magenta %v", got, magenta)
	}
	// A point well outside the sprite rect (and above it, empty air) is not the
	// sprite colour.
	ox, oy := rect.X+rect.W/2, rect.Y-40
	if got := img.RGBAAt(ox, oy); got == magenta {
		t.Fatalf("pixel above the sprite rect = %v, unexpectedly magenta", got)
	}
	// The rect is a TileW square standing on the projected ground centre.
	p := d.proj.Project(iso.V(4.5, 4.5, 0))
	s := iround(d.proj.TileW)
	wantRect := Rect{X: iround(p.X) - s/2, Y: iround(p.Y) - s, W: s, H: s}
	if rect != wantRect {
		t.Fatalf("spriteRect = %+v, want %+v", rect, wantRect)
	}
}

func TestBlitSpriteTransparentPixelPreservesDst(t *testing.T) {
	// A sprite whose (0,0) source pixel is transparent must leave the destination
	// untouched there (the A==0 skip branch) while its opaque body overwrites the
	// rest. Tested directly on blitSprite so the destination is deterministic.
	bg := stdcolor.RGBA{R: 40, G: 40, B: 40, A: 255}
	dst := solidSprite(2, 2, bg)
	src := raster.New(2, 2)
	body := stdcolor.RGBA{R: 0, G: 200, B: 0, A: 255}
	src.Set(1, 0, body)
	src.Set(0, 1, body)
	src.Set(1, 1, body)
	src.Set(0, 0, stdcolor.RGBA{}) // transparent

	blitSprite(dst, Rect{X: 0, Y: 0, W: 2, H: 2}, src)
	if got := dst.At(0, 0); got != bg {
		t.Fatalf("transparent source pixel overwrote dst: got %v, want bg %v", got, bg)
	}
	if got := dst.At(1, 1); got != body {
		t.Fatalf("opaque source pixel not drawn: got %v, want %v", got, body)
	}
}

// --- per-widget override precedence ------------------------------------------

func TestIsoIconWidgetOverridePrecedence(t *testing.T) {
	d := NewIsoDiagram(nil)
	// Default: nil Icons uses the package default.
	if d.iconRegistry() != IsoDefaultIcons() {
		t.Fatal("nil Icons should resolve to the default registry")
	}
	// A per-widget registry that redefines "server" as a sprite must win over the
	// default primitive server.
	magenta := stdcolor.RGBA{R: 255, G: 0, B: 255, A: 255}
	reg := NewIsoIconRegistry()
	reg.Register("server", IsoSpriteIcon{Img: solidSprite(8, 8, magenta)})
	d.Icons = reg
	if d.iconRegistry() != reg {
		t.Fatal("set Icons should be the widget's registry")
	}
	d.Doc().PutNode(IsoNode{ID: "s", X: 4, Y: 4, Icon: "server"})
	img, err := RenderImage(d, 400, 400, DefaultLight())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := d.Doc().Node("s")
	rect := d.spriteRect(n)
	if got := img.RGBAAt(rect.X+rect.W/2, rect.Y+rect.H/2); got != magenta {
		t.Fatalf("override server pixel = %v, want the sprite magenta %v", got, magenta)
	}
}

// --- blitSprite / blendPixel unit coverage -----------------------------------

func TestBlitSpriteGuards(t *testing.T) {
	dst := raster.New(4, 4)
	// nil source, empty source and empty rect all draw nothing (and must not panic).
	blitSprite(dst, Rect{X: 0, Y: 0, W: 4, H: 4}, nil)
	blitSprite(dst, Rect{X: 0, Y: 0, W: 4, H: 4}, raster.New(0, 0))
	blitSprite(dst, Rect{X: 0, Y: 0, W: 0, H: 4}, solidSprite(2, 2, stdcolor.RGBA{A: 255}))
	for i := range dst.Pix {
		if dst.Pix[i] != 0 {
			t.Fatalf("guard case wrote to dst at byte %d", i)
		}
	}
}

func TestBlitSpriteClipsToDst(t *testing.T) {
	dst := raster.New(4, 4)
	red := stdcolor.RGBA{R: 255, A: 255}
	// A rect that straddles every edge: only the in-bounds pixels are written.
	blitSprite(dst, Rect{X: -2, Y: -2, W: 8, H: 8}, solidSprite(8, 8, red))
	// Corners inside dst are painted.
	if dst.At(0, 0) != red || dst.At(3, 3) != red {
		t.Fatal("in-bounds pixels not painted through the clip")
	}
	// A rect entirely below/right of dst writes nothing new.
	dst2 := raster.New(4, 4)
	blitSprite(dst2, Rect{X: 10, Y: 10, W: 4, H: 4}, solidSprite(2, 2, red))
	for i := range dst2.Pix {
		if dst2.Pix[i] != 0 {
			t.Fatal("off-surface rect wrote to dst")
		}
	}
}

func TestBlendPixel(t *testing.T) {
	dst := raster.New(1, 1)
	dst.Set(0, 0, stdcolor.RGBA{R: 200, G: 200, B: 200, A: 255})
	// Opaque source replaces the destination exactly.
	src := stdcolor.RGBA{R: 10, G: 20, B: 30, A: 255}
	blendPixel(dst, 0, 0, src)
	if dst.At(0, 0) != src {
		t.Fatalf("opaque blend = %v, want %v", dst.At(0, 0), src)
	}
	// Half-alpha source blends by the same integer formula the code uses.
	dst.Set(0, 0, stdcolor.RGBA{R: 200, G: 200, B: 200, A: 255})
	s := stdcolor.RGBA{R: 100, G: 0, B: 0, A: 128}
	blendPixel(dst, 0, 0, s)
	a, ia := uint32(128), uint32(127)
	want := stdcolor.RGBA{
		R: uint8((uint32(100)*a + 200*ia) / 255),
		G: uint8((uint32(0)*a + 200*ia) / 255),
		B: uint8((uint32(0)*a + 200*ia) / 255),
		A: uint8((a*255 + 255*ia) / 255),
	}
	if dst.At(0, 0) != want {
		t.Fatalf("half-alpha blend = %v, want %v", dst.At(0, 0), want)
	}
}

// TestIsoIconDrawMixedDoc exercises the Draw path with every node kind at once:
// a bare-shape node (Icon==""), a primitive icon node and a sprite icon node, so
// the scene loop, the sprite loop and their skip branches all run.
func TestIsoIconDrawMixedDoc(t *testing.T) {
	reg := NewIsoIconRegistry()
	reg.Register("pic", IsoSpriteIcon{Img: solidSprite(6, 6, stdcolor.RGBA{B: 255, A: 255})})
	reg.Register("server", IsoPrimitiveIcon{Build: isoServerShapes})
	d := NewIsoDiagram(nil)
	d.Icons = reg
	d.Doc().PutNode(IsoNode{ID: "plain", X: 1, Y: 1})                // Icon=="" -> shape path + sprite-skip
	d.Doc().PutNode(IsoNode{ID: "srv", X: 3, Y: 3, Icon: "server"})  // primitive -> no sprite
	d.Doc().PutNode(IsoNode{ID: "pic", X: 5, Y: 5, Icon: "pic"})     // sprite
	d.Doc().PutNode(IsoNode{ID: "unk", X: 7, Y: 7, Icon: "mystery"}) // unknown -> cube fallback
	if _, err := RenderImage(d, 500, 500, DefaultLight()); err != nil {
		t.Fatal(err)
	}
}

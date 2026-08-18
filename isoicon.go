// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	stdcolor "image/color"
	"sync"

	gfxcolor "github.com/go-gfx/gfx/color"
	"github.com/go-gfx/gfx/iso"
	"github.com/go-gfx/gfx/raster"
)

// IsoIconDrawing is what an [IsoIcon] contributes for one node placement: a set
// of depth-sortable isometric primitives to composite into the diagram's scene
// and/or a billboarded sprite image to blit at the node's cell. A
// primitive-composed icon fills Shapes; a sprite/image icon fills Sprite; a
// hybrid may fill both (the shapes draw in the depth-sorted scene, the sprite
// blits over the rendered scene at the cell).
type IsoIconDrawing struct {
	// Shapes are the isometric solids (from github.com/go-gfx/gfx/iso) that draw
	// this icon; they are added to the widget's [iso.Scene] and depth-sorted with
	// every other node and connector so overlap is correct.
	Shapes []iso.Shape
	// Sprite, when non-nil, is a straight-alpha raster image blitted (billboarded,
	// nearest-neighbour scaled) into the node's cell rectangle after the scene is
	// rendered. This is how an external icon pack — arbitrary isometric PNG art —
	// is placed without any Go drawing code.
	Sprite *raster.Image
}

// IsoIcon renders one placeable component of an isometric diagram at a grid
// cell. It is the extension point of the icon library: a host registers custom
// icons (whole component packs) so an [IsoNode] can name one by id. An icon is a
// value evaluated at draw time, so it must be safe to call [IsoIcon.Render]
// concurrently for read.
type IsoIcon interface {
	// Render returns the drawing for a node whose 1x1 footprint's near corner is
	// grid cell (x, y), coloured from the node's resolved base colour base (the
	// same colour the plain-shape fallback would shade its faces from). The
	// implementation must not retain x, y or base.
	Render(x, y int, base stdcolor.RGBA) IsoIconDrawing
}

// IsoPrimitiveIcon is an [IsoIcon] composed from isometric primitives — the
// pure-Go, code-authored kind. Build returns the [iso.Shape]s (cubes, bricks,
// pyramids, slopes, lines, …) that draw the icon at cell (x, y) shaded from
// base. Authoring an icon is therefore just writing that function; the bundled
// architecture icons ([IsoDefaultIcons]) are exactly this.
type IsoPrimitiveIcon struct {
	// Build composes the icon's solids at grid cell (x, y) from base colour base.
	Build func(x, y int, base stdcolor.RGBA) []iso.Shape
}

// Render satisfies [IsoIcon] by returning the built shapes as the drawing.
func (i IsoPrimitiveIcon) Render(x, y int, base stdcolor.RGBA) IsoIconDrawing {
	return IsoIconDrawing{Shapes: i.Build(x, y, base)}
}

// IsoSpriteIcon is an [IsoIcon] that blits a fixed raster image — an isometric
// sprite from an external icon pack — billboarded at the node's cell. Img is the
// straight-alpha art (transparent pixels show the grid through). The node's base
// colour does not tint the sprite; the art carries its own colours.
type IsoSpriteIcon struct {
	// Img is the sprite blitted at the cell. A nil Img draws nothing.
	Img *raster.Image
}

// Render satisfies [IsoIcon] by returning the sprite as the drawing.
func (i IsoSpriteIcon) Render(x, y int, base stdcolor.RGBA) IsoIconDrawing {
	return IsoIconDrawing{Sprite: i.Img}
}

// IsoIconPack is a named set of icons registered together — a component library
// (e.g. an "aws" or "network" pack). Registering the pack adds each icon under
// the id "<Name>/<key>" when Name is non-empty (so packs from different sources
// never collide), or under the bare key when Name is empty.
type IsoIconPack struct {
	// Name is the pack's namespace prefix; empty registers the icons bare.
	Name string
	// Icons maps a within-pack key to the icon registered for it.
	Icons map[string]IsoIcon
}

// IsoFallbackIcon is the icon an [IsoIconRegistry] resolves an unknown (or empty)
// id to: a plain unit cube shaded from the node's base colour — identical to the
// widget's original bare-shape node. It is exported so a host can reuse it or
// substitute its own default.
var IsoFallbackIcon IsoIcon = IsoPrimitiveIcon{Build: isoBoxShapes}

// IsoIconRegistry maps a string icon-id to an [IsoIcon]. The package keeps one
// default registry ([IsoDefaultIcons], seeded with the built-in architecture
// icons); an [IsoDiagram] uses that unless its own Icons field overrides it. The
// registry is safe for concurrent Register/Resolve.
type IsoIconRegistry struct {
	mu    sync.RWMutex
	icons map[string]IsoIcon
}

// NewIsoIconRegistry returns an empty registry. Unknown ids resolve to
// [IsoFallbackIcon] until something is registered.
func NewIsoIconRegistry() *IsoIconRegistry {
	return &IsoIconRegistry{icons: map[string]IsoIcon{}}
}

// Register adds (or replaces) the icon stored under id.
func (r *IsoIconRegistry) Register(id string, icon IsoIcon) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.icons[id] = icon
}

// RegisterPack registers every icon in p (see [IsoIconPack] for the id scheme).
func (r *IsoIconRegistry) RegisterPack(p IsoIconPack) {
	for key, icon := range p.Icons {
		id := key
		if p.Name != "" {
			id = p.Name + "/" + key
		}
		r.Register(id, icon)
	}
}

// Resolve returns the icon registered under id and true, or [IsoFallbackIcon]
// and false when id is unknown (including the empty id). The always-usable icon
// it returns lets a caller draw without branching on the boolean.
func (r *IsoIconRegistry) Resolve(id string) (IsoIcon, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if icon, ok := r.icons[id]; ok {
		return icon, true
	}
	return IsoFallbackIcon, false
}

// IDs returns the registered ids in no particular order — for building a palette
// or listing an installed library.
func (r *IsoIconRegistry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.icons))
	for id := range r.icons {
		out = append(out, id)
	}
	return out
}

// defaultIsoIcons is the package-global registry, seeded once with the built-in
// architecture icons.
var defaultIsoIcons = newBuiltinIsoIcons()

// IsoDefaultIcons returns the package-global icon registry. [RegisterIcon] and
// [RegisterIconPack] mutate it, and an [IsoDiagram] whose own Icons field is nil
// draws through it.
func IsoDefaultIcons() *IsoIconRegistry { return defaultIsoIcons }

// RegisterIcon registers icon under id in the default registry ([IsoDefaultIcons]).
func RegisterIcon(id string, icon IsoIcon) { defaultIsoIcons.Register(id, icon) }

// RegisterIconPack registers a whole pack in the default registry.
func RegisterIconPack(p IsoIconPack) { defaultIsoIcons.RegisterPack(p) }

// IsoBuiltinIconIDs are the ids of the primitive-composed architecture icons the
// default registry ships with. They are stylised, code-authored isometric icons
// — not traced from any third-party art.
var IsoBuiltinIconIDs = []string{
	"server", "rack", "cloud", "database",
	"router", "switch", "storage", "box", "user",
}

// newBuiltinIsoIcons builds a registry seeded with the built-in icon set.
func newBuiltinIsoIcons() *IsoIconRegistry {
	r := NewIsoIconRegistry()
	server := IsoPrimitiveIcon{Build: isoServerShapes}
	r.Register("server", server)
	r.Register("rack", server) // a rack reads as the same tower
	r.Register("cloud", IsoPrimitiveIcon{Build: isoCloudShapes})
	r.Register("database", IsoPrimitiveIcon{Build: isoDatabaseShapes})
	r.Register("router", IsoPrimitiveIcon{Build: isoRouterShapes})
	r.Register("switch", IsoPrimitiveIcon{Build: isoSwitchShapes})
	r.Register("storage", IsoPrimitiveIcon{Build: isoStorageShapes})
	r.Register("box", IsoPrimitiveIcon{Build: isoBoxShapes})
	r.Register("user", IsoPrimitiveIcon{Build: isoUserShapes})
	return r
}

// --- built-in icon builders --------------------------------------------------
//
// Each builder composes one stylised isometric icon at grid cell (x, y) from the
// go-gfx/iso primitives, shading accent parts off the node's base colour with
// gfxcolor.Shade. They are our own designs, authored in Go.

// isoBoxShapes is a plain unit cube — the generic node and the fallback.
func isoBoxShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	return []iso.Shape{iso.Cube{Pos: iso.V(float64(x), float64(y), 0), Size: 1, Color: c}}
}

// isoServerShapes is a two-unit-tall tower with dark front slots — a server /
// rack. Its extra height above a unit cube is what visually distinguishes it.
func isoServerShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	slot := gfxcolor.Shade(c, 0.4)
	shapes := []iso.Shape{
		iso.Brick{Pos: iso.V(fx, fy, 0), Dim: iso.Dimension{W: 1, H: 1, D: 2}, Color: c},
	}
	for i := 1; i <= 2; i++ {
		z := float64(i) * 0.6
		shapes = append(shapes, iso.Line{
			From: iso.V(fx+1, fy, z), To: iso.V(fx+1, fy+1, z), Color: slot, Width: 2,
		})
	}
	return shapes
}

// isoCloudShapes is a cluster of overlapping lightened bricks raised off the
// ground — a puffy cloud.
func isoCloudShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	puff := gfxcolor.Shade(c, 1.15)
	offs := [][2]float64{{0, 0.2}, {0.35, 0}, {0.2, 0.45}}
	shapes := make([]iso.Shape, 0, len(offs))
	for _, o := range offs {
		shapes = append(shapes, iso.Brick{
			Pos:   iso.V(fx+o[0]*0.5, fy+o[1]*0.5, 0.6),
			Dim:   iso.Dimension{W: 0.55, H: 0.55, D: 0.4},
			Color: puff,
		})
	}
	return shapes
}

// isoDatabaseShapes is three stacked thin bricks with alternating banding — the
// classic stacked-cylinder database silhouette (a cylinder approximated in the
// axis-aligned primitive set).
func isoDatabaseShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	band := gfxcolor.Shade(c, 0.55)
	shapes := make([]iso.Shape, 0, 3)
	for i := 0; i < 3; i++ {
		col := c
		if i%2 == 1 {
			col = band
		}
		shapes = append(shapes, iso.Brick{
			Pos:   iso.V(fx+0.1, fy+0.1, float64(i)*0.42),
			Dim:   iso.Dimension{W: 0.8, H: 0.8, D: 0.34},
			Color: col,
		})
	}
	return shapes
}

// isoRouterShapes is a low base with two rising antennae — a router.
func isoRouterShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	ant := gfxcolor.Shade(c, 0.7)
	shapes := []iso.Shape{
		iso.Brick{Pos: iso.V(fx+0.05, fy+0.05, 0), Dim: iso.Dimension{W: 0.9, H: 0.9, D: 0.4}, Color: c},
	}
	for _, p := range [][2]float64{{0.3, 0.3}, {0.7, 0.6}} {
		shapes = append(shapes, iso.Line{
			From: iso.V(fx+p[0], fy+p[1], 0.4), To: iso.V(fx+p[0], fy+p[1], 1.1), Color: ant, Width: 2,
		})
	}
	return shapes
}

// isoSwitchShapes is a flat wide box with a row of dark front ports — a network
// switch.
func isoSwitchShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	port := gfxcolor.Shade(c, 0.35)
	shapes := []iso.Shape{
		iso.Brick{Pos: iso.V(fx, fy+0.2, 0), Dim: iso.Dimension{W: 1, H: 0.6, D: 0.28}, Color: c},
	}
	for i := 0; i < 4; i++ {
		yy := fy + 0.25 + float64(i)*0.12
		shapes = append(shapes, iso.Line{
			From: iso.V(fx+1, yy, 0.06), To: iso.V(fx+1, yy, 0.22), Color: port, Width: 1,
		})
	}
	return shapes
}

// isoStorageShapes is a stack of three drawers, each with a handle line — a
// storage array.
func isoStorageShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	handle := gfxcolor.Shade(c, 0.45)
	shapes := make([]iso.Shape, 0, 6)
	for i := 0; i < 3; i++ {
		z := float64(i) * 0.5
		shapes = append(shapes,
			iso.Brick{Pos: iso.V(fx, fy, z), Dim: iso.Dimension{W: 1, H: 1, D: 0.42}, Color: c},
			iso.Line{From: iso.V(fx+1, fy+0.35, z+0.2), To: iso.V(fx+1, fy+0.65, z+0.2), Color: handle, Width: 2},
		)
	}
	return shapes
}

// isoUserShapes is a small body brick topped by a lightened head cube — a
// generic user / actor.
func isoUserShapes(x, y int, c stdcolor.RGBA) []iso.Shape {
	fx, fy := float64(x), float64(y)
	head := gfxcolor.Shade(c, 1.1)
	return []iso.Shape{
		iso.Brick{Pos: iso.V(fx+0.25, fy+0.25, 0), Dim: iso.Dimension{W: 0.5, H: 0.5, D: 0.7}, Color: c},
		iso.Cube{Pos: iso.V(fx+0.3, fy+0.3, 0.7), Size: 0.4, Color: head},
	}
}

// --- sprite compositing ------------------------------------------------------

// blitSprite composites src into dst over the widget-local rectangle rect,
// nearest-neighbour scaled and source-over blended (straight alpha), clipped to
// dst. A fully opaque source pixel replaces the destination exactly. A nil or
// empty source, or an empty rect, draws nothing.
func blitSprite(dst *raster.Image, rect Rect, src *raster.Image) {
	if src == nil || src.W <= 0 || src.H <= 0 || rect.W <= 0 || rect.H <= 0 {
		return
	}
	for dy := 0; dy < rect.H; dy++ {
		yy := rect.Y + dy
		if yy < 0 || yy >= dst.H {
			continue
		}
		sy := dy * src.H / rect.H
		for dx := 0; dx < rect.W; dx++ {
			xx := rect.X + dx
			if xx < 0 || xx >= dst.W {
				continue
			}
			sc := src.At(dx*src.W/rect.W, sy)
			if sc.A == 0 {
				continue
			}
			blendPixel(dst, xx, yy, sc)
		}
	}
}

// blendPixel source-over composites the straight-alpha colour s onto dst's pixel
// (x, y). With s.A == 255 the destination becomes s exactly.
func blendPixel(dst *raster.Image, x, y int, s stdcolor.RGBA) {
	i := dst.PixOffset(x, y)
	a := uint32(s.A)
	ia := 255 - a
	dst.Pix[i] = uint8((uint32(s.R)*a + uint32(dst.Pix[i])*ia) / 255)
	dst.Pix[i+1] = uint8((uint32(s.G)*a + uint32(dst.Pix[i+1])*ia) / 255)
	dst.Pix[i+2] = uint8((uint32(s.B)*a + uint32(dst.Pix[i+2])*ia) / 255)
	dst.Pix[i+3] = uint8((a*255 + uint32(dst.Pix[i+3])*ia) / 255)
}

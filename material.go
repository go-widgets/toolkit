// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"image"
	"math"

	"github.com/go-images/images"
	"github.com/go-widgets/painter"
)

// MaterialKind names a standard translucent-background material — a blurred
// backdrop washed with a semi-transparent colour, the "vibrancy" a modern
// desktop paints behind a sidebar, a menu or a heads-up panel so the content
// behind the surface shows through, softened.
//
// The vocabulary is deliberately generic UI roles (a sidebar, a menu, a
// titlebar, a HUD) rather than any one platform's material names: a native
// back-end maps each kind onto its own system material (see the window package's
// Cocoa backend, which maps them to NSVisualEffectView materials), and the
// pure-Go fallback maps each onto a blur radius (sigma) plus a colour wash. A
// widget tree therefore asks for "a sidebar material" and renders correctly on
// every back-end without naming a platform.
type MaterialKind int

const (
	// MaterialWindowBackground is the whole-window translucent ground.
	MaterialWindowBackground MaterialKind = iota
	// MaterialSidebar is the list rail beside primary content.
	MaterialSidebar
	// MaterialTitlebar is the strip along the top of a window.
	MaterialTitlebar
	// MaterialMenu is a menu / dropdown surface.
	MaterialMenu
	// MaterialPopover is a transient floating panel anchored to a control.
	MaterialPopover
	// MaterialHUD is a dark heads-up panel (always dark, independent of theme).
	MaterialHUD
	// MaterialSelection is a light accent wash over a selected region; it has
	// no blur of its own (sigma 0), only a translucent tint.
	MaterialSelection
)

// MaterialBlend selects what a material blurs: the content BEHIND the window
// (the desktop showing through) or the content WITHIN the window drawn behind
// the material. It mirrors the two blending modes every platform vibrancy API
// offers. The pure-Go fallback treats both identically — it blurs whatever
// Source it is given — but a native back-end honours the distinction when it
// installs its system effect view.
type MaterialBlend int

const (
	// BlendBehindWindow blurs what is behind the window itself.
	BlendBehindWindow MaterialBlend = iota
	// BlendWithinWindow blurs the window content drawn behind the material.
	BlendWithinWindow
)

// Material is a translucent, blurred background panel with an optional child
// composited on top. It is the sidebar/menu/HUD "vibrancy" surface, expressed
// as an ordinary widget so a layout drops it in like any other.
//
// It renders one of two ways:
//
//   - Native backing. A back-end that can install a real platform vibrancy view
//     (the Cocoa backend's NSVisualEffectView) discovers every Material in the
//     tree with [CollectMaterials], places a system effect view behind each
//     one's Bounds, punches a transparent hole in the framebuffer there, and
//     calls [Material.SetNativeBacked](true). Draw then skips the fallback
//     entirely — the system blur shows through the hole and applies its own
//     material tint — and paints only the child. This is the seam; the mapping
//     from MaterialKind to a system material lives in the back-end, not here, so
//     this package names no platform.
//
//   - Pure-Go fallback. Everywhere else (X11, Wayland, wasm, an image render)
//     the material blurs a caller-supplied Source through a Gaussian and washes
//     a translucent Tint over it. Source is the content behind the material in
//     SURFACE coordinates — a compositor has the desktop wallpaper, a window
//     shell has the pixels drawn before the material — because the Painter has
//     no read-back, exactly as Thumbnail takes an explicit source buffer. The
//     Gaussian is [github.com/go-images/images.GaussianBlur], the fleet's image
//     library, not a kernel written here.
//
// With no Source and no native backing the material degrades to a plain
// translucent panel (just the Tint wash over whatever is already in the buffer),
// which still reads as a material, only without the softened backdrop.
type Material struct {
	Base

	// Kind selects the default blur radius and colour wash, and is what a native
	// back-end maps to its system material. See MaterialKind.
	Kind MaterialKind

	// Blend selects behind-window vs within-window blur. Honoured by a native
	// back-end; the fallback blurs Source either way.
	Blend MaterialBlend

	// Source is the RGBA content behind the material (SW*SH*4 bytes) in surface
	// coordinates, that the fallback blurs. An invalid or nil buffer degrades the
	// fallback to a plain translucent tint.
	Source []byte
	SW, SH int

	// Sigma overrides the Gaussian blur radius. Zero uses the Kind's default;
	// a Kind whose default is 0 (Selection) paints no blur.
	Sigma float64

	// Tint overrides the colour wash. The zero value (A == 0) uses the Kind's
	// default derived from the theme; a non-zero value is used verbatim.
	Tint painter.RGBA

	// Child is composited on top of the material, laid out to fill its Bounds.
	// nil draws no child.
	Child Widget

	// native is set by SetNativeBacked when a back-end has installed a real
	// platform vibrancy view behind this region. It makes Draw skip the fallback
	// blur+tint so the system view shows through.
	native bool

	// cache holds the last blurred crop (r.W*r.H*4), valid for the recorded
	// source, sigma and destination rectangle.
	cache                []byte
	cacheSigma           float64
	cacheX, cacheY       int
	cacheW, cacheH       int
	cacheSrcW, cacheSrcH int
	cacheSrcLen          int
}

// NewMaterial builds a Material of the given kind with no source and no child.
// Set Source (or SetSource) for the fallback blur, and Child for content on top.
func NewMaterial(kind MaterialKind) *Material {
	return &Material{Kind: kind}
}

// SetSource replaces the background buffer the fallback blurs and drops the
// cached blur. The buffer is referenced, not copied; length must be w*h*4.
func (m *Material) SetSource(pixels []byte, w, h int) {
	m.Source, m.SW, m.SH = pixels, w, h
	m.Invalidate()
}

// Invalidate drops the cached blur. Call it after overwriting the contents of
// Source in place; SetSource already does.
func (m *Material) Invalidate() { m.cache, m.cacheW, m.cacheH = nil, 0, 0 }

// SetNativeBacked records whether a back-end has installed a real platform
// vibrancy view behind this material. A back-end calls it after placing (true)
// or removing (false) the system effect view; the default is false (fallback).
func (m *Material) SetNativeBacked(v bool) { m.native = v }

// NativeBacked reports whether a native vibrancy view backs this material.
func (m *Material) NativeBacked() bool { return m.native }

// SetBounds positions the material and lays the child out to fill it.
func (m *Material) SetBounds(r Rect) {
	m.Base.SetBounds(r)
	if m.Child != nil {
		m.Child.SetBounds(r)
	}
}

// A11y marks the material itself as presentational: it is decorative backing,
// so a screen reader looks through it to the child, which the walk still
// descends into (see WalkA11y).
func (m *Material) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// HitTest passes pointer events through the decorative backing to whatever is
// composited behind it UNLESS a child covers the point — the same
// event-transparent idiom as Backdrop, but a material carrying interactive
// content (a sidebar's list) must let that content be clicked.
func (m *Material) HitTest(px, py int) bool {
	if m.Child != nil && m.Child.HitTest(px, py) {
		return true
	}
	return false
}

// OnEvent forwards events to the child (translated to child-local coordinates),
// matching the single-child container convention. A move is forwarded
// unconditionally so the child can clear a hover face; other kinds land only
// when the point is inside the child.
func (m *Material) OnEvent(ev Event) {
	if m.Child == nil {
		return
	}
	pr := m.Bounds()
	cb := m.Child.Bounds()
	if ev.Kind == EventMouseMove {
		m.Child.OnEvent(translateEvent(ev, pr, cb))
		return
	}
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	if cb.Contains(sx, sy) {
		m.Child.OnEvent(translateEvent(ev, pr, cb))
	}
}

// Draw paints the material and its child. See the type doc for the two paths.
func (m *Material) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if m.native {
		// The system effect view shows through the hole the back-end punched and
		// applies its own material tint; paint only the child on top.
		m.drawChild(p, theme)
		return
	}
	sigma := m.effectiveSigma()
	if sigma > 0 && m.hasSource() {
		m.ensureBlur(r, sigma)
		if m.cache != nil {
			blitImage(p, r, r, m.cache, r.W, r.H)
		}
	}
	fillRect(p, r.X, r.Y, r.W, r.H, m.effectiveTint(theme))
	m.drawChild(p, theme)
}

// drawChild paints the child (nil-safe).
func (m *Material) drawChild(p painter.Painter, theme *Theme) {
	if m.Child != nil {
		m.Child.Draw(p, theme)
	}
}

// hasSource reports whether Source is a usable RGBA buffer for its dims.
func (m *Material) hasSource() bool {
	return m.SW > 0 && m.SH > 0 && len(m.Source) >= m.SW*m.SH*4
}

// effectiveSigma is the blur radius: the Sigma override if positive, else the
// Kind's default.
func (m *Material) effectiveSigma() float64 {
	if m.Sigma > 0 {
		return m.Sigma
	}
	return materialSigma(m.Kind)
}

// effectiveTint is the colour wash: the Tint override if opaque enough to show
// (A != 0), else the Kind's default derived from the theme.
func (m *Material) effectiveTint(theme *Theme) painter.RGBA {
	if m.Tint.A != 0 {
		return m.Tint
	}
	return materialTint(m.Kind, theme)
}

// materialSigma is the default Gaussian sigma for a kind. Selection has none.
func materialSigma(kind MaterialKind) float64 {
	switch kind {
	case MaterialWindowBackground:
		return 18
	case MaterialSidebar:
		return 20
	case MaterialTitlebar:
		return 16
	case MaterialMenu, MaterialPopover:
		return 14
	case MaterialHUD:
		return 24
	default: // MaterialSelection and any future no-blur kind
		return 0
	}
}

// materialTint is the default translucent wash for a kind. Every value is
// semi-transparent (A neither 0 nor 0xFF) so the blurred backdrop shows through
// and the pixel back-end src-over composites it. HUD is a fixed dark wash,
// theme-independent; the rest derive from the theme so a material reads under
// any palette.
func materialTint(kind MaterialKind, theme *Theme) painter.RGBA {
	switch kind {
	case MaterialWindowBackground:
		return withAlpha(theme.Background, 160)
	case MaterialSidebar:
		return withAlpha(theme.SurfaceAlt, 190)
	case MaterialTitlebar:
		return withAlpha(theme.Background, 150)
	case MaterialMenu, MaterialPopover:
		return withAlpha(theme.Surface, 205)
	case MaterialHUD:
		return painter.RGBA{R: 28, G: 28, B: 30, A: 190}
	default: // MaterialSelection
		return withAlpha(theme.Accent, 90)
	}
}

// ensureBlur fills m.cache with the r-sized Gaussian blur of Source under r,
// recomputing only when the source, sigma or destination rectangle changed. The
// blur reads a padded region of Source (3*sigma each side, clamped to the source
// edge) so the result at r's boundary is the true blur, not darkened by a false
// edge, then crops the r-portion back out.
func (m *Material) ensureBlur(r Rect, sigma float64) {
	if m.blurValid(r, sigma) {
		return
	}
	pad := int(math.Ceil(3 * sigma))
	ew, eh := r.W+2*pad, r.H+2*pad
	packed := make([]byte, ew*eh*4)
	for ey := 0; ey < eh; ey++ {
		sy := clampInt(r.Y-pad+ey, 0, m.SH-1)
		for ex := 0; ex < ew; ex++ {
			sx := clampInt(r.X-pad+ex, 0, m.SW-1)
			s := (sy*m.SW + sx) * 4
			d := (ey*ew + ex) * 4
			copy(packed[d:d+4], m.Source[s:s+4])
		}
	}
	blurred, err := images.GaussianBlur(&image.RGBA{
		Pix:    packed,
		Stride: ew * 4,
		Rect:   image.Rect(0, 0, ew, eh),
	}, sigma)
	if err != nil {
		// sigma > 0 is guaranteed by the caller, which is the only error
		// GaussianBlur returns; fall back to no backdrop rather than panic.
		m.cache, m.cacheW, m.cacheH = nil, 0, 0
		return
	}
	crop := make([]byte, r.W*r.H*4)
	for dy := 0; dy < r.H; dy++ {
		srow := ((pad+dy)*ew + pad) * 4
		drow := dy * r.W * 4
		copy(crop[drow:drow+r.W*4], blurred.Pix[srow:srow+r.W*4])
	}
	m.cache = crop
	m.cacheSigma = sigma
	m.cacheX, m.cacheY, m.cacheW, m.cacheH = r.X, r.Y, r.W, r.H
	m.cacheSrcW, m.cacheSrcH, m.cacheSrcLen = m.SW, m.SH, len(m.Source)
}

// blurValid reports whether the cache was made from this source, sigma and
// destination rectangle.
func (m *Material) blurValid(r Rect, sigma float64) bool {
	return m.cache != nil &&
		m.cacheSigma == sigma &&
		m.cacheX == r.X && m.cacheY == r.Y &&
		m.cacheW == r.W && m.cacheH == r.H &&
		m.cacheSrcW == m.SW && m.cacheSrcH == m.SH && m.cacheSrcLen == len(m.Source)
}

// MaterialSpec is one material's placement and role, as a native back-end reads
// it: what system material to install (Kind), how to blend it (Blend) and where
// (Rect, in surface coordinates). It is the plain-data view returned alongside
// the widget by [CollectMaterials].
type MaterialSpec struct {
	Kind  MaterialKind
	Blend MaterialBlend
	Rect  Rect
}

// CollectMaterials returns every Material in the tree rooted at root, in visual
// order. A native back-end calls it each frame to reconcile its system vibrancy
// views: for each returned material it reads Kind/Blend/Bounds, installs or
// moves an effect view, and calls SetNativeBacked so the fallback stands down.
//
// It descends the same way WalkA11y does — through any widget exposing its
// children — so a material nested in a layout is still found. Bounds are surface
// coordinates (this toolkit's bounds are absolute), which is what a back-end
// compositing native views needs.
func CollectMaterials(root Widget) []*Material {
	var out []*Material
	var walk func(w Widget)
	walk = func(w Widget) {
		if w == nil {
			return
		}
		if m, ok := w.(*Material); ok {
			out = append(out, m)
		}
		if c, ok := w.(childContainer); ok {
			for _, child := range c.Children() {
				walk(child)
			}
		}
	}
	walk(root)
	return out
}

// Spec returns the material's placement and role as plain data.
func (m *Material) Spec() MaterialSpec {
	return MaterialSpec{Kind: m.Kind, Blend: m.Blend, Rect: m.Bounds()}
}

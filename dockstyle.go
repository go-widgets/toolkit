// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// DockItemState is the per-item information a DockStyle needs to paint an item's
// face and its running/active indicators. GlyphBox is where the widget will draw
// the icon, so a style can centre an indicator (e.g. a running dot) under it.
type DockItemState struct {
	Active   bool
	Running  bool
	GlyphBox Rect
}

// DockStyle paints an AppDock's decorative surfaces — the ground bar and each
// item's face plus its running/active indicators — so the same dock model,
// layout, magnification and hit-testing can wear different looks, exactly the
// way PostCard / GroupCard give one card model different faces. The widget draws
// the content (icon, label, attention badge); the style owns the chrome and
// returns the ink the content should use so a dark face can pick a light ink.
//
// Ship-with styles: ModernDockStyle (macOS — flat rounded faces, a dot under a
// running icon), BevelDockStyle (Fluxbox — raised/sunken 3D bevels) and
// WindowsDockStyle (taskbar — flat buttons with an accent underline). A host may
// implement its own.
type DockStyle interface {
	// DrawGround paints the bar background over r.
	DrawGround(p painter.Painter, theme *Theme, r Rect)
	// DrawFace paints one item's face and its running/active indicators over r,
	// returning the ink the widget should use for that item's icon + label.
	DrawFace(p painter.Painter, theme *Theme, r Rect, st DockItemState) (ink RGBA)
}

// fillBackdrop is the shared one-liner for a solid (optionally rounded) fill via
// a Backdrop, so every style composes its surfaces from the widget rather than
// hand-drawing them.
func fillBackdrop(p painter.Painter, theme *Theme, r Rect, fill RGBA, radius int) {
	b := &Backdrop{Fill: fill, Radius: radius}
	b.SetBounds(r)
	b.Draw(p, theme)
}

// dockRunningDot paints a small round "app is open" dot centred under the glyph
// along the face's bottom edge — the macOS/GNOME running marker, shared by the
// Modern and Bevel styles.
func dockRunningDot(p painter.Painter, theme *Theme, glyph, face Rect, ink RGBA) {
	dot := scaled(appDockRunDot)
	fillBackdrop(p, theme, Rect{
		X: glyph.X + (glyph.W-dot)/2, Y: face.Y + face.H - dot - scaled(1), W: dot, H: dot,
	}, ink, dot/2)
}

// ModernDockStyle is the macOS look: a SurfaceAlt ground and flat, rounded item
// faces — Surface at rest, Accent when active — with a running dot under the
// icon. It is the AppDock default.
type ModernDockStyle struct{}

func (ModernDockStyle) DrawGround(p painter.Painter, theme *Theme, r Rect) {
	fillBackdrop(p, theme, r, theme.SurfaceAlt, 0)
}

func (ModernDockStyle) DrawFace(p painter.Painter, theme *Theme, r Rect, st DockItemState) RGBA {
	face, ink := theme.Surface, theme.OnSurface
	if st.Active {
		face, ink = theme.Accent, accentInk(theme)
	}
	fillBackdrop(p, theme, r, face, scaled(appDockRadius))
	if st.Running {
		dockRunningDot(p, theme, st.GlyphBox, r, ink)
	}
	return ink
}

// BevelDockStyle is the Fluxbox look: a flat ground and square, 3D-bevelled item
// faces — a RAISED bevel at rest, a SUNKEN one when active (the "pressed in"
// current app) — with a running dot. Ink stays OnSurface on the light face.
type BevelDockStyle struct{}

func (BevelDockStyle) DrawGround(p painter.Painter, theme *Theme, r Rect) {
	fillBackdrop(p, theme, r, theme.SurfaceAlt, 0)
}

func (BevelDockStyle) DrawFace(p painter.Painter, theme *Theme, r Rect, st DockItemState) RGBA {
	fillBackdrop(p, theme, r, theme.Surface, 0)
	// Highlight/shadow edges derived from the face by lifting toward white and
	// sinking toward black, so the bevel reads on any theme's Surface.
	hi := blendRGBA(RGBA{R: 255, G: 255, B: 255, A: 255}, theme.Surface, 0.5)
	lo := blendRGBA(RGBA{A: 255}, theme.Surface, 0.4)
	if st.Active {
		drawSunkenBevel(p, r, hi, lo)
	} else {
		drawRaisedBevel(p, r, hi, lo)
	}
	if st.Running {
		dockRunningDot(p, theme, st.GlyphBox, r, theme.OnSurface)
	}
	return theme.OnSurface
}

// WindowsDockStyle is the taskbar look: a flat SurfaceAlt bar and flat,
// rectangular buttons — transparent at rest, a subtle highlight when running,
// an accent-tinted highlight when active — each running/active button carrying a
// centred accent underline (short for a background app, wider for the current
// one), the Windows 10/11 running marker.
type WindowsDockStyle struct{}

func (WindowsDockStyle) DrawGround(p painter.Painter, theme *Theme, r Rect) {
	fillBackdrop(p, theme, r, theme.SurfaceAlt, 0)
}

func (WindowsDockStyle) DrawFace(p painter.Painter, theme *Theme, r Rect, st DockItemState) RGBA {
	switch {
	case st.Active:
		fillBackdrop(p, theme, r, blendRGBA(theme.Accent, theme.SurfaceAlt, 0.22), 0)
	case st.Running:
		fillBackdrop(p, theme, r, blendRGBA(theme.Surface, theme.SurfaceAlt, 0.5), 0)
	}
	if st.Running || st.Active {
		barW := r.W / 3
		if st.Active {
			barW = r.W * 3 / 5
		}
		bh := scaled(2)
		fillBackdrop(p, theme, Rect{X: r.X + (r.W-barW)/2, Y: r.Y + r.H - bh, W: barW, H: bh}, theme.Accent, bh/2)
	}
	return theme.OnSurface
}

// drawRaisedBevel paints a 1-pixel raised bevel around r: a bright top + left
// and a dark bottom + right, so the face reads as pushed out toward the viewer.
func drawRaisedBevel(p painter.Painter, r Rect, hi, lo RGBA) {
	fillRect(p, r.X, r.Y, r.W, 1, hi)       // top
	fillRect(p, r.X, r.Y, 1, r.H, hi)       // left
	fillRect(p, r.X, r.Y+r.H-1, r.W, 1, lo) // bottom
	fillRect(p, r.X+r.W-1, r.Y, 1, r.H, lo) // right
}

// drawSunkenBevel paints a 1-pixel sunken bevel around r: dark top + left, bright
// bottom + right — the inverse of drawRaisedBevel — so the face reads as pressed
// in (the active/current item).
func drawSunkenBevel(p painter.Painter, r Rect, hi, lo RGBA) {
	fillRect(p, r.X, r.Y, r.W, 1, lo)
	fillRect(p, r.X, r.Y, 1, r.H, lo)
	fillRect(p, r.X, r.Y+r.H-1, r.W, 1, hi)
	fillRect(p, r.X+r.W-1, r.Y, 1, r.H, hi)
}

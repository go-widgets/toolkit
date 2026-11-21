// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package iconoir draws any glyph of the Iconoir set through the toolkit's
// [toolkit.SVGIcon] renderer.
//
// It is a package of its own because go-icons/iconoir embeds the whole set
// (about 1,500 SVGs) behind one //go:embed, which the linker cannot trim:
// importing it costs every glyph, used or not (go-widgets/toolkit#588). The
// toolkit itself carries only the glyphs its widgets draw. Import this package
// when a program needs arbitrary Iconoir names at run time; a program that
// draws a known handful is smaller handing their SVGs to [toolkit.SVGIcon].
//
// It replaces toolkit.DrawIconoir and toolkit.IconoirNames.
package iconoir

import (
	pack "github.com/go-icons/iconoir"
	"github.com/go-widgets/painter"
	"github.com/go-widgets/toolkit"
)

// Draw paints the named Iconoir glyph into r, recoloured to ink. An unknown
// name draws nothing.
func Draw(p painter.Painter, r toolkit.Rect, name string, ink toolkit.RGBA) {
	toolkit.SVGIcon(pack.Icon(name))(p, r, ink)
}

// Drawer returns a drawer for the named glyph, for a slot that takes a
// func(painter.Painter, toolkit.Rect, toolkit.RGBA) (Button.Icon,
// MenuItem.Icon, TreeTableNode.Icon…). An unknown name draws nothing.
func Drawer(name string) func(p painter.Painter, r toolkit.Rect, ink toolkit.RGBA) {
	return toolkit.SVGIcon(pack.Icon(name))
}

// Names is the sorted list of glyph names [Draw] accepts.
func Names() []string { return pack.Names() }

// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Toolbar is a horizontal strip of square icon-buttons + optional
// separators. Each entry has a Label (used as the fallback glyph
// character), an optional Icon (drawn as an RGBA blit when non-empty),
// an OnClick callback + a Disabled flag.
//
// Toolbar is the icon-strip that sits below a MenuBar; it composes
// cleanly with both Notebook + Statusbar so a "stock GTK" window can
// be assembled out of MenuBar + Toolbar + Notebook + Statusbar.
type Toolbar struct {
	Base
	Items    []ToolbarItem
	ButtonW  int // default ToolbarButtonW
	ButtonH  int // default ToolbarButtonH
	pressIdx int // -1 = none; set on the last click for visual feedback
}

// ToolbarItem is one cell in a Toolbar.
type ToolbarItem struct {
	Label    string
	Icon     []byte // optional ButtonW x ButtonH RGBA; nil = draw Label initial
	OnClick  func()
	Disabled bool

	// Separator, when true, draws a 1-pixel vertical divider instead of
	// a button. Label/Icon/OnClick are ignored.
	Separator bool
}

// Sizing constants. Square buttons read as a true icon-toolbar (vs the
// MenuBar's wider text cells).
const (
	ToolbarButtonW = 24
	ToolbarButtonH = 24
	ToolbarSepW    = 8
)

// NewToolbar builds a Toolbar with the given items.
func NewToolbar(items []ToolbarItem) *Toolbar {
	return &Toolbar{Items: items, ButtonW: ToolbarButtonW, ButtonH: ToolbarButtonH, pressIdx: -1}
}

// Draw paints the toolbar strip.
func (t *Toolbar) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	bw := t.ButtonW
	if bw <= 0 {
		bw = ToolbarButtonW
	}
	bh := t.ButtonH
	if bh <= 0 {
		bh = ToolbarButtonH
	}
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	x := r.X
	for i, it := range t.Items {
		if it.Separator {
			midX := x + ToolbarSepW/2
			fillRect(p, midX, r.Y+3, 1, bh-6, theme.Border)
			x += ToolbarSepW
			continue
		}
		bg := theme.Surface
		switch {
		case it.Disabled:
			bg = theme.Surface
		case i == t.pressIdx:
			bg = theme.Accent
		}
		fillRect(p, x, r.Y, bw, bh, bg)
		strokeRect(p, x, r.Y, bw, bh, theme.Border)
		if len(it.Icon) >= 4*bw*bh {
			blitRGBA(p, x, r.Y, bw, bh, it.Icon)
		} else {
			label := it.Label
			if label == "" {
				label = "?"
			}
			ch := string(label[0])
			tx := x + (bw-TextWidth(ch))/2
			ty := r.Y + (bh-GlyphHeight)/2
			ink := theme.OnSurface
			if it.Disabled {
				ink = theme.Border
			} else if i == t.pressIdx {
				ink = theme.Background
			}
			DrawText(p, tx, ty, ch, ink)
		}
		x += bw
	}
}

// OnEvent dispatches click events to the matching item.
func (t *Toolbar) OnEvent(ev Event) {
	if ev.Kind != EventClick {
		return
	}
	idx := t.hitTest(ev.X, ev.Y)
	if idx < 0 {
		return
	}
	it := t.Items[idx]
	if it.Separator || it.Disabled {
		return
	}
	t.pressIdx = idx
	if it.OnClick != nil {
		it.OnClick()
	}
}

func (t *Toolbar) hitTest(x, y int) int {
	bw := t.ButtonW
	if bw <= 0 {
		bw = ToolbarButtonW
	}
	bh := t.ButtonH
	if bh <= 0 {
		bh = ToolbarButtonH
	}
	if y < 0 || y >= bh {
		return -1
	}
	cx := 0
	for i, it := range t.Items {
		w := bw
		if it.Separator {
			w = ToolbarSepW
		}
		if x >= cx && x < cx+w {
			if it.Separator {
				return -1
			}
			return i
		}
		cx += w
	}
	return -1
}

// blitRGBA copies src (a w*h RGBA buffer) into the Painter at (x, y).
func blitRGBA(p painter.Painter, x, y, w, h int, src []byte) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			soff := (j*w + i) * 4
			if soff+3 >= len(src) {
				return
			}
			ink := RGBA{R: src[soff], G: src[soff+1], B: src[soff+2], A: src[soff+3]}
			putPixel(p, x+i, y+j, ink)
		}
	}
}

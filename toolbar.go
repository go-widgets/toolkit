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
	Items   []ToolbarItem
	ButtonW int // default ToolbarButtonW
	ButtonH int // default ToolbarButtonH
	// Orientation lays the buttons out left-to-right (Horizontal, the zero
	// value) or top-to-bottom (Vertical). A vertical toolbar draws its
	// separators as horizontal dividers, so the same Items slice works as a
	// side rail without change.
	Orientation Orientation
	pressIdx    int // -1 = none; set on the last click for visual feedback
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

// NewToolbar builds a Toolbar with the given items. ButtonW/ButtonH are left at
// zero — "use the default" — so the default square size resolves through scaled
// at draw time and grows with HiDPI and touch density; a caller that wants a
// fixed cell sets ButtonW/ButtonH explicitly (honoured verbatim, like
// CheckButton.Size). At compact/1x the resolved default is ToolbarButtonW ×
// ToolbarButtonH, byte-identical to before.
func NewToolbar(items []ToolbarItem) *Toolbar {
	return &Toolbar{Items: items, pressIdx: -1}
}

// buttonW / buttonH resolve the effective cell size: an explicit ButtonW/ButtonH
// verbatim, else the scaled default. sepW is the scaled separator step. All three
// equal their logical constants at compact/1x, keeping layout byte-identical.
func (t *Toolbar) buttonW() int {
	if t.ButtonW > 0 {
		return t.ButtonW
	}
	return scaled(ToolbarButtonW)
}

func (t *Toolbar) buttonH() int {
	if t.ButtonH > 0 {
		return t.ButtonH
	}
	return scaled(ToolbarButtonH)
}

func (t *Toolbar) sepW() int { return scaled(ToolbarSepW) }

// Draw paints the toolbar strip.
func (t *Toolbar) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	bw, bh, sepW := t.buttonW(), t.buttonH(), t.sepW()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	vertical := t.Orientation == Vertical
	// pos advances along the layout axis; the cross-axis origin is fixed.
	pos := r.X
	if vertical {
		pos = r.Y
	}
	for i, it := range t.Items {
		bx, by := pos, r.Y
		if vertical {
			bx, by = r.X, pos
		}
		if it.Separator {
			if vertical {
				// Horizontal divider spanning the button width.
				fillRect(p, bx+3, by+sepW/2, bw-6, 1, theme.Border)
			} else {
				fillRect(p, bx+sepW/2, by+3, 1, bh-6, theme.Border)
			}
			pos += sepW
			continue
		}
		bg := theme.Surface
		switch {
		case it.Disabled:
			bg = theme.Surface
		case i == t.pressIdx:
			bg = theme.Accent
		}
		fillRect(p, bx, by, bw, bh, bg)
		strokeRect(p, bx, by, bw, bh, theme.Border)
		if len(it.Icon) >= 4*bw*bh {
			blitRGBA(p, bx, by, bw, bh, it.Icon)
		} else {
			label := it.Label
			if label == "" {
				label = "?"
			}
			ch := string(label[0])
			tx := bx + (bw-t.textWidth(ch))/2
			ty := by + (bh-t.glyphHeight())/2
			ink := theme.OnSurface
			if it.Disabled {
				ink = theme.Border
			} else if i == t.pressIdx {
				ink = theme.Background
			}
			t.drawText(p, tx, ty, ch, ink)
		}
		if vertical {
			pos += bh
		} else {
			pos += bw
		}
	}
}

// OnEvent dispatches click events to the matching item.
func (t *Toolbar) OnEvent(ev Event) {
	if ev.Kind != EventClick || t.Disabled().Get() {
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
	bw, bh, sepW := t.buttonW(), t.buttonH(), t.sepW()
	// along is the coordinate on the layout axis, cross on the other one.
	along, cross, crossExtent := x, y, bh
	if t.Orientation == Vertical {
		along, cross, crossExtent = y, x, bw
	}
	if cross < 0 || cross >= crossExtent {
		return -1
	}
	c := 0
	for i, it := range t.Items {
		step := t.itemStep(it, bw, bh, sepW)
		if along >= c && along < c+step {
			if it.Separator {
				return -1
			}
			return i
		}
		c += step
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

// itemRect is the rectangle item i occupies, in the same surface coordinates
// Draw paints it at (a separator is a divider of sepW along the axis). An index
// outside Items answers the empty rectangle.
func (t *Toolbar) itemRect(i int) Rect {
	if i < 0 || i >= len(t.Items) {
		return Rect{}
	}
	r := t.Bounds()
	bw, bh, sepW := t.buttonW(), t.buttonH(), t.sepW()
	vertical := t.Orientation == Vertical
	pos := 0
	for j := 0; j < i; j++ {
		pos += t.itemStep(t.Items[j], bw, bh, sepW)
	}
	if t.Items[i].Separator {
		bw, bh = sepW, sepW
		if vertical {
			bw = t.buttonW()
		} else {
			bh = t.buttonH()
		}
	}
	if vertical {
		return Rect{X: r.X, Y: r.Y + pos, W: bw, H: bh}
	}
	return Rect{X: r.X + pos, Y: r.Y, W: bw, H: bh}
}

// itemStep is how far item it advances the layout axis.
func (t *Toolbar) itemStep(it ToolbarItem, bw, bh, sepW int) int {
	switch {
	case it.Separator:
		return sepW
	case t.Orientation == Vertical:
		return bh
	}
	return bw
}

// Children exposes each button as a synthetic, non-drawing node so the generic
// accessibility walk (WalkA11y) descends the toolbar into a toolbar -> button
// structure: a screen reader hears each button's name and disabled state, not
// just "toolbar, N items". Separators are decoration and get no node. Like
// TabBar's tab nodes they are rebuilt on demand from the current items and
// geometry, draw nothing and handle no events; the Toolbar paints and routes
// every item itself. The toolbar's own Disabled reaches them through the walk
// (see a11yDisablesSubtree), so only the item's own flag is read here.
func (t *Toolbar) Children() []Widget {
	out := make([]Widget, 0, len(t.Items))
	for i, it := range t.Items {
		if it.Separator {
			continue
		}
		node := &toolbarItemNode{item: it}
		node.SetBounds(t.itemRect(i))
		out = append(out, node)
	}
	return out
}

// toolbarItemNode is the synthetic stand-in for one Toolbar button. The Toolbar
// has no per-item keyboard focus, so Focused is never set.
type toolbarItemNode struct {
	Base
	item ToolbarItem
}

// A11y reports the node as a button named by its label, disabled when the item
// is.
func (n *toolbarItemNode) A11y() A11yInfo {
	return A11yInfo{Role: RoleButton, Name: n.item.Label, Disabled: n.item.Disabled}
}

var _ Accessible = (*toolbarItemNode)(nil)

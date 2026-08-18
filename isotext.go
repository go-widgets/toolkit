// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"

	"github.com/go-gfx/gfx/iso"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// --- selection ----------------------------------------------------------

// SelectedText returns the selected text annotation's ID, or "" when none is
// selected.
func (d *IsoDiagram) SelectedText() string { return d.selText.Get() }

// SelectedTextObservable exposes the text-selection property so a host can bind
// it into an MVVM view model (e.g. a text-properties inspector) that stays in
// sync with clicks in the diagram, rather than polling [IsoDiagram.SelectedText]
// every frame.
func (d *IsoDiagram) SelectedTextObservable() *mvvm.Observable[string] { return d.selText }

// SelectText selects the text annotation with the given id (or clears the text
// selection when id is ""). Selecting a text clears any node, connector or zone
// selection so only one entity ever highlights at once.
func (d *IsoDiagram) SelectText(id string) {
	if id != "" {
		d.clearOtherSelections(isoSelText)
	}
	d.selText.Set(id)
}

// --- geometry -----------------------------------------------------------

// textFont is the font a text annotation draws and measures in: the widget's
// effective font when the annotation left its Size at zero, otherwise the
// built-in bitmap at Size routed through the toolkit HiDPI/density scale — so a
// sized annotation scales in lockstep with the rest of the chrome.
func (d *IsoDiagram) textFont(t IsoText) Font {
	if t.Size <= 0 {
		return d.EffectiveFont()
	}
	return NewBitmapFont(scaled(t.Size))
}

// textInk is a text annotation's ink: its own Color, or the theme's OnSurface
// when it left its colour unset (A==0).
func (d *IsoDiagram) textInk(t IsoText, theme *Theme) RGBA {
	if t.Color.A == 0 {
		return theme.OnSurface
	}
	return t.Color
}

// textBox is the buffer-local pixel rectangle a text annotation occupies: its
// string, measured in its own font, centred on the projected ground centre of
// its anchor cell. An empty caption still gets a one-glyph box so it stays
// selectable and movable.
func (d *IsoDiagram) textBox(t IsoText) Rect {
	f := d.textFont(t)
	anchor := d.proj.Project(iso.V(float64(t.X)+0.5, float64(t.Y)+0.5, 0))
	tw := f.Measure(t.Text)
	if tw < f.Advance() {
		tw = f.Advance()
	}
	th := f.Height()
	return Rect{X: iround(anchor.X) - tw/2, Y: iround(anchor.Y) - th/2, W: tw, H: th}
}

// --- rendering (top layer) ----------------------------------------------

// drawTexts paints every text annotation over the whole scene (the topmost
// layer) in its own font and ink, and outlines the selected one — all in
// painter space, reusing the toolkit's own text rendering rather than
// hand-rolling glyphs.
func (d *IsoDiagram) drawTexts(p painter.Painter, b Rect, theme *Theme) {
	sel := d.selText.Get()
	for _, t := range d.doc.Texts() {
		box := d.textBox(t)
		if t.Text != "" {
			d.textFont(t).Draw(p, b.X+box.X, b.Y+box.Y, t.Text, d.textInk(t, theme))
		}
		if t.ID == sel {
			strokeRect(p, b.X+box.X, b.Y+box.Y, box.W, box.H, theme.Accent)
		}
	}
}

// --- hit testing --------------------------------------------------------

// textAtLocal returns the id of the topmost text annotation whose box contains
// widget-local (x, y), and whether one was hit. Later-inserted annotations
// (drawn on top) win, so the scan runs in reverse insertion order.
func (d *IsoDiagram) textAtLocal(x, y int) (string, bool) {
	texts := d.doc.Texts()
	for i := len(texts) - 1; i >= 0; i-- {
		box := d.textBox(texts[i])
		if x >= box.X && y >= box.Y && x < box.X+box.W && y < box.Y+box.H {
			return texts[i].ID, true
		}
	}
	return "", false
}

// --- move gesture -------------------------------------------------------

// beginTextMove arms a text-annotation move: it records the annotation's base
// cell and the ground cell under the press so it follows the pointer by a cell
// delta.
func (d *IsoDiagram) beginTextMove(id string, x, y int) {
	t, _ := d.doc.Text(id)
	d.gesture = isoGestureTextMove
	d.dragText = id
	d.grabEntX, d.grabEntY = t.X, t.Y
	d.grabCellX, d.grabCellY = d.cellAtLocal(x, y)
}

// moveTextDragTo moves the annotation being dragged so its anchor tracks the
// pointer by the cell delta from the grab.
func (d *IsoDiagram) moveTextDragTo(x, y int) {
	t, ok := d.doc.Text(d.dragText)
	if !ok {
		return
	}
	gx, gy := d.cellAtLocal(x, y)
	nx := d.grabEntX + (gx - d.grabCellX)
	ny := d.grabEntY + (gy - d.grabCellY)
	if t.X == nx && t.Y == ny {
		return
	}
	t.X, t.Y = nx, ny
	d.doc.PutText(t)
}

// --- editing commands ---------------------------------------------------

// nextTextID returns a text-annotation-unique id.
func (d *IsoDiagram) nextTextID() string {
	for {
		d.seq++
		id := "t" + strconv.Itoa(d.seq)
		if _, ok := d.doc.Text(id); ok {
			continue
		}
		return id
	}
}

// commitPlaceText drops a new (empty-captioned) text annotation at ground cell
// (gx, gy), selects it and returns its id — as one undoable command. The host
// fills the caption in with [IsoDiagram.SetSelectedTextContent].
func (d *IsoDiagram) commitPlaceText(gx, gy int) string {
	d.beginEdit()
	id := d.nextTextID()
	d.doc.PutText(IsoText{ID: id, X: gx, Y: gy})
	d.SelectText(id)
	d.invalidate()
	return id
}

// commitDeleteText removes a text annotation as an undoable command, clearing
// the selection if it was selected.
func (d *IsoDiagram) commitDeleteText(id string) {
	if _, ok := d.doc.Text(id); !ok {
		return
	}
	d.beginEdit()
	d.doc.RemoveText(id)
	if d.selText.Get() == id {
		d.selText.Set("")
	}
	d.invalidate()
}

// editText applies mutate to the annotation with id as one undoable edit, unless
// it is missing or mutate reports no change. It returns whether it changed
// anything.
func (d *IsoDiagram) editText(id string, mutate func(*IsoText) bool) bool {
	t, ok := d.doc.Text(id)
	if !ok {
		return false
	}
	if !mutate(&t) {
		return false
	}
	d.beginEdit()
	d.doc.PutText(t)
	d.invalidate()
	return true
}

// SetTextContent sets an annotation's caption (undoable). Returns false when the
// annotation is absent or already carries that text.
func (d *IsoDiagram) SetTextContent(id, text string) bool {
	return d.editText(id, func(t *IsoText) bool {
		if t.Text == text {
			return false
		}
		t.Text = text
		return true
	})
}

// SetTextColor sets an annotation's ink (undoable); pass a zero colour (A==0) to
// fall back to the theme OnSurface. Returns false when the annotation is absent
// or already has that colour.
func (d *IsoDiagram) SetTextColor(id string, col RGBA) bool {
	return d.editText(id, func(t *IsoText) bool {
		if t.Color == col {
			return false
		}
		t.Color = col
		return true
	})
}

// SetTextSize sets an annotation's type scale (undoable); zero uses the widget's
// effective font. Returns false when the annotation is absent or already that
// size.
func (d *IsoDiagram) SetTextSize(id string, size int) bool {
	return d.editText(id, func(t *IsoText) bool {
		if t.Size == size {
			return false
		}
		t.Size = size
		return true
	})
}

// SetTextPos sets an annotation's anchor cell (undoable). Returns false when the
// annotation is absent or already at that cell.
func (d *IsoDiagram) SetTextPos(id string, x, y int) bool {
	return d.editText(id, func(t *IsoText) bool {
		if t.X == x && t.Y == y {
			return false
		}
		t.X, t.Y = x, y
		return true
	})
}

// SetSelectedTextContent applies [IsoDiagram.SetTextContent] to the selected
// annotation (a no-op returning false when none is selected).
func (d *IsoDiagram) SetSelectedTextContent(text string) bool {
	return d.SetTextContent(d.selText.Get(), text)
}

// SetSelectedTextColor applies [IsoDiagram.SetTextColor] to the selected
// annotation.
func (d *IsoDiagram) SetSelectedTextColor(col RGBA) bool {
	return d.SetTextColor(d.selText.Get(), col)
}

// SetSelectedTextSize applies [IsoDiagram.SetTextSize] to the selected
// annotation.
func (d *IsoDiagram) SetSelectedTextSize(size int) bool {
	return d.SetTextSize(d.selText.Get(), size)
}

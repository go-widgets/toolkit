// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"slices"
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// TagField is a token / multi-tag text input: the user types into an
// in-progress buffer (Text) and each committed value becomes an inline
// removable pill (a Chip). Tokens flow left-to-right and wrap to a new
// row when the next one would overflow the widget's width; after the
// last token the in-progress Text is drawn with a caret, or -- when
// there are no tags and no text -- the muted Placeholder hint.
//
// Editing mirrors the toolkit's other text widgets: EventChar appends a
// rune to Text; Enter (or a comma) commits strings.TrimSpace(Text) as a
// new tag, skipping blank + duplicate values; Backspace on an empty Text
// removes the last tag; and a click on a token's "x" close slot removes
// that specific tag. Every change to the tag set Sets the Tags()
// [mvvm.Observable], so hosts subscribe / bind to it instead of a callback.
//
// Each token is rendered by reusing the Chip widget (Closable: true) so
// the pill body, label + close "x" all match a standalone Chip exactly;
// hit-testing routes the click through the very same Chip.OnEvent against
// a rectangle computed the same way Draw lays it out, so the visible "x"
// and its click target never drift apart.
type TagField struct {
	Base
	// tags is the committed token set (in insertion order) as a shared
	// [mvvm.Observable]; reached only through Tags() so a Set is the sole way
	// to change it and every subscriber/binding fires.
	tags *mvvm.Observable[[]string]
	// text is the in-progress input (shown with a caret after the last tag) as
	// a shared [mvvm.Observable]; reached only through Text().
	text *mvvm.Observable[string]
	// Placeholder is the muted hint drawn when there are no tags and no text.
	Placeholder string
	// focusState carries the keyboard focus flag (set true on click) and draws
	// the focus ring; it lets a host / container route subsequent keyboard input
	// to the field.
	focusState
}

// Tags is the committed token set (insertion order) as a shared
// [mvvm.Observable]: a host binds it two-way (or subscribes) instead of
// touching a field, and every commit / backspace / close Sets it — so a Set is
// the only way to change the tag set and there is no separate change callback.
// The Observable dedups with [slices.Equal], so Setting the current value
// notifies nobody. Lazily created so a bare &TagField{} works.
func (t *TagField) Tags() *mvvm.Observable[[]string] {
	if t.tags == nil {
		t.tags = mvvm.NewObservableEq(nil, func(a, b []string) bool { return slices.Equal(a, b) })
	}
	return t.tags
}

// Text is the in-progress input as a shared [mvvm.Observable]: character input
// and commits go through it, so a Set is the only way to change it. Lazily
// created so a bare &TagField{} works.
func (t *TagField) Text() *mvvm.Observable[string] {
	if t.text == nil {
		t.text = mvvm.NewObservable("")
	}
	return t.text
}

// tagFieldHGap / tagFieldVGap are the pixel gaps between tokens on a row
// and between wrapped rows respectively -- mirroring FlowLayout's HGap /
// VGap so a wrapping row of TagField pills reads like any other flow of
// chips.
const (
	tagFieldHGap = 4
	tagFieldVGap = 4
)

// NewTagField builds a TagField seeded with the given tags (a nil / empty
// slice is fine) and an empty in-progress Text.
func NewTagField(tags ...string) *TagField {
	t := &TagField{}
	t.Tags().Set(tags)
	return t
}

// HitRect is the TagField's field-level tap target: Bounds clamped up to the
// touch minimum on each axis and centred, byte-identical to Bounds at
// [DensityCompact]. The per-token close "x" slots are drawn and hit-tested by the
// reused Chip (see OnEvent), so their own touch behaviour follows Chip's scaling
// in lockstep rather than being re-derived here — keeping the token pills and
// their hit rects consistent with a standalone Chip at every density.
func (t *TagField) HitRect() Rect { return touchHitRect(t.Bounds()) }

// chipWidth is the auto-sized width a closable Chip carrying tag would
// take -- the same formula Chip.Draw uses (text + horizontal pads +
// close gap + close slot), measured in this widget's effective font so
// layout and the reused Chip agree.
func (t *TagField) chipWidth(tag string) int {
	return t.textWidth(tag) + 2*ChipPadX + ChipCloseGap + ChipCloseW
}

// layout flows the tokens from origin (ox, oy), wrapping when the next
// token would overflow the widget's Bounds width, and returns each
// token's rectangle plus the pen position (endX, endY) where the
// in-progress Text / Placeholder is drawn. Draw calls it with the
// surface origin (Bounds X/Y); OnEvent calls it with (0, 0) because the
// events it hit-tests are already widget-local.
func (t *TagField) layout(ox, oy int) (rects []Rect, endX, endY int) {
	w := t.Bounds().W
	rowH := t.glyphHeight() + 2*ChipPadY
	x, y := ox, oy
	for _, tag := range t.Tags().Get() {
		cw := t.chipWidth(tag)
		if x > ox && x+cw > ox+w {
			x = ox
			y += rowH + tagFieldVGap
		}
		rects = append(rects, Rect{X: x, Y: y, W: cw, H: rowH})
		x += cw + tagFieldHGap
	}
	return rects, x, y
}

// Draw flows each tag as a closable Chip, then draws the in-progress Text
// with a caret, or the muted Placeholder when the field is entirely
// empty. It renders through the widget's effective font (Base.Font).
func (t *TagField) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	rowH := t.glyphHeight() + 2*ChipPadY
	rects, x, y := t.layout(r.X, r.Y)
	tags := t.Tags().Get()
	for i, tag := range tags {
		c := &Chip{Text: tag, Closable: true}
		c.Font = t.Font
		c.SetBounds(rects[i])
		c.Draw(p, theme)
	}
	txt := t.Text().Get()
	ty := y + (rowH-t.glyphHeight())/2
	if len(tags) == 0 && txt == "" {
		if t.Placeholder != "" {
			t.drawText(p, x, ty, t.Placeholder, theme.SurfaceAlt)
		}
	} else {
		if txt != "" {
			t.drawText(p, x, ty, txt, theme.OnSurface)
		}
		cx := x + t.textWidth(txt)
		fillRect(p, cx, ty-1, 1, t.glyphHeight()+2, theme.OnSurface)
	}
	t.drawFocusRing(p, theme, r)
}

// commit clears the in-progress Text and, when it held a non-blank value
// not already present, appends it as a new tag by Setting Tags() (which
// notifies subscribers). Blank and duplicate inputs are dropped (Text is
// still cleared) without changing the tag set.
func (t *TagField) commit() {
	v := strings.TrimSpace(t.Text().Get())
	t.Text().Set("")
	if v == "" {
		return
	}
	tags := t.Tags().Get()
	for _, existing := range tags {
		if existing == v {
			return
		}
	}
	t.Tags().Set(append(slices.Clone(tags), v))
}

// OnEvent applies keyboard editing + click-to-remove. Character input
// appends to Text (a bare comma is swallowed because it is the commit
// key); Enter and comma commit the trimmed Text; Backspace on empty Text
// drops the last tag; and a click routes through the token's Chip so its
// "x" close slot removes that tag. Event coordinates are widget-local per
// the toolkit convention, so hit-testing lays the tokens out from (0, 0).
func (t *TagField) OnEvent(ev Event) {
	switch ev.Kind {
	case EventChar:
		if ev.Code == "," {
			return
		}
		t.Text().Set(t.Text().Get() + ev.Code)
	case EventKeyDown:
		switch ev.Code {
		case "Enter", ",":
			t.commit()
		case "Backspace":
			tags := t.Tags().Get()
			if t.Text().Get() == "" && len(tags) > 0 {
				t.Tags().Set(tags[: len(tags)-1 : len(tags)-1])
			}
		}
	case EventClick:
		t.focused = true
		tags := t.Tags().Get()
		rects, _, _ := t.layout(0, 0)
		for i, rc := range rects {
			if !rc.Contains(ev.X, ev.Y) {
				continue
			}
			removed := false
			c := &Chip{Text: tags[i], Closable: true, OnClose: func() { removed = true }}
			c.Font = t.Font
			c.SetBounds(rc)
			c.OnEvent(translateEvent(ev, Rect{}, rc))
			if removed {
				t.Tags().Set(append(tags[:i:i], tags[i+1:]...))
			}
			return
		}
	}
}

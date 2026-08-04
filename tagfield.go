// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

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
// that specific tag. Every change to the tag set fires OnChange with the
// current slice.
//
// Each token is rendered by reusing the Chip widget (Closable: true) so
// the pill body, label + close "x" all match a standalone Chip exactly;
// hit-testing routes the click through the very same Chip.OnEvent against
// a rectangle computed the same way Draw lays it out, so the visible "x"
// and its click target never drift apart.
type TagField struct {
	Base
	// Tags is the committed token set, in insertion order.
	Tags []string
	// Text is the in-progress input shown (with a caret) after the last tag.
	Text string
	// Placeholder is the muted hint drawn when there are no tags and no text.
	Placeholder string
	// OnChange fires whenever the tag set changes (commit / backspace / close).
	OnChange func(tags []string)
	// focusState carries the keyboard focus flag (set true on click) and draws
	// the focus ring; it lets a host / container route subsequent keyboard input
	// to the field.
	focusState
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
	return &TagField{Tags: tags}
}

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
	for _, tag := range t.Tags {
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
	for i, tag := range t.Tags {
		c := &Chip{Text: tag, Closable: true}
		c.Font = t.Font
		c.SetBounds(rects[i])
		c.Draw(p, theme)
	}
	ty := y + (rowH-t.glyphHeight())/2
	if len(t.Tags) == 0 && t.Text == "" {
		if t.Placeholder != "" {
			t.drawText(p, x, ty, t.Placeholder, theme.SurfaceAlt)
		}
	} else {
		if t.Text != "" {
			t.drawText(p, x, ty, t.Text, theme.OnSurface)
		}
		cx := x + t.textWidth(t.Text)
		fillRect(p, cx, ty-1, 1, t.glyphHeight()+2, theme.OnSurface)
	}
	t.drawFocusRing(p, theme, r)
}

// commit clears the in-progress Text and, when it held a non-blank value
// not already present, appends it as a new tag and fires OnChange. Blank
// and duplicate inputs are dropped (Text is still cleared) without
// firing.
func (t *TagField) commit() {
	v := strings.TrimSpace(t.Text)
	t.Text = ""
	if v == "" {
		return
	}
	for _, existing := range t.Tags {
		if existing == v {
			return
		}
	}
	t.Tags = append(t.Tags, v)
	t.fire()
}

// fire invokes OnChange with the current tag set when a callback is wired
// (nil-safe).
func (t *TagField) fire() {
	if t.OnChange != nil {
		t.OnChange(t.Tags)
	}
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
		t.Text += ev.Code
	case EventKeyDown:
		switch ev.Code {
		case "Enter", ",":
			t.commit()
		case "Backspace":
			if t.Text == "" && len(t.Tags) > 0 {
				t.Tags = t.Tags[:len(t.Tags)-1]
				t.fire()
			}
		}
	case EventClick:
		t.focused = true
		rects, _, _ := t.layout(0, 0)
		for i, rc := range rects {
			if !rc.Contains(ev.X, ev.Y) {
				continue
			}
			removed := false
			c := &Chip{Text: t.Tags[i], Closable: true, OnClose: func() { removed = true }}
			c.Font = t.Font
			c.SetBounds(rc)
			c.OnEvent(translateEvent(ev, Rect{}, rc))
			if removed {
				t.Tags = append(t.Tags[:i:i], t.Tags[i+1:]...)
				t.fire()
			}
			return
		}
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// Size-to-content editing (webcanvas#21, step T3).
//
// An editor placed among prose (a notebook cell, a playground page) must not
// scroll inside itself: the host sizes the canvas to the editor's content and the
// page scrolls. [TextView.NoVerticalScroll] turns the vertical scroll off (every
// line is laid out, ScrollLine stays 0, no vertical bar, no minimap) while the
// horizontal bar is kept for long lines, and [TextView.PreferredHeight] publishes
// the height that shows every line.
//
// PreferredHeight is an Observable rather than a getter because the value is
// derived from state the host does not own: the line count (every edit and every
// host Set of Text()), the font metrics (SetFont) and the width (SetBounds: a
// narrower editor may need the horizontal bar, whose track adds to the height).
// The host subscribes once and resizes the canvas from the callback, instead of
// polling after each event; Draw also refreshes it, so a change inherited through
// the global font is published on the next frame. The Observable only fires when
// the value actually changes.

// textBottomPad is the space under the last row, mirroring the 4 px top inset.
const textBottomPad = 4

// PreferredHeight is the widget height, in device pixels, that shows every line
// without a vertical scroll: top inset + lines x line pitch + bottom padding,
// plus the horizontal scrollbar track when the widest line does not fit the
// current width. An empty buffer counts as one line. It follows the model:
// subscribe to it, or Get() it after any change.
func (t *TextView) PreferredHeight() *mvvm.Observable[int] {
	if t.prefH == nil {
		t.prefH = mvvm.NewObservable(t.computePreferredHeight())
	}
	return t.prefH
}

// computePreferredHeight is the pure layout computation behind PreferredHeight.
func (t *TextView) computePreferredHeight() int {
	n := len(t.lines)
	if n < 1 {
		n = 1
	}
	h := 4 + n*(t.glyphHeight()+4) + textBottomPad
	if t.ShowScrollbars && t.Bounds().W > 0 {
		tw := t.Bounds().W - t.textLeftInset() - 1 - t.minimapW()
		if t.contentWidth() > tw {
			h += 1 + scrollbarTrack()
		}
	}
	return h
}

// refreshPreferredHeight republishes PreferredHeight when it changed; a no-op
// until a host asked for the Observable.
func (t *TextView) refreshPreferredHeight() {
	if t.prefH != nil {
		t.prefH.Set(t.computePreferredHeight())
	}
}

// SetBounds places the view and republishes PreferredHeight, since the width
// decides whether the horizontal scrollbar (and its track) is needed.
func (t *TextView) SetBounds(r Rect) {
	t.Base.SetBounds(r)
	t.refreshPreferredHeight()
}

// SetFont sets the per-widget font and republishes PreferredHeight, since the
// line pitch follows the font. Like Base.SetFont it returns the Base for chaining.
func (t *TextView) SetFont(f Font) *Base {
	b := t.Base.SetFont(f)
	t.refreshPreferredHeight()
	return b
}

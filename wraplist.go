// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// WrapList is a read-only, scrollable run of text items that wraps each item to
// the widget's own width. Where a [ListBox] clips a row at its edge, a WrapList
// breaks it into continuation lines, so the host never works out "how many
// characters fit" and tells the data: it hands over the text, and the widget
// breaks it at the width it has, again whenever it is resized.
//
// Every line is drawn at one pitch, taller than the font's glyph box, so lines
// never touch or overlap, whatever the font or the metric scale. No line is wider
// than the widget: a word that does not fit alone on a line is cut where it runs
// out of room. A continuation line is indented by two spaces beyond the item's
// own leading spaces.
//
// It is a [WidthMeasurer]: Measure reports the height the items need at a width,
// which is what a column asks of a child sized with AddNatural. The scroll
// position is in whole lines, and the last line is fully visible at the end of
// the scroll.
type WrapList struct {
	Base
	focusState
	items  []string
	scroll *mvvm.Observable[int]
	sbDrag scrollDrag

	// the lines laid out for the current items and bounds
	lines []string
	valid bool
}

// NewWrapList builds a WrapList of items.
func NewWrapList(items []string) *WrapList {
	return &WrapList{items: items, scroll: mvvm.NewObservable(0)}
}

// SetItems replaces the items. The scroll position is kept, clamped to the new
// extent.
func (l *WrapList) SetItems(items []string) {
	l.items = items
	l.valid = false
	l.ScrollTo(l.ScrollLine().Get())
}

// ScrollLine is the index of the top visible line as a shared [mvvm.Observable].
func (l *WrapList) ScrollLine() *mvvm.Observable[int] {
	if l.scroll == nil {
		l.scroll = mvvm.NewObservable(0)
	}
	return l.scroll
}

// SetBounds lays the widget out and keeps the scroll position inside the new
// extent: a resize re-wraps the lines.
func (l *WrapList) SetBounds(r Rect) {
	l.Base.SetBounds(r)
	l.valid = false
	l.ScrollTo(l.ScrollLine().Get())
}

// LineHeight is the pitch between lines: the font's glyph box plus a gap, so
// consecutive lines never overlap.
func (l *WrapList) LineHeight() int {
	h := l.glyphHeight()
	return h + max(h/4, scaled(2))
}

// textPad is the margin at the left of a line, and of its right edge.
func (l *WrapList) textPad() int { return scaled(4) }

// Lines is the items as they are laid out at the widget's current width, one
// string per drawn line.
func (l *WrapList) Lines() []string {
	l.layout()
	return l.lines
}

func (l *WrapList) layout() {
	if l.valid { // SetBounds and SetItems are the only things that change the answer
		return
	}
	l.valid = true
	w := l.Bounds().W
	l.lines = l.wrapAt(w)
	if len(l.lines)*l.LineHeight() > l.Bounds().H { // a scrollbar takes its gutter: wrap again narrower
		l.lines = l.wrapAt(w - scrollGutter())
	}
}

// wrapAt breaks every item to fit a widget w pixels wide, margins out.
func (l *WrapList) wrapAt(w int) []string {
	room := w - 2*l.textPad()
	var out []string
	for _, it := range l.items {
		out = append(out, wrapItem(l.EffectiveFont(), it, room)...)
	}
	return out
}

// Measure is the height the items need at the given width: every wrapped line at
// the line pitch. It never reports a scrollbar's gutter, since a widget given the
// height it asks for has nothing to scroll.
func (l *WrapList) Measure(width int) int {
	return len(l.wrapAt(width)) * l.LineHeight()
}

// fullLines is how many lines fit entirely in the widget's height.
func (l *WrapList) fullLines() int {
	return l.Bounds().H / max(l.LineHeight(), 1)
}

func (l *WrapList) maxScroll() int {
	l.layout()
	return max(len(l.lines)-l.fullLines(), 0)
}

// ScrollTo moves the top line to line, clamped so the last line ends flush with
// the bottom edge.
func (l *WrapList) ScrollTo(line int) {
	l.ScrollLine().Set(min(max(line, 0), l.maxScroll()))
}

// ScrollBy moves the top line by delta lines.
func (l *WrapList) ScrollBy(delta int) { l.ScrollTo(l.ScrollLine().Get() + delta) }

// Draw paints the lines from the top one down, clipped to the widget.
func (l *WrapList) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	l.layout()
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(r)
	}
	lh := l.LineHeight()
	top := min(l.ScrollLine().Get(), l.maxScroll())
	for i := top; i < len(l.lines) && (i-top)*lh < r.H; i++ {
		l.drawText(p, r.X+l.textPad(), r.Y+(i-top)*lh+(lh-l.glyphHeight())/2, l.lines[i], theme.OnSurface)
	}
	if canClip {
		clr.PopClip()
	}
	if g, ok := l.scrollbarGeom(); ok {
		track := scrollbarTrack()
		paintScrollTrack(p, theme, r.X+r.W-track, r.Y, track, r.H)
		paintScrollThumb(p, theme, r.X+g.cross0, r.Y+g.thumbStart, track, g.thumbLen)
	}
	l.drawFocusRing(p, theme, r)
}

func (l *WrapList) scrollbarGeom() (sbGeom, bool) {
	r := l.Bounds()
	l.layout()
	contentH := len(l.lines) * l.LineHeight()
	if r.H <= 0 || contentH <= r.H {
		return sbGeom{}, false
	}
	thumbH := max(r.H*r.H/contentH, scaled(8))
	ms := l.maxScroll()
	thumbY := 0
	if ms > 0 {
		thumbY = min(l.ScrollLine().Get(), ms) * (r.H - thumbH) / ms
	}
	return sbGeom{
		cross0:     r.W - scrollbarTrack(),
		crossW:     scrollbarTrack(),
		trackStart: 0,
		trackLen:   r.H,
		thumbStart: thumbY,
		thumbLen:   thumbH,
		travelNum:  r.H - thumbH,
		travelDen:  ms,
		maxScroll:  ms,
	}, true
}

// OnEvent scrolls: the wheel, the arrow, page, Home and End keys, and the
// scrollbar's thumb and track.
func (l *WrapList) OnEvent(ev Event) {
	if l.Disabled().Get() {
		l.sbDrag.release()
		return
	}
	switch ev.Kind {
	case EventScroll:
		l.ScrollBy(ev.Delta)
	case EventKeyDown:
		if i, ok := rovingIndex(l.ScrollLine().Get(), len(l.Lines()), max(l.fullLines(), 1), ev.Code); ok {
			l.ScrollTo(i)
		}
	case EventClick:
		g, ok := l.scrollbarGeom()
		l.sbDrag.press(g, ok, ev, max(l.fullLines(), 1), l.ScrollBy)
	case EventMouseDrag:
		g, ok := l.scrollbarGeom()
		l.sbDrag.drag(g, ok, ev, l.ScrollTo)
	case EventMouseUp:
		l.sbDrag.release()
	}
}

// A11y reports the list as a document.
func (l *WrapList) A11y() A11yInfo {
	return A11yInfo{Role: RoleDocument, Value: strings.Join(l.items, "\n"), Disabled: l.Disabled().Get()}
}

var (
	_ WidthMeasurer = (*WrapList)(nil)
	_ Accessible    = (*WrapList)(nil)
)

// wrapItem breaks one item into lines no wider than room pixels in font f. It
// breaks at spaces; a word wider than a line is cut where the line runs out. The
// item's leading spaces start every line and a continuation line adds two more.
// An empty item is one empty line, so blank lines in the text stay blank.
func wrapItem(f Font, item string, room int) []string {
	words := strings.Fields(item)
	if len(words) == 0 {
		return []string{""}
	}
	lead := item[:len(item)-len(strings.TrimLeft(item, " "))]
	cont := lead + "  "
	var out []string
	cur, prefix := lead, lead // the line being built, and what it starts with
	empty := true             // cur holds no word yet
	for _, w := range words {
		for {
			try := cur
			if !empty {
				try += " "
			}
			try += w
			if f.Measure(try) <= room {
				cur, empty = try, false
				break
			}
			if !empty { // does not fit after what is there: a new line
				out = append(out, cur)
				cur, prefix, empty = cont, cont, true
				continue
			}
			// alone on its line and still too wide: cut it where the room ends
			rs := []rune(w)
			n := 0
			for n < len(rs) && f.Measure(prefix+string(rs[:n+1])) <= room {
				n++
			}
			n = max(n, 1) // a line takes at least one rune, so the loop ends
			out = append(out, prefix+string(rs[:n]))
			cur, prefix, empty = cont, cont, true
			w = string(rs[n:])
			if w == "" {
				break
			}
		}
	}
	if !empty || len(out) == 0 {
		out = append(out, cur)
	}
	return out
}

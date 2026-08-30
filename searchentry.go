// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// SearchEntry is a single-line text input decorated with a leading
// search-prefix glyph and, when the text is non-empty, a trailing "clear"
// affordance on the right. Think GTK's SearchEntry: an Entry whose
// visual chrome hints at its role and offers a one-click reset. The
// widget inserts printable characters at the caret, deletes before it on
// Backspace, and clears on a click in the right-side X slot. Like Entry it has a
// movable caret: ArrowLeft/Right and Home/End move it, a click in the text places
// it, and edits happen at the caret — so a value can be corrected mid-text, not
// only at the end. The text scrolls horizontally to keep the caret in view. The
// caret renders only when Focused (set by the host). It has no IME — callers
// needing composed input should reach for Entry / TextView instead.
//
// The reactive text is MVVM-only: the current value lives in an unexported
// Observable exposed via [SearchEntry.Text]. A host binds it (Set / Subscribe
// / two-way); typing, Backspace, and the clear affordance Set it — there is no
// settable Text field and no OnChange callback.
//
// An optional leading Icon lets the host paint a real magnifier (or any
// glyph) in the left prefix slot instead of the "?" text stand-in. When
// set, Draw invokes Icon with the prefix slot's rect + the OnSurface ink
// and skips the "?" text; when nil, the classic "?" stand-in is drawn, so
// existing callers are unaffected. This mirrors Banner.Icon.
type SearchEntry struct {
	Base
	focusState // when focused, a text caret is drawn at the cursor
	Icon       func(p painter.Painter, r Rect, ink RGBA)

	text *mvvm.Observable[string]
	// cursor is the caret rune index in [0, len(runes)]; edits and the drawn caret
	// track it. scrollX is the horizontal scroll offset (device px) that keeps the
	// caret in view when the text is wider than the field. Both are clamped to the
	// current text on every event and Draw, so an external Text().Set stays safe.
	cursor  int
	scrollX int
	// editing is true only while OnEvent applies the widget's own edit, so the Text
	// subscription (see watchText) can tell a host / two-way-binding Set apart from
	// the widget's own and leave the caret the widget just placed alone. watched
	// guards the one-time subscription.
	editing bool
	watched bool
}

// Text is the current field value as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Text field. A
// keystroke edit, a Backspace, or a click on the clear affordance Sets it;
// subscribers are notified on change.
func (s *SearchEntry) Text() *mvvm.Observable[string] {
	if s.text == nil {
		s.text = mvvm.NewObservable("")
	}
	s.watchText()
	return s.text
}

// watchText subscribes (once) to the value so a Set from OUTSIDE the widget — a
// host or a two-way binding — parks the caret at the end, letting the next
// keystroke append rather than insert at a stale index. The widget's own edits
// set s.editing, which suppresses the reset so a mid-text insert keeps its caret.
func (s *SearchEntry) watchText() {
	if s.watched || s.text == nil {
		return
	}
	s.watched = true
	s.text.Subscribe(func(v string) {
		if !s.editing {
			s.cursor = len([]rune(v))
		}
	})
}

// SearchEntryPadX is the horizontal padding between the widget's outer
// border and the inner content (the search prefix, the text field, the
// clear affordance).
const SearchEntryPadX = 4

// SearchEntryIconW is the pixel width reserved for the leading prefix
// glyph and the trailing clear affordance. Both slots share the same
// width so hit-testing stays symmetric.
const SearchEntryIconW = 16

// searchEntryPrefix is the glyph rendered in the left icon slot. We
// pick "?" from the 5x7 bitmap font (font.go) as a stand-in for a
// magnifier — the toolkit's bitmap font does not carry a magnifier
// glyph and adding one just for this widget would be out of scale.
const searchEntryPrefix = "?"

// searchEntryClear is the glyph rendered in the right icon slot when
// the text is non-empty. Lower-case "x" reads as a subtle close/reset
// affordance next to the ink text.
const searchEntryClear = "x"

// NewSearchEntry builds a SearchEntry pre-loaded with initial text.
// The constructor does not notify subscribers for the initial value so
// callers can wire a subscription after construction without a spurious
// notification.
func NewSearchEntry(text string) *SearchEntry {
	s := &SearchEntry{}
	s.text = mvvm.NewObservable(text)
	s.cursor = len([]rune(text)) // caret parks at the end, like Entry
	s.watchText()
	return s
}

// Draw paints the entry body, the leading prefix glyph, the current
// text, and (when the text is non-empty) the trailing clear affordance.
func (s *SearchEntry) Draw(p painter.Painter, theme *Theme) {
	r := s.Bounds()
	text := s.Text().Get()
	// SearchEntryPadX / SearchEntryIconW are LOGICAL bases; routed through scaled
	// so the prefix/clear slots grow with HiDPI and touch Density. At compact/1x
	// scaled is identity, so the layout is byte-identical to before.
	padX, iconW := scaled(SearchEntryPadX), scaled(SearchEntryIconW)
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	textY := r.Y + (r.H-s.glyphHeight())/2
	// Left prefix slot: a host-supplied Icon (real magnifier) when set,
	// otherwise the "?" bitmap-font stand-in.
	if s.Icon != nil {
		iconR := Rect{X: r.X + padX, Y: r.Y, W: iconW, H: r.H}
		s.Icon(p, iconR, theme.OnSurface)
	} else {
		prefixX := r.X + padX + (iconW-s.glyphAdvance())/2
		s.drawText(p, prefixX, textY, searchEntryPrefix, theme.OnSurface)
	}
	// Middle text region, between the prefix slot and the (always-reserved) clear
	// slot. The text scrolls horizontally and clips to this region so the caret
	// stays in view on a value wider than the field.
	textX := r.X + padX + iconW
	innerW := r.W - 2*padX - 2*iconW
	if innerW < 0 {
		innerW = 0
	}
	runes := []rune(text)
	if s.cursor > len(runes) {
		s.cursor = len(runes)
	}
	caretW := s.textWidth(string(runes[:s.cursor]))
	s.scrollX = clampEntryScroll(s.scrollX, caretW, s.textWidth(text), innerW, s.focused)
	tx := textX - s.scrollX
	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(Rect{X: textX, Y: r.Y, W: innerW, H: r.H})
	}
	s.drawText(p, tx, textY, text, theme.OnSurface)
	// Caret at the cursor when focused, measured with the widget's own font so it
	// aligns with the text the widget just drew and tracks the same scroll offset.
	if s.focused {
		cw := s.glyphHeight() / 12
		if cw < 1 {
			cw = 1
		}
		fillRect(p, tx+caretW, textY, cw, s.glyphHeight(), theme.OnSurface)
	}
	if canClip {
		clr.PopClip()
	}
	// Right clear slot only when there is text to clear.
	if text != "" {
		clearX := r.X + r.W - padX - iconW + (iconW-s.glyphAdvance())/2
		s.drawText(p, clearX, textY, searchEntryClear, theme.Border)
	}
	s.drawFocusRing(p, theme, r)
}

// clearSlot is the drawn rectangle of the trailing "clear" affordance in
// absolute coordinates — the right-edge icon slot, iconW wide and the full field
// height. Shared by ClearHitRect so the touch grab is derived from exactly the
// slot Draw paints.
func (s *SearchEntry) clearSlot() Rect {
	r := s.Bounds()
	padX, iconW := scaled(SearchEntryPadX), scaled(SearchEntryIconW)
	return Rect{X: r.X + r.W - padX - iconW, Y: r.Y, W: iconW, H: r.H}
}

// HitRect is the SearchEntry's field-level tap target: Bounds clamped up to the
// touch minimum on each axis and centred. Byte-identical to Bounds at
// [DensityCompact].
func (s *SearchEntry) HitRect() Rect { return touchHitRect(s.Bounds()) }

// ClearHitRect is the finger target for the trailing "clear" affordance: the
// drawn clear slot clamped up to the touch minimum on each axis and centred over
// it, so the narrow 16-logical-pixel glyph still exposes a 44px grab under
// [DensityTouch]. At [DensityCompact] it equals the drawn slot byte-for-byte.
func (s *SearchEntry) ClearHitRect() Rect { return touchHitRect(s.clearSlot()) }

// OnEvent handles character insertion at the caret (EventChar), Backspace
// deletion before the caret and caret movement (EventKeyDown: Backspace,
// ArrowLeft/Right, Home, End), and clicks (EventClick): a click in the right icon
// slot clears the text, one in the text region places the caret. Other events are
// ignored. Every text mutation routes through the Text Observable's Set, notifying
// subscribers. Event X is field-local (0 at the widget's left edge), matching the
// clear-slot hit test.
func (s *SearchEntry) OnEvent(ev Event) {
	s.editing = true // suppress the caret-parking subscription for our own edits
	defer func() { s.editing = false }()
	runes := []rune(s.Text().Get())
	if s.cursor > len(runes) {
		s.cursor = len(runes)
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	switch ev.Kind {
	case EventChar:
		if ev.Code == "" {
			return
		}
		ch := []rune(ev.Code)
		runes = append(runes[:s.cursor], append(ch, runes[s.cursor:]...)...)
		s.cursor += len(ch)
		s.Text().Set(string(runes))
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			if s.cursor > 0 {
				runes = append(runes[:s.cursor-1], runes[s.cursor:]...)
				s.cursor--
				s.Text().Set(string(runes))
			}
		case "ArrowLeft":
			if s.cursor > 0 {
				s.cursor--
			}
		case "ArrowRight":
			if s.cursor < len(runes) {
				s.cursor++
			}
		case "Home":
			s.cursor = 0
		case "End":
			s.cursor = len(runes)
		}
	case EventClick:
		r := s.Bounds()
		padX, iconW := scaled(SearchEntryPadX), scaled(SearchEntryIconW)
		if len(runes) > 0 && ev.X >= r.W-padX-iconW && ev.X < r.W-padX {
			s.cursor = 0
			s.Text().Set("")
			return
		}
		// A click in the text region places the caret; map the field-local x back
		// through the prefix slot and the current scroll into the text's own space.
		s.cursor = s.caretIndexAt(string(runes), ev.X-(padX+iconW)+s.scrollX)
	}
}

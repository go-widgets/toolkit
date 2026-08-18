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
// widget appends printable characters, deletes on Backspace, and clears
// on a click in the right-side X slot. It draws a simple end-of-text
// caret when Focused (set by the host), measured with its own font so it
// always aligns; it has no cursor navigation or IME — callers needing
// those should reach for Entry / TextView instead.
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
	focusState // when focused, a text caret is drawn at the end of the text
	Icon       func(p painter.Painter, r Rect, ink RGBA)

	text *mvvm.Observable[string]
}

// Text is the current field value as a shared [mvvm.Observable]: a host binds
// it (Set / Subscribe / two-way) — there is no settable Text field. A
// keystroke edit, a Backspace, or a click on the clear affordance Sets it;
// subscribers are notified on change.
func (s *SearchEntry) Text() *mvvm.Observable[string] {
	if s.text == nil {
		s.text = mvvm.NewObservable("")
	}
	return s.text
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
	// Middle text.
	textX := r.X + padX + iconW
	s.drawText(p, textX, textY, text, theme.OnSurface)
	// Caret at the end of the text when focused. Measured with the widget's own
	// font so it always aligns with the text the widget just drew — a host must not
	// overlay its own caret with a different font engine.
	if s.focused {
		caretW := s.glyphHeight() / 12
		if caretW < 1 {
			caretW = 1
		}
		fillRect(p, textX+s.textWidth(text), textY, caretW, s.glyphHeight(), theme.OnSurface)
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

// OnEvent handles character insertion (EventChar), Backspace deletion
// (EventKeyDown / "Backspace"), and click-to-clear in the right icon
// slot (EventClick, when the text is non-empty). Other events are ignored.
// Every mutation routes through the Text Observable's Set, notifying
// subscribers.
func (s *SearchEntry) OnEvent(ev Event) {
	switch ev.Kind {
	case EventChar:
		if ev.Code == "" {
			return
		}
		s.Text().Set(s.Text().Get() + ev.Code)
	case EventKeyDown:
		if ev.Code != "Backspace" {
			return
		}
		runes := []rune(s.Text().Get())
		if len(runes) == 0 {
			return
		}
		s.Text().Set(string(runes[:len(runes)-1]))
	case EventClick:
		if s.Text().Get() == "" {
			return
		}
		r := s.Bounds()
		padX, iconW := scaled(SearchEntryPadX), scaled(SearchEntryIconW)
		clearLeft := r.W - padX - iconW
		clearRight := r.W - padX
		if ev.X >= clearLeft && ev.X < clearRight {
			s.Text().Set("")
		}
	}
}

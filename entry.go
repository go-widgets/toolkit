// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Entry is a single-line text input. Receives focus on click, edits its
// text via EventKeyDown (Backspace, ArrowLeft/Right, Home, End, Enter) +
// EventChar (printable runes). A 1-pixel vertical cursor renders at the
// cursor offset when Focused.
//
// The reactive contents live on the [Entry.Text] Observable: a host binds
// it (or subscribes) instead of reading a plain field, and every edit
// publishes through it — so there is no OnChange callback, a Set is the
// only way in and it is MVVM by construction. The cursor rune index and the
// in-flight IME preview are internal editing state.
//
// The widget treats the text as a rune index space so multi-byte UTF-8
// characters move the cursor by one position even when they take several
// bytes on the wire.
type Entry struct {
	Base
	focusState
	OnSubmit func(text string)

	// Placeholder is shown in the muted tone when the text is empty and no IME
	// composition is in flight (a hint like "search…" or "client id").
	Placeholder string

	// Mask, when non-zero, is the rune each character is displayed as (e.g. '•')
	// instead of the real text — for secrets/passwords. Text/Value keep the real
	// contents; only the display is masked.
	Mask rune

	// text is the committed contents, reactive via the Text() accessor.
	text *mvvm.Observable[string]
	// cursor is the caret rune index in [0, len(runes)].
	cursor int
	// composition holds the in-progress IME preview string (dead-key output, CJK
	// candidate, …). Non-empty while an IME composition is active; cleared on
	// EventCompositionEnd. The preview is NOT part of the text until the host
	// commits it via EventChar, so the text always reflects only committed input.
	composition string
	// scrollX is the horizontal scroll offset in device pixels: how far the text
	// is shifted left under the field so a caret past the right edge stays in view.
	// Recomputed each Draw to follow the caret and clamped so the text never
	// scrolls past either end; an unfocused field resets to 0 (reads from the
	// start). Text is clipped to the field, so a value wider than the field no
	// longer spills past its border.
	scrollX int
}

// entryPadX is the left inset in LOGICAL pixels between the field border and the
// text baseline column. Routed through [scaled] at every use so the gap grows
// with HiDPI and touch [Density] instead of staying a fixed 4 device pixels
// while the glyphs around it double; at [DensityCompact] and MetricScale 1 it is
// its literal 4, byte-identical to the pre-density field.
const entryPadX = 4

// HitRect is the Entry's interactive tap target: its drawn [Widget.Bounds] with
// each axis clamped up to the touch minimum and centred, so a single-line field
// only a glyph-row tall still offers a finger the platform's 44-logical-pixel
// reach under [DensityTouch]. At [DensityCompact] it equals Bounds byte-for-byte
// (the clamp is a pass-through), leaving desktop hit-testing unchanged.
func (e *Entry) HitRect() Rect { return touchHitRect(e.Bounds()) }

// NewEntry builds an Entry with initial text + cursor parked at end.
func NewEntry(initial string) *Entry {
	r := []rune(initial)
	e := &Entry{cursor: len(r)}
	e.text = mvvm.NewObservable(initial)
	return e
}

// Text is the entry's committed contents as a shared [mvvm.Observable]: a host
// binds it two-way (or subscribes) instead of touching a field, and every edit
// Sets it — so a Set is the only way to change the text and there is no separate
// change callback. Lazily created so a bare &Entry{} works.
func (e *Entry) Text() *mvvm.Observable[string] {
	if e.text == nil {
		e.text = mvvm.NewObservable("")
	}
	return e.text
}

// SetText replaces the whole contents programmatically and parks the caret at
// the end (the same place [NewEntry] leaves it), mirroring how a host that
// drives the value itself expects the field to read next. It goes through the
// Text() Observable — so subscribers/bindings fire — rather than touching a
// field, keeping the widget MVVM-only. For interactive editing the host feeds
// events to OnEvent instead; this is the programmatic-set seam.
func (e *Entry) SetText(s string) {
	e.cursor = len([]rune(s))
	e.Text().Set(s)
}

// Value returns the entry's current text. It is the accessor
// FormField.Value uses (via the unexported valueGetter interface) to
// pull an Entry's contents without depending on the Text observable
// directly, so a FormField wrapping an Entry can be validated.
func (e *Entry) Value() string { return e.Text().Get() }

// display is the text as rendered: each rune replaced by Mask when Mask is set
// (secret fields), else the text verbatim. Value/Text keep the real contents.
func (e *Entry) display() string {
	t := e.Text().Get()
	if e.Mask == 0 {
		return t
	}
	runes := []rune(t)
	for i := range runes {
		runes[i] = e.Mask
	}
	return string(runes)
}

// Draw paints the border, fill, text + (when Focused) a 1-px cursor
// stroke at the cursor's pixel position. The text is horizontally scrolled to
// keep the caret in view and clipped to the field, so a value wider than the
// field scrolls under it instead of spilling past the border.
func (e *Entry) Draw(p painter.Painter, theme *Theme) {
	r := e.Bounds()
	border := theme.Border
	if e.focused {
		border = theme.Accent
	}
	fillRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, theme.Surface)
	strokeRoundRect(p, r.X, r.Y, r.W, r.H, buttonRadius, border)
	textY := r.Y + (r.H-e.glyphHeight())/2
	shown := e.display()
	pad := scaled(entryPadX)

	// Fit the caret in the inner (padded) width, scrolling the text left when it
	// runs past the right edge. The composition preview sits at the caret, so it
	// counts toward the caret's effective x for the fit.
	runes := []rune(shown)
	if e.cursor > len(runes) {
		e.cursor = len(runes)
	}
	innerW := r.W - 2*pad
	caretW := e.textWidth(string(runes[:e.cursor]))
	compW := 0
	if e.composition != "" {
		compW = e.textWidth(e.composition)
	}
	totalW := e.textWidth(shown)
	if caretW+compW > totalW {
		totalW = caretW + compW
	}
	e.scrollX = clampEntryScroll(e.scrollX, caretW+compW, totalW, innerW, e.focused)

	// Clip the text to the inner content rect so the overflow scrolls under the
	// border rather than over it. Backends that cannot clip (a cell grid never
	// needs to here) degrade to the old unclipped draw, as List/Carousel do.
	tx := r.X + pad - e.scrollX
	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(Rect{X: r.X + pad, Y: r.Y, W: innerW, H: r.H})
	}
	if shown == "" && e.composition == "" && e.Placeholder != "" {
		e.drawText(p, r.X+pad, textY, e.Placeholder, theme.SurfaceAlt)
	} else {
		e.drawText(p, tx, textY, shown, theme.OnSurface)
	}
	if e.focused {
		// Caret x measured from the shown text up to the cursor, so it lands
		// correctly under a proportional / CJK font (not a fixed advance),
		// then shifted by the same scroll offset as the text.
		cx := tx + caretW
		if e.composition != "" {
			// IME composition preview: render the pending string in
			// the muted SurfaceAlt tone right at the cursor, ghosted +
			// underlined so the user sees dead-key / CJK candidates
			// without them entering the text. Mirrors TextView's
			// treatment. Unlike TextView (multi-line, cursor pinned to
			// CursorCol), Entry pushes its single caret past the
			// preview's pixel width so it visually tracks where the
			// next committed rune will land.
			e.drawText(p, cx, textY, e.composition, theme.SurfaceAlt)
			fillRect(p, cx, textY+e.glyphHeight(), compW, 1, theme.SurfaceAlt)
			cx += compW
		}
		fillRect(p, cx, textY-1, 1, e.glyphHeight()+2, theme.OnSurface)
	}
	if canClip {
		clr.PopClip()
	}
	e.drawFocusRing(p, theme, r)
}

// clampEntryScroll returns a horizontal scroll offset (device pixels, ≥0) that
// keeps the caret at caretW visible inside an innerW-wide viewport over text
// totalW wide, never scrolling past either end. A field that is not focused, or
// whose text fits, reads from the start (offset 0); a focused field follows the
// caret, revealing the tail when it runs off the right and the head when it comes
// back to the left.
func clampEntryScroll(cur, caretW, totalW, innerW int, focused bool) int {
	if innerW <= 0 || totalW <= innerW || !focused {
		return 0
	}
	s := cur
	if caretW-s < 0 { // caret fell off the left: pin it to the left edge
		s = caretW
	}
	if caretW-s > innerW { // caret ran off the right: pin it to the right edge
		s = caretW - innerW
	}
	if m := totalW - innerW; s > m { // never scroll past the text's end
		s = m
	}
	if s < 0 {
		s = 0
	}
	return s
}

// caretIndexAt returns the rune index in shown nearest the pixel position localX
// (measured from the text's own left, i.e. after the pad and scroll offset are
// removed). A click left of the first glyph parks at 0; one past the last glyph
// parks at the end; in between it snaps to whichever rune boundary the click is
// closer to, so a click lands where the eye expects. It lives on Base so every
// single-line text widget (Entry, SearchEntry) maps a click to a caret the same
// way, measured with that widget's own font.
func (b *Base) caretIndexAt(shown string, localX int) int {
	runes := []rune(shown)
	if localX <= 0 {
		return 0
	}
	for i := 0; i < len(runes); i++ {
		left := b.textWidth(string(runes[:i]))
		right := b.textWidth(string(runes[:i+1]))
		if localX < (left+right)/2 {
			return i
		}
	}
	return len(runes)
}

// OnEvent handles focus, keyboard navigation, character insertion +
// delete.
func (e *Entry) OnEvent(ev Event) {
	runes := []rune(e.Text().Get())
	switch ev.Kind {
	case EventClick:
		e.focused = true
		// Place the caret under the click, mapping the click x back through the
		// left pad and the current scroll offset into the text's own pixel space.
		pad := scaled(entryPadX)
		e.cursor = e.caretIndexAt(e.display(), ev.X-(e.Bounds().X+pad)+e.scrollX)
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			if e.cursor > 0 {
				runes = append(runes[:e.cursor-1], runes[e.cursor:]...)
				e.cursor--
				e.Text().Set(string(runes))
			}
		case "ArrowLeft":
			if e.cursor > 0 {
				e.cursor--
			}
		case "ArrowRight":
			if e.cursor < len(runes) {
				e.cursor++
			}
		case "Home":
			e.cursor = 0
		case "End":
			e.cursor = len(runes)
		case "Enter":
			if e.OnSubmit != nil {
				e.OnSubmit(e.Text().Get())
			}
		case "Ctrl+C":
			// Entry has no selection concept: Ctrl+C copies the whole
			// value, mirroring "select all + copy".
			if t := e.Text().Get(); t != "" {
				SetClipboardText(t)
			}
		case "Ctrl+X":
			if e.Text().Get() != "" {
				SetClipboardText(e.Text().Get())
				e.cursor = 0
				e.Text().Set("")
			}
		case "Ctrl+V":
			paste := []rune(ClipboardText())
			if len(paste) > 0 {
				runes = append(runes[:e.cursor], append(paste, runes[e.cursor:]...)...)
				e.cursor += len(paste)
				e.Text().Set(string(runes))
			}
		}
	case EventChar:
		// If an IME composition was in flight, the incoming char is
		// the commit result — clear the preview BEFORE inserting so
		// the buffer + display stay consistent.
		e.composition = ""
		ch := []rune(ev.Code)
		if len(ch) == 0 {
			return
		}
		runes = append(runes[:e.cursor], append(ch, runes[e.cursor:]...)...)
		e.cursor += len(ch)
		e.Text().Set(string(runes))
	case EventCompositionStart, EventCompositionUpdate:
		// Preview only — do NOT touch the text. Repaint responsibility
		// lies with the host, who typically calls Draw after each
		// composition event.
		e.composition = ev.Code
	case EventCompositionEnd:
		// Cancel / commit-without-follow-up: drop the preview. When
		// the host follows up with EventChar (commit path), the
		// EventChar arm above will re-clear + insert.
		e.composition = ""
	}
}

// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Entry is a single-line text input. Receives focus on click, edits
// Text via EventKeyDown (Backspace, ArrowLeft/Right, Home, End,
// Enter) + EventChar (printable runes). A 1-pixel vertical cursor
// renders at the cursor offset when Focused.
//
// The widget treats Text as a rune index space so multi-byte UTF-8
// characters move the cursor by one position even when they take
// several bytes on the wire.
type Entry struct {
	Base
	focusState
	Text     string
	Cursor   int // rune index in [0, len(runes)]
	OnChange func(text string)
	OnSubmit func(text string)

	// Placeholder is shown in the muted tone when Text is empty and no IME
	// composition is in flight (a hint like "search…" or "client id").
	Placeholder string

	// Mask, when non-zero, is the rune each character is displayed as (e.g. '•')
	// instead of the real text — for secrets/passwords. Text/Value keep the real
	// contents; only the display is masked.
	Mask rune

	// Composition holds the in-progress IME preview string (dead-key
	// output, CJK candidate, …). Non-empty while an IME composition is
	// active; cleared on EventCompositionEnd. Mirrors TextView's field
	// of the same name: the preview is NOT part of Text until the host
	// commits it via EventChar, so Text always reflects only committed
	// input.
	Composition string
}

// entryPadX is the left inset in LOGICAL pixels between the field border and the
// text baseline column. Routed through [scaled] at every use so the gap grows
// with HiDPI and touch [Density] instead of staying a fixed 4 device pixels
// while the glyphs around it double; at [DensityCompact] and MetricScale 1 it is
// its literal 4, byte-identical to the pre-density field.
const entryPadX = 4

// hitRectFor clamps a widget's drawn rectangle up to the current density's
// minimum hit dimension on each axis (via [TouchTarget]) and re-centres the
// enlarged rect over the original, so the interactive target meets the touch
// floor without moving or resizing what Draw paints. Under [DensityCompact]
// TouchTarget is a pass-through, so the returned rect equals r byte-for-byte —
// the shared engine behind every INPUTS/PICKERS HitRect, mirroring the worked
// [Switch.HitRect] example.
func hitRectFor(r Rect) Rect {
	w, h := TouchTarget(r.W), TouchTarget(r.H)
	return Rect{X: r.X - (w-r.W)/2, Y: r.Y - (h-r.H)/2, W: w, H: h}
}

// HitRect is the Entry's interactive tap target: its drawn [Widget.Bounds] with
// each axis clamped up to the touch minimum and centred, so a single-line field
// only a glyph-row tall still offers a finger the platform's 44-logical-pixel
// reach under [DensityTouch]. At [DensityCompact] it equals Bounds byte-for-byte
// (the clamp is a pass-through), leaving desktop hit-testing unchanged.
func (e *Entry) HitRect() Rect { return hitRectFor(e.Bounds()) }

// NewEntry builds an Entry with initial text + cursor parked at end.
func NewEntry(initial string) *Entry {
	r := []rune(initial)
	return &Entry{Text: initial, Cursor: len(r)}
}

// Value returns the entry's current text. It is the accessor
// FormField.Value uses (via the unexported valueGetter interface) to
// pull an Entry's contents without depending on the Text field name
// directly, so a FormField wrapping an Entry can be validated.
func (e *Entry) Value() string { return e.Text }

// display is the text as rendered: each rune replaced by Mask when Mask is set
// (secret fields), else Text verbatim. Value/Text keep the real contents.
func (e *Entry) display() string {
	if e.Mask == 0 {
		return e.Text
	}
	runes := []rune(e.Text)
	for i := range runes {
		runes[i] = e.Mask
	}
	return string(runes)
}

// Draw paints the border, fill, text + (when Focused) a 1-px cursor
// stroke at the cursor's pixel position.
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
	if shown == "" && e.Composition == "" && e.Placeholder != "" {
		e.drawText(p, r.X+pad, textY, e.Placeholder, theme.SurfaceAlt)
	} else {
		e.drawText(p, r.X+pad, textY, shown, theme.OnSurface)
	}
	if e.focused {
		// Caret x measured from the shown text up to the cursor, so it lands
		// correctly under a proportional / CJK font (not a fixed advance).
		runes := []rune(shown)
		if e.Cursor > len(runes) {
			e.Cursor = len(runes)
		}
		cx := r.X + pad + e.textWidth(string(runes[:e.Cursor]))
		if e.Composition != "" {
			// IME composition preview: render the pending string in
			// the muted SurfaceAlt tone right at the cursor, ghosted +
			// underlined so the user sees dead-key / CJK candidates
			// without them entering Text. Mirrors TextView's
			// treatment. Unlike TextView (multi-line, cursor pinned to
			// CursorCol), Entry pushes its single caret past the
			// preview's pixel width so it visually tracks where the
			// next committed rune will land.
			cw := e.textWidth(e.Composition)
			e.drawText(p, cx, textY, e.Composition, theme.SurfaceAlt)
			fillRect(p, cx, textY+e.glyphHeight(), cw, 1, theme.SurfaceAlt)
			cx += cw
		}
		fillRect(p, cx, textY-1, 1, e.glyphHeight()+2, theme.OnSurface)
	}
	e.drawFocusRing(p, theme, r)
}

// OnEvent handles focus, keyboard navigation, character insertion +
// delete.
func (e *Entry) OnEvent(ev Event) {
	runes := []rune(e.Text)
	switch ev.Kind {
	case EventClick:
		e.focused = true
	case EventKeyDown:
		switch ev.Code {
		case "Backspace":
			if e.Cursor > 0 {
				runes = append(runes[:e.Cursor-1], runes[e.Cursor:]...)
				e.Cursor--
				e.Text = string(runes)
				if e.OnChange != nil {
					e.OnChange(e.Text)
				}
			}
		case "ArrowLeft":
			if e.Cursor > 0 {
				e.Cursor--
			}
		case "ArrowRight":
			if e.Cursor < len(runes) {
				e.Cursor++
			}
		case "Home":
			e.Cursor = 0
		case "End":
			e.Cursor = len(runes)
		case "Enter":
			if e.OnSubmit != nil {
				e.OnSubmit(e.Text)
			}
		case "Ctrl+C":
			// Entry has no selection concept: Ctrl+C copies the whole
			// value, mirroring "select all + copy".
			if e.Text != "" {
				SetClipboardText(e.Text)
			}
		case "Ctrl+X":
			if e.Text != "" {
				SetClipboardText(e.Text)
				e.Text = ""
				e.Cursor = 0
				if e.OnChange != nil {
					e.OnChange(e.Text)
				}
			}
		case "Ctrl+V":
			paste := []rune(ClipboardText())
			if len(paste) > 0 {
				runes = append(runes[:e.Cursor], append(paste, runes[e.Cursor:]...)...)
				e.Cursor += len(paste)
				e.Text = string(runes)
				if e.OnChange != nil {
					e.OnChange(e.Text)
				}
			}
		}
	case EventChar:
		// If an IME composition was in flight, the incoming char is
		// the commit result — clear the preview BEFORE inserting so
		// the buffer + display stay consistent.
		e.Composition = ""
		ch := []rune(ev.Code)
		if len(ch) == 0 {
			return
		}
		runes = append(runes[:e.Cursor], append(ch, runes[e.Cursor:]...)...)
		e.Cursor += len(ch)
		e.Text = string(runes)
		if e.OnChange != nil {
			e.OnChange(e.Text)
		}
	case EventCompositionStart, EventCompositionUpdate:
		// Preview only — do NOT touch Text. Repaint responsibility
		// lies with the host, who typically calls Draw after each
		// composition event.
		e.Composition = ev.Code
	case EventCompositionEnd:
		// Cancel / commit-without-follow-up: drop the preview. When
		// the host follows up with EventChar (commit path), the
		// EventChar arm above will re-clear + insert.
		e.Composition = ""
	}
}

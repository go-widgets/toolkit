// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-opentype/bidi"

// TextDirection selects the base paragraph direction the toolkit uses when it
// reorders logical text into visual order before laying glyphs strictly
// left-to-right (see visualText). The zero value, DirLTR, is the default, and
// under it pure left-to-right text (Latin, digits, CJK) is reordered as a
// no-op — the visual order equals the logical order byte-for-byte, so existing
// rendering is unchanged.
type TextDirection int

const (
	// DirLTR forces a left-to-right base level. All-LTR text is unchanged.
	DirLTR TextDirection = iota
	// DirRTL forces a right-to-left base level, so neutral runs and trailing
	// whitespace resolve towards the right.
	DirRTL
	// DirAuto derives the base level from the first strong character of the
	// text (Unicode rules P2/P3), defaulting to left-to-right.
	DirAuto
)

// textDirection is the base direction every text primitive reorders against.
var textDirection = DirLTR

// SetTextDirection makes d the base direction used by DrawText and the font
// Draw paths when they reorder logical text to visual order. It affects only
// how mixed / right-to-left text is arranged; all-LTR text is untouched under
// the default DirLTR.
func SetTextDirection(d TextDirection) { textDirection = d }

// CurrentTextDirection returns the active base text direction.
func CurrentTextDirection() TextDirection { return textDirection }

// base maps the toolkit's TextDirection onto the bidi package's Direction.
func (d TextDirection) base() bidi.Direction {
	switch d {
	case DirRTL:
		return bidi.RightToLeft
	case DirAuto:
		return bidi.Auto
	default:
		return bidi.LeftToRight
	}
}

// hasRTL reports whether text contains any character that can force
// right-to-left arrangement — a strong right-to-left letter (Hebrew, Arabic:
// classes R / AL) or an Arabic number (AN). Pure-LTR text needs no reordering,
// so under the default DirLTR its visual order equals its logical order and the
// fast path in visualText keeps the bytes identical.
func hasRTL(text string) bool {
	for _, r := range text {
		switch bidi.ClassOf(r) {
		case bidi.R, bidi.AL, bidi.AN:
			return true
		}
	}
	return false
}

// visualText reorders text from logical order into visual (left-to-right)
// display order under the current TextDirection, so a downstream primitive that
// lays glyphs strictly left-to-right renders bidirectional text correctly. It
// is the single seam DrawText and every Font.Draw implementation pass their
// text through.
//
// Under the default DirLTR, all-LTR text takes a fast path that returns it
// unchanged, guaranteeing Latin/CJK rendering is byte-for-byte identical to the
// pre-bidi toolkit (bidi on all-LTR text is a no-op reorder).
//
// For text that needs arranging it runs the Unicode Bidirectional Algorithm
// (UAX #9): it resolves per-rune embedding levels, applies rule L4 glyph
// mirroring to paired punctuation in right-to-left runs, selects best-effort
// Arabic cursive presentation forms (the Unicode-level fallback — full
// contextual shaping additionally needs the font's GSUB init/medi/fina/isol
// features and ligatures), and finally permutes the runes to visual order
// (rule L2).
func visualText(text string) string {
	dir := textDirection
	if dir == DirLTR && !hasRTL(text) {
		return text
	}
	runes := []rune(text)
	levels := bidi.ResolveLevels(runes, dir.base())
	// Rule L4: substitute mirrored glyphs for paired punctuation that resolved
	// to a right-to-left level (e.g. "(" becomes ")" inside an RTL run).
	shaped := bidi.MirrorRunes(runes, levels)
	// Best-effort Arabic contextual forms: map each letter to its isolated /
	// initial / medial / final presentation form so isolated Arabic renders
	// with cursive-looking glyphs even without a GSUB shaper.
	forms := bidi.JoinForms(shaped)
	for i, r := range shaped {
		shaped[i] = bidi.PresentationForm(r, forms[i])
	}
	// Rule L2: reorder the (mirrored, shaped) runes into visual order.
	order := bidi.Reorder(shaped, levels)
	out := make([]rune, len(order))
	for i, idx := range order {
		out[i] = shaped[idx]
	}
	return string(out)
}

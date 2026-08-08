// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-opentype/fonts/notoemoji"
	"github.com/go-opentype/fonts/notosansthai"
)

// The emoji sequences below are written as explicit code points so the intent
// survives an editor that would otherwise normalise or hide the joiner.
const (
	person    = "\U0001F9D1" // 🧑 adult
	rocket    = "\U0001F680" // 🚀 rocket
	zwj       = "‍"          // zero-width joiner
	astronaut = person + zwj + rocket
	man       = "\U0001F468"
	woman     = "\U0001F469"
	boy       = "\U0001F466"
	family    = man + zwj + woman + zwj + boy
)

// emojiChain mirrors a real application chain: a Latin primary, then a script
// face that happens to carry the joiner (Thai ships U+200D for its own
// shaping), then the emoji face. The middle entry is the whole point — it is
// what used to steal the joiner.
func emojiChain(t *testing.T, px int) *fallbackFont {
	t.Helper()
	latin := newTTFace(t, testFontTTF, px)
	thai := newTTFace(t, notosansthai.TTF, px)
	emoji := newTTFace(t, notoemoji.TTF, px)
	f, err := NewFallbackFont(latin, thai, emoji)
	if err != nil {
		t.Fatal(err)
	}
	return f.(*fallbackFont)
}

func TestContinuesGrapheme(t *testing.T) {
	for _, r := range []rune{
		'‍', // ZWJ, Cf
		'‌', // ZWNJ, Cf
		'️', // VS16, Mn
		'́', // combining acute, Mn
		'ः', // Devanagari sign visarga, Mc
		'⃝', // combining enclosing circle, Me
	} {
		if !continuesGrapheme(r) {
			t.Errorf("U+%04X should continue the preceding grapheme", r)
		}
	}
	for _, r := range []rune{'a', '0', ' ', '中', '\U0001F680'} {
		if continuesGrapheme(r) {
			t.Errorf("U+%04X starts its own grapheme", r)
		}
	}
}

// TestZWJSequenceStaysOneRun is the regression this exists for. The joiner is
// carried by the Thai face, which comes first in the chain, so plain
// first-face-wins routing split the sequence into person | joiner | rocket —
// three runs, shaped independently, so the GSUB ligature composing them into a
// single astronaut never fired.
func TestZWJSequenceStaysOneRun(t *testing.T) {
	f := emojiChain(t, 32)

	// The Thai face really does claim the joiner, and the emoji face is not the
	// one that would win a plain first-face-wins race: without that, this test
	// would pass for the wrong reason.
	if !f.fonts[1].covers('\u200D') {
		t.Skip("chain assumption changed: the Thai face no longer carries the joiner")
	}

	runs := f.runs(astronaut)
	if len(runs) != 1 {
		got := make([]string, len(runs))
		for i, r := range runs {
			got[i] = r.text
		}
		t.Fatalf("the sequence split into %d runs %q, want one", len(runs), got)
	}
	if runs[0].text != astronaut {
		t.Fatalf("run text = %q, want the whole sequence", runs[0].text)
	}
}

// TestZWJSequenceComposesToOneGlyph is the user-visible consequence: the
// composed astronaut is ONE glyph, so it measures as one, not as person plus
// rocket side by side.
func TestZWJSequenceComposesToOneGlyph(t *testing.T) {
	f := emojiChain(t, 32)

	one := f.Measure(person)
	composed := f.Measure(astronaut)
	if composed != one {
		t.Fatalf("astronaut measures %d, want one glyph's %d (person+rocket would be %d)",
			composed, one, f.Measure(person+rocket))
	}
	// A three-part family sequence composes too.
	if got := f.Measure(family); got != one {
		t.Fatalf("family sequence measures %d, want one glyph's %d", got, one)
	}
}

// TestZWJSequenceDrawsOneGlyph proves it on pixels, not just metrics: the
// composed sequence must not paint the same ink as the two separate emoji.
func TestZWJSequenceDrawsOneGlyph(t *testing.T) {
	f := emojiChain(t, 48)
	// makeSurface fills the buffer with an opaque grey, so "ink" is any pixel the
	// draw actually changed, and maxX is how far right the run reached.
	const w, h = 400, 100
	ink := func(s string) (changed, maxX int) {
		buf := makeSurface(w, h)
		before := append([]byte(nil), buf...)
		f.Draw(newP(buf, w), 4, 4, s, RGBA{R: 0, G: 0, B: 0, A: 0xFF})
		maxX = -1
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 4
				if buf[i] != before[i] || buf[i+1] != before[i+1] || buf[i+2] != before[i+2] {
					changed++
					if x > maxX {
						maxX = x
					}
				}
			}
		}
		return changed, maxX
	}
	soloInk, _ := ink(person)
	seqInk, seqRight := ink(astronaut)
	_, pairRight := ink(person + rocket)

	if seqInk == 0 {
		t.Fatal("the composed sequence painted nothing")
	}
	// The whole sequence must fit inside ONE glyph's advance from the pen: that is
	// what composing means. Comparing ink extents to the person glyph directly
	// would be wrong — the astronaut is a different, slightly wider drawing at the
	// same advance.
	const penX = 4
	if adv := f.Measure(person); seqRight-penX > adv {
		t.Fatalf("the sequence's ink spans %d px, wider than one glyph's %d px advance — it did not compose",
			seqRight-penX, adv)
	}
	if seqRight >= pairRight {
		t.Fatalf("the sequence reaches x=%d, as far as the un-composed pair (x=%d)", seqRight, pairRight)
	}
	if seqInk == soloInk {
		t.Fatalf("the astronaut painted exactly the person's ink (%d px) — no substitution happened", seqInk)
	}
}

// TestCombiningMarkStaysWithItsBaseFace checks the general rule the ZWJ fix is
// one case of: a mark another face also carries must not be torn off its base
// when the base's own face can render it.
func TestCombiningMarkStaysWithItsBaseFace(t *testing.T) {
	f := emojiChain(t, 24)
	// VS16 is carried by the emoji face; a rocket followed by it is one run.
	runs := f.runs(rocket + "️")
	if len(runs) != 1 {
		t.Fatalf("rocket + VS16 split into %d runs, want one", len(runs))
	}
}

// TestUncoveredMarkStillFallsBack checks the rule is a preference, not a trap:
// when the current face CANNOT render the mark, routing still falls back to a
// face that can, exactly as before.
func TestUncoveredMarkStillFallsBack(t *testing.T) {
	latin := newTTFace(t, testFontTTF, 20)
	emoji := newTTFace(t, notoemoji.TTF, 20)
	f, err := NewFallbackFont(latin, emoji)
	if err != nil {
		t.Fatal(err)
	}
	fb := f.(*fallbackFont)
	// "a" is Latin-only; VS16 is emoji-only. The Latin face cannot render the
	// selector, so the run must split rather than draw .notdef.
	if !fb.fonts[0].covers('a') || fb.fonts[0].covers('️') {
		t.Skip("font assumption changed")
	}
	runs := fb.runs("a️")
	if len(runs) != 2 {
		t.Fatalf("an uncoverable mark should still fall back: got %d runs, want 2", len(runs))
	}
	if runs[1].font != fb.fonts[1] {
		t.Fatal("the mark should route to the face that covers it")
	}
}

// TestLeadingMarkPicksNormally covers the no-current-run branch: text starting
// with a mark has nothing to attach to, so it routes by coverage as usual.
func TestLeadingMarkPicksNormally(t *testing.T) {
	f := emojiChain(t, 20)
	runs := f.runs(zwj + "abc")
	if len(runs) == 0 {
		t.Fatal("want at least one run")
	}
	if runs[0].text == "" {
		t.Fatal("the leading mark must still be emitted")
	}
}

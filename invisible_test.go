// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-opentype/shape"
)

func TestInvisibleGlyphPredicate(t *testing.T) {
	if !invisible(shape.Glyph{GID: 0}) {
		t.Fatal(".notdef must not be painted")
	}
	if !invisible(shape.Glyph{GID: 42, Invisible: true}) {
		t.Fatal("a shaper-hidden glyph must not be painted")
	}
	if invisible(shape.Glyph{GID: 42}) {
		t.Fatal("an ordinary glyph must be painted")
	}
}

// TestDefaultIgnorablesLeaveNoMark is the user-visible contract: a soft hyphen,
// a word joiner or a stray variation selector inside a word must change neither
// the measured width nor a single pixel. Go Regular maps U+00AD to a real
// hyphen, so without the Invisible check it would stamp one mid-word.
func TestDefaultIgnorablesLeaveNoMark(t *testing.T) {
	f := newTTFace(t, testFontTTF, 28)

	const w, h = 400, 60
	render := func(s string) (buf []byte, width int) {
		buf = makeSurface(w, h)
		f.Draw(newP(buf, w), 4, 4, s, RGBA{R: 0, G: 0, B: 0, A: 0xFF})
		return buf, f.Measure(s)
	}
	plain, plainW := render("abcd")

	for _, r := range []rune{
		'\u00AD', // SOFT HYPHEN — a real glyph in this font
		'\u2060', // WORD JOINER — .notdef here
		'\u200B', // ZERO WIDTH SPACE
		'\u200D', // ZERO WIDTH JOINER
		'\uFEFF', // BYTE ORDER MARK
		'\uFE0F', // VARIATION SELECTOR-16
	} {
		got, gotW := render("ab" + string(r) + "cd")
		if gotW != plainW {
			t.Errorf("U+%04X: width %d, want %d — a default-ignorable must take no space", r, gotW, plainW)
		}
		for i := range got {
			if got[i] != plain[i] {
				t.Errorf("U+%04X: painted a different surface — it must leave no mark", r)
				break
			}
		}
	}
}

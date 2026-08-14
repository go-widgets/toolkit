// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-widgets/painter"
)

// sizedFont is a Font of a fixed, caller-chosen height/advance, so a test can
// give a PostCard element a distinctly-sized font and check it is honoured.
type sizedFont struct{ h, adv int }

func (f sizedFont) Height() int                                  { return f.h }
func (f sizedFont) Advance() int                                 { return f.adv }
func (f sizedFont) Measure(s string) int                         { return len(s) * f.adv }
func (f sizedFont) Draw(painter.Painter, int, int, string, RGBA) {}

// TestPostCardPerElementFonts checks the per-element fonts give the card a type
// hierarchy: a taller title font makes the whole card taller, and each element's
// font resolves to the one set (exercising orFont's non-nil branch).
func TestPostCardPerElementFonts(t *testing.T) {
	tall := sizedFont{h: 40, adv: 8}
	small := sizedFont{h: 10, adv: 4}
	c := NewPostCard("SRC", "chan", "A title", "meta")
	c.PillFont, c.SubtitleFont, c.MetaFont = small, small, small

	c.TitleFont = tall
	hTall := c.Measure(600)
	c.TitleFont = small
	hSmall := c.Measure(600)
	if hTall <= hSmall {
		t.Fatalf("a tall title font must make the card taller: tall=%d small=%d", hTall, hSmall)
	}

	// orFont's non-nil branch: each set element font resolves to itself.
	c.TitleFont, c.SubtitleFont, c.MetaFont, c.PillFont = tall, small, small, small
	if c.titleFont() != Font(tall) || c.subtitleFont() != Font(small) ||
		c.metaFont() != Font(small) || c.pillFont() != Font(small) {
		t.Fatal("per-element font resolution returned the wrong font")
	}
}

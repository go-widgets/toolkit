// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"

	"github.com/go-opentype/fonts/notosansthai"
	"github.com/go-widgets/painter"
)

func TestNewSyntheticBoldFontRejectsNil(t *testing.T) {
	if _, err := NewSyntheticBoldFont(nil); err == nil {
		t.Fatal("a nil font should error")
	}
}

// TestSyntheticBoldMetrics checks the wrapper re-spaces nothing: over-striking
// thickens glyphs, so the step and line height are the base face's. Only Measure
// grows, by the one column the second pass paints.
func TestSyntheticBoldMetrics(t *testing.T) {
	base := newTTFace(t, testFontTTF, 20)
	f, err := NewSyntheticBoldFont(base)
	if err != nil {
		t.Fatal(err)
	}
	if f.Advance() != base.Advance() {
		t.Fatalf("Advance = %d, want the base's %d", f.Advance(), base.Advance())
	}
	if f.Height() != base.Height() {
		t.Fatalf("Height = %d, want the base's %d", f.Height(), base.Height())
	}
	if got, want := f.Measure("Hello"), base.Measure("Hello")+syntheticBoldOffset; got != want {
		t.Fatalf("Measure = %d, want %d (base + the over-strike column)", got, want)
	}
	if got := f.Measure(""); got != 0 {
		t.Fatalf("empty Measure = %d, want 0", got)
	}
}

// TestSyntheticBoldPassThroughs checks the optional interfaces forward to the
// base face, so positioning and vector export keep working through the wrapper.
func TestSyntheticBoldPassThroughs(t *testing.T) {
	base := newTTFace(t, testFontTTF, 20)
	f, _ := NewSyntheticBoldFont(base)

	if a, ok := f.(interface{ Ascent() int }); !ok || a.Ascent() != base.Ascent() {
		t.Fatal("Ascent should forward to the base face")
	}
	if d, ok := f.(interface{ FontData() []byte }); !ok || len(d.FontData()) != len(testFontTTF) {
		t.Fatal("FontData should forward the base face's sfnt bytes")
	}
	if s, ok := f.(interface{ SizePx() int }); !ok || s.SizePx() != base.SizePx() {
		t.Fatal("SizePx should forward to the base face")
	}
}

// plainFont is a Font with none of the optional interfaces, exercising the
// zero-value fall-backs.
type plainFont struct{ w int }

func (p plainFont) Advance() int                               { return 1 }
func (p plainFont) Height() int                                { return 1 }
func (p plainFont) Measure(string) int                         { return p.w }
func (plainFont) Draw(painter.Painter, int, int, string, RGBA) {}

func TestSyntheticBoldFallsBackWhenBaseIsMinimal(t *testing.T) {
	f, _ := NewSyntheticBoldFont(plainFont{w: 7})
	if a := f.(interface{ Ascent() int }).Ascent(); a != 0 {
		t.Fatalf("Ascent = %d, want 0 for a base that exposes none", a)
	}
	if d := f.(interface{ FontData() []byte }).FontData(); d != nil {
		t.Fatal("FontData should be nil for a base that exposes none")
	}
	if s := f.(interface{ SizePx() int }).SizePx(); s != 0 {
		t.Fatalf("SizePx = %d, want 0 for a base that exposes none", s)
	}
}

// TestSyntheticBoldPaintsHeavier is the point of the wrapper: the same string
// must reach more pixels than the base face, and one column further right.
func TestSyntheticBoldPaintsHeavier(t *testing.T) {
	base := newTTFace(t, testFontTTF, 24)
	bold, _ := NewSyntheticBoldFont(base)

	const w, h = 300, 60
	ink := func(f Font) (changed, maxX int) {
		buf := makeSurface(w, h)
		before := append([]byte(nil), buf...)
		f.Draw(newP(buf, w), 4, 4, "Heading", RGBA{R: 0, G: 0, B: 0, A: 0xFF})
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
	baseInk, baseRight := ink(base)
	boldInk, boldRight := ink(bold)

	if boldInk <= baseInk {
		t.Fatalf("synthetic bold painted %d px, base %d — it must be heavier", boldInk, baseInk)
	}
	if boldRight != baseRight+syntheticBoldOffset {
		t.Fatalf("bold reaches x=%d, want the base's %d plus the over-strike column", boldRight, baseRight)
	}
}

// TestSyntheticBoldEmboldensAWholeChain checks the wrapper composes OVER a
// fallback chain, so a script the primary lacks is emboldened too rather than
// staying at body weight.
func TestSyntheticBoldEmboldensAWholeChain(t *testing.T) {
	latin := newTTFace(t, testFontTTF, 22)
	thai := newTTFace(t, notosansthai.TTF, 22)
	chain, err := NewFallbackFont(latin, thai)
	if err != nil {
		t.Fatal(err)
	}
	bold, err := NewSyntheticBoldFont(chain)
	if err != nil {
		t.Fatal(err)
	}
	const thaiText = "ไทย" // the primary cannot render this at all
	if got, want := bold.Measure(thaiText), chain.Measure(thaiText)+syntheticBoldOffset; got != want {
		t.Fatalf("Measure = %d, want %d", got, want)
	}

	const w, h = 200, 50
	count := func(f Font) int {
		buf := makeSurface(w, h)
		before := append([]byte(nil), buf...)
		f.Draw(newP(buf, w), 4, 4, thaiText, RGBA{R: 0, G: 0, B: 0, A: 0xFF})
		n := 0
		for i := 0; i+3 < len(buf); i += 4 {
			if buf[i] != before[i] {
				n++
			}
		}
		return n
	}
	if count(bold) <= count(chain) {
		t.Fatal("the fallback face's glyphs should be emboldened too")
	}
}

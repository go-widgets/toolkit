// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// Bitmap font + text-drawing helper. The 5x7 glyph table below is
// copied verbatim from wasmdesk/wasmbox's dock scene (see
// clients/dock/internal/scene/scene.go, `font5x7` map) and extended to
// cover the full ASCII subset the toolkit widgets need (digits, both
// letter cases, plus the punctuation listed in font.go's package doc).
// The bit layout is preserved: each glyph is 5 bytes (columns), the
// low 7 bits of each byte are rows top->bottom (bit 0 = top row,
// bit 6 = bottom row). The rendering loop below mirrors the dock's
// drawTextClipped column/row decode.

// Font is the toolkit's text metrics + rendering abstraction. Widgets lay
// themselves out against the ACTIVE font's metrics (via GlyphHeight /
// GlyphAdvance) and paint text through DrawText, so swapping the active font
// with SetFont rescales the whole UI's typography without touching any widget.
//
//   - Advance is the horizontal step from one glyph origin to the next.
//   - Height is the glyph box height.
//   - Draw paints text left-to-right at (x, y) in the given ink.
//
// All built-in widgets assume a monospace font (fixed Advance), which keeps
// grid-aligned layout math trivial.
type Font interface {
	Advance() int
	Height() int
	Draw(p painter.Painter, x, y int, text string, ink RGBA)
}

// baseGlyphW / baseGlyphH are the unscaled dimensions of the built-in 5x7
// bitmap (5 columns of body + 1 of spacing horizontally, 7 rows tall).
const (
	baseGlyphAdvance = 6
	baseGlyphHeight  = 7
)

// bitmapFont renders the built-in font5x7 table at an integer Scale: each lit
// bit becomes a Scale×Scale block, so Scale=2 doubles the type size ("retina"
// text) while staying a pure-integer, dependency-free rasteriser.
type bitmapFont struct{ Scale int }

// Advance is the scaled horizontal step per glyph.
func (f *bitmapFont) Advance() int { return baseGlyphAdvance * f.Scale }

// Height is the scaled glyph box height.
func (f *bitmapFont) Height() int { return baseGlyphHeight * f.Scale }

// NewBitmapFont returns the built-in 5x7 font scaled by the given integer
// factor (clamped to at least 1). SetFont(NewBitmapFont(2)) doubles all text.
func NewBitmapFont(scale int) Font {
	if scale < 1 {
		scale = 1
	}
	return &bitmapFont{Scale: scale}
}

// defaultFont is the unscaled 5x7 bitmap — the toolkit's out-of-the-box font.
var defaultFont = &bitmapFont{Scale: 1}

// activeFont is the font every widget currently lays out + renders against.
var activeFont Font = defaultFont

// SetFont makes f the active font. A nil f restores the built-in default. All
// subsequent layout (GlyphHeight / GlyphAdvance) and DrawText use it.
func SetFont(f Font) {
	if f == nil {
		f = defaultFont
	}
	activeFont = f
}

// CurrentFont returns the active font.
func CurrentFont() Font { return activeFont }

// GlyphHeight is the active font's glyph box height. It is a function (not a
// const) so widgets re-read it after SetFont; layout dimensions that derive
// from it are likewise functions.
func GlyphHeight() int { return activeFont.Height() }

// GlyphAdvance is the active font's horizontal step from one glyph to the next.
func GlyphAdvance() int { return activeFont.Advance() }

// font5x7 is the 5-column x 7-row bitmap font table. Each entry is one
// glyph: 5 bytes, one per column, low 7 bits encode the rows from top
// (bit 0) to bottom (bit 6). Characters not in the map render as a
// blank (the advance is still consumed so columns line up).
var font5x7 = map[byte][5]byte{
	// Digits.
	'0': {0x3E, 0x51, 0x49, 0x45, 0x3E},
	'1': {0x00, 0x42, 0x7F, 0x40, 0x00},
	'2': {0x42, 0x61, 0x51, 0x49, 0x46},
	'3': {0x21, 0x41, 0x45, 0x4B, 0x31},
	'4': {0x18, 0x14, 0x12, 0x7F, 0x10},
	'5': {0x27, 0x45, 0x45, 0x45, 0x39},
	'6': {0x3C, 0x4A, 0x49, 0x49, 0x30},
	'7': {0x01, 0x71, 0x09, 0x05, 0x03},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'9': {0x06, 0x49, 0x49, 0x29, 0x1E},

	// Upper-case letters.
	'A': {0x7E, 0x11, 0x11, 0x11, 0x7E},
	'B': {0x7F, 0x49, 0x49, 0x49, 0x36},
	'C': {0x3E, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7F, 0x41, 0x41, 0x22, 0x1C},
	'E': {0x7F, 0x49, 0x49, 0x49, 0x41},
	'F': {0x7F, 0x09, 0x09, 0x09, 0x01},
	'G': {0x3E, 0x41, 0x49, 0x49, 0x7A},
	'H': {0x7F, 0x08, 0x08, 0x08, 0x7F},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'J': {0x20, 0x40, 0x41, 0x3F, 0x01},
	'K': {0x7F, 0x08, 0x14, 0x22, 0x41},
	'L': {0x7F, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7F, 0x02, 0x0C, 0x02, 0x7F},
	'N': {0x7F, 0x04, 0x08, 0x10, 0x7F},
	'O': {0x3E, 0x41, 0x41, 0x41, 0x3E},
	'P': {0x7F, 0x09, 0x09, 0x09, 0x06},
	'Q': {0x3E, 0x41, 0x51, 0x21, 0x5E},
	'R': {0x7F, 0x09, 0x19, 0x29, 0x46},
	'S': {0x46, 0x49, 0x49, 0x49, 0x31},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'U': {0x3F, 0x40, 0x40, 0x40, 0x3F},
	'V': {0x1F, 0x20, 0x40, 0x20, 0x1F},
	'W': {0x7F, 0x20, 0x18, 0x20, 0x7F},
	'X': {0x63, 0x14, 0x08, 0x14, 0x63},
	'Y': {0x07, 0x08, 0x70, 0x08, 0x07},
	'Z': {0x61, 0x51, 0x49, 0x45, 0x43},

	// Lower-case letters.
	'a': {0x20, 0x54, 0x54, 0x54, 0x78},
	'b': {0x7F, 0x48, 0x44, 0x44, 0x38},
	'c': {0x38, 0x44, 0x44, 0x44, 0x20},
	'd': {0x38, 0x44, 0x44, 0x48, 0x7F},
	'e': {0x38, 0x54, 0x54, 0x54, 0x18},
	'f': {0x08, 0x7E, 0x09, 0x01, 0x02},
	'g': {0x0C, 0x52, 0x52, 0x52, 0x3E},
	'h': {0x7F, 0x08, 0x04, 0x04, 0x78},
	'i': {0x00, 0x44, 0x7D, 0x40, 0x00},
	'j': {0x20, 0x40, 0x44, 0x3D, 0x00},
	'k': {0x7F, 0x10, 0x28, 0x44, 0x00},
	'l': {0x00, 0x41, 0x7F, 0x40, 0x00},
	'm': {0x7C, 0x04, 0x18, 0x04, 0x78},
	'n': {0x7C, 0x08, 0x04, 0x04, 0x78},
	'o': {0x38, 0x44, 0x44, 0x44, 0x38},
	'p': {0x7C, 0x14, 0x14, 0x14, 0x08},
	'q': {0x08, 0x14, 0x14, 0x18, 0x7C},
	'r': {0x7C, 0x08, 0x04, 0x04, 0x08},
	's': {0x48, 0x54, 0x54, 0x54, 0x20},
	't': {0x04, 0x3F, 0x44, 0x40, 0x20},
	'u': {0x3C, 0x40, 0x40, 0x20, 0x7C},
	'v': {0x1C, 0x20, 0x40, 0x20, 0x1C},
	'w': {0x3C, 0x40, 0x30, 0x40, 0x3C},
	'x': {0x44, 0x28, 0x10, 0x28, 0x44},
	'y': {0x0C, 0x50, 0x50, 0x50, 0x3C},
	'z': {0x44, 0x64, 0x54, 0x4C, 0x44},

	// Punctuation + symbols.
	' ': {0x00, 0x00, 0x00, 0x00, 0x00},
	'.': {0x00, 0x60, 0x60, 0x00, 0x00},
	',': {0x00, 0x50, 0x30, 0x00, 0x00},
	':': {0x00, 0x36, 0x36, 0x00, 0x00},
	'-': {0x08, 0x08, 0x08, 0x08, 0x08},
	'_': {0x40, 0x40, 0x40, 0x40, 0x40},
	'/': {0x20, 0x10, 0x08, 0x04, 0x02},
	'?': {0x02, 0x01, 0x51, 0x09, 0x06},
	'!': {0x00, 0x00, 0x5F, 0x00, 0x00},
	'(': {0x00, 0x1C, 0x22, 0x41, 0x00},
	')': {0x00, 0x41, 0x22, 0x1C, 0x00},
	'<': {0x08, 0x14, 0x22, 0x41, 0x00},
	'>': {0x00, 0x41, 0x22, 0x14, 0x08},
	'+': {0x08, 0x08, 0x3E, 0x08, 0x08},
	'*': {0x14, 0x08, 0x3E, 0x08, 0x14},
	'=': {0x14, 0x14, 0x14, 0x14, 0x14},
	'#': {0x14, 0x7F, 0x14, 0x7F, 0x14},
	'%': {0x62, 0x64, 0x08, 0x13, 0x23}, // percent — needed by the calculator
}

// TextWidth returns the pixel width that DrawText would occupy if it
// rendered text in the active font. Every character (known or unknown)
// consumes one GlyphAdvance slot so callers can pre-size text containers from
// len(text) alone.
func TextWidth(text string) int { return GlyphAdvance() * len(text) }

// DrawText paints text left-to-right starting at (x, y) in widget-local
// coordinates, using the active font (see SetFont). It is a thin wrapper over
// the active Font's Draw so every widget's text rendering follows a font swap.
func DrawText(p painter.Painter, x, y int, text string, ink RGBA) {
	activeFont.Draw(p, x, y, text, ink)
}

// Draw paints text with the bitmap font. On a *painter.PixelPainter (the WUI +
// GUI path) each lit bit of the 5x7 glyph becomes a Scale×Scale block whose
// origin is (x + k*Advance, y) for the k-th rune, so the font scales cleanly.
// On any other painter (a *painter.CellPainter for a TUI, an SvgPainter for
// vector output) it delegates to the painter's own Text primitive — one rune
// per cell / a native <text> — where the pixel scale is not meaningful.
//
// Unknown characters render blank but still advance the cursor so column
// alignment is preserved; pixels clip per-pixel so overflowing glyphs degrade
// gracefully.
func (f *bitmapFont) Draw(p painter.Painter, x, y int, text string, ink RGBA) {
	if _, isPixel := p.(*painter.PixelPainter); !isPixel {
		p.Text(x, y, text, ink)
		return
	}
	adv := f.Advance()
	for k := 0; k < len(text); k++ {
		bits, ok := font5x7[text[k]]
		if !ok {
			continue
		}
		gx := x + k*adv
		for col := 0; col < 5; col++ {
			cb := bits[col]
			for row := 0; row < baseGlyphHeight; row++ {
				if cb&(1<<row) == 0 {
					continue
				}
				// Paint the Scale×Scale block for this lit bit.
				for dy := 0; dy < f.Scale; dy++ {
					for dx := 0; dx < f.Scale; dx++ {
						p.PutPixel(gx+col*f.Scale+dx, y+row*f.Scale+dy, ink)
					}
				}
			}
		}
	}
}

// putPixel writes one ink pixel at (px, py) via the Painter p.
// Retained as a package-internal shim so DrawText and the handful
// of pixel-precise widgets (Calendar's day-dot, Spinner's frame)
// read as short function calls rather than direct interface calls.
func putPixel(p painter.Painter, px, py int, ink RGBA) {
	p.PutPixel(px, py, ink)
}

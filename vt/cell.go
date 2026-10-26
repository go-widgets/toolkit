// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package vt

// Color is a cell colour: the terminal's default, one of the 256 indexed
// colours, or a 24-bit RGB value. The zero value is the default colour, so a
// zero Cell is a blank cell in the default colours.
type Color uint32

// Default is the terminal's default foreground or background colour.
const Default Color = 0

const (
	kindIndexed = 1 << 24
	kindRGB     = 2 << 24
	kindMask    = 0xff << 24
)

// Indexed returns indexed colour n: 0-7 the standard colours, 8-15 their
// bright forms, 16-231 the 6x6x6 cube, 232-255 the grey ramp.
func Indexed(n uint8) Color { return Color(kindIndexed | uint32(n)) }

// RGB returns a 24-bit colour.
func RGB(r, g, b uint8) Color {
	return Color(kindRGB | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// Index reports the colour's index when it is an indexed colour.
func (c Color) Index() (uint8, bool) {
	return uint8(c), c&kindMask == kindIndexed
}

// RGB reports the colour's components when it is a 24-bit colour.
func (c Color) RGB() (r, g, b uint8, ok bool) {
	return uint8(c >> 16), uint8(c >> 8), uint8(c), c&kindMask == kindRGB
}

// Attr is a set of SGR rendition flags.
type Attr uint8

// The renditions the emulator keeps. Blink is accepted and dropped: a
// blinking cell is an attention-grabber a safe terminal does not need.
const (
	Bold Attr = 1 << iota
	Faint
	Italic
	Underline
	Reverse
	Conceal
	Strike
)

// Cell is one character cell. A zero Rune is a blank (nothing was written
// there); FG and BG are the colours the cell was written or erased with.
type Cell struct {
	Rune   rune
	FG, BG Color
	Attr   Attr
}

// Pos addresses a cell in the whole history: Line counts from the oldest
// scrollback line (0) through the visible screen, Col from the left (0).
type Pos struct {
	Line, Col int
}

// Before reports whether p comes before q in reading order.
func (p Pos) Before(q Pos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}

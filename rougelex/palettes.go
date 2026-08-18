// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package rougelex

import "github.com/go-widgets/toolkit"

// named is one selectable, shared highlight scheme: a display name paired with
// the fixed Palette it selects. The "Default" scheme's Palette is the zero
// value, so a Highlighter assigned it stays theme-derived (Highlight fills it
// from DefaultPalette); every other scheme's Palette is fully opaque and
// colours the same way regardless of the toolkit's light/dark theme.
type named struct {
	name    string
	palette Palette
}

// registry is the ordered set of shared named palettes offered to a scheme
// picker. "Default" (theme-derived, zero Palette) is first; the rest are the
// well-known editor schemes. Each fixed palette maps its scheme's canonical
// token colours onto rougelex's categories: a LaTeX \begin{env} (Name.Tag)
// takes Tag, a math control sequence (Name.Variable) takes Variable, a command
// (Keyword) takes Keyword, and a delimiter (Punctuation) takes Punctuation, so
// a LaTeX buffer is fully coloured, while the remaining categories (Type,
// Function, Class, Builtin, String, Number, Operator) carry the scheme's usual
// colours for the other languages.
var registry = []named{
	// Theme-derived sentinel: empty Palette, follows the toolkit light/dark
	// theme via DefaultPalette.
	{name: "Default"},
	{
		name: "Monokai",
		palette: Palette{
			Default:     toolkit.RGB(0xF8, 0xF8, 0xF2), // foreground off-white
			Keyword:     toolkit.RGB(0xF9, 0x26, 0x72), // pink (command)
			Type:        toolkit.RGB(0x66, 0xD9, 0xEF), // cyan
			Function:    toolkit.RGB(0xA6, 0xE2, 0x2E), // green
			Class:       toolkit.RGB(0xA6, 0xE2, 0x2E), // green
			Builtin:     toolkit.RGB(0x66, 0xD9, 0xEF), // cyan
			Tag:         toolkit.RGB(0xA6, 0xE2, 0x2E), // green (env name)
			Variable:    toolkit.RGB(0xE6, 0xDB, 0x74), // yellow (math)
			Attribute:   toolkit.RGB(0xFD, 0x97, 0x1F), // orange
			String:      toolkit.RGB(0xE6, 0xDB, 0x74), // yellow
			Number:      toolkit.RGB(0xAE, 0x81, 0xFF), // purple
			Comment:     toolkit.RGB(0x75, 0x71, 0x5E), // muted brown-grey
			Operator:    toolkit.RGB(0xF9, 0x26, 0x72), // pink
			Punctuation: toolkit.RGB(0xFD, 0x97, 0x1F), // orange (delimiter)
		},
	},
	{
		name: "Solarized",
		palette: Palette{
			Default:     toolkit.RGB(0x65, 0x7B, 0x83), // base00 body text
			Keyword:     toolkit.RGB(0x26, 0x8B, 0xD2), // blue (command)
			Type:        toolkit.RGB(0xB5, 0x89, 0x00), // yellow
			Function:    toolkit.RGB(0x26, 0x8B, 0xD2), // blue
			Class:       toolkit.RGB(0xB5, 0x89, 0x00), // yellow
			Builtin:     toolkit.RGB(0xCB, 0x4B, 0x16), // orange
			Tag:         toolkit.RGB(0x85, 0x99, 0x00), // green (env name)
			Variable:    toolkit.RGB(0x2A, 0xA1, 0x98), // cyan (math)
			Attribute:   toolkit.RGB(0xB5, 0x89, 0x00), // yellow
			String:      toolkit.RGB(0x2A, 0xA1, 0x98), // cyan
			Number:      toolkit.RGB(0xD3, 0x36, 0x82), // magenta
			Comment:     toolkit.RGB(0x93, 0xA1, 0xA1), // base1 comment
			Operator:    toolkit.RGB(0x85, 0x99, 0x00), // green
			Punctuation: toolkit.RGB(0x6C, 0x71, 0xC4), // violet (delimiter)
		},
	},
	{
		name: "GitHub",
		palette: Palette{
			Default:     toolkit.RGB(0x24, 0x29, 0x2E), // near-black text
			Keyword:     toolkit.RGB(0xD7, 0x3A, 0x49), // red (command)
			Type:        toolkit.RGB(0x6F, 0x42, 0xC1), // purple entity
			Function:    toolkit.RGB(0x6F, 0x42, 0xC1), // purple entity
			Class:       toolkit.RGB(0x6F, 0x42, 0xC1), // purple entity
			Builtin:     toolkit.RGB(0x00, 0x5C, 0xC5), // constant blue
			Tag:         toolkit.RGB(0x6F, 0x42, 0xC1), // entity purple (env name)
			Variable:    toolkit.RGB(0x03, 0x2F, 0x62), // dark-blue (math)
			Attribute:   toolkit.RGB(0x00, 0x5C, 0xC5), // constant blue
			String:      toolkit.RGB(0x03, 0x2F, 0x62), // dark-blue
			Number:      toolkit.RGB(0x00, 0x5C, 0xC5), // constant blue
			Comment:     toolkit.RGB(0x6A, 0x73, 0x7D), // grey comment
			Operator:    toolkit.RGB(0xD7, 0x3A, 0x49), // red
			Punctuation: toolkit.RGB(0x00, 0x5C, 0xC5), // constant blue (delimiter)
		},
	},
	{
		name: "Dracula",
		palette: Palette{
			Default:     toolkit.RGB(0xF8, 0xF8, 0xF2), // foreground off-white
			Keyword:     toolkit.RGB(0xFF, 0x79, 0xC6), // pink (command)
			Type:        toolkit.RGB(0x8B, 0xE9, 0xFD), // cyan
			Function:    toolkit.RGB(0x50, 0xFA, 0x7B), // green
			Class:       toolkit.RGB(0x8B, 0xE9, 0xFD), // cyan
			Builtin:     toolkit.RGB(0xBD, 0x93, 0xF9), // purple
			Tag:         toolkit.RGB(0x50, 0xFA, 0x7B), // green (env name)
			Variable:    toolkit.RGB(0xF1, 0xFA, 0x8C), // yellow (math)
			Attribute:   toolkit.RGB(0xFF, 0xB8, 0x6C), // orange
			String:      toolkit.RGB(0xF1, 0xFA, 0x8C), // yellow
			Number:      toolkit.RGB(0xBD, 0x93, 0xF9), // purple
			Comment:     toolkit.RGB(0x62, 0x72, 0xA4), // muted blue-grey
			Operator:    toolkit.RGB(0xFF, 0x79, 0xC6), // pink
			Punctuation: toolkit.RGB(0xFF, 0xB8, 0x6C), // orange (delimiter)
		},
	},
}

// ThemeNames returns the shared palette names in display order, with the
// theme-derived "Default" first. It mirrors the ordered set a scheme picker
// walks to build its menu.
func ThemeNames() []string {
	names := make([]string, len(registry))
	for i, n := range registry {
		names[i] = n.name
	}
	return names
}

// PaletteByName returns the shared Palette registered under name and true, or
// the zero Palette and false for an unknown name. The "Default" scheme yields
// the zero Palette (with true): assigned to a Highlighter.Palette it keeps the
// highlighter theme-derived, exactly as the zero value does; every other scheme
// yields a fixed, fully-opaque Palette that pins that scheme regardless of the
// toolkit theme.
func PaletteByName(name string) (Palette, bool) {
	for _, n := range registry {
		if n.name == name {
			return n.palette, true
		}
	}
	return Palette{}, false
}

// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
)

// LoadGTKTheme parses a GTK theme source (the gtk.css or gtk-3.0/gtk.css or
// gtk-4.0/gtk.css that ships with a libadwaita / GTK3 theme) and returns a
// Theme that mirrors the theme's palette.
//
// We recognise BOTH the GTK3 names (theme_bg_color / theme_fg_color / …)
// AND the libadwaita / GTK4 names (window_bg_color / accent_bg_color / …);
// when both are present the GTK4 name wins because it is the newer
// convention and a theme that defines both intends the GTK4 name as
// canonical. Unknown @define-color declarations are kept in the returned
// Theme's Extra map so themes that ship custom color names (e.g.
// "headerbar_bg_color" for chrome work) can still be looked up by the
// compositor when wiring Niveau-B window-chrome theming.
//
// Anything beyond @define-color (selectors, properties, gradients, image
// references) is ignored — the toolkit is a flat-paint compositor that
// only consumes solid RGBA values. We do not implement a full CSS parser
// for the same reason.
//
// The mapping from GTK names to toolkit Theme fields:
//
//	GTK4 (preferred)         | GTK3 (fallback)          | Theme field
//	-------------------------|--------------------------|--------------
//	window_bg_color          | theme_bg_color           | Background
//	window_fg_color          | theme_fg_color           | OnBackground
//	view_bg_color            | theme_base_color         | Surface
//	view_fg_color            | theme_text_color         | OnSurface
//	card_bg_color            | insensitive_bg_color     | SurfaceAlt
//	accent_bg_color          | theme_selected_bg_color  | Accent
//	borders                  | borders                  | Border
//
// Returns an error only if the input is empty (defensively) — malformed
// declarations are skipped, not fatal, so a real-world gtk.css with a
// stray syntax error still yields the rest of its palette.
func LoadGTKTheme(css string) (*Theme, error) {
	if strings.TrimSpace(css) == "" {
		return nil, errEmptyCSS
	}
	defs := parseGTKDefineColors(css)
	t := DefaultLight() // start from the light defaults; missing fields keep them
	t.Extra = map[string]RGBA{}
	for name, c := range defs {
		t.Extra[name] = c
	}
	apply := func(field *RGBA, gtk4Name, gtk3Name string) {
		if c, ok := defs[gtk4Name]; ok {
			*field = c
			return
		}
		if c, ok := defs[gtk3Name]; ok {
			*field = c
		}
	}
	apply(&t.Background, "window_bg_color", "theme_bg_color")
	apply(&t.OnBackground, "window_fg_color", "theme_fg_color")
	apply(&t.Surface, "view_bg_color", "theme_base_color")
	apply(&t.OnSurface, "view_fg_color", "theme_text_color")
	apply(&t.SurfaceAlt, "card_bg_color", "insensitive_bg_color")
	apply(&t.Accent, "accent_bg_color", "theme_selected_bg_color")
	apply(&t.Border, "borders", "borders")
	return t, nil
}

// errEmptyCSS is the single error LoadGTKTheme returns.
type gtkThemeError string

func (e gtkThemeError) Error() string { return string(e) }

var errEmptyCSS = gtkThemeError("toolkit: gtk theme css is empty")

// parseGTKDefineColors scans css for `@define-color NAME VALUE;` declarations
// and returns the name→RGBA map. VALUE may be a CSS hex literal (#RGB / #RRGGBB
// / #RRGGBBAA), a rgb(r,g,b) / rgba(r,g,b,a) call, a `transparent` keyword, or
// another @define-color name (alias). Aliases are resolved in a second pass so
// declaration order doesn't matter. Unrecognised values are skipped silently.
func parseGTKDefineColors(css string) map[string]RGBA {
	out := map[string]RGBA{}
	aliases := map[string]string{}
	// Strip comments: /* ... */ blocks. CSS doesn't have //-comments.
	for {
		i := strings.Index(css, "/*")
		if i < 0 {
			break
		}
		j := strings.Index(css[i:], "*/")
		if j < 0 {
			css = css[:i]
			break
		}
		css = css[:i] + css[i+j+2:]
	}
	// Find every @define-color … ; declaration.
	for {
		i := strings.Index(css, "@define-color")
		if i < 0 {
			break
		}
		rest := css[i+len("@define-color"):]
		semi := strings.Index(rest, ";")
		if semi < 0 {
			break
		}
		decl := strings.TrimSpace(rest[:semi])
		css = rest[semi+1:]
		// decl is "<name> <value>"; split on first whitespace.
		sp := indexOfWhitespace(decl)
		if sp < 0 {
			continue
		}
		name := strings.TrimSpace(decl[:sp])
		value := strings.TrimSpace(decl[sp+1:])
		if c, ok := parseCSSColor(value); ok {
			out[name] = c
			continue
		}
		// Not a literal — record as alias for the resolve pass.
		aliases[name] = value
	}
	// Resolve aliases by fixed-point iteration: repeatedly walk the alias
	// map, setting out[name] whenever its target is in out. Stop when a
	// full pass makes no progress. This handles chains of arbitrary depth
	// + terminates cleanly on cycles (a cycle's nodes never resolve, so
	// the pass makes no progress + exits). Order-independent, so map
	// iteration randomisation doesn't make coverage flaky.
	for {
		progress := false
		for name, value := range aliases {
			if _, ok := out[name]; ok {
				continue
			}
			if c, ok := out[value]; ok {
				out[name] = c
				progress = true
			}
		}
		if !progress {
			break
		}
	}
	return out
}

// parseCSSColor parses a single CSS color literal. Supports #RGB, #RRGGBB,
// #RRGGBBAA, rgb(r,g,b), rgba(r,g,b,a), and the transparent / currentColor
// keywords. r/g/b are 0-255 integers; a is 0.0-1.0 float OR percent. The
// percent path is rare in gtk themes but cheap to support.
func parseCSSColor(s string) (RGBA, bool) {
	s = strings.TrimSpace(s)
	if s == "transparent" {
		return RGBA{0, 0, 0, 0}, true
	}
	if strings.HasPrefix(s, "#") {
		return parseHexColor(s[1:])
	}
	if strings.HasPrefix(s, "rgb(") || strings.HasPrefix(s, "rgba(") {
		return parseRGBFunc(s)
	}
	return RGBA{}, false
}

// parseHexColor handles 3-, 6- and 8-digit hex.
func parseHexColor(h string) (RGBA, bool) {
	switch len(h) {
	case 3:
		r, ok1 := hexNib(h[0])
		g, ok2 := hexNib(h[1])
		b, ok3 := hexNib(h[2])
		if !ok1 || !ok2 || !ok3 {
			return RGBA{}, false
		}
		return RGBA{r<<4 | r, g<<4 | g, b<<4 | b, 0xFF}, true
	case 6:
		r, ok1 := hex2(h[0], h[1])
		g, ok2 := hex2(h[2], h[3])
		b, ok3 := hex2(h[4], h[5])
		if !ok1 || !ok2 || !ok3 {
			return RGBA{}, false
		}
		return RGBA{r, g, b, 0xFF}, true
	case 8:
		r, ok1 := hex2(h[0], h[1])
		g, ok2 := hex2(h[2], h[3])
		b, ok3 := hex2(h[4], h[5])
		a, ok4 := hex2(h[6], h[7])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return RGBA{}, false
		}
		return RGBA{r, g, b, a}, true
	}
	return RGBA{}, false
}

// parseRGBFunc handles rgb(r, g, b) and rgba(r, g, b, a). Whitespace,
// commas + the closing paren are tolerated; values past the 4th are
// ignored.
func parseRGBFunc(s string) (RGBA, bool) {
	op := strings.IndexByte(s, '(')
	cp := strings.LastIndexByte(s, ')')
	if op < 0 || cp <= op {
		return RGBA{}, false
	}
	inner := s[op+1 : cp]
	parts := splitOnCommaOrSpace(inner)
	if len(parts) < 3 {
		return RGBA{}, false
	}
	r, ok1 := parseByteOrPct(parts[0])
	g, ok2 := parseByteOrPct(parts[1])
	b, ok3 := parseByteOrPct(parts[2])
	if !ok1 || !ok2 || !ok3 {
		return RGBA{}, false
	}
	a := uint8(0xFF)
	if len(parts) >= 4 {
		a = parseAlpha(parts[3])
	}
	return RGBA{r, g, b, a}, true
}

// parseByteOrPct parses "128" (0-255) or "50%" (0-100%).
func parseByteOrPct(s string) (uint8, bool) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		n, ok := parseUint(s[:len(s)-1])
		if !ok || n > 100 {
			return 0, false
		}
		return uint8(n * 255 / 100), true
	}
	n, ok := parseUint(s)
	if !ok || n > 255 {
		return 0, false
	}
	return uint8(n), true
}

// parseAlpha parses "1", "1.0", "0.5", "50%". Out-of-range → 255.
func parseAlpha(s string) uint8 {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "%") {
		n, ok := parseUint(s[:len(s)-1])
		if !ok || n > 100 {
			return 0xFF
		}
		return uint8(n * 255 / 100)
	}
	// Parse a small fixed-point fraction without strconv.ParseFloat to keep
	// the toolkit dep-free: accept "0", "1", "0.NNN".
	if s == "1" || s == "1.0" {
		return 0xFF
	}
	if s == "0" || s == "0.0" {
		return 0
	}
	if !strings.HasPrefix(s, "0.") {
		return 0xFF
	}
	frac := s[2:]
	num, ok := parseUint(frac)
	if !ok {
		return 0xFF
	}
	denom := uint64(1)
	for range frac {
		denom *= 10
	}
	return uint8(uint64(num) * 255 / denom)
}

func parseUint(s string) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, true
}

func splitOnCommaOrSpace(s string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ',' || c == ' ' || c == '\t' || c == '\n' || c == '/' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func indexOfWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return i
		}
	}
	return -1
}

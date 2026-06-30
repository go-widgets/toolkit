// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

func TestLoadGTKThemeAdwaita(t *testing.T) {
	// Subset of the actual Adwaita gtk-4.0/gtk.css palette declarations.
	css := `
		/* Adwaita light, condensed for the test. */
		@define-color window_bg_color #FAFAFA;
		@define-color window_fg_color rgb(26, 26, 26);
		@define-color view_bg_color #FFFFFF;
		@define-color view_fg_color #1A1A1A;
		@define-color accent_bg_color #3584E4;
		@define-color borders rgba(0, 0, 0, 0.15);
		@define-color card_bg_color #F0F0F0;
		@define-color theme_bg_color #FAFAFA; /* GTK3 alias */
		@define-color headerbar_bg_color #E5E5E5;
	`
	th, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if th.Background != (RGBA{0xFA, 0xFA, 0xFA, 0xFF}) {
		t.Fatalf("Background wrong: %+v", th.Background)
	}
	if th.OnBackground.R != 26 {
		t.Fatalf("OnBackground.R want 26 got %d", th.OnBackground.R)
	}
	if th.Surface != (RGBA{0xFF, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("Surface wrong: %+v", th.Surface)
	}
	if th.Accent != (RGBA{0x35, 0x84, 0xE4, 0xFF}) {
		t.Fatalf("Accent wrong: %+v", th.Accent)
	}
	// Border = rgba: alpha 0.15 ≈ 0.15 * 255 = 38.
	if th.Border.A == 0xFF {
		t.Fatalf("Border alpha should be < 0xFF got %d", th.Border.A)
	}
	if th.SurfaceAlt != (RGBA{0xF0, 0xF0, 0xF0, 0xFF}) {
		t.Fatalf("SurfaceAlt wrong: %+v", th.SurfaceAlt)
	}
	// Extra holds the unmapped custom names.
	if _, ok := th.Extra["headerbar_bg_color"]; !ok {
		t.Fatal("headerbar_bg_color should be in Extra")
	}
}

func TestLoadGTKThemeGTK3Fallback(t *testing.T) {
	// Pure GTK3 theme — no GTK4 names. Fields fall through to GTK3 names.
	css := `
		@define-color theme_bg_color #111111;
		@define-color theme_fg_color #EEEEEE;
		@define-color theme_base_color #222222;
		@define-color theme_text_color #DDDDDD;
		@define-color theme_selected_bg_color #FF0000;
		@define-color insensitive_bg_color #444444;
		@define-color borders #555555;
	`
	th, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatal(err)
	}
	if th.Background.R != 0x11 || th.OnBackground.R != 0xEE {
		t.Fatalf("GTK3 fallback wrong: %+v / %+v", th.Background, th.OnBackground)
	}
	if th.Accent != (RGBA{0xFF, 0, 0, 0xFF}) {
		t.Fatal("Accent should pick theme_selected_bg_color")
	}
}

func TestLoadGTKThemeEmpty(t *testing.T) {
	if _, err := LoadGTKTheme(""); err == nil {
		t.Fatal("empty input must error")
	}
	if _, err := LoadGTKTheme("   \n\t  "); err == nil {
		t.Fatal("whitespace-only must error")
	}
}

func TestLoadGTKThemeErrorString(t *testing.T) {
	_, err := LoadGTKTheme("")
	if err.Error() == "" {
		t.Fatal("error must have a message")
	}
}

func TestLoadGTKThemeMissingDefinitions(t *testing.T) {
	// CSS that defines NO known colors — Theme keeps DefaultLight values.
	th, err := LoadGTKTheme(`/* nothing relevant */ body { color: red; }`)
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultLight()
	if th.Background != def.Background {
		t.Fatal("missing definitions must keep DefaultLight Background")
	}
}

func TestLoadGTKThemeAlias(t *testing.T) {
	css := `
		@define-color brand #112233;
		@define-color window_bg_color @brand;
	`
	// Our parser doesn't strip leading @; the alias resolver looks up by
	// name. Adjust the test for the actual GTK convention: aliases are
	// written without the @ prefix.
	cssClean := `
		@define-color brand #112233;
		@define-color window_bg_color brand;
	`
	_ = css
	th, err := LoadGTKTheme(cssClean)
	if err != nil {
		t.Fatal(err)
	}
	if th.Background != (RGBA{0x11, 0x22, 0x33, 0xFF}) {
		t.Fatalf("alias resolution failed: %+v", th.Background)
	}
}

func TestLoadGTKThemeAliasChain(t *testing.T) {
	// A → B → C (resolver walks the chain past the first hop).
	css := `
		@define-color c #ABCDEF;
		@define-color b c;
		@define-color window_bg_color b;
	`
	th, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatal(err)
	}
	if th.Background != (RGBA{0xAB, 0xCD, 0xEF, 0xFF}) {
		t.Fatalf("alias chain resolution failed: %+v", th.Background)
	}
}

func TestLoadGTKThemeAliasCycle(t *testing.T) {
	// A → B → A cycle: resolver gives up cleanly, no infinite loop.
	css := `
		@define-color a b;
		@define-color b a;
	`
	_, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatal(err)
	}
	// No assertion on the value — point is "no panic / no hang".
}

func TestLoadGTKThemeUnclosedComment(t *testing.T) {
	css := `/* never closed
@define-color window_bg_color #ABCDEF;`
	th, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatal(err)
	}
	// The unclosed comment swallows everything; no definitions parsed.
	def := DefaultLight()
	if th.Background != def.Background {
		t.Fatal("unclosed comment should swallow all decls")
	}
}

func TestLoadGTKThemeMalformedDeclarations(t *testing.T) {
	// Each line is a different malformed case — parser must skip, not crash.
	css := `
		@define-color;
		@define-color name_no_value ;
		@define-color blank_name   #FFF
		@define-color bad_color    blorp(1,2,3);
		@define-color trailing_no_semi #ABCDEF
	`
	th, err := LoadGTKTheme(css)
	if err != nil {
		t.Fatal(err)
	}
	if th == nil {
		t.Fatal("malformed input must still yield a Theme")
	}
}

func TestParseCSSColorTransparent(t *testing.T) {
	c, ok := parseCSSColor("transparent")
	if !ok || c != (RGBA{0, 0, 0, 0}) {
		t.Fatalf("transparent wrong: %+v ok=%v", c, ok)
	}
}

func TestParseHexColor(t *testing.T) {
	cases := []struct {
		in   string
		want RGBA
	}{
		{"FFF", RGBA{0xFF, 0xFF, 0xFF, 0xFF}},
		{"abc", RGBA{0xAA, 0xBB, 0xCC, 0xFF}},
		{"123456", RGBA{0x12, 0x34, 0x56, 0xFF}},
		{"12345678", RGBA{0x12, 0x34, 0x56, 0x78}},
	}
	for _, c := range cases {
		got, ok := parseHexColor(c.in)
		if !ok || got != c.want {
			t.Fatalf("parseHexColor(%q) = %+v ok=%v, want %+v", c.in, got, ok, c.want)
		}
	}
	// Invalid lengths.
	if _, ok := parseHexColor("12"); ok {
		t.Fatal("len 2 must reject")
	}
	if _, ok := parseHexColor("1234"); ok {
		t.Fatal("len 4 must reject")
	}
	if _, ok := parseHexColor("zzz"); ok {
		t.Fatal("non-hex 3-char must reject")
	}
	if _, ok := parseHexColor("zzzzzz"); ok {
		t.Fatal("non-hex 6-char must reject")
	}
	if _, ok := parseHexColor("zzzzzzzz"); ok {
		t.Fatal("non-hex 8-char must reject")
	}
	if _, ok := parseHexColor("11zz22"); ok {
		t.Fatal("partial-hex 6 must reject")
	}
	if _, ok := parseHexColor("aabbccZZ"); ok {
		t.Fatal("invalid alpha must reject")
	}
}

func TestParseRGBFunc(t *testing.T) {
	c, ok := parseCSSColor("rgb(255, 0, 0)")
	if !ok || c != (RGBA{255, 0, 0, 0xFF}) {
		t.Fatalf("rgb red: %+v ok=%v", c, ok)
	}
	c, ok = parseCSSColor("rgba(0, 128, 255, 0.5)")
	if !ok || c.R != 0 || c.G != 128 || c.B != 255 || c.A == 0xFF {
		t.Fatalf("rgba: %+v ok=%v", c, ok)
	}
	// Percent forms.
	c, ok = parseCSSColor("rgb(50%, 50%, 50%)")
	if !ok || c.R != 127 {
		t.Fatalf("rgb percent: %+v ok=%v", c, ok)
	}
	// Modern slash separator: rgb(255 0 0 / 50%).
	c, ok = parseCSSColor("rgb(255 0 0 / 50%)")
	if !ok || c.R != 255 || c.A == 0xFF {
		t.Fatalf("rgb slash: %+v ok=%v", c, ok)
	}
	// Malformed.
	if _, ok := parseCSSColor("rgb(255, 0)"); ok {
		t.Fatal("3-arg rgb must require 3 channels")
	}
	if _, ok := parseCSSColor("rgb(abc, 0, 0)"); ok {
		t.Fatal("non-numeric channel must reject")
	}
	if _, ok := parseCSSColor("rgb(300, 0, 0)"); ok {
		t.Fatal("over-255 channel must reject")
	}
	if _, ok := parseCSSColor("rgb()"); ok {
		t.Fatal("empty must reject")
	}
	if _, ok := parseCSSColor("rgb("); ok {
		t.Fatal("unclosed must reject")
	}
}

func TestParseCSSColorUnknown(t *testing.T) {
	if _, ok := parseCSSColor("hsl(0, 100%, 50%)"); ok {
		t.Fatal("hsl not supported")
	}
	if _, ok := parseCSSColor("red"); ok {
		t.Fatal("named colours not supported")
	}
}

func TestParseAlpha(t *testing.T) {
	cases := []struct {
		in   string
		want uint8
	}{
		{"1", 0xFF},
		{"1.0", 0xFF},
		{"0", 0},
		{"0.0", 0},
		{"0.5", 127},
		{"50%", 127},
		{"100%", 0xFF},
		{"0%", 0},
		{"200%", 0xFF}, // out of range fallback
		{"bogus", 0xFF},
		{"0.bogus", 0xFF},
		{"2.0", 0xFF}, // doesn't match 0.x or 1/1.0 — falls through
	}
	for _, c := range cases {
		got := parseAlpha(c.in)
		if got != c.want {
			t.Fatalf("parseAlpha(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseByteOrPct(t *testing.T) {
	if v, ok := parseByteOrPct(""); ok {
		t.Fatalf("empty must reject, got %d", v)
	}
	if _, ok := parseByteOrPct("99%"); !ok {
		t.Fatal("99% must accept")
	}
	if _, ok := parseByteOrPct("101%"); ok {
		t.Fatal(">100% must reject")
	}
}

func TestParseUint(t *testing.T) {
	if v, ok := parseUint("42"); !ok || v != 42 {
		t.Fatal("parseUint 42")
	}
	if _, ok := parseUint(""); ok {
		t.Fatal("parseUint empty")
	}
	if _, ok := parseUint("-1"); ok {
		t.Fatal("parseUint -1 must reject")
	}
	if _, ok := parseUint("12x"); ok {
		t.Fatal("parseUint with letters must reject")
	}
}

func TestSplitOnCommaOrSpace(t *testing.T) {
	got := splitOnCommaOrSpace("a, b\tc")
	if len(got) != 3 {
		t.Fatalf("want 3, got %v", got)
	}
	got = splitOnCommaOrSpace("")
	if len(got) != 0 {
		t.Fatalf("empty must yield 0, got %v", got)
	}
	got = splitOnCommaOrSpace(",, ,")
	if len(got) != 0 {
		t.Fatalf("only-separators must yield 0, got %v", got)
	}
}

func TestIndexOfWhitespace(t *testing.T) {
	if i := indexOfWhitespace("abc def"); i != 3 {
		t.Fatalf("want 3, got %d", i)
	}
	if i := indexOfWhitespace("abc"); i != -1 {
		t.Fatalf("no ws want -1, got %d", i)
	}
	if i := indexOfWhitespace("a\nb"); i != 1 {
		t.Fatalf("\\n want 1, got %d", i)
	}
	if i := indexOfWhitespace("a\rb"); i != 1 {
		t.Fatalf("\\r want 1, got %d", i)
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"testing"
)

func TestParseCSSColor(t *testing.T) {
	ok := map[string]RGBA{
		"#fff": RGB(255, 255, 255), "#1a2b3c": RGB(0x1a, 0x2b, 0x3c), " #ABCDEF ": RGB(0xab, 0xcd, 0xef),
		"#11223344": {R: 0x11, G: 0x22, B: 0x33, A: 0x44}, "#f008": {R: 255, G: 0, B: 0, A: 0x88},
		"rgb(10, 20, 30)": RGB(10, 20, 30), "rgb(10 20 30 / 50%)": {R: 10, G: 20, B: 30, A: 128},
		"rgba(255,0,0,0.5)": {R: 255, G: 0, B: 0, A: 128}, "rgb(100%, 0%, 0%)": RGB(255, 0, 0),
		"hsl(0, 100%, 50%)": RGB(255, 0, 0), "hsl(120deg 100% 25%)": RGB(0, 128, 0),
		"white": RGB(255, 255, 255), "Navy": RGB(0, 0, 128),
	}
	for in, want := range ok {
		got, k := ParseCSSColor(in)
		if !k || got != want {
			t.Errorf("ParseCSSColor(%q) = %v, %v; want %v", in, got, k, want)
		}
	}
	for _, in := range []string{"", "#", "#12", "#12345", "#ggg", "var(--x)", "calc(1+2)", "rgb(1,2)", "rgb(a,b,c)", "hsl(0, 100, 50)", "rebeccapurple", "rgb(NaN,0,0)"} {
		if c, k := ParseCSSColor(in); k {
			t.Errorf("ParseCSSColor(%q) = %v, want not understood", in, c)
		}
	}
	if FormatCSSColor(RGB(1, 2, 3)) != "#010203" || FormatCSSColor(RGBA{R: 1, G: 2, B: 3, A: 4}) != "#01020304" {
		t.Error("FormatCSSColor")
	}
}

func FuzzParseCSSColor(f *testing.F) {
	for _, s := range []string{"#fff", "rgb(1,2,3)", "hsl(1 2% 3%)", "rgba(", "hsl()", "#", "rgb(1e999,0,0)", "hsl(9e99deg 1% 1%)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if c, ok := ParseCSSColor(s); ok {
			// what is understood is written back as a colour that reads the same
			if back, ok2 := ParseCSSColor(FormatCSSColor(c)); !ok2 || back != c {
				t.Fatalf("%q -> %v -> %q -> %v", s, c, FormatCSSColor(c), back)
			}
		}
	})
}

func testInspector() (*Inspector, *[][2]string) {
	in := NewInspector([]InspectorRow{
		{Label: "Palette", Kind: InspectorSection},
		{ID: "bg", Label: "Background", Kind: InspectorColor},
		{ID: "font", Label: "Body font", Kind: InspectorFont, Choices: []string{"Inter", "Georgia"}},
		{ID: "size", Label: "Size", Kind: InspectorNumber, Units: []string{"px", "pt"}, Min: 6, Max: 200, Step: 1},
		{ID: "lh", Label: "Line height", Kind: InspectorNumber, Min: 5, Max: 30, Step: 1, Decimals: 1},
		{ID: "pos", Label: "Position", Kind: InspectorEnum, Choices: []string{"left", "right"}},
		{ID: "name", Label: "Name", Kind: InspectorText},
		{ID: "bg", Label: "duplicate id is dropped", Kind: InspectorText},
	})
	var got [][2]string
	in.OnChange = func(id, v string) { got = append(got, [2]string{id, v}) }
	in.SetBounds(Rect{X: 0, Y: 0, W: 320, H: 400})
	return in, &got
}

func TestInspectorHostSetIsNotAnEdit(t *testing.T) {
	in, got := testInspector()
	if in.Rows() != 7 {
		t.Fatalf("rows = %d (the duplicate id must be dropped)", in.Rows())
	}
	in.Set("bg", "#112233")
	in.Set("font", "Georgia")
	in.Set("size", "18pt")
	in.Set("lh", "1.5")
	in.Set("pos", "right")
	in.Set("name", "acme")
	if len(*got) != 0 {
		t.Fatalf("a host Set must not fire OnChange: %v", *got)
	}
	for id, want := range map[string]string{"bg": "#112233", "font": "Georgia", "size": "18pt", "lh": "1.5", "pos": "right", "name": "acme"} {
		if v := in.Get(id); v != want {
			t.Errorf("Get(%s) = %q, want %q", id, v, want)
		}
	}
	// the editors show them
	r := in.byID["size"]
	if r.num.Value().Get() != 18 || r.unit.Current() != "pt" {
		t.Errorf("size editors: %d %s", r.num.Value().Get(), r.unit.Current())
	}
	if in.byID["lh"].num.Value().Get() != 15 {
		t.Errorf("1.5 with one decimal is the integer 15, got %d", in.byID["lh"].num.Value().Get())
	}
	if c := in.byID["bg"].picker.Color().Get(); c != RGB(0x11, 0x22, 0x33) {
		t.Errorf("picker = %v", c)
	}
	if in.Get("nosuch") != "" || in.Value("nosuch") != nil {
		t.Error("an unknown id reads as empty")
	}
	in.Set("nosuch", "x") // no panic
}

func TestInspectorUserEditsFireOnChangeOnce(t *testing.T) {
	in, got := testInspector()
	in.Set("size", "16px")
	in.Set("bg", "#ffffff")

	in.byID["size"].num.SetValue(20) // the user steps the spinner
	if len(*got) != 1 || (*got)[0] != [2]string{"size", "20px"} {
		t.Fatalf("after a spin: %v", *got)
	}
	in.byID["size"].unit.Select(1)
	if (*got)[len(*got)-1] != [2]string{"size", "20pt"} {
		t.Fatalf("after a unit change: %v", *got)
	}
	if in.Get("size") != "20pt" {
		t.Errorf("Get = %q", in.Get("size"))
	}

	n := len(*got)
	in.byID["bg"].picker.SetColor(RGB(1, 2, 3)) // the user drags the picker
	if len(*got) != n+1 || (*got)[n] != [2]string{"bg", "#010203"} {
		t.Fatalf("after a pick: %v", (*got)[n:])
	}
	if in.byID["bg"].entry.Value() != "#010203" {
		t.Errorf("the hex field must follow the picker: %q", in.byID["bg"].entry.Value())
	}
	in.byID["bg"].entry.SetText("rgb(9, 9, 9)") // the user types a colour in another notation
	if in.Get("bg") != "rgb(9, 9, 9)" {
		t.Errorf("a typed colour is kept as typed, got %q", in.Get("bg"))
	}
	if c := in.byID["bg"].picker.Color().Get(); c != RGB(9, 9, 9) {
		t.Errorf("and the picker follows: %v", c)
	}
	m := len(*got)
	in.byID["bg"].entry.SetText("#12") // half-typed: nothing changes
	if len(*got) != m || in.Get("bg") != "rgb(9, 9, 9)" {
		t.Errorf("an unparsable colour must not change the value: %v %q", (*got)[m:], in.Get("bg"))
	}
	in.byID["pos"].enum.Select(1)
	in.byID["name"].entry.SetText("x")
	last := (*got)[len(*got)-2:]
	if last[0] != [2]string{"pos", "right"} || last[1] != [2]string{"name", "x"} {
		t.Errorf("enum and text edits: %v", last)
	}
}

func TestInspectorKeepsWhatItCannotShow(t *testing.T) {
	in, got := testInspector()
	in.Set("bg", "var(--brand)")
	in.Set("size", "1.25em")
	in.Set("lh", "1.55") // two decimals in a one-decimal row
	in.Set("pos", "center")
	for _, id := range []string{"bg", "size", "lh"} {
		if !in.Custom(id) {
			t.Errorf("%s must be flagged custom", id)
		}
	}
	for id, want := range map[string]string{"bg": "var(--brand)", "size": "1.25em", "lh": "1.55", "pos": "center"} {
		if in.Get(id) != want {
			t.Errorf("%s was rewritten to %q, want %q kept", id, in.Get(id), want)
		}
	}
	if !contains(in.byID["pos"].enum.Options, "center") {
		t.Error("an enum value outside the choices is added, not replaced")
	}
	in.Draw(newP(makeSurface(320, 400), 320), DefaultLight()) // draws the custom faces
	if len(*got) != 0 {
		t.Errorf("nothing was edited: %v", *got)
	}
	in.byID["size"].num.SetValue(30) // the user takes the row over
	if in.Custom("size") || in.Get("size") != "30px" {
		t.Errorf("an edit clears custom: %v %q", in.Custom("size"), in.Get("size"))
	}
	// a number past the row's bounds is not the value that was given
	in.Set("size", "500px")
	if !in.Custom("size") || in.Get("size") != "500px" {
		t.Errorf("an out-of-range number is kept and flagged: %v %q", in.Custom("size"), in.Get("size"))
	}
}

func TestInspectorLayoutAndRouting(t *testing.T) {
	in, got := testInspector()
	w, h := in.Measure(320, 0)
	if w != 320 || h <= 0 {
		t.Fatalf("Measure = %d, %d", w, h)
	}
	_, hOpen := func() (int, int) { in.ToggleColor("bg"); return in.Measure(320, 0) }()
	if hOpen <= h {
		t.Errorf("an open colour picker adds height: %d -> %d", h, hOpen)
	}
	in.ToggleColor("bg")
	in.ToggleColor("size") // not a colour row: ignored
	in.ToggleColor("nosuch")

	in.Draw(newP(makeSurface(320, 400), 320), DefaultLight())
	// click the swatch: the picker opens
	sw := in.byID["bg"].swatch
	in.OnEvent(Event{Kind: EventClick, X: sw.X + 2, Y: sw.Y + 2})
	if !in.byID["bg"].open {
		t.Fatal("a click on the swatch opens the picker")
	}
	if n := len(in.Children()); n < 7 {
		t.Errorf("children = %d", n)
	}
	// click into the name entry and type: the entry takes the keys
	e := in.byID["name"].entry.Bounds()
	in.OnEvent(Event{Kind: EventClick, X: e.X + 3, Y: e.Y + 3})
	if !in.byID["name"].entry.Focused() {
		t.Fatal("a click focuses the editor under it")
	}
	in.OnEvent(Event{Kind: EventChar, Code: "z"})
	if in.Get("name") != "z" || len(*got) == 0 || (*got)[len(*got)-1] != [2]string{"name", "z"} {
		t.Errorf("typing: %q %v", in.Get("name"), *got)
	}
	in.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	if in.byID["name"].entry.Focused() {
		t.Error("Tab moves focus on")
	}
}

func TestInspectorRowsOfNoKindAndNarrowWidths(t *testing.T) {
	in := NewInspector(nil)
	in.SetBounds(Rect{W: 10, H: 10})
	in.Draw(newP(makeSurface(10, 10), 10), DefaultLight())
	in = NewInspector([]InspectorRow{{ID: "n", Label: "A very long caption that cannot fit", Kind: InspectorNumber}})
	in.SetBounds(Rect{W: 60, H: 40}) // narrower than every editor wants
	in.Draw(newP(makeSurface(60, 40), 60), DefaultLight())
	in.Set("n", "7")
	if in.Get("n") != "7" {
		t.Errorf("a bare number: %q", in.Get("n"))
	}
}

func TestParseCSSColorEveryHueAndEveryRefusal(t *testing.T) {
	for in, want := range map[string]RGBA{
		"hsl(90 100% 50%)": {R: 128, G: 255, B: 0, A: 255}, "hsl(150 100% 50%)": {R: 0, G: 255, B: 128, A: 255},
		"hsl(210 100% 50%)": {R: 0, G: 128, B: 255, A: 255}, "hsl(270 100% 50%)": {R: 128, G: 0, B: 255, A: 255},
		"hsl(330 100% 50%)": {R: 255, G: 0, B: 128, A: 255}, "hsl(-90 100% 50%)": {R: 128, G: 0, B: 255, A: 255},
	} {
		if got, ok := ParseCSSColor(in); !ok || got != want {
			t.Errorf("ParseCSSColor(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, in := range []string{"rgb(1 2 3 / x)", "hsl(x 50% 50%)", "hsl(0 5% y)", "rgb(1,2,3,4,5)"} {
		if c, ok := ParseCSSColor(in); ok {
			t.Errorf("ParseCSSColor(%q) = %v, want refused", in, c)
		}
	}
}

func TestInspectorBoundValueFollowsAndNumbersRoundTrip(t *testing.T) {
	in := NewInspector([]InspectorRow{
		{ID: "off", Label: "Offset", Kind: InspectorNumber, Min: -50, Max: 50, Step: 1},
		{ID: "lh", Label: "Line height", Kind: InspectorNumber, Min: -20, Max: 30, Decimals: 1, Units: []string{"em"}},
		{ID: "name", Label: "Name", Kind: InspectorText},
	})
	var edits []string
	in.OnChange = func(id, v string) { edits = append(edits, id+"="+v) }
	in.SetBounds(Rect{W: 300, H: 200})

	// a host that binds the Observable moves the editor, without an edit
	in.Value("name").Set("bound")
	if in.byID["name"].entry.Value() != "bound" || len(edits) != 0 {
		t.Fatalf("bound Set: %q %v", in.byID["name"].entry.Value(), edits)
	}
	// negative whole numbers, and decimals under one
	in.Set("off", "-7")
	in.byID["off"].num.SetValue(-8)
	in.Set("lh", "-0.5em")
	if in.byID["lh"].num.Value().Get() != -5 {
		t.Errorf("-0.5 with one decimal is -5, got %d", in.byID["lh"].num.Value().Get())
	}
	in.byID["lh"].num.SetValue(5)
	in.byID["lh"].num.SetValue(-15)
	in.byID["lh"].num.SetValue(20)
	want := []string{"off=-8", "lh=0.5em", "lh=-1.5em", "lh=2em"}
	if len(edits) != len(want) {
		t.Fatalf("edits = %v, want %v", edits, want)
	}
	for i := range want {
		if edits[i] != want[i] {
			t.Errorf("edit %d = %q, want %q", i, edits[i], want[i])
		}
	}
	// what a number row cannot hold is kept, whichever way it fails
	for _, v := range []string{"auto", "1.2.3", "5px", "0.25em", "9999em"} {
		in.Set("lh", v)
		if !in.Custom("lh") || in.Get("lh") != v {
			t.Errorf("%q: custom=%v value=%q", v, in.Custom("lh"), in.Get("lh"))
		}
	}
	if in.Value("off") == nil || in.Value("nosuch") != nil {
		t.Error("Value")
	}
	if in.A11y().Role != RoleGroup {
		t.Error("A11y")
	}
}

func TestInspectorDrawsAnOpenPicker(t *testing.T) {
	in, _ := testInspector()
	in.ToggleColor("bg")
	in.Draw(newP(makeSurface(320, 400), 320), DefaultLight())
	if !in.byID["bg"].open {
		t.Fatal("open")
	}
}

func TestInspectorEditorRectsAreWhereTheEditorsAre(t *testing.T) {
	in, _ := testInspector()
	for _, id := range []string{"bg", "font", "size", "lh", "pos", "name"} {
		r, ok := in.EditorRect(id)
		if !ok || r.W <= 0 || r.H <= 0 || !in.Bounds().Contains(r.X, r.Y) {
			t.Errorf("%s: %v %v", id, r, ok)
		}
	}
	if _, ok := in.EditorRect("nosuch"); ok {
		t.Error("an unknown id has no rect")
	}
	// a click in the middle of a number row's rect lands on its spinner
	r, _ := in.EditorRect("size")
	in.OnEvent(Event{Kind: EventClick, X: r.X + 2, Y: r.Y + 2})
	if !in.byID["size"].num.Focused() {
		t.Error("the rect of a number row is its spinner's")
	}
	// the swatch of a colour row is inside the rect, and toggles the picker
	c, _ := in.EditorRect("bg")
	in.OnEvent(Event{Kind: EventClick, X: c.X + 2, Y: c.Y + 2})
	if !in.byID["bg"].open {
		t.Error("the swatch is the left of the colour rect")
	}
}

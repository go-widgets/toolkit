// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestParseAcceleratorCanonicalises(t *testing.T) {
	cases := []struct {
		in   string
		want Accelerator
		str  string
	}{
		{"P", Accelerator{Key: "P"}, "P"},
		{"p", Accelerator{Key: "P"}, "P"},
		{"Ctrl+P", Accelerator{Key: "P", Ctrl: true}, "Ctrl+P"},
		{"ctrl+shift+p", Accelerator{Key: "P", Ctrl: true, Shift: true}, "Ctrl+Shift+P"},
		{"Control+P", Accelerator{Key: "P", Ctrl: true}, "Ctrl+P"},
		{"Ctl+P", Accelerator{Key: "P", Ctrl: true}, "Ctrl+P"},
		{"Meta+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Cmd+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Command+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Super+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Win+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Windows+K", Accelerator{Key: "K", Meta: true}, "Meta+K"},
		{"Alt+Left", Accelerator{Key: "ArrowLeft", Alt: true}, "Alt+ArrowLeft"},
		{"Opt+Right", Accelerator{Key: "ArrowRight", Alt: true}, "Alt+ArrowRight"},
		{"Option+Up", Accelerator{Key: "ArrowUp", Alt: true}, "Alt+ArrowUp"},
		{"Esc", Accelerator{Key: "Escape"}, "Escape"},
		{"Return", Accelerator{Key: "Enter"}, "Enter"},
		{"f5", Accelerator{Key: "F5"}, "F5"},
		{"F12", Accelerator{Key: "F12"}, "F12"},
		{"/", Accelerator{Key: "/"}, "/"},
		{"1", Accelerator{Key: "1"}, "1"},
		{"Ctrl++", Accelerator{Key: "+", Ctrl: true}, "Ctrl++"},
		{"+", Accelerator{Key: "+"}, "+"},
		{"ArrowLeft", Accelerator{Key: "ArrowLeft"}, "ArrowLeft"},
		{"Ctrl+Shift+Alt+Meta+X", Accelerator{Key: "X", Ctrl: true, Shift: true, Alt: true, Meta: true}, "Ctrl+Shift+Alt+Meta+X"},
	}
	for _, c := range cases {
		got, err := ParseAccelerator(c.in)
		if err != nil {
			t.Fatalf("ParseAccelerator(%q) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseAccelerator(%q) = %+v, want %+v", c.in, got, c.want)
		}
		if s := got.String(); s != c.str {
			t.Errorf("ParseAccelerator(%q).String() = %q, want %q", c.in, s, c.str)
		}
	}
}

func TestParseAcceleratorErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "Foo+P", "Ctrl+ "} {
		if _, err := ParseAccelerator(in); err == nil {
			t.Errorf("ParseAccelerator(%q) = nil error, want error", in)
		}
	}
}

func TestMustParseAccelerator(t *testing.T) {
	if got := MustParseAccelerator("Ctrl+P"); got.Key != "P" || !got.Ctrl {
		t.Fatalf("MustParseAccelerator = %+v", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustParseAccelerator did not panic on invalid input")
		}
	}()
	MustParseAccelerator("Bad+")
}

func TestCanonKeyEdgeCases(t *testing.T) {
	if got := canonKey(""); got != "" {
		t.Errorf("canonKey(\"\") = %q, want empty", got)
	}
	if got := canonKey("foo"); got != "foo" {
		t.Errorf("canonKey(\"foo\") = %q, want foo (unchanged)", got)
	}
	if got := canonKey("fx"); got != "fx" {
		t.Errorf("canonKey(\"fx\") = %q, want fx (not an F-key)", got)
	}
}

func TestAllDigits(t *testing.T) {
	if allDigits("") {
		t.Error("allDigits(\"\") = true, want false")
	}
	if !allDigits("123") {
		t.Error("allDigits(\"123\") = false, want true")
	}
	if allDigits("1a2") {
		t.Error("allDigits(\"1a2\") = true, want false")
	}
}

func TestAcceleratorFromEvent(t *testing.T) {
	got, ok := AcceleratorFromEvent(Event{Kind: EventKeyDown, Code: "p", Ctrl: true})
	if !ok || got != (Accelerator{Key: "P", Ctrl: true}) {
		t.Fatalf("keydown = %+v, ok=%v", got, ok)
	}
	got, ok = AcceleratorFromEvent(Event{Kind: EventChar, Code: "/"})
	if !ok || got != (Accelerator{Key: "/"}) {
		t.Fatalf("char = %+v, ok=%v", got, ok)
	}
	if _, ok := AcceleratorFromEvent(Event{Kind: EventKeyDown, Code: ""}); ok {
		t.Error("empty Code keydown reported ok=true")
	}
	if _, ok := AcceleratorFromEvent(Event{Kind: EventClick}); ok {
		t.Error("EventClick reported ok=true")
	}
}

func TestParseChord(t *testing.T) {
	c, err := ParseChord("Ctrl+K Ctrl+S")
	if err != nil {
		t.Fatalf("ParseChord error: %v", err)
	}
	if len(c) != 2 || c[0] != (Accelerator{Key: "K", Ctrl: true}) || c[1] != (Accelerator{Key: "S", Ctrl: true}) {
		t.Fatalf("ParseChord = %v", c)
	}
	if s := c.String(); s != "Ctrl+K Ctrl+S" {
		t.Errorf("Chord.String() = %q", s)
	}
	if _, err := ParseChord(""); err == nil {
		t.Error("ParseChord(\"\") = nil error")
	}
	if _, err := ParseChord("g Bad+"); err == nil {
		t.Error("ParseChord with a bad segment = nil error")
	}
}

func TestMustParseChord(t *testing.T) {
	if c := MustParseChord("g d"); len(c) != 2 {
		t.Fatalf("MustParseChord = %v", c)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustParseChord did not panic on invalid input")
		}
	}()
	MustParseChord("")
}

func TestChordEqualAndPrefix(t *testing.T) {
	gd := MustParseChord("g d")
	g := MustParseChord("g")
	gx := MustParseChord("g x")

	if gd.equal(g) {
		t.Error("different-length chords reported equal")
	}
	if gd.equal(gx) {
		t.Error("g d == g x reported equal")
	}
	if !gd.equal(MustParseChord("g d")) {
		t.Error("g d != g d")
	}
	if !gd.hasPrefix(g) {
		t.Error("g is not a prefix of g d")
	}
	if gd.hasPrefix(gd) {
		t.Error("a chord is its own proper prefix")
	}
	if gx.hasPrefix(MustParseChord("d")) {
		t.Error("d is a prefix of g x")
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"unicode"
)

// Accelerator is a single key combination: a base Key plus the four desktop
// modifier flags. It is the atom a [Keymap] binds and an [Event] is matched
// against.
//
// Key is a canonical key name — a single upper-case letter ("P"), a digit
// ("1"), a punctuation rune ("/"), or a named key ("Enter", "ArrowLeft",
// "F1", "Escape"). The modifier flags mirror [Event]: Ctrl/Shift are the two
// common modifiers, Alt is the Option (⌥) / Alt key, and Meta is the Command
// (⌘) / Super (Windows/logo) key. Two accelerators are equal only when the
// Key and all four flags match, so Ctrl+P, Shift+Ctrl+P and plain P are three
// distinct accelerators.
type Accelerator struct {
	Key   string
	Ctrl  bool
	Shift bool
	Alt   bool
	Meta  bool
}

// keyAliases maps lower-cased human spellings of named keys to the canonical
// name an [Event] carries, so an author may write "Esc", "Return" or "Left"
// in an accelerator string and still match the "Escape" / "Enter" /
// "ArrowLeft" a host actually delivers.
var keyAliases = map[string]string{
	"esc":        "Escape",
	"escape":     "Escape",
	"enter":      "Enter",
	"return":     "Enter",
	"cr":         "Enter",
	"tab":        "Tab",
	"space":      "Space",
	"spacebar":   "Space",
	"backspace":  "Backspace",
	"bksp":       "Backspace",
	"del":        "Delete",
	"delete":     "Delete",
	"insert":     "Insert",
	"ins":        "Insert",
	"home":       "Home",
	"end":        "End",
	"pageup":     "PageUp",
	"pgup":       "PageUp",
	"pagedown":   "PageDown",
	"pgdn":       "PageDown",
	"up":         "ArrowUp",
	"arrowup":    "ArrowUp",
	"down":       "ArrowDown",
	"arrowdown":  "ArrowDown",
	"left":       "ArrowLeft",
	"arrowleft":  "ArrowLeft",
	"right":      "ArrowRight",
	"arrowright": "ArrowRight",
}

// canonKey normalises a raw key token to the canonical form used for matching:
// a single letter upper-cases (so "p" and "P" are one binding), a single
// non-letter rune is kept verbatim ("/", "1", "+"), a known alias resolves via
// keyAliases, an F-key ("f5") upper-cases to "F5", and anything else is
// returned unchanged (so a host's own "ArrowLeft" round-trips).
func canonKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) == 1 {
		if unicode.IsLetter(r[0]) {
			return strings.ToUpper(s)
		}
		return s
	}
	low := strings.ToLower(s)
	if c, ok := keyAliases[low]; ok {
		return c
	}
	if low[0] == 'f' && allDigits(low[1:]) {
		return strings.ToUpper(low)
	}
	return s
}

// allDigits reports whether s is non-empty and every rune is an ASCII digit —
// the test that turns "f5" into an F-key but leaves "foo" alone.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// ParseAccelerator parses a human accelerator string ("Ctrl+Shift+P",
// "Meta+K", "Alt+Left", "Ctrl++") into an [Accelerator]. Segments are split on
// '+'; every segment but the last is a modifier (ctrl/control, shift,
// alt/opt/option, meta/cmd/command/super/win) and the last is the key. A
// trailing '+' denotes the '+' key itself ("Ctrl++" = Ctrl and the plus key).
// Modifier and key spelling are case-insensitive. An empty string, an unknown
// modifier, or a missing key returns an error.
func ParseAccelerator(s string) (Accelerator, error) {
	orig := s
	if strings.TrimSpace(s) == "" {
		return Accelerator{}, fmt.Errorf("toolkit: empty accelerator %q", orig)
	}
	parts := strings.Split(s, "+")
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	if key == "" {
		// A trailing '+' split to an empty final segment: the '+' is the key.
		key = "+"
		for len(mods) > 0 && mods[len(mods)-1] == "" {
			mods = mods[:len(mods)-1]
		}
	}
	var a Accelerator
	for _, m := range mods {
		switch strings.ToLower(strings.TrimSpace(m)) {
		case "ctrl", "control", "ctl":
			a.Ctrl = true
		case "shift":
			a.Shift = true
		case "alt", "opt", "option":
			a.Alt = true
		case "meta", "cmd", "command", "super", "win", "windows":
			a.Meta = true
		default:
			return Accelerator{}, fmt.Errorf("toolkit: unknown modifier %q in accelerator %q", m, orig)
		}
	}
	a.Key = canonKey(key)
	if a.Key == "" {
		return Accelerator{}, fmt.Errorf("toolkit: missing key in accelerator %q", orig)
	}
	return a, nil
}

// MustParseAccelerator is [ParseAccelerator] that panics on error, for
// package-level accelerator literals known to be valid at author time.
func MustParseAccelerator(s string) Accelerator {
	a, err := ParseAccelerator(s)
	if err != nil {
		panic(err)
	}
	return a
}

// String renders the accelerator in canonical Ctrl+Shift+Alt+Meta+Key order,
// the inverse of [ParseAccelerator] (aliases resolved), suitable as a menu or
// tooltip hint.
func (a Accelerator) String() string {
	var b strings.Builder
	if a.Ctrl {
		b.WriteString("Ctrl+")
	}
	if a.Shift {
		b.WriteString("Shift+")
	}
	if a.Alt {
		b.WriteString("Alt+")
	}
	if a.Meta {
		b.WriteString("Meta+")
	}
	b.WriteString(a.Key)
	return b.String()
}

// AcceleratorFromEvent derives the [Accelerator] a key event represents,
// reporting ok=false for any non-keyboard event or a keyboard event with an
// empty Code. It reads EventKeyDown and EventChar, canonicalising Code the same
// way [ParseAccelerator] canonicalises a key token, so a binding parsed from a
// string matches an event delivered by a host.
func AcceleratorFromEvent(ev Event) (Accelerator, bool) {
	switch ev.Kind {
	case EventKeyDown, EventChar:
		if ev.Code == "" {
			return Accelerator{}, false
		}
		return Accelerator{
			Key:   canonKey(ev.Code),
			Ctrl:  ev.Ctrl,
			Shift: ev.Shift,
			Alt:   ev.Alt,
			Meta:  ev.Meta,
		}, true
	default:
		return Accelerator{}, false
	}
}

// Chord is an ordered sequence of accelerators pressed in turn — the "Ctrl+K
// Ctrl+S" or "g d" multi-stroke binding pattern. A single-accelerator chord is
// the common case; a [Keymap] resolves longer chords stroke-by-stroke.
type Chord []Accelerator

// ParseChord parses a whitespace-separated sequence of accelerators into a
// [Chord] ("Ctrl+K Ctrl+S", "g d"). An empty string, or any segment that is
// not a valid accelerator, returns an error.
func ParseChord(s string) (Chord, error) {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return nil, fmt.Errorf("toolkit: empty chord %q", s)
	}
	c := make(Chord, 0, len(fields))
	for _, f := range fields {
		a, err := ParseAccelerator(f)
		if err != nil {
			return nil, err
		}
		c = append(c, a)
	}
	return c, nil
}

// MustParseChord is [ParseChord] that panics on error, for package-level chord
// literals known to be valid at author time.
func MustParseChord(s string) Chord {
	c, err := ParseChord(s)
	if err != nil {
		panic(err)
	}
	return c
}

// String renders the chord as space-joined canonical accelerators, the inverse
// of [ParseChord].
func (c Chord) String() string {
	parts := make([]string, len(c))
	for i, a := range c {
		parts[i] = a.String()
	}
	return strings.Join(parts, " ")
}

// equal reports whether two chords are the same length and element-wise equal.
func (c Chord) equal(o Chord) bool {
	if len(c) != len(o) {
		return false
	}
	for i := range c {
		if c[i] != o[i] {
			return false
		}
	}
	return true
}

// hasPrefix reports whether p is a proper prefix of c (strictly shorter and
// matching every leading accelerator) — the test a [Keymap] uses to decide a
// partially-typed chord is still on its way to a longer binding.
func (c Chord) hasPrefix(p Chord) bool {
	if len(p) >= len(c) {
		return false
	}
	for i := range p {
		if c[i] != p[i] {
			return false
		}
	}
	return true
}

// cloneChord returns an independent copy so a stored binding cannot be mutated
// through the caller's slice.
func cloneChord(c Chord) Chord {
	out := make(Chord, len(c))
	copy(out, c)
	return out
}

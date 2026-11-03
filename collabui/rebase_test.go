// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-crdt/crdt"
)

// apply is what a view does with a change: remove Removed runes at Pos, put Text
// there. It is the test's own copy of the rule [crdt.Change] states, so a failure
// here is about replay rather than about anybody's interpretation of it, and it
// reports an offset out of range rather than panicking — an out-of-range offset
// is a defect in replay and the test should say which one.
func apply(t *testing.T, text string, ch crdt.Change) string {
	t.Helper()
	r := []rune(text)
	if ch.Pos < 0 || ch.Removed < 0 || ch.Pos+ch.Removed > len(r) {
		t.Fatalf("change %+v is out of range of %q (%d runes)", ch, text, len(r))
	}
	return string(r[:ch.Pos]) + ch.Text + string(r[ch.Pos+ch.Removed:])
}

func TestRebaseMovesAnOffsetOnlyWhenTheChangeIsBeforeIt(t *testing.T) {
	// "abcd" with "XYZ" put in place of "bc", giving "aXYZd". Offsets 0 and 1 are
	// ahead of the change and do not move; 2 sat between the two characters that
	// went and goes the same one step into what replaced them; 3 and 4 are past it
	// and shift by the three put in less the two taken out. An offset is "past"
	// the change as soon as it is at its far end — 3 is before the 'd' both before
	// and after.
	through := crdt.Change{Pos: 1, Removed: 2, Text: "XYZ"}
	for pos, want := range map[int]int{0: 0, 1: 1, 2: 2, 3: 4, 4: 5} {
		if got := rebase(pos, through); got != want {
			t.Errorf("rebase(%d, %+v) = %d, want %d", pos, through, got, want)
		}
	}
	// And it goes no further in than there is to go: "abcde" with "X" in place of
	// "bcd" leaves one character to be one step into, so 2, 3 and 4 all arrive at
	// the far side of it rather than past it.
	short := crdt.Change{Pos: 1, Removed: 3, Text: "X"}
	for pos, want := range map[int]int{1: 1, 2: 2, 3: 2, 4: 2, 5: 3} {
		if got := rebase(pos, short); got != want {
			t.Errorf("rebase(%d, %+v) = %d, want %d", pos, short, got, want)
		}
	}
	// An offset AT the change keeps its place, so what the person typed goes in
	// front of what arrived.
	if got := rebase(2, crdt.Change{Pos: 2, Text: "Z"}); got != 2 {
		t.Errorf("rebase at the change's own position = %d, want 2 (ahead of it)", got)
	}
	// The zero change is the identity: the screen and the replica agree.
	for _, pos := range []int{0, 1, 7} {
		if got := rebase(pos, crdt.Change{}); got != pos {
			t.Errorf("rebase(%d, zero) = %d, want %d", pos, got, pos)
		}
	}
}

func TestReplayPutsAnEditWhereThePersonPutIt(t *testing.T) {
	for _, c := range []struct {
		what              string
		base, typed, held string
		want              string
	}{
		{"nothing pending, an insert goes in untouched", "abc", "abcY", "abc", "abcY"},
		{"nothing pending, a deletion goes in untouched", "abcd", "ad", "abcd", "ad"},
		{"typing after a pending insert moves along with it", "abc", "abcY", "Xabc", "XabcY"},
		{"typing before a pending insert does not move", "abc", "Zabc", "abcY", "ZabcY"},
		{"typing at a pending insert goes in front of it", "ab", "aZb", "aXb", "aZXb"},
		{"typing after a pending deletion moves back with it", "abcd", "abcdY", "ad", "adY"},
		{"a deletion keeps what was put inside it", "abcd", "ad", "abXcd", "aXd"},
		{"a deletion keeps what was put at its start", "abcd", "ad", "aXbcd", "aXd"},
		{"a deletion keeps what was put at its end", "abcd", "ad", "abcXd", "aXd"},
		{"a deletion spanning a whole pending insert keeps it", "abcd", "d", "aXbcd", "Xd"},
		{"a replacement puts its text where the deletion was", "abcd", "aZd", "abXcd", "aZXd"},
		{"what the person deleted was already gone", "abcd", "ad", "ad", "ad"},
		// Two remote edits at the ends of a line come back as one span covering
		// the line. Typing inside it must still land where it was typed, which is
		// what going the same distance into the replacement buys over collapsing
		// to the start of the span.
		{"typing inside a span two far-apart edits made", "hello world", "hello Xworld", "Hello worlD", "Hello XworlD"},
		{"typing inside a span that shrank clamps to its end", "abcde", "abXcde", "aYe", "aYXe"},
		{"an edit against a buffer that has not moved at all", "", "a", "", "a"},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := c.held
			for _, op := range replay(one(crdt.ChangesFrom(c.base, c.typed)), one(crdt.ChangesFrom(c.base, c.held))) {
				got = apply(t, got, op)
			}
			if got != c.want {
				t.Errorf("editing %q into %q while the replica held %q gives %q, want %q",
					c.base, c.typed, c.held, got, c.want)
			}
		})
	}
}

// one is the single contiguous change [crdt.ChangesFrom] reduces two texts to, or
// the zero change when they are the same.
func one(changes []crdt.Change) crdt.Change {
	if len(changes) == 0 {
		return crdt.Change{}
	}
	return changes[0]
}

// FuzzReplayKeepsWhatThePersonNeverSaw is the property the integration tests
// state for three texts, over any three: a local edit replayed into a replica
// that has moved on addresses offsets the replica actually has, puts the typed
// text in, and does not take out a single character that arrived from somewhere
// else — which is the half that is silent when it goes wrong.
func FuzzReplayKeepsWhatThePersonNeverSaw(f *testing.F) {
	f.Add("abc", "abcY", "Xabc")
	f.Add("abcd", "ad", "abXcd")
	f.Add("abc", "Zabc", "abcY")
	f.Add("", "a", "")
	f.Add("héllo wörld", "héllo", "héllo wörld, and more")
	f.Fuzz(func(t *testing.T, base, typed, held string) {
		// Runes, because every offset here is one. Invalid UTF-8 round-trips
		// through replacement characters and would make the lengths disagree.
		base, typed, held = valid(base), valid(typed), valid(held)
		pending := one(crdt.ChangesFrom(base, held))
		kept := pending.Text

		got := held
		for _, op := range replay(one(crdt.ChangesFrom(base, typed)), pending) {
			got = apply(t, got, op) // fails the test on an offset out of range
		}
		if local := one(crdt.ChangesFrom(base, typed)).Text; !strings.Contains(got, local) {
			t.Fatalf("what was typed (%q) is not in the result %q", local, got)
		}
		// ⛔ BY COUNT, NOT AS A SUBSTRING. What the person typed may land INSIDE
		// what the peer put there and split it — see replay — so the run is not
		// always contiguous in the result. Every character of it must still be
		// there, and that is the property worth asserting: placement is a guess,
		// and a character that was on somebody's screen a moment ago is not.
		if r, ok := lost(kept, got); !ok {
			t.Fatalf("the replica held %q, which the person never saw, and %q of it is gone from %q"+
				"\n\tbase  %q\n\ttyped %q\n\theld  %q", kept, string(r), got, base, typed, held)
		}
	})
}

// lost reports whether every rune of kept is still in got, as many times over,
// and names the first one that is not. Counting rather than searching for the run
// is what lets the typed text split it without the property becoming untrue.
func lost(kept, got string) (rune, bool) {
	left := map[rune]int{}
	for _, r := range got {
		left[r]++
	}
	for _, r := range kept {
		if left[r] == 0 {
			return r, false
		}
		left[r]--
	}
	return 0, true
}

// valid replaces anything that is not well-formed UTF-8, so a fuzzed string has
// as many runes as it has characters to address.
func valid(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "�")
}

// document is a replica holding n runes, the size a keystroke is measured over.
func document(b *testing.B, n int) *crdt.Doc {
	b.Helper()
	d := crdt.New(1)
	if _, err := d.Insert(0, strings.Repeat("the quick brown fox\n", n/20)); err != nil {
		b.Fatalf("Insert: %v", err)
	}
	return d
}

// What one keystroke costs the binding, with and without the read of the
// replica that rebasing it needs.
//
// Both are O(the document) per keystroke, and that is not new: [crdt.ChangesFrom]
// already turns both texts into rune slices to find the common prefix and suffix.
// The question this answers is whether reading the replica changes the ORDER of
// that cost or only its constant. Run the two together:
//
//	go test -run XXX -bench Keystroke -benchmem ./collabui
func BenchmarkKeystrokeAgainstTheBufferAlone(b *testing.B) {
	d := document(b, 100000)
	prev := d.String()
	next := prev + "!"
	b.ResetTimer()
	for b.Loop() {
		for _, ch := range crdt.ChangesFrom(prev, next) {
			_ = ch
		}
	}
}

func BenchmarkKeystrokeRebasedOntoTheReplica(b *testing.B) {
	d := document(b, 100000)
	prev := d.String()
	next := prev + "!"
	b.ResetTimer()
	for b.Loop() {
		var pending crdt.Change
		for _, ch := range crdt.ChangesFrom(prev, d.String()) {
			pending = ch
		}
		for _, ch := range crdt.ChangesFrom(prev, next) {
			_ = replay(ch, pending)
		}
	}
}

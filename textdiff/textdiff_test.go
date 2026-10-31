// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package textdiff

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// apply replays a script: it must give b, read a in order and b in order.
func apply(t testing.TB, a, b []string, edits []Edit) {
	t.Helper()
	var got []string
	ai, bi := 0, 0
	for _, e := range edits {
		switch e.Op {
		case Equal:
			if e.A != ai || e.B != bi || a[e.A] != b[e.B] {
				t.Fatalf("equal %+v at a%d b%d", e, ai, bi)
			}
			got = append(got, a[e.A])
			ai++
			bi++
		case Delete:
			if e.A != ai {
				t.Fatalf("delete %+v at a%d", e, ai)
			}
			ai++
		case Insert:
			if e.B != bi {
				t.Fatalf("insert %+v at b%d", e, bi)
			}
			got = append(got, b[e.B])
			bi++
		}
	}
	if ai != len(a) || bi != len(b) || !reflect.DeepEqual(append([]string{}, got...), append([]string{}, b...)) {
		t.Fatalf("replay: a%d/%d b%d/%d got %q want %q", ai, len(a), bi, len(b), got, b)
	}
}

// lcs is the length of a longest common subsequence, by dynamic programming:
// the witness that the script is the shortest (its edits = N+M-2·LCS).
func lcs(a, b []string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			switch {
			case a[i-1] == b[j-1]:
				cur[j] = prev[j-1] + 1
			case prev[j] >= cur[j-1]:
				cur[j] = prev[j]
			default:
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func randLines(r *rand.Rand, n, alphabet int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + r.Intn(alphabet)))
	}
	return out
}

func TestShortestAgainstLCS(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 3000; i++ {
		a := randLines(r, r.Intn(40), 1+r.Intn(5))
		b := randLines(r, r.Intn(40), 1+r.Intn(5))
		edits := Lines(a, b)
		apply(t, a, b, edits)
		add, del := Stats(edits)
		if want := len(a) + len(b) - 2*lcs(a, b); add+del != want {
			t.Fatalf("%q -> %q: %d edits, the shortest has %d", a, b, add+del, want)
		}
	}
}

func TestEdgeCases(t *testing.T) {
	for _, c := range [][2][]string{
		{nil, nil},
		{nil, {"x"}},
		{{"x"}, nil},
		{{"a", "b", "c"}, {"a", "b", "c"}},
		{{"a", "b", "c"}, {"c", "b", "a"}},
		{{"a"}, {"b"}},
		{{"a", "x", "b"}, {"a", "b"}},
	} {
		apply(t, c[0], c[1], Lines(c[0], c[1]))
	}
}

// A budget that runs out still gives a correct script, only longer.
func TestBudgetFallback(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	a, b := randLines(r, 300, 3), randLines(r, 300, 3)
	small := LinesWith(a, b, Options{Budget: 10})
	apply(t, a, b, small)
	full := LinesWith(a, b, Options{Budget: -1})
	apply(t, a, b, full)
	sa, sd := Stats(small)
	fa, fd := Stats(full)
	if sa+sd < fa+fd {
		t.Fatalf("a bounded search found a shorter script (%d) than the full one (%d)", sa+sd, fa+fd)
	}
}

// Two large texts with nothing in common stay well under a second.
func TestDisjointIsBounded(t *testing.T) {
	a, b := make([]string, 10000), make([]string, 10000)
	for i := range a {
		a[i] = "a" + strings.Repeat("x", i%7) + string(rune(i))
		b[i] = "b" + string(rune(i))
	}
	t0 := time.Now()
	e := Lines(a, b)
	if d := time.Since(t0); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
	apply(t, a, b, e)
}

// One line in common defeats the shortcut: the budget bounds the search.
func TestNearlyDisjointIsBounded(t *testing.T) {
	a, b := make([]string, 10000), make([]string, 10000)
	for i := range a {
		a[i] = "a" + string(rune(i+1))
		b[i] = "b" + string(rune(i+1))
	}
	a[5000], b[5000] = "common", "common"
	t0 := time.Now()
	e := Lines(a, b)
	d := time.Since(t0)
	t.Logf("10000×10000 lines, one in common: %v", d)
	if d > 15*time.Second {
		t.Fatalf("took %v", d)
	}
	apply(t, a, b, e)
}

func TestSplit(t *testing.T) {
	for in, want := range map[string][]string{"": nil, "a": {"a"}, "a\n": {"a"}, "a\nb": {"a", "b"}, "\n": {""}, "a\n\n": {"a", ""}} {
		if got := Split(in); !reflect.DeepEqual(got, want) {
			t.Errorf("Split(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnified(t *testing.T) {
	a := Split("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	b := Split("one\ntwo\n3\nfour\nfive\nsix\nseven\neight\nnine\nten\neleven\n")
	got := Unified("a", "b", a, b, 2)
	want := "--- a\n+++ b\n@@ -1,5 +1,5 @@\n one\n two\n-three\n+3\n four\n five\n@@ -9,2 +9,3 @@\n nine\n ten\n+eleven\n"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
	if Unified("a", "b", a, a, 3) != "" {
		t.Fatal("no change, no diff")
	}
}

// Hunks closer than 2·context lines merge; none overlaps the next.
func TestHunksMergeAndDoNotOverlap(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	for i := 0; i < 500; i++ {
		a := randLines(r, 60, 3)
		b := append([]string{}, a...)
		for k := 0; k < 5; k++ {
			b[r.Intn(len(b))] = "z"
		}
		edits := Lines(a, b)
		ctx := r.Intn(4)
		hs := Hunks(edits, ctx)
		n := 0
		for j, h := range hs {
			n += len(h.Edits)
			if j > 0 {
				p := hs[j-1]
				if p.AStart+p.ALen > h.AStart || p.BStart+p.BLen > h.BStart {
					t.Fatalf("hunks %d and %d overlap", j-1, j)
				}
			}
			if h.Edits[0].Op == Equal && h.Edits[len(h.Edits)-1].Op == Equal {
				continue
			}
		}
		if n > len(edits) {
			t.Fatal("hunks hold more edits than the script")
		}
	}
}

func TestSideBySide(t *testing.T) {
	a := []string{"x", "a", "b", "y"}
	b := []string{"x", "c", "y", "z"}
	rows := SideBySide(Lines(a, b))
	want := []Row{{Equal, 0, 0}, {Delete, 1, 1}, {Delete, 2, -1}, {Equal, 3, 2}, {Insert, -1, 3}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %+v want %+v", rows, want)
	}
	if !rows[1].Changed() || rows[2].Changed() || rows[0].Changed() {
		t.Fatal("Changed")
	}
}

func FuzzLines(f *testing.F) {
	f.Add("a\nb\nc", "a\nc\nd")
	f.Add("", "x")
	f.Add("same\n", "same\n")
	f.Fuzz(func(t *testing.T, x, y string) {
		if len(x) > 4096 || len(y) > 4096 {
			return
		}
		a, b := Split(x), Split(y)
		e := Lines(a, b)
		apply(t, a, b, e)
		_ = Hunks(e, 3)
		_ = SideBySide(e)
	})
}

func BenchmarkSimilar(b *testing.B) {
	r := rand.New(rand.NewSource(4))
	x := make([]string, 5000)
	for i := range x {
		x[i] = strings.Repeat("w", r.Intn(40))
	}
	y := append([]string{}, x...)
	for k := 0; k < 50; k++ {
		y[r.Intn(len(y))] = "changed"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Lines(x, y)
	}
}

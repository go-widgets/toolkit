// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "testing"

func TestColumnName(t *testing.T) {
	cases := []struct {
		col  int
		want string
	}{
		{0, "A"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{701, "ZZ"},
		{702, "AAA"},
	}
	for _, c := range cases {
		if got := ColumnName(c.col); got != c.want {
			t.Errorf("ColumnName(%d) = %q, want %q", c.col, got, c.want)
		}
	}
}

func TestRefA1(t *testing.T) {
	cases := []struct {
		ref  Ref
		want string
	}{
		{Ref{0, 0}, "A1"},
		{Ref{1, 2}, "B3"},
		{Ref{27, 9}, "AB10"},
	}
	for _, c := range cases {
		if got := c.ref.A1(); got != c.want {
			t.Errorf("%+v.A1() = %q, want %q", c.ref, got, c.want)
		}
	}
}

func TestParseRef(t *testing.T) {
	cases := []struct {
		s    string
		want Ref
		ok   bool
	}{
		{"A1", Ref{0, 0}, true},
		{"B3", Ref{1, 2}, true},
		{"AB10", Ref{27, 9}, true},
		{"", Ref{}, false},    // empty
		{"1", Ref{}, false},   // no letters
		{"A", Ref{}, false},   // no digits
		{"A1B", Ref{}, false}, // non-digit in row part
		{"A0", Ref{}, false},  // zero row
	}
	for _, c := range cases {
		got, ok := ParseRef(c.s)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseRef(%q) = (%+v,%v), want (%+v,%v)", c.s, got, ok, c.want, c.ok)
		}
	}
}

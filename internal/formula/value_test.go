// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import (
	"math"
	"testing"
)

func TestErrKindString(t *testing.T) {
	cases := []struct {
		e    ErrKind
		want string
	}{
		{ErrDiv0, "#DIV/0!"},
		{ErrRef, "#REF!"},
		{ErrName, "#NAME?"},
		{ErrCirc, "#CIRC!"},
		{ErrValue, "#VALUE!"},
		{ErrNum, "#NUM!"},
		{ErrNone, "#ERR!"},     // default arm
		{ErrKind(99), "#ERR!"}, // unknown also falls to default
	}
	for _, c := range cases {
		if got := c.e.String(); got != c.want {
			t.Errorf("ErrKind(%d).String() = %q, want %q", c.e, got, c.want)
		}
	}
}

func TestValueIsError(t *testing.T) {
	if !Error(ErrRef).IsError() {
		t.Error("error value must report IsError")
	}
	if Number(1).IsError() {
		t.Error("number value must not report IsError")
	}
}

func TestValueDisplay(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{Number(42), "42"},
		{Number(1.5), "1.5"},
		{Number(0), "0"},
		{Number(math.Copysign(0, -1)), "0"}, // -0 normalises to "0"
		{TextValue("hi"), "hi"},
		{Error(ErrDiv0), "#DIV/0!"},
		{Blank(), ""}, // default arm
	}
	for _, c := range cases {
		if got := c.v.Display(); got != c.want {
			t.Errorf("Display(%+v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestNumResult(t *testing.T) {
	if got := numResult(math.NaN()); !got.IsError() || got.Err != ErrNum {
		t.Errorf("numResult(NaN) = %+v, want #NUM!", got)
	}
	if got := numResult(math.Inf(1)); !got.IsError() || got.Err != ErrNum {
		t.Errorf("numResult(+Inf) = %+v, want #NUM!", got)
	}
	if got := numResult(3.5); got.Kind != KindNumber || got.Num != 3.5 {
		t.Errorf("numResult(3.5) = %+v, want number 3.5", got)
	}
}

func TestAsNumber(t *testing.T) {
	cases := []struct {
		v    Value
		want float64
		ok   bool
	}{
		{Number(7), 7, true},
		{Blank(), 0, true},
		{TextValue(" 3.5 "), 3.5, true}, // numeric text coerces
		{TextValue("abc"), 0, false},    // non-numeric text fails
		{Error(ErrRef), 0, false},       // default arm
	}
	for _, c := range cases {
		got, ok := asNumber(c.v)
		if got != c.want || ok != c.ok {
			t.Errorf("asNumber(%+v) = (%v,%v), want (%v,%v)", c.v, got, ok, c.want, c.ok)
		}
	}
}

func TestAsText(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{TextValue("hi"), "hi"},
		{Number(2), "2"},
		{Blank(), ""},
		{Error(ErrRef), "#REF!"}, // default arm
	}
	for _, c := range cases {
		if got := asText(c.v); got != c.want {
			t.Errorf("asText(%+v) = %q, want %q", c.v, got, c.want)
		}
	}
}

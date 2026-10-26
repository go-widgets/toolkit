// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestColorHexRoundTrip(t *testing.T) {
	c := RGBA{R: 0x12, G: 0xab, B: 0xcd, A: 0x80}
	s := ColorHex(c)
	if s != "#12abcd80" {
		t.Fatalf("ColorHex = %q, want #12abcd80", s)
	}
	got, err := ParseColorHex(s)
	if err != nil || got != c {
		t.Fatalf("ParseColorHex(%q) = %v, %v; want %v", s, got, err, c)
	}
	if ColorHex(RGBA{R: 1}) != "" {
		t.Fatal("the zero-alpha colour must render as the empty string")
	}
	if _, err := ParseColorHex("#12abcd"); err == nil {
		t.Fatal("a six-digit colour must be rejected")
	}
}

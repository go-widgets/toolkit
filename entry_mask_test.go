// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestEntryPlaceholderAndMask covers the placeholder (empty-text) branch, secret
// masking (display vs Value), and the caret clamp for an out-of-range cursor.
func TestEntryPlaceholderAndMask(t *testing.T) {
	th := DefaultLight()
	buf := makeSurface(120, 24)
	p := newP(buf, 120)

	// Placeholder shows when empty + focused (placeholder branch + caret path).
	e := NewEntry("")
	e.Placeholder = "client id"
	e.SetFocused(true)
	e.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	e.Draw(p, th)

	// Mask: display() masks each rune; Text/Value keep the real contents.
	m := NewEntry("secret")
	m.Mask = '•'
	m.SetFocused(true)
	m.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 24})
	m.Draw(p, th)
	if m.Value() != "secret" {
		t.Fatalf("Value must keep the real text, got %q", m.Value())
	}
	if r := []rune(m.display()); len(r) != 6 || r[0] != '•' || r[5] != '•' {
		t.Fatalf("masked display = %q, want six bullets", m.display())
	}

	// Caret clamp: a cursor past the end clamps to the run length.
	m.Cursor = 999
	m.Draw(p, th)
	if m.Cursor != 6 {
		t.Fatalf("cursor should clamp to 6, got %d", m.Cursor)
	}

	// Mask == 0 → display is the text verbatim.
	if NewEntry("abc").display() != "abc" {
		t.Fatal("no mask should return the text verbatim")
	}
}

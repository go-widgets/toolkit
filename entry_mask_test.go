// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"
)

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
	m.cursor = 999
	m.Draw(p, th)
	if m.cursor != 6 {
		t.Fatalf("cursor should clamp to 6, got %d", m.cursor)
	}

	// Mask == 0 → display is the text verbatim.
	if NewEntry("abc").display() != "abc" {
		t.Fatal("no mask should return the text verbatim")
	}
}

// A masked entry's secret reaches neither the accessibility tree nor the
// clipboard; the same keys on a plain entry do (the planted positive).
func TestEntryMaskKeepsTheSecret(t *testing.T) {
	plain := NewEntry("visible")
	if plain.A11y().Value != "visible" {
		t.Fatalf("plain A11y value = %q", plain.A11y().Value)
	}
	SetClipboardText("before")
	plain.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	if ClipboardText() != "visible" {
		t.Fatalf("plain Ctrl+C clipboard = %q", ClipboardText())
	}

	m := NewEntry("s3cret")
	m.Mask = '•'
	if v := m.A11y().Value; v != "••••••" {
		t.Fatalf("masked A11y value = %q, want six bullets", v)
	}
	SetClipboardText("before")
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+X"})
	if ClipboardText() != "before" {
		t.Fatalf("a masked entry put %q on the clipboard", ClipboardText())
	}
	if m.Text().Get() != "s3cret" {
		t.Fatalf("Ctrl+X on a masked entry changed it to %q", m.Text().Get())
	}
	// Pasting into a secret field still works.
	SetClipboardText("!")
	m.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if !strings.HasSuffix(m.Text().Get(), "!") && !strings.HasPrefix(m.Text().Get(), "!") {
		t.Fatalf("paste into a masked entry = %q", m.Text().Get())
	}
}

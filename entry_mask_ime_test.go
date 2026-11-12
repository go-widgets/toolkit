// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"strings"
	"testing"
)

// A masked entry must not show an IME preview in clear: the preview is the
// secret being typed.
func TestMaskedEntryShowsNoCompositionPreview(t *testing.T) {
	const w, h = 200, 30
	draw := func(e *Entry) []byte {
		e.focused = true // the preview is only ever painted by a focused entry
		e.SetBounds(Rect{W: w, H: h})
		buf := makeSurface(w, h)
		e.Draw(newP(buf, w), DefaultLight())
		return buf
	}
	plain := NewEntry("ab")
	plain.Mask = '•'
	before := draw(plain)

	m := NewEntry("ab")
	m.Mask = '•'
	m.OnEvent(Event{Kind: EventCompositionStart, Code: "s"})
	m.OnEvent(Event{Kind: EventCompositionUpdate, Code: "secret"})
	if m.composition != "" {
		t.Errorf("a masked entry kept the preview %q", m.composition)
	}
	if !bytes.Equal(draw(m), before) {
		t.Error("a composition changed what a masked entry draws")
	}
	if v := m.A11y().Value; strings.Contains(v, "secret") || v != "••" {
		t.Errorf("a11y value %q", v)
	}
	// the committed character still goes in, masked
	m.OnEvent(Event{Kind: EventChar, Code: "é"})
	if m.Text().Get() != "abé" || m.A11y().Value != "•••" {
		t.Errorf("commit: %q %q", m.Text().Get(), m.A11y().Value)
	}

	// a preview that was showing when Mask was set afterwards is not drawn
	late := NewEntry("ab")
	late.OnEvent(Event{Kind: EventCompositionUpdate, Code: "secret"})
	if bytes.Equal(draw(late), draw(NewEntry("ab"))) {
		t.Fatal("the unmasked preview should show (control)")
	}
	late.Mask = '•'
	if !bytes.Equal(draw(late), before) {
		t.Error("a preview already showing is drawn once the entry becomes masked")
	}

	// an unmasked entry still previews
	u := NewEntry("")
	u.OnEvent(Event{Kind: EventCompositionUpdate, Code: "ê"})
	if u.composition != "ê" {
		t.Error("an ordinary entry lost its preview")
	}
}

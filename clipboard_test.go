// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// fakeClipboard is a minimal custom Clipboard used to prove hosts can
// plug in their own OS-backed implementation via SetClipboard.
type fakeClipboard struct {
	get func() string
	set func(string)
}

func (f *fakeClipboard) ClipboardText() string     { return f.get() }
func (f *fakeClipboard) SetClipboardText(s string) { f.set(s) }

func TestMemClipboardGetSet(t *testing.T) {
	m := &memClipboard{}
	if got := m.ClipboardText(); got != "" {
		t.Fatalf("fresh memClipboard = %q, want empty", got)
	}
	m.SetClipboardText("hello")
	if got := m.ClipboardText(); got != "hello" {
		t.Fatalf("after SetClipboardText: got %q, want hello", got)
	}
}

func TestDefaultActiveClipboardIsMemClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil) // start from the documented default
	if _, ok := CurrentClipboard().(*memClipboard); !ok {
		t.Fatalf("default active clipboard = %T, want *memClipboard", CurrentClipboard())
	}
}

func TestClipboardConvenienceFuncsDelegate(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("via convenience func")
	if got := ClipboardText(); got != "via convenience func" {
		t.Fatalf("ClipboardText() = %q", got)
	}
	if got := CurrentClipboard().ClipboardText(); got != "via convenience func" {
		t.Fatalf("CurrentClipboard().ClipboardText() = %q", got)
	}
}

func TestSetClipboardCustomImplIsHonoured(t *testing.T) {
	defer SetClipboard(nil)
	var stored string
	fc := &fakeClipboard{
		get: func() string { return stored },
		set: func(s string) { stored = s },
	}
	SetClipboard(fc)
	if CurrentClipboard() != Clipboard(fc) {
		t.Fatal("CurrentClipboard() did not return the installed custom impl")
	}
	SetClipboardText("custom-backed")
	if stored != "custom-backed" {
		t.Fatalf("custom impl's backing store = %q, want custom-backed", stored)
	}
	if got := ClipboardText(); got != "custom-backed" {
		t.Fatalf("ClipboardText() via custom impl = %q", got)
	}
}

func TestSetClipboardNilRestoresDefault(t *testing.T) {
	defer SetClipboard(nil)
	fc := &fakeClipboard{
		get: func() string { return "ignored" },
		set: func(string) {},
	}
	SetClipboard(fc)
	SetClipboard(nil)
	m, ok := CurrentClipboard().(*memClipboard)
	if !ok {
		t.Fatalf("after SetClipboard(nil): %T, want *memClipboard", CurrentClipboard())
	}
	// The restored default is a fresh, empty memClipboard -- not the
	// pre-custom-install one carrying over any stale text.
	if got := m.ClipboardText(); got != "" {
		t.Fatalf("restored default clipboard text = %q, want empty", got)
	}
}

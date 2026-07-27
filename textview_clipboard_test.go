// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestTextViewCopySelectionWritesGlobalClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	if got := tv.CopySelection(); got != "hello" {
		t.Fatalf("CopySelection = %q, want hello", got)
	}
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("global clipboard = %q, want hello", got)
	}
}

func TestTextViewCopySelectionEmptyDoesNotTouchClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("previous")
	tv := NewTextView("hello")
	// No selection set: Selection is zero value == empty.
	if got := tv.CopySelection(); got != "" {
		t.Fatalf("CopySelection with no selection = %q, want empty", got)
	}
	if got := ClipboardText(); got != "previous" {
		t.Fatalf("empty copy must not clobber clipboard, got %q", got)
	}
}

func TestTextViewCutSelectionWritesGlobalClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	if got := tv.CutSelection(); got != "hello" {
		t.Fatalf("CutSelection = %q, want hello", got)
	}
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("global clipboard after cut = %q, want hello", got)
	}
	if tv.Text() != " world" {
		t.Fatalf("buffer after cut = %q", tv.Text())
	}
}

func TestTextViewCutSelectionEmptyDoesNotTouchClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("previous")
	tv := NewTextView("hello")
	if got := tv.CutSelection(); got != "" {
		t.Fatalf("CutSelection with no selection = %q, want empty", got)
	}
	if got := ClipboardText(); got != "previous" {
		t.Fatalf("empty cut must not clobber clipboard, got %q", got)
	}
}

// TestTextViewCopyPasteRoundTripCrossWidget proves the whole point of a
// global clipboard: copying in one TextView and pasting into a
// DIFFERENT TextView (or reading it back via CurrentClipboard) works,
// which was impossible when CopySelection only returned a string the
// host had to shuttle around itself.
func TestTextViewCopyPasteRoundTripCrossWidget(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	src := NewTextView("hello world")
	src.SetSelection(Selection{0, 0, 0, 5})
	src.CopySelection()

	// Read the clipboard independently of src, as a third party would.
	if got := CurrentClipboard().ClipboardText(); got != "hello" {
		t.Fatalf("CurrentClipboard() = %q, want hello", got)
	}

	dst := NewTextView("world")
	dst.Paste(ClipboardText())
	if dst.Text() != "helloworld" {
		t.Fatalf("cross-widget paste = %q, want helloworld", dst.Text())
	}
}

func TestTextViewCtrlCKeyCopiesSelectionToClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("Ctrl+C clipboard = %q, want hello", got)
	}
	if tv.Text() != "hello world" {
		t.Fatal("Ctrl+C must not mutate the buffer")
	}
}

func TestTextViewCtrlXKeyCutsSelectionToClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	tv := NewTextView("hello world")
	tv.SetSelection(Selection{0, 0, 0, 5})
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+X"})
	if got := ClipboardText(); got != "hello" {
		t.Fatalf("Ctrl+X clipboard = %q, want hello", got)
	}
	if tv.Text() != " world" {
		t.Fatalf("Ctrl+X buffer = %q, want \" world\"", tv.Text())
	}
}

func TestTextViewCtrlVKeyPastesFromClipboard(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	SetClipboardText("HEY")
	tv := NewTextView("world")
	tv.CursorLine, tv.CursorCol = 0, 0
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if tv.Text() != "HEYworld" {
		t.Fatalf("Ctrl+V buffer = %q, want HEYworld", tv.Text())
	}
}

func TestTextViewCtrlVKeyEmptyClipboardIsNoOp(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	tv := NewTextView("world")
	tv.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if tv.Text() != "world" {
		t.Fatalf("Ctrl+V with empty clipboard = %q, want unchanged \"world\"", tv.Text())
	}
	if len(tv.undo) != 0 {
		t.Fatalf("no-op paste must not push undo, len=%d", len(tv.undo))
	}
}

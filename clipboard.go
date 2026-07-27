// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// Clipboard is a back-end-neutral text clipboard shared by every
// text widget in the toolkit (Entry, TextView, ...). Copy/cut write
// to it, paste reads from it, so text copied in one widget can be
// pasted into any other -- including across widget types.
//
// The default implementation is an in-process memory buffer, which
// is adequate for tests and headless rendering but does not reach
// the real OS clipboard. A host that wants OS integration implements
// Clipboard itself -- e.g. the WAI/HTML5 Clipboard API on wasm, an
// OSC-52 escape sequence written to the TTY, NSPasteboard / the win32
// clipboard via cgo -- and installs it once at startup with
// SetClipboard. From then on every widget's copy/cut/paste goes
// through the host's implementation transparently.
type Clipboard interface {
	// ClipboardText returns the current clipboard contents, or ""
	// when the clipboard is empty or unavailable.
	ClipboardText() string
	// SetClipboardText replaces the clipboard contents.
	SetClipboardText(s string)
}

// memClipboard is the default Clipboard: a single string held in
// process memory, shared by every widget. It never touches the OS
// clipboard.
type memClipboard struct{ text string }

func (m *memClipboard) ClipboardText() string { return m.text }

func (m *memClipboard) SetClipboardText(s string) { m.text = s }

// activeClipboard is the process-wide Clipboard every text widget
// reads from and writes to. Defaults to a memClipboard; hosts
// override it via SetClipboard.
var activeClipboard Clipboard = &memClipboard{}

// SetClipboard installs c as the toolkit-wide active clipboard. Every
// widget's copy/cut/paste operation goes through c from this point
// on. Passing nil restores the default in-memory clipboard.
func SetClipboard(c Clipboard) {
	if c == nil {
		activeClipboard = &memClipboard{}
		return
	}
	activeClipboard = c
}

// CurrentClipboard returns the toolkit-wide active Clipboard.
func CurrentClipboard() Clipboard { return activeClipboard }

// ClipboardText returns the active clipboard's contents. Convenience
// shorthand for CurrentClipboard().ClipboardText().
func ClipboardText() string { return activeClipboard.ClipboardText() }

// SetClipboardText replaces the active clipboard's contents.
// Convenience shorthand for CurrentClipboard().SetClipboardText(s).
func SetClipboardText(s string) { activeClipboard.SetClipboardText(s) }

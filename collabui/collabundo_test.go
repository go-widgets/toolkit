// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import (
	"testing"

	"github.com/go-widgets/toolkit"
)

// Ctrl+Z in a collaborative editor reverts what THIS site typed. It must never
// delete a peer's text (toolkit#591): the widget's own snapshot stack records
// whole buffers, so restoring one that predates a peer's insertion would become
// a local deletion of that insertion, published as this site's operation.

// type sends one committed character through the widget's real key path, with
// the caret at (line, col).
func (p *collabPeer) typeAt(line, col int, ch string) {
	p.setCaret(line, col)
	p.do(func() { p.ed.OnEvent(toolkit.Event{Kind: toolkit.EventChar, Code: ch}) })
}

// key sends one key-down through the widget's real key path.
func (p *collabPeer) key(code string) {
	p.do(func() { p.ed.OnEvent(toolkit.Event{Kind: toolkit.EventKeyDown, Code: code}) })
}

func TestUndoNeverDeletesAPeersText(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "undo2", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "undo2", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	ada.typeAt(0, 0, "a")
	bo.await(t, "a on bo", func() bool { return bo.ed.Text().Get() == "a" })
	bo.typeAt(0, 1, "b")
	ada.await(t, "b on ada", func() bool { return ada.ed.Text().Get() == "ab" })

	ada.key("Ctrl+Z")

	want := "b"
	ada.await(t, "ada undo settled", func() bool { return ada.ed.Text().Get() == want })
	bo.await(t, "bo sees ada's undo", func() bool { return bo.ed.Text().Get() == want })
	if a, b := ada.replica(), bo.replica(); a != want || b != want {
		t.Fatalf("after ada's Ctrl+Z: ada replica %q, bo replica %q, want %q (bo's text must survive)", a, b, want)
	}
}

func TestUndoNeverDeletesTwoPeersText(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "undo3", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "undo3", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	cy := newCollabPeer(t, url, "undo3", "cy", RGBA{G: 0xff, A: 0xff}, 3)
	defer ada.close()
	defer bo.close()
	defer cy.close()

	ada.typeAt(0, 0, "a")
	bo.await(t, "a on bo", func() bool { return bo.ed.Text().Get() == "a" })
	bo.typeAt(0, 1, "b")
	cy.await(t, "ab on cy", func() bool { return cy.ed.Text().Get() == "ab" })
	cy.typeAt(0, 2, "c")
	ada.await(t, "abc on ada", func() bool { return ada.ed.Text().Get() == "abc" })
	bo.await(t, "abc on bo", func() bool { return bo.ed.Text().Get() == "abc" })

	ada.key("Ctrl+Z")

	want := "bc"
	for _, p := range []*collabPeer{ada, bo, cy} {
		p.await(t, "undo settled", func() bool { return p.ed.Text().Get() == want })
	}
	for name, p := range map[string]*collabPeer{"ada": ada, "bo": bo, "cy": cy} {
		if got := p.replica(); got != want {
			t.Fatalf("after ada's Ctrl+Z: %s replica %q, want %q (peers' text must survive)", name, got, want)
		}
	}
}

// Undo is per word, not per keystroke, and what a peer typed in the middle of the
// word is not part of it. Redo puts the word back. All of it through the keys.
func TestUndoIsPerWordAndSparesAPeerTypingInTheMiddle(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "undoword", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "undoword", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	for i, ch := range []string{"h", "e", "l", "l", "o", " "} {
		ada.typeAt(0, i, ch)
	}
	bo.await(t, "hello on bo", func() bool { return bo.ed.Text().Get() == "hello " })
	bo.typeAt(0, 6, "B")
	ada.await(t, "B on ada", func() bool { return ada.ed.Text().Get() == "hello B" })
	ada.typeAt(0, 7, "w")
	ada.typeAt(0, 8, "o")
	bo.await(t, "wo on bo", func() bool { return bo.ed.Text().Get() == "hello Bwo" })

	both := func(what, want string) {
		t.Helper()
		for _, p := range []*collabPeer{ada, bo} {
			p.await(t, what, func() bool { return p.ed.Text().Get() == want })
		}
	}
	ada.key("Ctrl+Z")
	both("the word wo to go", "hello B")
	ada.key("Ctrl+Z")
	both("hello to go, B to stay", "B")
	ada.key("Ctrl+Y")
	both("hello back", "hello B")
	ada.key("Ctrl+Shift+Z")
	both("wo back", "hello Bwo")
	ada.key("Ctrl+Z")
	ada.key("Ctrl+Z")
	both("both words undone again", "B")
	ada.key("Ctrl+Z") // nothing of ada's left: B is bo's
	both("nothing of bo's taken", "B")

	// A deletion is a step of its own and is put back by undo.
	ada.typeAt(0, 1, "a")
	ada.typeAt(0, 2, "b")
	both("ab typed after B", "Bab")
	ada.key("Backspace")
	both("b deleted", "Ba")
	ada.key("Ctrl+Z")
	both("b back", "Bab")
	ada.key("Ctrl+Z")
	both("the word ab undone as one", "B")
}

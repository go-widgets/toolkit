// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-crdt/collab"
	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/awareness"
)

// These tests drive two real collab sessions over an in-process WebSocket carrier
// (the same shape as collab's own tests) so the wire protocol is exercised end to
// end — no mock stands in for the CRDT or the transport. Two CodeEditors, each
// bound with a CollabText, share one document; the assertions are on what one
// editor ends up holding and painting after the other edits.

// collabSettle bounds how long a test waits for a change to cross the loopback.
// Everything runs in one process, so anything near this bound is a failure.
const collabSettle = 10 * time.Second

// partName is the shared text part both editors bind to.
const partName = "file:main.go"

// serveCollab starts an in-process collab server reachable over WebSocket and
// returns its ws:// URL, torn down when the test ends.
func serveCollab(t *testing.T) string {
	t.Helper()
	srv := collab.NewServer(collab.Config{})
	hs := httptest.NewServer(srv.ServeWebSocket("*"))
	t.Cleanup(hs.Close)
	return "ws" + strings.TrimPrefix(hs.URL, "http")
}

// joinClient opens one raw session, closed when the test ends.
func joinClient(t *testing.T, url, doc string, site crdt.SiteID) *collab.Client {
	t.Helper()
	c, err := collab.Join(context.Background(), collab.WebSocket(url), collab.ClientConfig{Document: doc, Site: site})
	if err != nil {
		t.Fatalf("Join(site %d): %v", site, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// A collabPeer is one editor plus its binding, driven from a single dedicated
// goroutine — its "UI goroutine". Every touch of the editor (local edits, the
// remote-apply closures the binding hands over, Close) runs on that goroutine, so
// the editor's MVVM state is never accessed concurrently, exactly as the toolkit
// requires. The test goroutine talks to it only through channels.
type collabPeer struct {
	ed     *CodeEditor
	client *collab.Client
	ct     *CollabText

	cmds   chan func()   // closures to run on the UI goroutine
	remote chan struct{} // a coalesced "a remote update was applied" signal
	quit   chan struct{}
	done   chan struct{} // closed when the UI goroutine exits

	// drainOff, when set (on the UI goroutine), stops the loop receiving the
	// binding's update closures, so remote applications are held — the convergence
	// test uses it to make two edits genuinely concurrent against one base.
	drainOff bool

	closeOnce sync.Once
}

// newCollabPeer joins the document, binds an editor to partName, and starts the
// peer's UI goroutine.
func newCollabPeer(t *testing.T, url, doc, name string, color RGBA, site crdt.SiteID) *collabPeer {
	t.Helper()
	p := &collabPeer{
		ed:     NewCodeEditor(""),
		client: joinClient(t, url, doc, site),
		cmds:   make(chan func(), 64),
		remote: make(chan struct{}, 1),
		quit:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	started := make(chan error, 1)
	go func() {
		defer close(p.done)
		ct, err := NewCollabText(p.ed, p.client, partName)
		if err != nil {
			started <- err
			return
		}
		ct.Name = name
		ct.Color = color
		ct.OnRemoteChange = func() {
			select {
			case p.remote <- struct{}{}:
			default:
			}
		}
		p.ct = ct
		started <- nil
		updates := ct.Updates()
		for {
			var up <-chan func()
			if !p.drainOff {
				up = updates
			}
			select {
			case fn := <-p.cmds:
				fn()
			case apply, ok := <-up:
				if !ok {
					updates = nil
					continue
				}
				apply()
			case <-p.quit:
				return
			}
		}
	}()
	if err := <-started; err != nil {
		t.Fatalf("NewCollabText: %v", err)
	}
	return p
}

// do runs fn on the peer's UI goroutine and waits for it.
func (p *collabPeer) do(fn func()) {
	done := make(chan struct{})
	p.cmds <- func() {
		fn()
		close(done)
	}
	<-done
}

// text reads the editor's buffer on the UI goroutine.
func (p *collabPeer) text() string {
	var s string
	p.do(func() { s = p.ed.Text().Get() })
	return s
}

// decorations copies the editor's remote-caret decorations on the UI goroutine.
func (p *collabPeer) decorations() []Decoration {
	var d []Decoration
	p.do(func() { d = append([]Decoration(nil), p.ed.Decorations...) })
	return d
}

// setText replaces the buffer as a user edit would.
func (p *collabPeer) setText(s string) { p.do(func() { p.ed.SetText(s) }) }

// setCaret moves the caret as a user would, collapsing any selection.
func (p *collabPeer) setCaret(line, col int) {
	p.do(func() {
		p.ed.Selection().Set(sel(line, col, line, col))
		p.ed.CursorLine().Set(line)
		p.ed.CursorCol().Set(col)
	})
}

// selectRange sets a selection and parks the caret at (caretLine, caretCol),
// which is how the editor reports a drag: the caret is one end, the anchor the
// other.
func (p *collabPeer) selectRange(sel Selection, caretLine, caretCol int) {
	p.do(func() {
		p.ed.Selection().Set(sel)
		p.ed.CursorLine().Set(caretLine)
		p.ed.CursorCol().Set(caretCol)
	})
}

// await blocks until cond holds on the UI goroutine, woken by remote-apply
// signals and bounded by a deadline.
func (p *collabPeer) await(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.After(collabSettle)
	for {
		var ok bool
		p.do(func() { ok = cond() })
		if ok {
			return
		}
		select {
		case <-p.remote:
		case <-deadline:
			p.do(func() { ok = cond() })
			if ok {
				return
			}
			t.Fatalf("timed out waiting for %s; text is %q", what, p.text())
		}
	}
}

// replica reads the shared text as this peer's own replica holds it. While
// draining is held that is NOT what the editor shows: the replica goes on merging
// what arrives off the wire whatever the screen is doing, and the gap between the
// two is where a local edit can be put in the wrong place.
func (p *collabPeer) replica() string {
	var s string
	p.do(func() { s = p.ct.text.String() })
	return s
}

// until blocks until cond holds on the UI goroutine, polling rather than waiting
// on a remote-apply signal. It is the one shape await cannot serve: a peer whose
// draining is held applies nothing and so signals nothing, and these tests wait
// for exactly that peer's replica to move.
func (p *collabPeer) until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(collabSettle)
	for {
		var ok bool
		p.do(func() { ok = cond() })
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; the editor shows %q and the replica holds %q",
				what, p.text(), p.replica())
		}
		time.Sleep(time.Millisecond)
	}
}

// close stops the binding on the UI goroutine, then the UI goroutine itself, then
// leaves the session (so a peer watching this one sees it depart). The client's
// own t.Cleanup close is idempotent.
func (p *collabPeer) close() {
	p.closeOnce.Do(func() {
		p.do(func() { _ = p.ct.Close() })
		close(p.quit)
		<-p.done
		_ = p.client.Close()
	})
}

func TestCollabTextSyncsTextAndRemoteCaret(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "demo", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "demo", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	// bo starts empty.
	if got := bo.text(); got != "" {
		t.Fatalf("bo starts with %q, want empty", got)
	}

	// ada types, then parks its caret at column 3.
	ada.setText("hello world")
	ada.setCaret(0, 3)

	// bo converges to ada's text.
	bo.await(t, "bo to receive ada's text", func() bool { return bo.ed.Text().Get() == "hello world" })
	if got := bo.text(); got != "hello world" {
		t.Fatalf("bo text = %q, want %q", got, "hello world")
	}

	// bo paints one decoration for ada: her colour, her name, her caret.
	bo.await(t, "ada's caret decoration", func() bool {
		for _, d := range bo.ed.Decorations {
			if d.Label == "ada" && d.CursorLine == 0 && d.CursorCol == 3 {
				return true
			}
		}
		return false
	})
	decs := bo.decorations()
	if len(decs) != 1 {
		t.Fatalf("bo has %d decorations, want 1: %+v", len(decs), decs)
	}
	if got, want := decs[0].Color, (RGBA{R: 0xff, A: 0xff}); got != want {
		t.Fatalf("decoration colour = %+v, want %+v (ada's)", got, want)
	}
	if !decs[0].Selection.IsEmpty() {
		t.Fatalf("decoration selection = %+v, want empty (caret only)", decs[0].Selection)
	}
}

func TestCollabTextReflectsSelectionAndDeletion(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "sel", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "sel", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	ada.setText("hello world")
	bo.await(t, "base text", func() bool { return bo.ed.Text().Get() == "hello world" })

	// A forward selection: caret at the end (0,5), anchor at (0,0).
	ada.selectRange(sel(0, 0, 0, 5), 0, 5)
	bo.await(t, "ada's forward selection", func() bool {
		for _, d := range bo.ed.Decorations {
			if d.Label == "ada" && d.Selection == sel(0, 0, 0, 5) && d.CursorCol == 5 {
				return true
			}
		}
		return false
	})

	// A backward selection over the same span: caret at the start (0,2), anchor at
	// (0,5). It must canonicalise to the same range but with the caret at 2 — this
	// exercises the other branch of the anchor/head choice.
	ada.selectRange(sel(0, 2, 0, 5), 0, 2)
	bo.await(t, "ada's backward selection", func() bool {
		for _, d := range bo.ed.Decorations {
			if d.Label == "ada" && d.Selection == sel(0, 2, 0, 5) && d.CursorCol == 2 {
				return true
			}
		}
		return false
	})

	// A deletion propagates as an updated buffer.
	ada.setText("hello")
	bo.await(t, "ada's deletion", func() bool { return bo.ed.Text().Get() == "hello" })
	if got := bo.text(); got != "hello" {
		t.Fatalf("bo text after deletion = %q, want %q", got, "hello")
	}
}

func TestCollabTextRemovesDecorationWhenPeerLeaves(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "leave", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "leave", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer bo.close()

	ada.setText("shared")
	ada.setCaret(0, 2)
	bo.await(t, "ada's decoration to appear", func() bool { return len(bo.ed.Decorations) == 1 })

	// ada leaves; her presence goes and so must her decoration.
	ada.close()
	bo.await(t, "ada's decoration to disappear", func() bool {
		for _, d := range bo.ed.Decorations {
			if d.Label == "ada" {
				return false
			}
		}
		return true
	})
	if got := bo.decorations(); len(got) != 0 {
		t.Fatalf("bo still has decorations after ada left: %+v", got)
	}
}

func TestCollabTextConcurrentEditsConverge(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "conv", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "conv", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	// Establish a shared base both replicas hold.
	ada.setText("abc")
	bo.await(t, "base on bo", func() bool { return bo.ed.Text().Get() == "abc" })

	// Hold remote application on both so each computes its edit against the same
	// base — a genuinely concurrent pair of inserts — then let them merge.
	ada.do(func() { ada.drainOff = true })
	bo.do(func() { bo.drainOff = true })
	ada.do(func() { ada.ed.SetText("Xabc") }) // Insert(0,"X")
	bo.do(func() { bo.ed.SetText("abcY") })   // Insert(3,"Y")
	ada.do(func() { ada.drainOff = false })
	bo.do(func() { bo.drainOff = false })
	// Nudge each loop so it re-enters the select now that draining is back on.
	ada.do(func() {})
	bo.do(func() {})

	ada.await(t, "ada to converge", func() bool { return ada.ed.Text().Get() == "XabcY" })
	bo.await(t, "bo to converge", func() bool { return bo.ed.Text().Get() == "XabcY" })

	// Exactly the two inserts landed on the shared base — no echo doubled anything.
	if a, b := ada.text(), bo.text(); a != b || a != "XabcY" {
		t.Fatalf("did not converge cleanly: ada=%q bo=%q, want %q", a, b, "XabcY")
	}

	// A fresh witness reads the same text off the wire, confirming the document —
	// not just the two views — holds it.
	w := joinClient(t, url, "conv", 9)
	h, err := w.Text(partName)
	if err != nil {
		t.Fatalf("witness Text: %v", err)
	}
	deadline := time.After(collabSettle)
	for h.String() != "XabcY" {
		select {
		case <-w.Changes():
		case <-deadline:
			t.Fatalf("witness sees %q, want %q", h.String(), "XabcY")
		}
	}
}

// pending drives the one interleaving TestCollabTextConcurrentEditsConverge used
// to reach by luck: a peer types while a remote edit has merged into its replica
// and has NOT yet been painted. The peer's edit is computed against what it could
// see; it must land where the person put it, not at that offset of a buffer they
// were never shown.
//
// Holding the drain and then waiting for the replica to move makes the losing
// interleaving the only one, so this is a statement rather than a race: with the
// local edit replayed at its raw offset it fails every run.
func whilePending(t *testing.T, doc, base, seen, typed, want string) {
	t.Helper()
	url := serveCollab(t)
	ada := newCollabPeer(t, url, doc, "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, doc, "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	ada.setText(base)
	bo.await(t, "the base on bo", func() bool { return bo.ed.Text().Get() == base })

	// bo stops painting. Its replica does not stop merging.
	bo.do(func() { bo.drainOff = true })
	ada.setText(seen)
	bo.until(t, "ada's edit to reach bo's replica but not bo's screen", func() bool {
		return bo.ct.text.String() == seen && bo.ed.Text().Get() == base
	})

	// bo edits the buffer in front of it — the base, still.
	bo.do(func() { bo.ed.SetText(typed) })
	bo.do(func() { bo.drainOff = false })
	bo.do(func() {}) // re-enter the select now that draining is back on

	ada.await(t, "ada to settle", func() bool { return ada.ed.Text().Get() == want })
	bo.await(t, "bo to settle", func() bool { return bo.ed.Text().Get() == want })
	if a, b := ada.text(), bo.text(); a != want || b != want {
		t.Fatalf("ada=%q bo=%q, want %q (bo edited %q into %q while its replica already held %q)",
			a, b, want, base, typed, seen)
	}
}

func TestATypedCharacterLandsWhereTheUserPutItWhileARemoteInsertIsPending(t *testing.T) {
	// bo appends Y to the "abc" it can see, while its replica already holds ada's
	// "Xabc". Offset 3 of what bo saw is offset 4 of what the replica holds.
	whilePending(t, "pend-after", "abc", "Xabc", "abcY", "XabcY")
}

func TestATypedCharacterBeforeAPendingInsertDoesNotMove(t *testing.T) {
	// The other side of the rule: bo types at offset 0, ahead of ada's insert at
	// offset 3, so the offset is already right and must be left alone.
	whilePending(t, "pend-before", "abc", "abcY", "Zabc", "ZabcY")
}

// ⛔ THE CASE THAT DECIDED THE RULE, MEASURED RATHER THAN ASSUMED. The binding
// learns what the replica took on by comparing it with the screen, and
// [crdt.ChangesFrom] answers any two texts with ONE contiguous edit — so two
// remote edits far apart come back as a single span covering the untouched line
// between them.
//
// An offset inside that span is NOT sent to its start. It goes the same distance
// into what replaced it, which for a line whose length did not change is exactly
// where it was typed. Collapsing to the start would have put bo's character at
// the beginning of the line — worse than the raw offset this whole file exists to
// correct, which is how the rule came to be written this way.
//
// Where the span's length DID change the result is an estimate, and the half that
// is not an estimate is asserted beside it: no character anybody wrote is gone.
// Asking Client.TakeChanges for the real edits instead of comparing two whole
// texts removes the estimate; that is go-widgets/toolkit#527.
func TestSeveralRemoteEditsAtOnceStillPlaceAKeystrokeWhereItWasTyped(t *testing.T) {
	url := serveCollab(t)
	ada := newCollabPeer(t, url, "wide", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	bo := newCollabPeer(t, url, "wide", "bo", RGBA{B: 0xff, A: 0xff}, 2)
	defer ada.close()
	defer bo.close()

	ada.setText("hello world")
	bo.await(t, "the base on bo", func() bool { return bo.ed.Text().Get() == "hello world" })

	// Two edits, one at each end, with the whole line between them untouched.
	bo.do(func() { bo.drainOff = true })
	ada.setText("Hello world")
	ada.setText("Hello worlD")
	bo.until(t, "both of ada's edits to reach bo's replica", func() bool {
		return bo.ct.text.String() == "Hello worlD" && bo.ed.Text().Get() == "hello world"
	})

	// bo types in the middle of the line it can see.
	bo.do(func() { bo.ed.SetText("hello Xworld") })
	bo.do(func() { bo.drainOff = false })
	bo.do(func() {})

	const want = "Hello XworlD"
	bo.await(t, "bo to settle", func() bool { return bo.ed.Text().Get() == want })
	ada.await(t, "ada to settle", func() bool { return ada.ed.Text().Get() == want })
	if a, b := ada.text(), bo.text(); a != want || b != want {
		t.Fatalf("ada=%q bo=%q, want %q — bo typed X between \"hello \" and \"world\"", a, b, want)
	}
}

func TestADeletionTakesTheCharactersTheUserCouldSeeAndNotTheOnesTheyCouldNot(t *testing.T) {
	// ada drops "X" into the middle of "abcd" — bo never saw it — and bo then
	// deletes "bc", the two characters it could see. ada's X stays: a person
	// cannot have meant to delete a character that was not on their screen.
	whilePending(t, "pend-delete", "abcd", "abXcd", "ad", "aXd")
}

func TestCollabTextCloseStopsPumpAndIsIdempotent(t *testing.T) {
	url := serveCollab(t)
	p := newCollabPeer(t, url, "close", "ada", RGBA{R: 0xff, A: 0xff}, 1)

	// The pump is running: its stopped channel is still open.
	select {
	case <-p.ct.stopped:
		t.Fatal("pump already stopped before Close")
	default:
	}

	p.close()

	// Close stopped the pump (the done signal) and closed the update channel.
	select {
	case <-p.ct.stopped:
	default:
		t.Fatal("pump did not stop after Close")
	}
	if _, ok := <-p.ct.Updates(); ok {
		t.Fatal("Updates channel still open after Close")
	}

	// A second Close is a no-op that still returns nil. The UI goroutine is gone,
	// but Close now only touches its own (already-cleared) state.
	if err := p.ct.Close(); err != nil {
		t.Fatalf("second Close = %v, want nil", err)
	}
}

func TestCollabTextClosedApplyIsInert(t *testing.T) {
	url := serveCollab(t)
	p := newCollabPeer(t, url, "inert", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	p.setText("kept")
	p.close()

	// A remote-apply closure that outlived Close must do nothing. The UI goroutine
	// is gone, so calling it here is single-threaded and safe.
	p.ct.applyRemote()
	if got := p.ed.Text().Get(); got != "kept" {
		t.Fatalf("editor changed after a post-Close apply: %q, want %q", got, "kept")
	}
}

func TestCollabTextUnknownPartName(t *testing.T) {
	url := serveCollab(t)
	client := joinClient(t, url, "bad", 1)
	if _, err := NewCollabText(NewCodeEditor(""), client, ""); err == nil {
		t.Fatal("NewCollabText with an empty part name = nil error, want an error")
	}
}

func TestCollabTextPumpStopsWhenSessionEndsIdle(t *testing.T) {
	url := serveCollab(t)
	p := newCollabPeer(t, url, "sessend", "ada", RGBA{R: 0xff, A: 0xff}, 1)
	defer p.close()

	// End the session underneath the binding while the pump sits idle: the pump's
	// outer select takes its client-Done branch.
	if err := p.client.Close(); err != nil {
		t.Fatalf("client.Close: %v", err)
	}
	select {
	case <-p.ct.stopped:
	case <-time.After(collabSettle):
		t.Fatal("pump did not stop when the session ended")
	}
}

func TestCollabTextMetaOmitsEmptyIdentity(t *testing.T) {
	url := serveCollab(t)
	client := joinClient(t, url, "meta", 1)
	ct, err := NewCollabText(NewCodeEditor(""), client, partName)
	if err != nil {
		t.Fatalf("NewCollabText: %v", err)
	}
	defer ct.Close()

	// No name, zero colour: both keys are omitted.
	if m := ct.meta(); len(m) != 0 {
		t.Fatalf("meta with no identity = %v, want empty", m)
	}
	// A name but still a zero colour: only the name key is present.
	ct.Name = "sole"
	m := ct.meta()
	if m[MetaName] != "sole" {
		t.Fatalf("meta name = %q, want %q", m[MetaName], "sole")
	}
	if _, ok := m[MetaColor]; ok {
		t.Fatalf("meta carried a colour key for a zero colour: %v", m)
	}
}

func TestRuneOffsetAndLineCol(t *testing.T) {
	const buf = "ab\ncdé\n"
	// buf lines: "ab" (2), "cdé" (3 runes), "" — total runes 2+1+3+1+0 = 7.
	cases := []struct {
		line, col, off int
	}{
		{0, 0, 0},
		{0, 2, 2}, // end of line 0
		{1, 0, 3}, // start of line 1
		{1, 3, 6}, // end of line 1 (after the é)
		{2, 0, 7}, // start of the trailing empty line
	}
	for _, c := range cases {
		if got := runeOffset(buf, c.line, c.col); got != c.off {
			t.Errorf("runeOffset(%d,%d) = %d, want %d", c.line, c.col, got, c.off)
		}
		gl, gc := lineCol(buf, c.off)
		if gl != c.line || gc != c.col {
			t.Errorf("lineCol(%d) = (%d,%d), want (%d,%d)", c.off, gl, gc, c.line, c.col)
		}
	}

	// Bounds and clamping.
	if got := runeOffset(buf, -1, 5); got != 0 {
		t.Errorf("runeOffset(negative line) = %d, want 0", got)
	}
	if got := runeOffset(buf, 99, 0); got != 7 {
		t.Errorf("runeOffset(line past end) = %d, want 7 (buffer length)", got)
	}
	if got := runeOffset(buf, 0, -1); got != 0 {
		t.Errorf("runeOffset(negative col) = %d, want 0", got)
	}
	if got := runeOffset(buf, 0, 99); got != 2 {
		t.Errorf("runeOffset(col past line end) = %d, want 2", got)
	}
	if l, c := lineCol(buf, -3); l != 0 || c != 0 {
		t.Errorf("lineCol(negative) = (%d,%d), want (0,0)", l, c)
	}
	if l, c := lineCol(buf, 999); l != 2 || c != 0 {
		t.Errorf("lineCol(past end) = (%d,%d), want (2,0)", l, c)
	}
}

// TestPublishCursorSkipsWhileApplyingRemote deterministically covers the
// applyingRemote guard in publishCursor. The end-to-end tests only hit it when a
// remote apply happens to move the caret (idempotent Sets fire no subscriber), so
// its coverage was timing/data-dependent and flaked the 100% gate. Driving the
// guard directly makes it deterministic: with applyingRemote set, publishCursor
// must return before touching the (nil here) editor/client — a caret notification
// caused by our own remote application is never echoed back as presence.
func TestPublishCursorSkipsWhileApplyingRemote(t *testing.T) {
	c := &CollabText{applyingRemote: true}
	c.publishCursor() // returns at the guard; a panic would mean it fell through
}

// TestRebuildDecorationsSkipsSelf deterministically covers the self-peer guard in
// rebuildDecorations. awareness.Peers() includes self once this client has
// published a cursor, and SetCursor seeds the local presence registry
// synchronously — so after it, the client's own site is in Peers() with certainty
// (the end-to-end tests only hit this branch when self-presence happens to have
// landed, which flaked the 100% gate). rebuildDecorations must skip that self
// entry, leaving no decoration for it.
func TestRebuildDecorationsSkipsSelf(t *testing.T) {
	url := serveCollab(t)
	client := joinClient(t, url, "doc-selfdec", crdt.SiteID(1))
	ed := NewCodeEditor("hello\nworld")
	ct := &CollabText{ed: ed, client: client, self: client.Site()}

	if err := client.SetCursor(awareness.Cursor{Anchor: 0, Head: 3}, map[string]string{}); err != nil {
		t.Fatalf("SetCursor: %v", err)
	}
	// Self is now the sole entry in Peers(); it must be filtered out.
	ct.rebuildDecorations("hello\nworld")
	if len(ed.Decorations) != 0 {
		t.Fatalf("self peer must be skipped, got %d decoration(s): %+v", len(ed.Decorations), ed.Decorations)
	}
}

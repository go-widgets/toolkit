// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/go-crdt/collab"
	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/awareness"
)

// Presence metadata keys. A participant publishes its display name and caret
// colour under these keys, and a peer reads them back to paint a [Decoration].
// They are this binding's private contract with itself — the awareness core
// carries the map verbatim and interprets none of it.
const (
	// MetaName is the presence key carrying a participant's display name, shown
	// as the tag beside its remote caret.
	MetaName = "name"
	// MetaColor is the presence key carrying a participant's caret colour, as the
	// "#rrggbbaa" string [Decoration.Color] is painted in.
	MetaColor = "color"
)

// CollabText is the live binding between a [CodeEditor] (or its embedded
// [TextView]) and one text part of a [github.com/go-crdt/collab] session: it
// turns the editor into one replica of a shared document, so any number of peers
// edit the same buffer at once and each sees the others' carets and selections
// painted in place. It is the text-editor analogue of [SyncSpreadsheet] over a
// [CollabSheet] and of an [IsoDiagram] over an [IsoCRDTDocument] — the reusable
// "CRDT ⇄ editor" glue, with no widget change: the [CodeEditor] edits exactly as
// before, its edits now flow into the shared document, and a merged remote edit
// (text or presence) flows back into it.
//
// It adds NO merge logic of its own. The convergence, commutativity, idempotence
// and associativity are the collab/crdt packages', inherited whole. This type is
// only the mapping between the editor's MVVM state and the session:
//
//   - Local → shared. A user edit ticks the editor's Text() [mvvm.Observable];
//     the binding diffs the previous buffer against the new one ([crdt.ChangesFrom])
//     and replays that one contiguous edit into the text part as rune-offset
//     Insert/Delete. A caret or selection move ticks CursorLine()/CursorCol()/
//     Selection() and is published as an [awareness.Cursor] (anchor + head rune
//     offsets) carrying this participant's [CollabText.Name] and [CollabText.Color].
//   - Shared → local. A pump goroutine wakes on the session's coalesced change
//     signal, loads the part's current text into the editor, and rebuilds the
//     editor's [TextView.Decorations] from the session's peers — one Decoration
//     per other participant, its caret and selection mapped from rune offsets to
//     (line, col) and painted in the peer's own colour and name.
//
// # Threading model
//
// The editor's MVVM state is not safe for concurrent use — like the rest of the
// toolkit it must be driven from one goroutine (call it the UI goroutine). The
// binding owns a pump goroutine that consumes the session's asynchronous change
// signal, but it NEVER touches editor state from that goroutine. Instead it hands
// each remote application to the host over [CollabText.Updates] as a closure the
// host runs on its own UI goroutine — the same "the host calls back on its own UI
// thread" contract [Browser] uses, marshalled here through a channel so there is
// a clean happens-before and no field to race on. A host drains it with
//
//	for apply := range ct.Updates() {
//		apply() // on the UI goroutine — applies remote text + rebuilds decorations
//	}
//
// The channel closes when the pump stops (a [CollabText.Close] or the session
// ending), which ends the range. Local edits, that drain loop and
// [CollabText.Close] all run on the one UI goroutine.
//
// [CollabText.Close] stops the pump, waits for it to exit and unsubscribes from
// the editor; it must be called on the UI goroutine and is idempotent. Because
// the pump selects on its stop signal even while offering an update, a host that
// has stopped draining never wedges Close.
type CollabText struct {
	ed     *CodeEditor
	text   *collab.Text
	client *collab.Client
	self   crdt.SiteID

	// Name is this participant's display name, published with every caret move so
	// a peer can tag the remote caret. Set it before the first caret move; an
	// empty name publishes no name key.
	Name string
	// Color is this participant's caret colour, published with every caret move.
	// Its zero value (A == 0) publishes no colour key, and a peer paints the
	// caret in the theme default.
	Color RGBA

	// OnRemoteChange, when set, is called at the end of every remote application —
	// a repaint seam for the host, the analogue of [CollabSheet.Subscribe]. It
	// runs on the UI goroutine, inside the apply closure the host ran.
	OnRemoteChange func()

	// prev is the buffer as the binding last saw it, the baseline a local edit is
	// diffed against. UI goroutine only.
	prev string
	// applyingRemote is set while a remote update is being written into the
	// editor, so the editor's own change notifications do not echo that update
	// back into the session as a fresh local edit. UI goroutine only.
	applyingRemote bool
	// closed short-circuits a remote application still queued when Close ran. UI
	// goroutine only.
	closed bool

	apply   chan func()
	unsub   []func()
	done    chan struct{}
	stopped chan struct{}
	once    sync.Once
}

// NewCollabText binds ed to the text part named textName of client's session and
// returns the live binding. The part's current text is loaded into the editor at
// once, and from then on local edits flow into the session and remote edits flow
// back once the host drains [CollabText.Updates]. An invalid textName (empty or
// malformed; see [crdt.Part]) is returned as an error and nothing is wired.
//
// The returned binding owns a pump goroutine; call [CollabText.Close] to stop it.
func NewCollabText(ed *CodeEditor, client *collab.Client, textName string) (*CollabText, error) {
	handle, err := client.Text(textName)
	if err != nil {
		return nil, err
	}
	c := &CollabText{
		ed:      ed,
		text:    handle,
		client:  client,
		self:    client.Site(),
		apply:   make(chan func()),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	// Load the shared text into the editor, then take the loaded buffer as the
	// diff baseline. Subscriptions are wired after the load so it is not mistaken
	// for a local edit.
	ed.SetText(handle.String())
	c.prev = ed.Text().Get()
	c.unsub = append(c.unsub,
		ed.Text().SubscribeChanged(c.syncLocal),
		ed.CursorLine().SubscribeChanged(c.publishCursor),
		ed.CursorCol().SubscribeChanged(c.publishCursor),
		ed.Selection().SubscribeChanged(c.publishCursor),
	)
	go c.pump()
	return c, nil
}

// Updates delivers remote-application closures for the host to run on its UI
// goroutine; see the threading model on [CollabText]. The channel closes when the
// pump stops.
func (c *CollabText) Updates() <-chan func() { return c.apply }

// syncLocal replays a user edit into the shared text. It is the editor's
// Text()-changed observer; a change the binding itself just applied is skipped
// via applyingRemote so it is not echoed back.
func (c *CollabText) syncLocal() {
	if c.applyingRemote {
		return
	}
	next := c.ed.Text().Get()
	// ChangesFrom reduces previous↔next to the single contiguous edit between
	// their common prefix and suffix, expressed in rune offsets — exactly the
	// text part's Insert/Delete coordinates.
	for _, ch := range crdt.ChangesFrom(c.prev, next) {
		if ch.Removed > 0 {
			_ = c.text.Delete(ch.Pos, ch.Removed)
		}
		if ch.Text != "" {
			_ = c.text.Insert(ch.Pos, ch.Text)
		}
	}
	c.prev = next
}

// publishCursor publishes the local caret and selection as presence. It is the
// observer of the three caret observables; a move the binding itself caused while
// applying a remote update is skipped.
func (c *CollabText) publishCursor() {
	if c.applyingRemote {
		return
	}
	buf := c.ed.Text().Get()
	head := runeOffset(buf, c.ed.CursorLine().Get(), c.ed.CursorCol().Get())
	anchor := head
	if sel := c.ed.Selection().Get(); !sel.IsEmpty() {
		start := runeOffset(buf, sel.StartLine, sel.StartCol)
		end := runeOffset(buf, sel.EndLine, sel.EndCol)
		// The caret sits at one end of the selection; the anchor is the other.
		if head == end {
			anchor = start
		} else {
			anchor = end
		}
	}
	_ = c.client.SetCursor(awareness.Cursor{Anchor: anchor, Head: head}, c.meta())
}

// meta builds this participant's presence map from Name and Color, omitting a key
// whose value is empty so a peer falls back to its own defaults.
func (c *CollabText) meta() map[string]string {
	m := map[string]string{}
	if c.Name != "" {
		m[MetaName] = c.Name
	}
	if col := isoColorToHex(c.Color); col != "" {
		m[MetaColor] = col
	}
	return m
}

// pump is the shared→local goroutine: it wakes on the session's coalesced change
// signal and offers a remote-application closure to the host, until Close or the
// session ends. A change arms pending, whose send is offered in the same select
// as the two stop signals — so the stop signals stay live whether or not a
// delivery is outstanding, and a host that has stopped draining can never wedge
// Close. Further changes while a delivery is armed simply re-arm it (coalesced):
// one delivery after the last change is enough, since applyRemote always reads
// the part's current text.
func (c *CollabText) pump() {
	defer close(c.stopped)
	defer close(c.apply)
	var pending chan func() // nil until a change is waiting to be delivered
	for {
		select {
		case <-c.done:
			return
		case <-c.client.Done():
			return
		case <-c.client.Changes():
			pending = c.apply
		case pending <- c.applyRemote:
			pending = nil
		}
	}
}

// applyRemote loads the shared text into the editor and rebuilds the remote-caret
// decorations. The host runs it on the UI goroutine. applyingRemote is held
// across the whole update so neither the reloaded text nor the moved caret echoes
// back into the session.
func (c *CollabText) applyRemote() {
	if c.closed {
		return
	}
	// Drain the accumulated per-part changes so the session does not retain them;
	// v1 rebuilds the buffer wholesale from the part's current text rather than
	// applying the changes incrementally.
	c.client.TakeChanges()
	next := c.text.String()

	c.applyingRemote = true
	// Preserve the local caret across the full-buffer replace by its absolute rune
	// offset (SetText would otherwise park it at the origin); lineCol reclamps it
	// into the new buffer.
	head := runeOffset(c.prev, c.ed.CursorLine().Get(), c.ed.CursorCol().Get())
	c.ed.SetText(next)
	c.prev = c.ed.Text().Get()
	line, col := lineCol(c.prev, head)
	c.ed.CursorLine().Set(line)
	c.ed.CursorCol().Set(col)
	c.rebuildDecorations(c.prev)
	c.applyingRemote = false

	if c.OnRemoteChange != nil {
		c.OnRemoteChange()
	}
}

// rebuildDecorations replaces the editor's decorations with one per other
// participant, each caret and selection mapped from the peer's rune offsets to
// (line, col) over buf. A departed peer is simply absent from Peers, so its
// decoration is gone; an empty peer set clears them all.
func (c *CollabText) rebuildDecorations(buf string) {
	var decs []Decoration
	for _, p := range c.client.Peers() {
		if p.Site == c.self {
			continue
		}
		headLine, headCol := lineCol(buf, p.Cursor.Head)
		anchorLine, anchorCol := lineCol(buf, p.Cursor.Anchor)
		color, _ := isoColorFromHex(p.Meta[MetaColor])
		decs = append(decs, Decoration{
			Label:      p.Meta[MetaName],
			Color:      color,
			CursorLine: headLine,
			CursorCol:  headCol,
			Selection:  SelectionRange(anchorLine, anchorCol, headLine, headCol),
		})
	}
	c.ed.Decorations = decs
}

// Close stops the pump goroutine, waits for it to exit, and unsubscribes from the
// editor. It must be called on the UI goroutine and is idempotent.
func (c *CollabText) Close() error {
	c.once.Do(func() { close(c.done) })
	<-c.stopped
	c.closed = true
	for _, u := range c.unsub {
		u()
	}
	c.unsub = nil
	return nil
}

// runeOffset returns the rune offset in buf of the caret at (line, col), where
// buf is the editor's whole buffer with lines joined by '\n'. A line past the end
// maps to the buffer's length, and a column past its line's end to that line's
// end, so a caret at any bound maps to a valid offset.
func runeOffset(buf string, line, col int) int {
	if line < 0 {
		return 0
	}
	lines := strings.Split(buf, "\n")
	if line >= len(lines) {
		return utf8.RuneCountInString(buf)
	}
	off := 0
	for i := 0; i < line; i++ {
		off += utf8.RuneCountInString(lines[i]) + 1 // + 1 for the '\n'
	}
	runes := utf8.RuneCountInString(lines[line])
	if col < 0 {
		col = 0
	} else if col > runes {
		col = runes
	}
	return off + col
}

// lineCol is the inverse of runeOffset: it maps a rune offset in buf to the
// (line, col) the editor addresses caret positions by. A non-positive offset is
// the origin, and an offset past the end clamps to the end of the last line.
func lineCol(buf string, off int) (line, col int) {
	if off <= 0 {
		return 0, 0
	}
	lines := strings.Split(buf, "\n")
	remaining := off
	for i, ln := range lines {
		n := utf8.RuneCountInString(ln)
		if remaining <= n {
			return i, remaining
		}
		remaining -= n + 1 // this line plus its '\n'
	}
	last := len(lines) - 1
	return last, utf8.RuneCountInString(lines[last])
}

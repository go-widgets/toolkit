// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package collabui

import (
	"unicode/utf8"

	"github.com/go-crdt/crdt"
)

// ⛔ A PERSON EDITS THE BUFFER IN FRONT OF THEM, AND THE REPLICA HAS OFTEN MOVED
// ON. Those are two coordinate systems, and an offset means something different
// in each.
//
// The binding hands a remote application to the host as a closure the host runs
// on its own UI goroutine (see the threading model on [CollabText]), so between a
// peer's edit merging into this replica and the editor being repainted with it
// there is a window — a frame, a host that batches, a host that is busy — during
// which the screen shows one text and the replica holds another. A keystroke in
// that window is computed against what the person could see. Replayed at its raw
// offset into what the replica now holds, it lands somewhere else.
//
// It is not a convergence failure, which is what makes it hard to see: every
// replica agrees, on a document nobody wrote. Convergence belongs to
// github.com/go-crdt/crdt and is not in question here. What is in question is
// intention, and preserving it across an edit the author did not see is what
// operational transformation's INCLUSION TRANSFORMATION is for — Ellis & Gibbs,
// "Concurrency Control in Groupware Systems" (SIGMOD '89), and the client half of
// Jupiter (Nichols et al., UIST '95), where a client transforms its own operation
// against the operations it has received and not yet applied.
//
// This is that transformation and only that: the remote edit is already in this
// replica, strictly before ours, so there is no symmetric transform to agree on
// with anybody and no TP1/TP2 obligation to discharge. There is one question —
// where did the person mean to put this? — and it has a local answer.
//
// [crdt.ChangesFrom] reduces any two texts to ONE contiguous edit (common prefix,
// common suffix, what is left between them), so both the local edit and the
// remote delta are single changes and there is no sequence to compose.

// rebase maps a rune offset in the buffer the person was looking at onto the same
// point in the text as the replica now holds it, given the one change that brings
// the former up to the latter.
//
// An offset AT the remote change's position is left where it is, so what the
// person typed goes in front of what arrived rather than behind it: they typed at
// a point with nothing after it that they could see, and the choice is arbitrary
// in the only case where it is observable.
//
// An offset AMONG characters the change took away goes the same distance into
// what replaced them, as far as that reaches. ⛔ NOT TO THE START OF THE SPAN,
// and the difference is the whole behaviour of the binding on an ordinary
// document. [crdt.ChangesFrom] answers any two texts with ONE change, so two
// remote edits far apart come back as a single span covering the untouched line
// between them; collapsing to its start would send a keystroke typed in that line
// to the beginning of it, which is WORSE than the raw offset this function exists
// to correct. Going the same distance in agrees with the raw offset exactly where
// the raw offset was right, and clamps where it was not.
//
// The zero [crdt.Change] is the identity, which is the ordinary case — the screen
// and the replica agree, and nothing moves.
func rebase(pos int, through crdt.Change) int {
	put := utf8.RuneCountInString(through.Text)
	switch {
	case pos <= through.Pos:
		return pos
	case pos >= through.Pos+through.Removed:
		return pos + put - through.Removed
	default:
		return through.Pos + min(pos-through.Pos, put)
	}
}

// replay turns one edit made against a stale buffer into the operations to apply
// to the text as the replica now holds it, in the order to apply them.
//
// ⛔ A DELETION TAKES THE CHARACTERS THE PERSON COULD SEE AND NOT THE ONES THEY
// COULD NOT. A remote insertion that landed inside the deleted span is not part
// of what was selected — it was not on the screen when the selection was made —
// so the span is deleted around it, as two edits, and the character survives.
// Mapping the span's two ends and deleting everything between them would destroy
// it, which is worse than putting a keystroke in the wrong place: the misplaced
// keystroke is visible and someone fixes it, and this one is a character that was
// there a moment ago and is not any more.
//
// The two deletions are emitted highest-first so the second one's offset is still
// valid once the first has been applied. The typed text goes in last, at the
// point the deletions closed up to — which, when the person was pointing into the
// middle of what a peer put there, is inside it. That splits the peer's run and
// keeps every character of it, which is the right way round: placement is a guess
// and text is not.
func replay(local, through crdt.Change) []crdt.Change {
	start := rebase(local.Pos, through)
	end := rebase(local.Pos+local.Removed, through)
	// The span the remote change PUT there, in the replica's coordinates. Nothing
	// in it was on the screen the person was deleting from.
	kept, keptEnd := through.Pos, through.Pos+utf8.RuneCountInString(through.Text)

	var out []crdt.Change
	if hi := max(start, keptEnd); hi < end {
		out = append(out, crdt.Change{Pos: hi, Removed: end - hi})
	}
	if lo := min(end, kept); start < lo {
		out = append(out, crdt.Change{Pos: start, Removed: lo - start})
	}
	if local.Text != "" {
		out = append(out, crdt.Change{Pos: start, Text: local.Text})
	}
	return out
}

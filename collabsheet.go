// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"strings"

	"github.com/go-crdt/crdt"
	"github.com/go-crdt/crdt/structured"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit/internal/formula"
)

// CollabSheet is the collaborative backing model for a [Spreadsheet]: an
// A1-addressed grid whose rows, columns and cells live in the shared
// structured.Sheet CRDT core, so any number of replicas may edit it at once —
// offline, in any delivery order — and every replica converges to the same
// sheet. It adds NO merge logic of its own; the convergence, commutativity,
// idempotence and associativity are the structured/crdt package's, inherited
// whole. This type is only the two things a widget needs on top: an A1
// (column, row) façade over the CRDT's stable row and column identities, and an
// observable revision that ticks on every change so a bound view repaints.
//
// # Stable addressing
//
// A cell is stored against the identities of its row and column, not their
// positions, so a concurrent insertion or deletion of some other row on a peer
// renumbers positions but leaves every cell where its author put it. The
// positional methods here ([CollabSheet.SetCellText] and friends) resolve an
// index through the current [CollabSheet.RowCount]/[CollabSheet.ColCount] order
// at the moment of the call, which is exactly how a spreadsheet UI addresses a
// cell.
//
// # Transport
//
// Every mutation returns the [crdt.PartOps] to broadcast to peers, already
// applied locally; a replica integrates a peer's batches with
// [CollabSheet.Apply]. A late joiner loads a [CollabSheet.Snapshot] with
// [LoadCollabSheet], and a delta sync uses [CollabSheet.Version] and
// [CollabSheet.OpsSince]. All of this is the structured.Sheet's own transport,
// re-exposed unchanged.
//
// # Fan-out through MVVM
//
// State crossing into a view crosses through go-widgets/mvvm: a local edit and
// an applied remote batch both tick an [mvvm.Observable] revision, and
// [CollabSheet.Subscribe] registers a change observer over it — the seam
// [SyncSpreadsheet] uses to mirror a merged remote edit back into a widget.
//
// A CollabSheet is not safe for concurrent use; drive it from one goroutine and
// exchange operations, not the value, between replicas.
type CollabSheet struct {
	sheet *structured.Sheet
	rev   *mvvm.Observable[uint64]
}

// ErrCellOutOfRange reports a positional cell access outside the sheet's current
// row/column extent. It is the one error the cell methods raise; the underlying
// CRDT writes cannot fail once an in-range identity is resolved.
var ErrCellOutOfRange = errors.New("toolkit: spreadsheet cell out of range")

// NewCollabSheet returns an empty collaborative sheet that issues operations as
// site. Every replica editing one sheet concurrently must pass a distinct
// [crdt.SiteID].
func NewCollabSheet(site crdt.SiteID) *CollabSheet {
	return &CollabSheet{sheet: structured.NewSheet(site), rev: mvvm.NewObservable[uint64](0)}
}

// LoadCollabSheet rebuilds a collaborative sheet from a [CollabSheet.Snapshot],
// to be edited as site. A malformed snapshot is returned as an error, never a
// panic.
func LoadCollabSheet(site crdt.SiteID, snapshot []byte) (*CollabSheet, error) {
	sh, err := structured.LoadSheet(site, snapshot)
	if err != nil {
		return nil, err
	}
	return &CollabSheet{sheet: sh, rev: mvvm.NewObservable[uint64](0)}, nil
}

// notify ticks the revision so every change observer runs. The value only ever
// increases, so the observable's equality test never suppresses the notice.
func (c *CollabSheet) notify() { c.rev.Set(c.rev.Get() + 1) }

// Subscribe registers fn to run after every change — a local edit or an applied
// remote batch — and returns a function that unsubscribes it. It is the seam a
// view binds to so it repaints when the sheet, however edited, changes.
func (c *CollabSheet) Subscribe(fn func()) (unsubscribe func()) { return c.rev.SubscribeChanged(fn) }

// Rev returns the current revision counter, which ticks on every change. A view
// or a test reads it to tell whether the sheet moved.
func (c *CollabSheet) Rev() uint64 { return c.rev.Get() }

// Site returns the replica identity this sheet issues operations as.
func (c *CollabSheet) Site() crdt.SiteID { return c.sheet.Site() }

// Snapshot encodes the whole sheet, for a joining peer or for persistence. Two
// replicas holding the same operations produce identical bytes, so it doubles
// as a convergence oracle.
func (c *CollabSheet) Snapshot() []byte { return c.sheet.Snapshot() }

// Version returns what this replica holds, to hand a peer that will send back
// what it is missing; see [CollabSheet.OpsSince].
func (c *CollabSheet) Version() crdt.CompositeVersion { return c.sheet.Version() }

// OpsSince returns the operations this replica holds that v does not, ready to
// send to the peer that produced v. Pass a nil version for everything.
func (c *CollabSheet) OpsSince(v crdt.CompositeVersion) []crdt.PartOps {
	return c.sheet.OpsSince(v)
}

// Pending reports how many received operations are still waiting for the
// operations they depend on. It is zero once a replica has caught up.
func (c *CollabSheet) Pending() int { return c.sheet.Pending() }

// Apply integrates batches of operations from peers, tolerating duplicates and
// reordering, and ticks the revision so a bound view repaints. An invalid batch
// is reported as an error and changes nothing.
func (c *CollabSheet) Apply(batches ...crdt.PartOps) error {
	if err := c.sheet.Apply(batches...); err != nil {
		return err
	}
	c.notify()
	return nil
}

// RowCount returns the number of rows present.
func (c *CollabSheet) RowCount() int { return c.sheet.RowCount() }

// ColCount returns the number of columns present.
func (c *CollabSheet) ColCount() int { return c.sheet.ColCount() }

// InsertRow adds a row at index pos and returns the operation to broadcast. pos
// may equal [CollabSheet.RowCount], which appends; a pos outside [0, RowCount]
// is an error.
func (c *CollabSheet) InsertRow(pos int) (crdt.PartOps, error) {
	_, ops, err := c.sheet.InsertRow(pos)
	if err != nil {
		return crdt.PartOps{}, err
	}
	c.notify()
	return ops, nil
}

// AppendRow adds a row after the last and returns the operation to broadcast.
func (c *CollabSheet) AppendRow() (crdt.PartOps, error) { return c.InsertRow(c.sheet.RowCount()) }

// InsertCol adds a column at index pos and returns the operation to broadcast.
func (c *CollabSheet) InsertCol(pos int) (crdt.PartOps, error) {
	_, ops, err := c.sheet.InsertCol(pos)
	if err != nil {
		return crdt.PartOps{}, err
	}
	c.notify()
	return ops, nil
}

// AppendCol adds a column after the last and returns the operation to broadcast.
func (c *CollabSheet) AppendCol() (crdt.PartOps, error) { return c.InsertCol(c.sheet.ColCount()) }

// DeleteRow removes the row at index pos and returns the operation to broadcast.
// The cells of a removed row are left addressed by their (now unreferenced) row
// identity, exactly as the CRDT core keeps them. A pos outside [0, RowCount) is
// an error.
func (c *CollabSheet) DeleteRow(pos int) (crdt.PartOps, error) {
	ops, err := c.sheet.DeleteRow(pos)
	if err != nil {
		return crdt.PartOps{}, err
	}
	c.notify()
	return ops, nil
}

// DeleteCol removes the column at index pos and returns the operation to
// broadcast, on the same terms as [CollabSheet.DeleteRow].
func (c *CollabSheet) DeleteCol(pos int) (crdt.PartOps, error) {
	ops, err := c.sheet.DeleteCol(pos)
	if err != nil {
		return crdt.PartOps{}, err
	}
	c.notify()
	return ops, nil
}

// ids resolves a positional (column, row) address to the stable identities the
// CRDT stores a cell against, reporting whether the address is in range.
func (c *CollabSheet) ids(col, row int) (structured.RowID, structured.ColID, bool) {
	if col < 0 || row < 0 {
		return structured.RowID{}, structured.ColID{}, false
	}
	rows := c.sheet.Rows()
	cols := c.sheet.Cols()
	if row >= len(rows) || col >= len(cols) {
		return structured.RowID{}, structured.ColID{}, false
	}
	return rows[row], cols[col], true
}

// textCell renders a raw cell string as the structured cell it stores: a
// leading-"=" marks a formula, anything else a literal. The raw text is carried
// verbatim either way, so the widget's own formula engine re-parses it on read
// and no computed value is ever replicated.
func textCell(raw string) structured.Cell {
	if strings.HasPrefix(raw, "=") {
		return structured.Formula(raw)
	}
	return structured.Literal(raw)
}

// SetCellText stores raw in the cell at column col, row row and returns the
// operation to broadcast. An empty raw clears the cell. The cell is addressed by
// the row's and column's stable identities, so the write lands on the same cell
// on every replica however the sheet's shape has since changed. An out-of-range
// address is [ErrCellOutOfRange] and changes nothing.
func (c *CollabSheet) SetCellText(col, row int, raw string) (crdt.PartOps, error) {
	rowID, colID, ok := c.ids(col, row)
	if !ok {
		return crdt.PartOps{}, ErrCellOutOfRange
	}
	var op crdt.PartOps
	if raw == "" {
		// The key is a valid (row, col) identity pair, so a map delete cannot fail.
		op, _ = c.sheet.ClearCell(rowID, colID)
	} else {
		// Likewise a map set with a valid key cannot fail.
		op, _ = c.sheet.SetCell(rowID, colID, textCell(raw))
	}
	c.notify()
	return op, nil
}

// CellText returns the raw text stored in the cell at column col, row row — the
// formula or literal an editor re-opens — or "" for an empty or out-of-range
// cell.
func (c *CollabSheet) CellText(col, row int) string {
	rowID, colID, ok := c.ids(col, row)
	if !ok {
		return ""
	}
	cell, ok := c.sheet.GetCell(rowID, colID)
	if !ok {
		return ""
	}
	return cell.Text
}

// SyncSpreadsheet wires a [Spreadsheet] widget onto a [CollabSheet] without
// changing the widget's API: the widget renders and edits exactly as before,
// but its committed edits now flow into the collaborative model and a merged
// remote edit flows back into the widget. It is the observable adaptation layer
// the collaborative model exposes to the existing widget.
//
// On call it mirrors the collab sheet's current cells into the widget, then
// subscribes so every later change — local or remote — re-mirrors, and routes
// the widget's [Spreadsheet.OnCellChange] into [CollabSheet.SetCellText]. Only
// the overlapping (column, row) window is mirrored, so a widget smaller than the
// collaborative sheet shows its top-left corner and never addresses a cell it
// has no room for. The returned detach unsubscribes and clears the widget's
// change hook.
func SyncSpreadsheet(ss *Spreadsheet, cs *CollabSheet) (detach func()) {
	pull := func() {
		rows := cs.RowCount()
		if rows > ss.Rows() {
			rows = ss.Rows()
		}
		cols := cs.ColCount()
		if cols > ss.Cols() {
			cols = ss.Cols()
		}
		for r := 0; r < rows; r++ {
			for col := 0; col < cols; col++ {
				ss.SetCell(col, r, cs.CellText(col, r))
			}
		}
	}
	pull()
	unsub := cs.Subscribe(pull)
	ss.OnCellChange = func(ref formula.Ref, raw string) {
		// A widget cell always lies within the mirrored window, so the write is in
		// range; its ops reach peers through the collab model's own transport.
		cs.SetCellText(ref.Col, ref.Row, raw)
	}
	return func() {
		unsub()
		ss.OnCellChange = nil
	}
}

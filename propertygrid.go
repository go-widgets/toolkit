// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// PropertyGrid is a two-column Name/Value grid for viewing and editing a set
// of named properties: the Value column is inline-editable, and committing an
// edit fires OnChange with the property's name and new value. It composes an
// editable Table (see NewTable / TableColumn.Editable), so it inherits the
// Table's rendering, selection, windowed scrolling and per-cell editing --
// PropertyGrid just adds the property-oriented API on top (Add / SetValue /
// Value, keyed by name rather than row index).
//
// The Name column is read-only; only Value cells open an editor on click.
type PropertyGrid struct {
	Base
	// OnChange fires when a Value cell edit is committed (Enter): name is the
	// edited property, value its new text. Nil is safe.
	OnChange func(name, value string)

	table *Table
	names []string // property name per row, parallel to table.Rows
}

// NewPropertyGrid builds an empty property grid (Name | Value, Value
// editable). Populate it with Add / SetValue.
func NewPropertyGrid() *PropertyGrid {
	pg := &PropertyGrid{
		table: NewTable([]TableColumn{
			{Title: "Name"},                  // auto width, read-only
			{Title: "Value", Editable: true}, // auto width, inline-editable
		}, nil),
	}
	// Route the Table's committed cell edit back out as an OnChange keyed by
	// the property name (the Table has already written table.Rows[row][1]).
	pg.table.OnCellEdit = func(row, _ int, value string) {
		if pg.OnChange != nil && row >= 0 && row < len(pg.names) {
			pg.OnChange(pg.names[row], value)
		}
	}
	return pg
}

// Table exposes the underlying Table for advanced configuration (column
// widths, RowIcon, sorting, ...). Mutating Rows directly is not supported --
// go through Add / SetValue so the name index stays in sync.
func (pg *PropertyGrid) Table() *Table { return pg.table }

// Add appends a property row. Duplicate names are allowed (Value/SetValue
// address the first match), mirroring how a raw Table allows duplicate rows.
func (pg *PropertyGrid) Add(name, value string) {
	pg.names = append(pg.names, name)
	pg.table.Rows = append(pg.table.Rows, []string{name, value})
}

// Clear removes every property.
func (pg *PropertyGrid) Clear() {
	pg.names = nil
	pg.table.Rows = nil
	pg.table.Selected = -1
}

// RemoveAt removes the property at row index i, keeping the name index and the
// backing Table in sync and clearing a now-stale selection. An out-of-range i
// is a no-op. Exposed so a host can implement a "delete property" menu action
// (hit-test the row with PropertyGrid.Table().RowAt).
func (pg *PropertyGrid) RemoveAt(i int) {
	if i < 0 || i >= len(pg.names) {
		return
	}
	pg.names = append(pg.names[:i], pg.names[i+1:]...)
	pg.table.Rows = append(pg.table.Rows[:i], pg.table.Rows[i+1:]...)
	pg.table.Selected = -1
}

// Value returns the current value of the named property, or "" if there is no
// such property.
func (pg *PropertyGrid) Value(name string) string {
	if i := pg.indexOf(name); i >= 0 {
		return pg.table.Rows[i][1]
	}
	return ""
}

// SetValue updates the named property's value in place, or appends it if it is
// not present yet.
func (pg *PropertyGrid) SetValue(name, value string) {
	if i := pg.indexOf(name); i >= 0 {
		pg.table.Rows[i][1] = value
		return
	}
	pg.Add(name, value)
}

// indexOf returns the row index of the first property named name, or -1.
func (pg *PropertyGrid) indexOf(name string) int {
	for i, n := range pg.names {
		if n == name {
			return i
		}
	}
	return -1
}

// SetBounds positions the grid and its backing Table.
func (pg *PropertyGrid) SetBounds(r Rect) {
	pg.Base.SetBounds(r)
	pg.table.SetBounds(r)
}

// Draw paints the grid via its backing Table.
func (pg *PropertyGrid) Draw(p painter.Painter, theme *Theme) {
	pg.table.Draw(p, theme)
}

// OnEvent forwards to the backing Table (selection, and Value-cell editing).
func (pg *PropertyGrid) OnEvent(ev Event) {
	pg.table.OnEvent(ev)
}

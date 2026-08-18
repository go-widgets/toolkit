// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestPropertyGridAddValueSetValue covers Add, Value (hit + miss), and
// SetValue (in-place update + append-when-absent).
func TestPropertyGridAddValueSetValue(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("Width", "800")
	pg.Add("Height", "600")

	if pg.Value("Width") != "800" || pg.Value("Height") != "600" {
		t.Fatalf("values = %q/%q, want 800/600", pg.Value("Width"), pg.Value("Height"))
	}
	if pg.Value("Missing") != "" {
		t.Fatalf("missing property Value = %q, want empty", pg.Value("Missing"))
	}

	pg.SetValue("Width", "1024") // in-place
	if pg.Value("Width") != "1024" {
		t.Fatalf("after SetValue, Width = %q, want 1024", pg.Value("Width"))
	}
	pg.SetValue("Depth", "24") // append
	if pg.Value("Depth") != "24" || len(pg.names) != 3 {
		t.Fatalf("SetValue append failed: Depth=%q names=%d", pg.Value("Depth"), len(pg.names))
	}
}

// TestPropertyGridClear empties the grid.
func TestPropertyGridClear(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("A", "1")
	pg.table.Selected().Set(0)
	pg.Clear()
	if len(pg.names) != 0 || len(pg.table.Rows) != 0 || pg.table.Selected().Get() != -1 {
		t.Fatalf("after Clear: names=%d rows=%d sel=%d, want 0/0/-1",
			len(pg.names), len(pg.table.Rows), pg.table.Selected().Get())
	}
}

// TestPropertyGridEditFiresOnChange: clicking a Value cell opens an editor and
// committing (Enter) fires OnChange keyed by the property name and writes the
// value back (readable via Value).
func TestPropertyGridEditFiresOnChange(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("Title", "Hello")
	pg.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})

	var gotName, gotVal string
	calls := 0
	pg.OnChange = func(name, value string) { gotName, gotVal, calls = name, value, calls+1 }

	// Click the Value cell of row 0 (Value column is the right half; x=150).
	pg.OnEvent(Event{Kind: EventClick, X: 150, Y: TableHeaderHeight + 2})
	pg.OnEvent(Event{Kind: EventChar, Code: "!"})
	pg.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})

	if calls != 1 || gotName != "Title" || gotVal != "Hello!" {
		t.Fatalf("OnChange = (%q,%q,%d), want (Title, Hello!, 1)", gotName, gotVal, calls)
	}
	if pg.Value("Title") != "Hello!" {
		t.Fatalf("value after edit = %q, want Hello!", pg.Value("Title"))
	}
}

// TestPropertyGridNameColumnNotEditable: clicking the Name cell selects/does
// not open an editor (Name column is read-only), so no OnChange fires.
func TestPropertyGridNameColumnNotEditable(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("Title", "Hello")
	pg.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	fired := false
	pg.OnChange = func(string, string) { fired = true }

	// Click the Name cell (left half; x=40) then type + Enter.
	pg.OnEvent(Event{Kind: EventClick, X: 40, Y: TableHeaderHeight + 2})
	pg.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if fired {
		t.Fatal("editing must not start on the read-only Name column")
	}
}

// TestPropertyGridEditNilOnChangeNoPanic: committing an edit with OnChange
// unset writes the value and must not panic.
func TestPropertyGridEditNilOnChangeNoPanic(t *testing.T) {
	pg := NewPropertyGrid() // OnChange nil
	pg.Add("K", "v")
	pg.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	pg.OnEvent(Event{Kind: EventClick, X: 150, Y: TableHeaderHeight + 2})
	pg.OnEvent(Event{Kind: EventChar, Code: "2"})
	pg.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if pg.Value("K") != "v2" {
		t.Fatalf("value = %q, want v2", pg.Value("K"))
	}
}

// TestPropertyGridDrawAndTableAccessor renders the grid and checks the Table
// accessor exposes the backing table.
func TestPropertyGridDrawAndTableAccessor(t *testing.T) {
	pg := NewPropertyGrid()
	pg.Add("Name", "Value")
	pg.SetBounds(Rect{X: 0, Y: 0, W: 160, H: 80})
	if pg.Table() == nil || pg.Table() != pg.table {
		t.Fatal("Table() must return the backing table")
	}
	buf := makeSurface(160, 80)
	pg.Draw(newP(buf, 160), DefaultLight())
	// The header + a row painted something in the top-left.
	if !anyPainted(buf, 160, 0, 0, 160, TableHeaderHeight+TableRowHeight) {
		t.Fatal("property grid painted nothing")
	}
}

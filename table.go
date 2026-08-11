// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"sort"
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// Table renders a structured data grid: a fixed header row of column
// titles above a body of text rows. The widget is the missing piece
// vs GTK's ColumnView + DaisyUI's Table -- the toolkit's ListBox +
// TreeView give a single column of items, whereas Table lays cells
// out horizontally under labelled columns.
//
// Visual (per row):
//
//	+----------------+--------+-------------+
//	| Header A       | Hdr B  | Header C    |  <- TableHeaderHeight, SurfaceAlt
//	+----------------+--------+-------------+
//	| row 0 cell 0   | 0.1    | 0.2         |  <- TableRowHeight, Surface
//	| row 1 cell 0   | 1.1    | 1.2         |  <- TableRowHeight, Background
//	| ...
//	+----------------+--------+-------------+
//
// Selected row (if 0 <= Selected < len(Rows)) paints in Theme.Accent
// with the accent-inverted ink -- theme.Extra["OnAccent"] when the
// GTK loader supplied one, otherwise theme.Background (the same
// fallback the Button + ListBox + TreeView selected states already
// use, so the visual reads consistent across widgets). When
// MultiSelect is true every row in the multi-row selection set paints
// the same way, not just Selected (which keeps acting as the anchor
// for Shift-range clicks) -- see MultiSelect + SelectedRows.
//
// The widget is content-only: it never reorders Rows itself. Header
// clicks + separator drags are surfaced through OnSort/OnColumnResize
// so the host (which owns the data model) can re-sort Rows or persist
// a new column width, then hand the Table back its updated state.
type Table struct {
	Base
	focusState
	// Columns are the header cells (title + optional pixel width).
	// A zero Width means "auto" -- the column claims an equal share of
	// whatever pixel budget is left after the fixed-Width columns.
	Columns []TableColumn
	// Rows is the body content. Each inner slice SHOULD have
	// len == len(Columns); rows shorter than that render only the
	// cells they carry (missing trailing cells are drawn as blank
	// space, the row background still paints edge-to-edge).
	Rows [][]string
	// Selected is the 0-indexed row highlighted with Theme.Accent;
	// -1 (or any out-of-range value) means "no selection" and the
	// zebra stripe pattern paints unmodified. While MultiSelect is
	// true, Selected doubles as the anchor a Shift-click ranges from;
	// it is still the ONLY row painted while MultiSelect is false.
	Selected int

	// RowIcon, when non-nil, supplies an optional leading icon for each
	// body row -- the missing piece for file-list-style views (a
	// per-row file-type glyph before the name) that previously forced a
	// host to hand-compose custom rows instead of using Table directly.
	// It is called with a 0-indexed body row and returns the icon's
	// painter (any of the stock DrawIconXxx functions in icons.go, or a
	// caller's own of the same TableIconFunc signature) plus an ok flag;
	// returning ok == false (or a nil painter) means "this row has no
	// icon" while still reserving the gutter, so text stays aligned down
	// the first column whether or not a given row carries a glyph.
	//
	// When RowIcon is set, Draw reserves a fixed leading gutter
	// (TableCellPadX + TableIconSize) inside the FIRST column, paints the
	// row's icon there in the row's own ink (accent-inverted on the
	// selected row, OnSurface otherwise), and shifts that column's text
	// right by the gutter. Column boundaries, separators, sort, scroll
	// and row/column hit-testing are all unchanged -- the gutter is
	// carved out of column 0's interior, so a click on the icon still
	// resolves to column 0 and to its row exactly as a click on the text
	// would.
	//
	// The zero value (nil) is the original, pre-feature behaviour: no
	// gutter is reserved and Draw renders byte-for-byte identically to
	// before this field existed.
	RowIcon func(row int) (draw TableIconFunc, ok bool)

	// MultiSelect switches body-row clicks (handled by OnEvent) from
	// inert to selection-driving: a plain click selects only that row
	// (clearing any other selection, moving the Selected anchor to
	// it); a Ctrl-click toggles that row's membership without
	// disturbing the anchor; a Shift-click selects the inclusive
	// range between the anchor (Selected) and the clicked row,
	// likewise leaving the anchor in place so repeated Shift-clicks
	// keep ranging from the same origin. Header-row clicks (sort) and
	// separator drags (resize) are unaffected either way.
	//
	// The zero value (false) is the original passive-viewer
	// behaviour: OnEvent never touches Selected or any selection
	// state for a body-row click, and Draw highlights only Selected --
	// byte-for-byte the same as before this field existed.
	MultiSelect bool

	// selectedRows is the multi-row selection set. A nil map means
	// "nothing selected", mirroring how Selected == -1 means no
	// single-row anchor. It is consulted by Draw/IsRowSelected only
	// while MultiSelect is true, but the SetRowSelection /
	// ToggleRowSelect / SelectRowRange / ClearRowSelection API works
	// regardless -- a host may pre-seed a selection before switching
	// MultiSelect on.
	selectedRows map[int]bool

	// ScrollRow is the 0-indexed body row currently painted at the top
	// of the body (the header itself never scrolls). Draw + rowAt both
	// read it through clampScrollRow, so an out-of-range value set
	// directly (or left stale after Rows shrinks) never windows past
	// [0, maxScrollRow()] -- the same defensive-collapse idiom Selected
	// and SortColumn already use. The zero value (0) is the original,
	// pre-feature behaviour: the body starts at row 0, and if every row
	// fits within Bounds().H, Draw renders byte-identically to before
	// this field existed (no scrollbar, no windowing). Use ScrollTo /
	// ScrollBy / scrollToSelected to move it -- they keep the field
	// itself clamped, unlike a raw assignment.
	ScrollRow int

	// FrozenColumns is the number of leading columns pinned in place while
	// the rest scroll horizontally (see ScrollX). Clamped into
	// [0, len(Columns)]. Meaningful only when the columns are all
	// fixed-width and overflow the viewport (hScrollable); with any
	// auto-width column the table fits to width and never scrolls
	// horizontally, so this is inert. The zero value (0) pins nothing --
	// byte-identical to before this field existed.
	FrozenColumns int

	// ScrollX is the horizontal pixel offset of the SCROLLABLE (non-frozen)
	// columns. Read through clampScrollX, so an out-of-range value never
	// scrolls past the content. The zero value (0) shows the columns from
	// their left edge. Inert unless hScrollable (fixed columns wider than
	// the viewport); use ScrollXTo / ScrollXBy to move it clamped.
	ScrollX int

	// SortColumn is the 0-indexed column currently sorted, or -1 (or
	// any out-of-range value) for "no sort" -- Draw skips the ▲/▼
	// indicator and OnEvent treats every header click as a fresh sort.
	// The Table never reorders Rows itself; SortColumn/SortAsc only
	// drive the indicator glyph, matching how Selected only drives the
	// accent highlight.
	SortColumn int
	// SortAsc is the direction of SortColumn: true draws ▲ (ascending),
	// false draws ▼ (descending). Meaningless while SortColumn is out
	// of range.
	SortAsc bool
	// OnSort fires when a Sortable header cell is clicked. col is the
	// clicked column; ascending is the NEW direction after the click
	// (clicking the already-active column toggles it, clicking a new
	// column resets to ascending). The Table updates SortColumn/SortAsc
	// itself before firing so the very next Draw shows the indicator;
	// the host is responsible for re-sorting Rows and handing them back.
	OnSort func(col int, ascending bool)

	// OnColumnResize fires whenever a separator drag (or a direct
	// SetColumnWidth call) changes a column's width. newWidth is the
	// clamped pixel width now in effect.
	OnColumnResize func(col, newWidth int)

	// OnCellEdit fires when an inline cell edit is committed (Enter): the
	// Table has already written the new value into Rows[row][col]. Nil is
	// safe. Only cells in a column with Editable set can be edited.
	OnCellEdit func(row, col int, value string)

	// OnSelect fires whenever the highlighted row (Selected, which doubles
	// as the keyboard cursor) changes through a user interaction: a
	// MultiSelect body-row click that moves the anchor, or a keyboard cursor
	// move (Arrow / Page / Home / End). row is the new Selected index. The
	// Table has already updated Selected before firing, and the callback runs
	// only when the value actually changes, so a re-select of the same row is
	// silent. Nil is safe -- a host that does not track selection leaves it
	// unset. This single-argument slot is what makes Selected observable (e.g.
	// via mvvmtk.BindTableSelection).
	OnSelect func(row int)

	// editRow/editCol point at the cell whose inline editor is open (both
	// -1 = none; NewTable seeds them). editor is the live editing control
	// (the stock text field, or a column's custom Editor) while an edit is in
	// progress. editErr holds a rejected commit's validation error (nil while
	// the open value is valid or no edit is in progress). See
	// beginEdit/commitEdit/cancelEdit.
	editRow, editCol int
	editor           CellEditor
	editErr          error

	// EditActivation selects how a click begins an inline edit on an Editable
	// cell (see TableEditActivation): the zero value EditOnSingleClick keeps
	// the original "one click edits" behaviour, EditOnDoubleClick makes a
	// single click select and a double-click (or Enter on the cursor row) edit
	// -- the desktop details-view idiom -- and EditManual disables click/key
	// activation so only BeginEdit opens an editor.
	EditActivation TableEditActivation

	// OnCellEditRejected fires when a committed edit fails its column's
	// Validate rule: the Table has NOT written the value and leaves the editor
	// open. row/col name the cell, value is the rejected text, err is the
	// rule's error (its Error() is the message to surface). Nil is safe.
	OnCellEditRejected func(row, col int, value string, err error)

	// SelfSort opts the Table into sorting its own Rows on a Sortable header
	// click: it calls SortByColumn (reordering Rows in place through the
	// column's Comparator) before firing OnSort, so a host need not re-sort
	// and hand the data back. The zero value (false) keeps the original
	// content-only behaviour -- a header click only updates the indicator and
	// fires OnSort, never touching Rows.
	SelfSort bool

	// ShowSummary, when true, appends a grand-total footer line (a distinct
	// SurfaceAlt band, drawn as the LAST visual line of the body) that shows
	// each column's aggregate over every row -- blank for a column whose
	// Aggregate is AggregateNone. When GroupBy is also active, a per-group
	// summary line is additionally emitted after each (expanded) group's rows,
	// aggregating just that group's members. The footer folds into the same
	// visual-line model group headers use, so lineCount/rowAt/scrollbar stay
	// consistent. The zero value (false) is the original, pre-feature
	// behaviour: no summary line is emitted and, ungrouped, the body renders
	// byte-for-byte as it did before this field existed.
	ShowSummary bool

	// GroupBy, when in [0,len(Columns)), turns on row grouping: consecutive
	// rows sharing that column's value are gathered under a clickable group
	// header (value + member count + a disclosure triangle) that collapses
	// its members. -1 (the default, seeded by NewTable) is ungrouped -- the
	// body then renders byte-identically to a Table that never had this
	// field. Grouping assumes Rows are already ordered by the group column
	// (consecutive runs); it does not sort. Drag-to-reorder is suppressed
	// while grouped (a cross-group move has no well-defined meaning).
	GroupBy int
	// collapsed records which group values are folded shut (keyed by the
	// group column's value). Lazily created on the first toggle; a nil map
	// reads as "nothing collapsed".
	collapsed map[string]bool

	// Reorderable opts the Table into drag-to-reorder BODY rows: it makes
	// the Table both a DragSource (a press on a body row becomes a
	// draggable "tablerow:<index>" payload -- see DragData) and a
	// DropTarget for that same payload (see AcceptsDrop). The zero value
	// (false) is the original, pre-feature behaviour: DragData always
	// returns "", AcceptsDrop always returns false, and every drag event
	// (EventDragMove / EventDragLeave / EventDrop) is a no-op -- Draw and
	// OnEvent render/behave byte-identically to before this field
	// existed. Header-cell sort clicks and separator-drag resizes never
	// start a row drag regardless of this flag -- only a press that lands
	// on a BODY row does.
	Reorderable bool

	// OnReorder fires after a successful drop reorders Rows in place:
	// from is the row's index BEFORE the move, to is where it now sits.
	// Nil-guarded -- a host that doesn't care to be notified simply
	// leaves it unset.
	OnReorder func(from, to int)

	// dragRow is the body row pressed via the most recent body-row
	// EventClick, recorded only while Reorderable is true (a header or
	// separator press never touches it). It is what DragData reports as
	// the drag payload. -1 ("nothing pressed yet") mirrors how Selected
	// == -1 means "no selection" -- NewTable seeds it, and DragData
	// defensively collapses any out-of-range value to "" the same way
	// Draw collapses an out-of-range Selected to "no highlight".
	dragRow int

	// dropIndicator is the insertion index (0..len(Rows), inclusive --
	// len(Rows) means "after the last row") the in-progress drag would
	// land on, or -1 for "no drag in progress" -- driven by
	// EventDragMove / EventDragLeave and painted by Draw as a thin line
	// between body rows. NewTable seeds it to -1; only consulted while
	// Reorderable is true.
	dropIndicator int

	// resizing + resizingCol track an in-progress separator drag started
	// by a header-row EventClick on a separator hit (see
	// ColumnSeparatorAt) and cleared on EventMouseUp. resizing is a
	// separate bool (rather than resizingCol == -1) so the zero value of
	// a directly-constructed Table{} -- resizingCol == 0 -- can never be
	// mistaken for "dragging separator 0".
	resizing    bool
	resizingCol int

	// sbDrag tracks an in-progress drag of the vertical scrollbar thumb.
	sbDrag scrollDrag

	// RowDetail, when non-nil, opts each body row into an expander: a leading
	// disclosure ▶/▼ appears at the left of column 0, and clicking it toggles
	// that row's expansion. An expanded row shows a detail line (one
	// TableRowHeight-tall band, its text supplied by RowDetail(row), indented)
	// inserted right after the row via the same visual-line model group
	// headers use, so all geometry (rowAt/visualIndex/scrollbar/cellRect)
	// keeps working. Clicking the disclosure toggles expansion; a click on the
	// cell itself still selects/edits as before. The zero value (nil) is the
	// original, pre-feature behaviour: no gutter is reserved, no row can
	// expand, and Draw renders byte-for-byte as before this field existed.
	RowDetail func(row int) string

	// expanded records which body rows are currently expanded (keyed by row
	// index). Lazily created on the first toggle; a nil map reads as "nothing
	// expanded". Only meaningful while RowDetail is non-nil.
	expanded map[int]bool
}

// TableColumn is one column definition: a header title + an optional
// fixed pixel Width. A Width of 0 marks the column as "auto" -- its
// width is computed at Draw time by evenly dividing the remaining
// pixel budget among all auto columns.
type TableColumn struct {
	Title string
	Width int // pixels; 0 = auto (equal share of remaining space)
	// Align controls horizontal placement of BOTH the header title and
	// every body cell in this column. The zero value (AlignLeft) keeps
	// the original left-justified behaviour; AlignRight is the natural
	// choice for numeric columns, AlignCenter for short status flags.
	Align Align
	// Sortable opts this column into header-click sorting. The zero
	// value (false) makes a header click a no-op, so existing callers
	// that never set it keep the original passive-viewer behaviour.
	Sortable bool
	// Editable opts this column's body cells into inline editing: a click
	// on such a cell opens a text editor over it (see Table.OnCellEdit).
	// The zero value (false) keeps the original read-only behaviour.
	Editable bool
	// Aggregate selects how this column's cells are reduced to a single
	// value on a summary line (see Table.ShowSummary). The zero value
	// (AggregateNone) leaves the column blank on every summary line, so a
	// column that opts out is byte-identical to before this field existed.
	Aggregate TableAggregate

	// Editor is the per-column editor seam: when set, beginEdit calls it to
	// build the CellEditor overlaid on a cell of this column instead of the
	// stock text field, so a column can edit through a numeric field, a
	// drop-down, a date picker, ... The zero value (nil) uses the default
	// text editor, byte-identical to before this field existed. See
	// CellEditor.
	Editor func() CellEditor
	// Validate, when set, is run against an edit's proposed value at commit
	// time (Enter). A non-nil error rejects the commit: the value is NOT
	// written into Rows, the editor stays open for correction, EditError
	// reports the error, and OnCellEditRejected fires. The zero value (nil)
	// accepts every value, so a column that opts out commits exactly as
	// before. It is the toolkit's own validation.Rule shape, so the stock
	// rules (Required, MinLen, Pattern, All, ...) wire straight in.
	Validate Rule
	// Comparator orders two of this column's cell strings for SortByColumn
	// (and, on the GroupBy column, for ArrangeGroups): it returns <0 when a
	// sorts before b, 0 when equal, >0 when after. The zero value (nil) uses
	// defaultCellCompare (numeric when both cells parse as numbers, else
	// lexicographic), so a column that opts out sorts sensibly without any
	// wiring.
	Comparator func(a, b string) int
	// GroupKey derives the grouping key from a cell value when this column is
	// the GroupBy column: rows sharing a key fall in one group and the key is
	// the group header's label (e.g. a first-letter or date-bucket key). The
	// zero value (nil) groups by the raw cell value, byte-identical to before
	// this field existed.
	GroupKey func(cell string) string
	// AggregateFunc is the custom-aggregate seam: when set, it reduces the
	// column's cells over a summary line's row range to the displayed string,
	// overriding the built-in Aggregate (so a column can show a median, a
	// "min–max" span, a distinct count, ...). It receives one entry per row in
	// range (ragged rows contribute ""). The zero value (nil) uses the
	// built-in Aggregate, byte-identical to before this field existed.
	AggregateFunc func(cells []string) string
}

// TableAggregate names the reduction applied to a column's cells on a
// summary line (see TableColumn.Aggregate + Table.ShowSummary). The zero
// value AggregateNone means "no aggregate" -- the column stays blank on
// summary lines.
type TableAggregate int

const (
	// AggregateNone is the default: the column contributes nothing to a
	// summary line (it renders blank there).
	AggregateNone TableAggregate = iota
	// AggregateSum totals the column's numeric cells (non-numeric cells are
	// skipped; strconv.ParseFloat decides what parses).
	AggregateSum
	// AggregateAvg is the mean of the column's numeric cells.
	AggregateAvg
	// AggregateCount is the number of rows covered by the summary line,
	// regardless of whether their cells are numeric.
	AggregateCount
	// AggregateMin is the smallest of the column's numeric cells.
	AggregateMin
	// AggregateMax is the largest of the column's numeric cells.
	AggregateMax
)

// TableEditActivation selects how a click (and Enter) begins an inline edit on
// an Editable cell. It only governs how an edit STARTS; commit/cancel and the
// editor itself are unchanged across the modes.
type TableEditActivation int

const (
	// EditOnSingleClick is the default: a single click on an Editable cell
	// opens its editor immediately, byte-identical to before this field
	// existed.
	EditOnSingleClick TableEditActivation = iota
	// EditOnDoubleClick makes a single click select the cell's row and a
	// double-click (an EventClick tagged Code == TableDoubleClick) open the
	// editor -- the desktop details-view rename idiom. In this mode Enter on
	// the cursor row also opens the first Editable column's editor.
	EditOnDoubleClick
	// EditManual disables click- and key-driven activation entirely: an
	// Editable cell edits only when the host calls BeginEdit.
	EditManual
)

// TableDoubleClick is the Event.Code a host tags onto an EventClick to mark it
// a double-click, so a Table in EditOnDoubleClick mode can tell the second
// click of a double-click from a fresh single click without the toolkit
// growing a dedicated double-click EventKind.
const TableDoubleClick = "double"

// CellEditor is the editing control a Table overlays on a cell while an inline
// edit is in progress. The default is a text field (see newTextCellEditor);
// TableColumn.Editor is the per-column seam that swaps in another control -- a
// numeric field, a drop-down, a date picker. A CellEditor is a Widget (so the
// Table sizes and draws it over the cell) plus the four hooks the commit
// machinery needs.
type CellEditor interface {
	Widget
	// CellValue returns the editor's current text, read when the edit commits
	// and written back into the row.
	CellValue() string
	// SetCellValue seeds the editor with the cell's current text when the edit
	// opens.
	SetCellValue(string)
	// OnCellSubmit registers the callback the editor fires when the user
	// accepts the value (e.g. Enter) so the Table commits the edit.
	OnCellSubmit(func())
	// Focus gives (true) or removes (false) keyboard focus.
	Focus(bool)
}

// textCellEditor adapts the stock Entry to the CellEditor seam. It embeds the
// *Entry so the Widget methods (Bounds/SetBounds/Draw/HitTest/OnEvent) and the
// editing behaviour are exactly the Entry's -- the default editor is
// byte-identical to the pre-seam inline editor.
type textCellEditor struct{ *Entry }

func (e textCellEditor) CellValue() string     { return e.Text }
func (e textCellEditor) SetCellValue(s string) { e.Text = s; e.Cursor = len([]rune(s)) }
func (e textCellEditor) OnCellSubmit(fn func()) {
	e.Entry.OnSubmit = func(string) { fn() }
}
func (e textCellEditor) Focus(b bool) { e.SetFocused(b) }

// newTextCellEditor builds the default text editor for cell (row,col), seeded
// with val and inheriting the Table's font so complex text edits legibly.
func (t *Table) newTextCellEditor(val string) CellEditor {
	en := NewEntry(val)
	en.Font = t.Font
	return textCellEditor{en}
}

// tableErrorBorder is the stroke colour of an editing cell whose pending value
// failed validation -- the same brick red the Alert widget's error face uses,
// so a rejected edit reads as an error consistently across the toolkit.
var tableErrorBorder = RGB(0xC0, 0x30, 0x30)

// TableIconFunc paints a Table's optional per-row leading icon into the
// square rect r using ink. It is deliberately the exact signature of the
// stock DrawIconXxx painters in icons.go (DrawIconOpen, DrawIconNew,
// ...), so a caller wires any of those straight into Table.RowIcon --
// e.g. `func(row int) (toolkit.TableIconFunc, bool) { return
// toolkit.DrawIconOpen, true }` -- or supplies its own painter of the
// same shape for a file-type glyph the stock set doesn't cover.
type TableIconFunc func(p painter.Painter, r Rect, ink RGBA)

// TableIconSize is the pixel width+height of the square rect a per-row
// leading icon (see Table.RowIcon) is painted into -- sized to sit
// comfortably inside a TableRowHeight-tall body row with a little
// vertical breathing room.
const TableIconSize = 16

// TableHeaderHeight is the pixel height of the header row.
const TableHeaderHeight = 24

// TableRowHeight is the pixel height of one body row.
const TableRowHeight = 22

// TableCellPadX is the left/right pixel padding applied inside every
// header + body cell before its text lands.
const TableCellPadX = 4

// tableEmptyPlaceholder is the label rendered under the header when
// Rows is empty. Split into a constant so tests can assert width
// without hard-coding the string in two places.
const tableEmptyPlaceholder = "(no data)"

// NewTable builds a Table with the given columns + rows. Selected
// starts at -1 (no row selected) so a freshly constructed Table
// renders with plain zebra striping.
func NewTable(cols []TableColumn, rows [][]string) *Table {
	return &Table{
		Columns:       cols,
		Rows:          rows,
		Selected:      -1,
		SortColumn:    -1,
		dragRow:       -1,
		dropIndicator: -1,
		editRow:       -1,
		editCol:       -1,
		GroupBy:       -1,
	}
}

// Table is both a DragSource and a DropTarget for its own body rows --
// see Reorderable, DragData + AcceptsDrop.
var (
	_ DragSource = (*Table)(nil)
	_ DropTarget = (*Table)(nil)
)

// tableRowDragPrefix scopes a Table's drag-to-reorder payload to a
// private scheme so a Table never mistakes a foreign drag (e.g. a
// DropZone file path) for one of its own rows, and vice versa.
const tableRowDragPrefix = "tablerow:"

// DragData reports the drag payload for the body row currently pressed
// (see dragRow) -- "tablerow:<index>" -- while Reorderable is true and
// a body row was actually the most recent press; otherwise "". A stale
// dragRow left over after Rows shrinks collapses to "" the same
// defensive way Draw collapses an out-of-range Selected.
func (t *Table) DragData() string {
	if !t.reorderActive() || t.dragRow < 0 || t.dragRow >= len(t.Rows) {
		return ""
	}
	return tableRowDragPrefix + strconv.Itoa(t.dragRow)
}

// AcceptsDrop reports whether payload is one of this Table's own
// "tablerow:" drags -- true only while Reorderable is true AND the
// payload parses as a well-formed tablerow payload. A foreign payload
// (a different scheme, or garbage) is always rejected, including while
// Reorderable is false.
func (t *Table) AcceptsDrop(payload string) bool {
	if !t.reorderActive() {
		return false
	}
	_, ok := parseTableRowDragPayload(payload)
	return ok
}

// parseTableRowDragPayload extracts the source row index from a
// "tablerow:<index>" payload. ok is false for any payload that isn't
// that exact scheme or whose index isn't a non-negative integer -- the
// single choke point EventDrop + AcceptsDrop both use so a garbage or
// foreign payload is rejected identically by both.
func parseTableRowDragPayload(payload string) (int, bool) {
	rest, ok := strings.CutPrefix(payload, tableRowDragPrefix)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// Draw paints the header + body + column separators through p using
// theme's palette. Widths for auto columns are computed here, so
// resizing the widget's Bounds() between frames re-flows the columns
// automatically.
func (t *Table) Draw(p painter.Painter, theme *Theme) {
	r := t.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	overflow := t.bodyOverflows()
	widths := t.columnWidths(t.contentWidth())
	// gutter is the leading space reserved inside column 0 for a row's
	// disclosure chevron (RowDetail) and per-row icon (RowIcon); 0 (so every
	// gutter branch below is inert) while both are nil -- keeping that path
	// byte-identical to before either feature existed.
	gutter := t.leadGutter()

	// --- Header row ------------------------------------------------
	fillRect(p, r.X, r.Y, r.W, TableHeaderHeight, theme.SurfaceAlt)
	// 1-px bottom-edge stroke separates the header from the body.
	fillRect(p, r.X, r.Y+TableHeaderHeight-1, r.W, 1, theme.Border)
	// Header cell titles. sortCol collapses an out-of-range SortColumn
	// to -1, mirroring how Draw resolves Selected below -- an
	// unconstructed or stale Table never crashes drawing an indicator
	// on a column that no longer exists.
	sortCol := -1
	if t.SortColumn >= 0 && t.SortColumn < len(t.Columns) {
		sortCol = t.SortColumn
	}
	hty := r.Y + (TableHeaderHeight-t.glyphHeight())/2
	hmid := r.Y + TableHeaderHeight/2
	if !t.hScrollable() {
		hx := r.X
		for i := range t.Columns {
			t.paintHeaderCell(p, theme, widths, hx, hty, hmid, i, sortCol)
			hx += widths[i]
		}
	} else {
		// Frozen/scrolled split: scrolled headers clipped to the region
		// right of the frozen block, then frozen headers painted at their
		// pinned positions.
		f := t.frozenCount()
		fx := r.X + t.frozenWidth(widths)
		right := r.X + t.contentWidth()
		withClip(p, Rect{X: fx, Y: r.Y, W: right - fx, H: TableHeaderHeight}, func() {
			for i := f; i < len(t.Columns); i++ {
				t.paintHeaderCell(p, theme, widths, t.columnScreenX(r.X, widths, i), hty, hmid, i, sortCol)
			}
		})
		for i := 0; i < f; i++ {
			t.paintHeaderCell(p, theme, widths, t.columnScreenX(r.X, widths, i), hty, hmid, i, sortCol)
		}
	}

	// --- Body ------------------------------------------------------
	bodyY := r.Y + TableHeaderHeight
	if len(t.Rows) == 0 {
		// "(no data)" centred horizontally within the widget, sitting
		// one TableRowHeight below the header.
		tw := t.textWidth(tableEmptyPlaceholder)
		tx := r.X + (r.W-tw)/2
		ty := bodyY + (TableRowHeight-t.glyphHeight())/2
		t.drawText(p, tx, ty, tableEmptyPlaceholder, theme.OnSurface)
		return
	}
	// Resolve which body row is highlighted -- Selected out of range
	// collapses to -1 so the loop below never enters the accent branch
	// for a bogus index.
	selRow := -1
	if t.Selected >= 0 && t.Selected < len(t.Rows) {
		selRow = t.Selected
	}
	onAccent := accentInk(theme)
	// scroll is the top visible row, defensively re-collapsed from
	// ScrollRow exactly like selRow/sortCol above. end is one past the
	// last row the current body height can show; when every row fits
	// (the common, pre-feature case) end == len(t.Rows) and scroll == 0,
	// so this loop paints identically to the un-windowed original.
	scroll := t.clampScrollRow()
	end := scroll + t.bodyVisibleRows()
	if end > t.lineCount() {
		end = t.lineCount()
	}
	// Clip the body to its own rect before painting rows. bodyVisibleRows
	// rounds UP, so when Bounds().H isn't an exact multiple of
	// TableRowHeight the last windowed row is only partially inside the
	// widget -- without a clip its background/text would spill past
	// r.Y+r.H onto whatever the host draws below this widget, and (once
	// the scrollbar is painted afterwards) would already have overdrawn
	// where the thumb belongs. Back-ends that can't clip just render
	// unclipped, the same graceful degradation ScrollView already relies
	// on for its child.
	clr, canClip := p.(painter.Clipper)
	if canClip {
		clr.PushClip(Rect{X: r.X, Y: bodyY, W: r.W, H: r.Y + r.H - bodyY})
	}
	if t.useLineModel() {
		// Line-model body: walk the visual-line list, painting a group header
		// band for header lines, an aggregate footer band for summary lines, an
		// indented detail band for detail lines, and a normal cell row (via the
		// shared drawDataRow) for data lines. end is already clamped to
		// lineCount().
		lines := t.lines()
		for vi := scroll; vi < end; vi++ {
			ln := lines[vi]
			y := bodyY + (vi-scroll)*TableRowHeight
			switch {
			case ln.header:
				t.drawGroupHeader(p, theme, r, y, ln)
			case ln.summary:
				t.drawSummaryRow(p, theme, r, widths, gutter, y, ln)
			case ln.detail:
				t.drawDetailRow(p, theme, r, y, ln)
			default:
				t.drawDataRow(p, theme, r, widths, gutter, y, ln.dataRow, selRow, onAccent)
			}
		}
	} else {
		// Ungrouped body: one line per row, on-screen position == index.
		// Byte-identical to the pre-grouping loop.
		for i := scroll; i < end; i++ {
			y := bodyY + (i-scroll)*TableRowHeight
			t.drawDataRow(p, theme, r, widths, gutter, y, i, selRow, onAccent)
		}
	}
	// Drag-to-reorder insertion indicator: a thin Accent-coloured line
	// between body rows at dropIndicator, painted only while a
	// Reorderable drag is actually in progress (dropIndicator >= 0 --
	// EventDragLeave / a fresh drop reset it to -1, and NewTable seeds
	// it there too). Drawn inside the same clip as the body rows so it
	// never spills past Bounds() any more than a row itself would.
	if t.reorderActive() && t.dropIndicator >= 0 && t.dropIndicator <= len(t.Rows) {
		iy := bodyY + (t.dropIndicator-scroll)*TableRowHeight
		fillRect(p, r.X, iy-1, r.W, 2, theme.Accent)
	}
	if canClip {
		clr.PopClip()
	}

	// --- Column separators ----------------------------------------
	// One 1-px vertical stroke between adjacent columns, spanning the
	// full widget height (header + body). No stroke on the outer left
	// or right edge -- the widget's parent frame owns those.
	if !t.hScrollable() {
		sepX := r.X
		for i := 0; i < len(t.Columns)-1; i++ {
			sepX += widths[i]
			fillRect(p, sepX, r.Y, 1, r.H, theme.Border)
		}
	} else {
		// Frozen separators stay pinned; scrolled separators shift with the
		// content and are clipped to the scrollable region; a slightly
		// stronger stroke marks the frozen/scrolled boundary.
		f := t.frozenCount()
		fx := r.X + t.frozenWidth(widths)
		right := r.X + t.contentWidth()
		sepX := r.X
		for i := 0; i < f-1; i++ {
			sepX += widths[i]
			fillRect(p, sepX, r.Y, 1, r.H, theme.Border)
		}
		withClip(p, Rect{X: fx, Y: r.Y, W: right - fx, H: r.H}, func() {
			for i := f; i < len(t.Columns)-1; i++ {
				fillRect(p, t.columnScreenX(r.X, widths, i+1), r.Y, 1, r.H, theme.Border)
			}
		})
		if f > 0 {
			fillRect(p, fx, r.Y, 1, r.H, theme.Border)
		}
	}

	// --- Vertical scrollbar (right edge, body only) -----------------
	// Only drawn while the body actually overflows -- with all rows
	// fitting (the byte-identical case) this is skipped entirely, same
	// as the pre-feature Table never painting one.
	if overflow {
		t.drawScrollbar(p, theme, r)
	}

	// --- Horizontal scrollbar (bottom edge) -------------------------
	// Drawn only while the columns overflow the viewport (hScrollable);
	// otherwise skipped entirely, so a fit-to-width table is unchanged.
	if t.hScrollable() {
		t.drawHScrollbar(p, theme, r, widths)
	}

	// Inline cell editor overlay, on top of everything: when an edit is in
	// progress (beginEdit) the Entry is positioned over its cell via cellRect
	// -- the same geometry Draw laid the cell text out with -- and drawn last
	// so it covers the underlying (now stale) cell content and the cursor is
	// visible. cellRect tracks ScrollRow, so scrolling moves the editor with
	// its row (off-view rows simply draw outside the buffer, harmlessly).
	if t.editRow >= 0 && t.editor != nil {
		if vi := t.visualIndex(t.editRow); vi >= 0 {
			rc := t.cellRect(t.editRow, t.editCol)
			t.editor.SetBounds(rc)
			t.editor.Draw(p, theme)
			// A rejected (failing-validation) pending value rings the editing
			// cell in the error colour so the reason it will not commit is
			// visible without leaving the cell.
			if t.editErr != nil {
				strokeRect(p, rc.X, rc.Y, rc.W, rc.H, tableErrorBorder)
			}
		}
	}
	t.drawFocusRing(p, theme, r)
}

// drawDataRow paints one body data row (background + zebra/selection tint +
// per-column text and optional leading icon) at on-surface y for data-row
// index i. It is the shared cell-painting body of Draw, called by both the
// grouped and ungrouped loops so the two render rows identically.
func (t *Table) drawDataRow(p painter.Painter, theme *Theme, r Rect, widths []int, gutter, y, i, selRow int, onAccent RGBA) {
	row := t.Rows[i]
	bg := theme.Surface
	ink := theme.OnSurface
	// Highlighted covers both the single-row anchor (always) and, while
	// MultiSelect is on, any row in the multi-row selection set. With
	// MultiSelect false the second half short-circuits, so this collapses to
	// exactly the pre-MultiSelect condition.
	highlighted := i == selRow || (t.MultiSelect && t.IsRowSelected(i))
	switch {
	case highlighted:
		bg = theme.Accent
		ink = onAccent
	case i%2 == 1:
		// Zebra keyed on the row's ABSOLUTE index i (not its on-screen
		// position) so scrolling never shifts which rows read as odd/even.
		bg = theme.Background
	}
	fillRect(p, r.X, y, r.W, TableRowHeight, bg)
	cty := y + (TableRowHeight-t.glyphHeight())/2
	if !t.hScrollable() {
		cx := r.X
		for j := range t.Columns {
			t.paintCell(p, widths, gutter, cx, cty, y, i, j, ink, row)
			cx += widths[j]
		}
		return
	}
	// Frozen/scrolled split: paint scrolled cells clipped to the region
	// right of the frozen block (so they never bleed over the pinned
	// columns or the vertical-scrollbar gutter), then the frozen cells at
	// their pinned positions on top.
	f := t.frozenCount()
	fx := r.X + t.frozenWidth(widths)
	right := r.X + t.contentWidth()
	withClip(p, Rect{X: fx, Y: y, W: right - fx, H: TableRowHeight}, func() {
		for j := f; j < len(t.Columns); j++ {
			t.paintCell(p, widths, gutter, t.columnScreenX(r.X, widths, j), cty, y, i, j, ink, row)
		}
	})
	for j := 0; j < f; j++ {
		t.paintCell(p, widths, gutter, t.columnScreenX(r.X, widths, j), cty, y, i, j, ink, row)
	}
}

// paintCell paints one body cell of row i, column j: column 0's optional
// leading disclosure chevron (RowDetail) and per-row icon (RowIcon), then the
// cell text, left edge at cellX. The leading gutter is laid out
// disclosure-then-icon, each carved from the left of column 0; gutter is the
// combined width (t.leadGutter()) so cellW shrinks by exactly the reserved
// space. Shared by the plain and frozen/scrolled body passes. With neither
// feature set gutter is 0 and this renders byte-identically to before; with
// only RowIcon set the disclosure branch is inert, leaving the icon layout
// unchanged.
func (t *Table) paintCell(p painter.Painter, widths []int, gutter, cellX, cty, y, i, j int, ink RGBA, row []string) {
	cellW := widths[j]
	if j == 0 && gutter > 0 {
		if dg := t.disclosureGutter(); dg > 0 {
			drawDisclosureChevron(p, cellX+TableCellPadX+4, y+TableRowHeight/2, t.expanded[i], ink)
			cellX += dg
		}
		if t.iconGutter() > 0 {
			if draw, ok := t.resolveRowIcon(i); ok {
				iconY := y + (TableRowHeight-TableIconSize)/2
				draw(p, Rect{X: cellX + TableCellPadX, Y: iconY, W: TableIconSize, H: TableIconSize}, ink)
			}
			cellX += t.iconGutter()
		}
		cellW -= gutter
	}
	if j < len(row) {
		t.drawText(p, cellTextX(&t.Base, cellX, cellW, row[j], t.Columns[j].Align), cty, row[j], ink)
	}
}

// paintHeaderCell paints one header cell of column i: its title (aligned) and,
// when it is the sorted column, the ▲/▼ indicator. Shared by the plain and
// frozen/scrolled header passes.
func (t *Table) paintHeaderCell(p painter.Painter, theme *Theme, widths []int, x, hty, hmid, i, sortCol int) {
	col := t.Columns[i]
	t.drawText(p, cellTextX(&t.Base, x, widths[i], col.Title, col.Align), hty, col.Title, theme.OnBackground)
	if i == sortCol {
		drawSortIndicator(p, x+widths[i]-8, hmid, t.SortAsc, theme.OnBackground)
	}
}

// withClip runs fn with clip rect pushed onto p when p supports clipping,
// otherwise runs fn unclipped -- the same graceful degradation the body-row
// clip already relies on for painters that can't clip.
func withClip(p painter.Painter, clip Rect, fn func()) {
	if clr, ok := p.(painter.Clipper); ok {
		clr.PushClip(clip)
		fn()
		clr.PopClip()
	} else {
		fn()
	}
}

// drawGroupHeader paints a group-header band at on-surface y: a SurfaceAlt
// stripe spanning the width, a disclosure chevron (▼ open / ▶ collapsed) and
// the group value with its member count, "value (n)".
func (t *Table) drawGroupHeader(p painter.Painter, theme *Theme, r Rect, y int, ln tableLine) {
	fillRect(p, r.X, y, r.W, TableRowHeight, theme.SurfaceAlt)
	chevX := r.X + TableCellPadX + 4
	drawDisclosureChevron(p, chevX, y+TableRowHeight/2, !ln.collapsed, theme.OnSurface)
	tx := r.X + TableCellPadX + TableIconSize
	ty := y + (TableRowHeight-t.glyphHeight())/2
	t.drawText(p, tx, ty, ln.group+" ("+strconv.Itoa(ln.count)+")", theme.OnSurface)
}

// drawSummaryRow paints an aggregate summary band at on-surface y: a distinct
// SurfaceAlt stripe spanning the width, then each column's aggregate text
// (blank for AggregateNone, aligned like the column's cells) over the rows the
// line covers ([sumStart,sumEnd)). It mirrors drawDataRow's frozen/scrolled
// geometry so a summary line lines up with the data columns under horizontal
// scroll, and honours the same column-0 icon gutter so its first cell aligns
// with the rows above.
func (t *Table) drawSummaryRow(p painter.Painter, theme *Theme, r Rect, widths []int, gutter, y int, ln tableLine) {
	fillRect(p, r.X, y, r.W, TableRowHeight, theme.SurfaceAlt)
	cty := y + (TableRowHeight-t.glyphHeight())/2
	paint := func(j, cellX int) {
		text := t.aggregate(t.Columns[j].Aggregate, j, ln.sumStart, ln.sumEnd)
		if text == "" {
			return
		}
		cellW := widths[j]
		if j == 0 && gutter > 0 {
			cellX += gutter
			cellW -= gutter
		}
		t.drawText(p, cellTextX(&t.Base, cellX, cellW, text, t.Columns[j].Align), cty, text, theme.OnSurface)
	}
	if !t.hScrollable() {
		cx := r.X
		for j := range t.Columns {
			paint(j, cx)
			cx += widths[j]
		}
		return
	}
	f := t.frozenCount()
	fx := r.X + t.frozenWidth(widths)
	right := r.X + t.contentWidth()
	withClip(p, Rect{X: fx, Y: y, W: right - fx, H: TableRowHeight}, func() {
		for j := f; j < len(t.Columns); j++ {
			paint(j, t.columnScreenX(r.X, widths, j))
		}
	})
	for j := 0; j < f; j++ {
		paint(j, t.columnScreenX(r.X, widths, j))
	}
}

// drawDetailRow paints an expander detail band at on-surface y: a distinct
// SurfaceAlt stripe spanning the width, then the row's detail text (from
// RowDetail) indented past the leading gutter so it reads as nested under its
// parent row. Reached only for detail lines, which lines() emits solely while
// RowDetail is non-nil, so RowDetail is safe to call here.
func (t *Table) drawDetailRow(p painter.Painter, theme *Theme, r Rect, y int, ln tableLine) {
	fillRect(p, r.X, y, r.W, TableRowHeight, theme.SurfaceAlt)
	tx := r.X + t.leadGutter() + TableCellPadX
	ty := y + (TableRowHeight-t.glyphHeight())/2
	t.drawText(p, tx, ty, t.RowDetail(ln.dataRow), theme.OnSurface)
}

// tableScrollbarThumbMin is the pixel floor a scrollbar thumb is
// clamped to, mirroring ScrollView's own floor so a huge row count
// never shrinks the thumb into an unclickable sliver.
const tableScrollbarThumbMin = 8

// drawScrollbar paints the right-edge scrollbar track + thumb over the
// body rows -- the header sits above it and never scrolls, so the
// track spans only [bodyY, r.Y+r.H). It reuses ScrollView.Draw's exact
// pixel-proportion formula for the vertical thumb, substituting "rows
// converted to pixels" (len(t.Rows)*TableRowHeight, scroll*TableRowHeight)
// for ScrollView's arbitrary child content height/offset. Only called
// by Draw while bodyOverflows() is true, which guarantees trackH > 0
// and contentH > trackH (see bodyOverflows), so no defensive
// zero/negative-denominator guard is needed here.
func (t *Table) drawScrollbar(p painter.Painter, theme *Theme, r Rect) {
	// The caller only invokes this while the body overflows, which is exactly
	// when scrollbarGeom reports live, so the ok flag is always true here.
	g, _ := t.scrollbarGeom()
	fillRect(p, r.X+g.cross0, r.Y+g.trackStart, scrollbarWidth, g.trackLen, theme.SurfaceAlt)
	fillRect(p, r.X+g.cross0, r.Y+g.thumbStart, scrollbarWidth, g.thumbLen, theme.Accent)
}

// scrollbarGeom returns the vertical scrollbar's widget-local geometry and
// whether it is live (the body overflows). The header never scrolls, so the
// track spans only [TableHeaderHeight, r.H). It is the single definition of the
// track + thumb shared by drawScrollbar and OnEvent. The thumb is sized + placed
// in the visual-line pixel space (lineCount()*TableRowHeight) so groups + detail
// rows count toward the extent, exactly as the previous inline math did; the
// scroll value the thumb travel maps to is a body ScrollRow, clamped to
// maxScrollRow(). bodyOverflows() guarantees contentH > trackH > 0, so the
// travelDen below is positive.
func (t *Table) scrollbarGeom() (sbGeom, bool) {
	if !t.bodyOverflows() {
		return sbGeom{}, false
	}
	r := t.Bounds()
	trackTop := TableHeaderHeight
	trackH := r.H - TableHeaderHeight
	contentH := t.lineCount() * TableRowHeight
	thumbH := trackH * trackH / contentH
	if thumbH < tableScrollbarThumbMin {
		thumbH = tableScrollbarThumbMin
	}
	scroll := t.clampScrollRow()
	return sbGeom{
		cross0:     r.W - scrollbarWidth,
		trackStart: trackTop,
		trackLen:   trackH,
		thumbStart: trackTop + scroll*TableRowHeight*(trackH-thumbH)/(contentH-trackH),
		thumbLen:   thumbH,
		travelNum:  TableRowHeight * (trackH - thumbH),
		travelDen:  contentH - trackH,
		maxScroll:  t.maxScrollRow(),
	}, true
}

// drawHScrollbar paints the bottom-edge horizontal scrollbar over the
// scrollable (non-frozen) region: the track spans [frozenWidth, contentWidth)
// and the thumb is sized/positioned by the same pixel-proportion formula as
// the vertical scrollbar, substituting the scrollable content/viewport widths
// for the row-pixel heights. Only called while hScrollable, which guarantees
// contentW > trackW (naturalWidth > contentWidth) so the denominator below is
// positive.
func (t *Table) drawHScrollbar(p painter.Painter, theme *Theme, r Rect, widths []int) {
	fw := t.frozenWidth(widths)
	trackX := r.X + fw
	trackY := r.Y + r.H - scrollbarWidth
	trackW := t.contentWidth() - fw
	fillRect(p, trackX, trackY, trackW, scrollbarWidth, theme.SurfaceAlt)
	contentW := t.naturalWidth() - fw
	thumbW := trackW * trackW / contentW
	if thumbW < tableScrollbarThumbMin {
		thumbW = tableScrollbarThumbMin
	}
	thumbX := trackX + t.clampScrollX()*(trackW-thumbW)/(contentW-trackW)
	fillRect(p, thumbX, trackY, thumbW, scrollbarWidth, theme.Accent)
}

// columnWidths distributes the total pixel budget across every column.
// Fixed-Width columns take exactly their declared width; auto
// (Width == 0) columns split the remainder equally, with any integer
// remainder pushed onto the last auto column so all widths sum to
// total. If there are no auto columns the widths returned are simply
// the declared Widths -- they may exceed or fall short of total, but
// the painter's clipping keeps that safe.
func (t *Table) columnWidths(total int) []int {
	n := len(t.Columns)
	if n == 0 {
		return nil
	}
	widths := make([]int, n)
	fixedTotal := 0
	autoCount := 0
	lastAutoIdx := -1
	for i, col := range t.Columns {
		if col.Width > 0 {
			widths[i] = col.Width
			fixedTotal += col.Width
		} else {
			autoCount++
			lastAutoIdx = i
		}
	}
	if autoCount == 0 {
		return widths
	}
	remaining := total - fixedTotal
	if remaining < 0 {
		remaining = 0
	}
	share := remaining / autoCount
	for i, col := range t.Columns {
		if col.Width <= 0 {
			widths[i] = share
		}
	}
	// Push integer-division leftover onto the last auto column so
	// the sum of widths equals total (only reachable when there is
	// budget left over after the fixed columns).
	sum := 0
	for _, w := range widths {
		sum += w
	}
	widths[lastAutoIdx] += total - sum
	if widths[lastAutoIdx] < 0 {
		widths[lastAutoIdx] = 0
	}
	return widths
}

// cellTextX returns the x at which to start drawing text of the given
// string inside a cell whose left edge is cellX and whose width is
// cellW, honouring align. TableCellPadX is reserved on both the left
// (AlignLeft) and right (AlignRight) edges; AlignCenter ignores the
// padding and centres within the full cell. The result is clamped to
// the left padding so text never starts before the cell's inner edge.
//
// b supplies the effective font so right/centre alignment measures the
// text in the Table's own font (b.textWidth) rather than the global one.
func cellTextX(b *Base, cellX, cellW int, text string, align Align) int {
	switch align {
	case AlignRight:
		x := cellX + cellW - TableCellPadX - b.textWidth(text)
		if min := cellX + TableCellPadX; x < min {
			x = min
		}
		return x
	case AlignCenter:
		x := cellX + (cellW-b.textWidth(text))/2
		if min := cellX + TableCellPadX; x < min {
			x = min
		}
		return x
	default: // AlignLeft
		return cellX + TableCellPadX
	}
}

// iconGutter is the leading pixel gutter reserved inside the first
// column for a per-row icon: TableCellPadX (the icon's own left inset,
// matching a cell's normal text inset) plus TableIconSize. It is 0 --
// and every gutter-aware branch in Draw a no-op -- while RowIcon is nil,
// which is what keeps the no-icon render byte-identical to before this
// feature existed.
func (t *Table) iconGutter() int {
	if t.RowIcon == nil {
		return 0
	}
	return TableCellPadX + TableIconSize
}

// disclosureGutter is the leading pixel gutter reserved inside the first
// column for a row's expander disclosure chevron: the same TableCellPadX +
// TableIconSize a per-row icon claims. It is 0 -- and every disclosure branch
// inert -- while RowDetail is nil, keeping the no-expander render
// byte-identical to before this feature existed.
func (t *Table) disclosureGutter() int {
	if t.RowDetail == nil {
		return 0
	}
	return TableCellPadX + TableIconSize
}

// leadGutter is the total leading gutter carved from column 0: the disclosure
// chevron's gutter (RowDetail) followed by the per-row icon's (RowIcon). Draw
// reserves it once and threads it through drawDataRow/drawSummaryRow/cellRect
// so cells, summaries and the inline editor all shift right by the same space.
// With neither feature set it is 0, the byte-identical pre-feature layout.
func (t *Table) leadGutter() int {
	return t.disclosureGutter() + t.iconGutter()
}

// resolveRowIcon reports the icon painter for body row i, collapsing
// "no icon for this row" to (nil, false) the same defensive way Draw
// collapses an out-of-range Selected/ScrollRow: a caller's RowIcon that
// returns ok == false, OR one that returns a nil painter, both yield no
// icon (the gutter is still reserved by iconGutter regardless, so the
// column stays aligned). It is only ever called while RowIcon != nil
// (Draw guards on iconGutter() > 0), so it dereferences RowIcon
// directly.
func (t *Table) resolveRowIcon(i int) (TableIconFunc, bool) {
	draw, ok := t.RowIcon(i)
	if !ok || draw == nil {
		return nil, false
	}
	return draw, true
}

// accentInk returns the ink colour to draw ON a Theme.Accent field.
// The GTK loader may populate theme.Extra["OnAccent"] with the
// theme's canonical accent-inverted colour; if absent we fall back
// to theme.Background, matching what Button + ListBox + TreeView
// already do for their selected/pressed accent branches.
func accentInk(theme *Theme) RGBA {
	if theme.Extra != nil {
		if c, ok := theme.Extra["OnAccent"]; ok {
			return c
		}
	}
	return theme.Background
}

// drawSortIndicator paints a small 5-px-tall ▲ (ascending) / ▼
// (descending) triangle centred on (cx, cy), using the same
// row-by-row fillRect technique as Expander/Accordion's disclosure
// chevron so the glyph reads consistently across the toolkit.
func drawSortIndicator(p painter.Painter, cx, cy int, ascending bool, ink RGBA) {
	if ascending {
		// ▲ : narrow tip at the top, widening to the flat bottom row.
		for t := 0; t < 5; t++ {
			fillRect(p, cx-t, cy-2+t, 1+2*t, 1, ink)
		}
		return
	}
	// ▼ : flat top (widest row), point at bottom (narrow tip) --
	// identical shape to drawDisclosureChevron's expanded state.
	for t := 0; t < 5; t++ {
		fillRect(p, cx-t, cy+2-t, 1+2*t, 1, ink)
	}
}

// tableMinColumnWidth is the floor SetColumnWidth clamps to -- small
// enough to still show a sliver of a cell, large enough that a column
// can never be dragged into (or past) zero width.
const tableMinColumnWidth = 20

// tableSeparatorHitTolerance is the +/- pixel band around a column
// separator's exact x that still counts as a hit, mirroring the
// forgiving hit-test every pointer-driven drag handle needs (an exact
// 1-px target is unusable with a mouse).
const tableSeparatorHitTolerance = 3

// columnAt returns the index of the column whose cell spans localX (a
// Table-local x coordinate), or -1 if localX falls outside every
// column -- e.g. an empty Columns slice or a localX past the last
// column's right edge when the fixed-Width columns overflow the
// widget's Bounds().
func (t *Table) columnAt(localX int) int {
	widths := t.columnWidths(t.contentWidth())
	if !t.hScrollable() {
		x := 0
		for i, w := range widths {
			if localX >= x && localX < x+w {
				return i
			}
			x += w
		}
		return -1
	}
	// A click left of the frozen block hit-tests the frozen columns at their
	// natural (pinned) offset [0, f); a click in the scrollable region is
	// shifted by clampScrollX back into natural space and hit-tests the
	// non-frozen columns [f, n). Both walk the same cumulative natural x.
	f := t.frozenCount()
	fw := t.frozenWidth(widths)
	nat, lo := localX, 0
	if localX >= fw {
		nat, lo = localX+t.clampScrollX(), f
	}
	x := 0
	for i := 0; i < len(widths); i++ {
		if i >= lo && nat >= x && nat < x+widths[i] {
			return i
		}
		x += widths[i]
	}
	return -1
}

// ColumnSeparatorAt returns the 0-based index of the separator under
// localX (a Table-local x coordinate) -- the separator between column
// i and column i+1 -- within tableSeparatorHitTolerance pixels, or -1
// if localX is not near any separator. A single-column (or empty)
// Table has no separators and always returns -1.
func (t *Table) ColumnSeparatorAt(localX int) int {
	if len(t.Columns) < 2 {
		return -1
	}
	widths := t.columnWidths(t.contentWidth())
	if !t.hScrollable() {
		x := 0
		for i := 0; i < len(widths)-1; i++ {
			x += widths[i]
			if localX >= x-tableSeparatorHitTolerance && localX <= x+tableSeparatorHitTolerance {
				return i
			}
		}
		return -1
	}
	// The separator after column i sits at column i+1's on-screen left edge;
	// a scrolled separator is only hittable while it lies within the visible
	// scrollable region [frozenWidth, contentWidth).
	fw := t.frozenWidth(widths)
	right := t.contentWidth()
	for i := 0; i < len(widths)-1; i++ {
		sx := t.columnScreenX(0, widths, i+1)
		if i+1 > t.frozenCount() && (sx < fw || sx > right) {
			continue
		}
		if localX >= sx-tableSeparatorHitTolerance && localX <= sx+tableSeparatorHitTolerance {
			return i
		}
	}
	return -1
}

// SetColumnWidth pins column col to a fixed pixel width w (clamped to
// tableMinColumnWidth), then fires OnColumnResize with the clamped
// value. Like a Paned's MoveHandle, this is the direct, host-callable
// entry point a drag handler (internal or external) drives; an
// out-of-range col is a no-op. Setting a width converts an "auto"
// column into a fixed one, exactly as dragging a Paned's handle turns
// its 50/50 default into an explicit Position.
func (t *Table) SetColumnWidth(col, w int) {
	if col < 0 || col >= len(t.Columns) {
		return
	}
	if w < tableMinColumnWidth {
		w = tableMinColumnWidth
	}
	t.Columns[col].Width = w
	if t.OnColumnResize != nil {
		t.OnColumnResize(col, w)
	}
}

// bodyVisibleRows is how many body-row slots the widget's current
// Bounds().H offers below the fixed header, rounded UP -- the last
// slot may show only a partial row rather than leave a gap when the
// height isn't an exact multiple of TableRowHeight. Draw clips the
// body to Bounds() so that partial row never paints past the widget's
// own edge (see the PushClip call in Draw).
func (t *Table) bodyVisibleRows() int {
	h := t.Bounds().H - TableHeaderHeight
	if h <= 0 {
		return 0
	}
	return (h + TableRowHeight - 1) / TableRowHeight
}

// bodyOverflows reports whether Rows holds more entries than fit in
// the body at the widget's current height -- the single condition
// that gates the right-edge scrollbar's presence, and the column
// width reservation contentWidth carves out for it, in Draw. A body
// with no visible row capacity at all (Bounds().H shorter than the
// header) never "overflows" -- there is no body pixel row for a
// scrollbar track to occupy.
func (t *Table) bodyOverflows() bool {
	vis := t.bodyVisibleRows()
	return vis > 0 && t.lineCount() > vis
}

// maxScrollRow is the highest legal ScrollRow: enough rows short of
// the end that the body still shows a full window of bodyVisibleRows
// rows, or 0 once Rows no longer overflows (including the empty-Rows
// case) -- exactly the "[0, max(0, len(Rows)-visibleRows)]" range
// ScrollRow's doc promises.
func (t *Table) maxScrollRow() int {
	max := t.lineCount() - t.bodyVisibleRows()
	if max < 0 {
		max = 0
	}
	return max
}

// grouped reports whether row grouping is active: GroupBy must name a real
// column. The ungrouped default (-1) short-circuits every grouping branch so
// the body renders exactly as before this feature existed.
func (t *Table) grouped() bool {
	return t.GroupBy >= 0 && t.GroupBy < len(t.Columns)
}

// useLineModel reports whether the body is rendered from the explicit
// visual-line list (lines()) rather than the plain one-line-per-row fast path.
// It is true whenever grouping is active, a summary footer is requested, OR a
// row is expanded -- the three features that deviate from the 1:1
// row-to-line mapping. When false every lines()-based helper falls back to the
// identity model, so the body renders byte-for-byte as it did before any of
// them existed.
func (t *Table) useLineModel() bool {
	return t.grouped() || t.ShowSummary || t.hasExpanded()
}

// hasExpanded reports whether any row is currently expanded into a detail line
// -- true only while RowDetail is set (a detail line has no text source
// otherwise) AND the expanded set holds at least one row. toggleExpand deletes
// an entry when folding a row shut, so a non-empty set always means a live
// expansion.
func (t *Table) hasExpanded() bool {
	return t.RowDetail != nil && len(t.expanded) > 0
}

// rowExpanded reports whether row is currently expanded (and expandable at all,
// i.e. RowDetail is set). It is the single test lines() uses to decide whether
// to emit a detail line after a data row.
func (t *Table) rowExpanded(row int) bool {
	return t.RowDetail != nil && t.expanded[row]
}

// reorderActive reports whether drag-to-reorder is in effect: opted in via
// Reorderable AND the plain row-to-line model is in force (not grouped, no
// summary footer, nothing expanded). Any of those interleaves non-row lines
// the flat rowInsertIndexAt math can't place a drop against, so they suppress
// reordering entirely.
func (t *Table) reorderActive() bool {
	return t.Reorderable && !t.useLineModel()
}

// tableLine is one on-screen line of the body when the visual-line model is
// active (see useLineModel): a group header (header==true, carrying the group
// value, its member count and whether it is collapsed), a summary line
// (summary==true, aggregating rows [sumStart,sumEnd)), a detail line
// (detail==true, carrying its parent row in dataRow), or a data line pointing
// at a row in t.Rows.
type tableLine struct {
	header    bool
	group     string // group value (header lines)
	count     int    // members in the group (header lines)
	collapsed bool   // whether the group is folded (header lines)
	dataRow   int    // index into t.Rows (data + detail lines)
	summary   bool   // whether this is an aggregate summary line
	sumStart  int    // first row (inclusive) a summary line aggregates
	sumEnd    int    // one past the last row a summary line aggregates
	detail    bool   // whether this is an expander detail line (see dataRow)
}

// aggregate reduces column col over the contiguous row range [start,end) to a
// display string per agg: "" for AggregateNone, the row count for
// AggregateCount, and the sum/mean/min/max of the range's numeric cells (cells
// that fail strconv.ParseFloat, or that a ragged row omits, are skipped) for
// the numeric kinds. A numeric aggregate with no numeric cell in range yields
// "" so an all-text column reads blank rather than "0".
func (t *Table) aggregate(agg TableAggregate, col, start, end int) string {
	// A column's custom AggregateFunc overrides the built-in reduction: it
	// receives one entry per row in range (ragged rows contribute ""), so a
	// column can display a median, a distinct count, a "min-max" span, etc.
	if col >= 0 && col < len(t.Columns) {
		if fn := t.Columns[col].AggregateFunc; fn != nil {
			cells := make([]string, 0, end-start)
			for r := start; r < end; r++ {
				if col < len(t.Rows[r]) {
					cells = append(cells, t.Rows[r][col])
				} else {
					cells = append(cells, "")
				}
			}
			return fn(cells)
		}
	}
	switch agg {
	case AggregateNone:
		return ""
	case AggregateCount:
		return strconv.Itoa(end - start)
	}
	var sum, min, max float64
	count := 0
	for r := start; r < end; r++ {
		if col >= len(t.Rows[r]) {
			continue
		}
		v, err := strconv.ParseFloat(t.Rows[r][col], 64)
		if err != nil {
			continue
		}
		if count == 0 {
			min, max = v, v
		} else {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		sum += v
		count++
	}
	if count == 0 {
		return ""
	}
	switch agg {
	case AggregateSum:
		return formatAggregate(sum)
	case AggregateAvg:
		return formatAggregate(sum / float64(count))
	case AggregateMin:
		return formatAggregate(min)
	default: // AggregateMax
		return formatAggregate(max)
	}
}

// formatAggregate renders an aggregate float with a sensible number format: a
// value that is exactly integral prints without a fractional part, otherwise
// it prints with two decimal places.
func formatAggregate(f float64) string {
	if i := int64(f); float64(i) == f {
		return strconv.FormatInt(i, 10)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}

// groupValue returns row r's grouping key: the GroupBy column's cell value
// (or "" when the row is too short to have that cell), passed through the
// column's GroupKey deriver when one is set. Rows sharing a key group together
// and the key labels the group header. It is only called while grouped(), so
// GroupBy indexes a real column.
func (t *Table) groupValue(r int) string {
	cell := ""
	if t.GroupBy < len(t.Rows[r]) {
		cell = t.Rows[r][t.GroupBy]
	}
	if gk := t.Columns[t.GroupBy].GroupKey; gk != nil {
		return gk(cell)
	}
	return cell
}

// lines builds the ordered visual-line list for the line-model body. Grouped,
// it emits for each run of consecutive rows sharing a GroupBy value a header
// line, then (unless collapsed) that run's data lines and an optional per-group
// summary. Ungrouped, it emits one data line per row. In both cases an expanded
// row is followed by its detail line, and a grand-total summary footer closes
// the list when ShowSummary is set. It returns nil when the plain
// one-line-per-row model is in force (see useLineModel), the signal every
// geometry helper uses to fall back to it. Runs are consecutive: lines assumes
// Rows are already ordered by the group column and does not sort.
func (t *Table) lines() []tableLine {
	if !t.useLineModel() {
		return nil
	}
	// emitRow appends row k's data line and, if it is expanded, its detail line.
	emitRow := func(out []tableLine, k int) []tableLine {
		out = append(out, tableLine{dataRow: k})
		if t.rowExpanded(k) {
			out = append(out, tableLine{detail: true, dataRow: k})
		}
		return out
	}
	var out []tableLine
	if t.grouped() {
		i := 0
		for i < len(t.Rows) {
			val := t.groupValue(i)
			j := i + 1
			for j < len(t.Rows) && t.groupValue(j) == val {
				j++
			}
			collapsed := t.collapsed[val]
			out = append(out, tableLine{header: true, group: val, count: j - i, collapsed: collapsed})
			if !collapsed {
				for k := i; k < j; k++ {
					out = emitRow(out, k)
				}
				// Per-group summary line, aggregating just this group's rows,
				// after its (expanded) members. Suppressed while collapsed so a
				// folded group hides its summary along with its rows.
				if t.ShowSummary {
					out = append(out, tableLine{summary: true, sumStart: i, sumEnd: j})
				}
			}
			i = j
		}
	} else {
		for k := range t.Rows {
			out = emitRow(out, k)
		}
	}
	// Grand-total footer as the final line, aggregating every row. Only when
	// there is at least one row -- an empty Table renders its "(no data)"
	// placeholder instead (Draw returns before the body loop).
	if t.ShowSummary && len(t.Rows) > 0 {
		out = append(out, tableLine{summary: true, sumStart: 0, sumEnd: len(t.Rows)})
	}
	return out
}

// lineCount is the number of on-screen body lines: len(Rows) when ungrouped,
// or the grouped line count (headers + expanded rows) when grouping is on. It
// is what the scroll geometry (maxScrollRow / bodyOverflows / the scrollbar)
// measures the body against.
func (t *Table) lineCount() int {
	if !t.useLineModel() {
		return len(t.Rows)
	}
	return len(t.lines())
}

// visualIndex maps a data-row index to its on-screen line position (so the
// inline editor can be placed over the right line). Ungrouped it is the
// identity; grouped it scans the line list. A row inside a collapsed group has
// no visible line and yields -1.
func (t *Table) visualIndex(dataRow int) int {
	if !t.useLineModel() {
		return dataRow
	}
	for vi, ln := range t.lines() {
		if !ln.header && !ln.summary && !ln.detail && ln.dataRow == dataRow {
			return vi
		}
	}
	return -1
}

// toggleGroup folds or unfolds the group with the given value, lazily creating
// the collapsed map on first use.
func (t *Table) toggleGroup(val string) {
	if t.collapsed == nil {
		t.collapsed = map[string]bool{}
	}
	t.collapsed[val] = !t.collapsed[val]
	t.ScrollRow = t.clampScrollRow()
}

// toggleExpand flips row's expansion, lazily creating the expanded map on first
// use and deleting an entry when folding a row shut (so a non-empty map always
// means a live expansion -- see hasExpanded). An out-of-range row is a no-op.
// It reclamps ScrollRow the same way toggleGroup does, since the line count
// changed under the current window.
func (t *Table) toggleExpand(row int) {
	if row < 0 || row >= len(t.Rows) {
		return
	}
	if t.expanded == nil {
		t.expanded = map[int]bool{}
	}
	if t.expanded[row] {
		delete(t.expanded, row)
	} else {
		t.expanded[row] = true
	}
	t.ScrollRow = t.clampScrollRow()
}

// onDisclosure reports whether a Table-local x lands on column 0's disclosure
// gutter -- the leading strip an expander chevron occupies (see
// disclosureGutter). It is 0-wide (and so always false) while RowDetail is nil,
// so a click there falls through to normal cell selection/editing.
func (t *Table) onDisclosure(localX int) bool {
	widths := t.columnWidths(t.contentWidth())
	left := t.columnScreenX(0, widths, 0)
	return localX >= left && localX < left+t.disclosureGutter()
}

// clampScrollRow returns ScrollRow collapsed into [0, maxScrollRow()]
// -- the same "an out-of-range field never crashes Draw" idiom
// Selected + SortColumn already use, applied read-only here so a
// stale or directly-assigned ScrollRow (e.g. left over after Rows
// shrinks) never windows past the valid row range. It does NOT mutate
// t.ScrollRow; ScrollTo/ScrollBy/scrollToSelected are the API that
// keeps the field itself clamped going forward.
func (t *Table) clampScrollRow() int {
	s := t.ScrollRow
	if s < 0 {
		s = 0
	}
	if max := t.maxScrollRow(); s > max {
		s = max
	}
	return s
}

// contentWidth is the pixel budget columnWidths distributes for both
// the header + body: the widget's full width, minus a scrollbarWidth
// reservation on the right edge while the body overflows vertically.
// Draw, columnAt, ColumnSeparatorAt and the resize-drag branch of
// OnEvent all resolve column geometry through this one spot so the
// header, body, hit-testing and the scrollbar always agree on where
// every column + separator sits.
func (t *Table) contentWidth() int {
	w := t.Bounds().W
	if t.bodyOverflows() {
		w -= scrollbarWidth
		if w < 0 {
			w = 0
		}
	}
	return w
}

// hasAutoColumn reports whether any column is auto-width (Width <= 0). Auto
// columns make columnWidths fit the table to the viewport, which is
// incompatible with horizontal scrolling -- so their presence disables it.
func (t *Table) hasAutoColumn() bool {
	for _, c := range t.Columns {
		if c.Width <= 0 {
			return true
		}
	}
	return false
}

// naturalWidth is the columns' unfitted total pixel width -- the sum of the
// resolved column widths. With no auto columns (the only case it is consulted
// for) columnWidths returns the declared widths verbatim, so this is their
// plain sum regardless of the viewport.
func (t *Table) naturalWidth() int {
	total := 0
	for _, w := range t.columnWidths(t.contentWidth()) {
		total += w
	}
	return total
}

// hScrollable reports whether horizontal scrolling is active: the columns are
// all fixed-width AND their natural total exceeds the horizontal budget
// (contentWidth, which already reserves the vertical scrollbar's gutter). When
// false every horizontal-scroll branch is inert and the body renders exactly
// as it did before this feature.
func (t *Table) hScrollable() bool {
	return len(t.Columns) > 0 && !t.hasAutoColumn() && t.naturalWidth() > t.contentWidth()
}

// frozenCount is FrozenColumns clamped into [0, len(Columns)] and forced to 0
// unless horizontal scrolling is active (pinning columns is meaningless when
// nothing scrolls past them).
func (t *Table) frozenCount() int {
	if !t.hScrollable() {
		return 0
	}
	f := t.FrozenColumns
	if f < 0 {
		f = 0
	}
	if f > len(t.Columns) {
		f = len(t.Columns)
	}
	return f
}

// frozenWidth is the summed pixel width of the frozen leading columns.
func (t *Table) frozenWidth(widths []int) int {
	fw := 0
	for i := 0; i < t.frozenCount() && i < len(widths); i++ {
		fw += widths[i]
	}
	return fw
}

// maxScrollX is the highest legal ScrollX: enough that the last column's right
// edge lines up with the viewport's right edge. Consulted only through
// clampScrollX (which first checks hScrollable, i.e. naturalWidth >
// contentWidth), so the difference is always positive here.
func (t *Table) maxScrollX() int {
	return t.naturalWidth() - t.contentWidth()
}

// clampScrollX returns ScrollX collapsed into [0, maxScrollX()], read-only
// (like clampScrollRow). Zero whenever horizontal scrolling is inactive.
func (t *Table) clampScrollX() int {
	if !t.hScrollable() {
		return 0
	}
	s := t.ScrollX
	if s < 0 {
		s = 0
	}
	if m := t.maxScrollX(); s > m {
		s = m
	}
	return s
}

// ScrollXTo sets ScrollX to px, clamped into [0, maxScrollX()] -- the direct
// entry point a horizontal scrollbar drag or a wheel handler drives.
func (t *Table) ScrollXTo(px int) {
	t.ScrollX = px
	t.ScrollX = t.clampScrollX()
}

// ScrollXBy adjusts ScrollX by delta pixels (positive scrolls right), clamped
// the same way as ScrollXTo.
func (t *Table) ScrollXBy(delta int) {
	t.ScrollXTo(t.ScrollX + delta)
}

// columnScreenX returns column j's on-surface left edge given the resolved
// widths: frozen columns sit at their natural offset, scrolled columns are
// shifted left by clampScrollX. When horizontal scrolling is inactive
// frozenCount is 0 and clampScrollX is 0, so this is just the natural offset
// r.X + sum(widths[:j]) -- identical to the pre-feature geometry.
func (t *Table) columnScreenX(rx int, widths []int, j int) int {
	x := rx
	for k := 0; k < j && k < len(widths); k++ {
		x += widths[k]
	}
	if j >= t.frozenCount() {
		x -= t.clampScrollX()
	}
	return x
}

// ScrollTo sets ScrollRow to row, clamped into [0, maxScrollRow()] --
// the direct, host-callable entry point a scrollbar drag or a
// PageUp/PageDown key handler drives, mirroring how SetColumnWidth is
// the direct entry point a separator drag drives.
func (t *Table) ScrollTo(row int) {
	t.ScrollRow = row
	t.ScrollRow = t.clampScrollRow()
}

// ScrollBy adjusts ScrollRow by delta rows (positive scrolls down,
// negative scrolls up), clamped the same way as ScrollTo. A mouse
// wheel or arrow-key handler calls this directly.
func (t *Table) ScrollBy(delta int) {
	t.ScrollTo(t.ScrollRow + delta)
}

// scrollToSelected nudges ScrollRow just far enough to bring Selected
// back into the visible window: up if Selected sits above ScrollRow,
// down if it sits at or past the bottom of the window, otherwise
// ScrollRow is left untouched. Selected < 0 ("no selection") is a
// guarded no-op -- without it the arithmetic below would compute a
// bogus target from a -1 row index, the same off-by-one that panicked
// the tui Table's equivalent helper with "index out of range [-1]".
func (t *Table) scrollToSelected() {
	if t.Selected < 0 {
		return
	}
	vis := t.bodyVisibleRows()
	if vis <= 0 {
		return
	}
	switch {
	case t.Selected < t.ScrollRow:
		t.ScrollTo(t.Selected)
	case t.Selected >= t.ScrollRow+vis:
		t.ScrollTo(t.Selected - vis + 1)
	}
}

// handleKey drives the keyboard roving cursor: Arrow/Page/Home/End move
// Selected over the body rows (auto-scrolling to keep it visible),
// Shift+ArrowUp/Down extends a MultiSelect range from the cursor, and Enter /
// Space activate the cursor row like a click. A disabled Table, or one with
// no rows, ignores every key. Selected doubles as the cursor, so the Accent
// highlight tracks keyboard navigation exactly as it does a mouse selection.
func (t *Table) handleKey(ev Event) {
	if t.Disabled || len(t.Rows) == 0 {
		return
	}
	switch ev.Code {
	case "Enter", " ", "Space":
		// In EditOnDoubleClick mode Enter opens the cursor row's first
		// Editable column (the desktop rename-on-Enter idiom); otherwise it
		// activates the cursor row exactly as before.
		if ev.Code == "Enter" && t.beginEditCursor() {
			return
		}
		t.activateCursor()
		return
	}
	if t.MultiSelect && ev.Shift && (ev.Code == "ArrowUp" || ev.Code == "ArrowDown") {
		t.extendRowSelection(ev.Code)
		return
	}
	if idx, ok := rovingIndex(t.Selected, len(t.Rows), t.bodyVisibleRows(), ev.Code); ok {
		t.setSelected(idx)
		// A plain move mirrors a plain click: in MultiSelect mode the cursor
		// row becomes the sole selection (and the anchor).
		if t.MultiSelect {
			t.SetRowSelection(idx)
		}
		t.scrollToSelected()
	}
}

// setSelected moves the highlighted-row cursor to row and fires OnSelect
// (nil-safe) when the value actually changes -- the shared mutate+notify path
// a keyboard cursor move and a MultiSelect anchor-moving click both take.
// Re-selecting the current row is silent, keeping a two-way binding loop-free.
func (t *Table) setSelected(row int) {
	if t.Selected == row {
		return
	}
	t.Selected = row
	if t.OnSelect != nil {
		t.OnSelect(row)
	}
}

// activateCursor applies to the cursor row exactly what a plain click on it
// would: in MultiSelect mode it collapses the selection to that single row.
// A single-select Table has no click-side callback, so activation there just
// keeps the cursor row as Selected (it already is). An out-of-range cursor is
// a no-op.
func (t *Table) activateCursor() {
	if t.Selected < 0 || t.Selected >= len(t.Rows) {
		return
	}
	if t.MultiSelect {
		t.SetRowSelection(t.Selected)
	}
}

// editActivates reports whether an EventClick on an Editable cell should open
// its editor, per EditActivation: every click in EditOnSingleClick (the
// default), only a double-click (Code TableDoubleClick) in EditOnDoubleClick,
// never in EditManual.
func (t *Table) editActivates(ev Event) bool {
	switch t.EditActivation {
	case EditOnDoubleClick:
		return ev.Code == TableDoubleClick
	case EditManual:
		return false
	default: // EditOnSingleClick
		return true
	}
}

// beginEditCursor opens the cursor row's first Editable column for editing and
// reports whether an editor actually opened. It is the Enter-to-edit path,
// active only in EditOnDoubleClick mode (so single-click and manual tables keep
// Enter as a pure row activation); an out-of-range cursor or a row with no
// Editable column is a no-op that returns false.
func (t *Table) beginEditCursor() bool {
	if t.EditActivation != EditOnDoubleClick {
		return false
	}
	if t.Selected < 0 || t.Selected >= len(t.Rows) {
		return false
	}
	for c := range t.Columns {
		if t.Columns[c].Editable {
			t.beginEdit(t.Selected, c)
			return t.editor != nil
		}
	}
	return false
}

// extendRowSelection grows the multi-row selection by one row in the arrow's
// direction and moves the lead cursor (Selected) onto the newly entered row,
// keeping it visible -- the keyboard analogue of Shift+click's range extend.
// The previous cursor row is folded into the set first so the anchor is never
// lost; a move already at the top/bottom edge simply re-selects the same row.
// Only reached while MultiSelect is on and Rows is non-empty.
func (t *Table) extendRowSelection(code string) {
	prev := t.Selected
	if prev < 0 {
		prev = 0
	}
	next := prev
	if code == "ArrowDown" {
		next = min(prev+1, len(t.Rows)-1)
	} else {
		next = max(prev-1, 0)
	}
	if t.selectedRows == nil {
		t.selectedRows = make(map[int]bool)
	}
	t.selectedRows[prev] = true
	t.selectedRows[next] = true
	t.setSelected(next)
	t.scrollToSelected()
}

// IsRowSelected reports whether row i is a member of the multi-row
// selection set. A negative i is always false -- mirrors how every
// other row/column index in this file collapses an invalid value
// instead of indexing into (or panicking on) the underlying map/slice.
// It answers from the raw set regardless of MultiSelect; only Draw and
// OnEvent gate their use of it on MultiSelect being true.
func (t *Table) IsRowSelected(i int) bool {
	if i < 0 {
		return false
	}
	return t.selectedRows[i]
}

// SelectedRows returns every selected row index in ascending order, or
// nil if nothing is selected. The slice is a fresh copy -- mutating it
// has no effect on the Table's selection state.
func (t *Table) SelectedRows() []int {
	if len(t.selectedRows) == 0 {
		return nil
	}
	out := make([]int, 0, len(t.selectedRows))
	for i := range t.selectedRows {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// SetRowSelection replaces the current selection with exactly rows.
// Negative entries are dropped; calling it with no arguments (or with
// only negative ones) clears the selection, same end state as
// ClearRowSelection.
func (t *Table) SetRowSelection(rows ...int) {
	sel := make(map[int]bool, len(rows))
	for _, r := range rows {
		if r >= 0 {
			sel[r] = true
		}
	}
	t.selectedRows = sel
}

// ClearRowSelection empties the multi-row selection set.
func (t *Table) ClearRowSelection() {
	t.selectedRows = nil
}

// ToggleRowSelect flips row i's membership in the selection set --
// selecting it if absent, deselecting it if present. A negative i is a
// no-op.
func (t *Table) ToggleRowSelect(i int) {
	if i < 0 {
		return
	}
	if t.selectedRows == nil {
		t.selectedRows = make(map[int]bool)
	}
	if t.selectedRows[i] {
		delete(t.selectedRows, i)
	} else {
		t.selectedRows[i] = true
	}
}

// SelectRowRange replaces the selection with the inclusive range
// between a and b -- callers may pass either endpoint first, matching
// how a Shift-click can land above OR below the anchor. A negative
// endpoint clamps to 0 (so an anchor of -1, "nothing selected yet",
// still yields a sane from-the-top range instead of an empty one); if
// both endpoints are negative the resulting selection is empty.
func (t *Table) SelectRowRange(a, b int) {
	if a > b {
		a, b = b, a
	}
	if a < 0 {
		a = 0
	}
	sel := make(map[int]bool)
	for i := a; i <= b; i++ {
		sel[i] = true
	}
	t.selectedRows = sel
}

// rowAt returns the body row index whose vertical band contains
// localY (a Table-local y coordinate, i.e. relative to the widget's
// own top edge and therefore still including the header offset), or
// -1 if localY lands in/above the header or at/past the last row --
// the same "collapse to -1 outside the valid range" idiom columnAt
// and ColumnSeparatorAt already use for x coordinates. The offset
// within the body is added to clampScrollRow() (not raw ScrollRow) so
// a click always resolves to whatever row Draw actually painted at
// that y, even with an out-of-range ScrollRow.
// RowAt returns the data-row index under widget-local (x, y), or -1 for the
// header band, a group-header/summary line, or empty space past the last row.
// Exposed so a host can hit-test a right-click and build a context menu for the
// row under the cursor (x is accepted for signature symmetry with the other
// widgets' hit helpers; the row is determined by y).
func (t *Table) RowAt(x, y int) int { return t.rowAt(y) }

func (t *Table) rowAt(localY int) int {
	if localY < TableHeaderHeight {
		return -1
	}
	vi := t.clampScrollRow() + (localY-TableHeaderHeight)/TableRowHeight
	if !t.useLineModel() {
		if vi < 0 || vi >= len(t.Rows) {
			return -1
		}
		return vi
	}
	// Line model: the visual line at vi is a header (no data row), a summary
	// line (no data row) or a data line pointing at t.Rows.
	lines := t.lines()
	if vi < 0 || vi >= len(lines) || lines[vi].header || lines[vi].summary || lines[vi].detail {
		return -1
	}
	return lines[vi].dataRow
}

// lineAt returns the visual line under a Table-local y, and false when y is in
// the header strip, above/below the body lines, or the Table is ungrouped (so
// there are no group headers to hit). It is how a click discovers a group
// header to toggle.
func (t *Table) lineAt(localY int) (tableLine, bool) {
	if localY < TableHeaderHeight || !t.grouped() {
		return tableLine{}, false
	}
	vi := t.clampScrollRow() + (localY-TableHeaderHeight)/TableRowHeight
	lines := t.lines()
	if vi < 0 || vi >= len(lines) {
		return tableLine{}, false
	}
	return lines[vi], true
}

// cellRect returns the on-surface rectangle a body cell occupies, in the
// same absolute coordinate space Draw paints into (Bounds()-relative). It
// mirrors the Draw body geometry exactly: the row band is bodyY +
// (row-clampScrollRow)*TableRowHeight tall by TableRowHeight, and the column
// span is the cumulative columnWidths, with the first column's leading gutter
// (disclosure + icon, if any) carved off the left so the editor sits over the
// text area. It is what positions the inline cell editor.
func (t *Table) cellRect(row, col int) Rect {
	r := t.Bounds()
	widths := t.columnWidths(t.contentWidth())
	y := r.Y + TableHeaderHeight + (t.visualIndex(row)-t.clampScrollRow())*TableRowHeight
	cellX := t.columnScreenX(r.X, widths, col)
	cellW := widths[col]
	if col == 0 {
		if g := t.leadGutter(); g > 0 {
			cellX += g
			cellW -= g
		}
	}
	return Rect{X: cellX, Y: y, W: cellW, H: TableRowHeight}
}

// beginEdit opens an inline text editor over cell (row,col), seeded with the
// cell's current value and focused so keystrokes land in it. It is a no-op
// for an out-of-range cell or a column that is not Editable. Committing
// (Enter) writes the value back and fires OnCellEdit; Escape cancels.
func (t *Table) beginEdit(row, col int) {
	if row < 0 || row >= len(t.Rows) || col < 0 || col >= len(t.Columns) {
		return
	}
	if !t.Columns[col].Editable {
		return
	}
	val := ""
	if col < len(t.Rows[row]) {
		val = t.Rows[row][col]
	}
	var ed CellEditor
	if f := t.Columns[col].Editor; f != nil {
		ed = f()
	} else {
		ed = t.newTextCellEditor(val)
	}
	ed.SetCellValue(val)
	ed.Focus(true)
	ed.OnCellSubmit(func() { t.commitEdit() })
	t.editRow, t.editCol, t.editor, t.editErr = row, col, ed, nil
}

// BeginEdit opens an inline editor over cell (row,col) programmatically -- the
// command-style entry point a view model (or a host in EditManual mode) uses to
// start an edit without a click. It applies the same guards as a click-driven
// edit: an out-of-range cell or a column that is not Editable is a no-op.
func (t *Table) BeginEdit(row, col int) { t.beginEdit(row, col) }

// commitEdit validates the open editor's text against the column's Validate
// rule and, when it passes, writes it back into Rows, fires OnCellEdit
// (nil-safe) and closes the editor. A rejected value is left in the still-open
// editor: Rows is untouched, EditError reports the rule's error and
// OnCellEditRejected fires. It is a no-op when no edit is in progress. A cell
// whose row is shorter than editCol (a ragged Row) is left untouched -- there
// is no slot to write.
func (t *Table) commitEdit() {
	if t.editRow < 0 || t.editor == nil {
		return
	}
	r, c, v := t.editRow, t.editCol, t.editor.CellValue()
	if rule := t.Columns[c].Validate; rule != nil {
		if err := rule(v); err != nil {
			t.editErr = err
			if t.OnCellEditRejected != nil {
				t.OnCellEditRejected(r, c, v, err)
			}
			return
		}
	}
	if r < len(t.Rows) && c < len(t.Rows[r]) {
		t.Rows[r][c] = v
	}
	t.editRow, t.editCol, t.editor, t.editErr = -1, -1, nil, nil
	if t.OnCellEdit != nil {
		t.OnCellEdit(r, c, v)
	}
}

// CommitEdit is the public trigger for committing the open edit (Enter's path)
// -- a command a view model binds to. It is a no-op when no edit is in
// progress, and honours the column's Validate rule exactly like an Enter
// commit.
func (t *Table) CommitEdit() { t.commitEdit() }

// cancelEdit discards the open editor without touching Rows or firing
// OnCellEdit, clearing any validation error.
func (t *Table) cancelEdit() {
	t.editRow, t.editCol, t.editor, t.editErr = -1, -1, nil, nil
}

// CancelEdit is the public trigger for discarding the open edit (Escape's
// path) -- a command a view model binds to. It is a no-op when no edit is in
// progress.
func (t *Table) CancelEdit() { t.cancelEdit() }

// Editing reports the cell whose inline editor is currently open. editing is
// false (and row/col are -1) when no edit is in progress -- the observable
// getter a view model reads to reflect edit state.
func (t *Table) Editing() (row, col int, editing bool) {
	if t.editRow < 0 || t.editor == nil {
		return -1, -1, false
	}
	return t.editRow, t.editCol, true
}

// EditError returns the validation error of the open edit's last rejected
// commit, or nil when the pending value is valid (or no edit is in progress).
// It is what surfaces the reason an edit would not commit.
func (t *Table) EditError() error { return t.editErr }

// rowInsertIndexAt returns the drag-to-reorder insertion index for a
// Table-local y coordinate: 0..len(Rows) inclusive, where len(Rows)
// means "after the last row". Unlike rowAt (which answers "which row's
// band contains y" and returns -1 outside the body), this rounds to
// the NEAREST row boundary so the drop indicator can land either above
// or below the row the pointer happens to be over, and clamps into
// range instead of returning a sentinel -- a drag hovering above the
// header or past the last row always resolves to a valid (if
// degenerate) insertion point, exactly like a real drag-and-drop
// indicator would.
func (t *Table) rowInsertIndexAt(localY int) int {
	scroll := t.clampScrollRow()
	if localY < TableHeaderHeight {
		return scroll
	}
	idx := scroll + (localY-TableHeaderHeight+TableRowHeight/2)/TableRowHeight
	if idx > len(t.Rows) {
		idx = len(t.Rows)
	}
	return idx
}

// remapRowIndex answers "row i (in the array BEFORE a single-element
// move) now lives at what index (in the array AFTER the move)", given
// the moved row's original index `from` and its resting index `final`
// (both in the same before/after sense as reorderRow's own from/final).
// It is the shared arithmetic reorderRow uses to keep Selected +
// selectedRows following the moved row instead of silently pointing at
// whatever row slid into their old slot.
func remapRowIndex(i, from, final int) int {
	switch {
	case i == from:
		return final
	case from < final:
		if i > from && i <= final {
			return i - 1
		}
	case from > final:
		if i >= final && i < from {
			return i + 1
		}
	}
	return i
}

// reorderRow moves the row at index from to insertion index to (0..
// len(Rows), pre-move coordinates -- see rowInsertIndexAt), updates
// Selected + selectedRows so the selection follows the moved row, then
// fires OnReorder with from and the row's actual resting index (which
// differs from to by one when the row moved downward past its own
// origin -- removing it first shifts every later index down). An
// out-of-range from is a no-op; to is clamped into [0, len(Rows)]
// first, so a caller can pass a raw rowInsertIndexAt result unchecked.
func (t *Table) reorderRow(from, to int) {
	n := len(t.Rows)
	if from < 0 || from >= n {
		return
	}
	if to < 0 {
		to = 0
	} else if to > n {
		to = n
	}
	final := to
	if to > from {
		final--
	}
	// final is provably within [0, n-1] here: the `to` clamp above keeps
	// to in [0, n], and from is in [0, n-1] (checked above), so both the
	// to<=from branch (final=to<=from<=n-1) and the to>from branch
	// (final=to-1, with from<to<=n so final in [from, n-1]) land inside
	// the post-removal slice's valid insertion range -- no further clamp
	// needed (and none would ever be exercised: n-1 == len(t.Rows) right
	// after the removal below).
	row := t.Rows[from]
	t.Rows = append(t.Rows[:from], t.Rows[from+1:]...)
	t.Rows = append(t.Rows, nil)
	copy(t.Rows[final+1:], t.Rows[final:])
	t.Rows[final] = row

	if t.Selected >= 0 {
		t.Selected = remapRowIndex(t.Selected, from, final)
	}
	if len(t.selectedRows) > 0 {
		remapped := make(map[int]bool, len(t.selectedRows))
		for i := range t.selectedRows {
			remapped[remapRowIndex(i, from, final)] = true
		}
		t.selectedRows = remapped
	}

	if t.OnReorder != nil {
		t.OnReorder(from, final)
	}
}

// toggleSort updates SortColumn/SortAsc for a header click on col --
// re-clicking the active column flips SortAsc, clicking a new column
// resets to ascending -- then fires OnSort with the resulting
// direction. The Table never touches Rows itself; the host re-sorts
// and hands the Table its updated data.
func (t *Table) toggleSort(col int) {
	asc := true
	if t.SortColumn == col {
		asc = !t.SortAsc
	}
	if t.SelfSort {
		// SortByColumn reorders Rows and sets SortColumn/SortAsc itself.
		t.SortByColumn(col, asc)
	} else {
		t.SortColumn, t.SortAsc = col, asc
	}
	if t.OnSort != nil {
		t.OnSort(col, asc)
	}
}

// defaultCellCompare orders two cell strings: numerically when BOTH parse as
// float64 (so "9" sorts before "10", not after), otherwise lexicographically.
// It is the ordering SortByColumn and ArrangeGroups use for a column that
// supplies no Comparator.
func defaultCellCompare(a, b string) int {
	af, aerr := strconv.ParseFloat(a, 64)
	bf, berr := strconv.ParseFloat(b, 64)
	if aerr == nil && berr == nil {
		switch {
		case af < bf:
			return -1
		case af > bf:
			return 1
		default:
			return 0
		}
	}
	return strings.Compare(a, b)
}

// cellAt returns row r's cell in column c, or "" when the row is too short
// (ragged) -- the safe reader SortByColumn/ArrangeGroups compare through.
func (t *Table) cellAt(r, c int) string {
	if r >= 0 && r < len(t.Rows) && c >= 0 && c < len(t.Rows[r]) {
		return t.Rows[r][c]
	}
	return ""
}

// SortByColumn reorders Rows in place by column col, ascending or descending,
// using the column's Comparator (or defaultCellCompare when it has none), and
// records the result in SortColumn/SortAsc so the header shows the indicator.
// The sort is stable, so rows equal under the comparator keep their prior
// order. Selected, the multi-row selection set and the expanded-row set all
// follow their rows to the new positions, and ScrollRow is re-clamped. An
// out-of-range col is a no-op. This is the opt-in counterpart to the
// content-only OnSort path -- a host or view model calls it (or sets SelfSort
// to have header clicks call it) to let the Table own its ordering.
func (t *Table) SortByColumn(col int, ascending bool) {
	if col < 0 || col >= len(t.Columns) {
		return
	}
	cmp := t.Columns[col].Comparator
	if cmp == nil {
		cmp = defaultCellCompare
	}
	order := make([]int, len(t.Rows))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		c := cmp(t.cellAt(order[a], col), t.cellAt(order[b], col))
		if !ascending {
			c = -c
		}
		return c < 0
	})
	t.applyRowOrder(order)
	t.SortColumn, t.SortAsc = col, ascending
}

// ArrangeGroups reorders Rows so the GroupBy column's groups become contiguous
// and are themselves ordered by that column's Comparator (applied to the group
// keys, honouring GroupKey), the prerequisite the grouped line model needs
// (it gathers CONSECUTIVE runs of a shared key and does not sort). The sort is
// stable, so rows within a group keep their prior order, and Selected /
// selection / expansion follow their rows. It is a no-op unless grouping is
// active (GroupBy names a real column). Call it after loading or mutating Rows
// out of group order; a host that already delivers grouped-ordered rows need
// not.
func (t *Table) ArrangeGroups() {
	if !t.grouped() {
		return
	}
	cmp := t.Columns[t.GroupBy].Comparator
	if cmp == nil {
		cmp = defaultCellCompare
	}
	order := make([]int, len(t.Rows))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return cmp(t.groupValue(order[a]), t.groupValue(order[b])) < 0
	})
	t.applyRowOrder(order)
}

// applyRowOrder permutes Rows so the new row i is the old row order[i], then
// remaps Selected, the multi-row selection set and the expanded-row set through
// the inverse permutation so every piece of per-row state follows its row to
// its new index instead of pointing at whatever row slid into the old slot. It
// re-clamps ScrollRow since the visible window's contents changed. order must
// be a permutation of [0,len(Rows)) -- SortByColumn and ArrangeGroups build it
// that way.
func (t *Table) applyRowOrder(order []int) {
	inv := make([]int, len(order))
	newRows := make([][]string, len(order))
	for newI, oldI := range order {
		newRows[newI] = t.Rows[oldI]
		inv[oldI] = newI
	}
	t.Rows = newRows
	if t.Selected >= 0 && t.Selected < len(inv) {
		t.Selected = inv[t.Selected]
	}
	if len(t.selectedRows) > 0 {
		remap := make(map[int]bool, len(t.selectedRows))
		for i := range t.selectedRows {
			if i >= 0 && i < len(inv) {
				remap[inv[i]] = true
			}
		}
		t.selectedRows = remap
	}
	if len(t.expanded) > 0 {
		remap := make(map[int]bool, len(t.expanded))
		for i := range t.expanded {
			if i >= 0 && i < len(inv) {
				remap[inv[i]] = true
			}
		}
		t.expanded = remap
	}
	t.ScrollRow = t.clampScrollRow()
}

// OnEvent implements header-click sorting, separator drag-resize, and
// (while MultiSelect is true) body-row multi-selection. The toolkit's
// event model is click-only (see Paned): a resize drag begins on an
// EventClick that lands on a separator (ColumnSeparatorAt), is driven
// tick-by-tick by EventMouseDrag while the button stays down, and ends
// on EventMouseUp -- the same grab/move/release state machine
// RangeSlider uses for its thumbs. A click that lands on a header cell
// instead of a separator sorts that column (if Sortable).
//
// A click below the header row is a header/sort/resize no-op -- it
// falls through to the body-row branch instead. With MultiSelect
// false that branch is itself a no-op (the original, selection-free
// behaviour); with MultiSelect true a plain click selects only that
// row and moves the Selected anchor to it, Ctrl toggles the row
// without moving the anchor, and Shift selects the inclusive range
// between the anchor and the clicked row (also without moving the
// anchor, so repeated Shift-clicks keep ranging from the same
// origin). A click past the last row (rowAt returns -1) is ignored.
func (t *Table) OnEvent(ev Event) {
	// While an inline cell editor is open it owns the keyboard: characters
	// and edit keys route to it, Enter commits (via the editor's OnSubmit),
	// and Escape cancels. A click inside the editing cell is left to the
	// editor (cursor placement); a click anywhere else commits first, then
	// falls through so it selects/edits the newly-clicked cell normally.
	if t.editRow >= 0 && t.editor != nil {
		switch ev.Kind {
		case EventChar:
			t.editor.OnEvent(ev)
			return
		case EventKeyDown:
			if ev.Code == "Escape" {
				t.cancelEdit()
				return
			}
			t.editor.OnEvent(ev)
			return
		case EventClick:
			if t.rowAt(ev.Y) == t.editRow && t.columnAt(ev.X) == t.editCol {
				return
			}
			t.commitEdit()
		}
	}
	switch ev.Kind {
	case EventScroll:
		// Native wheel scroll: shift the visible row window by Delta rows
		// (ScrollBy clamps at top + bottom).
		t.ScrollBy(ev.Delta)
	case EventKeyDown:
		// Arrow / Page / Home / End move the selection cursor (Selected) over
		// the body rows, auto-scrolling to keep it visible; Shift+Arrow
		// extends a MultiSelect range; Enter / Space activate the cursor row.
		// Reached only when no inline editor is open (the editor branch above
		// already consumed the keystroke while editing).
		t.handleKey(ev)
	case EventClick:
		// A press on the scrollbar grabs its thumb (or pages the track); it
		// must not also select/edit a row, so consume it first.
		if g, ok := t.scrollbarGeom(); t.sbDrag.press(g, ok, ev, t.bodyVisibleRows(), t.ScrollBy) {
			return
		}
		if ev.Y < 0 {
			return
		}
		if ev.Y < TableHeaderHeight {
			if sep := t.ColumnSeparatorAt(ev.X); sep >= 0 {
				t.resizing = true
				t.resizingCol = sep
				return
			}
			col := t.columnAt(ev.X)
			if col >= 0 && col < len(t.Columns) && t.Columns[col].Sortable {
				t.toggleSort(col)
			}
			return
		}
		// A click on a group header toggles its collapse (grouped only) and
		// never selects or edits.
		if ln, ok := t.lineAt(ev.Y); ok && ln.header {
			t.toggleGroup(ln.group)
			return
		}
		row := t.rowAt(ev.Y)
		// A click on a data row's disclosure gutter toggles its expansion and
		// never selects or edits -- it takes precedence over the Editable-cell
		// and selection branches so the chevron area stays a pure toggle even in
		// an Editable first column. Inert (onDisclosure false) while RowDetail
		// is nil, so the pre-feature behaviour is unchanged.
		if row >= 0 && t.onDisclosure(ev.X) {
			t.toggleExpand(row)
			return
		}
		// A click on an Editable cell opens an inline editor over it
		// (and takes precedence over row selection/drag for that cell) --
		// but only when EditActivation says this click activates: a single
		// click in EditOnSingleClick (the default), a double-click (Code
		// TableDoubleClick) in EditOnDoubleClick, never in EditManual. A
		// non-activating click falls through so the cell's row can select.
		if row >= 0 {
			if col := t.columnAt(ev.X); col >= 0 && col < len(t.Columns) && t.Columns[col].Editable && t.editActivates(ev) {
				t.beginEdit(row, col)
				return
			}
		}
		// A body-row press starts a potential row drag whenever reordering
		// is active -- independent of MultiSelect, so drag-to-reorder works
		// on a passive-viewer Table too. Grouping suppresses it.
		if t.reorderActive() && row >= 0 {
			t.dragRow = row
		}
		if !t.MultiSelect {
			return
		}
		if row < 0 {
			return
		}
		switch {
		case ev.Shift:
			t.SelectRowRange(t.Selected, row)
		case ev.Ctrl:
			t.ToggleRowSelect(row)
		default:
			t.SetRowSelection(row)
			t.setSelected(row)
		}
	case EventMouseDrag:
		// A scrollbar-thumb drag takes precedence over a column resize.
		if t.sbDrag.active {
			g, ok := t.scrollbarGeom()
			t.sbDrag.drag(g, ok, ev, t.ScrollTo)
			return
		}
		if !t.resizing {
			return
		}
		widths := t.columnWidths(t.contentWidth())
		// The column's on-screen left edge (events are Table-local, so rx=0).
		// columnScreenX folds in the frozen/scroll offset; with no h-scroll it
		// is just the natural cumulative left, byte-identical to before.
		left := t.columnScreenX(0, widths, t.resizingCol)
		t.SetColumnWidth(t.resizingCol, ev.X-left)
	case EventMouseUp:
		t.resizing = false
		t.sbDrag.release()
	case EventDragMove:
		if !t.reorderActive() {
			return
		}
		t.dropIndicator = t.rowInsertIndexAt(ev.Y)
	case EventDragLeave:
		t.dropIndicator = -1
	case EventDrop:
		t.dropIndicator = -1
		if !t.reorderActive() {
			return
		}
		from, ok := parseTableRowDragPayload(ev.Code)
		if !ok {
			return
		}
		t.reorderRow(from, t.rowInsertIndexAt(ev.Y))
	}
}

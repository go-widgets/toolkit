// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"strings"

	"github.com/go-widgets/painter"
)

// DatabaseEditor assembles the toolkit's leaf widgets into a database
// workbench — the missing piece vs a standalone TreeView / TextView / Table:
// a schema/object tree on the left, a SQL editor at the top-right and an
// editable results grid at the bottom-right, wired to a run/execute toolbar.
//
// Layout (absolute pixel regions, computed in SetBounds):
//
//	+---------------------------------------------------+
//	| toolbar (Run · Refresh)               full width  |  BarHeight
//	+----------------+----------------------------------+
//	|                | SQL editor (TextView)            |  EditorHeight
//	| schema tree    +----------------------------------+
//	| (TreeView)     | error strip                      |  ErrorHeight
//	|                +----------------------------------+
//	|                | results grid (Table, editable)   |  remainder
//	+----------------+----------------------------------+
//	  TreeWidth
//
// CRITICAL — driver-agnostic: the toolkit bundles NO real database drivers.
// DatabaseEditor is pure UI over an injected DataSource; a host wires in a
// go-ruby-{pg,mysql,sqlite3,mongodb,redis} adapter (or, in tests, an in-memory
// fake) that speaks the three-method seam below.
type DatabaseEditor struct {
	Base
	source DataSource

	// Composed leaf widgets. Exposed through the accessors (Tree/Editor/Grid/
	// Toolbar) rather than as fields so callers customise them without being
	// able to swap the layout-managed pointers out from under SetBounds.
	tree   *TreeView
	editor *TextView
	grid   *Table
	bar    *Toolbar

	// errRect is the error-strip region between the editor and the grid,
	// re-derived on every SetBounds. It is painted by DatabaseEditor itself
	// (not a child widget), so a click there routes to nothing.
	errRect Rect

	// lastErr is the most recent operation error (schema load / query / exec),
	// or nil once an operation succeeds. Painted in the error strip and, when it
	// transitions to non-nil, delivered to OnError.
	lastErr error

	// OnCellEdit forwards the results grid's committed inline edits: row and col
	// index into the last result set plus the new value. Nil is safe.
	OnCellEdit func(row, col int, value string)

	// OnError fires when an operation fails, with the surfaced error. Nil is
	// safe; the error is painted in the error strip regardless of this hook.
	OnError func(err error)

	// OnQuery fires after a successful Run with the fetched result set, so a host
	// can update a status bar ("42 rows"). Nil is safe.
	OnQuery func(columns []string, rows [][]string)

	// Layout metrics. A non-positive value selects the constant default, so a
	// zero-valued field is the stock layout (see the dbEditorDefault* consts).
	TreeWidth    int
	BarHeight    int
	EditorHeight int
	ErrorHeight  int
}

// DataSource is the driver-agnostic seam a DatabaseEditor renders over. The
// toolkit deliberately keeps it tiny so the widget stays a lean pure-UI layer:
// a host injects an adapter that maps these methods onto a real engine, and a
// test injects an in-memory fake.
type DataSource interface {
	// Schema returns the object tree the left pane renders: databases, each
	// with its tables/views, each with its columns.
	Schema() (Schema, error)
	// Query runs a read statement and returns the result set that fills the
	// grid: the column titles and the rows (each already stringified, one cell
	// per column).
	Query(sql string) (columns []string, rows [][]string, err error)
}

// Execer is the OPTIONAL write half of a DataSource. A source that also runs
// non-result statements (INSERT / UPDATE / DELETE / DDL) implements it;
// DatabaseEditor detects it with a type assertion so the core DataSource
// interface stays lean and a read-only source need not implement it.
type Execer interface {
	// Exec runs a statement that returns no rows and reports how many were
	// affected.
	Exec(sql string) (affected int64, err error)
}

// Schema is the whole object tree a DataSource exposes.
type Schema struct {
	Databases []DatabaseInfo
}

// DatabaseInfo is one database (schema / catalog) and its tables and views.
type DatabaseInfo struct {
	Name   string
	Tables []TableInfo
}

// TableInfo is one table or view and its columns.
type TableInfo struct {
	Name    string
	IsView  bool
	Columns []ColumnInfo
}

// ColumnInfo is one column of a table or view.
type ColumnInfo struct {
	Name string
	// Type is the optional SQL data type (e.g. "INTEGER", "TEXT"); "" when the
	// adapter does not report one.
	Type string
}

// Default layout metrics, in logical pixels (scaled to device pixels at draw
// time). Selected whenever the matching field is non-positive.
const (
	dbEditorDefaultTreeWidth    = 200
	dbEditorDefaultBarHeight    = 28
	dbEditorDefaultEditorHeight = 120
	dbEditorDefaultErrorHeight  = 18
)

// dbErrorInk is the ink the error strip paints a surfaced error in — the same
// brick red the Table's rejected-edit border uses, so an error reads
// consistently across the toolkit.
var dbErrorInk = RGB(0xC0, 0x30, 0x30)

// errNoExec is returned by Exec when the injected DataSource does not implement
// the optional Execer half.
var errNoExec = errors.New("data source does not support Exec")

// NewDatabaseEditor builds a DatabaseEditor over source and eagerly loads its
// schema into the tree. A schema-load error is surfaced (lastErr / OnError) but
// does not stop construction, so a caller always gets a usable widget it can
// Refresh later. source must be non-nil.
func NewDatabaseEditor(source DataSource) *DatabaseEditor {
	d := &DatabaseEditor{source: source}

	d.tree = NewTreeView(nil)
	d.editor = NewTextView("")
	d.editor.Highlighter = SQLHighlight
	d.grid = NewTable(nil, nil)
	d.grid.OnCellEdit = func(row, col int, value string) {
		if d.OnCellEdit != nil {
			d.OnCellEdit(row, col, value)
		}
	}
	d.bar = NewToolbar([]ToolbarItem{
		{Label: "Run", OnClick: d.Run},
		{Separator: true},
		{Label: "Refresh", OnClick: func() { d.Refresh() }},
	})

	d.Refresh()
	return d
}

// Tree is the schema/object TreeView (left pane). Exposed so a host wires
// selection (Tree().OnActivate) — e.g. to seed a "SELECT * FROM <table>" query.
func (d *DatabaseEditor) Tree() *TreeView { return d.tree }

// Editor is the SQL editor pane (top-right). A host reads Editor().Text().Get() or
// seeds it with SetText. Swapping this TextView for a future CodeEditor widget
// is a follow-up; the highlighter seam (SQLHighlight) already lives here.
func (d *DatabaseEditor) Editor() *TextView { return d.editor }

// Grid is the results Table (bottom-right). Its columns are made Editable by
// Run so a cell edit fires OnCellEdit.
func (d *DatabaseEditor) Grid() *Table { return d.grid }

// Toolbar is the run/execute action strip (top). Item 0 is Run, item 2 is
// Refresh (item 1 is a separator).
func (d *DatabaseEditor) Toolbar() *Toolbar { return d.bar }

// Err reports the most recent operation error, or nil once an operation
// succeeded. It is the programmatic counterpart of the painted error strip.
func (d *DatabaseEditor) Err() error { return d.lastErr }

// SQL returns the current text of the SQL editor.
func (d *DatabaseEditor) SQL() string { return d.editor.Text().Get() }

// SetSQL replaces the SQL editor's text.
func (d *DatabaseEditor) SetSQL(sql string) { d.editor.SetText(sql) }

// Refresh reloads the schema from the source and rebuilds the tree. On error it
// surfaces the error (setError) and leaves the previous tree in place, returning
// the error so a caller can react.
func (d *DatabaseEditor) Refresh() error {
	s, err := d.source.Schema()
	if err != nil {
		d.setError(err)
		return err
	}
	d.tree.Root = buildSchemaTree(s)
	d.setError(nil)
	return nil
}

// Run executes the SQL editor's current text as a query and fills the results
// grid. A query error is surfaced and leaves the previous grid contents intact;
// a success clears the error, replaces the grid's columns + rows and fires
// OnQuery.
func (d *DatabaseEditor) Run() {
	cols, rows, err := d.source.Query(d.editor.Text().Get())
	if err != nil {
		d.setError(err)
		return
	}
	d.setError(nil)
	d.fillGrid(cols, rows)
	if d.OnQuery != nil {
		d.OnQuery(cols, rows)
	}
}

// Exec runs the SQL editor's text as a non-result statement through the
// source's optional Execer half. It reports the affected-row count and true on
// success; on a missing Execer or an execution error it surfaces the error and
// returns false.
func (d *DatabaseEditor) Exec() (affected int64, ok bool) {
	ex, isExecer := d.source.(Execer)
	if !isExecer {
		d.setError(errNoExec)
		return 0, false
	}
	n, err := ex.Exec(d.editor.Text().Get())
	if err != nil {
		d.setError(err)
		return 0, false
	}
	d.setError(nil)
	return n, true
}

// setError records err as the current error and, when err is a NEW non-nil
// error, delivers it to OnError. Passing nil clears the error without firing the
// hook, so a success silently resets the strip.
func (d *DatabaseEditor) setError(err error) {
	d.lastErr = err
	if err != nil && d.OnError != nil {
		d.OnError(err)
	}
}

// fillGrid rebuilds the results Table from a query's column titles + rows. Every
// column is made Editable so an in-place edit fires the grid's OnCellEdit (which
// forwards to DatabaseEditor.OnCellEdit).
func (d *DatabaseEditor) fillGrid(cols []string, rows [][]string) {
	tcols := make([]TableColumn, len(cols))
	for i, c := range cols {
		tcols[i] = TableColumn{Title: c, Editable: true}
	}
	d.grid.Columns = tcols
	d.grid.Rows = rows
	d.grid.Selected().Set(-1)
	d.grid.ScrollRow().Set(0)
}

// buildSchemaTree turns a Schema into the TreeNode hierarchy the left pane
// renders: a fixed "Databases" root, one expanded node per database, one node
// per table/view (views suffixed " (view)"), and one leaf per column
// ("name : TYPE" when a type is known, else just the name).
func buildSchemaTree(s Schema) *TreeNode {
	root := &TreeNode{Label: dbSchemaRootLabel, Expanded: true}
	for _, db := range s.Databases {
		dbNode := &TreeNode{Label: db.Name, Expanded: true, Data: db.Name}
		for _, tbl := range db.Tables {
			label := tbl.Name
			if tbl.IsView {
				label += dbViewSuffix
			}
			tblNode := &TreeNode{Label: label, Data: tbl.Name}
			for _, col := range tbl.Columns {
				cl := col.Name
				if col.Type != "" {
					cl = col.Name + " : " + col.Type
				}
				tblNode.Children = append(tblNode.Children, &TreeNode{Label: cl, Data: col.Name})
			}
			dbNode.Children = append(dbNode.Children, tblNode)
		}
		root.Children = append(root.Children, dbNode)
	}
	return root
}

const (
	// dbSchemaRootLabel is the label of the tree's synthetic top node.
	dbSchemaRootLabel = "Databases"
	// dbViewSuffix marks a view apart from a base table in the tree.
	dbViewSuffix = " (view)"
)

// SetBounds positions every child region. Bounds in this toolkit are absolute
// (surface) coordinates, so children receive absolute rects (see WalkA11y).
func (d *DatabaseEditor) SetBounds(r Rect) {
	d.Base.SetBounds(r)
	d.relayout()
}

// relayout derives each child's absolute rect from the current bounds + metrics.
// Pure arithmetic: no clamping branches, so a caller sizing the widget too small
// simply gets thin regions rather than a special case.
func (d *DatabaseEditor) relayout() {
	r := d.Bounds()
	bh := scaled(dbEditorMetric(d.BarHeight, dbEditorDefaultBarHeight))
	tw := scaled(dbEditorMetric(d.TreeWidth, dbEditorDefaultTreeWidth))
	eh := scaled(dbEditorMetric(d.EditorHeight, dbEditorDefaultEditorHeight))
	erh := scaled(dbEditorMetric(d.ErrorHeight, dbEditorDefaultErrorHeight))

	// Toolbar: full-width strip across the top.
	d.bar.SetBounds(Rect{X: r.X, Y: r.Y, W: r.W, H: bh})

	bodyY := r.Y + bh
	bodyH := r.H - bh

	// Tree: left column of the body.
	d.tree.SetBounds(Rect{X: r.X, Y: bodyY, W: tw, H: bodyH})

	rightX := r.X + tw
	rightW := r.W - tw

	// SQL editor: top of the right column.
	d.editor.SetBounds(Rect{X: rightX, Y: bodyY, W: rightW, H: eh})

	// Error strip: a self-painted band under the editor.
	errY := bodyY + eh
	d.errRect = Rect{X: rightX, Y: errY, W: rightW, H: erh}

	// Results grid: the remainder of the right column.
	gridY := errY + erh
	d.grid.SetBounds(Rect{X: rightX, Y: gridY, W: rightW, H: bodyH - eh - erh})
}

// dbEditorMetric returns v when positive, else the supplied default — the
// "0 means default" convention every layout field uses.
func dbEditorMetric(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// Draw paints the toolbar, tree, SQL editor, error strip and results grid.
func (d *DatabaseEditor) Draw(p painter.Painter, theme *Theme) {
	d.bar.Draw(p, theme)
	d.tree.Draw(p, theme)
	d.editor.Draw(p, theme)
	d.drawErrorStrip(p, theme)
	d.grid.Draw(p, theme)
}

// drawErrorStrip fills the error band with the SurfaceAlt tone and, when an
// error is current, paints its message in dbErrorInk.
func (d *DatabaseEditor) drawErrorStrip(p painter.Painter, theme *Theme) {
	r := d.errRect
	fillRect(p, r.X, r.Y, r.W, r.H, theme.SurfaceAlt)
	if d.lastErr != nil {
		ty := r.Y + (r.H-d.glyphHeight())/2
		d.drawText(p, r.X+scaled(4), ty, d.lastErr.Error(), dbErrorInk)
	}
}

// Children yields the interactive child widgets in visual order so the a11y
// walker (WalkA11y) descends into them. The error strip is self-painted, not a
// child, so it is not listed.
func (d *DatabaseEditor) Children() []Widget {
	return []Widget{d.bar, d.tree, d.editor, d.grid}
}

// A11y reports the DatabaseEditor as a labelled group; WalkA11y then descends
// through Children to announce the toolbar, tree, editor and grid.
func (d *DatabaseEditor) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: "Database editor"}
}

// OnEvent routes a widget-local event to the child whose absolute bounds contain
// it, translated into that child's local space. A click that lands on the
// self-painted error strip (no child) is a no-op.
func (d *DatabaseEditor) OnEvent(ev Event) {
	pr := d.Bounds()
	sx, sy := ev.X+pr.X, ev.Y+pr.Y
	for _, child := range d.Children() {
		b := child.Bounds()
		if b.W > 0 && b.H > 0 && b.Contains(sx, sy) {
			child.OnEvent(translateEvent(ev, pr, b))
			return
		}
	}
}

// Compile-time checks that DatabaseEditor participates in the toolkit's core
// contracts.
var (
	_ Widget         = (*DatabaseEditor)(nil)
	_ Accessible     = (*DatabaseEditor)(nil)
	_ childContainer = (*DatabaseEditor)(nil)
)

// --- SQL syntax highlighting -------------------------------------------------

// SQL highlight inks. Fixed rather than theme-derived because the TextView
// Highlighter seam is called without a theme; the tones are chosen to read on
// both the light and dark default themes.
var (
	sqlKeywordColor = RGB(0x00, 0x5C, 0xC8) // blue
	sqlStringColor  = RGB(0x0A, 0x80, 0x0A) // green
	sqlNumberColor  = RGB(0x0A, 0x60, 0x90) // teal
	sqlCommentColor = RGB(0x80, 0x80, 0x80) // grey
)

// sqlKeywords is the reserved-word set SQLHighlight paints in sqlKeywordColor.
// Matching is case-insensitive (the lexer upper-cases the identifier first).
var sqlKeywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "INSERT": true, "INTO": true,
	"VALUES": true, "UPDATE": true, "SET": true, "DELETE": true, "CREATE": true,
	"TABLE": true, "VIEW": true, "DROP": true, "ALTER": true, "JOIN": true,
	"LEFT": true, "RIGHT": true, "INNER": true, "OUTER": true, "ON": true,
	"GROUP": true, "BY": true, "ORDER": true, "HAVING": true, "LIMIT": true,
	"OFFSET": true, "AS": true, "AND": true, "OR": true, "NOT": true,
	"NULL": true, "IS": true, "IN": true, "LIKE": true, "DISTINCT": true,
	"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true,
	"INDEX": true, "PRIMARY": true, "KEY": true, "FOREIGN": true, "REFERENCES": true,
	"UNION": true, "ALL": true, "ASC": true, "DESC": true, "BETWEEN": true,
}

// SQLHighlight is a TextView.Highlighter that colours one line of SQL: line
// comments (-- to end of line), single-quoted string literals, numeric
// literals, and reserved keywords. Any run it does not classify keeps the
// default ink. It is line-local (no multi-line string / block-comment state),
// which matches the TextView's per-line Highlighter contract.
func SQLHighlight(_ int, line string) []TextSpan {
	var spans []TextSpan
	r := []rune(line)
	i := 0
	for i < len(r) {
		c := r[i]
		switch {
		case c == '-' && i+1 < len(r) && r[i+1] == '-':
			// Line comment: everything to the end of the line.
			spans = append(spans, TextSpan{Start: i, End: len(r), Color: sqlCommentColor})
			i = len(r)
		case c == '\'':
			// String literal: to the closing quote (or end of line if unterminated).
			j := i + 1
			for j < len(r) && r[j] != '\'' {
				j++
			}
			if j < len(r) {
				j++ // include the closing quote
			}
			spans = append(spans, TextSpan{Start: i, End: j, Color: sqlStringColor})
			i = j
		case isSQLDigit(c):
			j := i
			for j < len(r) && (isSQLDigit(r[j]) || r[j] == '.') {
				j++
			}
			spans = append(spans, TextSpan{Start: i, End: j, Color: sqlNumberColor})
			i = j
		case isSQLIdentStart(c):
			j := i
			for j < len(r) && isSQLIdentPart(r[j]) {
				j++
			}
			if sqlKeywords[strings.ToUpper(string(r[i:j]))] {
				spans = append(spans, TextSpan{Start: i, End: j, Color: sqlKeywordColor})
			}
			i = j
		default:
			i++
		}
	}
	return spans
}

// isSQLDigit reports whether c is an ASCII decimal digit.
func isSQLDigit(c rune) bool { return c >= '0' && c <= '9' }

// isSQLIdentStart reports whether c can begin a SQL identifier/keyword.
func isSQLIdentStart(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isSQLIdentPart reports whether c can continue a SQL identifier/keyword.
func isSQLIdentPart(c rune) bool { return isSQLIdentStart(c) || isSQLDigit(c) }

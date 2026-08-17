// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"errors"
	"reflect"
	"testing"
)

// --- in-memory fake DataSource (the test instrument) -------------------------

// fakeDataSource is a canned, driver-free DataSource: it returns a fixed schema
// and result set (or a pre-seeded error) so the DatabaseEditor tests exercise
// pure UI over the seam with no real database. It deliberately does NOT
// implement Execer, so it also serves the "Exec unsupported" branch.
type fakeDataSource struct {
	schema    Schema
	schemaErr error

	cols     []string
	rows     [][]string
	queryErr error

	lastQuery string
}

func (f *fakeDataSource) Schema() (Schema, error) { return f.schema, f.schemaErr }

func (f *fakeDataSource) Query(sql string) ([]string, [][]string, error) {
	f.lastQuery = sql
	if f.queryErr != nil {
		return nil, nil, f.queryErr
	}
	return f.cols, f.rows, nil
}

// fakeExecSource additionally implements Execer, for the Exec branches.
type fakeExecSource struct {
	fakeDataSource
	execN    int64
	execErr  error
	lastExec string
}

func (f *fakeExecSource) Exec(sql string) (int64, error) {
	f.lastExec = sql
	if f.execErr != nil {
		return 0, f.execErr
	}
	return f.execN, nil
}

// cannedSchema is one database with a base table (typed columns), a view
// (untyped column) — enough to cover every buildSchemaTree branch.
func cannedSchema() Schema {
	return Schema{Databases: []DatabaseInfo{{
		Name: "main",
		Tables: []TableInfo{
			{Name: "users", Columns: []ColumnInfo{
				{Name: "id", Type: "INTEGER"},
				{Name: "name", Type: "TEXT"},
			}},
			{Name: "active_users", IsView: true, Columns: []ColumnInfo{
				{Name: "id"}, // no Type → bare-name leaf
			}},
		},
	}}}
}

// newCannedSource builds a fake wired with the canned schema + a 2x2 result set.
func newCannedSource() *fakeDataSource {
	return &fakeDataSource{
		schema: cannedSchema(),
		cols:   []string{"id", "name"},
		rows:   [][]string{{"1", "Alice"}, {"2", "Bob"}},
	}
}

// dbScanColor reports whether any pixel inside r exactly equals c. r must lie
// within the w-wide surface.
func dbScanColor(buf []byte, w int, r Rect, c RGBA) bool {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if pixelAt(buf, w, x, y) == c {
				return true
			}
		}
	}
	return false
}

// --- CONTROL RUN: validate the test instruments before relying on them -------

// TestDBEditorInstrumentControl proves the fake DataSource, the pixel scanner
// and the SQL highlighter are known-good BEFORE the widget tests depend on
// them, per the org "control-run new instruments" standard.
func TestDBEditorInstrumentControl(t *testing.T) {
	// (1) The fake returns exactly the canned data, unmediated by any widget.
	src := newCannedSource()
	s, err := src.Schema()
	if err != nil {
		t.Fatalf("control: fake Schema errored: %v", err)
	}
	if len(s.Databases) != 1 || s.Databases[0].Name != "main" ||
		len(s.Databases[0].Tables) != 2 {
		t.Fatalf("control: canned schema shape wrong: %+v", s)
	}
	cols, rows, err := src.Query("SELECT 1")
	if err != nil {
		t.Fatalf("control: fake Query errored: %v", err)
	}
	if !reflect.DeepEqual(cols, []string{"id", "name"}) ||
		!reflect.DeepEqual(rows, [][]string{{"1", "Alice"}, {"2", "Bob"}}) {
		t.Fatalf("control: canned result wrong: cols=%v rows=%v", cols, rows)
	}
	if src.lastQuery != "SELECT 1" {
		t.Fatalf("control: fake did not record the query: %q", src.lastQuery)
	}

	// (2) The pixel scanner finds a colour it is asked for and rejects one it is
	// not — validated against a plain fillRect of a KNOWN colour (the proven
	// raster primitive), so a later "error shows" assertion means what it says.
	const w, h = 20, 10
	buf := makeSurface(w, h)
	known := RGB(0x12, 0x34, 0x56)
	fillRect(newP(buf, w), 4, 2, 6, 5, known)
	if !dbScanColor(buf, w, Rect{X: 4, Y: 2, W: 6, H: 5}, known) {
		t.Fatal("control: scanner missed a colour that IS present")
	}
	if dbScanColor(buf, w, Rect{X: 4, Y: 2, W: 6, H: 5}, RGB(0x99, 0x99, 0x99)) {
		t.Fatal("control: scanner reported a colour that is NOT present")
	}

	// (3) The highlighter classifies a lone keyword at the exact span — the
	// instrument the SQL-editor test leans on.
	spans := SQLHighlight(0, "SELECT")
	if len(spans) != 1 || spans[0] != (TextSpan{Start: 0, End: 6, Color: sqlKeywordColor}) {
		t.Fatalf("control: SELECT span = %+v, want one keyword span [0,6)", spans)
	}
}

// --- construction ------------------------------------------------------------

func TestDBEditorConstructionLoadsSchema(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())

	// The schema tree renders the canned schema EXACTLY (this is "the tree
	// renders the schema", asserted structurally rather than by eyeball).
	root := d.Tree().Root
	if root == nil || root.Label != dbSchemaRootLabel || !root.Expanded {
		t.Fatalf("root = %+v, want expanded %q", root, dbSchemaRootLabel)
	}
	if len(root.Children) != 1 || root.Children[0].Label != "main" {
		t.Fatalf("db node = %+v, want single 'main'", root.Children)
	}
	db := root.Children[0]
	if !db.Expanded || db.Data != "main" {
		t.Fatalf("db node not expanded / wrong Data: %+v", db)
	}
	if len(db.Children) != 2 {
		t.Fatalf("want 2 tables, got %d", len(db.Children))
	}
	users, view := db.Children[0], db.Children[1]
	if users.Label != "users" || users.Data != "users" {
		t.Fatalf("table node = %+v, want 'users'", users)
	}
	if view.Label != "active_users"+dbViewSuffix {
		t.Fatalf("view node label = %q, want view-suffixed", view.Label)
	}
	// Typed columns render "name : TYPE"; the untyped view column renders bare.
	if len(users.Children) != 2 ||
		users.Children[0].Label != "id : INTEGER" ||
		users.Children[1].Label != "name : TEXT" {
		t.Fatalf("column leaves = %+v, want typed labels", users.Children)
	}
	if users.Children[0].Data != "id" {
		t.Fatalf("column Data = %v, want 'id'", users.Children[0].Data)
	}
	if len(view.Children) != 1 || view.Children[0].Label != "id" {
		t.Fatalf("view column leaf = %+v, want bare 'id'", view.Children)
	}
	if d.Err() != nil {
		t.Fatalf("clean construction left an error: %v", d.Err())
	}
	// Accessors return the composed widgets.
	if d.Editor() == nil || d.Grid() == nil || d.Toolbar() == nil {
		t.Fatal("accessors returned nil composed widgets")
	}
	if d.Editor().Highlighter == nil {
		t.Fatal("SQL editor has no highlighter wired")
	}
}

func TestDBEditorConstructionSchemaErrorSurfaced(t *testing.T) {
	boom := errors.New("no schema")
	src := &fakeDataSource{schemaErr: boom}
	d := NewDatabaseEditor(src) // OnError is nil here → setError's no-hook branch
	if d.Err() != boom {
		t.Fatalf("schema error not surfaced: %v", d.Err())
	}
	if d.Tree().Root != nil {
		t.Fatal("failed schema load should leave the tree empty")
	}
}

// --- layout: exact bounds ----------------------------------------------------

func TestDBEditorLayoutDefaultBounds(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 10, Y: 20, W: 600, H: 400})

	// Defaults: bar 28, tree 200, editor 120, error 18 (metricScale == 1).
	want := map[string]Rect{
		"bar":    {X: 10, Y: 20, W: 600, H: 28},
		"tree":   {X: 10, Y: 48, W: 200, H: 372},
		"editor": {X: 210, Y: 48, W: 400, H: 120},
		"grid":   {X: 210, Y: 186, W: 400, H: 234},
	}
	if got := d.Toolbar().Bounds(); got != want["bar"] {
		t.Fatalf("bar bounds = %+v, want %+v", got, want["bar"])
	}
	if got := d.Tree().Bounds(); got != want["tree"] {
		t.Fatalf("tree bounds = %+v, want %+v", got, want["tree"])
	}
	if got := d.Editor().Bounds(); got != want["editor"] {
		t.Fatalf("editor bounds = %+v, want %+v", got, want["editor"])
	}
	if got := d.Grid().Bounds(); got != want["grid"] {
		t.Fatalf("grid bounds = %+v, want %+v", got, want["grid"])
	}
	// The error strip sits exactly between editor and grid.
	if d.errRect != (Rect{X: 210, Y: 168, W: 400, H: 18}) {
		t.Fatalf("errRect = %+v, want {210,168,400,18}", d.errRect)
	}
}

func TestDBEditorLayoutCustomMetrics(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.TreeWidth, d.BarHeight, d.EditorHeight, d.ErrorHeight = 150, 30, 100, 20
	d.SetBounds(Rect{X: 0, Y: 0, W: 500, H: 300})

	if got := d.Toolbar().Bounds(); got != (Rect{X: 0, Y: 0, W: 500, H: 30}) {
		t.Fatalf("custom bar bounds = %+v", got)
	}
	if got := d.Tree().Bounds(); got != (Rect{X: 0, Y: 30, W: 150, H: 270}) {
		t.Fatalf("custom tree bounds = %+v", got)
	}
	if got := d.Editor().Bounds(); got != (Rect{X: 150, Y: 30, W: 350, H: 100}) {
		t.Fatalf("custom editor bounds = %+v", got)
	}
	// error strip: y = 30 + 100 = 130, h = 20; grid fills the rest.
	if d.errRect != (Rect{X: 150, Y: 130, W: 350, H: 20}) {
		t.Fatalf("custom errRect = %+v", d.errRect)
	}
	if got := d.Grid().Bounds(); got != (Rect{X: 150, Y: 150, W: 350, H: 150}) {
		t.Fatalf("custom grid bounds = %+v", got)
	}
}

// --- draw: schema on screen + error strip ------------------------------------

func TestDBEditorDrawRendersSchemaAndClearStrip(t *testing.T) {
	const w, h = 600, 400
	theme := DefaultLight()
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)

	// The tree pane painted node text (OnSurface ink) — the schema is on screen.
	if !dbScanColor(buf, w, d.Tree().Bounds(), theme.OnSurface) {
		t.Fatal("tree pane painted no schema text")
	}
	// With no error, the strip carries NO error ink.
	if dbScanColor(buf, w, d.errRect, dbErrorInk) {
		t.Fatal("error ink present despite no error")
	}
}

func TestDBEditorErrorShows(t *testing.T) {
	const w, h = 600, 400
	theme := DefaultLight()
	src := newCannedSource()
	src.queryErr = errors.New("boom")

	var gotErr error
	d := NewDatabaseEditor(src)
	d.OnError = func(e error) { gotErr = e } // setError's hook branch
	d.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	d.SetSQL("SELECT bad")

	d.Run()
	if d.Err() == nil || d.Err().Error() != "boom" {
		t.Fatalf("Run should have surfaced the query error, got %v", d.Err())
	}
	if gotErr == nil || gotErr.Error() != "boom" {
		t.Fatalf("OnError not fired with the query error, got %v", gotErr)
	}

	buf := makeSurface(w, h)
	d.Draw(newP(buf, w), theme)
	if !dbScanColor(buf, w, d.errRect, dbErrorInk) {
		t.Fatal("error strip did not paint the error in red")
	}
}

// --- run: fills the grid at exact cells --------------------------------------

func TestDBEditorRunFillsGrid(t *testing.T) {
	src := newCannedSource()
	var qCols []string
	var qRows [][]string
	d := NewDatabaseEditor(src)
	d.OnQuery = func(c []string, r [][]string) { qCols, qRows = c, r }
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})
	d.SetSQL("SELECT id, name FROM users")

	d.Run()

	if src.lastQuery != "SELECT id, name FROM users" {
		t.Fatalf("source saw query %q", src.lastQuery)
	}
	g := d.Grid()
	if len(g.Columns) != 2 || g.Columns[0].Title != "id" || g.Columns[1].Title != "name" {
		t.Fatalf("grid columns = %+v, want id/name", g.Columns)
	}
	// Both columns must be Editable so a cell edit can fire the callback.
	if !g.Columns[0].Editable || !g.Columns[1].Editable {
		t.Fatalf("result columns not Editable: %+v", g.Columns)
	}
	// Exact-cell content.
	if g.Rows[0][0] != "1" || g.Rows[0][1] != "Alice" ||
		g.Rows[1][0] != "2" || g.Rows[1][1] != "Bob" {
		t.Fatalf("grid cells = %+v, want the canned 2x2", g.Rows)
	}
	if g.Selected != -1 || g.ScrollRow != 0 {
		t.Fatalf("grid not reset after fill: Selected=%d ScrollRow=%d", g.Selected, g.ScrollRow)
	}
	// OnQuery observed the same result set.
	if !reflect.DeepEqual(qCols, []string{"id", "name"}) || len(qRows) != 2 {
		t.Fatalf("OnQuery got cols=%v rows=%v", qCols, qRows)
	}
	if d.Err() != nil {
		t.Fatalf("successful Run left an error: %v", d.Err())
	}
}

// TestDBEditorEditedCellFiresCallback drives a REAL inline edit through the
// results grid and asserts the DatabaseEditor's OnCellEdit forwards it.
func TestDBEditorEditedCellFiresCallback(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})
	d.SetSQL("SELECT id, name FROM users")
	d.Run()

	var gotRow, gotCol int
	var gotVal string
	fired := 0
	d.OnCellEdit = func(row, col int, value string) {
		fired++
		gotRow, gotCol, gotVal = row, col, value
	}

	g := d.Grid()
	g.BeginEdit(1, 1)                            // edit "Bob"
	g.OnEvent(Event{Kind: EventChar, Code: "X"}) // type → "BobX"
	g.CommitEdit()

	if fired != 1 || gotRow != 1 || gotCol != 1 || gotVal != "BobX" {
		t.Fatalf("forwarded edit = (%d,%d,%q) fired=%d, want (1,1,\"BobX\") fired=1",
			gotRow, gotCol, gotVal, fired)
	}
	if g.Rows[1][1] != "BobX" {
		t.Fatalf("grid cell not updated: %q", g.Rows[1][1])
	}
}

// TestDBEditorEditWithoutCallbackNoPanic covers the nil-OnCellEdit branch of the
// forwarding closure.
func TestDBEditorEditWithoutCallbackNoPanic(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})
	d.SetSQL("SELECT id, name FROM users")
	d.Run()
	g := d.Grid()
	g.BeginEdit(0, 0)
	g.CommitEdit() // OnCellEdit is nil on the DatabaseEditor → closure no-ops
	if g.Rows[0][0] != "1" {
		t.Fatalf("unexpected cell mutation: %q", g.Rows[0][0])
	}
}

// --- SQL / SetSQL / Refresh --------------------------------------------------

func TestDBEditorSQLRoundTrip(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetSQL("SELECT 42")
	if d.SQL() != "SELECT 42" {
		t.Fatalf("SQL round-trip = %q", d.SQL())
	}
}

func TestDBEditorRefreshReloads(t *testing.T) {
	src := newCannedSource()
	d := NewDatabaseEditor(src)
	// Swap the schema underneath, then Refresh.
	src.schema = Schema{Databases: []DatabaseInfo{{Name: "other"}}}
	if err := d.Refresh(); err != nil {
		t.Fatalf("Refresh errored: %v", err)
	}
	if d.Tree().Root.Children[0].Label != "other" {
		t.Fatalf("Refresh did not reload schema: %+v", d.Tree().Root.Children)
	}
}

func TestDBEditorRefreshErrorKeepsTree(t *testing.T) {
	src := newCannedSource()
	d := NewDatabaseEditor(src)
	before := d.Tree().Root
	src.schemaErr = errors.New("gone")
	if err := d.Refresh(); err == nil {
		t.Fatal("Refresh should return the schema error")
	}
	if d.Tree().Root != before {
		t.Fatal("failed Refresh must leave the previous tree in place")
	}
	if d.Err() == nil {
		t.Fatal("failed Refresh did not surface the error")
	}
}

// --- Exec (optional Execer half) ---------------------------------------------

func TestDBEditorExecUnsupported(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource()) // fakeDataSource is NOT an Execer
	n, ok := d.Exec()
	if ok || n != 0 {
		t.Fatalf("Exec on a non-Execer = (%d,%v), want (0,false)", n, ok)
	}
	if d.Err() != errNoExec {
		t.Fatalf("Err = %v, want errNoExec", d.Err())
	}
}

func TestDBEditorExecSuccess(t *testing.T) {
	src := &fakeExecSource{execN: 3}
	src.schema = cannedSchema()
	d := NewDatabaseEditor(src)
	d.SetSQL("DELETE FROM users")
	n, ok := d.Exec()
	if !ok || n != 3 {
		t.Fatalf("Exec = (%d,%v), want (3,true)", n, ok)
	}
	if src.lastExec != "DELETE FROM users" {
		t.Fatalf("Exec saw %q", src.lastExec)
	}
	if d.Err() != nil {
		t.Fatalf("successful Exec left an error: %v", d.Err())
	}
}

func TestDBEditorExecError(t *testing.T) {
	src := &fakeExecSource{execErr: errors.New("denied")}
	src.schema = cannedSchema()
	d := NewDatabaseEditor(src)
	n, ok := d.Exec()
	if ok || n != 0 {
		t.Fatalf("failed Exec = (%d,%v), want (0,false)", n, ok)
	}
	if d.Err() == nil || d.Err().Error() != "denied" {
		t.Fatalf("Exec error not surfaced: %v", d.Err())
	}
}

// --- event routing -----------------------------------------------------------

func TestDBEditorToolbarRunAndRefresh(t *testing.T) {
	src := newCannedSource()
	d := NewDatabaseEditor(src)
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})
	d.SetSQL("SELECT id, name FROM users")

	// Click the Run button (toolbar item 0, ~24px wide) — routes through OnEvent
	// into the toolbar, which fires the wired Run action.
	d.OnEvent(Event{Kind: EventClick, X: 12, Y: 14})
	if src.lastQuery != "SELECT id, name FROM users" {
		t.Fatalf("Run toolbar click did not query; lastQuery=%q", src.lastQuery)
	}

	// Click the Refresh button (item 2, after the 24px Run + 8px separator).
	src.schema = Schema{Databases: []DatabaseInfo{{Name: "reloaded"}}}
	d.OnEvent(Event{Kind: EventClick, X: 40, Y: 14})
	if d.Tree().Root.Children[0].Label != "reloaded" {
		t.Fatalf("Refresh toolbar click did not reload: %+v", d.Tree().Root.Children)
	}
}

func TestDBEditorEventRoutesToPanes(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})

	// A click in the SQL editor focuses it (routed + translated into its space).
	d.OnEvent(Event{Kind: EventClick, X: 300, Y: 60})
	if !d.Editor().Focused {
		t.Fatal("click over the editor pane did not reach it")
	}

	// A click in the tree pane selects a node.
	d.OnEvent(Event{Kind: EventClick, X: 20, Y: 40})
	if d.Tree().Selected == nil {
		t.Fatal("click over the tree pane did not reach it")
	}
}

func TestDBEditorEventOnErrorStripIsNoOp(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 0, Y: 0, W: 600, H: 400})
	// errRect (default metrics, origin at 0,0) is {200,148,390,18}: a click there
	// lands on no child, exercising OnEvent's fall-through path.
	before := d.Grid().Selected
	d.OnEvent(Event{Kind: EventClick, X: 250, Y: 155})
	if d.Grid().Selected != before {
		t.Fatal("a click on the (non-interactive) error strip changed grid state")
	}
}

// --- accessibility -----------------------------------------------------------

func TestDBEditorA11yGroupAndChildren(t *testing.T) {
	d := NewDatabaseEditor(newCannedSource())
	d.SetBounds(Rect{X: 5, Y: 6, W: 600, H: 400})

	if info := d.A11y(); info.Role != RoleGroup || info.Name != "Database editor" {
		t.Fatalf("A11y = %+v, want RoleGroup 'Database editor'", info)
	}

	nodes := WalkA11y(d)
	// First node is the group itself, at the widget's exact bounds.
	if len(nodes) == 0 || nodes[0].Role != RoleGroup {
		t.Fatalf("WalkA11y first node = %+v, want the group", nodes)
	}
	if nodes[0].Rect != (Rect{X: 5, Y: 6, W: 600, H: 400}) {
		t.Fatalf("group node rect = %+v, want the widget bounds", nodes[0].Rect)
	}
	// The walk descends into every interactive child.
	roles := map[Role]bool{}
	for _, n := range nodes {
		roles[n.Role] = true
	}
	for _, want := range []Role{RoleToolbar, RoleTree, RoleTextbox, RoleGrid} {
		if !roles[want] {
			t.Fatalf("WalkA11y did not descend to role %q; saw %v", want, roles)
		}
	}
	// Children() reports the four panes in visual order.
	kids := d.Children()
	if len(kids) != 4 || kids[0] != Widget(d.Toolbar()) || kids[3] != Widget(d.Grid()) {
		t.Fatalf("Children order wrong: %T..%T", kids[0], kids[3])
	}
}

// --- SQL highlighter ---------------------------------------------------------

func TestSQLHighlightBroadLine(t *testing.T) {
	// One line exercising: keyword, non-keyword ident, whitespace/punct (default),
	// number, string (terminated) and a line comment.
	line := "SELECT * FROM t WHERE id = 123 AND name = 'bob' -- note"
	spans := SQLHighlight(0, line)

	// Collapse to a colour lookup per rune for exact, position-precise checks.
	r := []rune(line)
	ink := make([]RGBA, len(r))
	for i := range ink {
		ink[i] = RGBA{} // "unclassified" sentinel
	}
	for _, s := range spans {
		for i := s.Start; i < s.End; i++ {
			ink[i] = s.Color
		}
	}
	idx := func(sub string) int { return runeIndex(line, sub) }

	if ink[idx("SELECT")] != sqlKeywordColor {
		t.Fatal("SELECT not keyword-coloured")
	}
	if ink[idx("FROM")] != sqlKeywordColor || ink[idx("WHERE")] != sqlKeywordColor ||
		ink[idx("AND")] != sqlKeywordColor {
		t.Fatal("a reserved keyword was missed")
	}
	// 't' and 'id' and 'name' are identifiers, not keywords → unclassified.
	if ink[idx("id")] != (RGBA{}) {
		t.Fatal("non-keyword identifier was wrongly coloured")
	}
	if ink[idx("123")] != sqlNumberColor {
		t.Fatal("number not coloured")
	}
	if ink[idx("'bob'")] != sqlStringColor || ink[idx("'bob'")+4] != sqlStringColor {
		t.Fatal("string literal (incl. closing quote) not coloured")
	}
	if ink[idx("-- note")] != sqlCommentColor || ink[len(r)-1] != sqlCommentColor {
		t.Fatal("line comment not coloured to end of line")
	}
	// The '*' and '=' punctuation stays unclassified (default branch).
	if ink[idx("*")] != (RGBA{}) {
		t.Fatal("punctuation was wrongly coloured")
	}
}

func TestSQLHighlightUnterminatedString(t *testing.T) {
	// No closing quote → the string span runs to end of line (the j==len branch).
	spans := SQLHighlight(0, "x 'abc")
	found := false
	for _, s := range spans {
		if s.Color == sqlStringColor {
			if s.Start != 2 || s.End != 6 {
				t.Fatalf("unterminated string span = [%d,%d), want [2,6)", s.Start, s.End)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("unterminated string was not coloured")
	}
}

func TestSQLHighlightEmptyLine(t *testing.T) {
	if spans := SQLHighlight(0, ""); len(spans) != 0 {
		t.Fatalf("empty line produced spans: %+v", spans)
	}
}

func TestSQLIdentAndDigitPredicates(t *testing.T) {
	// Directly pin the character-class helpers at their true/false edges.
	cases := []struct {
		c                       rune
		digit, identSt, identPt bool
	}{
		{'0', true, false, true},
		{'9', true, false, true},
		{'a', false, true, true},
		{'Z', false, true, true},
		{'_', false, true, true},
		{' ', false, false, false},
		{'*', false, false, false},
	}
	for _, tc := range cases {
		if isSQLDigit(tc.c) != tc.digit ||
			isSQLIdentStart(tc.c) != tc.identSt ||
			isSQLIdentPart(tc.c) != tc.identPt {
			t.Fatalf("predicates for %q wrong", string(tc.c))
		}
	}
}

// runeIndex returns the rune index of the first occurrence of sub in s, or -1.
// (Byte index == rune index here since the test strings are ASCII, but the
// highlighter works in rune space, so map through runes explicitly.)
func runeIndex(s, sub string) int {
	rs, rsub := []rune(s), []rune(sub)
	for i := 0; i+len(rsub) <= len(rs); i++ {
		if string(rs[i:i+len(rsub)]) == sub {
			return i
		}
	}
	return -1
}

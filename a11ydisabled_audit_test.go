// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Is a disabled widget ANNOUNCED as disabled?
//
// [Base.Disabled] makes a widget inert: OnEvent early-returns and Draw paints a
// greyed face. A sighted user sees that. A screen-reader user hears it only if
// [A11yInfo.Disabled] says so, and each A11y() has to remember to copy the flag
// across -- #541 added the field and wired Entry and Toggle, #573 wired Button
// and DropDown, and a grep then found a dozen more that read Disabled in Draw
// and OnEvent and forgot it in A11y(). Fixing a dozen by hand leaves the next
// widget to forget in the same way, so this is the gate.
//
// Two halves, because a catalogue only covers the widgets someone remembered to
// list:
//
//   - TestA11yDisabledCatalogueIsComplete parses the package source and finds
//     every exported type that embeds Base (directly or through another type)
//     and answers A11y() (its own or a promoted one). Each must be in the
//     catalogue or in disabledA11yExempt with a reason. The first version
//     asked "does this type READ Disabled?" and a source scan cannot answer
//     that: it misses a read through a field chain, a reader promoted by
//     embedding, and a helper or closure. Embedding Base is decidable, so a NEW
//     widget fails here until someone decides.
//   - TestA11yDisabledAudit builds each catalogue widget, disables it and
//     requires A11y().Disabled, then enables it and requires the flag to clear.
//
// Unexported types (synthetic tab/ribbon/iso nodes, mini buttons) are reachable
// only through a parent's Children() and are not widgets a host builds, so they
// are outside the rule; the parents are audited.

// disabledA11yCatalogue maps a type name to a constructor for a fresh instance.
var disabledA11yCatalogue = map[string]func() Accessible{
	"Accordion":      func() Accessible { return NewAccordion([]AccordionSection{{Title: "a"}}) },
	"Browser":        func() Accessible { return NewBrowser() },
	"Button":         func() Accessible { return &Button{} },
	"CodeEditor":     func() Accessible { return NewCodeEditor("") },
	"Entry":          func() Accessible { return NewEntry("") },
	"TextView":       func() Accessible { return NewTextView("") },
	"Calendar":       func() Accessible { return NewCalendar(2026, 8, 16) },
	"Carousel":       func() Accessible { return NewCarousel(nil) },
	"CheckButton":    func() Accessible { return &CheckButton{} },
	"CodeMinimap":    func() Accessible { return NewCodeMinimap() },
	"ColumnBrowser":  func() Accessible { return &ColumnBrowser{} },
	"ComboBox":       func() Accessible { return &ComboBox{} },
	"CycleButton":    func() Accessible { return NewCycleButton("a", "b") },
	"DropDown":       func() Accessible { return &DropDown{} },
	"Expander":       func() Accessible { return NewExpander("e", nil) },
	"Fab":            func() Accessible { return NewFab("plus", nil) },
	"FileTree":       func() Accessible { return NewFileTree(nil) },
	"FolderTabs":     func() Accessible { return NewFolderTabs([]string{"a", "b"}, 0) },
	"GalleryView":    func() Accessible { return NewGalleryView() },
	"GraphView":      func() Accessible { return NewGraphView(NewGraphViewModel()) },
	"IconButton":     func() Accessible { return &IconButton{} },
	"IconGrid":       func() Accessible { return NewIconGrid() },
	"IsoDiagram":     func() Accessible { return NewIsoDiagram(nil) },
	"IsoIconPalette": func() Accessible { return NewIsoIconPalette(NewIsoIconRegistry()) },
	"Link":           func() Accessible { return NewLink("l", nil) },
	"ListBox":        func() Accessible { return NewListBox([]string{"a"}) },
	"Menu":           func() Accessible { return NewMenu([]MenuItem{{Label: "a"}}) },
	"MenuBar":        func() Accessible { return NewMenuBar() },
	"Notebook":       func() Accessible { return NewNotebook() },
	"PageStrip":      func() Accessible { return NewPageStrip(2) },
	"PagedView":      func() Accessible { return NewPagedView(nil) },
	"Pagination":     func() Accessible { return NewPagination(1, 5) },
	"PullToRefresh":  func() Accessible { return NewPullToRefresh(nil) },
	"RadioButton":    func() Accessible { return &RadioButton{} },
	"RangeSlider":    func() Accessible { return NewRangeSlider(0, 10, 2, 8) },
	"Rating":         func() Accessible { return NewRating(3, 5) },
	"Ribbon":         func() Accessible { return NewRibbon() },
	"Scale":          func() Accessible { return NewScale(0, 10, 5) },
	"Scrollbar":      func() Accessible { return &Scrollbar{} },
	"SourceList":     func() Accessible { return NewSourceList() },
	"SpinButton":     func() Accessible { return NewSpinButton(0, 10, 5, 1) },
	"SplitButton":    func() Accessible { return &SplitButton{} },
	"Statusbar":      func() Accessible { return NewStatusbar([]string{"a"}) },
	"SwipeActions":   func() Accessible { return NewSwipeActions(nil) },
	"Switch":         func() Accessible { return &Switch{} },
	"TabBar":         func() Accessible { return NewTabBar([]TabItem{{Label: "a"}}, 0) },
	"Table":          func() Accessible { return NewTable(nil, nil) },
	"Terminal":       func() Accessible { return NewTerminal(10, 4, 0) },
	"ToggleButton":   func() Accessible { return &ToggleButton{} },
	"TreeTable":      func() Accessible { return NewTreeTable(nil, nil) },
	"TreeView":       func() Accessible { return NewTreeView(&TreeNode{Label: "r"}) },
	"ViewSwitcher":   func() Accessible { return NewViewSwitcher([]string{"a", "b"}, 0) },
	"WheelPicker":    func() Accessible { return NewWheelPicker([]string{"a", "b"}) },
	"WorkspacePager": func() Accessible { return NewWorkspacePager(2, 0) },

	// Honoured in #578 part 4; behaviour is tested by TestDisabledWidgetsHonourTheFlag.
	"Dialog":      func() Accessible { return NewDialog("t", nil) },
	"Gantt":       func() Accessible { return NewGantt(nil) },
	"ModalWindow": func() Accessible { return NewModalWindow("t", nil) },
	"Paned":       func() Accessible { return NewHPaned(nil, nil) },
	"RichEditor":  func() Accessible { return NewRichEditor(nil) },
	"Spreadsheet": func() Accessible { return NewSpreadsheet(3, 3) },
	"Surface":     func() Accessible { return NewSurface(nil) },
	"Window":      func() Accessible { return NewWindow("t", nil) },

	// Honoured in #578 part 3; behaviour is tested by TestDisabledWidgetsHonourTheFlag.
	"Border":            func() Accessible { return NewBorder() },
	"FontChooser":       func() Accessible { return NewFontChooser(nil) },
	"FontPicker":        func() Accessible { return NewFontPicker([]string{"a", "b"}) },
	"ForgePane":         func() Accessible { return NewForgePane(newFakeForge()) },
	"Frame":             func() Accessible { return NewFrame(nil) },
	"Inspector":         func() Accessible { return NewInspector([]InspectorRow{{ID: "t", Kind: InspectorText}}) },
	"Kanban":            func() Accessible { return NewKanban(nil) },
	"LogView":           func() Accessible { return NewLogView() },
	"MarkdownEditor":    func() Accessible { return NewMarkdownEditor("a") },
	"PropertyGrid":      func() Accessible { return NewPropertyGrid() },
	"RichEditorToolbar": func() Accessible { return NewRichEditorToolbar(nil) },
	"Splitter": func() Accessible {
		return NewSplitter(false, &SplitPane{Widget: NewLabel("a"), Weight: mvvm.NewObservable(1)},
			&SplitPane{Widget: NewLabel("b"), Weight: mvvm.NewObservable(1)})
	},
	"TagField":     func() Accessible { return NewTagField("a") },
	"TerminalView": func() Accessible { return NewTerminalView(4, 2) },
	"TimePicker":   func() Accessible { return NewTimePicker(9, 30) },
	"Wizard":       func() Accessible { return NewWizard([]WizardStep{{Title: "a"}, {Title: "b"}}) },

	// Honoured in #578 part 2; behaviour is tested by TestDisabledWidgetsHonourTheFlag.
	"ActionSheet":     func() Accessible { return NewActionSheet("t") },
	"Agenda":          func() Accessible { return NewAgenda(nil) },
	"AgendaSidebar":   func() Accessible { return NewAgendaSidebar(nil) },
	"ClosablePane":    func() Accessible { return NewClosablePane("t", nil) },
	"ColorChooser":    func() Accessible { return NewColorChooser(RGBA{}) },
	"ColorPicker":     func() Accessible { return NewColorPicker(RGBA{}) },
	"CommandPalette":  func() Accessible { return NewCommandPalette(nil) },
	"ContextMenu":     func() Accessible { return NewContextMenu(NewMenu(nil)) },
	"DatabaseEditor":  func() Accessible { return NewDatabaseEditor(newCannedSource()) },
	"DatePicker":      func() Accessible { return NewDatePicker(2026, 8, 16) },
	"DateRangePicker": func() Accessible { return NewDateRangePicker(2026, 8) },
	"DiffView":        func() Accessible { return NewDiffView("a", "b") },
	"Dock":            func() Accessible { return NewDock(nil) },
	"DropZone":        func() Accessible { return NewDropZone("p") },
	"FileChooser":     func() Accessible { return NewFileChooser(&TreeNode{Label: "r"}, nil) },
	"FindReplace":     func() Accessible { return NewFindReplace() },

	// Honoured in #578 part 1; behaviour is tested by TestDisabledWidgetsHonourTheFlag.
	"ActionRow":     func() Accessible { return NewActionRow("t") },
	"AddressBar":    func() Accessible { return &AddressBar{} },
	"Banner":        func() Accessible { return NewBanner("t") },
	"Breadcrumbs":   func() Accessible { return NewBreadcrumbs([]string{"a"}) },
	"ButtonGroup":   func() Accessible { return NewButtonGroup() },
	"Chip":          func() Accessible { return NewChip("c") },
	"FormField":     func() Accessible { return NewFormField("f", nil) },
	"PagingToolbar": func() Accessible { return NewPagingToolbar(1, 2) },
	"SearchEntry":   func() Accessible { return NewSearchEntry("") },
	"SettingRow":    func() Accessible { return NewSettingRow("r", nil) },
	"SettingsGroup": func() Accessible { return NewSettingsGroup("g") },
	"StatusIcon":    func() Accessible { return NewStatusIcon(nil) },
	"Steps":         func() Accessible { return NewSteps([]string{"a"}, 0) },
	"Thumbnail":     func() Accessible { return NewThumbnail(make([]byte, 4), 1, 1) },
	"Toast":         func() Accessible { return NewToast("t", ToastInfo) },
	"Toolbar":       func() Accessible { return NewToolbar(nil) },
}

// disabledReaders returns the types, among those that answer A11y(), whose own
// methods call Disabled() or isDisabled() directly on their receiver. Calls
// through another value (a child button, a toolbar item's field) are that
// value's business, and so is A11y() itself -- reading the flag there is the fix,
// not a reason to be audited.
func disabledReaders(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	reads, answers := map[string]bool{}, map[string]bool{}
	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv == nil || fd.Body == nil {
					continue
				}
				recv := fd.Recv.List[0]
				var typ ast.Expr = recv.Type
				if s, ok := typ.(*ast.StarExpr); ok {
					typ = s.X
				}
				id, ok := typ.(*ast.Ident)
				if !ok {
					continue
				}
				if fd.Name.Name == "A11y" {
					answers[id.Name] = true
					continue
				}
				if len(recv.Names) == 0 {
					continue
				}
				self := recv.Names[0].Name
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					c, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					s, ok := c.Fun.(*ast.SelectorExpr)
					if !ok || (s.Sel.Name != "Disabled" && s.Sel.Name != "isDisabled") {
						return true
					}
					if x, ok := s.X.(*ast.Ident); ok && x.Name == self {
						reads[id.Name] = true
					}
					return true
				})
			}
		}
	}
	var out []string
	for name := range reads {
		if answers[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// accessibleBaseTypes returns every exported type of the package that embeds
// Base -- directly, or through another type of the package that does -- and
// answers A11y(), its own or one promoted from an embedded type.
func accessibleBaseTypes(t *testing.T) []string { return scanAccessibleBaseTypes(t, ".") }

// scanAccessibleBaseTypes is accessibleBaseTypes over several package
// directories at once. A type of the root package is named bare; a type of
// another directory is named "<dir>.<Type>" (slash-separated, relative to the
// module root), so two packages may use one name. Inside a sub-package the root
// package is reached as `toolkit.X`, which resolves to the bare name X, so a type
// there that embeds toolkit.Base, toolkit.TextView or another sub-package type
// is followed the same way. Generic types count: `*VirtualList[T]` embeds and
// receives like `*VirtualList`.
func scanAccessibleBaseTypes(t *testing.T, dirs ...string) []string {
	t.Helper()
	with, _ := scanBaseTypes(t, dirs...)
	return with
}

// scanBaseTypesWithoutA11y is the other half of the scan: the exported types
// that embed Base but answer no A11y() at all, their own or promoted. The
// completeness gate cannot see them (it asks "which types answer A11y()?"), so a
// widget that forgot the method altogether was skipped instead of refused.
func scanBaseTypesWithoutA11y(t *testing.T, dirs ...string) []string {
	t.Helper()
	_, without := scanBaseTypes(t, dirs...)
	return without
}

// toolkitModule is the import path of the module under audit.
const toolkitModule = "github.com/go-widgets/toolkit"

// qualifiedImports maps each local name a file gives an import to the qualifier
// the scan uses for that package: "" for the module root, "<dir>." for a
// sub-package, and absent for a package outside the module. The local name is
// the alias when there is one, else the last path element -- so `tk
// "github.com/go-widgets/toolkit"` resolves like the bare import, which the
// first version (it compared the identifier to the literal "toolkit") did not.
//
// A dot import of the module is REFUSED with an error: its names arrive
// unqualified, indistinguishable from the package's own, so the scan cannot
// follow them and would otherwise skip the types that embed them in silence.
func qualifiedImports(f *ast.File) (map[string]string, error) {
	out := map[string]string{}
	for _, im := range f.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		if im.Name != nil && im.Name.Name == "." && (path == toolkitModule || strings.HasPrefix(path, toolkitModule+"/")) {
			return nil, fmt.Errorf("dot import of %s: the a11y scan cannot follow unqualified names; import it by name", path)
		}
		var qual string
		switch {
		case path == toolkitModule:
			qual = ""
		case strings.HasPrefix(path, toolkitModule+"/"):
			qual = strings.TrimPrefix(path, toolkitModule+"/") + "."
		default:
			continue
		}
		local := path[strings.LastIndex(path, "/")+1:]
		if im.Name != nil {
			local = im.Name.Name
		}
		out[local] = qual
	}
	return out, nil
}

// scanBaseTypes is scanBaseTypesErr failing the test on an error.
func scanBaseTypes(t *testing.T, dirs ...string) (withA11y, withoutA11y []string) {
	t.Helper()
	with, without, err := scanBaseTypesErr(dirs...)
	if err != nil {
		t.Fatal(err)
	}
	return with, without
}

// scanBaseTypesErr returns the exported Base-embedding types with and without an
// A11y() method, or an error for a source it cannot read or follow.
func scanBaseTypesErr(dirs ...string) (withA11y, withoutA11y []string, err error) {
	embeds := map[string][]string{}
	ownA11y := map[string]bool{}
	for _, dir := range dirs {
		qual := ""
		if dir != "." {
			qual = filepath.ToSlash(dir) + "."
		}
		// name resolves a type expression to its qualified name, or "" for one the
		// scan cannot follow (a type of an unrelated package). imports is the
		// file's own table: an alias is a property of one file.
		var name func(e ast.Expr, imports map[string]string) string
		name = func(e ast.Expr, imports map[string]string) string {
			switch x := e.(type) {
			case *ast.StarExpr:
				return name(x.X, imports)
			case *ast.IndexExpr:
				return name(x.X, imports)
			case *ast.IndexListExpr:
				return name(x.X, imports)
			case *ast.Ident:
				return qual + x.Name
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					if q, ok := imports[id.Name]; ok {
						return q + x.Sel.Name
					}
				}
			}
			return ""
		}
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			return nil, nil, err
		}
		for _, p := range pkgs {
			for _, f := range p.Files {
				imports, err := qualifiedImports(f)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", dir, err)
				}
				for _, d := range f.Decls {
					switch d := d.(type) {
					case *ast.GenDecl:
						for _, s := range d.Specs {
							ts, ok := s.(*ast.TypeSpec)
							if !ok {
								continue
							}
							st, ok := ts.Type.(*ast.StructType)
							if !ok {
								continue
							}
							for _, fld := range st.Fields.List {
								if len(fld.Names) != 0 {
									continue
								}
								if n := name(fld.Type, imports); n != "" {
									embeds[qual+ts.Name.Name] = append(embeds[qual+ts.Name.Name], n)
								}
							}
						}
					case *ast.FuncDecl:
						if d.Recv == nil || d.Name.Name != "A11y" {
							continue
						}
						if n := name(d.Recv.List[0].Type, imports); n != "" {
							ownA11y[n] = true
						}
					}
				}
			}
		}
	}
	// reach reports whether name is, or embeds (transitively), a type satisfying
	// pred; seen guards a cycle.
	var reach func(name string, pred func(string) bool, seen map[string]bool) bool
	reach = func(name string, pred func(string) bool, seen map[string]bool) bool {
		if pred(name) {
			return true
		}
		if seen[name] {
			return false
		}
		seen[name] = true
		for _, e := range embeds[name] {
			if reach(e, pred, seen) {
				return true
			}
		}
		return false
	}
	for name := range embeds {
		if bare := name[strings.LastIndex(name, ".")+1:]; !ast.IsExported(bare) || name == "Base" {
			continue
		}
		isBase := func(n string) bool { return n == "Base" }
		hasA11y := func(n string) bool { return ownA11y[n] }
		if !reach(name, isBase, map[string]bool{}) {
			continue
		}
		if reach(name, hasA11y, map[string]bool{}) {
			withA11y = append(withA11y, name)
		} else {
			withoutA11y = append(withoutA11y, name)
		}
	}
	sort.Strings(withA11y)
	sort.Strings(withoutA11y)
	return withA11y, withoutA11y, nil
}

// TestA11yDisabledCatalogueIsComplete fails when an exported widget that embeds
// Base and answers A11y() is neither in the catalogue nor exempted with a reason,
// when a list names a type that no longer exists as such, when a type is in both
// lists, when an exemption has no reason, or when an exempt type nonetheless
// reads Disabled in its own methods (it belongs in the catalogue).
func TestA11yDisabledCatalogueIsComplete(t *testing.T) {
	found := accessibleBaseTypes(t)
	if len(found) < 100 {
		t.Fatalf("the source scan found %d accessible Base types: the scan is blind", len(found))
	}
	seen := map[string]bool{}
	for _, name := range found {
		seen[name] = true
		_, cat := disabledA11yCatalogue[name]
		_, exempt := disabledA11yExempt[name]
		if !cat && !exempt {
			t.Errorf("%s embeds Base and answers A11y() but is in neither disabledA11yCatalogue nor disabledA11yExempt", name)
		}
		if cat && exempt {
			t.Errorf("%s is both in the catalogue and exempt", name)
		}
	}
	for name := range disabledA11yCatalogue {
		if !seen[name] {
			t.Errorf("disabledA11yCatalogue lists %s, which is not an accessible Base type", name)
		}
	}
	for name, reason := range disabledA11yExempt {
		if !seen[name] {
			t.Errorf("disabledA11yExempt lists %s, which is not an accessible Base type", name)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("disabledA11yExempt[%s] has no reason", name)
		}
	}
	// The cheap direct-read scan still catches the plain case; the exemption list
	// must not hide it.
	for _, name := range disabledReaders(t) {
		if _, ok := disabledA11yExempt[name]; ok {
			t.Errorf("%s is exempt but its own methods read Disabled: audit it", name)
		}
	}
}

// auditA11yDisabled disables a fresh widget and reports how its A11y() differs
// from the flag: empty when it follows the flag both ways.
func auditA11yDisabled(build func() Accessible) (problem string) {
	defer func() {
		if r := recover(); r != nil {
			problem = fmt.Sprintf("A11y panicked: %v", r)
		}
	}()
	w := build()
	d, ok := w.(interface{ Disabled() *mvvm.Observable[bool] })
	if !ok {
		return "has no Disabled() observable"
	}
	if w.A11y().Disabled {
		return "A11y().Disabled is true before it is disabled"
	}
	d.Disabled().Set(true)
	if !w.A11y().Disabled {
		return "disabled, but A11y().Disabled is false"
	}
	d.Disabled().Set(false)
	if w.A11y().Disabled {
		return "re-enabled, but A11y().Disabled is still true"
	}
	return ""
}

// TestA11yDisabledAudit is the inventory: it FAILS with every catalogue widget
// whose A11y() does not follow its Disabled flag.
func TestA11yDisabledAudit(t *testing.T) {
	names := make([]string, 0, len(disabledA11yCatalogue))
	for name := range disabledA11yCatalogue {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	bad := 0
	for _, name := range names {
		if p := auditA11yDisabled(disabledA11yCatalogue[name]); p != "" {
			bad++
			fmt.Fprintf(&b, "  %-16s %s\n", name, p)
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d widgets do not announce Disabled:\n%s", bad, len(names), b.String())
	}
}

// TestDisabledTabStripsDisableTheirTabs: the per-tab nodes a strip exposes to
// the a11y walk inherit its Disabled flag, so a tab is not announced as
// operable inside a tablist that is not.
func TestDisabledTabStripsDisableTheirTabs(t *testing.T) {
	for name, build := range map[string]func() (interface{ Children() []Widget }, func(bool)){
		"TabBar": func() (interface{ Children() []Widget }, func(bool)) {
			b := NewTabBar([]TabItem{{Label: "a"}, {Label: "b"}}, 0)
			return b, b.Disabled().Set
		},
		"FolderTabs": func() (interface{ Children() []Widget }, func(bool)) {
			f := NewFolderTabs([]string{"a", "b"}, 0)
			return f, f.Disabled().Set
		},
	} {
		w, set := build()
		for _, off := range []bool{true, false} {
			set(off)
			kids := w.Children()
			if len(kids) != 2 {
				t.Fatalf("%s: %d tab nodes, want 2", name, len(kids))
			}
			for i, k := range kids {
				if got := k.(Accessible).A11y().Disabled; got != off {
					t.Errorf("%s tab %d: A11y().Disabled = %v with the strip disabled=%v", name, i, got, off)
				}
			}
		}
	}
}

// disabledContainerCases builds the containers whose Disabled flag makes their
// children inert, each holding enabled children (nested ones included), and
// says how many descendant A11y nodes a walk must find.
func disabledContainerCases() map[string]struct {
	build func() (Widget, *mvvm.Observable[bool])
	want  int
} {
	type tc = struct {
		build func() (Widget, *mvvm.Observable[bool])
		want  int
	}
	return map[string]tc{
		"ActionRow": {func() (Widget, *mvvm.Observable[bool]) {
			a := NewActionRow("t")
			a.Prefix, a.Suffix = NewButton("p", nil), &Switch{}
			return a, a.Disabled()
		}, 2},
		"SettingRow": {func() (Widget, *mvvm.Observable[bool]) {
			r := NewSettingRow("r", &Switch{})
			return r, r.Disabled()
		}, 1},
		"SettingsGroup": {func() (Widget, *mvvm.Observable[bool]) {
			g := NewSettingsGroup("g", NewSettingRow("a", &Switch{}), NewSettingRow("b", NewButton("x", nil)))
			return g, g.Disabled()
		}, 4}, // two rows and their controls
		"ButtonGroup": {func() (Widget, *mvvm.Observable[bool]) {
			g := NewButtonGroup(NewButton("a", nil), NewButton("b", nil))
			return g, g.Disabled()
		}, 2},
		"FormField": {func() (Widget, *mvvm.Observable[bool]) {
			f := NewFormField("f", NewEntry(""))
			return f, f.Disabled()
		}, 1},
		"Toolbar": {func() (Widget, *mvvm.Observable[bool]) {
			tb := NewToolbar([]ToolbarItem{{Label: "a"}, {Separator: true}, {Label: "b"}})
			return tb, tb.Disabled()
		}, 2}, // the separator has no node
		"Frame": {func() (Widget, *mvvm.Observable[bool]) {
			f := NewFrame(NewButton("a", nil))
			return f, f.Disabled()
		}, 1},
		"Dialog": {func() (Widget, *mvvm.Observable[bool]) {
			d := NewDialog("t", NewButton("a", nil))
			return d, d.Disabled()
		}, 1},
		"Window": {func() (Widget, *mvvm.Observable[bool]) {
			w := NewWindow("t", NewButton("a", nil))
			return w, w.Disabled()
		}, 1},
		"Wizard": {func() (Widget, *mvvm.Observable[bool]) {
			w := NewWizard([]WizardStep{{Title: "a", Body: NewButton("x", nil)}, {Title: "b", Body: &Switch{}}})
			return w, w.Disabled()
		}, 2},
		"Inspector": {func() (Widget, *mvvm.Observable[bool]) {
			in := NewInspector([]InspectorRow{{ID: "t", Kind: InspectorText}})
			return in, in.Disabled()
		}, 1},
		"Splitter": {func() (Widget, *mvvm.Observable[bool]) {
			s := NewSplitter(false, &SplitPane{Widget: NewButton("a", nil), Weight: mvvm.NewObservable(1)},
				&SplitPane{Widget: NewButton("b", nil), Weight: mvvm.NewObservable(1)})
			s.SetBounds(Rect{W: 200, H: 100}) // the visible panes are decided by the layout
			return s, s.Disabled()
		}, 2}, // presentational itself: no node of its own
	}
}

// TestDisabledContainersDisableTheirDescendantsInA11y: while a container is
// disabled every descendant node of the a11y walk reports Disabled, although no
// child's own flag changed; re-enabled, they clear -- except a child disabled on
// its own, which stays so.
func TestDisabledContainersDisableTheirDescendantsInA11y(t *testing.T) {
	for name, c := range disabledContainerCases() {
		t.Run(name, func(t *testing.T) {
			w, off := c.build()
			skip := 1 // the container's own node
			if w.(Accessible).A11y().Role == RolePresentation {
				skip = 0
			}
			descendants := func() []A11yNode { return WalkA11y(w)[skip:] }
			if got := len(descendants()); got != c.want {
				t.Fatalf("%d descendant nodes, want %d", got, c.want)
			}
			for _, n := range descendants() {
				if n.Disabled {
					t.Errorf("enabled container: %s node is Disabled", n.Role)
				}
			}
			off.Set(true)
			for _, n := range descendants() {
				if !n.Disabled {
					t.Errorf("disabled container: %s node reports enabled", n.Role)
				}
			}
			off.Set(false)
			for _, n := range descendants() {
				if n.Disabled {
					t.Errorf("re-enabled container: %s node still Disabled", n.Role)
				}
			}
		})
	}
}

// A child that was disabled before its container keeps that after the container
// is enabled again: the walk derives, it never writes the children's flags.
func TestDisabledContainerLeavesChildFlagsAlone(t *testing.T) {
	sw := &Switch{}
	sw.Disabled().Set(true)
	row := NewSettingRow("r", sw)
	row.Disabled().Set(true)
	row.Disabled().Set(false)
	if got := WalkA11y(row)[1]; !got.Disabled {
		t.Error("the child disabled on its own was cleared with its container")
	}
	sw.Disabled().Set(false)
	row.Disabled().Set(true)
	if sw.Disabled().Get() {
		t.Error("disabling the container wrote the child's own flag")
	}
	nested := NewSettingsGroup("g", row)
	row.Disabled().Set(false)
	nested.Disabled().Set(true)
	if got := WalkA11y(nested)[2]; !got.Disabled { // group, row, switch
		t.Error("a grandchild of a disabled container reads enabled")
	}
}

// TestAuditA11yDisabledReportsFailures drives the helper over its three failure
// shapes so the audit's own branches are exercised: a widget that never reports
// disabled, one that never clears, and one that panics.
func TestAuditA11yDisabledReportsFailures(t *testing.T) {
	if got := auditA11yDisabled(func() Accessible { return &Label{} }); !strings.Contains(got, "A11y().Disabled is false") {
		t.Errorf("Label (never reports disabled): %q", got)
	}
	if got := auditA11yDisabled(func() Accessible { return &stuckDisabled{} }); !strings.Contains(got, "still true") {
		t.Errorf("stuck widget: %q", got)
	}
	if got := auditA11yDisabled(func() Accessible { return &bornDisabled{} }); !strings.Contains(got, "before it is disabled") {
		t.Errorf("born-disabled widget: %q", got)
	}
	if got := auditA11yDisabled(func() Accessible { return &noObservable{} }); !strings.Contains(got, "no Disabled()") {
		t.Errorf("widget without observable: %q", got)
	}
	if got := auditA11yDisabled(func() Accessible { var b *Button; return b }); !strings.Contains(got, "panicked") {
		t.Errorf("nil widget: %q", got)
	}
}

// stuckDisabled latches: once it has seen the flag set it never reports clear.
type stuckDisabled struct {
	Base
	latched bool
}

func (s *stuckDisabled) A11y() A11yInfo {
	if s.Disabled().Get() {
		s.latched = true
	}
	return A11yInfo{Disabled: s.latched}
}

type bornDisabled struct{ Base }

func (s *bornDisabled) A11y() A11yInfo { return A11yInfo{Disabled: true} }

type noObservable struct{}

func (noObservable) Bounds() Rect                 { return Rect{} }
func (noObservable) SetBounds(Rect)               {}
func (noObservable) Draw(painter.Painter, *Theme) {}
func (noObservable) HitTest(int, int) bool        { return false }
func (noObservable) OnEvent(Event)                {}
func (noObservable) A11y() A11yInfo               { return A11yInfo{} }

// The three reasons a widget that embeds Base is not in the catalogue.
const (
	exemptStatic = "display only: no activation, selection or editing (at most hover or scroll), so it has no inert state to announce"
	exemptLayout = "layout, routing or decoration: forwards events to children whose own state governs, or paints chrome; it has no control of its own"
	// exemptNotWidget is what is left of the former "interactive but never honours
	// Disabled" class (#578): AppDock answers A11y() but is not a Widget, because
	// HitTest returns the item index, so it cannot be built as an Accessible here.
	exemptNotWidget = "answers A11y() but is not a Widget (HitTest returns the item index, not a bool), so the catalogue cannot hold it; changing that public signature is a breaking API change, tracked separately"
)

// disabledA11yExempt lists the accessible Base types that are deliberately not
// audited, each with the reason. A type in it must not read Disabled in its own
// methods (TestA11yDisabledCatalogueIsComplete): the moment it does, it belongs
// in the catalogue.
var disabledA11yExempt = exemptions(map[string][]string{
	exemptStatic: {
		"Alert", "AreaChart", "ArticleCard", "Avatar", "Badge", "BarChart", "Card", "CardMeta", "ChatBubble", "Clock",
		"ContrastBadge", "Diff", "Gauge", "GroupCard", "Image", "Kbd", "Label", "LevelBar", "LineChart",
		"LinkCard", "LoadMask", "MarkdownView", "MediaCard", "Notification", "PieChart", "PostCard",
		"ProgressBar", "ProgressCircle", "RadarChart", "SelectionBox",
		"ScatterChart", "SegmentedBar", "Sparkline", "Spinner", "Stat", "TimeSeriesChart", "Timeline",
		"Tooltip", "XYPlot",
	},
	exemptLayout: {
		"AlignBox", "Backdrop", "Container", "DockPanel", "EmptyState", "Grid", "HBox", "HeaderBar",
		"Material", "Native", "Overlay", "Padding", "Popover", "PopoverHost", "ScrollView", "Skeleton",
		"SkeletonGroup", "Stack", "StatusArea", "VBox", "Viewport", "Wallpaper", "WindowDecoration",
	},
	exemptNotWidget: {"AppDock"},
})

// exemptions flattens reason -> names into name -> reason.
func exemptions(byReason map[string][]string) map[string]string {
	out := map[string]string{}
	for reason, names := range byReason {
		for _, n := range names {
			out[n] = reason
		}
	}
	return out
}

// TestA11yDisabledContainersAreMarked derives the set of containers from the
// catalogue instead of a hand-kept list: every catalogued widget that holds
// children (answers Children()) must carry the a11yDisablesSubtree marker or
// be in disabledSubtreeExempt with the reason its children are not made inert
// by its own flag. A container added later fails here until it is decided.
func TestA11yDisabledContainersAreMarked(t *testing.T) {
	containers := 0
	for name, build := range disabledA11yCatalogue {
		w := build()
		if _, ok := w.(childContainer); !ok {
			continue
		}
		containers++
		_, marked := w.(a11yDisablesSubtree)
		reason, exempt := disabledSubtreeExempt[name]
		if !marked && !exempt {
			t.Errorf("%s holds children but is neither an a11yDisablesSubtree nor in disabledSubtreeExempt", name)
		}
		if marked && exempt {
			t.Errorf("%s is both marked and exempt", name)
		}
		if exempt && strings.TrimSpace(reason) == "" {
			t.Errorf("disabledSubtreeExempt[%s] has no reason", name)
		}
	}
	if containers < 20 {
		t.Fatalf("only %d catalogued containers found: the derivation is blind", containers)
	}
	for name := range disabledSubtreeExempt {
		w, ok := disabledA11yCatalogue[name]
		if !ok {
			t.Errorf("disabledSubtreeExempt lists %s, which is not in the catalogue", name)
			continue
		}
		if _, ok := w().(childContainer); !ok {
			t.Errorf("disabledSubtreeExempt lists %s, which holds no children", name)
		}
	}
}

// disabledSubtreeExempt lists the catalogued containers whose own Disabled flag
// does not make their children inert, with the reason.
var disabledSubtreeExempt = map[string]string{
	"TabBar":     exemptTabNodes,
	"FolderTabs": exemptTabNodes,
	"Accordion":  exemptKeysOnly,
	"Carousel":   exemptKeysOnly,
	"Expander":   exemptKeysOnly,
	"Notebook":   exemptKeysOnly,
	"TextView":   "gates text edits only; its annotation children still take clicks",
	"CodeEditor": "gates text edits only; its annotation children still take clicks",
	"Ribbon":     "announces Disabled but does not drop its controls' input, so its children are not inert",
}

const (
	exemptTabNodes = "builds its tab nodes from its own flag (TestDisabledTabStripsDisableTheirTabs), so the walk has nothing to derive"
	exemptKeysOnly = "gates only its own keyboard handling; the body, slide or content still takes pointer input, so its children are not inert"
)

// Every marked container is walked disabled, so each marker is exercised through
// the walk and a marked widget's own node reports the flag.
func TestA11yDisabledMarkedContainersWalkDisabled(t *testing.T) {
	for name, build := range disabledA11yCatalogue {
		w := build()
		if _, ok := w.(a11yDisablesSubtree); !ok {
			continue
		}
		w.(interface{ Disabled() *mvvm.Observable[bool] }).Disabled().Set(true)
		for _, n := range WalkA11y(w) {
			if !n.Disabled {
				t.Errorf("%s: node %s reads enabled inside a disabled marked container", name, n.Role)
			}
		}
	}
}

// TestScanResolvesAliasedImportsAndFindsWidgetsWithoutA11y drives the scan over
// a fixture holding each shape: a Base embedded through `tk`, a widget with no
// A11y(), a type reaching Base through an aliased sub-package, an unrelated
// aliased package, and an unexported type. The first version compared the
// identifier to the literal "toolkit" and skipped all of the aliased ones.
func TestScanResolvesAliasedImportsAndFindsWidgetsWithoutA11y(t *testing.T) {
	const fx = "testdata/a11yscan"
	with, without := scanBaseTypes(t, fx, "virtual", ".")
	has := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}
	for _, want := range []string{fx + ".Aliased", fx + ".Wrapped"} {
		if !has(with, want) {
			t.Errorf("%s (aliased import) is not found among the accessible Base types", want)
		}
	}
	if !has(without, fx+".Forgetful") {
		t.Errorf("%s.Forgetful has no A11y() and the scan did not report it: %v", fx, without)
	}
	for _, not := range []string{fx + ".NotAWidget", fx + ".hidden"} {
		if has(with, not) || has(without, not) {
			t.Errorf("%s is not a Base widget but the scan reports it", not)
		}
	}
	if got := scanBaseTypesWithoutA11y(t, fx); len(got) != 1 || got[0] != fx+".Forgetful" {
		t.Errorf("scanBaseTypesWithoutA11y = %v, want just Forgetful", got)
	}
}

// noA11yExempt lists the exported Base types with no A11y() of any kind, each
// with the reason. Empty: every widget of the module answers.
var noA11yExempt = map[string]string{}

// TestA11yEveryBaseTypeAnswersA11y refuses a root-package widget that embeds
// Base and has no A11y(): the completeness gate only ever saw types that answer.
func TestA11yEveryBaseTypeAnswersA11y(t *testing.T) {
	for _, name := range scanBaseTypesWithoutA11y(t, ".") {
		if reason, ok := noA11yExempt[name]; !ok || strings.TrimSpace(reason) == "" {
			t.Errorf("%s embeds Base but has no A11y(): give it one, or exempt it in noA11yExempt with a reason", name)
		}
	}
}

// A dot import of the module cannot be followed (its names are unqualified), so
// the scan fails loudly instead of skipping the types that embed through it.
func TestScanRefusesADotImport(t *testing.T) {
	_, _, err := scanBaseTypesErr("testdata/a11ydot")
	if err == nil || !strings.Contains(err.Error(), "dot import") {
		t.Fatalf("scan of a dot-importing package: err = %v, want a refusal naming the dot import", err)
	}
	if _, _, err := scanBaseTypesErr("testdata/does-not-exist"); err == nil {
		t.Fatal("scan of a missing directory must report the error, not an empty result")
	}
}

// An alias is a property of one file: `tk` is the toolkit in aliased.go and an
// unrelated package in leak.go, whose type must not be taken for a widget.
func TestScanAliasDoesNotLeakAcrossFiles(t *testing.T) {
	const fx = "testdata/a11yscan"
	with, without := scanBaseTypes(t, fx, ".")
	for _, n := range append(append([]string{}, with...), without...) {
		if n == fx+".Leak" {
			t.Fatalf("%s is reported as a Base type: an alias leaked from another file", n)
		}
	}
	found := false
	for _, n := range with {
		found = found || n == fx+".Aliased"
	}
	if !found {
		t.Fatal("the alias in its own file no longer resolves")
	}
}

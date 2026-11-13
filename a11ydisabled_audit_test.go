// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
//     every type whose own methods read Disabled (or the focus walker's
//     isDisabled) on their receiver and which answers A11y(). Such a type must
//     be in the catalogue, so a NEW widget with a Disabled-aware Draw/OnEvent
//     fails here until it is audited.
//   - TestA11yDisabledAudit builds each catalogue widget, disables it and
//     requires A11y().Disabled, then enables it and requires the flag to clear.
//
// Base is the one reader outside the audit: it owns the observable and has no
// A11y() of its own.

// disabledA11yCatalogue maps a type name to a constructor for a fresh instance.
var disabledA11yCatalogue = map[string]func() Accessible{
	"Accordion":      func() Accessible { return NewAccordion([]AccordionSection{{Title: "a"}}) },
	"Browser":        func() Accessible { return NewBrowser() },
	"Button":         func() Accessible { return &Button{} },
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

// TestA11yDisabledCatalogueIsComplete fails when a widget reads Disabled and
// answers A11y() but is not in the catalogue, or when the catalogue names a type
// that no longer reads Disabled.
func TestA11yDisabledCatalogueIsComplete(t *testing.T) {
	readers := disabledReaders(t)
	if len(readers) == 0 {
		t.Fatal("the source scan found no widget reading Disabled: the scan is blind")
	}
	seen := map[string]bool{}
	for _, name := range readers {
		seen[name] = true
		if _, ok := disabledA11yCatalogue[name]; !ok {
			t.Errorf("%s reads Disabled and has A11y() but is not in disabledA11yCatalogue", name)
		}
	}
	for name := range disabledA11yCatalogue {
		if !seen[name] {
			t.Errorf("disabledA11yCatalogue lists %s, which no longer reads Disabled", name)
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

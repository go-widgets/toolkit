// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every container yields the children it was given, in visual order.
func TestContainersExposeTheirChildren(t *testing.T) {
	a, b := NewButton("a", nil), NewButton("b", nil)

	notebook := NewNotebook()
	notebook.AddTab("one", a)
	dock := NewDock(b)
	dock.Dock(a, DockTop, 20)
	grid := NewGrid(2, 1)
	grid.Attach(a, 0, 0)
	grid.Attach(b, 1, 0)

	cases := []struct {
		name string
		w    childContainer
		want []Widget
	}{
		{"accordion", NewAccordion([]AccordionSection{{Title: "s", Body: a}}), []Widget{a}},
		{"actionrow", &ActionRow{Prefix: a, Suffix: b}, []Widget{a, b}},
		{"border", &Border{North: a, Center: b}, []Widget{a, b}},
		{"carousel", NewCarousel([]Widget{a, b}), []Widget{a, b}},
		{"dialog", NewDialog("t", a), []Widget{a}},
		{"dock", dock, []Widget{a, b}},
		{"expander", NewExpander("l", a), []Widget{a}},
		{"formfield", NewFormField("l", a), []Widget{a}},
		{"frame", NewFrame(a), []Widget{a}},
		{"grid", grid, []Widget{a, b}},
		{"headerbar", &HeaderBar{Start: []Widget{a}, End: []Widget{b}}, []Widget{a, b}},
		{"notebook", notebook, []Widget{a}},
		{"overlay", &Overlay{Content: a, Layers: []Widget{b}}, []Widget{a, b}},
		{"paned", &Paned{First: a, Second: b}, []Widget{a, b}},
		{"popover", NewPopover(a), []Widget{a}},
		{"scrollview", NewScrollView(a), []Widget{a}},
		{"statusbar", &Statusbar{Left: []StatusSegment{{Widget: a}}, Right: []StatusSegment{{Widget: b}}}, []Widget{a, b}},
		{"window", NewWindow("t", a), []Widget{a}},
		{"wizard", NewWizard([]WizardStep{{Title: "s", Body: a}}), []Widget{a}},
	}
	for _, c := range cases {
		got := c.w.Children()
		if len(got) != len(c.want) {
			t.Errorf("%s: %d children, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: child %d = %v, want %v", c.name, i, got[i], c.want[i])
			}
		}
	}
}

// An empty slot is stepped over rather than handed on: containers are built
// incrementally, and no walker should have to defend against a nil child.
func TestContainersSkipNilChildren(t *testing.T) {
	a := NewButton("a", nil)
	cases := []struct {
		name string
		w    childContainer
	}{
		{"accordion", NewAccordion([]AccordionSection{{Title: "s"}, {Title: "t", Body: a}})},
		{"actionrow", &ActionRow{Suffix: a}},
		{"border", &Border{South: a}},
		{"carousel", NewCarousel([]Widget{nil, a})},
		{"dialog", NewDialog("t", nil)},
		{"dock", NewDock(nil)},
		{"expander", NewExpander("l", nil)},
		{"formfield", NewFormField("l", nil)},
		{"frame", NewFrame(nil)},
		{"grid", NewGrid(1, 1)},
		{"headerbar", &HeaderBar{Start: []Widget{nil, a}}},
		{"notebook", NewNotebook()},
		{"overlay", &Overlay{Layers: []Widget{nil}}},
		{"paned", &Paned{Second: a}},
		{"popover", NewPopover(nil)},
		{"scrollview", NewScrollView(nil)},
		{"statusbar", &Statusbar{Left: []StatusSegment{{Text: "x"}, {Widget: a}}}},
		{"window", NewWindow("t", nil)},
		{"wizard", NewWizard([]WizardStep{{Title: "s"}})},
	}
	for _, c := range cases {
		for i, ch := range c.w.Children() {
			if ch == nil {
				t.Errorf("%s: child %d is nil", c.name, i)
			}
		}
	}
}

// The accessibility tree must describe scrolled content where it is PAINTED.
// ScrollView draws its child by moving its bounds and putting them back, so
// without ChildOffset the walk reports the unscrolled position — 250 pixels
// below the control, in this case.
func TestWalkA11yFollowsTheScrolledViewport(t *testing.T) {
	btn := NewButton("Deep", nil)
	btn.SetBounds(Rect{X: 0, Y: 300, W: 80, H: 24})
	inner := NewContainer(nil)
	inner.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 600})
	inner.AddWidget(btn)

	sv := NewScrollView(inner)
	sv.SetBounds(Rect{X: 0, Y: 0, W: 200, H: 100})
	sv.SetContentSize(200, 600)
	sv.Scroll(0, 250)

	var found bool
	for _, n := range WalkA11y(sv) {
		if n.Name != "Deep" {
			continue
		}
		found = true
		if n.Rect.Y != 50 {
			t.Fatalf("button reported at Y=%d, want 50 (300 laid out, 250 scrolled away)", n.Rect.Y)
		}
	}
	if !found {
		t.Fatal("the scrolled button is not in the accessibility tree at all")
	}
	if dx, dy := sv.ChildOffset(); dx != 0 || dy != -250 {
		t.Fatalf("ChildOffset = %d,%d, want 0,-250", dx, dy)
	}
}

// A widget that holds other widgets and does not expose them is a dead end for
// every generic walk — CollectRuns, and the accessibility bridges built on
// WalkA11y. That failure is SILENT: the tree simply stops, and an application's
// whole scrollable content goes missing from a screen reader with nothing
// logged. This test reads the package and refuses to let the next one through.
func TestEveryContainerExposesItsChildren(t *testing.T) {
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("cannot list the package: %v", err)
	}

	holds := map[string][]string{}      // type -> fields that hold widgets
	fieldTypes := map[string][]string{} // type -> every field type, for indirect holders
	exposes := map[string]bool{}
	isWidget := map[string]bool{}

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			switch d := n.(type) {
			case *ast.TypeSpec:
				st, ok := d.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, fl := range st.Fields.List {
					ft := exprType(fl.Type)
					fieldTypes[d.Name.Name] = append(fieldTypes[d.Name.Name], ft)
					if ft != "Widget" && ft != "[]Widget" {
						continue
					}
					if len(fl.Names) == 0 {
						holds[d.Name.Name] = append(holds[d.Name.Name], "<embedded>")
						continue
					}
					for _, nm := range fl.Names {
						holds[d.Name.Name] = append(holds[d.Name.Name], nm.Name)
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil || len(d.Recv.List) == 0 {
					return true
				}
				rt := recvType(d.Recv.List[0].Type)
				switch d.Name.Name {
				case "Children":
					exposes[rt] = true
				case "Draw":
					isWidget[rt] = true
				}
			}
			return true
		})
	}

	var missing []string
	for typ, fields := range holds {
		if isWidget[typ] && !exposes[typ] {
			missing = append(missing, typ+" holds "+strings.Join(fields, ", "))
		}
	}
	// A widget reaching its children THROUGH a helper struct is just as much a
	// dead end, and easier to miss.
	for typ, fts := range fieldTypes {
		if !isWidget[typ] || exposes[typ] {
			continue
		}
		for _, ft := range fts {
			if len(holds[strings.TrimPrefix(ft, "[]")]) > 0 {
				missing = append(missing, typ+" holds widgets through "+ft)
				break
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("these widgets hold children that no generic walk can reach — "+
			"give each a Children() method in children.go:\n  %s",
			strings.Join(missing, "\n  "))
	}
}

func exprType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.ArrayType:
		return "[]" + exprType(t.Elt)
	}
	return ""
}

func recvType(e ast.Expr) string {
	if s, ok := e.(*ast.StarExpr); ok {
		e = s.X
	}
	if i, ok := e.(*ast.Ident); ok {
		return i.Name
	}
	return ""
}

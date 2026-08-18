// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// mvvmMigratedWidgets is the set of widgets whose mutable state has been migrated
// to the MVVM layer. For every widget in this set the gate below makes it
// IMPOSSIBLE to reintroduce an imperative state field: each exported field must
// be a func hook (a painter / callback config seam), an *mvvm.Observable /
// *mvvm.Command / *mvvm.ObservableList, or an appearance/layout CONFIG field
// named in the widget's allowlist here. Any other exported field — a bare
// string / bool / int / enum that is the widget's reactive STATE — fails the
// build, forcing the state onto an Observable accessor.
//
// The value is the set of exported fields that are legitimate set-once
// appearance/layout config (not reactive state), so they may stay plain fields.
// The set shrinks nothing over time; it grows only as widgets are migrated.
//
// This is the enforcement half of the "everything through MVVM" rule: unexported
// state + Observable accessors already make `w.State = x` a compile error, and
// this gate makes ADDING a new imperative state field to a migrated widget a CI
// failure. New entries are added here as each widget is migrated in later batches.
var mvvmMigratedWidgets = map[string]map[string]bool{
	// AddressBar: Radius + TextPad are device-pixel layout metrics (config); the
	// icon hooks are funcs and Commit is an *mvvm.Command (both auto-allowed); all
	// of its reactive state (URL / Editing / Focused / Bookmarked / Copied) is
	// unexported behind Observable accessors.
	"AddressBar": {"Radius": true, "TextPad": true},
	// Max is cell count; FilledColor/EmptyColor are set-once appearance overrides
	// for the star tones. All reactive state (the score) is on the Value Observable.
	"Rating": {"Max": true, "FilledColor": true, "EmptyColor": true},
	// Batch 2 (leaf widgets): reactive state is unexported behind accessors
	// (Fraction/Active/Value/Checked/Pressed/On/Expanded); the values below are
	// set-once appearance/layout config that legitimately stays a plain field.
	"ProgressCircle": {},
	"Spinner":        {"Phase": true, "Style": true},
	"Scale":          {"Min": true, "Max": true, "Orientation": true, "Step": true},
	"SpinButton":     {"Min": true, "Max": true, "Step": true},
	"CheckButton":    {"Label": true, "Size": true},
	"ToggleButton":   {"Label": true},
	"Switch":         {},
	"Expander":       {"Label": true, "Content": true},
	// Batch 3 (index/selection widgets): the current index / open / collapsed
	// state is unexported behind accessors; the slices + flags below are config.
	"Frame":        {"Padding": true, "Title": true, "Collapsible": true},
	"ViewSwitcher": {"Views": true},
	"CycleButton":  {"Options": true},
	"Pagination":   {"Total": true},
	"Steps":        {"Labels": true, "Orientation": true},
	"Notebook":     {"Tabs": true, "TabSide": true},
	"DropDown":     {"Options": true, "OpenUp": true},
	"Accordion":    {"Sections": true, "Multiple": true},
	// Batch 4 (selection/range widgets): the selected index / low-high / open /
	// checked state is unexported behind accessors; the slices + flags below are
	// config (func hooks like OnActivate/OnReorder/OnCardMove/OnTaskChange/OnRefresh
	// are auto-allowed).
	"Carousel":      {"Slides": true, "Wrap": true},
	"PagingToolbar": {"PageCount": true, "ShowRefresh": true},
	"RadioButton":   {"Label": true},
	"RadioGroup":    {"Members": true},
	"ComboBox":      {"Options": true, "Placeholder": true},
	// ListBox: Selected is migrated (PR #285) but ScrollRow (scroll-position state)
	// is not yet, so ListBox is intentionally NOT gated yet — a follow-up migrates
	// ScrollRow, then it joins the map. Same for Table/TreeView (ScrollRow/ScrollX).
	"Gantt":       {"Tasks": true, "Units": true},
	"RangeSlider": {"Orientation": true, "Step": true, "Min": true, "Max": true},
	"Kanban":      {"Columns": true},
}

// TestMigratedWidgetsHaveNoImperativeState is the enforcement gate. It parses the
// package, and for each migrated widget checks every exported struct field is a
// func, an *mvvm.* handle, or an allowlisted config name — never a bare state
// field. See [mvvmMigratedWidgets].
func TestMigratedWidgetsHaveNoImperativeState(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	structs := map[string]*ast.StructType{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok {
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
			}
			return true
		})
	}

	for widget, config := range mvvmMigratedWidgets {
		st := structs[widget]
		if st == nil {
			t.Errorf("migrated widget %q not found in the package — did it get renamed?", widget)
			continue
		}
		var offenders []string
		for _, f := range st.Fields.List {
			if len(f.Names) == 0 {
				continue // embedded (e.g. Base) — not a state field
			}
			for _, id := range f.Names {
				if !id.IsExported() {
					continue // unexported state — already unreachable from outside
				}
				if isFuncField(f.Type) || isMVVMHandle(f.Type) || config[id.Name] {
					continue
				}
				offenders = append(offenders, id.Name)
			}
		}
		sort.Strings(offenders)
		if len(offenders) > 0 {
			t.Errorf("migrated widget %q exposes imperative state field(s) %v — move them onto an "+
				"*mvvm.Observable accessor, or (if they are set-once appearance/layout config) add "+
				"them to mvvmMigratedWidgets[%q]", widget, offenders, widget)
		}
	}
}

// isFuncField reports whether the field type is a function (a painter / callback
// config seam, e.g. a LeadingIcon painter), which is legitimately a plain field.
func isFuncField(e ast.Expr) bool {
	_, ok := e.(*ast.FuncType)
	return ok
}

// isMVVMHandle reports whether the field type is an *mvvm.X (Observable / Command
// / ObservableList), including the generic *mvvm.Observable[T] form.
func isMVVMHandle(e ast.Expr) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if idx, ok := e.(*ast.IndexExpr); ok { // *mvvm.Observable[T]
		e = idx.X
	}
	if idx, ok := e.(*ast.IndexListExpr); ok { // multi-type generic, defensive
		e = idx.X
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "mvvm"
}

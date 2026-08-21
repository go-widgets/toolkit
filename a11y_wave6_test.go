// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestEveryWidgetIsAccessible is the guard that keeps this complete. It parses
// the package, finds every widget (a struct embedding Base) and every A11y()
// method, and fails naming any widget that describes itself as nothing.
//
// Without it the gap reopens silently: CollectA11y SKIPS a widget that does not
// implement Accessible, so a consumer walking the tree cannot tell "this has no
// semantics" from "nobody wrote its A11y()". Twenty-one widgets had drifted into
// that state before this test existed.
func TestEveryWidgetIsAccessible(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	widgets := map[string]string{} // widget -> file
	impls := map[string]bool{}
	embedsBase := regexp.MustCompile(`(?m)^\s*Base\s*$`)

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
			switch d := n.(type) {
			case *ast.TypeSpec:
				st, ok := d.Type.(*ast.StructType)
				if !ok || !d.Name.IsExported() {
					return true
				}
				// An embedded Base is what makes a type a widget here.
				for _, f := range st.Fields.List {
					if len(f.Names) == 0 {
						if id, ok := f.Type.(*ast.Ident); ok && id.Name == "Base" {
							widgets[d.Name.Name] = name
						}
					}
				}
			case *ast.FuncDecl:
				if d.Name.Name != "A11y" || d.Recv == nil || len(d.Recv.List) == 0 {
					return true
				}
				if star, ok := d.Recv.List[0].Type.(*ast.StarExpr); ok {
					if id, ok := star.X.(*ast.Ident); ok {
						impls[id.Name] = true
					}
				}
			}
			return true
		})
		_ = embedsBase
	}

	if len(widgets) < 50 {
		t.Fatalf("only found %d widgets; the scan is broken, not the coverage", len(widgets))
	}
	var missing []string
	for w, f := range widgets {
		if !impls[w] {
			missing = append(missing, w+" ("+filepath.Base(f)+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("%d widget(s) have no A11y() and would be silently skipped by CollectA11y:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestImageCarriesItsAltText is the defect that started this: Image reported
// RoleImg with no name and had no field to hold one, so an image could not be
// described at all. Its own doc comment said the host would "fill in" the name,
// which A11y()'s by-value return makes impossible.
func TestImageCarriesItsAltText(t *testing.T) {
	img := &Image{Alt: "Sun setting behind a launch tower"}
	got := img.A11y()
	if got.Role != RoleImg {
		t.Fatalf("Role = %q, want %q", got.Role, RoleImg)
	}
	if got.Name != "Sun setting behind a launch tower" {
		t.Fatalf("Name = %q, want the alt text", got.Name)
	}
	// Decoration legitimately has no name — the same rule as an empty HTML alt.
	if n := (&Image{}).A11y().Name; n != "" {
		t.Fatalf("undescribed image Name = %q, want empty", n)
	}
}

func TestThumbnailNamePrefersAltOverLabel(t *testing.T) {
	if got := (&Thumbnail{Alt: "A rocket on the pad", Label: "IMG_0421.jpg"}).A11y(); got.Name != "A rocket on the pad" {
		t.Fatalf("Name = %q, want the alt text over the filename caption", got.Name)
	}
	if got := (&Thumbnail{Label: "holiday.jpg"}).A11y(); got.Name != "holiday.jpg" {
		t.Fatalf("Name = %q, want the caption as the fallback", got.Name)
	}
	if got := (&Thumbnail{}).A11y(); got.Role != RoleImg {
		t.Fatalf("Role = %q, want %q", got.Role, RoleImg)
	}
}

// TestLayoutIsPresentational pins the deliberate choice: a container arranges
// content and is not content, so it reports RolePresentation rather than being
// absent. "Look through me" and "I was never described" must not look the same.
func TestLayoutIsPresentational(t *testing.T) {
	for name, w := range map[string]Accessible{
		"HBox": NewHBox(), "VBox": NewVBox(), "Container": &Container{},
		"Stack": &Stack{}, "Backdrop": &Backdrop{}, "Wallpaper": &Wallpaper{},
		"Scrollbar": &Scrollbar{}, "Grid": &Grid{}, "Border": &Border{},
	} {
		if got := w.A11y(); got.Role != RolePresentation {
			t.Errorf("%s Role = %q, want %q", name, got.Role, RolePresentation)
		}
	}
}

func TestContentWidgetsDescribeThemselves(t *testing.T) {
	cases := []struct {
		name string
		w    Accessible
		role Role
		want string // expected Name, "" to skip
	}{
		{"Window", &Window{Title: "Preferences"}, RoleDialog, "Preferences"},
		{"WindowDecoration", &WindowDecoration{Title: "Preferences"}, RoleBanner, "Preferences"},
		{"AgendaSidebar", &AgendaSidebar{Title: "Calendars"}, RoleNavigation, "Calendars"},
		{"LoadMask", &LoadMask{Message: "Loading…"}, RoleStatus, "Loading…"},
		{"ButtonGroup", &ButtonGroup{}, RoleGroup, ""},
		{"Gantt", &Gantt{}, RoleImg, ""},
		{"Dock", &Dock{}, RoleToolbar, ""},
		{"StatusArea", &StatusArea{}, RoleToolbar, ""},
		{"StatusIcon", &StatusIcon{}, RoleStatus, ""},
		{"PagingToolbar", NewPagingToolbar(2, 7), RoleToolbar, ""},
	}
	for _, c := range cases {
		got := c.w.A11y()
		if got.Role != c.role {
			t.Errorf("%s Role = %q, want %q", c.name, got.Role, c.role)
		}
		if c.want != "" && got.Name != c.want {
			t.Errorf("%s Name = %q, want %q", c.name, got.Name, c.want)
		}
	}
	// The paging toolbar's whole point is the position it reports.
	if got := NewPagingToolbar(2, 7).A11y().Value; got != "page 2 of 7" {
		t.Fatalf("PagingToolbar Value = %q", got)
	}
	// A LoadMask says it is busy only while it actually is.
	busyMask := &LoadMask{}
	busyMask.Active().Set(true)
	if got := busyMask.A11y().Value; got != "busy" {
		t.Fatalf("active LoadMask Value = %q, want busy", got)
	}
	if got := (&LoadMask{}).A11y().Value; got != "" {
		t.Fatalf("inactive LoadMask Value = %q, want empty", got)
	}
	// A status icon announces its badge count when it has one.
	if got := (&StatusIcon{Badge: &Badge{Text: "3"}}).A11y().Value; got != "3" {
		t.Fatalf("StatusIcon Value = %q, want the badge text", got)
	}
}

// TestBrowserNamesThePageItShows checks the document role carries the URL, so a
// reader can say WHICH page rather than just "a document".
func TestBrowserNamesThePageItShows(t *testing.T) {
	b := NewBrowser()
	got := b.A11y()
	if got.Role != RoleDocument {
		t.Fatalf("Role = %q, want %q", got.Role, RoleDocument)
	}
	if got.Name != b.CurrentURL() {
		t.Fatalf("Name = %q, want the current URL %q", got.Name, b.CurrentURL())
	}
}

// TestCollectA11ySkipsPresentation pins the contract the existing callers
// depend on: making every widget answer A11y() must NOT flood the collected
// tree with layout furniture. A box now describes itself as presentational, and
// CollectA11y drops it — same output as when it answered nothing at all, but for
// a stated reason rather than an omission.
func TestCollectA11ySkipsPresentation(t *testing.T) {
	box := NewVBox()
	btn := NewButton("Save", nil)
	lbl := NewLabel("Ready")

	got := CollectA11y([]Widget{box, btn, lbl})
	if len(got) != 2 {
		t.Fatalf("collected %d, want 2 (the box is presentational)", len(got))
	}
	if got[0].Role != RoleButton || got[1].Role != RoleText {
		t.Fatalf("collected %+v, want the button then the label", got)
	}
	// The box still describes itself when asked directly — that is the whole
	// difference between "ignore me" and "nobody wrote my A11y()".
	if box.A11y().Role != RolePresentation {
		t.Fatal("the box should report itself as presentational")
	}
}

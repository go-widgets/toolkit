// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-crdt/crdt"
	"github.com/go-widgets/toolkit/internal/scalesweep"
)

//go:generate go run ./internal/scalesweep/gen

// The Retina sweep. A window dragged between a 2x and a 1x display changes
// [MetricScale] under widgets that are already built, so two things must hold for
// EVERY widget the toolkit has, not for the ones someone remembered:
//
//  1. live == fresh: a widget built and drawn at one scale, then drawn after the
//     scale changed, is pixel-identical to one built at the new scale. Anything
//     that reads a metric once and keeps it (a face cache, a size computed in a
//     constructor) fails here, in both directions.
//  2. scale means size: the ink of a widget at 2x spans twice what it spans at
//     1x. A size given in pixels instead of logical pixels fails here even when
//     (1) holds, because it is then wrong the same way fresh and live.
//
// The widgets come from the source ([scalesweep.Types]), so a new one is swept
// the day it lands; the generated list is checked against the source and the
// test says how to regenerate it.

func TestSweepTypeListIsCurrent(t *testing.T) {
	names, err := scalesweep.Types(".")
	if err != nil {
		t.Fatal(err)
	}
	have, err := os.ReadFile("scalesweep_types_gen_test.go")
	if err != nil {
		t.Fatalf("scalesweep_types_gen_test.go: %v -- run `go generate ./...`", err)
	}
	if !bytes.Equal(have, scalesweep.Generate(names)) {
		t.Fatal("the exported types changed since scalesweep_types_gen_test.go was generated: run `go generate ./...` and commit it, so the new type is swept")
	}
}

// tryBuild runs mk, reporting a panic instead of dying with it.
func tryBuild(mk func() any) (v any, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			v, panicked = r, true
		}
	}()
	return mk(), false
}

// zero is the zero value of T, for the generated constructor calls.
func zero[T any]() (z T) { return }

// sweepSize is the logical size every widget is drawn in.
const sweepW, sweepH = 240, 160

// populate gives a freshly built widget something to show: text in every
// string field and in its Text() observable, and a per-widget font size. A
// widget with nothing to draw cannot show a size bug.
func populate(v any) {
	rv := reflect.ValueOf(v).Elem()
	for i := 0; i < rv.NumField(); i++ {
		f, sf := rv.Field(i), rv.Type().Field(i)
		if !sf.IsExported() || !f.CanSet() {
			continue
		}
		switch {
		case f.Kind() == reflect.String && f.String() == "":
			f.SetString("Sample")
		case f.Kind() == reflect.Int && sf.Name == "FontSize":
			f.SetInt(18)
		case f.Type() == reflect.TypeOf([]string(nil)) && f.Len() == 0:
			f.Set(reflect.ValueOf([]string{"one", "two", "three"}))
		case f.Type() == reflect.TypeOf((*TreeNode)(nil)) && f.IsNil():
			f.Set(reflect.ValueOf(&TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{{Label: "a"}, {Label: "b"}}}))
		}
	}
	if m := reflect.ValueOf(v).MethodByName("Text"); m.IsValid() && m.Type().NumIn() == 0 && m.Type().NumOut() == 1 {
		if set := m.Call(nil)[0].MethodByName("Set"); set.IsValid() && set.Type().NumIn() == 1 && set.Type().In(0).Kind() == reflect.String {
			set.Call([]reflect.Value{reflect.ValueOf("No server")})
		}
	}
}

// sweepDraw draws w at the CURRENT scale into a fresh surface.
func sweepDraw(w Widget) ([]byte, int, int) {
	s := MetricScale()
	bw, bh := int(float64(sweepW)*s), int(float64(sweepH)*s)
	buf := makeSurface(bw, bh)
	w.SetBounds(Rect{W: bw, H: bh})
	w.Draw(newP(buf, bw), DefaultDark())
	return buf, bw, bh
}

// sweepReset puts the globals back; the sweep changes them on purpose.
func sweepReset() {
	SetMetricScale(1)
	SetFont(nil)
	openTypeLogicalPx = 0
}

// sweepAt builds the widget with newW at scale s0, draws it, moves to s1 and
// draws again; s0 == s1 is the fresh case.
func sweepAt(newW func() Widget, s0, s1 float64) (buf []byte, w, h int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		sweepReset()
	}()
	sweepReset()
	SetMetricScale(s0)
	if e := UseOpenTypeText(); e != nil {
		return nil, 0, 0, e
	}
	wd := newW()
	populate(wd)
	sweepDraw(wd)
	SetMetricScale(s1)
	buf, w, h = sweepDraw(wd)
	return buf, w, h, nil
}

// ink is the bounding box of what the widget painted over the sentinel fill.
func ink(buf []byte, w, h int) (x0, y0, x1, y1 int) {
	x0, y0, x1, y1 = w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := (y*w + x) * 4
			if buf[o] == 0xC8 && buf[o+1] == 0xC8 && buf[o+2] == 0xC8 && buf[o+3] == 0xFF {
				continue
			}
			x0, y0 = min(x0, x), min(y0, y)
			x1, y1 = max(x1, x), max(y1, y)
		}
	}
	return
}

// sweepArgs builds the widgets whose constructor needs an argument. The list
// is the ONLY hand-kept part and it fails closed: a widget that cannot be built
// from its zero value or a no-argument constructor and is not here fails the
// sweep, instead of being skipped.
var sweepArgs = map[string]func() any{
	"ClosablePane":    func() any { return NewClosablePane("Title", &Label{}) },
	"CodeEditor":      func() any { return NewCodeEditor("x := 1") },
	"DatabaseEditor":  func() any { return NewDatabaseEditor(newCannedSource()) },
	"DatePicker":      func() any { return NewDatePicker(2026, 8, 16) },
	"DateRangePicker": func() any { return NewDateRangePicker(2026, 8) },
	"DiffView":        func() any { return NewDiffView("a\nb", "a\nc") },
	"EmptyState":      func() any { return NewEmptyState("No server") },
	"FileChooser":     func() any { return NewFileChooser(&TreeNode{Label: "r"}, nil) },
	"FileTree":        func() any { return NewFileTree(nil) },
	"FontPicker":      func() any { return NewFontPicker([]string{"Alpha", "Beta"}) },
	"ForgePane":       func() any { return NewForgePane(newFakeForge()) },
	"GraphView":       func() any { return NewGraphView(NewGraphViewModel()) },
	"IsoDiagram":      func() any { return NewIsoDiagram(nil) },
	"IsoIconPalette":  func() any { return NewIsoIconPalette(NewIsoIconRegistry()) },
	"PullToRefresh":   func() any { return NewPullToRefresh(&Label{}) },
	"Backdrop": func() any {
		return NewBackdrop(RGBA{R: 10, G: 10, B: 10, A: 255}, RGBA{R: 90, G: 90, B: 90, A: 255}, 12)
	},
	"Clock":             func() any { return NewClock(time.Date(2026, 8, 16, 10, 9, 0, 0, time.UTC)) },
	"CollabSheet":       func() any { return NewCollabSheet(crdt.SiteID(1)) },
	"IsoCRDTDocument":   func() any { return NewIsoCRDTDocument(crdt.SiteID(1)) },
	"TerminalView":      func() any { return NewTerminalView(20, 5) },
	"RichEditorToolbar": func() any { return NewRichEditorToolbar(nil) },
	"Spreadsheet":       func() any { return NewSpreadsheet(4, 4) },
}

// sweepBlankOK are the widgets that paint nothing at rest: containers with no
// child, overlays and popups drawn only while open, and state-driven widgets.
// Their metrics are exercised through their children or by their own tests.
var sweepBlankOK = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Fields(`Accordion ActionSheet AlignBox Base Border Carousel
		CommandPalette Container ContextMenu Dock DockPanel FileTree FindReplace Grid HBox
		Image Inspector Kanban LoadMask Native Notification Overlay Padding PieChart Popover
		PopoverHost PullToRefresh SkeletonGroup Sparkline Spinner Splitter Stack StatusArea
		StatusIcon Surface SwipeActions Toast Tooltip VBox Viewport WindowDecoration Wizard
		WorkspacePager`) {
		m[n] = true
	}
	return m
}()

func TestEveryWidgetFollowsAScaleChange(t *testing.T) {
	var swept, skipped int
	var live, size, blank []string
	t.Cleanup(sweepReset)
	// ⛔ Fails closed: a type whose constructor the generator cannot call must be
	// built by hand, never from its zero value, which skips what the constructor
	// sets up (the NewListBox RowHeight that went unseen).
	for _, n := range sweepNeedsArgs {
		if _, ok := sweepArgs[n]; !ok {
			t.Errorf("%s has a constructor with parameters and no entry in sweepArgs: the sweep would build it from its zero value", n)
		}
	}
	for _, st := range sweepTypes {
		mk := st.new
		if f, ok := sweepArgs[st.name]; ok {
			mk = f
		}
		if v, panicked := tryBuild(mk); panicked {
			// ⛔ A constructor that refuses its zero arguments is not skipped: the
			// type goes in sweepArgs with arguments it accepts.
			if _, ok := sweepArgs[st.name]; !ok {
				t.Errorf("%s: its constructor panics on zero arguments (%v) and sweepArgs has no entry", st.name, v)
			}
			continue
		} else if _, isWidget := v.(Widget); !isWidget {
			continue // not a widget
		}
		name := st.name
		newW := func() Widget { return mk().(Widget) }
		fresh2, w2, h2, err := sweepAt(newW, 2, 2)
		if err != nil {
			skipped++
			t.Logf("skipped %s: %v", name, err)
			continue
		}
		again2, _, _, _ := sweepAt(newW, 2, 2)
		if !bytes.Equal(fresh2, again2) {
			skipped++
			t.Logf("skipped %s: two fresh renders at 2x differ (time- or randomness-driven)", name)
			continue
		}
		swept++
		for _, dir := range [][2]float64{{1, 2}, {2, 1}} {
			want, got := fresh2, []byte(nil)
			wantW := w2
			if dir[1] == 1 {
				want, wantW, _, _ = sweepAt(newW, 1, 1)
			}
			got, gotW, _, err := sweepAt(newW, dir[0], dir[1])
			if err != nil || gotW != wantW || !bytes.Equal(got, want) {
				live = append(live, fmt.Sprintf("%s %gx->%gx", name, dir[0], dir[1]))
			}
		}
		one, w1, h1, _ := sweepAt(newW, 1, 1)
		ax0, ay0, ax1, ay1 := ink(one, w1, h1)
		bx0, by0, bx1, by1 := ink(fresh2, w2, h2)
		if ax1 < 0 {
			blank = append(blank, name)
			continue // paints nothing at rest; nothing to compare
		}
		near := func(a, b int) bool { d := b - 2*a; return d >= -4-a/16 && d <= 4+a/16 }
		if !near(ax1-ax0+1, bx1-bx0+1) || !near(ay1-ay0+1, by1-by0+1) {
			size = append(size, fmt.Sprintf("%s ink %dx%d at 1x, %dx%d at 2x", name, ax1-ax0+1, ay1-ay0+1, bx1-bx0+1, by1-by0+1))
		}
	}
	t.Logf("swept %d widgets, skipped %d", swept, skipped)
	if skipped > 0 {
		t.Errorf("%d widgets could not be swept (see the log above): give them a constructor in sweepArgs", skipped)
	}
	if swept < 100 {
		t.Fatalf("only %d widgets swept: the discovery is not finding them", swept)
	}
	slices.Sort(live)
	slices.Sort(size)
	slices.Sort(blank)
	// ⛔ A widget that paints nothing at rest hides every metric it owns (that is
	// how a baked row height stays unseen), so blank is not "nothing to compare":
	// the set must equal the documented allowlist. A new blank widget fails, and
	// so does a listed one that starts painting (remove it from the list then).
	for _, n := range blank {
		if !sweepBlankOK[n] {
			t.Errorf("%s paints nothing when freshly built and populated, so the sweep cannot see its metrics: give it content in populate, or list it in sweepBlankOK with the reason", n)
		}
	}
	for n := range sweepBlankOK {
		if !slices.Contains(blank, n) {
			t.Errorf("%s is listed in sweepBlankOK but is not blank any more (or not swept): remove it from the list", n)
		}
	}
	if len(live) > 0 {
		t.Errorf("%d: a live scale change draws differently from a fresh build:\n  %s", len(live), strings.Join(live, "\n  "))
	}
	if len(size) > 0 {
		t.Errorf("%d: ink does not double from 1x to 2x:\n  %s", len(size), strings.Join(size, "\n  "))
	}
}

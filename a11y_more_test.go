// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// a11yCase is a (name, widget, expected A11yInfo) triple shared by every
// table-driven test below.
type a11yCase struct {
	name string
	w    Accessible
	want A11yInfo
}

func checkA11yCases(t *testing.T, cases []a11yCase) {
	t.Helper()
	for _, c := range cases {
		if got := c.w.A11y(); got != c.want {
			t.Errorf("%s A11y() = %+v, want %+v", c.name, got, c.want)
		}
	}
}

func TestA11yActionInputWidgets(t *testing.T) {
	pressed := NewToggleButton("Bold", true)
	unpressed := NewToggleButton("Italic", false)
	dz := NewDropZone("Drop files here")

	checkA11yCases(t, []a11yCase{
		{"toggle-pressed", pressed, A11yInfo{Role: RoleButton, Name: "Bold", Value: "pressed"}},
		{"toggle-unpressed", unpressed, A11yInfo{Role: RoleButton, Name: "Italic", Value: ""}},
		{"iconbutton", NewIconButton("trash", nil), A11yInfo{Role: RoleButton, Name: "trash"}},
		{"splitbutton", NewSplitButton("Save", nil), A11yInfo{Role: RoleButton, Name: "Save"}},
		{"searchentry", NewSearchEntry("golang"), A11yInfo{Role: RoleSearchbox, Value: "golang"}},
		{"textview", NewTextView("line1\nline2"), A11yInfo{Role: RoleTextbox, Value: "line1\nline2"}},
		{"spinbutton", NewSpinButton(0, 10, 4, 1), A11yInfo{Role: RoleSpinbutton, Value: "4", HasRange: true, Min: 0, Max: 10, Now: 4}},
		{"rangeslider", NewRangeSlider(0, 100, 20, 80), A11yInfo{Role: RoleGroup, Value: "20..80", HasRange: true, Min: 0, Max: 100, Now: 20}},
		{"dropdown", NewDropDown([]string{"a", "b", "c"}, 1), A11yInfo{Role: RoleCombobox, Name: "b"}},
		{"rating", NewRating(3, 5), A11yInfo{Role: RoleSlider, Value: "3/5", HasRange: true, Min: 0, Max: 5, Now: 3}},
		{"colorchooser", NewColorChooser(RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}), A11yInfo{Role: RoleGroup, Value: "#102030"}},
		{"colorpicker", NewColorPicker(RGBA{R: 0xAA, G: 0xBB, B: 0xCC, A: 0xFF}), A11yInfo{Role: RoleGroup, Value: "#AABBCC"}},
		{"dropzone", dz, A11yInfo{Role: RoleGroup, Name: "Drop files here"}},
	})
}

func TestA11yFontChooser(t *testing.T) {
	fc := NewFontChooser([]FontOption{{Name: "Regular"}, {Name: "Bold"}})
	fc.Selected = 1
	if got := fc.A11y(); got != (A11yInfo{Role: RoleCombobox, Name: "Bold"}) {
		t.Errorf("FontChooser in-range A11y() = %+v", got)
	}
	fc.Selected = 99 // out of range: Name stays empty
	if got := fc.A11y(); got != (A11yInfo{Role: RoleCombobox, Name: ""}) {
		t.Errorf("FontChooser out-of-range A11y() = %+v", got)
	}
}

func TestA11yFileChooser(t *testing.T) {
	root := &TreeNode{Label: "project"}
	fc := NewFileChooser(root, nil)
	fc.selectedFile = "main.go"
	if got := fc.A11y(); got != (A11yInfo{Role: RoleGroup, Name: "project", Value: "main.go"}) {
		t.Errorf("FileChooser with root A11y() = %+v", got)
	}
	nilRoot := NewFileChooser(nil, nil)
	if got := nilRoot.A11y(); got != (A11yInfo{Role: RoleGroup, Name: "", Value: ""}) {
		t.Errorf("FileChooser nil root A11y() = %+v", got)
	}
}

func TestA11yListBox(t *testing.T) {
	l := NewListBox([]string{"one", "two", "three"})
	l.Selected().Set(1)
	if got := l.A11y(); got != (A11yInfo{Role: RoleListbox, Value: "two"}) {
		t.Errorf("ListBox single-select A11y() = %+v", got)
	}

	m := NewListBox([]string{"one", "two", "three"})
	m.MultiSelect = true
	m.SetSelection(0, 2)
	if got := m.A11y(); got != (A11yInfo{Role: RoleListbox, Value: "2 selected"}) {
		t.Errorf("ListBox multi-select A11y() = %+v", got)
	}

	empty := NewListBox(nil)
	empty.Selected().Set(-1)
	if got := empty.A11y(); got != (A11yInfo{Role: RoleListbox, Value: ""}) {
		t.Errorf("ListBox empty A11y() = %+v", got)
	}
}

func TestA11yTable(t *testing.T) {
	cols := []TableColumn{{Title: "Name"}, {Title: "Age"}}
	rows := [][]string{{"Ann", "30"}, {"Bo", "25"}}

	tb := NewTable(cols, rows)
	tb.Selected().Set(0)
	if got := tb.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "row 1 selected"}) {
		t.Errorf("Table single-select A11y() = %+v", got)
	}

	mb := NewTable(cols, rows)
	mb.MultiSelect = true
	mb.SetRowSelection(0, 1)
	if got := mb.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "2 rows selected"}) {
		t.Errorf("Table multi-select A11y() = %+v", got)
	}

	empty := NewTable(cols, nil)
	empty.Selected().Set(-1)
	if got := empty.A11y(); got != (A11yInfo{Role: RoleGrid, Value: ""}) {
		t.Errorf("Table empty A11y() = %+v", got)
	}
}

func TestA11yTreeView(t *testing.T) {
	child1 := &TreeNode{Label: "child1"}
	child2 := &TreeNode{Label: "child2"}
	root := &TreeNode{Label: "root", Expanded: true, Children: []*TreeNode{child1, child2}}

	tv := NewTreeView(root)
	tv.Selected().Set(child1)
	if got := tv.A11y(); got != (A11yInfo{Role: RoleTree, Value: "child1"}) {
		t.Errorf("TreeView single-select A11y() = %+v", got)
	}

	mv := NewTreeView(root)
	mv.MultiSelect = true
	mv.SetSelection(child1, child2)
	if got := mv.A11y(); got != (A11yInfo{Role: RoleTree, Value: "2 selected"}) {
		t.Errorf("TreeView multi-select A11y() = %+v", got)
	}

	none := NewTreeView(root)
	if got := none.A11y(); got != (A11yInfo{Role: RoleTree, Value: ""}) {
		t.Errorf("TreeView no-selection A11y() = %+v", got)
	}
}

func TestA11yViewSwitcher(t *testing.T) {
	vs := NewViewSwitcher([]string{"List", "Grid"}, 1)
	if got := vs.A11y(); got != (A11yInfo{Role: RoleTablist, Name: "Grid"}) {
		t.Errorf("ViewSwitcher in-range A11y() = %+v", got)
	}
	oor := &ViewSwitcher{Views: []string{"List", "Grid"}}
	oor.Current().Set(9)
	if got := oor.A11y(); got != (A11yInfo{Role: RoleTablist, Name: ""}) {
		t.Errorf("ViewSwitcher out-of-range A11y() = %+v", got)
	}
}

func TestA11yNotebook(t *testing.T) {
	nb := NewNotebook()
	nb.AddTab("Home", nil)
	nb.AddTab("Settings", nil)
	nb.Active().Set(1)
	if got := nb.A11y(); got != (A11yInfo{Role: RoleTablist, Name: "Settings"}) {
		t.Errorf("Notebook in-range A11y() = %+v", got)
	}
	nb.Active().Set(-1)
	if got := nb.A11y(); got != (A11yInfo{Role: RoleTablist, Name: ""}) {
		t.Errorf("Notebook out-of-range A11y() = %+v", got)
	}
}

func TestA11yNavigationAndMenuWidgets(t *testing.T) {
	checkA11yCases(t, []a11yCase{
		{"breadcrumbs", NewBreadcrumbs([]string{"Home", "Docs", "API"}), A11yInfo{Role: RoleNavigation, Name: "Home / Docs / API"}},
		{"pagination", NewPagination(3, 10), A11yInfo{Role: RoleNavigation, Value: "3/10"}},
	})
}

func TestA11ySteps(t *testing.T) {
	s := NewSteps([]string{"Account", "Payment", "Confirm"}, 1)
	if got := s.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "Payment"}) {
		t.Errorf("Steps in-range A11y() = %+v", got)
	}
	oor := &Steps{Labels: []string{"Account"}}
	oor.Current().Set(9)
	if got := oor.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("Steps out-of-range A11y() = %+v", got)
	}
}

func TestA11yMenu(t *testing.T) {
	m := NewMenu([]MenuItem{{Label: "Cut"}, {Label: "Copy"}})
	m.Hover().Set(1)
	if got := m.A11y(); got != (A11yInfo{Role: RoleMenu, Value: "Copy"}) {
		t.Errorf("Menu hover A11y() = %+v", got)
	}
	m.Hover().Set(-1)
	if got := m.A11y(); got != (A11yInfo{Role: RoleMenu, Value: ""}) {
		t.Errorf("Menu no-hover A11y() = %+v", got)
	}
}

func TestA11yMenuBar(t *testing.T) {
	mb := NewMenuBar()
	mb.AddMenu("File", NewMenu(nil))
	mb.AddMenu("Edit", NewMenu(nil))
	mb.Active = 0
	if got := mb.A11y(); got != (A11yInfo{Role: RoleMenuBar, Value: "File"}) {
		t.Errorf("MenuBar active A11y() = %+v", got)
	}
	mb.Active = -1
	if got := mb.A11y(); got != (A11yInfo{Role: RoleMenuBar, Value: ""}) {
		t.Errorf("MenuBar inactive A11y() = %+v", got)
	}
}

func TestA11yContextMenuAndCommandPalette(t *testing.T) {
	cm := NewContextMenu(NewMenu(nil))
	cm.Open = true
	if got := cm.A11y(); got != (A11yInfo{Role: RoleMenu, Value: "open"}) {
		t.Errorf("ContextMenu open A11y() = %+v", got)
	}
	cm.Open = false
	if got := cm.A11y(); got != (A11yInfo{Role: RoleMenu, Value: ""}) {
		t.Errorf("ContextMenu closed A11y() = %+v", got)
	}

	cp := NewCommandPalette([]PaletteCommand{{Label: "Open File"}})
	cp.SetQuery("open")
	if got := cp.A11y(); got != (A11yInfo{Role: RoleDialog, Value: "open"}) {
		t.Errorf("CommandPalette A11y() = %+v", got)
	}
}

func TestA11yFeedbackWidgets(t *testing.T) {
	pb := NewProgressBar()
	pb.Fraction = 0.42
	pc := NewProgressCircle()
	pc.Fraction().Set(0.5)
	lvl := NewLevelBar(10)
	lvl.Value = 4
	activeSpinner := NewSpinner()
	activeSpinner.Active().Set(true)
	idleSpinner := NewSpinner()

	checkA11yCases(t, []a11yCase{
		{"alert", NewAlert("Sync failed", AlertError), A11yInfo{Role: RoleAlert, Name: "Sync failed"}},
		{"banner", NewBanner("You are offline"), A11yInfo{Role: RoleStatus, Name: "You are offline"}},
		{"toast", NewToast("Copied", ToastInfo), A11yInfo{Role: RoleStatus, Name: "Copied"}},
		{"notification", NewNotification("New message"), A11yInfo{Role: RoleStatus, Name: "New message"}},
		{"progressbar", pb, A11yInfo{Role: RoleProgressbar, Value: "42%", HasRange: true, Min: 0, Max: 1, Now: 0.42}},
		{"levelbar", lvl, A11yInfo{Role: RoleMeter, Value: "4/10", HasRange: true, Min: 0, Max: 10, Now: 4}},
		{"progresscircle", pc, A11yInfo{Role: RoleProgressbar, Value: "50%"}},
		{"spinner-active", activeSpinner, A11yInfo{Role: RoleStatus, Value: "busy"}},
		{"spinner-idle", idleSpinner, A11yInfo{Role: RoleStatus, Value: ""}},
		{"stat", NewStat("Revenue", "$42k"), A11yInfo{Role: RoleGroup, Name: "Revenue", Value: "$42k"}},
		{"segmentedbar", NewSegmentedBar([]BarSegment{{Value: 1, Label: "a"}}), A11yInfo{Role: RoleGroup}},
	})
}

func TestA11yDisplayWidgets(t *testing.T) {
	checkA11yCases(t, []a11yCase{
		{"image", NewImage(make([]byte, 4), 1, 1), A11yInfo{Role: RoleImg}},
		{"avatar", NewAvatar("JD"), A11yInfo{Role: RoleImg, Name: "JD"}},
		{"badge", NewBadge("New"), A11yInfo{Role: RoleStatus, Name: "New"}},
		{"chip-plain", NewChip("go"), A11yInfo{Role: RoleText, Name: "go"}},
		{"chip-closable", NewClosableChip("go", nil), A11yInfo{Role: RoleButton, Name: "go"}},
		{"kbd", NewKbd("Ctrl+S"), A11yInfo{Role: RoleText, Name: "Ctrl+S"}},
		{"card", NewCard("Title", "Body", "Footer"), A11yInfo{Role: RoleGroup, Name: "Title"}},
		{"chatbubble", NewChatBubble("hi", ChatFromUser), A11yInfo{Role: RoleText, Name: "hi"}},
		{"timeline", NewTimeline([]TimelineEvent{{Title: "a"}, {Title: "b"}}), A11yInfo{Role: RoleList, Value: "2 events"}},
		{"markdownview", NewMarkdownView("# Title"), A11yInfo{Role: RoleDocument}},
		{"diff", NewDiff([]DiffLine{{Text: "+a"}, {Text: "-b"}}), A11yInfo{Role: RoleGroup, Value: "2 lines"}},
	})
}

func TestA11yChromeWidgets(t *testing.T) {
	popoverWithTitle := NewPopover(nil)
	popoverWithTitle.Title = "Filters"

	expandedExpander := NewExpander("Details", nil)
	expandedExpander.Expanded().Set(true)
	collapsedExpander := NewExpander("Details", nil)

	modalOverlay := NewOverlay(nil)
	modalOverlay.Modal = true
	plainOverlay := NewOverlay(nil)

	formWithError := NewFormField("Email", nil)
	formWithError.Error = "required"

	checkA11yCases(t, []a11yCase{
		{"headerbar", NewHeaderBar("Inbox"), A11yInfo{Role: RoleBanner, Name: "Inbox"}},
		{"statusbar", NewStatusbar([]string{"Ln 1", "UTF-8"}), A11yInfo{Role: RoleStatus, Value: "Ln 1 | UTF-8"}},
		{"dialog", NewDialog("Confirm", nil), A11yInfo{Role: RoleDialog, Name: "Confirm"}},
		{"messagedialog", NewMessageDialog("Oops", "Something broke", nil), A11yInfo{Role: RoleDialog, Name: "Oops"}},
		{"popover", popoverWithTitle, A11yInfo{Role: RoleDialog, Name: "Filters"}},
		{"tooltip", NewTooltip("More info"), A11yInfo{Role: RoleTooltip, Name: "More info"}},
		{"expander-expanded", expandedExpander, A11yInfo{Role: RoleGroup, Name: "Details", Value: "expanded"}},
		{"expander-collapsed", collapsedExpander, A11yInfo{Role: RoleGroup, Name: "Details", Value: ""}},
		{"formfield", formWithError, A11yInfo{Role: RoleGroup, Name: "Email", Value: "required"}},
		{"frame", NewFrame(nil), A11yInfo{Role: RoleGroup}},
		{"paned", NewHPaned(nil, nil), A11yInfo{Role: RoleGroup}},
		{"scrollview", NewScrollView(nil), A11yInfo{Role: RoleGroup}},
		{"overlay-modal", modalOverlay, A11yInfo{Role: RoleGroup, Value: "modal"}},
		{"overlay-plain", plainOverlay, A11yInfo{Role: RoleGroup, Value: ""}},
	})
}

func TestA11yAccordion(t *testing.T) {
	single := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}})
	single.Expanded().Set(1)
	if got := single.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "B"}) {
		t.Errorf("Accordion single-mode A11y() = %+v", got)
	}
	single.Expanded().Set(-1)
	if got := single.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("Accordion nothing-expanded A11y() = %+v", got)
	}

	multi := NewAccordion([]AccordionSection{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	multi.Multiple = true
	multi.toggle(0)
	multi.toggle(2)
	if got := multi.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "A, C"}) {
		t.Errorf("Accordion multi-mode A11y() = %+v", got)
	}
}

func TestA11yCarouselAndWizard(t *testing.T) {
	c := NewCarousel([]Widget{NewLabel("1"), NewLabel("2"), NewLabel("3")})
	c.Current().Set(1)
	if got := c.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "2/3"}) {
		t.Errorf("Carousel A11y() = %+v", got)
	}

	w := NewWizard([]WizardStep{{Title: "Account"}, {Title: "Payment"}})
	w.Current().Set(1)
	if got := w.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "Payment"}) {
		t.Errorf("Wizard in-range A11y() = %+v", got)
	}
	w.Current().Set(9)
	if got := w.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("Wizard out-of-range A11y() = %+v", got)
	}
}

func TestA11yDateWidgets(t *testing.T) {
	cal := NewCalendar(2026, 7, 27)
	if got := cal.A11y(); got != (A11yInfo{Role: RoleGrid, Value: "2026-07-27"}) {
		t.Errorf("Calendar A11y() = %+v", got)
	}

	dp := NewDatePicker(2026, 1, 5)
	if got := dp.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "2026-01-05"}) {
		t.Errorf("DatePicker with Cal A11y() = %+v", got)
	}
	nilCal := &DatePicker{}
	if got := nilCal.A11y(); got != (A11yInfo{Role: RoleGroup, Value: ""}) {
		t.Errorf("DatePicker nil Cal A11y() = %+v", got)
	}

	rp := NewDateRangePicker(2026, 1)
	rp.Start().Set(Date{Y: 2026, M: 1, D: 1})
	rp.End().Set(Date{Y: 2026, M: 1, D: 31})
	if got := rp.A11y(); got != (A11yInfo{Role: RoleGroup, Value: "2026-01-01..2026-01-31"}) {
		t.Errorf("DateRangePicker A11y() = %+v", got)
	}
}

func TestA11yCharts(t *testing.T) {
	checkA11yCases(t, []a11yCase{
		{"linechart", NewLineChart([]float64{1, 2, 3}), A11yInfo{Role: RoleImg, Value: "3 points"}},
		{"barchart", NewBarChart([]float64{1, 2}), A11yInfo{Role: RoleImg, Value: "2 bars"}},
		{"piechart", NewPieChart([]float64{1, 2, 3, 4}), A11yInfo{Role: RoleImg, Value: "4 slices"}},
	})
}

func TestA11ySkeletonActionRowToolbar(t *testing.T) {
	checkA11yCases(t, []a11yCase{
		{"skeleton", NewSkeleton(SkeletonText, 3), A11yInfo{Role: RolePresentation}},
		{"actionrow", NewActionRow("Notifications"), A11yInfo{Role: RoleGroup, Name: "Notifications"}},
		{"toolbar", NewToolbar([]ToolbarItem{{Label: "Save"}, {Label: "Open"}}), A11yInfo{Role: RoleToolbar, Value: "2 items"}},
	})
}

func TestA11yHelperFunctions(t *testing.T) {
	if got := stateValue(true, "open"); got != "open" {
		t.Errorf("stateValue(true, ...) = %q, want open", got)
	}
	if got := stateValue(false, "open"); got != "" {
		t.Errorf("stateValue(false, ...) = %q, want empty", got)
	}
	if got := formatFloat(3.5); got != "3.5" {
		t.Errorf("formatFloat(3.5) = %q, want 3.5", got)
	}
	if got := percent(0.75); got != "75%" {
		t.Errorf("percent(0.75) = %q, want 75%%", got)
	}
	if got := hexColor(RGBA{R: 1, G: 2, B: 3, A: 255}); got != "#010203" {
		t.Errorf("hexColor = %q, want #010203", got)
	}
	if got := isoDate(2026, 7, 27); got != "2026-07-27" {
		t.Errorf("isoDate = %q, want 2026-07-27", got)
	}
}

// TestA11yCollectA11yIncludesExpandedWidgets sanity-checks that CollectA11y
// (a11y.go) picks up widgets added by this file the same way it already
// picks up the original seven.
func TestA11yCollectA11yIncludesExpandedWidgets(t *testing.T) {
	widgets := []Widget{
		NewToggleButton("Bold", true),
		&HBox{}, // pure layout container: not Accessible, skipped
		NewBadge("New"),
	}
	got := CollectA11y(widgets)
	if len(got) != 2 {
		t.Fatalf("collected %d, want 2 (HBox skipped)", len(got))
	}
	if got[0].Role != RoleButton || got[1].Role != RoleStatus {
		t.Errorf("collected = %+v", got)
	}
}

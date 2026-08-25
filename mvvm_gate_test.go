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
	// AddressBar: Radius + TextPad are device-pixel layout metrics and Placeholder
	// is set-once prompt text (all config); the icon hooks are funcs and Commit is
	// an *mvvm.Command (both auto-allowed); all of its reactive state (URL /
	// Editing / Focused / Bookmarked / Copied) is unexported behind Observable
	// accessors.
	"AddressBar": {"Radius": true, "TextPad": true, "Placeholder": true},
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
	// WorkspacePager: the selected workspace index is unexported behind the
	// Current() Observable accessor; a click / arrow / Home / End Sets it. Count,
	// Labels, Occupied and Orientation are set-once layout/content config.
	"WorkspacePager": {"Count": true, "Labels": true, "Occupied": true, "Orientation": true},
	"Notebook":       {"Tabs": true, "TabSide": true},
	"DropDown":       {"Options": true, "OpenUp": true},
	"Accordion":      {"Sections": true, "Multiple": true},
	// Batch 4 (selection/range widgets): the selected index / low-high / open /
	// checked state is unexported behind accessors; the slices + flags below are
	// config (func hooks like OnActivate/OnReorder/OnCardMove/OnTaskChange/OnRefresh
	// are auto-allowed).
	"Carousel":      {"Slides": true, "Wrap": true},
	"PagingToolbar": {"PageCount": true, "ShowRefresh": true},
	"RadioButton":   {"Label": true},
	"RadioGroup":    {"Members": true},
	"ComboBox":      {"Options": true, "Placeholder": true},
	"Gantt":         {"Tasks": true, "Units": true},
	"RangeSlider":   {"Orientation": true, "Step": true, "Min": true, "Max": true},
	"Kanban":        {"Columns": true},
	// PagedView: all reactive state (mode / current page / zoom) is unexported
	// behind Observable accessors, the owned sub-widgets + pages are unexported,
	// so there is no exported state field to allow — the empty set enforces that.
	"PagedView": {},
	// Batch 5 (date/time/color/misc): the selected date/time, colour, view, step,
	// text and hover state are unexported behind accessors; the values below are
	// set-once config (func hooks auto-allowed).
	"ColorChooser": {},
	// ColorPicker: the HSV working state + alpha + derived-RGBA mirror are all
	// unexported; Color() is the bindable Observable and OnEyedrop a func hook,
	// so there is no exported state field.
	"ColorPicker": {},
	"TimePicker":  {"MinuteStep": true, "Use12h": true},
	"Calendar":    {"TodayY": true, "TodayM": true, "TodayD": true},
	"DatePicker":  {"Cal": true},
	"Wizard":      {"Steps": true, "PressFeedback": true},
	"Agenda":      {"Events": true, "DayNames": true, "StartHour": true, "EndHour": true, "Calendars": true, "Year": true, "Month": true},
	"SearchEntry": {},
	"Menu":        {"Items": true, "Scale": true},
	// Batch 6 (data/scroll + display): selection/scroll/sort/visibility/metric
	// state is unexported behind accessors; the values below are config (data
	// slices, layout flags, static captions; func hooks auto-allowed). ListBox is
	// now fully migrated (its ScrollRow joined Selected) so it finally joins here.
	"Table":     {"Columns": true, "Rows": true, "MultiSelect": true, "FrozenColumns": true, "EditActivation": true, "GroupBy": true, "Reorderable": true, "SelfSort": true, "ShowSummary": true},
	"TreeView":  {"Root": true, "RowHeight": true, "MultiSelect": true, "HideRoot": true, "HideScrollbar": true},
	"TreeTable": {"Columns": true, "Root": true},
	"ListBox":   {"Items": true, "RowHeight": true, "MultiSelect": true, "Reorderable": true, "Sections": true},
	"Toast":     {"Text": true, "Kind": true, "ActionLabel": true, "Lines": true, "Actions": true, "Pixels": true, "IW": true, "IH": true, "Icon": true},
	"Stat":      {"Title": true},
	"Sparkline": {"Values": true, "Kind": true, "Fill": true, "ShowLast": true},
	"Tooltip":   {"Text": true, "Placement": true},
	// LogView: the scrollback history is internal (unexported entries/rows,
	// mutated only through Append/Clear); the embedded viewport + its content
	// child are unexported. MaxEntries is set-once bounding config, not reactive
	// state, so it is the sole allowlisted field.
	"LogView": {"MaxEntries": true},
	// CodeMinimap is draw-only chrome: its buffer/spans/top/visible are per-paint
	// snapshots fed through Update (not settable widget state), so it holds no
	// reactive state at all. Its only exported field, OnScrollToLine, is a func
	// hook (auto-allowed), so the empty config set enforces that no imperative
	// state field is ever added.
	"CodeMinimap": {},
	// Entry: its committed contents live on the Text() Observable; the cursor
	// index and the in-flight IME preview are unexported internal editing state;
	// OnSubmit is an action func hook. Placeholder + Mask are set-once
	// appearance config, so they are the only allowlisted fields.
	"Entry": {"Placeholder": true, "Mask": true},
	// FolderTabs: the active-tab index is unexported behind the Selected
	// Observable accessor; OnSelect is a func hook (auto-allowed). Labels is the
	// set-once tab captions (layout config, not reactive state), so it is the
	// sole allowlisted field.
	"FolderTabs": {"Labels": true},
	// TabBar: the highlighted item index is unexported behind the Selected()
	// Observable accessor; a tap/swipe/arrow-key Sets it and subscribers replace
	// the old OnSelect callback. Items (destinations), SwipeNavigation and the
	// Gestures recognizer are set-once config.
	"TabBar": {"Items": true, "SwipeNavigation": true, "Gestures": true},
	// Breadcrumbs is stateless: Segments is the set-once path (config) and OnSelect
	// is a func hook. Gating it locks out any future imperative state field.
	"Breadcrumbs": {"Segments": true},
	// Surface is a draw-only view: Frame/Elements/OnInput are all func hooks
	// (auto-allowed), so it holds no reactive state at all.
	"Surface": {},
	// IconGrid / GalleryView: the selected index (-1 = none) is unexported behind
	// the Selected() Observable; SetSelected clamps+scrolls onto it and OnActivate
	// is a func hook. Cells/Items + IconSize/Empty are set-once config.
	"IconGrid":    {"Cells": true, "IconSize": true, "Empty": true},
	"GalleryView": {"Items": true, "Empty": true},
	// RichEditor: the edited document lives on the Doc() Observable, and the
	// caret / selection / focus / scroll offset are each their own Observable
	// accessor; the pending-style, selection anchor, font caches, last theme and
	// the search-match overlay ranges are unexported internal state. Its only
	// exported fields are MatchColor / CurrentMatchColor — set-once appearance
	// overrides for the search-match bands (the CodeEditor convention), not
	// reactive state — so any other new state field still fails the gate.
	"RichEditor": {"MatchColor": true, "CurrentMatchColor": true},
	// RichEditorToolbar: the bound editor, button strip, child buttons + their
	// predicates and the subscription handles are all unexported; each button's
	// lit state lives on its own Selected() Observable. IconSize and Spacing are
	// set-once layout config, the only exported fields.
	"RichEditorToolbar": {"IconSize": true, "Spacing": true},
	// The following hold no exported reactive state — their selection/toggle/scroll
	// state is unexported behind accessors or internal, and the exported fields are
	// host-set config (data slices, layout metrics, mode/style enums) or func hooks
	// (auto-allowed). Gating them locks out any future imperative state field.
	"AgendaSidebar": {"Calendars": true, "Title": true},
	"AppDock":       {"Items": true, "Magnify": true, "MaxScale": true, "Radius": true, "Style": true},
	// DockPanel holds no reactive state of its own: the launcher bar, the leading
	// and trailing accessory groups and the context menu are all set-once composed
	// widgets/config (the menu's open state lives on its own ContextMenu.Open()
	// Observable). Every exported field below is composition config.
	"DockPanel":     {"Dock": true, "Leading": true, "Trailing": true, "Menu": true},
	"Browser":       {"HideScrollbar": true, "Phase": true, "Scale": true, "HideChrome": true},
	"ColumnBrowser": {"ColumnWidth": true},
	"IsoDiagram":    {"DefaultShape": true, "Icons": true, "Mode": true, "AnimationPeriod": true, "Cols": true, "Rows": true},
	"PropertyGrid":  {},
	// Label: the string is unexported behind the Text() Observable; Align/VAlign/
	// Ellipsis/Ink/FontSize are set-once appearance config.
	"Label": {"Align": true, "VAlign": true, "Ellipsis": true, "Ink": true, "FontSize": true},
	// Clock: the displayed instant is unexported behind the Time() Observable; the
	// internal drawing Label is unexported; Func is a func hook (auto-allowed).
	// Format and Align are set-once appearance config.
	"Clock": {"Format": true, "Align": true},
	// Button: the caption + sticky selection are unexported behind Label()/Selected()
	// Observables; OnClick/Icon are func hooks; Style/PressFeedback/Flat are config.
	"Button": {"Style": true, "PressFeedback": true, "Flat": true},
	// TagField: the tag set + entry text are unexported behind Tags()/Text()
	// Observable accessors; Placeholder is set-once config.
	"TagField": {"Placeholder": true},
	// DateRangePicker: the selected range is unexported behind Start()/End()
	// Observable accessors; Cal is the composed *Calendar sub-widget (config).
	"DateRangePicker": {"Cal": true},
	// WheelPicker: the per-column selection lives in an unexported columns slice;
	// OnChange is a func hook (auto-allowed). VisibleRows is set-once config.
	"WheelPicker": {"VisibleRows": true},
	// SourceList: the selected (section,row) is unexported; OnSelect/OnReorder are
	// func hooks (auto-allowed). Sections is the set-once data (config).
	"SourceList": {"Sections": true},
	// TextView: the committed contents live on the Text() Observable and the
	// caret line/col, the scroll offset, the selection range and the focus flag
	// are each their own Observable accessor; the line buffer + IME preview are
	// unexported internal editing state. Highlighter/RowBackground are func hooks
	// (auto-allowed). Decorations (host-driven co-editor carets), ShowLineNumbers
	// and GutterColor are set-once display config.
	"TextView": {"Decorations": true, "ShowLineNumbers": true, "GutterColor": true},
	// Scroll/overlay/structural batch: each widget's reactive state is now unexported
	// behind an Observable accessor — ScrollView's offsetX/offsetY, Scrollbar's offset,
	// Paned's position, ContextMenu's open, CommandPalette's visible, DropZone's hover,
	// FontChooser's selected, MenuBar's active, FormField's error. The fields below are
	// set-once config: composed children/data slices, layout metrics and mode flags
	// (func hooks like OnDrop/OnChoose/OnDismiss/OnPositionChanged are auto-allowed).
	"ScrollView":     {"Child": true},
	"Scrollbar":      {"Total": true, "Viewport": true, "Horizontal": true},
	"Paned":          {"First": true, "Second": true, "Orientation": true},
	"ContextMenu":    {"Menu": true, "AnchorX": true, "AnchorY": true},
	"CommandPalette": {"Commands": true},
	"DropZone":       {"Prompt": true},
	"FontChooser":    {"Options": true},
	"MenuBar":        {"Names": true, "Menus": true},
	"FormField":      {"Label": true, "Help": true, "Child": true, "Rules": true},
	// Builder/overlay widgets whose only reactive state (open/present/completion) is
	// already unexported behind an Observable (ActionSheet.presented, Fab.expanded,
	// CodeEditor.compOpen/compSel) or which are purely draw-only chrome with no
	// reactive state at all (Border/Material/Overlay/Skeleton). Gating them locks out
	// any future imperative state field; every exported field below is set-once
	// composition/appearance/layout config (func hooks auto-allowed).
	"ActionSheet": {"Title": true, "Actions": true, "Cancel": true, "Content": true, "PreferredHeight": true, "Detents": true, "Draggable": true, "ShowHandle": true, "DismissFraction": true, "FlingVelocity": true, "SlideDuration": true, "FrameSeconds": true, "ScrimAlpha": true},
	"Border":      {"North": true, "South": true, "East": true, "West": true, "Center": true, "NorthSize": true, "SouthSize": true, "EastSize": true, "WestSize": true, "NorthSplit": true, "SouthSplit": true, "EastSplit": true, "WestSplit": true},
	"CodeEditor":  {"Language": true, "Syntax": true, "HighlightCurrentLine": true, "CurrentLineColor": true, "MatchColor": true, "CurrentMatchColor": true},
	// FindReplace: the query/replace text, the three search toggles and the
	// count/index/invalid/visible state all live on unexported Observables behind
	// accessors (Query/Replace/Regex/CaseSensitive/WholeWord/Total/Current/
	// Invalid/Visible); the composed children are unexported. Its only exported
	// fields are the OnX func hooks (auto-allowed), so the empty config set
	// enforces that no imperative state field is ever added.
	"FindReplace":   {},
	"Fab":           {"Icon": true, "Label": true, "Corner": true, "Margin": true, "Diameter": true, "Actions": true},
	"Material":      {"Kind": true, "Blend": true, "Source": true, "SW": true, "SH": true, "Sigma": true, "Tint": true, "Child": true},
	"Overlay":       {"Content": true, "Layers": true, "Modal": true},
	"Skeleton":      {"Kind": true, "Lines": true, "LineH": true, "LineGap": true, "LastFrac": true, "Radius": true, "Animated": true, "Phase": true},
	"SkeletonGroup": {"Animated": true, "Phase": true},
	// Reactive-tail batch: the last widgets holding exported reactive state now
	// keep it on unexported *mvvm.Observable behind accessors — LoadMask.Active,
	// Notification/Popover.Visible, Banner.Revealed, Stack.Visible, Gauge/LevelBar
	// .Value, ProgressBar.Fraction, Thumbnail.Selected/Hover, GroupCard.Checked/
	// Expanded, and every chart's Hover + hovered-index. The fields below are
	// set-once config (data series, ranges, labels, palettes, layout metrics; func
	// hooks like OnDrop/OnAction/OnClick auto-allowed). ProgressBar.Phase and the
	// chart hover-index observables mirror the Spinner.Phase animation precedent.
	"LoadMask":     {"Message": true, "Scrim": true},
	"Notification": {"Text": true, "Life": true},
	"Popover":      {"Child": true, "Title": true},
	"Banner":       {"Text": true, "ButtonLabel": true},
	"Stack":        {"Pages": true},
	"Gauge":        {"Min": true, "Max": true, "Bands": true, "Caption": true, "Thickness": true},
	"LevelBar":     {"Max": true, "Orientation": true, "Label": true, "Thresholds": true},
	"ProgressBar":  {"Label": true, "Orientation": true, "Indeterminate": true, "Phase": true},
	"Thumbnail":    {"Pixels": true, "IW": true, "IH": true, "Label": true, "Alt": true, "Area": true},
	"GroupCard":    {"Pill": true, "PillColor": true, "PillInk": true, "Status": true, "StatusColor": true, "StatusInk": true, "Title": true, "Meta": true, "Members": true, "Actionable": true, "Action": true, "TitleFont": true, "MetaFont": true, "PillFont": true},
	"AreaChart":    {"Series": true, "Min": true, "Max": true, "Colors": true},
	"BarChart":     {"Values": true, "Max": true},
	"LineChart":    {"Series": true, "Min": true, "Max": true},
	"PieChart":     {"Values": true, "Colors": true},
	"RadarChart":   {"Axes": true, "Series": true, "Max": true, "Colors": true},
	"ScatterChart": {"Series": true, "Colors": true},
	// Base is embedded by every widget: its Disabled inert-state flag is now the
	// unexported `disabled` behind the Disabled() accessor (so `w.Disabled = x`
	// fails to compile fleet-wide), leaving Font as the sole set-once config field.
	"Base": {"Font": true},
	// Config/display-chrome gate-lock: these Draw-widgets hold NO reactive UI
	// state — every exported field is set-once content/appearance/layout config
	// (or a func hook, auto-allowed). Gating them locks out any future imperative
	// reactive field. Two documented judgement calls: TerminalView's CursorCol/
	// CursorRow/CursorVisible are the emulator's own write-head over its grid
	// document (mutated by Write/Put/SetCell — a document model like TextView's
	// line buffer, not app-bound reactive state); MarkdownEditor's SideBySide/
	// Preview/Split are host-set view configuration, not a cross-component
	// Observable. Window's Closable/Resizable/etc. are capability flags, not a
	// current-state.
	"ActionRow":        {"Prefix": true, "Subtitle": true, "Suffix": true, "Title": true},
	"Alert":            {"Kind": true, "Text": true},
	"ArticleCard":      {"Body": true, "BodyLines": true, "Meta": true, "Title": true},
	"Avatar":           {"Color": true, "Initials": true},
	"Backdrop":         {"Bevel": true, "Fill": true, "GradientDir": true, "GradientTo": true, "Grid": true, "Interactive": true, "NoFill": true, "Radius": true, "Step": true, "Stroke": true, "StrokeWidth": true},
	"Badge":            {"Fill": true, "Ink": true, "Text": true},
	"ButtonGroup":      {"Buttons": true, "Orientation": true},
	"Card":             {"Body": true, "Footer": true, "Title": true},
	"CardMeta":         {"Author": true, "Comments": true, "Score": true, "Time": true},
	"ChatBubble":       {"Sender": true, "Text": true},
	"Chip":             {"Closable": true, "Dot": true, "Text": true},
	"Container":        {"Layout": true},
	"DatabaseEditor":   {"BarHeight": true, "EditorHeight": true, "ErrorHeight": true, "TreeWidth": true},
	"Dialog":           {"Buttons": true, "Content": true, "Title": true},
	"HeaderBar":        {"End": true, "Start": true, "Subtitle": true, "Title": true},
	"IconButton":       {"Icon": true},
	"Image":            {"Alt": true, "H": true, "Pixels": true, "Scale": true, "W": true},
	"IsoIconPalette":   {"DefaultGroupName": true, "Title": true},
	"Kbd":              {"Keys": true},
	"HBox":             {"Align": true, "Pack": true, "Spacing": true},
	"VBox":             {"Align": true, "Pack": true, "Spacing": true},
	"Grid":             {"ColWidths": true, "RowHeights": true, "Spacing": true},
	"LinkCard":         {"Domain": true, "Favicon": true, "Meta": true, "Title": true},
	"MarkdownEditor":   {"Preview": true, "Source": true},
	"MarkdownView":     {"Source": true},
	"MediaCard":        {"Meta": true, "Thumbnail": true, "Title": true},
	"PostCard":         {"MaxTitleLines": true, "Meta": true, "MetaFont": true, "Pill": true, "PillColor": true, "PillFont": true, "PillInk": true, "Subtitle": true, "SubtitleFont": true, "ThumbH": true, "ThumbPlaceholder": true, "ThumbW": true, "Thumbnail": true, "Title": true, "TitleFont": true},
	"PullToRefresh":    {"Child": true, "Style": true, "Threshold": true},
	"SegmentedBar":     {"Orientation": true, "Segments": true},
	"SplitButton":      {"Arrow": true, "Label": true},
	"Statusbar":        {"SegmentMinW": true, "Segments": true, "Left": true, "Center": true, "Right": true},
	"StatusIcon":       {"Badge": true, "IH": true, "IW": true, "Icon": true, "Ink": true, "Pixels": true, "Tooltip": true},
	"StatusArea":       {"Background": true, "Gap": true, "IconSize": true, "Icons": true},
	"SwipeActions":     {"ActionWidth": true, "Content": true, "DestructiveDen": true, "DestructiveFull": true, "DestructiveNum": true, "Leading": true, "Projection": true, "Trailing": true},
	"TerminalView":     {"CellH": true, "CellW": true, "Cells": true, "Cols": true, "DefaultBG": true, "DefaultFG": true, "Rows": true},
	"Timeline":         {"Events": true, "Horizontal": true},
	"Toolbar":          {"ButtonH": true, "ButtonW": true, "Items": true, "Orientation": true},
	"Wallpaper":        {"Bottom": true, "IH": true, "IW": true, "Interactive": true, "Mode": true, "Pixels": true, "Top": true},
	"Window":           {"Body": true, "Closable": true, "Maximizable": true, "Minimizable": true, "Resizable": true, "Title": true},
	"WindowDecoration": {"Border": true, "BorderColor": true, "Buttons": true, "Grip": true, "GripColor": true, "Hairline": true, "Shadow": true, "ShowGrip": true, "Title": true, "TitleCenter": true, "TitleColor": true, "TitleInk": true, "Titlebar": true},
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

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strconv"
	"strings"
)

// Additional Role constants for the widgets implemented in this file. See
// a11y.go for the original seven roles (button/text/textbox/checkbox/
// radio/switch/slider) and the Accessible/A11yInfo/CollectA11y machinery
// they share with everything below.
const (
	RoleSearchbox    Role = "searchbox"
	RoleSpinbutton   Role = "spinbutton"
	RoleCombobox     Role = "combobox"
	RoleListbox      Role = "listbox"
	RoleGrid         Role = "grid"
	RoleTree         Role = "tree"
	RoleTablist      Role = "tablist"
	RoleNavigation   Role = "navigation"
	RoleMenu         Role = "menu"
	RoleMenuBar      Role = "menubar"
	RoleAlert        Role = "alert"
	RoleStatus       Role = "status"
	RoleProgressbar  Role = "progressbar"
	RoleMeter        Role = "meter"
	RoleImg          Role = "img"
	RoleGroup        Role = "group"
	RoleDialog       Role = "dialog"
	RoleTooltip      Role = "tooltip"
	RoleBanner       Role = "banner"
	RoleList         Role = "list"
	RoleDocument     Role = "document"
	RoleToolbar      Role = "toolbar"
	RolePresentation Role = "presentation"
	// RoleHeading is a non-selectable group caption; RoleListItem is one
	// selectable entry of a list. A sectioned ListBox exposes its section
	// captions as headings and its rows as list items (see ListBox.Children).
	RoleHeading  Role = "heading"
	RoleListItem Role = "listitem"
	// RoleLink is a hyperlink — text that navigates when activated (see Link).
	RoleLink Role = "link"
)

// stateValue returns word when on is true, "" otherwise. It is the shared
// idiom behind every boolean-state Value string below (pressed, expanded,
// open, modal, busy, ...) -- the wider-vocabulary sibling of checkedValue,
// which is pinned to the word "checked" for the original checkbox/radio
// widgets.
func stateValue(on bool, word string) string {
	if on {
		return word
	}
	return ""
}

// formatFloat renders v the same way Scale.A11y (a11y.go) already does, for
// widgets that report a raw float64 in their Value.
func formatFloat(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// percent renders a 0..1 fraction as a whole-number percentage string.
func percent(fraction float64) string { return strconv.Itoa(int(fraction*100)) + "%" }

// hexColor renders c's RGB channels (ignoring Alpha) as a "#RRGGBB" string.
func hexColor(c RGBA) string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// isoDate renders a (year, month, day) triple as "YYYY-MM-DD".
func isoDate(y, m, d int) string { return fmt.Sprintf("%04d-%02d-%02d", y, m, d) }

// --- Action / input ---------------------------------------------------

// A11y reports the ToggleButton as a button carrying its pressed state.
func (t *ToggleButton) A11y() A11yInfo {
	return A11yInfo{Role: RoleButton, Name: t.Label, Value: stateValue(t.Pressed().Get(), "pressed")}
}

// A11y reports the IconButton as a button named by its icon identifier (it
// has no separate text label).
func (b *IconButton) A11y() A11yInfo { return A11yInfo{Role: RoleButton, Name: b.Icon} }

// A11y reports the SplitButton as a button named by its label.
func (b *SplitButton) A11y() A11yInfo { return A11yInfo{Role: RoleButton, Name: b.Label} }

// A11y reports the SearchEntry as a searchbox carrying its current text.
func (s *SearchEntry) A11y() A11yInfo { return A11yInfo{Role: RoleSearchbox, Value: s.Text().Get()} }

// A11y reports the TextView as a textbox carrying its full buffer text.
func (t *TextView) A11y() A11yInfo { return A11yInfo{Role: RoleTextbox, Value: t.Text().Get()} }

// A11y reports the SpinButton as a spinbutton carrying its numeric value, both
// as a Value string and as the numeric Min/Max/Now range triple.
func (s *SpinButton) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleSpinbutton,
		Value:    strconv.Itoa(s.Value().Get()),
		HasRange: true,
		Min:      float64(s.Min),
		Max:      float64(s.Max),
		Now:      float64(s.Value().Get()),
	}
}

// A11y reports the RangeSlider as a group carrying its "low..high" band --
// two cooperating handles read more naturally as one control's range value
// than as two independent sliders. The numeric triple exposes the track
// bounds in Min/Max; Now carries the Low handle (the arrow keys' default
// handle), since a single aria-valuenow cannot hold both -- the full band
// stays in Value.
func (s *RangeSlider) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleGroup,
		Value:    formatFloat(s.Low().Get()) + ".." + formatFloat(s.High().Get()),
		HasRange: true,
		Min:      s.Min,
		Max:      s.Max,
		Now:      s.Low().Get(),
	}
}

// A11y reports the DropDown as a combobox named by its currently-selected
// option.
func (d *DropDown) A11y() A11yInfo { return A11yInfo{Role: RoleCombobox, Name: d.Current()} }

// A11y reports the Rating as a slider carrying its "value/max" score, plus the
// numeric Min/Max/Now range triple (Min is 0, the empty rating).
func (r *Rating) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleSlider,
		Value:    strconv.Itoa(r.Value().Get()) + "/" + strconv.Itoa(r.Max),
		HasRange: true,
		Min:      0,
		Max:      float64(r.Max),
		Now:      float64(r.Value().Get()),
	}
}

// A11y reports the ColorChooser as a group carrying its current colour as
// a "#RRGGBB" hex string.
func (c *ColorChooser) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: hexColor(c.Color().Get())}
}

// A11y reports the ColorPicker as a group carrying its current colour
// (derived from the HSV+alpha state) as a "#RRGGBB" hex string.
func (c *ColorPicker) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: hexColor(c.rgba())}
}

// A11y reports the FontChooser as a combobox named by its currently-selected
// font option.
func (f *FontChooser) A11y() A11yInfo {
	name := ""
	if f.Selected().Get() >= 0 && f.Selected().Get() < len(f.Options) {
		name = f.Options[f.Selected().Get()].Name
	}
	return A11yInfo{Role: RoleCombobox, Name: name}
}

// A11y reports the FileChooser as a group named by its root directory,
// carrying the currently-selected file path as its Value.
func (f *FileChooser) A11y() A11yInfo {
	name := ""
	if f.Root != nil {
		name = f.Root.Label
	}
	return A11yInfo{Role: RoleGroup, Name: name, Value: f.selectedFile}
}

// A11y reports the DropZone as a group named by its drop prompt.
func (d *DropZone) A11y() A11yInfo { return A11yInfo{Role: RoleGroup, Name: d.Prompt} }

// --- Selection / data ---------------------------------------------------

// A11y reports the ListBox as a listbox. Value is the selected item's text
// in single-select mode, or a "N selected" count while MultiSelect is on.
func (l *ListBox) A11y() A11yInfo {
	v := ""
	if l.MultiSelect {
		if n := len(l.SelectedIndices()); n > 0 {
			v = strconv.Itoa(n) + " selected"
		}
	} else if sel := l.Selected().Get(); sel >= 0 && sel < l.itemCount() {
		v = l.flatItems()[sel]
	}
	return A11yInfo{Role: RoleListbox, Value: v}
}

// A11y reports the Table as a grid. Value names the selected row in
// single-select mode, or a "N rows selected" count while MultiSelect is on.
func (t *Table) A11y() A11yInfo {
	v := ""
	if t.MultiSelect {
		if n := len(t.SelectedRows()); n > 0 {
			v = strconv.Itoa(n) + " rows selected"
		}
	} else if sel := t.Selected().Get(); sel >= 0 && sel < len(t.Rows) {
		v = "row " + strconv.Itoa(sel+1) + " selected"
	}
	return A11yInfo{Role: RoleGrid, Value: v}
}

// A11y reports the TreeView as a tree. Value is the selected node's label in
// single-select mode, or a "N selected" count while MultiSelect is on.
func (t *TreeView) A11y() A11yInfo {
	v := ""
	if t.MultiSelect {
		if n := len(t.SelectedNodes()); n > 0 {
			v = strconv.Itoa(n) + " selected"
		}
	} else if sel := t.Selected().Get(); sel != nil {
		v = sel.Label
	}
	return A11yInfo{Role: RoleTree, Value: v}
}

// A11y reports the ViewSwitcher as a tablist named by its current view.
func (v *ViewSwitcher) A11y() A11yInfo {
	name := ""
	if cur := v.Current().Get(); cur >= 0 && cur < len(v.Views) {
		name = v.Views[cur]
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}

// A11y reports the Notebook as a tablist named by its active tab.
func (n *Notebook) A11y() A11yInfo {
	name := ""
	if active := n.Active().Get(); active >= 0 && active < len(n.Tabs) {
		name = n.Tabs[active].Label
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}

// --- Navigation / menu ---------------------------------------------------

// A11y reports the Breadcrumbs as navigation named by its full path.
func (b *Breadcrumbs) A11y() A11yInfo {
	return A11yInfo{Role: RoleNavigation, Name: strings.Join(b.Segments, " / ")}
}

// A11y reports the Pagination as navigation carrying its "current/total"
// page position.
func (p *Pagination) A11y() A11yInfo {
	return A11yInfo{Role: RoleNavigation, Value: strconv.Itoa(p.Current().Get()) + "/" + strconv.Itoa(p.Total)}
}

// A11y reports the Steps strip as a group carrying its current step's label.
func (s *Steps) A11y() A11yInfo {
	v := ""
	c := s.Current().Get()
	if c >= 0 && c < len(s.Labels) {
		v = s.Labels[c]
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the Menu as a menu carrying the hovered row's label, if any.
func (m *Menu) A11y() A11yInfo {
	v := ""
	if hv := m.Hover().Get(); hv >= 0 && hv < len(m.Items) {
		v = m.Items[hv].Label
	}
	return A11yInfo{Role: RoleMenu, Value: v}
}

// A11y reports the MenuBar as a menubar carrying the currently-open top-level
// menu's name, if any.
func (m *MenuBar) A11y() A11yInfo {
	v := ""
	if m.Active().Get() >= 0 && m.Active().Get() < len(m.Names) {
		v = m.Names[m.Active().Get()]
	}
	return A11yInfo{Role: RoleMenuBar, Value: v}
}

// A11y reports the ContextMenu as a menu carrying its open/closed state.
func (c *ContextMenu) A11y() A11yInfo {
	return A11yInfo{Role: RoleMenu, Value: stateValue(c.Open().Get(), "open")}
}

// A11y reports the CommandPalette as a dialog carrying its typed query.
func (c *CommandPalette) A11y() A11yInfo { return A11yInfo{Role: RoleDialog, Value: c.query} }

// --- Feedback / status ---------------------------------------------------

// A11y reports the Alert as an alert named by its message.
func (a *Alert) A11y() A11yInfo { return A11yInfo{Role: RoleAlert, Name: a.Text} }

// A11y reports the Banner as a status region named by its message.
func (b *Banner) A11y() A11yInfo { return A11yInfo{Role: RoleStatus, Name: b.Text} }

// A11y reports the Toast as a status region named by its message.
func (t *Toast) A11y() A11yInfo { return A11yInfo{Role: RoleStatus, Name: t.Text} }

// A11y reports the Notification as a status region named by its message.
func (n *Notification) A11y() A11yInfo { return A11yInfo{Role: RoleStatus, Name: n.Text} }

// A11y reports the ProgressBar as a progressbar carrying its fraction as a
// whole-number percentage, plus the numeric range triple over the fraction's
// natural [0, 1] span (Now is the raw fraction).
func (p *ProgressBar) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleProgressbar,
		Value:    percent(p.Fraction().Get()),
		HasRange: true,
		Min:      0,
		Max:      1,
		Now:      p.Fraction().Get(),
	}
}

// A11y reports the LevelBar as a meter carrying its "value/max" reading, plus
// the numeric Min/Max/Now range triple (Min is 0, the empty meter).
func (l *LevelBar) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleMeter,
		Value:    strconv.Itoa(l.Value().Get()) + "/" + strconv.Itoa(l.Max),
		HasRange: true,
		Min:      0,
		Max:      float64(l.Max),
		Now:      float64(l.Value().Get()),
	}
}

// A11y reports the ProgressCircle as a progressbar carrying its fraction as
// a whole-number percentage.
func (p *ProgressCircle) A11y() A11yInfo {
	return A11yInfo{Role: RoleProgressbar, Value: percent(p.Fraction().Get())}
}

// A11y reports the Spinner as a status region carrying "busy" while active.
func (s *Spinner) A11y() A11yInfo {
	return A11yInfo{Role: RoleStatus, Value: stateValue(s.Active().Get(), "busy")}
}

// A11y reports the Stat as a group named by its title, carrying its
// headline value.
func (s *Stat) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: s.Title, Value: s.Value().Get()}
}

// A11y reports the SegmentedBar as a group. The individual segments carry
// no independent accessible identity (BarSegment is a plain data struct,
// not a Widget), so the bar as a whole is the accessible unit.
func (s *SegmentedBar) A11y() A11yInfo { return A11yInfo{Role: RoleGroup} }

// --- Display ---------------------------------------------------

// A11y reports the Image as an img named by its Alt text.
func (i *Image) A11y() A11yInfo { return A11yInfo{Role: RoleImg, Name: i.Alt} }

// A11y reports the Avatar as an img named by its initials.
func (a *Avatar) A11y() A11yInfo { return A11yInfo{Role: RoleImg, Name: a.Initials} }

// A11y reports the Badge as a status region named by its text.
func (b *Badge) A11y() A11yInfo { return A11yInfo{Role: RoleStatus, Name: b.Text} }

// A11y reports the Chip as a button when it exposes a close affordance
// (Closable), or as plain text otherwise.
func (c *Chip) A11y() A11yInfo {
	role := RoleText
	if c.Closable {
		role = RoleButton
	}
	return A11yInfo{Role: role, Name: c.Text}
}

// A11y reports the Kbd as text naming the key combination it renders.
func (k *Kbd) A11y() A11yInfo { return A11yInfo{Role: RoleText, Name: k.Keys} }

// A11y reports the Card as a group named by its title.
func (c *Card) A11y() A11yInfo { return A11yInfo{Role: RoleGroup, Name: c.Title} }

// A11y reports the ChatBubble as text carrying its message.
func (c *ChatBubble) A11y() A11yInfo { return A11yInfo{Role: RoleText, Name: c.Text} }

// A11y reports the Timeline as a list carrying its event count.
func (t *Timeline) A11y() A11yInfo {
	return A11yInfo{Role: RoleList, Value: strconv.Itoa(len(t.Events)) + " events"}
}

// A11y reports the MarkdownView as a document. Source is typically a full
// document body, too long to usefully surface as an accessible Name.
func (m *MarkdownView) A11y() A11yInfo { return A11yInfo{Role: RoleDocument} }

// A11y reports the Diff as a group carrying its line count.
func (d *Diff) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: strconv.Itoa(len(d.Lines)) + " lines"}
}

// --- Chrome / overlay / layout ---------------------------------------------------

// A11y reports the HeaderBar as a banner named by its title.
func (h *HeaderBar) A11y() A11yInfo { return A11yInfo{Role: RoleBanner, Name: h.Title} }

// A11y reports the Statusbar as a status region carrying its segments joined
// into one string: the plain text Segments first, then every interactive
// Left/Center/Right segment's label (its Text, or its hosted widget's accessible
// name/value when it has no text). With no interactive segments this is exactly
// strings.Join(Segments, " | ") — the pre-groups behaviour.
func (s *Statusbar) A11y() A11yInfo {
	parts := append([]string(nil), s.Segments...)
	for _, g := range [][]StatusSegment{s.Left, s.Center, s.Right} {
		for _, seg := range g {
			parts = append(parts, statusSegLabel(seg))
		}
	}
	return A11yInfo{Role: RoleStatus, Value: strings.Join(parts, " | ")}
}

// statusSegLabel is the accessible label of one interactive segment: its Text
// when set, else its hosted widget's accessible Name (or Value when the widget
// names itself only by value), else the empty string.
func statusSegLabel(seg StatusSegment) string {
	if seg.Text != "" {
		return seg.Text
	}
	a, ok := seg.Widget.(Accessible)
	if !ok {
		return ""
	}
	info := a.A11y()
	if info.Name != "" {
		return info.Name
	}
	return info.Value
}

// A11y reports the Dialog as a dialog named by its title. This also covers
// NewMessageDialog, which returns a plain *Dialog rather than a distinct
// type.
func (d *Dialog) A11y() A11yInfo { return A11yInfo{Role: RoleDialog, Name: d.Title} }

// A11y reports the Popover as a dialog named by its title.
func (p *Popover) A11y() A11yInfo { return A11yInfo{Role: RoleDialog, Name: p.Title} }

// A11y reports the Tooltip as a tooltip carrying its text.
func (t *Tooltip) A11y() A11yInfo { return A11yInfo{Role: RoleTooltip, Name: t.Text} }

// A11y reports the Expander as a group named by its label, carrying its
// expanded/collapsed state.
func (e *Expander) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: e.Label, Value: stateValue(e.Expanded().Get(), "expanded")}
}

// A11y reports the Accordion as a group carrying the titles of every
// currently-expanded section (one in single mode, zero or more in Multiple
// mode), joined together.
func (a *Accordion) A11y() A11yInfo {
	var open []string
	for i, s := range a.Sections {
		if a.isExpanded(i) {
			open = append(open, s.Title)
		}
	}
	return A11yInfo{Role: RoleGroup, Value: strings.Join(open, ", ")}
}

// A11y reports the Carousel as a group carrying its "current/total" slide
// position.
func (c *Carousel) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: strconv.Itoa(c.Current().Get()+1) + "/" + strconv.Itoa(len(c.Slides))}
}

// A11y reports the Wizard as a group carrying its current step's title.
func (w *Wizard) A11y() A11yInfo {
	v := ""
	if cur := w.Current().Get(); cur >= 0 && cur < len(w.Steps) {
		v = w.Steps[cur].Title
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the FormField as a group named by its label, carrying its
// error text (if any) as Value.
func (f *FormField) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: f.Label, Value: f.Error().Get()}
}

// A11y reports the Frame as a plain grouping container. Frame carries no
// title text of its own (see the type doc), so Name is always empty --
// unlike the other "group" widgets above that surface a label.
func (f *Frame) A11y() A11yInfo { return A11yInfo{Role: RoleGroup} }

// A11y reports the Paned as a plain grouping container for its two panes.
func (p *Paned) A11y() A11yInfo { return A11yInfo{Role: RoleGroup} }

// A11y reports the ScrollView as a plain grouping container for its child.
func (s *ScrollView) A11y() A11yInfo { return A11yInfo{Role: RoleGroup} }

// A11y reports the Overlay as a group carrying its modal state.
func (o *Overlay) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: stateValue(o.Modal, "modal")}
}

// A11y reports the Calendar as a grid carrying its selected date.
func (c *Calendar) A11y() A11yInfo {
	return A11yInfo{Role: RoleGrid, Value: isoDate(c.Year().Get(), c.Month().Get(), c.Day().Get())}
}

// A11y reports the DatePicker as a group carrying its selected date.
func (d *DatePicker) A11y() A11yInfo {
	v := ""
	if d.Cal != nil {
		v = isoDate(d.Cal.Year().Get(), d.Cal.Month().Get(), d.Cal.Day().Get())
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the DateRangePicker as a group carrying its "start..end"
// date range.
func (d *DateRangePicker) A11y() A11yInfo {
	start, end := d.Start().Get(), d.End().Get()
	return A11yInfo{
		Role:  RoleGroup,
		Value: isoDate(start.Y, start.M, start.D) + ".." + isoDate(end.Y, end.M, end.D),
	}
}

// --- Charts ---------------------------------------------------

// A11y reports the LineChart as an img carrying its data-point count.
func (l *LineChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(l.Series)) + " points"}
}

// A11y reports the BarChart as an img carrying its bar count.
func (b *BarChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(b.Values)) + " bars"}
}

// A11y reports the PieChart as an img carrying its slice count.
func (p *PieChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(p.Values)) + " slices"}
}

// --- Everything else ---------------------------------------------------
//
// HBox, VBox, Grid and Stack are pure layout containers -- they have no
// label, value or semantic role of their own beyond arranging children that
// (if accessible) already report themselves via CollectA11y. Giving them a
// role would just add a redundant, unnamed node to the accessibility tree,
// so they deliberately do NOT implement Accessible.
//
// Skeleton is the opposite case: it is a decorative loading placeholder
// that a screen reader should skip entirely, so it reports the ARIA
// "presentation" role rather than being skipped outright.

// A11y reports the Skeleton as a decorative presentation element -- a
// screen reader should not announce a loading placeholder as content.
func (s *Skeleton) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the SkeletonGroup as a decorative presentation element,
// for the same reason as the primitive Skeleton: a composed loading
// placeholder is not content.
func (g *SkeletonGroup) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// A11y reports the ActionRow as a group named by its title.
func (a *ActionRow) A11y() A11yInfo { return A11yInfo{Role: RoleGroup, Name: a.Title} }

// A11y reports the Toolbar as a toolbar carrying its item count.
func (t *Toolbar) A11y() A11yInfo {
	return A11yInfo{Role: RoleToolbar, Value: strconv.Itoa(len(t.Items)) + " items"}
}

// Compile-time checks that each widget added in this file satisfies
// Accessible, mirroring the block at the bottom of a11y.go.
var (
	_ Accessible = (*ToggleButton)(nil)
	_ Accessible = (*IconButton)(nil)
	_ Accessible = (*SplitButton)(nil)
	_ Accessible = (*SearchEntry)(nil)
	_ Accessible = (*TextView)(nil)
	_ Accessible = (*SpinButton)(nil)
	_ Accessible = (*RangeSlider)(nil)
	_ Accessible = (*DropDown)(nil)
	_ Accessible = (*Rating)(nil)
	_ Accessible = (*ColorChooser)(nil)
	_ Accessible = (*ColorPicker)(nil)
	_ Accessible = (*FontChooser)(nil)
	_ Accessible = (*FileChooser)(nil)
	_ Accessible = (*DropZone)(nil)
	_ Accessible = (*ListBox)(nil)
	_ Accessible = (*Table)(nil)
	_ Accessible = (*TreeView)(nil)
	_ Accessible = (*ViewSwitcher)(nil)
	_ Accessible = (*Notebook)(nil)
	_ Accessible = (*Breadcrumbs)(nil)
	_ Accessible = (*Pagination)(nil)
	_ Accessible = (*Steps)(nil)
	_ Accessible = (*Menu)(nil)
	_ Accessible = (*MenuBar)(nil)
	_ Accessible = (*ContextMenu)(nil)
	_ Accessible = (*CommandPalette)(nil)
	_ Accessible = (*Alert)(nil)
	_ Accessible = (*Banner)(nil)
	_ Accessible = (*Toast)(nil)
	_ Accessible = (*Notification)(nil)
	_ Accessible = (*ProgressBar)(nil)
	_ Accessible = (*LevelBar)(nil)
	_ Accessible = (*ProgressCircle)(nil)
	_ Accessible = (*Spinner)(nil)
	_ Accessible = (*Stat)(nil)
	_ Accessible = (*SegmentedBar)(nil)
	_ Accessible = (*Image)(nil)
	_ Accessible = (*Avatar)(nil)
	_ Accessible = (*Badge)(nil)
	_ Accessible = (*Chip)(nil)
	_ Accessible = (*Kbd)(nil)
	_ Accessible = (*Card)(nil)
	_ Accessible = (*ChatBubble)(nil)
	_ Accessible = (*Timeline)(nil)
	_ Accessible = (*MarkdownView)(nil)
	_ Accessible = (*Diff)(nil)
	_ Accessible = (*HeaderBar)(nil)
	_ Accessible = (*Statusbar)(nil)
	_ Accessible = (*Dialog)(nil)
	_ Accessible = (*Popover)(nil)
	_ Accessible = (*Tooltip)(nil)
	_ Accessible = (*Expander)(nil)
	_ Accessible = (*Accordion)(nil)
	_ Accessible = (*Carousel)(nil)
	_ Accessible = (*Wizard)(nil)
	_ Accessible = (*FormField)(nil)
	_ Accessible = (*Frame)(nil)
	_ Accessible = (*Paned)(nil)
	_ Accessible = (*ScrollView)(nil)
	_ Accessible = (*Overlay)(nil)
	_ Accessible = (*Calendar)(nil)
	_ Accessible = (*DatePicker)(nil)
	_ Accessible = (*DateRangePicker)(nil)
	_ Accessible = (*LineChart)(nil)
	_ Accessible = (*BarChart)(nil)
	_ Accessible = (*PieChart)(nil)
	_ Accessible = (*Skeleton)(nil)
	_ Accessible = (*SkeletonGroup)(nil)
	_ Accessible = (*ActionRow)(nil)
	_ Accessible = (*Toolbar)(nil)
)

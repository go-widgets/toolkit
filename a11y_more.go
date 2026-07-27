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
	return A11yInfo{Role: RoleButton, Name: t.Label, Value: stateValue(t.Pressed, "pressed")}
}

// A11y reports the IconButton as a button named by its icon identifier (it
// has no separate text label).
func (b *IconButton) A11y() A11yInfo { return A11yInfo{Role: RoleButton, Name: b.Icon} }

// A11y reports the SplitButton as a button named by its label.
func (b *SplitButton) A11y() A11yInfo { return A11yInfo{Role: RoleButton, Name: b.Label} }

// A11y reports the SearchEntry as a searchbox carrying its current text.
func (s *SearchEntry) A11y() A11yInfo { return A11yInfo{Role: RoleSearchbox, Value: s.Text} }

// A11y reports the TextView as a textbox carrying its full buffer text.
func (t *TextView) A11y() A11yInfo { return A11yInfo{Role: RoleTextbox, Value: t.Text()} }

// A11y reports the SpinButton as a spinbutton carrying its numeric value.
func (s *SpinButton) A11y() A11yInfo {
	return A11yInfo{Role: RoleSpinbutton, Value: strconv.Itoa(s.Value)}
}

// A11y reports the RangeSlider as a group carrying its "low..high" band --
// two cooperating handles read more naturally as one control's range value
// than as two independent sliders.
func (s *RangeSlider) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: formatFloat(s.Low) + ".." + formatFloat(s.High)}
}

// A11y reports the DropDown as a combobox named by its currently-selected
// option.
func (d *DropDown) A11y() A11yInfo { return A11yInfo{Role: RoleCombobox, Name: d.Current()} }

// A11y reports the Rating as a slider carrying its "value/max" score.
func (r *Rating) A11y() A11yInfo {
	return A11yInfo{Role: RoleSlider, Value: strconv.Itoa(r.Value) + "/" + strconv.Itoa(r.Max)}
}

// A11y reports the ColorChooser as a group carrying its current colour as
// a "#RRGGBB" hex string.
func (c *ColorChooser) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: hexColor(c.Color)}
}

// A11y reports the ColorPicker as a group carrying its current colour
// (derived from the HSV+alpha state) as a "#RRGGBB" hex string.
func (c *ColorPicker) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: hexColor(c.Color())}
}

// A11y reports the FontChooser as a combobox named by its currently-selected
// font option.
func (f *FontChooser) A11y() A11yInfo {
	name := ""
	if f.Selected >= 0 && f.Selected < len(f.Options) {
		name = f.Options[f.Selected].Name
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
	} else if l.Selected >= 0 && l.Selected < len(l.Items) {
		v = l.Items[l.Selected]
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
	} else if t.Selected >= 0 && t.Selected < len(t.Rows) {
		v = "row " + strconv.Itoa(t.Selected+1) + " selected"
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
	} else if t.Selected != nil {
		v = t.Selected.Label
	}
	return A11yInfo{Role: RoleTree, Value: v}
}

// A11y reports the ViewSwitcher as a tablist named by its current view.
func (v *ViewSwitcher) A11y() A11yInfo {
	name := ""
	if v.Current >= 0 && v.Current < len(v.Views) {
		name = v.Views[v.Current]
	}
	return A11yInfo{Role: RoleTablist, Name: name}
}

// A11y reports the Notebook as a tablist named by its active tab.
func (n *Notebook) A11y() A11yInfo {
	name := ""
	if n.Active >= 0 && n.Active < len(n.Tabs) {
		name = n.Tabs[n.Active].Label
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
	return A11yInfo{Role: RoleNavigation, Value: strconv.Itoa(p.Current) + "/" + strconv.Itoa(p.Total)}
}

// A11y reports the Steps strip as a group carrying its current step's label.
func (s *Steps) A11y() A11yInfo {
	v := ""
	if s.Current >= 0 && s.Current < len(s.Labels) {
		v = s.Labels[s.Current]
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the Menu as a menu carrying the hovered row's label, if any.
func (m *Menu) A11y() A11yInfo {
	v := ""
	if m.Hover >= 0 && m.Hover < len(m.Items) {
		v = m.Items[m.Hover].Label
	}
	return A11yInfo{Role: RoleMenu, Value: v}
}

// A11y reports the MenuBar as a menubar carrying the currently-open top-level
// menu's name, if any.
func (m *MenuBar) A11y() A11yInfo {
	v := ""
	if m.Active >= 0 && m.Active < len(m.Names) {
		v = m.Names[m.Active]
	}
	return A11yInfo{Role: RoleMenuBar, Value: v}
}

// A11y reports the ContextMenu as a menu carrying its open/closed state.
func (c *ContextMenu) A11y() A11yInfo {
	return A11yInfo{Role: RoleMenu, Value: stateValue(c.Open, "open")}
}

// A11y reports the CommandPalette as a dialog carrying its typed query.
func (c *CommandPalette) A11y() A11yInfo { return A11yInfo{Role: RoleDialog, Value: c.Query} }

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
// whole-number percentage.
func (p *ProgressBar) A11y() A11yInfo {
	return A11yInfo{Role: RoleProgressbar, Value: percent(p.Fraction)}
}

// A11y reports the LevelBar as a meter carrying its "value/max" reading.
func (l *LevelBar) A11y() A11yInfo {
	return A11yInfo{Role: RoleMeter, Value: strconv.Itoa(l.Value) + "/" + strconv.Itoa(l.Max)}
}

// A11y reports the ProgressCircle as a progressbar carrying its fraction as
// a whole-number percentage.
func (p *ProgressCircle) A11y() A11yInfo {
	return A11yInfo{Role: RoleProgressbar, Value: percent(p.Fraction)}
}

// A11y reports the Spinner as a status region carrying "busy" while active.
func (s *Spinner) A11y() A11yInfo {
	return A11yInfo{Role: RoleStatus, Value: stateValue(s.Active, "busy")}
}

// A11y reports the Stat as a group named by its title, carrying its
// headline value.
func (s *Stat) A11y() A11yInfo { return A11yInfo{Role: RoleGroup, Name: s.Title, Value: s.Value} }

// A11y reports the SegmentedBar as a group. The individual segments carry
// no independent accessible identity (BarSegment is a plain data struct,
// not a Widget), so the bar as a whole is the accessible unit.
func (s *SegmentedBar) A11y() A11yInfo { return A11yInfo{Role: RoleGroup} }

// --- Display ---------------------------------------------------

// A11y reports the Image as an img. Image carries no alt-text field of its
// own, so Name is left for the host to fill in from context.
func (i *Image) A11y() A11yInfo { return A11yInfo{Role: RoleImg} }

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

// A11y reports the Statusbar as a status region carrying its segments
// joined into one string.
func (s *Statusbar) A11y() A11yInfo {
	return A11yInfo{Role: RoleStatus, Value: strings.Join(s.Segments, " | ")}
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
	return A11yInfo{Role: RoleGroup, Name: e.Label, Value: stateValue(e.Expanded, "expanded")}
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
	return A11yInfo{Role: RoleGroup, Value: strconv.Itoa(c.Current+1) + "/" + strconv.Itoa(len(c.Slides))}
}

// A11y reports the Wizard as a group carrying its current step's title.
func (w *Wizard) A11y() A11yInfo {
	v := ""
	if w.Current >= 0 && w.Current < len(w.Steps) {
		v = w.Steps[w.Current].Title
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the FormField as a group named by its label, carrying its
// error text (if any) as Value.
func (f *FormField) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: f.Label, Value: f.Error}
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
	return A11yInfo{Role: RoleGrid, Value: isoDate(c.Year, c.Month, c.Day)}
}

// A11y reports the DatePicker as a group carrying its selected date.
func (d *DatePicker) A11y() A11yInfo {
	v := ""
	if d.Cal != nil {
		v = isoDate(d.Cal.Year, d.Cal.Month, d.Cal.Day)
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the DateRangePicker as a group carrying its "start..end"
// date range.
func (d *DateRangePicker) A11y() A11yInfo {
	return A11yInfo{
		Role:  RoleGroup,
		Value: isoDate(d.Start.Y, d.Start.M, d.Start.D) + ".." + isoDate(d.End.Y, d.End.M, d.End.D),
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
	_ Accessible = (*ActionRow)(nil)
	_ Accessible = (*Toolbar)(nil)
)

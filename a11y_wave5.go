// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strconv"
	"strings"
)

// Wave-5 accessibility metadata. These widgets shipped without an A11y()
// method, so a screen reader saw nothing where they rendered; this file closes
// that gap using the roles, A11yInfo shape and Accessible/CollectA11y machinery
// defined in a11y.go (and the shared helpers in a11y_more.go). Every role used
// here already exists -- no new Role constant is introduced.

// --- Interactive widgets ---------------------------------------------------

// A11y reports the ComboBox as a combobox named by its current field text
// (either free text typed or a picked option).
func (c *ComboBox) A11y() A11yInfo { return A11yInfo{Role: RoleCombobox, Name: c.Text} }

// A11y reports the TreeTable as a tree carrying the selected node's first-column
// label (the column that carries the tree structure), or "" with no selection.
func (t *TreeTable) A11y() A11yInfo {
	v := ""
	if t.Selected != nil && len(t.Selected.Cells) > 0 {
		v = t.Selected.Cells[0]
	}
	return A11yInfo{Role: RoleTree, Value: v}
}

// A11y reports the TagField as a group carrying its tag labels joined into one
// string -- the tokens are plain strings, not independent widgets, so the field
// as a whole is the accessible unit.
func (t *TagField) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: strings.Join(t.Tags, ", ")}
}

// A11y reports the CycleButton as a button named by its currently-shown option
// (the value that advances on each click), or "" when it has no options.
func (c *CycleButton) A11y() A11yInfo {
	name := ""
	if c.Index >= 0 && c.Index < len(c.Options) {
		name = c.Options[c.Index]
	}
	return A11yInfo{Role: RoleButton, Name: name}
}

// A11y reports the Kanban board as a group carrying the selected card's title,
// or "" when nothing is selected (the -1/-1 sentinel or an out-of-range pair),
// mirroring how Draw collapses a stale selection.
func (k *Kanban) A11y() A11yInfo {
	v := ""
	if k.SelectedCol >= 0 && k.SelectedCol < len(k.Columns) &&
		k.SelectedCard >= 0 && k.SelectedCard < len(k.Columns[k.SelectedCol].Cards) {
		v = k.Columns[k.SelectedCol].Cards[k.SelectedCard].Title
	}
	return A11yInfo{Role: RoleGroup, Value: v}
}

// A11y reports the TimePicker as a group carrying its selected time as a
// 24-hour "HH:MM" string (the picker always stores 24-hour internally).
func (t *TimePicker) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Value: fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)}
}

// A11y reports the PropertyGrid as a grid carrying the selected property's name,
// or "" with no selection (or before the backing table is built).
func (pg *PropertyGrid) A11y() A11yInfo {
	v := ""
	if pg.table != nil {
		if s := pg.table.Selected; s >= 0 && s < len(pg.names) {
			v = pg.names[s]
		}
	}
	return A11yInfo{Role: RoleGrid, Value: v}
}

// A11y reports the MarkdownEditor as a textbox carrying its editable source
// text (Text() yields "" when the source pane is nil).
func (m *MarkdownEditor) A11y() A11yInfo {
	return A11yInfo{Role: RoleTextbox, Value: m.Text()}
}

// A11y reports the TerminalView as a textbox carrying its visible cell text,
// rows joined with newlines and trailing blank cells/rows trimmed.
func (t *TerminalView) A11y() A11yInfo {
	return A11yInfo{Role: RoleTextbox, Value: terminalText(t)}
}

// terminalText flattens a TerminalView's cell grid into plain text: unset cells
// (rune 0) become spaces, each row is right-trimmed, and wholly-blank trailing
// rows are dropped. A zero-value or under-allocated grid yields "".
func terminalText(t *TerminalView) string {
	if t.Cols <= 0 || t.Rows <= 0 || len(t.Cells) < t.Cols*t.Rows {
		return ""
	}
	lines := make([]string, t.Rows)
	for row := 0; row < t.Rows; row++ {
		var b strings.Builder
		for col := 0; col < t.Cols; col++ {
			ru := t.Cells[row*t.Cols+col].Rune
			if ru == 0 {
				ru = ' '
			}
			b.WriteRune(ru)
		}
		lines[row] = strings.TrimRight(b.String(), " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// A11y reports the Agenda as a grid carrying its focused period as a
// "YYYY-MM" string (the month the calendar views centre on), or "" when no
// period can be resolved from the fields or events.
func (a *Agenda) A11y() A11yInfo {
	v := ""
	if y, m, ok := a.focusYM(); ok {
		v = fmt.Sprintf("%04d-%02d", y, m)
	}
	return A11yInfo{Role: RoleGrid, Value: v}
}

// --- Charts ---------------------------------------------------

// A11y reports the AreaChart as an img carrying its series count, mirroring the
// LineChart/BarChart/PieChart convention (AreaChart plots one band per series).
func (c *AreaChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(c.Series)) + " series"}
}

// A11y reports the ScatterChart as an img carrying its series count.
func (c *ScatterChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(c.Series)) + " series"}
}

// A11y reports the RadarChart as an img carrying its axis count -- the salient
// dimension of a radar plot.
func (c *RadarChart) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(c.Axes)) + " axes"}
}

// A11y reports the Sparkline as an img carrying its data-point count (it plots a
// single series, like LineChart).
func (s *Sparkline) A11y() A11yInfo {
	return A11yInfo{Role: RoleImg, Value: strconv.Itoa(len(s.Values)) + " points"}
}

// A11y reports the Gauge as a meter carrying its current value both as a Value
// string and as the numeric Min/Max/Now range triple.
func (g *Gauge) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleMeter,
		Value:    formatFloat(g.Value),
		HasRange: true,
		Min:      g.Min,
		Max:      g.Max,
		Now:      g.Value,
	}
}

// Compile-time checks that each widget added in this file satisfies
// Accessible, mirroring the blocks at the bottom of a11y.go and a11y_more.go.
var (
	_ Accessible = (*ComboBox)(nil)
	_ Accessible = (*TreeTable)(nil)
	_ Accessible = (*TagField)(nil)
	_ Accessible = (*CycleButton)(nil)
	_ Accessible = (*Kanban)(nil)
	_ Accessible = (*TimePicker)(nil)
	_ Accessible = (*PropertyGrid)(nil)
	_ Accessible = (*MarkdownEditor)(nil)
	_ Accessible = (*TerminalView)(nil)
	_ Accessible = (*Agenda)(nil)
	_ Accessible = (*AreaChart)(nil)
	_ Accessible = (*ScatterChart)(nil)
	_ Accessible = (*RadarChart)(nil)
	_ Accessible = (*Sparkline)(nil)
	_ Accessible = (*Gauge)(nil)
)

// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

// TestWidgetsStayWithinBounds is a SYSTEMATIC bounds-containment sweep: every
// "framed"/leaf/container widget (the ones that must render inside their box)
// is drawn into a padded surface and must not paint a single pixel outside its
// Bounds(). This is the guard the weaker "something painted" tests lacked — it
// caught the Notebook fixed-80px-tab overflow and the Calendar left-border
// erasure. Widgets that INTENTIONALLY overflow their bounds (popovers, menus,
// tooltips, toasts/notifications that self-anchor, dialogs that centre, open
// dropdowns/date-pickers) are deliberately excluded.
func TestWidgetsStayWithinBounds(t *testing.T) {
	r := Rect{X: 30, Y: 30, W: 240, H: 150}
	const w, h = 320, 230 // r + 50px pad on right/bottom, 30 on top/left

	cases := []struct {
		name string
		make func() Widget
	}{
		{"button", func() Widget { return NewButton("Click me", nil) }},
		{"togglebutton", func() Widget { return NewToggleButton("Toggle", true) }},
		{"checkbutton", func() Widget { return NewCheckButton("Enable", true) }},
		{"radiobutton", func() Widget { rb := NewRadioButton("Opt"); rb.Checked().Set(true); return rb }},
		{"switch", func() Widget { return NewSwitch(true) }},
		{"splitbutton", func() Widget { return NewSplitButton("Deploy", nil) }},
		{"label", func() Widget { return NewLabel("Label text") }},
		{"kbd", func() Widget { return NewKbd("Ctrl+K") }},
		{"badge", func() Widget { return NewBadge("42") }},
		{"chip", func() Widget { return NewChip("frontend") }},
		{"avatar", func() Widget { return NewAvatar("DL") }},
		{"entry", func() Widget { return NewEntry("editable text") }},
		{"textview", func() Widget { return NewTextView("line one\nline two\nline three") }},
		{"spinbutton", func() Widget { return NewSpinButton(0, 100, 42, 1) }},
		{"scale", func() Widget { return NewScale(0, 100, 50) }},
		{"progressbar", func() Widget { p := NewProgressBar(); p.Fraction = 0.6; return p }},
		{"levelbar", func() Widget { l := NewLevelBar(10); l.Value = 7; return l }},
		{"progresscircle", func() Widget { pc := NewProgressCircle(); pc.Fraction().Set(0.6); return pc }},
		{"spinner", func() Widget { s := NewSpinner(); s.Active().Set(true); return s }},
		{"listbox", func() Widget { return NewListBox([]string{"a", "b", "c", "d", "e", "f", "g", "h"}) }},
		{"treeview", func() Widget {
			return NewTreeView(&TreeNode{Label: "/", Expanded: true, Children: []*TreeNode{{Label: "src"}, {Label: "docs"}}})
		}},
		{"table", func() Widget {
			return NewTable([]TableColumn{{Title: "Name", Width: 120}, {Title: "Size"}}, [][]string{{"a", "1"}, {"b", "2"}})
		}},
		{"table-grouped", func() Widget {
			tb := NewTable([]TableColumn{{Title: "G", Width: 90}, {Title: "V"}}, [][]string{{"x", "1"}, {"x", "2"}, {"y", "3"}})
			tb.GroupBy = 0
			return tb
		}},
		{"propertygrid", func() Widget { pg := NewPropertyGrid(); pg.Add("W", "1024"); pg.Add("H", "768"); return pg }},
		{"notebook", func() Widget {
			nb := NewNotebook()
			nb.AddTab("Line", NewLineChart([]float64{3, 7, 2, 8, 5}))
			nb.AddTab("Bar", NewBarChart([]float64{4, 7, 2}))
			nb.AddTab("Pie", NewPieChart([]float64{3, 5, 2}))
			nb.AddTab("Docs", NewMarkdownView("# x\n- y"))
			return nb
		}},
		{"frame", func() Widget {
			f := NewFrame(NewLabel("in a frame"))
			f.Title = "Panel"
			f.Collapsible = true
			return f
		}},
		{"card", func() Widget { return NewCard("Title", "Body one.\nBody two.", "footer") }},
		{"expander", func() Widget { e := NewExpander("Details", NewLabel("body")); e.Expanded().Set(true); return e }},
		{"paned", func() Widget { return NewHPaned(NewLabel("L"), NewLabel("R")) }},
		{"calendar", func() Widget { c := NewCalendar(2026, 7, 2); c.SetToday(2026, 7, 2); return c }},
		{"agenda-month", func() Widget {
			a := NewAgenda([]AgendaEvent{{Title: "E", Y: 2026, M: 7, D: 3}})
			a.Year, a.Month = 2026, 7
			a.View().Set(AgendaMonth)
			return a
		}},
		{"linechart", func() Widget { return NewLineChart([]float64{3, 7, 2, 8, 5, 9}) }},
		{"barchart", func() Widget { return NewBarChart([]float64{4, 7, 2, 8, 5}) }},
		{"piechart", func() Widget { return NewPieChart([]float64{3, 5, 2, 4}) }},
		{"areachart", func() Widget { return NewAreaChart([][]float64{{3, 6, 4, 8}, {1, 3, 2, 5}}) }},
		{"scatterchart", func() Widget {
			return NewScatterChart([][]ScatterPoint{{{X: 1, Y: 2}, {X: 3, Y: 5}, {X: 6, Y: 4}}})
		}},
		{"radarchart", func() Widget {
			return NewRadarChart([]string{"A", "B", "C", "D", "E"}, [][]float64{{8, 6, 7, 4, 9}})
		}},
		{"sparkline", func() Widget { s := NewSparkline([]float64{3, 5, 4, 8, 6, 9}); s.ShowLast = true; return s }},
		{"gantt", func() Widget {
			return NewGantt([]GanttTask{{Label: "A", Start: 0, End: 3, Progress: 0.5}, {Label: "B", Start: 2, End: 6}})
		}},
		{"kanban", func() Widget {
			return NewKanban([]KanbanColumn{{Title: "To Do", Cards: []KanbanCard{{Title: "x"}}}, {Title: "Done"}})
		}},
		{"alert", func() Widget { return NewAlert("Saved.", AlertSuccess) }},
		{"banner", func() Widget { return NewBanner("Update available.") }},
		{"timeline", func() Widget {
			return NewTimeline([]TimelineEvent{{Title: "Open"}, {Title: "Merge", Kind: TimelineSuccess}})
		}},
		{"stat", func() Widget { s := NewStat("Reqs", "12,845"); s.Change = "+8%"; s.Trend = StatUp; return s }},
		{"steps", func() Widget { return NewSteps([]string{"Plan", "Build", "Ship"}, 1) }},
		{"pagination", func() Widget { return NewPagination(3, 12) }},
		{"pagingtoolbar", func() Widget { pt := NewPagingToolbar(6, 12); pt.ShowRefresh = true; return pt }},
		{"toolbar", func() Widget {
			return NewToolbar([]ToolbarItem{{Label: "N"}, {Label: "O"}, {Separator: true}, {Label: "S"}})
		}},
		{"statusbar", func() Widget { return NewStatusbar([]string{"Ready", "Ln 42", "UTF-8"}) }},
		{"headerbar", func() Widget { hb := NewHeaderBar("Files"); hb.Subtitle = "~/docs"; return hb }},
		{"viewswitcher", func() Widget { return NewViewSwitcher([]string{"Day", "Week", "Month"}, 0) }},
		{"markdownview", func() Widget { return NewMarkdownView("# Title\n\n- one\n- two\n\nparagraph") }},
		{"loadmask", func() Widget { m := NewLoadMask("Loading…"); m.Active = true; return m }},
		{"material-sidebar", func() Widget {
			m := NewMaterial(MaterialSidebar)
			src := make([]byte, 400*300*4)
			for i := range src {
				src[i] = byte(i)
			}
			m.SetSource(src, 400, 300)
			m.Child = NewLabel("Sidebar")
			return m
		}},
		{"material-hud-nosource", func() Widget { return NewMaterial(MaterialHUD) }},
		{"barchart-dense", func() Widget { // more bars than pixels -> must clip
			v := make([]float64, 300)
			for i := range v {
				v[i] = float64(i%10) + 1
			}
			return NewBarChart(v)
		}},
		{"sparkbar-dense", func() Widget {
			v := make([]float64, 300)
			for i := range v {
				v[i] = float64(i%10) + 1
			}
			s := NewSparkline(v)
			s.Kind = SparkBar
			return s
		}},
		{"colorchooser", func() Widget { return NewColorChooser(RGB(0x0d, 0x94, 0x88)) }},
		{"diff", func() Widget {
			lines := make([]DiffLine, 0, 12)
			for i := 0; i < 12; i++ { // more lines than the 150px box holds
				lines = append(lines, DiffLine{Text: "a fairly long unified-diff line of source text", Kind: DiffKind(i % 3)})
			}
			return NewDiff(lines)
		}},
	}

	for _, c := range cases {
		buf := makeSurface(w, h)
		wg := c.make()
		wg.SetBounds(r)
		wg.Draw(newP(buf, w), DefaultLight())
		minX, minY, maxX, maxY := nbPaintedBBox(buf, w, h)
		if maxX < 0 { // nothing painted at all is suspicious but not an overflow
			continue
		}
		if minX < r.X || minY < r.Y || maxX >= r.X+r.W || maxY >= r.Y+r.H {
			t.Errorf("%s paints outside bounds %+v: painted X[%d..%d] Y[%d..%d]", c.name, r, minX, maxX, minY, maxY)
		}
	}
}

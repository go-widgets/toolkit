// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"

	"github.com/go-widgets/mvvm"
)

// honourProbe is a child that counts the events it receives, so a container's
// test can tell "forwarded" from "dropped".
type honourProbe struct {
	Base
	hits int
}

func (p *honourProbe) OnEvent(Event) { p.hits++ }

// honourCase is one widget under test: the events that move it when enabled,
// the observable state they would change, and a count of callbacks fired.
type honourCase struct {
	w      Widget
	off    *mvvm.Observable[bool]
	state  func() string
	events []Event
	fired  func() int
}

func centre(r Rect) (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

func honourCases() map[string]func() honourCase {
	click := func(x, y int) Event { return Event{Kind: EventClick, X: x, Y: y} }
	return map[string]func() honourCase{
		// Part 3.
		"Border": func() honourCase {
			p := &honourProbe{}
			b := NewBorder()
			b.Center = p
			b.SetBounds(Rect{W: 100, H: 100})
			return honourCase{w: b, off: b.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(5, 5)},
				fired:  func() int { return p.hits }}
		},
		"Frame": func() honourCase {
			p := &honourProbe{}
			f := NewFrame(p)
			f.SetBounds(Rect{W: 100, H: 100})
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(50, 50)},
				fired:  func() int { return p.hits }}
		},
		"FontChooser": func() honourCase {
			n := 0
			fc := NewFontChooser([]FontOption{{Name: "a", Font: NewBitmapFont(1)}, {Name: "b", Font: NewBitmapFont(2)}})
			fc.OnChoose = func(int, Font) { n++ }
			fc.SetBounds(Rect{W: 120, H: 200})
			y := FontChooserPad + fc.rowHeight(0) + FontChooserRowPad
			return honourCase{w: fc, off: fc.Disabled(),
				state:  func() string { return fmt.Sprint(fc.Selected().Get()) },
				events: []Event{click(10, y)},
				fired:  func() int { return n }}
		},
		"FontPicker": func() honourCase {
			f := NewFontPicker([]string{"a", "b", "c"})
			f.SetBounds(Rect{W: 300, H: 24})
			x, y := centre(f.family.Bounds())
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return fmt.Sprint(f.family.Open().Get()) },
				events: []Event{click(x, y)},
				fired:  func() int { return 0 }}
		},
		"ForgePane": func() honourCase {
			m := newFakeForge()
			f := laidOutForge(m, 600, 600)
			x, y := centre(f.client.connect.Bounds())
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y)},
				fired:  func() int { return m.ran["connect"] }}
		},
		"Inspector": func() honourCase {
			in, got := testInspector()
			r, _ := in.EditorRect("name")
			x, y := centre(r)
			return honourCase{w: in, off: in.Disabled(),
				state:  func() string { return in.Get("name") },
				events: []Event{click(x, y), {Kind: EventChar, Code: "z"}},
				fired:  func() int { return len(*got) }}
		},
		"Kanban": func() honourCase {
			var n int
			k := sampleBoard()
			k.SetBounds(Rect{W: 600, H: 300})
			k.OnCardClick = func(int, int) { n++ }
			colW := (600 - 2*KanbanColGap) / 3
			from, to := k.cardLocalRect(0, 0, colW), k.cardLocalRect(2, 0, colW)
			return honourCase{w: k, off: k.Disabled(),
				state: func() string {
					return fmt.Sprint(k.SelectedCol().Get(), k.SelectedCard().Get(), len(k.Columns[2].Cards))
				},
				events: []Event{click(from.X+4, from.Y+4),
					{Kind: EventMouseDrag, X: to.X + 4, Y: to.Y + 4},
					{Kind: EventMouseUp, X: to.X + 4, Y: to.Y + 4}},
				fired: func() int { return n }}
		},
		"LogView": func() honourCase {
			lv := NewLogView()
			lv.SetBounds(Rect{W: 200, H: 40})
			for i := range 30 {
				lv.Append("t", LogInfo, fmt.Sprint("line ", i))
			}
			return honourCase{w: lv, off: lv.Disabled(),
				state:  func() string { return fmt.Sprint(lv.sv.OffsetY().Get()) },
				events: []Event{{Kind: EventScroll, Delta: -3}},
				fired:  func() int { return 0 }}
		},
		"MarkdownEditor": func() honourCase {
			m := NewMarkdownEditor("hello")
			m.SetBounds(Rect{W: 100, H: 60})
			return honourCase{w: m, off: m.Disabled(),
				state:  func() string { return fmt.Sprint(m.Text(), m.Source.HasFocus().Get()) },
				events: []Event{click(10, 10), {Kind: EventChar, Code: "x"}},
				fired:  func() int { return 0 }}
		},
		"PropertyGrid": func() honourCase {
			n := 0
			pg := NewPropertyGrid()
			pg.OnChange = func(string, string) { n++ }
			pg.Add("a", "1")
			pg.SetBounds(Rect{W: 200, H: 100})
			return honourCase{w: pg, off: pg.Disabled(),
				state: func() string { return pg.Value("a") },
				events: []Event{click(150, TableHeaderHeight+2),
					{Kind: EventChar, Code: "!"}, {Kind: EventKeyDown, Code: "Enter"}},
				fired: func() int { return n }}
		},
		"RichEditorToolbar": func() honourCase {
			n := 0
			t := NewRichEditorToolbar(nil)
			orig := t.buttons[0].OnClick
			t.buttons[0].OnClick = func() { n++; orig() }
			t.SetBounds(Rect{W: 400, H: 28})
			x, y := centre(t.buttons[0].Bounds())
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"Splitter": func() honourCase {
			s := honourSplitter()
			hr, _ := s.HandleRect(0)
			x, y := centre(hr)
			return honourCase{w: s, off: s.Disabled(),
				state: func() string { return fmt.Sprint(s.PaneSize(0), s.Dragging().Get()) },
				events: []Event{click(x, y), {Kind: EventMouseDrag, X: x + 20, Y: y},
					{Kind: EventMouseUp, X: x + 20, Y: y}},
				fired: func() int { return 0 }}
		},
		"TagField": func() honourCase {
			t := NewTagField()
			t.SetBounds(Rect{W: 200, H: 24})
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return fmt.Sprint(t.Tags().Get(), t.Text().Get()) },
				events: []Event{{Kind: EventChar, Code: "a"}, {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return 0 }}
		},
		"TerminalView": func() honourCase {
			n := 0
			v := NewTerminalView(4, 2)
			v.OnKey = func(Event) { n++ }
			return honourCase{w: v, off: v.Disabled(),
				state:  func() string { return "" },
				events: []Event{{Kind: EventKeyDown, Code: "k"}},
				fired:  func() int { return n }}
		},
		"TimePicker": func() honourCase {
			tp := NewTimePicker(9, 30)
			tp.SetBounds(Rect{W: 200, H: 30})
			hourCell, _, _, _ := tp.layout()
			up, _ := spinButtons(hourCell)
			x, y := centre(up)
			return honourCase{w: tp, off: tp.Disabled(),
				state:  func() string { return tp.String() },
				events: []Event{click(x, y)},
				fired:  func() int { return 0 }}
		},
		"Wizard": func() honourCase {
			w := NewWizard([]WizardStep{{Title: "a"}, {Title: "b"}})
			w.SetBounds(Rect{W: 300, H: 200})
			x, y := centre(w.nextRect())
			return honourCase{w: w, off: w.Disabled(),
				state:  func() string { return fmt.Sprint(w.Current().Get(), w.pressedBtn) },
				events: []Event{click(x, y)},
				fired:  func() int { return 0 }}
		},
		// Part 2.
		"ActionSheet": func() honourCase {
			a := NewActionSheet("t")
			a.AddAction("go", nil)
			a.SetBounds(Rect{W: 300, H: 400})
			a.Open()
			return honourCase{w: a, off: a.Disabled(),
				state:  func() string { return fmt.Sprint(a.State()) },
				events: []Event{click(1, 1), {Kind: EventKeyDown, Code: "Escape"}},
				fired:  func() int { return 0 }}
		},
		"Agenda": func() honourCase {
			n := 0
			a := agendaMonthFixture()
			a.OnDayActivate = func(int, int, int) { n++ }
			x, y := dayCellCenter(a, 10)
			return honourCase{w: a, off: a.Disabled(),
				state:  func() string { return fmt.Sprint(a.Selected().Get()) },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"AgendaSidebar": func() honourCase {
			n := 0
			s := NewAgendaSidebar([]AgendaCalendar{{Name: "Work"}, {Name: "Home"}})
			s.OnToggle = func(int) { n++ }
			s.SetBounds(Rect{W: 160, H: 120})
			r := s.rowRect(1)
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return fmt.Sprint(s.Calendars[1].Hidden) },
				events: []Event{click(10, r.Y+r.H/2)},
				fired:  func() int { return n }}
		},
		"ClosablePane": func() honourCase {
			n := 0
			p := NewClosablePane("t", &honourProbe{})
			p.Button.OnClick = func() { n++ }
			p.SetBounds(Rect{W: 200, H: 100})
			x, y := centre(p.Button.Bounds())
			return honourCase{w: p, off: p.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"ColorChooser": func() honourCase {
			c := NewColorChooser(RGB(10, 20, 30))
			c.SetBounds(Rect{W: 200, H: 100})
			return honourCase{w: c, off: c.Disabled(),
				state:  func() string { return fmt.Sprint(c.Color().Get()) },
				events: []Event{click(150, scaled(ColorChooserChannelPadY)+3), {Kind: EventMouseDrag, X: 60, Y: 5}},
				fired:  func() int { return 0 }}
		},
		"ColorPicker": func() honourCase {
			n := 0
			c := NewColorPicker(RGB(10, 20, 30))
			c.OnEyedrop = func() { n++ }
			sx, sy := centre(c.svRectLocal())
			ex, ey := centre(c.eyedropRectLocal())
			return honourCase{w: c, off: c.Disabled(),
				state:  func() string { return fmt.Sprint(c.Color().Get()) },
				events: []Event{click(sx, sy), click(ex, ey)},
				fired:  func() int { return n }}
		},
		"CommandPalette": func() honourCase {
			n := 0
			c := NewCommandPalette([]PaletteCommand{{Label: "a", Action: func() { n++ }}})
			c.Visible().Set(true)
			return honourCase{w: c, off: c.Disabled(),
				state:  func() string { return c.A11y().Value },
				events: []Event{{Kind: EventChar, Code: "a"}, {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return n }}
		},
		"ContextMenu": func() honourCase {
			c := NewContextMenu(NewMenu([]MenuItem{{Label: "a"}}))
			c.SetBounds(Rect{W: 300, H: 300})
			c.Popup(10, 10)
			return honourCase{w: c, off: c.Disabled(),
				state:  func() string { return fmt.Sprint(c.Open().Get()) },
				events: []Event{click(290, 290)},
				fired:  func() int { return 0 }}
		},
		"DatabaseEditor": func() honourCase {
			src := newCannedSource()
			d := NewDatabaseEditor(src)
			d.SetBounds(Rect{W: 600, H: 400})
			d.SetSQL("select 1")
			tb := d.Toolbar().Bounds()
			return honourCase{w: d, off: d.Disabled(),
				state:  func() string { return src.lastQuery },
				events: []Event{click(tb.X+5, tb.Y+tb.H/2)},
				fired:  func() int { return 0 }}
		},
		"DatePicker": func() honourCase {
			d := NewDatePicker(2026, 8, 16)
			d.SetBounds(Rect{W: 120, H: 24})
			return honourCase{w: d, off: d.Disabled(),
				state:  func() string { return fmt.Sprint(d.Open().Get()) },
				events: []Event{click(5, 5)},
				fired:  func() int { return 0 }}
		},
		"DateRangePicker": func() honourCase {
			d := NewDateRangePicker(2026, 8)
			d.SetBounds(Rect{W: 300, H: 200})
			return honourCase{w: d, off: d.Disabled(),
				state:  func() string { return fmt.Sprint(d.Cal.Year().Get(), d.Cal.Month().Get()) },
				events: []Event{click(1, 1)},
				fired:  func() int { return 0 }}
		},
		"DiffView": func() honourCase {
			old, nw := "", ""
			for i := range 100 {
				old += fmt.Sprintf("line %d\n", i)
				if i%30 == 15 {
					nw += fmt.Sprintf("changed %d\n", i)
				} else {
					nw += fmt.Sprintf("line %d\n", i)
				}
			}
			v := NewDiffView(old, nw)
			v.SetBounds(Rect{W: 400, H: 120})
			return honourCase{w: v, off: v.Disabled(),
				state:  func() string { return fmt.Sprint(v.top, v.current, v.Focused()) },
				events: []Event{{Kind: EventScroll, Delta: 3}, {Kind: EventKeyDown, Code: "n"}, click(5, 5)},
				fired:  func() int { return 0 }}
		},
		"Dock": func() honourCase {
			p := &honourProbe{}
			d := NewDock(p)
			d.SetBounds(Rect{W: 100, H: 100})
			return honourCase{w: d, off: d.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(5, 5)},
				fired:  func() int { return p.hits }}
		},
		"DropZone": func() honourCase {
			n := 0
			d := NewDropZone("p")
			d.OnDrop = func([]string) { n++ }
			return honourCase{w: d, off: d.Disabled(),
				state:  func() string { return fmt.Sprint(d.Hover().Get()) },
				events: []Event{{Kind: EventDragStart}, {Kind: EventDrop, Code: "a"}, {Kind: EventDragStart}},
				fired:  func() int { return n }}
		},
		"FileChooser": func() honourCase {
			n := 0
			f := NewFileChooser(&TreeNode{Label: "r"}, nil)
			f.OnAccept = func(string) { n++ }
			f.SetBounds(Rect{W: 400, H: 300})
			x, y := centre(f.openButton.Bounds())
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"FindReplace": func() honourCase {
			n := 0
			f := NewFindReplace()
			f.OnNext = func() { n++ }
			f.SetBounds(Rect{W: 600, H: 100})
			f.Open()
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return f.Query().Get() },
				events: []Event{{Kind: EventChar, Code: "x"}, {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return n }}
		},
		// Part 1.
		"AddressBar": func() honourCase {
			a := &AddressBar{}
			a.SetBounds(Rect{W: 200, H: 24})
			a.URL().Set("u")
			return honourCase{w: a, off: a.Disabled(),
				state:  func() string { return fmt.Sprint(a.HasFocus().Get(), a.Editing().Get(), a.URL().Get()) },
				events: []Event{click(10, 10), {Kind: EventChar, Code: "x"}},
				fired:  func() int { return 0 }}
		},
		"SearchEntry": func() honourCase {
			s := NewSearchEntry("ab")
			s.SetBounds(Rect{W: 200, H: 24})
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return s.Text().Get() },
				events: []Event{{Kind: EventChar, Code: "x"}, {Kind: EventKeyDown, Code: "ArrowLeft"}, {Kind: EventKeyDown, Code: "Backspace"}},
				fired:  func() int { return 0 }}
		},
		"Chip": func() honourCase {
			n := 0
			c := NewChip("c")
			c.Closable = true
			c.OnClose = func() { n++ }
			c.SetBounds(Rect{W: 80, H: 20})
			return honourCase{w: c, off: c.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(80-scaled(ChipPadX)-1, 5)},
				fired:  func() int { return n }}
		},
		"Banner": func() honourCase {
			n := 0
			b := NewBanner("t")
			b.ButtonLabel = "Go"
			b.OnAction = func() { n++ }
			b.SetBounds(Rect{W: 300, H: 32})
			br, _ := b.buttonRect()
			x, y := centre(br)
			return honourCase{w: b, off: b.Disabled(),
				state:  func() string { return fmt.Sprint(b.Revealed().Get()) },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"Toast": func() honourCase {
			n := 0
			t := NewToast("t", ToastInfo)
			t.Actions = []ToastAction{{Label: "ok", Callback: func() { n++ }}}
			t.Visible().Set(true)
			t.SetBounds(Rect{W: 200, H: 24})
			x, y := centre(t.ButtonRects()[0])
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return fmt.Sprint(t.Visible().Get()) },
				events: []Event{click(x, y)},
				fired:  func() int { return n }}
		},
		"Steps": func() honourCase {
			s := NewSteps([]string{"a", "b", "c"}, 0)
			s.SetBounds(Rect{W: 300, H: 40})
			x := 2*(scaled(StepBoxW)+scaled(StepConnectorW)) + 1
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return fmt.Sprint(s.Current().Get()) },
				events: []Event{click(x, (40-scaled(StepBoxH))/2+1)},
				fired:  func() int { return 0 }}
		},
		"Thumbnail": func() honourCase {
			n := 0
			t := NewThumbnail(make([]byte, 16), 2, 2)
			t.OnClick = func() { n++ }
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(1, 1)},
				fired:  func() int { return n }}
		},
		"StatusIcon": func() honourCase {
			n := 0
			s := NewStatusIcon(nil)
			s.OnClick = func() { n++ }
			s.OnRightClick = func() { n += 10 }
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(1, 1), {Kind: EventClick, Code: StatusIconSecondary}},
				fired:  func() int { return n }}
		},
		"ActionRow": func() honourCase {
			p := &honourProbe{}
			a := NewActionRow("t")
			a.Prefix = p
			a.SetBounds(Rect{W: 300, H: 40})
			return honourCase{w: a, off: a.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(1, 1)},
				fired:  func() int { return p.hits }}
		},
		"SettingsGroup": func() honourCase {
			p := &honourProbe{}
			row := NewSettingRow("r", p)
			g := NewSettingsGroup("g", row)
			g.SetBounds(Rect{W: 300, H: 100})
			x, y := centre(row.controlRect())
			return honourCase{w: g, off: g.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y)},
				fired:  func() int { return p.hits }}
		},
		"SettingRow": func() honourCase {
			p := &honourProbe{}
			p.SetBounds(Rect{W: 50, H: 20})
			s := NewSettingRow("r", p)
			s.SetBounds(Rect{W: 300, H: 40})
			x, y := centre(s.controlRect())
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y), {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return p.hits }}
		},
		"Toolbar": func() honourCase {
			n := 0
			t := NewToolbar([]ToolbarItem{{Label: "a", OnClick: func() { n++ }}})
			t.SetBounds(Rect{W: 100, H: 24})
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return fmt.Sprint(t.pressIdx) },
				events: []Event{click(5, 5)},
				fired:  func() int { return n }}
		},
		"Breadcrumbs": func() honourCase {
			n := 0
			b := NewBreadcrumbs([]string{"a", "b"})
			b.OnSelect = func(int) { n++ }
			return honourCase{w: b, off: b.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(0, 1)},
				fired:  func() int { return n }}
		},
		"ButtonGroup": func() honourCase {
			n := 0
			b1, b2 := &Button{}, &Button{}
			b1.OnClick = func() { n++ }
			b2.OnClick = func() { n++ }
			g := NewButtonGroup(b1, b2)
			g.SetBounds(Rect{W: 200, H: 30})
			return honourCase{w: g, off: g.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(10, 10)},
				fired:  func() int { return n }}
		},
		"PagingToolbar": func() honourCase {
			n := 0
			t := NewPagingToolbar(2, 5)
			t.ShowRefresh = true
			t.OnRefresh = func() { n++ }
			t.SetBounds(Rect{W: 600, H: 24})
			l := t.layout()
			nx, ny := centre(l.next)
			rx, ry := centre(l.refresh)
			return honourCase{w: t, off: t.Disabled(),
				state:  func() string { return fmt.Sprint(t.Page().Get()) },
				events: []Event{click(nx, ny), click(rx, ry)},
				fired:  func() int { return n }}
		},
		"FormField": func() honourCase {
			p := &honourProbe{}
			p.SetBounds(Rect{W: 50, H: 20})
			f := NewFormField("f", p)
			f.SetBounds(Rect{W: 300, H: 60})
			x, y := centre(f.childRect())
			return honourCase{w: f, off: f.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(x, y), {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return p.hits }}
		},
	}
}

// TestDisabledWidgetsHonourTheFlag: a disabled widget's events leave its state
// unchanged and fire no callback; re-enabling restores it; A11y follows (the
// A11y half is audited by TestA11yDisabledAudit through the catalogue).
func TestDisabledWidgetsHonourTheFlag(t *testing.T) {
	defer SetFont(nil) // the FontChooser row installs the active font
	for name, build := range honourCases() {
		t.Run(name, func(t *testing.T) {
			c := build()
			before := c.state()
			c.off.Set(true)
			for _, ev := range c.events {
				c.w.OnEvent(ev)
			}
			if got := c.state(); got != before {
				t.Errorf("disabled: state %q, want unchanged %q", got, before)
			}
			if n := c.fired(); n != 0 {
				t.Errorf("disabled: %d callbacks fired, want 0", n)
			}
			c.off.Set(false)
			for _, ev := range c.events {
				c.w.OnEvent(ev)
			}
			if c.state() == before && c.fired() == 0 {
				t.Errorf("re-enabled: the events changed nothing, so the gate proves nothing")
			}
		})
	}
}

// A disabled container's subtree must not take focus: routeFocusKey would send
// keys straight to the focused child, past the container's own gate.
func TestDisabledContainerSubtreeTakesNoFocus(t *testing.T) {
	e := NewEntry("ab")
	ff := NewFormField("f", e)
	v := NewVBox()
	v.Append(ff)
	v.SetBounds(Rect{W: 300, H: 90})
	ff.Disabled().Set(true)
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	v.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := e.Text().Get(); got != "ab" {
		t.Fatalf("disabled FormField: Entry became %q, want \"ab\"", got)
	}
	ff.Disabled().Set(false)
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	v.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := e.Text().Get(); got != "abz" {
		t.Fatalf("re-enabled FormField: Entry is %q, want \"abz\"", got)
	}
}

// Keyboard path, part 2: routeFocusKey sends keys straight to a focused
// descendant, so a disabled ClosablePane must neither be walked into nor take a
// key directly. The body Entry is the witness.
func TestDisabledClosablePaneTakesNoKeys(t *testing.T) {
	e := NewEntry("ab")
	cp := NewClosablePane("t", e)
	v := NewVBox()
	v.Append(cp)
	v.SetBounds(Rect{W: 300, H: 120})
	type pane = ClosablePane
	press := func(w Widget) {
		w.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		w.OnEvent(Event{Kind: EventChar, Code: "z"})
	}

	cp.Disabled().Set(true)
	press(v)
	if got := e.Text().Get(); got != "ab" {
		t.Fatalf("disabled pane, keys through the walk: Entry became %q, want \"ab\"", got)
	}
	cp.Disabled().Set(false)
	// Tab twice: the first stop is the close button, the second the Entry.
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
	cp.Disabled().Set(true)
	// Focus is already inside; the pane must refuse keys handed to it directly.
	cp.OnEvent(Event{Kind: EventChar, Code: "z"})
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if got := e.Text().Get(); got != "ab" {
		t.Fatalf("disabled pane, keys sent directly: Entry became %q, want \"ab\"", got)
	}
	cp.Disabled().Set(false)
	cp.OnEvent(Event{Kind: EventChar, Code: "z"})
	if got := e.Text().Get(); got != "abz" {
		t.Fatalf("re-enabled pane: Entry is %q, want \"abz\"", got)
	}
}

// A disabled DiffView is skipped by the focus walk, and refuses keys sent to it
// after it already held focus.
func TestDisabledDiffViewTakesNoKeys(t *testing.T) {
	old, nw := "", ""
	for i := range 100 {
		old += fmt.Sprintf("line %d\n", i)
		if i%30 == 15 {
			nw += "changed\n"
		} else {
			nw += fmt.Sprintf("line %d\n", i)
		}
	}
	d := NewDiffView(old, nw)
	v := NewVBox()
	v.Append(d)
	v.SetBounds(Rect{W: 400, H: 120})
	key := func(code string) { v.OnEvent(Event{Kind: EventKeyDown, Code: code}) }

	d.Disabled().Set(true)
	key("Tab")
	key("n")
	if d.Focused() || d.current != -1 {
		t.Fatalf("disabled DiffView took focus %v / moved to change %d", d.Focused(), d.current)
	}
	d.Disabled().Set(false)
	key("Tab")
	key("n")
	if !d.Focused() || d.current != 0 {
		t.Fatalf("enabled DiffView: focus %v, change %d, want true / 0", d.Focused(), d.current)
	}
	d.Disabled().Set(true)
	d.OnEvent(Event{Kind: EventKeyDown, Code: "n"})
	if d.current != 0 {
		t.Fatalf("disabled DiffView, key sent directly: change %d, want 0", d.current)
	}
}

// CommandPalette.HandleKey is a public keyboard entry that does not go through
// OnEvent: the flag must hold there too.
func TestDisabledCommandPaletteHandleKey(t *testing.T) {
	n := 0
	c := NewCommandPalette([]PaletteCommand{{Label: "a", Action: func() { n++ }}})
	c.Visible().Set(true)
	c.Disabled().Set(true)
	c.HandleKey(Event{Kind: EventChar, Code: "a"})
	c.HandleKey(Event{Kind: EventKeyDown, Code: "Enter"})
	if c.A11y().Value != "" || n != 0 {
		t.Fatalf("disabled palette: query %q, %d commands run, want \"\" and 0", c.A11y().Value, n)
	}
	c.Disabled().Set(false)
	c.HandleKey(Event{Kind: EventChar, Code: "a"})
	if c.A11y().Value != "a" {
		t.Fatalf("enabled palette: query %q, want \"a\"", c.A11y().Value)
	}
	c.HandleKey(Event{Kind: EventKeyDown, Code: "Enter"})
	if n != 1 {
		t.Fatalf("enabled palette: %d commands run, want 1", n)
	}
}

// The composites of this batch other than ClosablePane hold widgets but are
// dead ends for the focus walk (no focusableChildren), so a key can reach their
// children only through their own OnEvent, which the flag gates. If one of them
// ever becomes a focus enumerator, this fails and the gate must be re-checked.
func TestBatch2CompositesAreFocusDeadEnds(t *testing.T) {
	for name, w := range map[string]Widget{
		"ActionSheet":     NewActionSheet("t"),
		"DatabaseEditor":  NewDatabaseEditor(newCannedSource()),
		"Dock":            NewDock(nil),
		"FileChooser":     NewFileChooser(&TreeNode{Label: "r"}, nil),
		"FindReplace":     NewFindReplace(),
		"DatePicker":      NewDatePicker(2026, 8, 16),
		"DateRangePicker": NewDateRangePicker(2026, 8),
	} {
		if _, ok := w.(focusEnumerator); ok {
			t.Errorf("%s is a focusEnumerator: keys can reach its children past its gate", name)
		}
	}
}

// popoverCase is a PopoverOwner under a PopoverHost with its popup open.
type popoverCase struct {
	host  *PopoverHost
	owner PopoverOwner
	off   *mvvm.Observable[bool]
	state func() string
	moves bool // a click on the popup changes state when enabled
}

func popoverCases() map[string]func() popoverCase {
	hosted := func(w Widget, h int) *PopoverHost {
		v := NewVBox()
		v.AddFixed(w, h)
		v.AddFlex(NewLabel("content"), 1)
		host := NewPopoverHost(v)
		host.SetBounds(Rect{W: 1100, H: 400})
		v.SetBounds(Rect{W: 1100, H: 400})
		return host
	}
	return map[string]func() popoverCase{
		"DatePicker": func() popoverCase {
			d := NewDatePicker(2026, 8, 16)
			host := hosted(d, 24)
			d.Open().Set(true)
			return popoverCase{host, d, d.Disabled(),
				func() string { return fmt.Sprint(d.Cal.Day().Get(), d.Open().Get()) }, true}
		},
		"ComboBox": func() popoverCase {
			c := NewComboBox([]string{"one", "two", "three"})
			host := hosted(c, 24)
			c.Open().Set(true)
			return popoverCase{host, c, c.Disabled(),
				func() string { return c.Text().Get() + fmt.Sprint(c.Open().Get()) }, true}
		},
		"DropDown": func() popoverCase {
			d := NewDropDown([]string{"one", "two", "three"}, 0)
			host := hosted(d, 24)
			d.Open().Set(true)
			return popoverCase{host, d, d.Disabled(),
				func() string { return fmt.Sprint(d.Selected().Get(), d.Open().Get()) }, true}
		},
		"Ribbon": func() popoverCase {
			run := &sampleRun{}
			r := run.ribbon()
			host := hosted(r, r.Height())
			r.SetBounds(Rect{W: 1100, H: r.Height()})
			r.activate(rtCell(r, "Style"), partMain)
			return popoverCase{host, r, r.Disabled(),
				func() string { return fmt.Sprint(run.ids, r.pop != nil) }, false}
		},
	}
}

// PopoverHost offers a click to an open popup before the tree: a second click
// route that skips OnEvent. A disabled owner must show no popup and consume no
// click on it, even when it was open before it was disabled.
func TestDisabledPopoverOwnersTakeNoPopupClicks(t *testing.T) {
	for name, build := range popoverCases() {
		t.Run(name, func(t *testing.T) {
			c := build()
			pb := c.owner.PopoverBounds()
			x, y := pb.X+pb.W/2, pb.Y+pb.H/2
			if name == "DatePicker" {
				// first column, second week row: a day other than the 16th
				x = pb.X + scaled(CalendarCellW)/2
				y = pb.Y + scaled(CalendarHeaderH) + 2*scaled(CalendarCellH) + scaled(CalendarCellH)/2
			}
			c.off.Set(true)
			before := c.state()
			if n := len(PopoverOwners(c.host)); n != 0 {
				t.Errorf("disabled: %d popovers showing, want 0", n)
			}
			if c.owner.PopoverClick(x, y) {
				t.Errorf("disabled: PopoverClick consumed a click")
			}
			c.host.OnEvent(Event{Kind: EventClick, X: x, Y: y})
			if got := c.state(); got != before {
				t.Errorf("disabled: state %q, want unchanged %q", got, before)
			}
			c.off.Set(false)
			if n := len(PopoverOwners(c.host)); n != 1 {
				t.Fatalf("re-enabled: %d popovers showing, want 1", n)
			}
			c.host.OnEvent(Event{Kind: EventClick, X: x, Y: y})
			if c.moves && c.state() == before {
				t.Errorf("re-enabled: the click changed nothing, so the gate proves nothing")
			}
		})
	}
}

// DropZone.AcceptsDrop is what a host asks before showing a drop as accepted.
func TestDisabledDropZoneRefusesDrops(t *testing.T) {
	d := NewDropZone("p")
	d.Disabled().Set(true)
	if d.AcceptsDrop("a") {
		t.Fatal("disabled DropZone accepts a drop")
	}
	d.Disabled().Set(false)
	if !d.AcceptsDrop("a") || d.AcceptsDrop("") {
		t.Fatal("enabled DropZone: want accept non-empty, refuse empty")
	}
}

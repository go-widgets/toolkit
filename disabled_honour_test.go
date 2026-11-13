// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"testing"

	"github.com/go-richdoc/richdoc"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit/internal/formula"
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
		// Part 4.
		"Dialog": func() honourCase {
			closes, oks := 0, 0
			p := &honourProbe{}
			ok := NewButton("ok", func() { oks++ })
			d := NewDialog("t", p, ok)
			d.Closable = true
			d.OnClose = func() { closes++ }
			d.SetBounds(Rect{W: 300, H: 200})
			bx, by := centre(ok.Bounds())
			cx, cy := centre(d.closeButton().Bounds())
			px, py := centre(p.Bounds())
			return honourCase{w: d, off: d.Disabled(),
				state: func() string { return fmt.Sprint(d.dragOffX, d.dragOffY, d.dragging) },
				// the drag goes last: it moves the panel and every point above with it
				events: []Event{click(px, py), click(bx, by), click(cx, cy),
					click(5, 5), {Kind: EventMouseDrag, X: 25, Y: 15}, {Kind: EventMouseUp, X: 25, Y: 15}},
				fired: func() int { return closes + oks + p.hits }}
		},
		"Gantt": func() honourCase {
			n := 0
			g := NewGantt([]GanttTask{{Label: "A", Start: 0, End: 6}})
			g.Units = 12
			g.OnTaskChange = func(int, int, int) { n++ }
			g.SetBounds(Rect{W: 600, H: GanttHeaderH + 2*GanttRowH})
			y := scaled(GanttHeaderH) + scaled(GanttRowH)/2
			x := (g.barXLocal(0) + g.barXLocal(6)) / 2
			return honourCase{w: g, off: g.Disabled(),
				state: func() string { return fmt.Sprint(g.Selected().Get(), g.Tasks[0], g.editing) },
				events: []Event{click(x, y), {Kind: EventMouseDrag, X: x + 60, Y: y},
					{Kind: EventMouseUp, X: x + 60, Y: y}},
				fired: func() int { return n }}
		},
		"ModalWindow": func() honourCase {
			closes, oks := 0, 0
			p := &honourProbe{}
			ok := NewButton("ok", func() { oks++ })
			m := NewModalWindow("t", p, ok)
			m.OnClose = func() { closes++ }
			m.SetBounds(Rect{W: 800, H: 600})
			bx, by := centre(ok.Bounds())
			px, py := centre(p.Bounds())
			return honourCase{w: m, off: m.Disabled(),
				state: func() string { return fmt.Sprint(m.Panel.dragOffX, m.Panel.dragOffY) },
				events: []Event{{Kind: EventKeyDown, Code: "Escape"}, click(1, 1),
					click(px, py), click(bx, by)},
				fired: func() int { return closes + oks + p.hits }}
		},
		"Paned": func() honourCase {
			moves := 0
			a, b := &honourProbe{}, &honourProbe{}
			p := NewHPaned(a, b)
			p.OnPositionChanged = func(int) { moves++ }
			p.SetBounds(Rect{W: 200, H: 100})
			h := p.Position().Get() + 1
			return honourCase{w: p, off: p.Disabled(),
				state: func() string { return fmt.Sprint(p.Position().Get(), p.resizing) },
				events: []Event{click(h, 5), {Kind: EventMouseDrag, X: 60, Y: 5},
					{Kind: EventMouseUp, X: 60, Y: 5}, click(5, 5)},
				fired: func() int { return moves + a.hits + b.hits }}
		},
		"RichEditor": func() honourCase {
			e := NewRichEditor(richdoc.New().P(richdoc.Txt("hi")).Doc())
			e.SetBounds(Rect{W: 300, H: 200})
			return honourCase{w: e, off: e.Disabled(),
				state: func() string {
					return fmt.Sprint(richdoc.PlainText(e.docValue()), e.Caret().Get(), e.Focused())
				},
				events: []Event{click(1, 1), {Kind: EventChar, Code: "z"}, {Kind: EventKeyDown, Code: "Backspace"}},
				fired:  func() int { return 0 }}
		},
		"Spreadsheet": func() honourCase {
			n := 0
			s := NewSpreadsheet(3, 3)
			s.OnCellChange = func(formula.Ref, string) { n++ }
			s.SetBounds(Rect{W: 400, H: 200})
			x, y := centre(s.cellRect(1, 1))
			return honourCase{w: s, off: s.Disabled(),
				state: func() string {
					c, r := s.Active()
					return fmt.Sprint(c, r, s.Editing(), s.CellRaw(1, 1))
				},
				events: []Event{click(x, y), {Kind: EventChar, Code: "7"}, {Kind: EventKeyDown, Code: "Enter"}},
				fired:  func() int { return n }}
		},
		"Surface": func() honourCase {
			n := 0
			s := NewSurface(nil)
			s.OnInput = func(Event) { n++ }
			s.SetBounds(Rect{W: 50, H: 50})
			return honourCase{w: s, off: s.Disabled(),
				state:  func() string { return "" },
				events: []Event{click(5, 5), {Kind: EventChar, Code: "a"}},
				fired:  func() int { return n }}
		},
		"Window": func() honourCase {
			closes, mins, maxes := 0, 0, 0
			p := &honourProbe{}
			w := NewWindow("t", p)
			w.Closable, w.Minimizable, w.Maximizable = true, true, true
			w.OnClose = func() { closes++ }
			w.OnMinimize = func() { mins++ }
			w.OnMaximize = func() { maxes++ }
			w.SetBounds(Rect{X: 10, Y: 20, W: 200, H: 150})
			tools := w.toolRects()
			ev := func(r Rect) Event {
				x, y := centre(r)
				return click(x-10, y-20)
			}
			bx, by := centre(w.bodyAreaRect())
			return honourCase{w: w, off: w.Disabled(),
				state: func() string { return "" },
				events: []Event{ev(tools[WindowClose]), ev(tools[WindowMinimize]), ev(tools[WindowMaximize]),
					click(bx-10, by-20)},
				fired: func() int { return closes + mins + maxes + p.hits }}
		},
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
			// before any walk (which would close the list): the owner's own gate
			if c.owner.PopoverOpen() {
				t.Errorf("disabled: PopoverOpen is true before any walk")
			}
			if c.owner.PopoverClick(x, y) {
				t.Errorf("disabled: PopoverClick consumed a click before any walk")
			}
			if n := len(PopoverOwners(c.host)); n != 0 {
				t.Errorf("disabled: %d popovers showing, want 0", n)
			}
			// the walk closes a list that was open (see closePopups)
			before := c.state()
			if c.owner.PopoverClick(x, y) {
				t.Errorf("disabled: PopoverClick consumed a click")
			}
			c.host.OnEvent(Event{Kind: EventClick, X: x, Y: y})
			if got := c.state(); got != before {
				t.Errorf("disabled: state %q, want unchanged %q", got, before)
			}
			c.off.Set(false)
			if o, ok := c.owner.(interface{ Open() *mvvm.Observable[bool] }); ok {
				o.Open().Set(true) // closed by the walk: the person opens it again
			}
			before = c.state()
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

// Part 4 --------------------------------------------------------------------

// A disabled Dialog is inert as a whole: the title-bar close, minimise,
// maximise and move go with the content, because the application dismisses and
// places a dialog it has disabled. A drag in flight is dropped where it was and
// does not resume on enabling. The application's own calls stay open.
func TestDisabledDialogFrameIsInert(t *testing.T) {
	closes := 0
	d := NewDialog("t", &honourProbe{})
	d.Closable, d.Minimisable, d.Maximisable = true, true, true
	d.OnClose = func() { closes++ }
	d.DragBounds = Rect{W: 1000, H: 1000}
	d.SetBounds(Rect{X: 100, Y: 100, W: 300, H: 200})
	at := func(b *IconButton) Event {
		x, y := centre(b.Bounds())
		return Event{Kind: EventClick, X: x - d.Bounds().X, Y: y - d.Bounds().Y}
	}
	cl, mx, mn := at(d.closeButton()), at(d.maxButton()), at(d.minButton())

	d.Disabled().Set(true)
	for _, ev := range []Event{cl, mx, mn} {
		d.OnEvent(ev)
	}
	if closes != 0 || d.Maximised().Get() || d.Minimised().Get() {
		t.Fatalf("disabled: close %d, maximised %v, minimised %v, want all untouched", closes, d.Maximised().Get(), d.Minimised().Get())
	}
	// the application's calls are not input and stay open
	d.Minimised().Set(true)
	d.SetBounds(Rect{X: 100, Y: 100, W: 320, H: 220})
	if !d.Minimised().Get() || d.Bounds().W != 320 {
		t.Fatalf("disabled: programmatic Minimised/SetBounds refused (minimised %v, w %d)", d.Minimised().Get(), d.Bounds().W)
	}
	d.Minimised().Set(false)

	// a drag in flight when it is disabled
	d.Disabled().Set(false)
	d.OnEvent(Event{Kind: EventClick, X: 5, Y: 5})
	d.OnEvent(Event{Kind: EventMouseDrag, X: 15, Y: 5})
	if d.dragOffX != 10 {
		t.Fatalf("enabled drag moved the panel by %d, want 10", d.dragOffX)
	}
	d.Disabled().Set(true)
	d.OnEvent(Event{Kind: EventMouseDrag, X: 45, Y: 5})
	if d.dragOffX != 10 || d.dragging {
		t.Fatalf("disabled: offset %d, dragging %v, want 10 and false", d.dragOffX, d.dragging)
	}
	d.Disabled().Set(false)
	d.OnEvent(Event{Kind: EventMouseDrag, X: 45, Y: 5})
	if d.dragOffX != 10 {
		t.Fatalf("the dropped drag resumed after enabling: offset %d, want 10", d.dragOffX)
	}
	d.OnEvent(at(d.closeButton())) // the panel moved with the enabled drag
	if closes != 1 {
		t.Fatalf("re-enabled close fired %d times, want 1", closes)
	}
}

// A ModalWindow that is disabled ignores Escape and the scrim click as well, and
// drops a title-bar drag its panel had in flight.
func TestDisabledModalWindowDropsPanelDrag(t *testing.T) {
	m := NewModalWindow("t", nil)
	m.SetBounds(Rect{W: 800, H: 600})
	pb := m.Panel.Bounds()
	m.OnEvent(Event{Kind: EventClick, X: pb.X + 5, Y: pb.Y + 5})
	m.OnEvent(Event{Kind: EventMouseDrag, X: pb.X + 15, Y: pb.Y + 5})
	if m.Panel.dragOffX != 10 {
		t.Fatalf("enabled drag moved the panel by %d, want 10", m.Panel.dragOffX)
	}
	m.Disabled().Set(true)
	m.OnEvent(Event{Kind: EventMouseDrag, X: pb.X + 45, Y: pb.Y + 5})
	m.Disabled().Set(false)
	m.OnEvent(Event{Kind: EventMouseDrag, X: pb.X + 45, Y: pb.Y + 5})
	if m.Panel.dragOffX != 10 || m.Panel.dragging {
		t.Fatalf("panel offset %d, dragging %v, want 10 and false", m.Panel.dragOffX, m.Panel.dragging)
	}
}

// Window.MoveBy and ResizeTo are the application's calls (the window never moves
// itself), so a disabled window still obeys them.
func TestDisabledWindowStillMovesAndResizesByCall(t *testing.T) {
	w := NewWindow("t", nil)
	w.Resizable = true
	w.SetBounds(Rect{X: 10, Y: 20, W: 200, H: 150})
	w.Disabled().Set(true)
	w.MoveBy(5, 6)
	w.ResizeTo(220, 160)
	if b := w.Bounds(); b.X != 15 || b.Y != 26 || b.W != 220 || b.H != 160 {
		t.Fatalf("disabled window after MoveBy/ResizeTo: %+v", b)
	}
}

// A bar dragged when the chart is disabled is let go where the drag left it and
// announced once, as a release would; the release that follows adds nothing.
func TestDisabledGanttDragInFlightCompletes(t *testing.T) {
	var got [][3]int
	g := NewGantt([]GanttTask{{Label: "A", Start: 0, End: 6}})
	g.Units = 12
	g.OnTaskChange = func(i, s, e int) { got = append(got, [3]int{i, s, e}) }
	g.SetBounds(Rect{W: 600, H: GanttHeaderH + 2*GanttRowH})
	y := scaled(GanttHeaderH) + scaled(GanttRowH)/2
	x := (g.barXLocal(0) + g.barXLocal(6)) / 2
	g.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	g.OnEvent(Event{Kind: EventMouseDrag, X: x + 60, Y: y})
	moved := g.Tasks[0]
	if moved.Start == 0 {
		t.Fatal("the enabled drag moved nothing, so the test proves nothing")
	}
	g.Disabled().Set(true)
	g.OnEvent(Event{Kind: EventMouseDrag, X: x + 120, Y: y})
	if len(got) != 1 || got[0] != [3]int{0, moved.Start, moved.End} || g.Tasks[0] != moved {
		t.Fatalf("disabled mid-drag: callbacks %v, task %+v, want one call with %+v", got, g.Tasks[0], moved)
	}
	g.Disabled().Set(false)
	g.OnEvent(Event{Kind: EventMouseDrag, X: x + 120, Y: y})
	g.OnEvent(Event{Kind: EventMouseUp, X: x + 120, Y: y})
	if len(got) != 1 || g.Tasks[0] != moved {
		t.Fatalf("the dropped drag resumed: callbacks %v, task %+v", got, g.Tasks[0])
	}
	// the application's own scroll still works
	g.Disabled().Set(true)
	g.ScrollBy(1)
}

// A handle in hand when the pane is disabled is let go; MoveHandle, the
// application's call, still moves it and still announces.
func TestDisabledPanedHandleInFlight(t *testing.T) {
	moves := 0
	p := NewHPaned(&honourProbe{}, &honourProbe{})
	p.OnPositionChanged = func(int) { moves++ }
	p.SetBounds(Rect{W: 200, H: 100})
	h := p.Position().Get() + 1
	p.OnEvent(Event{Kind: EventClick, X: h, Y: 5})
	p.OnEvent(Event{Kind: EventMouseDrag, X: 60, Y: 5})
	if p.Position().Get() != 60 {
		t.Fatalf("enabled drag left the handle at %d, want 60", p.Position().Get())
	}
	p.Disabled().Set(true)
	p.OnEvent(Event{Kind: EventMouseDrag, X: 80, Y: 5})
	p.Disabled().Set(false)
	p.OnEvent(Event{Kind: EventMouseDrag, X: 80, Y: 5})
	if p.Position().Get() != 60 || p.resizing {
		t.Fatalf("handle at %d, resizing %v, want 60 and false", p.Position().Get(), p.resizing)
	}
	p.Disabled().Set(true)
	before := moves
	p.MoveHandle(30)
	if p.Position().Get() != 30 || moves != before+1 {
		t.Fatalf("disabled MoveHandle: position %d, %d announcements, want 30 and %d", p.Position().Get(), moves, before+1)
	}
}

// A cell being edited when the sheet is disabled is committed (what was typed is
// kept and announced), and a scrollbar in hand is let go. SetCell, BeginEdit and
// ScrollBy are application calls and stay open.
func TestDisabledSpreadsheetInFlight(t *testing.T) {
	var got []string
	s := NewSpreadsheet(3, 50)
	s.OnCellChange = func(_ formula.Ref, raw string) { got = append(got, raw) }
	s.SetBounds(Rect{W: 400, H: 100})

	s.OnEvent(Event{Kind: EventChar, Code: "9"})
	if !s.Editing() {
		t.Fatal("typing did not open an editor")
	}
	s.Disabled().Set(true)
	s.OnEvent(Event{Kind: EventChar, Code: "8"})
	if s.Editing() || len(got) != 1 || got[0] != "9" || s.CellRaw(0, 0) != "9" {
		t.Fatalf("disabled mid-edit: editing %v, committed %v, cell %q, want closed, [9], 9", s.Editing(), got, s.CellRaw(0, 0))
	}

	s.Disabled().Set(false)
	g, ok := s.vscrollGeom()
	if !ok {
		t.Fatal("no vertical scrollbar: the test needs an overflowing sheet")
	}
	x, y := g.cross0+1, g.thumbStart+1
	s.OnEvent(Event{Kind: EventClick, X: x, Y: y})
	s.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: y + 10})
	_, row := s.ScrollOffset()
	if row == 0 {
		t.Fatal("the enabled thumb drag scrolled nothing, so the test proves nothing")
	}
	s.Disabled().Set(true)
	s.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: y + 30})
	s.Disabled().Set(false)
	s.OnEvent(Event{Kind: EventMouseDrag, X: x, Y: y + 30})
	if _, r := s.ScrollOffset(); r != row {
		t.Fatalf("the dropped scrollbar drag resumed: row %d, want %d", r, row)
	}

	s.Disabled().Set(true)
	s.SetCell(1, 1, "5")
	s.BeginEdit()
	s.ScrollBy(0, 1)
	if s.CellRaw(1, 1) != "5" || !s.Editing() {
		t.Fatalf("disabled: programmatic SetCell/BeginEdit refused (cell %q, editing %v)", s.CellRaw(1, 1), s.Editing())
	}
	if _, r := s.ScrollOffset(); r != row+1 {
		t.Fatalf("disabled: ScrollBy left row %d, want %d", r, row+1)
	}
}

// RichEditor.InsertText is the application's call (a paste, a macro, a bound
// view model), not input, so it still edits a disabled editor.
func TestDisabledRichEditorStillEditsByCall(t *testing.T) {
	e := NewRichEditor(richdoc.New().P(richdoc.Txt("hi")).Doc())
	e.Disabled().Set(true)
	e.InsertText("!")
	if got := richdoc.PlainText(e.docValue()); got != "!hi" && got != "hi!" {
		t.Fatalf("disabled editor after InsertText: %q", got)
	}
}

// Keyboard: the two focusable leaves are walked past while disabled, and a key
// handed to them directly changes nothing.
func TestDisabledEditorsTakeNoTypedKeys(t *testing.T) {
	re := NewRichEditor(richdoc.New().P(richdoc.Txt("hi")).Doc())
	sp := NewSpreadsheet(3, 3)
	for name, w := range map[string]Widget{"RichEditor": re, "Spreadsheet": sp} {
		v := NewVBox()
		v.Append(w)
		v.SetBounds(Rect{W: 400, H: 200})
		state := func() string {
			if name == "RichEditor" {
				return richdoc.PlainText(re.docValue())
			}
			return fmt.Sprint(sp.Editing())
		}
		before := state()
		w.(interface{ Disabled() *mvvm.Observable[bool] }).Disabled().Set(true)
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		v.OnEvent(Event{Kind: EventChar, Code: "z"})
		w.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := state(); got != before {
			t.Errorf("%s disabled: state %q after typing, want %q", name, got, before)
		}
		w.(interface{ Disabled() *mvvm.Observable[bool] }).Disabled().Set(false)
		v.OnEvent(Event{Kind: EventKeyDown, Code: "Tab"})
		v.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := state(); got == before {
			t.Errorf("%s enabled: typing changed nothing, so the gate proves nothing", name)
		}
	}
}

// A Dialog and a ModalWindow forward keys to their content on their own: a
// focused Entry inside takes none while they are disabled. The other composites
// of this batch are dead ends for the focus walk, so a key reaches their
// children only through their own gated OnEvent; if one becomes a focus
// enumerator this fails and the gate must be re-checked.
func TestDisabledDialogsTakeNoKeysForTheirContent(t *testing.T) {
	for name, build := range map[string]func(*Entry) (Widget, *mvvm.Observable[bool]){
		"Dialog": func(e *Entry) (Widget, *mvvm.Observable[bool]) {
			d := NewDialog("t", e)
			d.SetBounds(Rect{W: 300, H: 200})
			return d, d.Disabled()
		},
		"ModalWindow": func(e *Entry) (Widget, *mvvm.Observable[bool]) {
			m := NewModalWindow("t", e)
			m.SetBounds(Rect{W: 800, H: 600})
			return m, m.Disabled()
		},
	} {
		e := NewEntry("ab")
		e.SetFocused(true)
		w, off := build(e)
		off.Set(true)
		w.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := e.Text().Get(); got != "ab" {
			t.Errorf("%s disabled: Entry became %q, want \"ab\"", name, got)
		}
		off.Set(false)
		w.OnEvent(Event{Kind: EventChar, Code: "z"})
		if got := e.Text().Get(); got != "abz" {
			t.Errorf("%s enabled: Entry is %q, want \"abz\"", name, got)
		}
	}
	for name, w := range map[string]Widget{
		"Dialog": NewDialog("t", nil), "ModalWindow": NewModalWindow("t", nil), "Window": NewWindow("t", nil),
		"Paned": NewHPaned(nil, nil), "Surface": NewSurface(nil), "Gantt": NewGantt(nil),
	} {
		if _, ok := w.(focusEnumerator); ok {
			t.Errorf("%s is a focusEnumerator: keys can reach its children past its gate", name)
		}
	}
}

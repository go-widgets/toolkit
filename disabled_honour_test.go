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

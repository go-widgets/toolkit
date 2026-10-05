// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Stack holds N named pages (Widgets) but shows only ONE at a time --
// the page named by Visible. Use AddPage / SetVisible to navigate.
// Events route to the visible page only, and so do the focus system and
// every generic tree walk (see [Stack.Children]): a widget on the visible page
// takes keyboard focus and typed keys, and a DropDown on it opens its popover,
// exactly as inside a [Container].
//
// Suitable for application "screens" (settings vs main vs about),
// wizard steps, or anywhere the user expects a CLEAN swap with no
// transition.
type Stack struct {
	Base
	Pages map[string]Widget

	// visible names the shown page as reactive state; see [Stack.Visible].
	visible *mvvm.Observable[string]
}

// NewStack builds an empty Stack with no pages + no visible name.
func NewStack() *Stack { return &Stack{Pages: map[string]Widget{}} }

// Visible names the currently-shown page as a shared [mvvm.Observable]:
// AddPage/SetVisible Set it, and Draw/OnEvent read it live. Lazily created,
// defaulting to no visible page (the empty name).
func (s *Stack) Visible() *mvvm.Observable[string] {
	if s.visible == nil {
		s.visible = mvvm.NewObservable("")
	}
	return s.visible
}

// AddPage registers a page under name. If this is the first page,
// it auto-becomes Visible so an unconfigured Stack still draws
// something.
func (s *Stack) AddPage(name string, w Widget) {
	s.Pages[name] = w
	if s.Visible().Get() == "" {
		s.Visible().Set(name)
	}
}

// SetVisible swaps the showing page. Names not in Pages are
// silently ignored so the caller can SetVisible blind.
func (s *Stack) SetVisible(name string) {
	if _, ok := s.Pages[name]; ok {
		s.Visible().Set(name)
	}
}

// SetBounds also propagates to the visible page so it fills the
// Stack's rect. Other pages have stale bounds until SetVisible
// brings them forward -- they re-bound at draw time.
func (s *Stack) SetBounds(r Rect) {
	s.Base.SetBounds(r)
	if p := s.visiblePage(); p != nil {
		p.SetBounds(r)
	}
}

// visiblePage returns the page named by Visible, or nil when no page is shown.
func (s *Stack) visiblePage() Widget { return s.Pages[s.Visible().Get()] }

// Children yields the visible page only. A hidden page is not on screen, yet it
// keeps the bounds it last had -- which are the Stack's own -- so handing it to
// a generic walk would put its controls where the visible page's are: a screen
// reader would announce them, and an open popover on a hidden page would take
// the visible page's clicks.
func (s *Stack) Children() []Widget { return nonNil(s.visiblePage()) }

// focusableChildren yields the visible page so the focus walker reaches the
// focusable widgets on it, and never those of a hidden page (see Children).
func (s *Stack) focusableChildren() []Widget { return s.Children() }

// Draw paints only the visible page.
func (s *Stack) Draw(p painter.Painter, theme *Theme) {
	if page := s.visiblePage(); page != nil {
		page.SetBounds(s.Bounds())
		page.Draw(p, theme)
	}
}

// OnEvent routes keyboard events through the focus system -- Tab/Shift+Tab
// move focus through the visible page's focusable widgets, and any other key or
// character goes to the focused one -- exactly as a [Container] does. A click
// additionally moves focus to the focusable widget it lands on. Every pointer
// event is then forwarded to the visible page, which fills the Stack, so its
// coordinates need no translation.
func (s *Stack) OnEvent(ev Event) {
	if routeFocusKey(s, ev) {
		return
	}
	page := s.visiblePage()
	if page == nil {
		return
	}
	if ev.Kind == EventClick {
		pr := s.Bounds()
		focusClick(s, ev.X+pr.X, ev.Y+pr.Y)
	}
	page.OnEvent(ev)
}

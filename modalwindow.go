// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// ModalWindow is a self-contained modal dialog: a dimming, click-catching scrim
// over the whole surface with a centred [Dialog] panel floating on top. Unlike a
// bare Dialog — which is only the card, leaving the host to composite a backdrop
// and place it — a ModalWindow owns the scrim, centres its panel, and dismisses
// itself on the panel's close (×) button, on Escape, and (by default) on a click
// on the scrim outside the panel. Every dismissal path calls OnClose, the single
// hook a host wires to unmount the modal.
//
// It is assembled entirely from toolkit widgets: a [Backdrop] scrim (a
// translucent, Interactive ground) and a [Dialog] panel (title bar + optional
// top input bar + content + action buttons + the close button). The panel is
// reached directly as Panel, so a host configures the title/content/buttons and
// enables the optional search field on it (or uses [NewSearchModal]).
//
// A ModalWindow reports itself as presentational to the accessibility walk and
// exposes the scrim + panel as its children, so the panel's own RoleDialog node
// (and the content within it) is what a screen reader announces — the wrapper
// adds no duplicate dialog node.
type ModalWindow struct {
	Base

	// Scrim is the dimming, click-catching ground painted behind the panel.
	Scrim *Backdrop
	// Panel is the centred dialog card. Configure its Title, Content, Buttons and
	// optional Input here; its Closable close button and OnClose are pre-wired by
	// the constructor to dismiss the modal.
	Panel *Dialog
	// OnClose is called on every dismissal — the panel's × button, an Escape key,
	// or (when CloseOnScrim) a click on the scrim outside the panel. nil is safe.
	OnClose func()

	// CloseOnScrim, when true, dismisses the modal when a click lands on the scrim
	// outside the panel. [NewModalWindow] sets it true (the least-surprising
	// desktop default); set it false for a strict modal that ignores outside
	// clicks and can be dismissed only via a button or Escape. The zero value of a
	// bare &ModalWindow{} is false.
	CloseOnScrim bool

	// PanelW, PanelH are the panel's preferred size in LOGICAL pixels, centred in
	// the modal's bounds and clamped to fit. Zero selects ModalPanelW / ModalPanelH.
	PanelW, PanelH int
}

// ModalScrimAlpha is the opacity (0..255) of the dimming scrim, matching the
// action-sheet scrim so modals dim the background to the same depth.
const ModalScrimAlpha uint8 = 120

// ModalPanelW and ModalPanelH are the default centred-panel size in logical
// pixels when PanelW / PanelH are left zero.
const (
	ModalPanelW = 360
	ModalPanelH = 260
)

// NewModalWindow builds a modal around a centred Dialog panel: a translucent,
// click-catching scrim, a Closable panel whose × button and Escape dismiss it,
// and CloseOnScrim enabled so a click outside the panel dismisses it too. Pass
// the panel title, its content widget (may be nil) and any action buttons.
func NewModalWindow(title string, content Widget, buttons ...*Button) *ModalWindow {
	m := &ModalWindow{
		Scrim:        &Backdrop{Fill: RGBA{A: ModalScrimAlpha}, Interactive: true},
		CloseOnScrim: true,
	}
	d := NewDialog(title, content, buttons...)
	d.Closable = true
	d.OnClose = func() { m.dismiss() }
	m.Panel = d
	return m
}

// NewSearchModal builds a [NewModalWindow] whose panel carries a focused
// [SearchEntry] as its top input bar, and returns both. Bind or subscribe to the
// returned entry's Text() observable to drive a live search over the content.
func NewSearchModal(title string, content Widget, buttons ...*Button) (*ModalWindow, *SearchEntry) {
	m := NewModalWindow(title, content, buttons...)
	se := NewSearchEntry("")
	se.SetFocused(true)
	m.Panel.Input = se
	return m, se
}

// SetBounds spreads the scrim over the whole surface and centres the panel
// within it, clamped to fit.
func (m *ModalWindow) SetBounds(r Rect) {
	m.Base.SetBounds(r)
	if m.Scrim != nil {
		m.Scrim.SetBounds(r)
	}
	w, h := scaled(ModalPanelW), scaled(ModalPanelH)
	if m.PanelW != 0 {
		w = scaled(m.PanelW)
	}
	if m.PanelH != 0 {
		h = scaled(m.PanelH)
	}
	if w > r.W {
		w = r.W
	}
	if h > r.H {
		h = r.H
	}
	if m.Panel != nil {
		// Constrain a title-bar drag to the modal's own bounds so the panel — and
		// thus its title bar — can never be dragged off-screen. Set before laying
		// the panel out so the drag offset (if the user has already moved it) is
		// clamped against the current surface on this relayout too.
		m.Panel.DragBounds = r
		m.Panel.SetBounds(Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h})
	}
}

// Draw paints the scrim, then the panel on top of it.
func (m *ModalWindow) Draw(p painter.Painter, theme *Theme) {
	if m.Scrim != nil {
		m.Scrim.Draw(p, theme)
	}
	if m.Panel != nil {
		m.Panel.Draw(p, theme)
	}
}

// OnEvent handles Escape (dismiss) and scrim clicks (dismiss when CloseOnScrim),
// and forwards everything else to the panel. Event coordinates are modal-local
// (the surface-frame convention Overlay uses); a click is "inside the panel" when
// its absolute point falls in the panel's bounds.
func (m *ModalWindow) OnEvent(ev Event) {
	if ev.Kind == EventKeyDown && ev.Code == "Escape" {
		m.dismiss()
		return
	}
	if m.Panel == nil {
		return
	}
	if ev.Kind == EventClick {
		inside := m.Panel.Bounds().Contains(ev.X+m.Bounds().X, ev.Y+m.Bounds().Y)
		if !inside {
			if m.CloseOnScrim {
				m.dismiss()
			}
			return
		}
	}
	m.Panel.OnEvent(translateEvent(ev, m.Bounds(), m.Panel.Bounds()))
}

// dismiss fires OnClose (nil-safe) — the shared path for the × button, Escape and
// a scrim click.
func (m *ModalWindow) dismiss() {
	if m.OnClose != nil {
		m.OnClose()
	}
}

// A11y reports the modal as presentational: the wrapper is scrim + z-order, and
// the panel it contains carries the RoleDialog node a reader announces. See the
// type doc.
func (m *ModalWindow) A11y() A11yInfo { return A11yInfo{Role: RolePresentation} }

// Children yields the scrim then the panel, the order they are painted in, so a
// generic walk (accessibility) descends into the panel and its content.
func (m *ModalWindow) Children() []Widget {
	var out []Widget
	if m.Scrim != nil {
		out = append(out, m.Scrim)
	}
	if m.Panel != nil {
		out = append(out, m.Panel)
	}
	return out
}

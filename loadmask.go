// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// LoadMaskSpinnerSize is the pixel side of a LoadMask's centred spinner.
const LoadMaskSpinnerSize = 32

// LoadMask is a "busy" overlay: while Active it dims its whole bounds with a
// translucent scrim, paints a centred indeterminate Spinner and an optional
// Message, and swallows pointer events so the content beneath cannot be
// interacted with mid-load. Inactive, it draws nothing and lets events pass
// through (HitTest false), so it is safe to leave permanently mounted as the
// topmost Overlay layer and just toggle Active.
//
// Drive the spinner animation by calling Tick(dt) from the host's frame loop
// (no goroutine/timer), the same cadence contract as Spinner and ProgressCircle.
type LoadMask struct {
	Base
	// Active gates the whole widget: false draws nothing and is
	// event-transparent; true dims + shows the spinner/message + swallows
	// events. The zero value is inactive.
	Active bool
	// Message is an optional caption shown under the spinner (e.g. "Loading…").
	Message string
	// Scrim is the dimming colour painted over the bounds. The zero value
	// uses a translucent black (src-over blended by the pixel back-end), so a
	// LoadMask dropped in with no configuration reads as a subtle dim.
	Scrim painter.RGBA

	spinner *Spinner
}

// NewLoadMask builds an inactive LoadMask with the given message (may be "").
func NewLoadMask(message string) *LoadMask {
	return &LoadMask{Message: message, spinner: NewSpinner()}
}

// Tick advances the spinner animation by deltaSeconds (a no-op visual while
// inactive, but cheap to keep calling).
func (m *LoadMask) Tick(deltaSeconds float64) { m.spinner.Tick(deltaSeconds) }

// HitTest reports whether the mask should catch a pointer event: only while
// Active, so an inactive mask is fully transparent to clicks and an active one
// shields the content beneath it (the modal-scrim idiom, see Backdrop). While
// Active a covered event routes to the inherited Base.OnEvent no-op, i.e. it is
// swallowed and never reaches the content underneath.
func (m *LoadMask) HitTest(px, py int) bool {
	return m.Active && m.Bounds().Contains(px, py)
}

// Draw dims the bounds and paints the spinner + message while Active; inactive
// or empty-bounds it paints nothing.
func (m *LoadMask) Draw(p painter.Painter, theme *Theme) {
	if !m.Active {
		return
	}
	r := m.Bounds()
	if r.W <= 0 || r.H <= 0 {
		return
	}
	scrim := m.Scrim
	if scrim == (painter.RGBA{}) {
		scrim = painter.RGBA{A: 110} // translucent black
	}
	fillRect(p, r.X, r.Y, r.W, r.H, scrim)

	// Centre the spinner; when a message is present, lift the spinner so the
	// spinner + caption together stay vertically centred.
	sz := LoadMaskSpinnerSize
	gap := 4
	block := sz
	if m.Message != "" {
		block = sz + gap + m.glyphHeight()
	}
	sx := r.X + (r.W-sz)/2
	top := r.Y + (r.H-block)/2
	m.spinner.Active = true
	m.spinner.SetBounds(Rect{X: sx, Y: top, W: sz, H: sz})
	m.spinner.Draw(p, theme)

	if m.Message != "" {
		tw := m.textWidth(m.Message)
		m.drawText(p, r.X+(r.W-tw)/2, top+sz+gap, m.Message, theme.OnSurface)
	}
}

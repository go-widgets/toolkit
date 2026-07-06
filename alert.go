// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/painter"

// AlertKind selects the semantic colour of an Alert banner. Info reuses
// the theme's Accent (the same blue used by focus rings + link text);
// the other three carry hard-coded shades tuned for meaning — green
// for success, amber for warning, red for error — because the theme
// palette doesn't carry semantic slots and adding them would blow up
// the Theme surface for every app.
type AlertKind int

const (
	// AlertInfo is a neutral heads-up ("Backup started"). Rendered in
	// Theme.Accent so it matches the app's own accent colour.
	AlertInfo AlertKind = iota
	// AlertSuccess signals a completed operation ("Saved!"). Green.
	AlertSuccess
	// AlertWarning flags a non-fatal issue ("Battery low"). Amber.
	AlertWarning
	// AlertError signals a failure the user must address ("Sync
	// failed"). Red.
	AlertError
)

// Alert is a persistent banner sitting at the top or bottom of a view,
// carrying a Text message coloured by Kind. Shares Notification's
// filled-panel-with-border shape but differs in three ways:
//
//  1. No Life field: an Alert stays on screen until the host removes it
//     (the "you are offline" banner). No Tick(), no auto-hide.
//  2. No Visible toggle: an Alert that exists is drawn. To hide an
//     alert the host stops rendering it (or drops it from the tree).
//  3. Coloured by Kind: Notification is always Accent; Alert varies
//     colour by severity so success + error read differently at a glance.
//
// The banner is not interactive; the parent view supplies a dismiss
// button as a separate Button if the design calls for one.
type Alert struct {
	Base
	Text string
	Kind AlertKind
}

// AlertPadX / AlertPadY set the internal margin between the banner
// edges and the text. Matches Notification's PadX/PadY so the two
// widgets read as siblings when they're used side-by-side.
const (
	AlertPadX = 12
	AlertPadY = 8
)

// NewAlert constructs an Alert with the given Text + Kind. Bounds are
// zero-initialised; the host is responsible for positioning + sizing
// the banner (typically full-width across the top of the parent view).
func NewAlert(text string, kind AlertKind) *Alert {
	return &Alert{Text: text, Kind: kind}
}

// alertFace maps a Kind to a background colour. AlertInfo defers to
// the theme so it blends with the app's accent choice; the other
// three carry fixed shades since the theme doesn't (and shouldn't)
// grow semantic-colour slots for every widget that wants one.
func alertFace(kind AlertKind, theme *Theme) RGBA {
	switch kind {
	case AlertSuccess:
		return RGB(0x2E, 0x8B, 0x57) // sea green
	case AlertWarning:
		return RGB(0xE0, 0xA0, 0x30) // amber
	case AlertError:
		return RGB(0xC0, 0x30, 0x30) // brick red
	default: // AlertInfo (also any out-of-range Kind values)
		return theme.Accent
	}
}

// Draw paints the filled panel + border + text. The ink is
// Theme.Background so it stays legible against every Kind's face — the
// same inversion trick Notification uses against its own Accent panel.
func (a *Alert) Draw(p painter.Painter, theme *Theme) {
	r := a.Bounds()
	face := alertFace(a.Kind, theme)
	fillRect(p, r.X, r.Y, r.W, r.H, face)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
	DrawText(p, r.X+AlertPadX, r.Y+AlertPadY, a.Text, theme.Background)
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Link is an inline hyperlink: a single run of text painted in the theme's
// Accent ink (the link colour) that fires OnClick when activated and grows an
// accent underline while the pointer hovers it — the visual "this is clickable"
// feedback a bare accent-coloured Label cannot give. It is the interactive
// counterpart of Label, and the inline sibling of LinkCard (which is a whole
// card, not a text run).
//
// Unlike a decorative Label, a Link is interactive: it tracks its own hover
// face from EventMouseMove (like Button) so a host that already forwards moves
// to its children gets the underline for free, and it activates on EventClick
// or an Enter/Space key press while focused. There is no pointer-cursor field —
// the toolkit exposes no cursor seam, so the underline alone signals the link.
//
// The underline is drawn only while hovered, unless Underline is set to force
// it permanently on (an always-underlined link). A caller re-paints via Draw
// after any state change, as with every other toolkit widget.
type Link struct {
	Base
	// text is the link caption, reactive via the Text() accessor.
	text *mvvm.Observable[string]
	// OnClick is fired when the link is activated (click or Enter/Space).
	// May be nil — the link still renders and tracks hover, it just does
	// nothing when activated.
	OnClick func()
	// Ink overrides the link colour. The zero value (A==0) means "inherit the
	// theme's Accent colour" (the conventional link ink); set a colour with a
	// non-zero alpha to paint the link in it.
	Ink RGBA
	// Underline forces the underline on regardless of hover — an
	// always-underlined link. The zero value (false) underlines only while the
	// pointer is over the link, which is the hover-feedback default.
	Underline bool
	// Align is the horizontal alignment of the text within the widget's bounds
	// (left by default, matching Label).
	Align Align
	// VAlign is the vertical alignment within the bounds height; VAuto keeps the
	// centre-when-taller layout (matching Label).
	VAlign VAlign

	hovered bool
}

// NewLink constructs a Link with the given caption and click handler. Handler
// may be nil (a link that renders + tracks hover but does nothing on click).
func NewLink(text string, onClick func()) *Link {
	l := &Link{OnClick: onClick}
	l.text = mvvm.NewObservable(text)
	return l
}

// Text is the link's caption as a shared [mvvm.Observable]: a host binds it (or
// subscribes) instead of touching a field. Lazily created so a bare &Link{}
// works.
func (l *Link) Text() *mvvm.Observable[string] {
	if l.text == nil {
		l.text = mvvm.NewObservable("")
	}
	return l.text
}

// SetHovered lets a parent container drive the hover state directly (from its
// own enter/leave dispatch) instead of routing EventMouseMove — the same seam
// Button exposes. Both routes reach the identical underline.
func (l *Link) SetHovered(v bool) { l.hovered = v }

// Hovered reports whether the link currently shows its hover (underlined) face —
// true while the pointer is over it. Exposed so a host (or a test) can read the
// state the underline is derived from.
func (l *Link) Hovered() bool { return l.hovered }

// ink is the colour the link paints in: its Ink override when set, else the
// theme's Accent (the conventional link ink).
func (l *Link) ink(theme *Theme) RGBA {
	if l.Ink.A != 0 {
		return l.Ink
	}
	return theme.Accent
}

// Draw paints the caption in the link ink and, while hovered (or when Underline
// is forced on), an accent underline one logical pixel thick spanning the text
// directly beneath the glyphs. Text is positioned per Align / VAlign, matching
// Label so a Link can stand in for an accent-coloured Label with no layout
// change.
func (l *Link) Draw(p painter.Painter, theme *Theme) {
	r := l.Bounds()
	f := l.EffectiveFont()
	gh := f.Height()
	text := l.Text().Get()
	tw := f.Measure(text)

	ty := r.Y // VTop: anchored to the top edge.
	switch l.VAlign {
	case VAuto:
		if r.H > gh {
			ty = r.Y + (r.H-gh)/2
		}
	case VMiddle:
		ty = r.Y + (r.H-gh)/2
	case VBottom:
		ty = r.Y + r.H - gh
	}

	tx := r.X
	switch l.Align {
	case AlignCenter:
		tx = r.X + (r.W-tw)/2
	case AlignRight:
		tx = r.X + r.W - tw
	}
	if tx < r.X {
		tx = r.X // never start left of the widget
	}

	ink := l.ink(theme)
	f.Draw(p, tx, ty, text, ink)

	if l.hovered || l.Underline {
		// The underline sits one logical pixel under the glyph box, spanning
		// exactly the text width, in the link ink — the same accent-bar idiom the
		// tab indicator uses (fillRect at a fixed thickness).
		fillRect(p, tx, ty+gh, tw, strokeWidth(), ink)
	}
}

// OnEvent activates the link on a click or an Enter/Space key press (while
// focused), and tracks its hover face from EventMouseMove — raising the
// underline when the pointer is over the link and clearing it when a container
// forwards a move whose point has left it. A disabled link ignores every event.
func (l *Link) OnEvent(ev Event) {
	if l.Disabled().Get() {
		return
	}
	switch ev.Kind {
	case EventClick:
		l.activate()
	case EventKeyDown:
		switch ev.Code {
		case "Enter", " ", "Space":
			l.activate()
		}
	case EventMouseMove:
		l.hovered = l.localInBounds(ev.X, ev.Y)
	}
}

// activate fires OnClick (nil-safe) — the single path shared by a click and an
// Enter/Space press so both routes behave identically.
func (l *Link) activate() {
	if l.OnClick != nil {
		l.OnClick()
	}
}

// A11y reports the link as a hyperlink named by its caption.
func (l *Link) A11y() A11yInfo {
	return A11yInfo{Role: RoleLink, Name: l.Text().Get()}
}

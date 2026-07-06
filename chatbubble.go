// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/painter"
)

// ChatSender enumerates which side of a chat transcript a ChatBubble
// belongs to. The two roles paint differently: user messages align
// to the right of the widget's Bounds in Theme.Accent, other-party
// messages align to the left in Theme.SurfaceAlt. The distinction
// is purely visual — the sender does not affect layout otherwise.
type ChatSender int

const (
	// ChatFromUser marks a message sent by the local user. Bubble
	// right-aligned in Theme.Accent; ink = accent-inverted colour.
	ChatFromUser ChatSender = iota
	// ChatFromOther marks a message from a remote party. Bubble
	// left-aligned in Theme.SurfaceAlt; ink = Theme.OnSurface.
	ChatFromOther
)

// ChatBubble is a chat-transcript speech bubble: a small rounded
// rectangle (borrowed shape only — the toolkit's raster stays sharp-
// cornered) holding a short message string. Multi-line text is
// supported by splitting Text on '\n'; each line renders on its own
// glyph row.
//
// Sizing: the bubble grows to fit the widest text line plus 2*PadX,
// capped at ChatBubbleMaxW so a runaway paste doesn't spill the
// widget's Bounds. Height is len(lines) * lineH + 2*PadY where lineH
// = GlyphHeight + ChatBubbleLineSpacing.
//
// ChatBubble is a passive display widget — it does not intercept
// input (HitTest / OnEvent stay as Base defaults). A caller that
// wants a tap-to-copy or long-press-menu bubble wraps this with an
// outer container that handles the gesture.
type ChatBubble struct {
	Base
	Text   string
	Sender ChatSender
}

// Sizing constants for the bubble geometry.
const (
	// ChatBubblePadX is the horizontal inset between the bubble edge
	// and the text.
	ChatBubblePadX = 10
	// ChatBubblePadY is the vertical inset above the first text line
	// and below the last.
	ChatBubblePadY = 6
	// ChatBubbleMaxW caps the bubble's rendered width in pixels so a
	// pathologically long line stays inside a reasonable column.
	ChatBubbleMaxW = 220
	// ChatBubbleLineSpacing is the extra vertical gap between two
	// text lines in a multi-line bubble.
	ChatBubbleLineSpacing = 2
)

// NewChatBubble constructs a ChatBubble carrying text sent by sender.
func NewChatBubble(text string, sender ChatSender) *ChatBubble {
	return &ChatBubble{Text: text, Sender: sender}
}

// Draw paints the bubble: fill in Theme.Accent (user) or SurfaceAlt
// (other), 1-pixel Theme.Border stroke, and one DrawText per '\n'-
// separated line in Text. Width is derived from the widest line's
// TextWidth capped by ChatBubbleMaxW; height is derived from the
// line count. Position is right-aligned within Bounds() for
// ChatFromUser and left-aligned for ChatFromOther, mirroring the
// canonical chat-transcript convention.
func (c *ChatBubble) Draw(p painter.Painter, theme *Theme) {
	r := c.Bounds()
	lines := strings.Split(c.Text, "\n")
	maxW := 0
	for _, ln := range lines {
		w := TextWidth(ln)
		if w > maxW {
			maxW = w
		}
	}
	bubbleW := maxW + 2*ChatBubblePadX
	if bubbleW > ChatBubbleMaxW {
		bubbleW = ChatBubbleMaxW
	}
	lineH := GlyphHeight + ChatBubbleLineSpacing
	bubbleH := len(lines)*lineH - ChatBubbleLineSpacing + 2*ChatBubblePadY

	var bx int
	var fill, ink RGBA
	if c.Sender == ChatFromUser {
		bx = r.X + r.W - bubbleW
		fill = theme.Accent
		ink = accentInk(theme)
	} else {
		bx = r.X
		fill = theme.SurfaceAlt
		ink = theme.OnSurface
	}
	by := r.Y

	fillRect(p, bx, by, bubbleW, bubbleH, fill)
	strokeRect(p, bx, by, bubbleW, bubbleH, theme.Border)

	ty := by + ChatBubblePadY
	for _, ln := range lines {
		DrawText(p, bx+ChatBubblePadX, ty, ln, ink)
		ty += lineH
	}
}

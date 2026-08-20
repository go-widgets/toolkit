// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// GroupCard is a collapsible summary card for a set of related items — a
// multi-part post, a thread, a release split across files. Its header is always
// shown: a disclosure chevron, a coloured source pill, an optional status pill
// (e.g. "complete" / "incomplete"), a title, and a muted meta line. When the post
// is Actionable it also carries, right-aligned in the header, a download checkbox
// and an action pill (e.g. "Reconstruct"). Expanding the card lists its Members —
// one preformatted line per part — beneath the header, divider-separated.
//
// Layout (inside the CardPadX/Y inset):
//
//	┌──────────────────────────────────────────────┐
//	│ ▸ [Pill] [Status]              [x] (Action)    │  ← header: chevron, badges, affordance
//	│   Title over one elided line                   │
//	│   meta · line · here                           │
//	│   ── member line 1 ──────────────              │  ← Members, only when Expanded
//	│   ── member line 2 ──────────────              │
//	└──────────────────────────────────────────────┘
//
// Like PostCard it is passive content: it lays out and paints itself and reports
// its exact height through Measure(width) (taller when Expanded); a feed list
// (CardList / VirtualList) puts selection / hover affordances on top, and reads
// the chevron / checkbox / action hit rectangles (ChevronRect / CheckRect /
// ActionRect) to route clicks. The title, meta and member lines are real Labels
// exposed through Children, so CollectRuns lifts them out as selectable text runs.
type GroupCard struct {
	Base
	// Pill is the coloured source tag (e.g. "Usenet"). Empty draws no pill.
	Pill string
	// PillColor / PillInk colour the source pill; the zero value (A==0) falls back
	// to Theme.Accent / a readable ink (see PostCard.pillInk).
	PillColor, PillInk RGBA
	// Status is the optional status pill beside the source pill (e.g. "complete").
	// Empty hides it.
	Status string
	// StatusColor / StatusInk colour the status pill; the zero value falls back to
	// Theme.Accent / a readable ink.
	StatusColor, StatusInk RGBA
	// Title is the group's headline (e.g. the release base name), one elided line.
	Title string
	// Meta is the muted summary line (e.g. "12 parts · 3 files · 40 MB").
	Meta string
	// Members are the expanded part lines, one preformatted string per row.
	Members []string
	// Actionable enables the header affordance: a download checkbox and the Action
	// pill. When false neither is drawn and CheckRect / ActionRect are empty.
	Actionable bool
	// Action is the pill label shown when Actionable (e.g. "Reconstruct"). Empty
	// draws no pill but still reserves the checkbox when Actionable.
	Action string

	// expanded / checked are the reactive disclosure + download-checkbox state;
	// see [GroupCard.Expanded] and [GroupCard.Checked].
	expanded *mvvm.Observable[bool]
	checked  *mvvm.Observable[bool]

	// Per-element fonts, each optional (nil falls back to EffectiveFont). TitleFont
	// sizes the headline, MetaFont the meta + member lines, PillFont the badges and
	// the action pill.
	TitleFont, MetaFont, PillFont Font

	// Built leaves, rebuilt on every assemble so Children reflects the current
	// content. They are the SelectableText runs CollectRuns lifts out.
	titleLbl   *Label
	metaLbl    *Label
	memberLbls []*Label
}

const (
	// GroupChevronW is the logical width of the disclosure-chevron column.
	GroupChevronW = 16
	// GroupMemberH is the logical height of one expanded member row.
	GroupMemberH = 20
	// GroupCheckSize is the logical side length of the download checkbox.
	GroupCheckSize = 18
)

// NewGroupCard builds a collapsed GroupCard from its header text fields.
func NewGroupCard(pill, title, meta string) *GroupCard {
	return &GroupCard{Pill: pill, Title: title, Meta: meta}
}

// Expanded is the reactive disclosure state as a shared [mvvm.Observable]: true
// shows the member list below the header. Lazily created, defaulting to collapsed.
func (c *GroupCard) Expanded() *mvvm.Observable[bool] {
	if c.expanded == nil {
		c.expanded = mvvm.NewObservable(false)
	}
	return c.expanded
}

// Checked is the reactive download-checkbox state as a shared [mvvm.Observable].
// Lazily created, defaulting to unchecked.
func (c *GroupCard) Checked() *mvvm.Observable[bool] {
	if c.checked == nil {
		c.checked = mvvm.NewObservable(false)
	}
	return c.checked
}

func (c *GroupCard) titleFont() Font { return orFont(c.TitleFont, c.EffectiveFont()) }
func (c *GroupCard) metaFont() Font  { return orFont(c.MetaFont, c.EffectiveFont()) }
func (c *GroupCard) pillFont() Font  { return orFont(c.PillFont, c.EffectiveFont()) }

// badgeRowH is the header badge row's height: the pill font plus the badge's
// vertical padding.
func (c *GroupCard) badgeRowH() int { return c.pillFont().Height() + 2*scaled(BadgePadY) }

// titleSlot is the vertical slot the single title line occupies.
func (c *GroupCard) titleSlot() int { return c.titleFont().Height() + scaled(CardLineSpacing) }

// metaH is the meta line's height.
func (c *GroupCard) metaH() int { return c.metaFont().Height() }

// memberH is one expanded member row's height.
func (c *GroupCard) memberH() int { return scaled(GroupMemberH) }

// headerContentH is the header's stacked content height (badges, title, meta),
// exclusive of the card's CardPadY inset.
func (c *GroupCard) headerContentH() int { return c.badgeRowH() + c.titleSlot() + c.metaH() }

// membersH is the total height the expanded member rows add (0 when collapsed).
func (c *GroupCard) membersH() int {
	if !c.Expanded().Get() {
		return 0
	}
	return len(c.Members) * c.memberH()
}

// Measure reports the card's exact height at outer width width: the CardPadY inset
// top and bottom, the header content, and — when expanded — the member rows.
func (c *GroupCard) Measure(width int) int {
	return 2*scaled(CardPadY) + c.headerContentH() + c.membersH()
}

// innerRect is the content rectangle: Bounds inset by CardPadX / Y.
func (c *GroupCard) innerRect() Rect {
	r := c.Bounds()
	return Rect{X: r.X + scaled(CardPadX), Y: r.Y + scaled(CardPadY), W: r.W - 2*scaled(CardPadX), H: r.H - 2*scaled(CardPadY)}
}

// chevronW is the device-pixel width of the chevron column.
func (c *GroupCard) chevronW() int { return scaled(GroupChevronW) }

// contentX is the left edge of the header content column (right of the chevron).
func (c *GroupCard) contentX(inner Rect) int { return inner.X + c.chevronW() }

// pillWidth is the device-pixel width a pill of text occupies (font width plus
// the badge's horizontal padding on each side). Zero for empty text.
func (c *GroupCard) pillWidth(text string) int {
	if text == "" {
		return 0
	}
	return c.pillFont().Measure(text) + 2*scaled(BadgePadX)
}

// ChevronRect is the square hit target for the disclosure chevron, vertically
// centred on the header content at the card's left.
func (c *GroupCard) ChevronRect() Rect {
	inner := c.innerRect()
	d := c.chevronW()
	return Rect{X: inner.X, Y: inner.Y + (c.headerContentH()-d)/2, W: d, H: d}
}

// actionW / actionH size the Action pill. actionW is only ever called from
// ActionRect, which has already established Action != "".
func (c *GroupCard) actionW() int {
	return c.pillFont().Measure(c.Action) + 2*scaled(BadgePadX) + scaled(CardPadX)
}
func (c *GroupCard) actionH() int { return c.pillFont().Height() + 2*scaled(BadgePadY) + scaled(4) }

// ActionRect is the Action pill's rectangle, right-aligned and vertically centred
// on the header. Empty when the card is not Actionable or Action is unset.
func (c *GroupCard) ActionRect() Rect {
	if !c.Actionable || c.Action == "" {
		return Rect{}
	}
	inner := c.innerRect()
	w, h := c.actionW(), c.actionH()
	return Rect{X: inner.X + inner.W - w, Y: inner.Y + (c.headerContentH()-h)/2, W: w, H: h}
}

// CheckRect is the download checkbox's rectangle, left of the Action pill (or
// right-aligned when there is no Action pill). Empty when not Actionable.
func (c *GroupCard) CheckRect() Rect {
	if !c.Actionable {
		return Rect{}
	}
	inner := c.innerRect()
	cb := scaled(GroupCheckSize)
	right := inner.X + inner.W
	if ar := c.ActionRect(); ar.W > 0 {
		right = ar.X - scaled(CardPadX)
	}
	return Rect{X: right - cb, Y: inner.Y + (c.headerContentH()-cb)/2, W: cb, H: cb}
}

// MemberRect is the i-th member row's rectangle within the expanded body.
func (c *GroupCard) MemberRect(i int) Rect {
	inner := c.innerRect()
	return Rect{X: c.contentX(inner), Y: inner.Y + c.headerContentH() + i*c.memberH(), W: inner.X + inner.W - c.contentX(inner), H: c.memberH()}
}

// pillInk / statusInk resolve each badge's ink, deriving a readable one from the
// fill when unset (see PostCard.pillInk).
func (c *GroupCard) pillInk() RGBA   { return badgeInk(c.PillInk, c.PillColor) }
func (c *GroupCard) statusInk() RGBA { return badgeInk(c.StatusInk, c.StatusColor) }

// badgeInk returns ink when set, else a readable ink derived from fill, else the
// zero RGBA (letting Badge fall back to the theme background).
func badgeInk(ink, fill RGBA) RGBA {
	if ink.A != 0 {
		return ink
	}
	if fill.A != 0 {
		return readableInk(fill)
	}
	return RGBA{}
}

// assemble (re)builds the card's title / meta / member Labels at the current
// bounds, so Draw paints them and Children / CollectRuns lift them out. The title
// wraps to the space left of the header affordance; the meta and member lines span
// the content column.
func (c *GroupCard) assemble() {
	inner := c.innerRect()
	cx := c.contentX(inner)

	titleRight := inner.X + inner.W
	if cr := c.CheckRect(); cr.W > 0 {
		titleRight = cr.X - scaled(CardPadX)
	}
	titleW := titleRight - cx
	if titleW < 1 {
		titleW = 1
	}

	titleY := inner.Y + c.badgeRowH()
	c.titleLbl = NewLabel(c.Title)
	c.titleLbl.Font = c.titleFont()
	c.titleLbl.Ellipsis = true
	c.titleLbl.SetBounds(Rect{X: cx, Y: titleY, W: titleW, H: c.titleFont().Height()})

	metaY := titleY + c.titleSlot()
	metaW := inner.X + inner.W - cx
	if metaW < 1 {
		metaW = 1
	}
	c.metaLbl = NewLabel(c.Meta)
	c.metaLbl.Font = c.metaFont()
	c.metaLbl.Ellipsis = true
	c.metaLbl.SetBounds(Rect{X: cx, Y: metaY, W: metaW, H: c.metaH()})

	c.memberLbls = nil
	if c.Expanded().Get() {
		for i, mem := range c.Members {
			mr := c.MemberRect(i)
			lbl := NewLabel(mem)
			lbl.Font = c.metaFont()
			lbl.Ellipsis = true
			lbl.VAlign = VMiddle
			lbl.SetBounds(Rect{X: mr.X + scaled(CardPadX), Y: mr.Y, W: mr.W - 2*scaled(CardPadX), H: mr.H})
			c.memberLbls = append(c.memberLbls, lbl)
		}
	}
}

// Children yields the card's selectable Labels in visual order — the title, the
// meta line, then each expanded member line — so CollectRuns lifts them out as
// text runs. Chevron, badges, checkbox and action pill are decoration and are not
// returned. Calling Children re-assembles the tree at the card's current bounds.
func (c *GroupCard) Children() []Widget {
	c.assemble()
	out := make([]Widget, 0, len(c.memberLbls)+2)
	if c.Title != "" {
		out = append(out, c.titleLbl)
	}
	if c.Meta != "" {
		out = append(out, c.metaLbl)
	}
	for _, l := range c.memberLbls {
		out = append(out, l)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Draw paints the card frame, the header (chevron, source + status pills,
// download checkbox + action pill), the title and meta, and — when expanded — the
// divider-separated member rows. Muted inks are theme-derived here, at paint time.
func (c *GroupCard) Draw(p painter.Painter, theme *Theme) {
	inner := cardFrame(p, theme, c.Bounds())
	c.assemble()

	// Chevron, centred in its column.
	chev := c.ChevronRect()
	drawDisclosureChevron(p, chev.X+chev.W/2, chev.Y+chev.H/2, c.Expanded().Get(), theme.OnSurface)

	// Source + status pills on the badge row.
	x := c.contentX(inner)
	badgeY := inner.Y
	if c.Pill != "" {
		bw := c.pillWidth(c.Pill)
		c.drawBadge(p, theme, Rect{X: x, Y: badgeY, W: bw, H: c.badgeRowH()}, c.Pill, c.PillColor, c.pillInk())
		x += bw + scaled(CardGapX)
	}
	if c.Status != "" {
		bw := c.pillWidth(c.Status)
		c.drawBadge(p, theme, Rect{X: x, Y: badgeY, W: bw, H: c.badgeRowH()}, c.Status, c.StatusColor, c.statusInk())
	}

	// Header affordance: download checkbox + action pill.
	if cr := c.CheckRect(); cr.W > 0 {
		cb := &CheckButton{Size: cr.W}
		cb.Checked().Set(c.Checked().Get())
		cb.SetBounds(cr)
		cb.Draw(p, theme)
	}
	if ar := c.ActionRect(); ar.W > 0 {
		fillRoundRect(p, ar.X, ar.Y, ar.W, ar.H, ar.H/2, theme.Accent)
		f := c.pillFont()
		tx := ar.X + (ar.W-f.Measure(c.Action))/2
		ty := ar.Y + (ar.H-f.Height())/2
		f.Draw(p, tx, ty, c.Action, readableInk(theme.Accent))
	}

	// Title + meta.
	c.titleLbl.Ink = theme.OnSurface
	c.titleLbl.Draw(p, theme)
	c.metaLbl.Ink = dimInk(theme)
	c.metaLbl.Draw(p, theme)

	// Member rows: a top divider then the part line.
	for i, lbl := range c.memberLbls {
		mr := c.MemberRect(i)
		fillRect(p, mr.X, mr.Y, mr.W, 1, theme.Border)
		lbl.Ink = theme.OnSurface
		lbl.Draw(p, theme)
	}
}

// drawBadge paints one header pill (source or status) as a Badge.
func (c *GroupCard) drawBadge(p painter.Painter, theme *Theme, r Rect, text string, fill, ink RGBA) {
	b := &Badge{Text: text, Fill: fill, Ink: ink}
	b.Font = c.pillFont()
	b.SetBounds(r)
	b.Draw(p, theme)
}

// A11y reports the card as a group named by its title.
func (c *GroupCard) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: c.Title}
}

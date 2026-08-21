// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// markdownEditorDividerW is the pixel thickness of the 1-px divider Draw
// paints between the source pane and the preview pane.
const markdownEditorDividerW = 1

// MarkdownEditor is a split source/preview editor: a TextView holding
// editable Markdown source on one side, a MarkdownView rendering that
// source live on the other. It composes the two existing widgets purely
// through their public API (TextView.Text/SetText, MarkdownView.Source) --
// it does not modify either.
//
// Only the Source pane is interactive; a click landing in the Preview pane
// is a no-op (the preview is display-only, per MarkdownView's own docs).
// Every event routed to Source is followed by a re-sync of Preview.Source
// so the rendered pane never drifts from the edited buffer.
type MarkdownEditor struct {
	Base

	// Source is the editable Markdown-source pane.
	Source *TextView
	// Preview is the read-only rendered pane, kept in sync with Source.
	Preview *MarkdownView

	// splitFrac (fraction 0..1 of Bounds given to Source along the split axis;
	// a value outside (0,1), including the zero value, falls back to 0.5 via
	// split()) and sideBySide (true = Source left / Preview right; false =
	// stacked) are reactive view config behind the Split() / SideBySide()
	// accessors.
	splitFrac  *mvvm.Observable[float64]
	sideBySide *mvvm.Observable[bool]
}

// Split is the fraction (0..1, exclusive) of Bounds given to the Source pane
// along the split axis, as a shared [mvvm.Observable]; a value outside (0,1)
// falls back to 0.5. Lazily created, defaulting to 0 (so split() yields 0.5).
func (m *MarkdownEditor) Split() *mvvm.Observable[float64] {
	if m.splitFrac == nil {
		m.splitFrac = mvvm.NewObservable(0.0)
	}
	return m.splitFrac
}

// SideBySide selects the split axis as a shared [mvvm.Observable]: true lays
// Source left / Preview right, false stacks Source above Preview. Lazily
// created, defaulting to false (stacked); NewMarkdownEditor starts it true.
func (m *MarkdownEditor) SideBySide() *mvvm.Observable[bool] {
	if m.sideBySide == nil {
		m.sideBySide = mvvm.NewObservable(false)
	}
	return m.sideBySide
}

// NewMarkdownEditor builds a MarkdownEditor seeded with initial Markdown
// source: Source gets a TextView pre-loaded with initial (an empty string
// still yields a valid single-line buffer, per NewTextView), Preview gets a
// MarkdownView already rendering that same text. Split defaults to 0.5 and
// SideBySide defaults to true (left/right).
func NewMarkdownEditor(initial string) *MarkdownEditor {
	m := &MarkdownEditor{
		Source:  NewTextView(initial),
		Preview: NewMarkdownView(initial),
	}
	m.Split().Set(0.5)
	m.SideBySide().Set(true)
	return m
}

// Text returns the current source-pane text, or "" when Source is nil.
func (m *MarkdownEditor) Text() string {
	if m.Source == nil {
		return ""
	}
	return m.Source.Text().Get()
}

// SetText replaces the source-pane text + resyncs Preview.
func (m *MarkdownEditor) SetText(s string) {
	if m.Source != nil {
		m.Source.SetText(s)
	}
	m.syncPreview()
}

// syncPreview re-feeds Source's current text into Preview.Source, which is
// MarkdownView's exported (constructor-only-otherwise) text field -- it
// re-parses from it on every Draw, so setting it is the public re-set path.
// A nil Source or Preview makes this a no-op.
func (m *MarkdownEditor) syncPreview() {
	if m.Source == nil || m.Preview == nil {
		return
	}
	m.Preview.Source = m.Source.Text().Get()
}

// split resolves the effective split fraction, defaulting to 0.5 for any
// value outside the open interval (0, 1).
func (m *MarkdownEditor) split() float64 {
	if m.Split().Get() <= 0 || m.Split().Get() >= 1 {
		return 0.5
	}
	return m.Split().Get()
}

// sourceRect is Source's placement within Bounds: the leading share (by
// split fraction) along the SideBySide axis.
func (m *MarkdownEditor) sourceRect() Rect {
	r := m.Bounds()
	frac := m.split()
	if m.SideBySide().Get() {
		return Rect{X: r.X, Y: r.Y, W: int(float64(r.W) * frac), H: r.H}
	}
	return Rect{X: r.X, Y: r.Y, W: r.W, H: int(float64(r.H) * frac)}
}

// previewRect is Preview's placement: the trailing share, offset past the
// divider.
func (m *MarkdownEditor) previewRect() Rect {
	r := m.Bounds()
	sr := m.sourceRect()
	if m.SideBySide().Get() {
		x := sr.X + sr.W + markdownEditorDividerW
		return Rect{X: x, Y: r.Y, W: r.X + r.W - x, H: r.H}
	}
	y := sr.Y + sr.H + markdownEditorDividerW
	return Rect{X: r.X, Y: y, W: r.W, H: r.Y + r.H - y}
}

// Draw lays out + paints Source and Preview side by side (or stacked),
// split per Split/SideBySide, with a markdownEditorDividerW-px divider
// between them.
func (m *MarkdownEditor) Draw(p painter.Painter, theme *Theme) {
	r := m.Bounds()
	sr, pr := m.sourceRect(), m.previewRect()
	if m.Source != nil {
		m.Source.SetBounds(sr)
		m.Source.Draw(p, theme)
	}
	if m.Preview != nil {
		m.Preview.SetBounds(pr)
		m.Preview.Draw(p, theme)
	}
	if m.SideBySide().Get() {
		fillRect(p, sr.X+sr.W, r.Y, markdownEditorDividerW, r.H, theme.Border)
	} else {
		fillRect(p, r.X, sr.Y+sr.H, r.W, markdownEditorDividerW, theme.Border)
	}
}

// OnEvent routes every event to Source, translated into its local frame,
// then resyncs Preview. Only EventClick is hit-tested against the pane
// split first: a click landing in the Preview pane (or on the divider) is
// a no-op, since Preview is display-only and nothing there should steal
// focus from Source. Non-click events (keyboard, IME, drag) always go to
// Source, matching the "only Source is interactive" model -- there is no
// competing widget that could plausibly want them instead.
func (m *MarkdownEditor) OnEvent(ev Event) {
	if m.Source == nil {
		return
	}
	r := m.Bounds()
	sr := m.sourceRect()
	m.Source.SetBounds(sr)
	if ev.Kind == EventClick {
		sx, sy := ev.X+r.X, ev.Y+r.Y
		if !sr.Contains(sx, sy) {
			return
		}
	}
	m.Source.OnEvent(translateEvent(ev, r, sr))
	m.syncPreview()
}

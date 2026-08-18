// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"

	"github.com/go-widgets/painter"
)

// LogLevel is the severity of a log entry, which selects the ink its message is
// painted in. The four levels mirror the semantic tones the rest of the toolkit
// already uses (see Alert / Toast): Warn is amber and Error is brick red — the
// two that must jump out of a scrolling console — while Info paints in the plain
// OnSurface ink and Debug in the dim, subordinate tone. Any out-of-range value
// falls back to Info's neutral ink.
type LogLevel int

const (
	// LogDebug is diagnostic noise a developer opts into: painted dim so it
	// recedes behind the entries that matter.
	LogDebug LogLevel = iota
	// LogInfo is a normal progress line ("compile started"): the neutral
	// OnSurface ink.
	LogInfo
	// LogWarn flags a non-fatal problem ("deprecated \\usepackage"): amber, the
	// same tone Alert/Toast use for a warning.
	LogWarn
	// LogError flags a failure the user must address ("undefined control
	// sequence"): brick red, matching Alert/Toast's error tone.
	LogError
)

// LogView is a scrollable console panel: a vertical list of timestamped log
// entries, newest at the BOTTOM, that accumulates scrollback history. It is the
// widget a diagnostics/output tab is built on — a compiler log, a build console,
// a request trace — where lines arrive over time and the user scrolls back
// through what has already been printed.
//
// Each entry carries a HOST-FORMATTED timestamp string, a LogLevel, and a
// message. The widget never reads the clock itself (no time.Now / time.Time),
// so its rendering is fully deterministic and testable: the host formats the
// time however it likes (a browser locale string, a monotonic build clock, …)
// and hands the finished text to Append. The timestamp is painted in a dim ink
// at the start of the row; the message follows in the level's ink. A message
// containing embedded newlines is rendered as one row per line, the
// continuation rows hanging under the message column (never clipped to a single
// line).
//
// Scrolling is delegated to an embedded ScrollView, so the panel gets vertical
// AND horizontal scrolling, a wheel, drag/touch panning with momentum, and
// scrollbar thumbs for free — a long line scrolls sideways, a tall history
// scrolls up. History is bounded by MaxEntries: with a positive bound the oldest
// entries are dropped once the count exceeds it; 0 (the default) keeps
// everything.
//
// Auto-follow: Append scrolls to the newest entry ONLY when the view is already
// pinned to the bottom. A user who has scrolled up to read history is therefore
// never yanked back down by an incoming line, while a user watching the tail
// keeps seeing the newest entry — the behaviour every good log console has.
//
// LogView implements Widget (Draw + OnEvent for wheel / drag / key scrolling,
// Bounds / SetBounds / HitTest via Base). Its reactive history is internal — a
// host feeds it through Append / Clear and reads it through Len — so there is no
// host-settable state field; MaxEntries is set-once bounding config.
type LogView struct {
	Base

	// MaxEntries bounds the retained scrollback. When > 0, Append drops the
	// oldest entries so the history never exceeds MaxEntries; when 0 (the zero
	// value) the history is unbounded. It is set-once bounding config, not
	// reactive state — the entries themselves live in the unexported history
	// below, mutated only through Append / Clear.
	MaxEntries int

	// entries is the accumulated history, oldest first; newest is drawn at the
	// bottom. rows is its flattened, per-visual-line form (one message line per
	// row, timestamps attached to each entry's first row), rebuilt on every
	// mutation so Draw only ever iterates precomputed rows.
	entries []logEntry
	rows    []logRow

	// sv is the viewport that scrolls the history; content is the child it
	// paints, a thin adapter back onto this LogView's rows. Both are created
	// lazily (see ensure) so a bare &LogView{} works without a constructor.
	sv      *ScrollView
	content *logContent
}

// logEntry is one appended record: a host-formatted timestamp, a severity, and
// the message already split into visual lines on "\n" (always at least one).
type logEntry struct {
	timestamp string
	level     LogLevel
	lines     []string
}

// logRow is one flattened visual line ready to paint: the entry's level + this
// line's text, whether it is the entry's FIRST line (which carries the
// timestamp), and the device-pixel x the message starts at (past the timestamp
// column, shared by every line of the entry so continuations hang under the
// message).
type logRow struct {
	timestamp string
	level     LogLevel
	text      string
	first     bool
	msgX      int
}

// Log-panel layout metrics, in LOGICAL pixels (scaled to device pixels via
// [scaled] at use). logPadX is the left/right content inset, logGap the space
// between the timestamp column and the message, logRowPadY the vertical padding
// added to the glyph height to form a comfortable row pitch.
const (
	logPadX    = 6
	logGap     = 6
	logRowPadY = 4
)

// NewLogView builds an empty, unbounded LogView. Set MaxEntries afterwards to
// bound the scrollback. Bounds start zero; the host sizes the panel via
// SetBounds like any other widget.
func NewLogView() *LogView {
	lv := &LogView{}
	lv.ensure()
	return lv
}

// ensure lazily wires the viewport + its content child, so a bare &LogView{}
// (no constructor) is usable: the first Append / Draw / OnEvent / SetBounds
// builds them. Subsequent calls are a cheap no-op.
func (lv *LogView) ensure() {
	if lv.sv != nil {
		return
	}
	lv.content = &logContent{lv: lv}
	lv.sv = NewScrollView(lv.content)
	lv.sv.SetBounds(lv.Bounds())
	lv.syncContent()
}

// SetBounds positions the panel and forwards the same rectangle to the embedded
// viewport, so the scrollable region always matches the widget's footprint.
func (lv *LogView) SetBounds(r Rect) {
	lv.Base.SetBounds(r)
	lv.ensure()
	lv.sv.SetBounds(r)
}

// Len reports how many entries the history currently holds (after any
// MaxEntries eviction). A multi-line message counts as one entry.
func (lv *LogView) Len() int { return len(lv.entries) }

// Append adds one entry — a host-formatted timestamp, a severity level, and a
// message (embedded newlines become separate rows) — to the bottom of the
// history. When MaxEntries is exceeded the oldest entries are dropped. The view
// scrolls to the new entry ONLY if it was already pinned to the bottom, so a
// user reading scrollback is not yanked down.
func (lv *LogView) Append(timestamp string, level LogLevel, message string) {
	lv.ensure()
	atBottom := lv.atBottom()
	lv.entries = append(lv.entries, logEntry{
		timestamp: timestamp,
		level:     level,
		lines:     strings.Split(message, "\n"),
	})
	lv.evict()
	lv.syncContent()
	if atBottom {
		lv.scrollToBottom()
	}
}

// Clear empties the history and returns the viewport to the top-left.
func (lv *LogView) Clear() {
	lv.ensure()
	lv.entries = nil
	lv.syncContent()
	lv.sv.OffsetX, lv.sv.OffsetY = 0, 0
}

// evict drops the oldest entries once the history exceeds a positive MaxEntries.
// A non-positive MaxEntries means "unbounded" and never evicts.
func (lv *LogView) evict() {
	if lv.MaxEntries <= 0 {
		return
	}
	if len(lv.entries) > lv.MaxEntries {
		lv.entries = lv.entries[len(lv.entries)-lv.MaxEntries:]
	}
}

// atBottom reports whether the viewport is pinned to the last entry — the
// condition under which Append follows the tail. A history that fits (max scroll
// 0) is always "at bottom", so the first entries auto-follow.
func (lv *LogView) atBottom() bool {
	return lv.sv.OffsetY >= lv.sv.maxOffsetY()
}

// scrollToBottom pins the viewport to the newest entry. Scroll clamps the
// overshoot to the real maximum.
func (lv *LogView) scrollToBottom() { lv.sv.Scroll(0, scrollExtreme) }

// rowHeight is the device-pixel pitch of one visual line: the effective font's
// glyph height plus the scaled vertical padding.
func (lv *LogView) rowHeight() int { return lv.glyphHeight() + scaled(logRowPadY) }

// syncContent rebuilds the flattened rows from the entries and republishes the
// content's logical size to the viewport, so the scrollbars size correctly and
// scrolling is clamped. Called after every mutation.
func (lv *LogView) syncContent() {
	pad := scaled(logPadX)
	gap := scaled(logGap)
	rows := make([]logRow, 0, len(lv.entries))
	maxW := 0
	for _, e := range lv.entries {
		msgX := pad + lv.textWidth(e.timestamp) + gap
		for li, line := range e.lines {
			row := logRow{level: e.level, text: line, first: li == 0, msgX: msgX}
			if row.first {
				row.timestamp = e.timestamp
			}
			rows = append(rows, row)
			if w := msgX + lv.textWidth(line); w > maxW {
				maxW = w
			}
		}
	}
	lv.rows = rows
	contentW := maxW + pad
	contentH := len(rows) * lv.rowHeight()
	lv.content.SetBounds(Rect{W: contentW, H: contentH})
	lv.sv.SetContentSize(contentW, contentH)
}

// Draw paints the console: a Surface ground, the scrolled rows (via the embedded
// viewport, which clips + translates them and paints the scrollbars), then a
// border on top so it frames the panel cleanly.
func (lv *LogView) Draw(p painter.Painter, theme *Theme) {
	lv.ensure()
	r := lv.Bounds()
	lv.sv.SetBounds(r)
	fillRect(p, r.X, r.Y, r.W, r.H, theme.Surface)
	lv.sv.Draw(p, theme)
	strokeRect(p, r.X, r.Y, r.W, r.H, theme.Border)
}

// drawRows paints the visible window of rows into r (the content child's
// rectangle, whose origin the viewport has set to the panel's top-left and whose
// size is the logical content extent). Only the rows overlapping the scrolled
// viewport are drawn — the history may be far taller than the screen — and the
// viewport's translate places each at the right on-screen position.
func (lv *LogView) drawRows(p painter.Painter, theme *Theme, r Rect) {
	rowH := lv.rowHeight()
	off := lv.sv.OffsetY
	vpH := lv.sv.viewport().H
	first := off / rowH
	last := (off+vpH)/rowH + 1
	if last > len(lv.rows) {
		last = len(lv.rows)
	}
	pad := scaled(logPadX)
	for i := first; i < last; i++ {
		row := lv.rows[i]
		top := r.Y + i*rowH
		textY := top + (rowH-lv.glyphHeight())/2
		if row.first {
			lv.drawText(p, r.X+pad, textY, row.timestamp, dimInk(theme))
		}
		lv.drawText(p, r.X+row.msgX, textY, row.text, levelInk(row.level, theme))
	}
}

// OnEvent forwards input to the embedded viewport, giving the panel native
// wheel, drag/touch and keyboard (arrows / page / home / end) scrolling.
func (lv *LogView) OnEvent(ev Event) {
	lv.ensure()
	lv.sv.OnEvent(ev)
}

// levelInk maps a severity to its message ink. Warn/Error carry the toolkit's
// semantic amber/red (the same shades Alert + Toast use); Info is the neutral
// OnSurface ink and Debug the dim subordinate tone. An out-of-range level falls
// back to Info's neutral ink.
func levelInk(level LogLevel, theme *Theme) RGBA {
	switch level {
	case LogWarn:
		return RGB(0xE0, 0xA0, 0x30) // amber — matches AlertWarning
	case LogError:
		return RGB(0xC0, 0x30, 0x30) // brick red — matches AlertError
	case LogDebug:
		return dimInk(theme)
	default: // LogInfo (and any out-of-range level)
		return theme.OnSurface
	}
}

// RoleLog is the ARIA role="log" a LogView reports: a live region where new
// entries are appended and older ones are retained, which a screen reader
// announces incrementally as they arrive.
const RoleLog Role = "log"

// A11y reports the panel as an ARIA log region, its Value the current entry
// count — mirroring the "N events" reading Timeline gives its list.
func (lv *LogView) A11y() A11yInfo {
	return A11yInfo{Role: RoleLog, Value: strconv.Itoa(lv.Len()) + " entries"}
}

// logContent is the viewport's child: a thin adapter that paints its parent
// LogView's rows. It owns no state of its own beyond the Base rectangle the
// viewport sizes to the logical content extent.
type logContent struct {
	Base
	lv *LogView
}

// Draw defers to the LogView, handing it the content rectangle so the rows land
// at the viewport's origin (before the viewport's own scroll translate).
func (c *logContent) Draw(p painter.Painter, theme *Theme) {
	c.lv.drawRows(p, theme, c.Bounds())
}

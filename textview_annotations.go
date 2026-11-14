// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// Line annotations: a comment attached to a line of a TextView / CodeEditor.
//
// The model is [TextView.Annotations], an MVVM list that is the single source of
// truth: the gutter markers, the minimap ticks, the accessibility nodes and the
// panel are all read from it, and a host that wants to persist or sync comments
// subscribes to it. An annotation is ANCHORED: its Line follows the text it was
// written on through edits (see reanchorAnnotations), instead of staying at a
// fixed index.
//
// Shape, after VS Code's comment threads (a gutter affordance that opens a
// widget under the line) and CodeMirror's gutter markers + block widgets (a
// marker per line in a gutter, the widget a view-state decoration), reduced to
// what a single-line-at-a-time editor needs: one comment per line, plain text.
//
// The text is untrusted. It is plain text only (drawn with the font, never
// parsed), control characters are dropped, and the count and length are bounded
// by [MaxAnnotations] and [MaxAnnotationRunes].

const (
	// MaxAnnotations bounds how many annotations a view shows and exposes.
	MaxAnnotations = 256
	// MaxAnnotationRunes bounds the length of one annotation, in runes.
	MaxAnnotationRunes = 2000
)

// Annotation is a comment on one line.
type Annotation struct {
	// ID identifies the annotation across edits (assigned by SetAnnotation); a
	// host that appends to the list directly may leave it 0.
	ID int
	// Line is the 0-based buffer line. It is kept on its text across edits.
	Line int
	// Text is the comment, plain text; "\n" separates its lines.
	Text string
}

// SanitizeAnnotation makes s a bounded plain-text comment: control characters
// other than "\n" are dropped (a tab becomes a space), invalid UTF-8 is replaced
// and the result is cut to [MaxAnnotationRunes].
func SanitizeAnnotation(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= MaxAnnotationRunes {
			break
		}
		switch {
		case r == '\t':
			r = ' '
		case r == '\r':
			continue
		case r < 0x20 && r != '\n', r == 0x7f, r >= 0x80 && r < 0xa0:
			continue
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// annotation panel + gutter state.
type annState struct {
	list    *mvvm.ObservableList[Annotation]
	nextID  int
	prev    []string // the buffer the annotations were last anchored against
	open    bool     // the panel is showing
	openAt  int      // its line
	draft   string   // the text being edited
	markers []*annotationMarker
}

// EnableAnnotations turns the annotation gutter on: a column left of the line
// numbers holding one marker per annotated line, a click on it (or
// Ctrl+Shift+M for the caret's line) opening the panel. Off by default.
func (t *TextView) EnableAnnotations(on bool) { t.annOn = on }

// AnnotationsEnabled reports whether the annotation gutter is on.
func (t *TextView) AnnotationsEnabled() bool { return t.annOn }

// Annotations is the source of truth: the list of annotations, as an MVVM
// ObservableList. Mutating it (Append, Set, RemoveAt) is how a host loads or
// edits comments; the view repaints from it. SetAnnotation and DeleteAnnotation
// keep the one-per-line, bounded and sanitised shape.
func (t *TextView) Annotations() *mvvm.ObservableList[Annotation] {
	if t.ann.list == nil {
		t.ann.list = mvvm.NewObservableList[Annotation]()
		t.ann.list.SubscribeChanged(t.annSnapshot)
	}
	return t.ann.list
}

// AnnotationAt is the annotation on line, and whether there is one.
func (t *TextView) AnnotationAt(line int) (Annotation, bool) {
	if t.ann.list == nil {
		return Annotation{}, false
	}
	for i := 0; i < t.ann.list.Len() && i < MaxAnnotations; i++ {
		if a := t.ann.list.At(i); a.Line == line {
			a.Text = SanitizeAnnotation(a.Text)
			return a, true
		}
	}
	return Annotation{}, false
}

// SetAnnotation adds the annotation on line or replaces its text; empty text
// (after sanitising) deletes it. A line outside the buffer, or an add past
// [MaxAnnotations], is refused (false).
func (t *TextView) SetAnnotation(line int, text string) bool {
	if line < 0 || line >= len(t.lines) {
		return false
	}
	text = SanitizeAnnotation(text)
	l := t.Annotations()
	for i := 0; i < l.Len(); i++ {
		if a := l.At(i); a.Line == line {
			if strings.TrimSpace(text) == "" {
				l.RemoveAt(i)
			} else {
				a.Text = text
				l.Set(i, a)
			}
			t.annSnapshot()
			return true
		}
	}
	if strings.TrimSpace(text) == "" {
		return true
	}
	if l.Len() >= MaxAnnotations {
		return false
	}
	t.ann.nextID++
	l.Append(Annotation{ID: t.ann.nextID, Line: line, Text: text})
	t.annSnapshot()
	return true
}

// DeleteAnnotation removes the annotation on line.
func (t *TextView) DeleteAnnotation(line int) { t.SetAnnotation(line, "") }

// OpenAnnotation opens the panel on line to read, add or edit its comment. The
// view takes the keyboard for the panel: typed text edits it, Enter commits
// (empty deletes), Escape cancels.
func (t *TextView) OpenAnnotation(line int) {
	if !t.annOn || t.Disabled().Get() || line < 0 || line >= len(t.lines) {
		return
	}
	t.ann.open, t.ann.openAt = true, line
	t.ann.draft = ""
	if a, ok := t.AnnotationAt(line); ok {
		t.ann.draft = a.Text
	}
	t.refreshPreferredHeight()
}

// CloseAnnotation closes the panel, keeping the stored comment as it was.
func (t *TextView) CloseAnnotation() {
	t.ann.open = false
	t.refreshPreferredHeight()
}

// AnnotationOpen reports the line the panel is open on, if it is.
func (t *TextView) AnnotationOpen() (line int, open bool) { return t.ann.openAt, t.ann.open }

// annSnapshot records the buffer the annotations are anchored to.
func (t *TextView) annSnapshot() {
	t.ann.prev = append(t.ann.prev[:0], t.lines...)
}

// reanchorAnnotations moves every annotation to follow its text after an edit.
// The old and new buffers share a prefix and a suffix of unchanged lines; a
// line in the suffix shifts by the change in line count, a line in the changed
// middle stays at the same offset into it (an edited line keeps its comment),
// and one whose line was deleted settles on the last line of the middle, or on
// the line now standing where it was. No comment is lost to an edit.
func (t *TextView) reanchorAnnotations() {
	l := t.ann.list
	if l == nil || l.Len() == 0 {
		t.ann.prev = nil
		return
	}
	old := t.ann.prev
	if old == nil {
		t.annSnapshot()
		return
	}
	nw := t.lines
	pre := 0
	for pre < len(old) && pre < len(nw) && old[pre] == nw[pre] {
		pre++
	}
	suf := 0
	for suf < len(old)-pre && suf < len(nw)-pre && old[len(old)-1-suf] == nw[len(nw)-1-suf] {
		suf++
	}
	newMid := len(nw) - pre - suf
	delta := len(nw) - len(old)
	moved := make([]Annotation, l.Len())
	changed := false
	for i := range moved {
		a := l.At(i)
		switch {
		case a.Line < pre:
		case a.Line >= len(old)-suf:
			a.Line += delta
		default:
			k := a.Line - pre
			if k >= newMid {
				k = newMid - 1
			}
			a.Line = pre + max(k, 0)
		}
		a.Line = max(0, min(a.Line, len(nw)-1))
		if a != l.At(i) {
			changed = true
		}
		moved[i] = a
	}
	t.annSnapshot()
	if !changed {
		return
	}
	for i, a := range moved {
		l.Set(i, a) // the subscription re-takes the snapshot, which is already current
	}
}

// annGutter is the width of the annotation column, 0 when annotations are off.
func (t *TextView) annGutter() int {
	if !t.annOn {
		return 0
	}
	return scaled(14)
}

// annotationMarker is the gutter marker of one annotated line: a focusable
// button in the accessibility tree named after its line, whose value is the
// comment, so a screen reader reads the comment and the mirror carries it.
type annotationMarker struct {
	Base
	focusState
	tv   *TextView
	line int
	id   int
	text string
}

// A11yID keeps the marker's accessibility key stable while lines shift.
func (m *annotationMarker) A11yID() string {
	if m.id != 0 {
		return "annotation-" + strconv.Itoa(m.id)
	}
	return "annotation-line-" + strconv.Itoa(m.line)
}

// A11y reports the marker as a button "Comment on line N" carrying the comment.
func (m *annotationMarker) A11y() A11yInfo {
	return A11yInfo{
		Role:     RoleButton,
		Name:     "Comment on line " + strconv.Itoa(m.line+1),
		Value:    m.text,
		Focused:  m.focused,
		Disabled: m.tv.Disabled().Get(),
	}
}

// OnEvent opens the panel on a click or Enter / Space.
func (m *annotationMarker) OnEvent(ev Event) {
	if m.tv.Disabled().Get() {
		return
	}
	if ev.Kind == EventClick || (ev.Kind == EventKeyDown && (ev.Code == "Enter" || ev.Code == " ")) {
		m.tv.OpenAnnotation(m.line)
	}
}

// Draw paints nothing: the TextView draws the marker in its gutter; this widget
// is the accessibility and focus face of it.
func (m *annotationMarker) Draw(painter.Painter, *Theme) {}

// Children exposes the annotation markers of the lines on show, so the
// accessibility walk (and with it the webcanvas mirror) reaches them.
func (t *TextView) Children() []Widget {
	if !t.annOn || t.ann.list == nil || t.ann.list.Len() == 0 {
		return nil
	}
	r := t.Bounds()
	start := t.clampedScrollLine()
	lineH := t.glyphHeight() + lineGap()
	t.ann.markers = t.ann.markers[:0]
	var out []Widget
	for i := 0; i < t.ann.list.Len() && i < MaxAnnotations; i++ {
		a := t.ann.list.At(i)
		if a.Line < start || a.Line >= len(t.lines) {
			continue
		}
		y := r.Y + textInset() + (a.Line-start)*lineH
		if y+lineH > r.Y+r.H {
			continue
		}
		m := &annotationMarker{tv: t, line: a.Line, id: a.ID, text: SanitizeAnnotation(a.Text)}
		m.SetBounds(Rect{X: r.X + 1, Y: y - lineGap()/2, W: t.annGutter(), H: lineH})
		t.ann.markers = append(t.ann.markers, m)
		out = append(out, m)
	}
	return out
}

// drawAnnotationGutter paints a marker on each annotated line on show, and the
// panel when it is open.
func (t *TextView) drawAnnotationGutter(p painter.Painter, theme *Theme, start, lineH int) {
	if !t.annOn || t.ann.list == nil {
		return
	}
	r := t.Bounds()
	ink := minimapAccent(theme)
	for i := 0; i < t.ann.list.Len() && i < MaxAnnotations; i++ {
		a := t.ann.list.At(i)
		if a.Line < start || a.Line >= len(t.lines) {
			continue
		}
		y := r.Y + textInset() + (a.Line-start)*lineH
		if y >= r.Y+r.H {
			continue
		}
		// A speech-bubble: a filled square with a notch, in the accent.
		s := t.glyphHeight() - scaled(1)
		x := r.X + scaled(3)
		fillRect(p, x, y, s, s, ink)
		fillRect(p, x+scaled(1), y+s, scaled(2), scaled(2), ink)
	}
}

// drawAnnotationPanel paints the open panel under its line (above it when there
// is no room), over the text: the comment as it is being edited, with a caret
// and the key hints.
func (t *TextView) drawAnnotationPanel(p painter.Painter, theme *Theme, start, lineH int) {
	if !t.ann.open {
		return
	}
	b := t.Bounds()
	rc := t.annPanelRect(start, lineH)
	rc.X, rc.Y = rc.X+b.X, rc.Y+b.Y
	fillRect(p, rc.X, rc.Y, rc.W, rc.H, theme.SurfaceAlt)
	strokeRect(p, rc.X, rc.Y, rc.W, rc.H, minimapAccent(theme))
	pad := scaled(4)
	rows := t.annPanelRows(rc.W - 2*pad)
	y := rc.Y + pad
	for _, row := range rows {
		t.drawText(p, rc.X+pad, y, row, theme.OnSurface)
		y += t.glyphHeight() + scaled(2)
	}
	hint := "Enter: save   Esc: cancel   empty: delete"
	t.drawText(p, rc.X+pad, y, hint, dimInk(theme))
}

// annPanelRows is the draft laid out in lines that fit width, with a caret bar
// at its end, at most annPanelMaxRows rows.
func (t *TextView) annPanelRows(width int) []string {
	const maxRows = 6
	var rows []string
	for _, para := range strings.Split(t.ann.draft+"|", "\n") {
		cur := ""
		for _, r := range para {
			if cur != "" && t.textWidth(cur+string(r)) > width {
				rows = append(rows, cur)
				cur = ""
			}
			cur += string(r)
		}
		rows = append(rows, cur)
	}
	if len(rows) > maxRows {
		rows = rows[len(rows)-maxRows:] // the end, where the caret is
	}
	return rows
}

// annPanelRect is the panel's rectangle in widget-local pixels.
func (t *TextView) annPanelRect(start, lineH int) Rect {
	r := t.Bounds()
	x := t.annGutter() + scaled(4)
	w := t.annPanelWidth(r.W)
	if w != r.W-x-scaled(8) {
		x = 1
	}
	h := t.annPanelHeight(w)
	y := textInset() + (t.ann.openAt-start+1)*lineH
	if y+h > r.H {
		y = max(r.H-h, 0) // the room PreferredHeight reserves makes this rare
	}
	return Rect{X: x, Y: y, W: w, H: h}
}

// annPanelWidth is the panel's width for a view of width w.
func (t *TextView) annPanelWidth(w int) int {
	x := t.annGutter() + scaled(4)
	if pw := w - x - scaled(8); pw >= scaled(120) {
		return pw
	}
	return max(w-2, 1)
}

// annPanelHeight is the panel's height at width w: the draft's rows and the
// hint row.
func (t *TextView) annPanelHeight(w int) int {
	rows := len(t.annPanelRows(w - 2*scaled(4)))
	return scaled(4)*2 + (rows+1)*(t.glyphHeight()+scaled(2))
}

// annPanelExtra is the height PreferredHeight adds while the panel is open, so
// the panel under the last line is not cut off by a view sized to its text.
func (t *TextView) annPanelExtra() int {
	if !t.ann.open {
		return 0
	}
	return t.annPanelHeight(t.annPanelWidth(t.Bounds().W)) + lineGap()
}

// annEvent handles an event while the panel is open; true when it consumed it.
func (t *TextView) annEvent(ev Event) bool {
	if !t.ann.open {
		return false
	}
	switch ev.Kind {
	case EventChar:
		if utf8.RuneCountInString(t.ann.draft) < MaxAnnotationRunes {
			t.ann.draft = SanitizeAnnotation(t.ann.draft + ev.Code)
		}
		t.refreshPreferredHeight()
		return true
	case EventKeyDown:
		switch ev.Code {
		case "Enter":
			t.SetAnnotation(t.ann.openAt, t.ann.draft)
			t.ann.open = false
		case "Escape":
			t.ann.open = false
		case "Backspace":
			if _, n := utf8.DecodeLastRuneInString(t.ann.draft); n > 0 {
				t.ann.draft = t.ann.draft[:len(t.ann.draft)-n]
			}
		case "Ctrl+V":
			t.ann.draft = SanitizeAnnotation(t.ann.draft + ClipboardText())
		}
		t.refreshPreferredHeight() // Enter and Escape close the panel, the others resize it
		return true
	case EventClick:
		rc := t.annPanelRect(t.clampedScrollLine(), t.glyphHeight()+lineGap())
		if ev.X >= rc.X && ev.X < rc.X+rc.W && ev.Y >= rc.Y && ev.Y < rc.Y+rc.H {
			return true // a click in the panel stays in it
		}
		t.ann.open = false // a click elsewhere closes it, then acts as usual
		t.refreshPreferredHeight()
	}
	return false
}

// annGutterPress opens the panel when the press landed in the annotation
// column. True when it did.
func (t *TextView) annGutterPress(ev Event) bool {
	if !t.annOn || ev.X < 0 || ev.X >= t.annGutter() || ev.Y < textInset() {
		return false
	}
	lineH := t.glyphHeight() + lineGap()
	line := (ev.Y-textInset())/lineH + t.clampedScrollLine()
	if line >= len(t.lines) {
		return false
	}
	t.OpenAnnotation(line)
	return true
}

// annotationMinimapMarks are the minimap ticks of the annotated lines.
func (t *TextView) annotationMinimapMarks(dst []MinimapMark) []MinimapMark {
	if !t.annOn || t.ann.list == nil {
		return dst
	}
	for i := 0; i < t.ann.list.Len() && i < MaxAnnotations; i++ {
		dst = append(dst, MinimapMark{Line: t.ann.list.At(i).Line, Kind: MarkMatch})
	}
	return dst
}

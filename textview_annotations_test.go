// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

func annEditor(text string) *CodeEditor {
	c := NewCodeEditor(text)
	c.NoVerticalScroll = true
	c.EnableAnnotations(true)
	c.SetBounds(Rect{W: 600, H: 400})
	return c
}

func lineOf(c *CodeEditor, ann string) int {
	for i := 0; i < c.Annotations().Len(); i++ {
		if a := c.Annotations().At(i); a.Text == ann {
			return a.Line
		}
	}
	return -1
}

func TestAnnotationFollowsItsLineThroughEdits(t *testing.T) {
	c := annEditor("a\nb\nc\nd")
	c.SetAnnotation(2, "on c")
	// Enter at the start of line 0 pushes everything down.
	c.SetInputSelection(0, 0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got := lineOf(c, "on c"); got != 3 {
		t.Fatalf("after a line inserted above, the comment is on line %d, want 3", got)
	}
	// Typing on the commented line keeps it.
	c.CursorLine().Set(3)
	c.CursorCol().Set(1)
	c.OnEvent(Event{Kind: EventChar, Code: "x"})
	if got := lineOf(c, "on c"); got != 3 {
		t.Fatalf("after typing on its line, comment on %d, want 3", got)
	}
	// Splitting the commented line keeps the comment on its first half.
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if got := lineOf(c, "on c"); got != 3 {
		t.Fatalf("after splitting its line, comment on %d, want 3", got)
	}
	// Deleting a line above moves it up.
	c.CursorLine().Set(1)
	c.CursorCol().Set(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"}) // joins line 1 into 0
	if got := lineOf(c, "on c"); got != 2 {
		t.Fatalf("after a line removed above, comment on %d, want 2", got)
	}
	// A host replacing the text keeps comments on the lines that are still there.
	c.Text().Set(strings.Join(c.lines, "\n") + "\ntail")
	if got := lineOf(c, "on c"); got != 2 {
		t.Fatalf("after an append, comment on %d, want 2", got)
	}
}

func TestDeletingACommentedLineKeepsItsComment(t *testing.T) {
	c := annEditor("a\nb\nc")
	c.SetAnnotation(1, "keep")
	c.SetText("a\nc")
	if n := c.Annotations().Len(); n != 1 {
		t.Fatalf("%d annotations, want 1: an edit must not lose a comment", n)
	}
	if got := lineOf(c, "keep"); got < 0 || got > 1 {
		t.Fatalf("comment on line %d, outside the buffer", got)
	}
}

func TestAnnotationAPIAndBounds(t *testing.T) {
	c := annEditor("a\nb")
	if !c.SetAnnotation(0, "one") || !c.SetAnnotation(0, "two") {
		t.Fatal("set refused")
	}
	if c.Annotations().Len() != 1 {
		t.Fatal("one comment per line: a second set replaces")
	}
	if a, _ := c.AnnotationAt(0); a.Text != "two" {
		t.Fatalf("text %q", a.Text)
	}
	if c.SetAnnotation(5, "x") {
		t.Fatal("a line outside the buffer must be refused")
	}
	c.DeleteAnnotation(0)
	if c.Annotations().Len() != 0 {
		t.Fatal("delete")
	}
	long := strings.Repeat("é", MaxAnnotationRunes+50)
	c.SetAnnotation(0, long)
	if a, _ := c.AnnotationAt(0); len([]rune(a.Text)) != MaxAnnotationRunes {
		t.Fatalf("length %d, want %d", len([]rune(a.Text)), MaxAnnotationRunes)
	}
	if got := SanitizeAnnotation("a\x00b\x1b[31mc\r\n\td\u0085"); got != "ab[31mc\n d" {
		t.Fatalf("sanitised %q", got)
	}
	big := annEditor(strings.Repeat("x\n", 2*MaxAnnotations))
	for i := 0; i < 2*MaxAnnotations; i++ {
		big.SetAnnotation(i, "c")
	}
	if n := big.Annotations().Len(); n != MaxAnnotations {
		t.Fatalf("%d annotations, want the bound %d", n, MaxAnnotations)
	}
}

func TestAnnotationPanelAddEditDelete(t *testing.T) {
	c := annEditor("a\nb")
	pref := c.PreferredHeight().Get()
	c.OnEvent(Event{Kind: EventClick, X: 3, Y: textInset() + 1}) // the gutter of line 0
	if l, open := c.AnnotationOpen(); !open || l != 0 {
		t.Fatalf("gutter click: open=%v line=%d", open, l)
	}
	if c.PreferredHeight().Get() <= pref {
		t.Fatal("an open panel must make room for itself")
	}
	for _, r := range "hi" {
		c.OnEvent(Event{Kind: EventChar, Code: string(r)})
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a, ok := c.AnnotationAt(0); !ok || a.Text != "hi" {
		t.Fatalf("added %+v %v", a, ok)
	}
	if c.InputText() != "a\nb" {
		t.Fatal("typing into the panel must not touch the buffer")
	}
	if c.PreferredHeight().Get() != pref {
		t.Fatal("closing the panel gives the room back")
	}
	// edit
	c.OpenAnnotation(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	c.OnEvent(Event{Kind: EventChar, Code: "o"})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a, _ := c.AnnotationAt(0); a.Text != "ho" {
		t.Fatalf("edited %q", a.Text)
	}
	// escape cancels
	c.OpenAnnotation(0)
	c.OnEvent(Event{Kind: EventChar, Code: "zzz"})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Escape"})
	if a, _ := c.AnnotationAt(0); a.Text != "ho" {
		t.Fatal("escape must keep the stored comment")
	}
	// emptying deletes
	c.OpenAnnotation(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if _, ok := c.AnnotationAt(0); ok {
		t.Fatal("an emptied comment is deleted")
	}
	// the keyboard path
	c.CursorLine().Set(1)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+Shift+M"})
	if l, open := c.AnnotationOpen(); !open || l != 1 {
		t.Fatal("Ctrl+Shift+M opens the panel on the caret's line")
	}
	// a disabled view does not open one
	c.CloseAnnotation()
	c.Disabled().Set(true)
	c.OpenAnnotation(0)
	if _, open := c.AnnotationOpen(); open {
		t.Fatal("a disabled view opened the panel")
	}
}

func TestAnnotationMarkersAreAccessible(t *testing.T) {
	c := annEditor("a\nb\nc")
	c.AccessibleName = "cell"
	c.SetAnnotation(1, "look <b>here</b>")
	nodes := WalkA11y(c)
	var found *A11yNode
	for i := range nodes {
		if nodes[i].Role == RoleButton {
			found = &nodes[i]
		}
	}
	if found == nil {
		t.Fatalf("no marker node in %+v", nodes)
	}
	if found.Name != "Comment on line 2: look <b>here</b>" || found.Value != "look <b>here</b>" {
		t.Fatalf("marker %q / %q", found.Name, found.Value)
	}
	if found.Rect.W <= 0 || found.Rect.H <= 0 {
		t.Fatalf("a marker with no area is dropped by the mirror: %+v", found.Rect)
	}
	if _, ok := Widget(c.Children()[0]).(Focusable); !ok {
		t.Fatal("the marker must be focusable")
	}
	// a long, multi-line comment names the button by its first line, bounded
	c.SetAnnotation(0, strings.Repeat("é", 500)+"\nsecond")
	for _, n := range WalkA11y(c) {
		if n.Role == RoleButton && strings.HasPrefix(n.Name, "Comment on line 1: ") {
			if got := len([]rune(n.Name)); got != len("Comment on line 1: ")+annotationNameRunes+1 || strings.Contains(n.Name, "second") {
				t.Fatalf("name %d runes: %q", got, n.Name)
			}
			if !strings.HasSuffix(n.Value, "second") {
				t.Fatal("the value must hold the whole comment")
			}
		}
	}
	c.DeleteAnnotation(0)
	// the marker opens the panel
	c.Children()[0].OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if l, open := c.AnnotationOpen(); !open || l != 1 {
		t.Fatal("Enter on the marker opens the panel")
	}
	// off: no children
	c.EnableAnnotations(false)
	if len(c.Children()) != 0 {
		t.Fatal("markers with annotations off")
	}
}

func TestAnnotationsDrawAndShiftTheText(t *testing.T) {
	c := annEditor("a\nb")
	base := NewCodeEditor("a\nb")
	base.NoVerticalScroll = true
	if c.textLeftInset()-base.textLeftInset() != c.annGutter() || c.annGutter() == 0 {
		t.Fatal("the gutter column must be reserved while annotations are on")
	}
	c.SetAnnotation(0, "x")
	c.OpenAnnotation(0)
	c.SetBounds(Rect{W: 600, H: c.PreferredHeight().Get()})
	buf := make([]byte, 600*c.Bounds().H*4)
	c.Draw(painter.NewPixelPainter(buf, 600, c.Bounds().H), DefaultLight())
}

func TestMinimapInNoScrollModeIsOptInAndHidesWhenNarrow(t *testing.T) {
	c := NewCodeEditor(scrollDoc(40, 0, 0))
	c.NoVerticalScroll = true
	c.ShowMinimap = true
	c.SetBounds(Rect{W: 800, H: 600})
	if c.minimapW() != 0 {
		t.Fatal("NoVerticalScroll keeps its minimap off unless MinimapWhenFixed")
	}
	c.MinimapWhenFixed = true
	if c.minimapW() != scaled(minimapFixedWidth) {
		t.Fatalf("minimapW = %d", c.minimapW())
	}
	before := c.PreferredHeight().Get()
	c.SetBounds(Rect{W: 360, H: 600})
	if c.minimapW() != 0 {
		t.Fatal("a 360 px cell keeps its whole text width")
	}
	if got, want := c.textAreaWidth(), 360-c.textLeftInset()-1; got != want {
		t.Fatalf("at 360 the text area is %d, want %d", got, want)
	}
	c.SetBounds(Rect{W: 800, H: 600})
	if c.PreferredHeight().Get() != before {
		t.Fatalf("the minimap changed PreferredHeight: %d vs %d", c.PreferredHeight().Get(), before)
	}
	if c.ScrollLine().Get() != 0 {
		t.Fatal("no scroll")
	}
}

func TestMinimapPressInNoScrollModeMovesTheCaret(t *testing.T) {
	c := NewCodeEditor(scrollDoc(30, 0, 0))
	c.NoVerticalScroll, c.ShowMinimap, c.MinimapWhenFixed = true, true, true
	c.SetBounds(Rect{X: 5, Y: 7, W: 800, H: 600})
	rc := c.minimapRect()
	_ = c.syncMinimap()
	rowH := c.minimap.rowH()
	c.OnEvent(Event{Kind: EventClick, X: rc.X + 5, Y: rc.Y + 10*rowH + 1})
	if c.CursorLine().Get() != 10 {
		t.Fatalf("caret on %d, want 10", c.CursorLine().Get())
	}
	if c.ScrollLine().Get() != 0 {
		t.Fatal("it must not scroll")
	}
	c.OnEvent(Event{Kind: EventMouseDrag, X: rc.X + 5, Y: rc.Y + 14*rowH + 1})
	if sel := c.Selection().Get(); sel.IsEmpty() || sel.EndLine != 14 {
		t.Fatalf("a drag extends the selection: %+v", sel)
	}
	c.OnEvent(Event{Kind: EventMouseUp})
	// a mark per annotation
	c.EnableAnnotations(true)
	c.SetAnnotation(3, "m")
	if got := c.annotationMinimapMarks(nil); len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("marks %+v", got)
	}
}

func TestAnnotationEdgeCases(t *testing.T) {
	defer SetClipboard(nil)
	SetClipboard(nil)
	c := annEditor("a\nb\nc")
	if !c.AnnotationsEnabled() {
		t.Fatal("enabled")
	}
	if !c.SetAnnotation(1, "  ") || c.Annotations().Len() != 0 {
		t.Fatal("an empty comment on a bare line adds nothing")
	}
	// a host-appended annotation has no ID: its key follows its line
	c.Annotations().Append(Annotation{Line: 2, Text: "host"})
	m := c.Children()[0].(*annotationMarker)
	if m.A11yID() != "annotation-line-2" {
		t.Fatalf("key %q", m.A11yID())
	}
	c.Disabled().Set(true)
	m.OnEvent(Event{Kind: EventClick})
	if _, open := c.AnnotationOpen(); open {
		t.Fatal("a disabled view's marker opened the panel")
	}
	c.Disabled().Set(false)
	m.OnEvent(Event{Kind: EventMouseMove}) // not an opening event
	if _, open := c.AnnotationOpen(); open {
		t.Fatal("a mouse move opened the panel")
	}
	// paste into the panel
	c.OpenAnnotation(0)
	SetClipboardText("pasted")
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+V"})
	if c.ann.draft != "pasted" {
		t.Fatalf("draft %q", c.ann.draft)
	}
	// a click inside the panel stays in it; one outside closes it
	rc := c.annPanelRect(0, c.glyphHeight()+lineGap())
	c.OnEvent(Event{Kind: EventClick, X: rc.X + 2, Y: rc.Y + 2})
	if _, open := c.AnnotationOpen(); !open {
		t.Fatal("a click in the panel closed it")
	}
	c.OnEvent(Event{Kind: EventClick, X: c.Bounds().W - 2, Y: c.Bounds().H - 2})
	if _, open := c.AnnotationOpen(); open {
		t.Fatal("a click outside kept it open")
	}
	// a press in the gutter below the last line opens nothing
	c.OnEvent(Event{Kind: EventClick, X: 3, Y: c.Bounds().H - 3})
	if _, open := c.AnnotationOpen(); open {
		t.Fatal("the gutter below the text opened a panel")
	}
}

func TestAnnotationPanelLayoutWrapsClampsAndNarrows(t *testing.T) {
	c := annEditor("a\nb")
	c.OpenAnnotation(0)
	c.ann.draft = strings.Repeat("word ", 400)
	rows := c.annPanelRows(scaled(200))
	if len(rows) != 6 {
		t.Fatalf("%d rows, want the 6-row bound", len(rows))
	}
	for _, r := range rows[:len(rows)-1] {
		if c.textWidth(r) > scaled(200) {
			t.Fatalf("row %q is wider than the panel", r)
		}
	}
	// a narrow view: the panel takes the width it has
	c.SetBounds(Rect{W: 100, H: 80})
	rc := c.annPanelRect(0, c.glyphHeight()+lineGap())
	if rc.X != 1 || rc.W != 98 {
		t.Fatalf("narrow panel %+v", rc)
	}
	// no room below: it is pulled up inside the view
	c.SetBounds(Rect{W: 600, H: 30})
	if rc := c.annPanelRect(0, c.glyphHeight()+lineGap()); rc.Y < 0 || rc.Y+rc.H > 30 && rc.Y != 0 {
		t.Fatalf("panel %+v leaves a 30 px view", rc)
	}
	// and it draws, in both
	buf := make([]byte, 600*30*4)
	c.Draw(painter.NewPixelPainter(buf, 600, 30), DefaultLight())
}

func TestAnnotationsOutsideTheWindowAreNotShown(t *testing.T) {
	c := NewCodeEditor(scrollDoc(40, 0, 0))
	c.EnableAnnotations(true)
	c.SetBounds(Rect{W: 600, H: 120})
	for _, l := range []int{0, 12, 39} {
		c.SetAnnotation(l, "x")
	}
	c.ScrollLine().Set(10)
	var lines []int
	for _, w := range c.Children() {
		lines = append(lines, w.(*annotationMarker).line)
	}
	if len(lines) != 1 || lines[0] != 12 {
		t.Fatalf("markers on %v, want only line 12 (0 is above the window, 39 below)", lines)
	}
	buf := make([]byte, 600*120*4)
	c.Draw(painter.NewPixelPainter(buf, 600, 120), DefaultLight())
	// a mark on a line the buffer lost is skipped, not drawn
	c.Annotations().Append(Annotation{Line: 999, Text: "gone"})
	c.Draw(painter.NewPixelPainter(buf, 600, 120), DefaultLight())
	if len(c.Children()) != 1 {
		t.Fatal("a marker for a line that is not there")
	}
}

func TestMinimapMoveCaretOnAnEmptyView(t *testing.T) {
	tv := &TextView{}
	tv.minimapMoveCaret(3, false) // no lines: nothing to move, nothing to panic
}

// An input method (the browser's text layer, an IME) types through ReplaceInput:
// while the panel is open that is the panel's text, not the buffer's.
func TestAnInputMethodTypesIntoTheOpenPanel(t *testing.T) {
	c := annEditor("abc")
	c.OpenAnnotation(0)
	c.ReplaceInput(-1, -1, "hi ")
	c.ReplaceInput(-1, -1, "\n") // Enter's newline: the KeyDown saves, the text must not gain a line
	c.SetMarkedInput("preview", 0, 0, -1, -1)
	c.ReplaceInput(-1, -1, "there")
	if c.ann.draft != "hi there" {
		t.Fatalf("draft %q", c.ann.draft)
	}
	if c.InputText() != "abc" {
		t.Fatalf("the buffer changed: %q", c.InputText())
	}
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if a, _ := c.AnnotationAt(0); a.Text != "hi there" {
		t.Fatalf("saved %+v", a)
	}
	// closed, it is an ordinary editor again
	c.SetInputSelection(3, 3)
	c.ReplaceInput(-1, -1, "d")
	if c.InputText() != "abcd" {
		t.Fatalf("%q", c.InputText())
	}
}

// The browser's text layer takes the pointer over its whole element, so the
// accessible rectangle leaves out the controls that are not text.
func TestTheAccessibleRectangleLeavesOutTheGutterAndTheMinimap(t *testing.T) {
	plain := NewCodeEditor("a\nb")
	plain.SetBounds(Rect{X: 10, Y: 20, W: 600, H: 100})
	if plain.A11yRect() != plain.Bounds() {
		t.Fatalf("no annotations, no minimap: %+v", plain.A11yRect())
	}
	c := annEditor("a\nb")
	c.SetBounds(Rect{X: 10, Y: 20, W: 800, H: 100})
	c.ShowMinimap, c.MinimapWhenFixed = true, true
	r := c.A11yRect()
	if r.X != 10+c.annGutter() || r.X+r.W != 10+c.minimapRect().X || r.Y != 20 || r.H != 100 {
		t.Fatalf("%+v", r)
	}
	for _, n := range WalkA11y(c) {
		if n.Role == RoleTextbox && n.Rect != r {
			t.Fatalf("the walk reports %+v, want %+v", n.Rect, r)
		}
	}
	narrow := annEditor("a")
	narrow.SetBounds(Rect{W: 10, H: 30})
	if narrow.A11yRect() != narrow.Bounds() {
		t.Fatal("a view narrower than its gutter keeps its bounds")
	}
}

// A browser reports Enter to the text layer as well as to the key handler: the
// key saves the comment, and the newline that follows is not the buffer's.
func TestTheNewlineAfterSavingACommentIsNotTyped(t *testing.T) {
	c := annEditor("abc")
	c.OpenAnnotation(0)
	c.ReplaceInput(-1, -1, "x")
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	c.ReplaceInput(-1, -1, "\n")
	if c.InputText() != "abc" {
		t.Fatalf("the buffer gained a newline: %q", c.InputText())
	}
	c.ReplaceInput(-1, -1, "\n") // a second one is a person's
	if c.InputText() != "\nabc" && c.InputText() != "abc\n" {
		t.Fatalf("%q", c.InputText())
	}
	// and the allowance does not outlive the next key
	c.OpenAnnotation(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	c.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	n := len(c.InputText())
	c.ReplaceInput(-1, -1, "\n")
	if len(c.InputText()) != n+1 {
		t.Fatal("a newline after another key was eaten")
	}
	// and the first text after Enter ends it
	c.OpenAnnotation(0)
	c.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	c.ReplaceInput(-1, -1, "z")
	c.ReplaceInput(-1, -1, "\n")
	if !strings.Contains(c.InputText(), "z\n") {
		t.Fatalf("%q", c.InputText())
	}
}

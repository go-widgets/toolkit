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
	if found.Name != "Comment on line 2" || found.Value != "look <b>here</b>" {
		t.Fatalf("marker %q / %q", found.Name, found.Value)
	}
	if found.Rect.W <= 0 || found.Rect.H <= 0 {
		t.Fatalf("a marker with no area is dropped by the mirror: %+v", found.Rect)
	}
	if _, ok := Widget(c.Children()[0]).(Focusable); !ok {
		t.Fatal("the marker must be focusable")
	}
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

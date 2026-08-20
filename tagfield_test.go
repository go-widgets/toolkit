// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"reflect"
	"testing"
)

// painted reports whether any pixel in buf differs from the makeSurface
// sentinel (0xC8,0xC8,0xC8,0xFF) -- i.e. the widget drew something.
func painted(buf []byte) bool {
	for i := 0; i+3 < len(buf); i += 4 {
		if buf[i] != 0xC8 || buf[i+1] != 0xC8 || buf[i+2] != 0xC8 {
			return true
		}
	}
	return false
}

func TestNewTagField(t *testing.T) {
	tf := NewTagField("a", "b")
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"a", "b"}) {
		t.Fatalf("Tags = %v", tf.Tags().Get())
	}
	if tf.Text().Get() != "" {
		t.Fatalf("Text = %q, want empty", tf.Text().Get())
	}
}

// TestTagFieldAccessorsLazyInit exercises both arms of each accessor on a bare
// &TagField{}: the nil lazy-init pass (first call) and the return-existing pass
// (second call returns the same Observable).
func TestTagFieldAccessorsLazyInit(t *testing.T) {
	tf := &TagField{}
	if got := tf.Tags().Get(); got != nil {
		t.Fatalf("fresh Tags() = %v, want nil", got)
	}
	if tf.Tags() != tf.Tags() {
		t.Fatal("Tags() must return the same Observable on repeat calls")
	}
	if got := tf.Text().Get(); got != "" {
		t.Fatalf("fresh Text() = %q, want empty", got)
	}
	if tf.Text() != tf.Text() {
		t.Fatal("Text() must return the same Observable on repeat calls")
	}
}

// TestTagFieldDrawWraps draws two tokens into a width that forces the
// second onto a new row, exercising the wrap branch in layout, then the
// caret branch (tags present, empty Text).
func TestTagFieldDrawWraps(t *testing.T) {
	tf := NewTagField("aa", "bb")
	// chipWidth("aa") == 2*6 + 32 == 44; two 44-wide chips + gap cannot
	// share a 60px row, so "bb" wraps to a second row.
	tf.SetBounds(Rect{X: 0, Y: 0, W: 60, H: 40})
	if w := tf.chipWidth("aa"); w != 44 {
		t.Fatalf("chipWidth = %d, want 44", w)
	}
	rects, _, endY := tf.layout(0, 0)
	if len(rects) != 2 {
		t.Fatalf("rects = %d, want 2", len(rects))
	}
	if rects[1].Y == rects[0].Y {
		t.Fatalf("second token did not wrap: %+v", rects)
	}
	if endY == 0 {
		t.Fatalf("endY not advanced past first row: %d", endY)
	}
	buf := makeSurface(60, 40)
	tf.Draw(newP(buf, 60), DefaultLight())
	if !painted(buf) {
		t.Fatal("Draw painted nothing")
	}
}

// TestTagFieldDrawPlaceholder covers the empty-field placeholder branch.
func TestTagFieldDrawPlaceholder(t *testing.T) {
	tf := &TagField{Placeholder: "add tags..."}
	tf.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 16})
	buf := makeSurface(120, 16)
	tf.Draw(newP(buf, 120), DefaultLight())
	if !painted(buf) {
		t.Fatal("placeholder drew nothing")
	}
}

// TestTagFieldDrawEmptyNoPlaceholder covers the empty-field branch when
// Placeholder is also empty: nothing is drawn.
func TestTagFieldDrawEmptyNoPlaceholder(t *testing.T) {
	tf := &TagField{}
	tf.SetBounds(Rect{X: 0, Y: 0, W: 80, H: 16})
	buf := makeSurface(80, 16)
	tf.Draw(newP(buf, 80), DefaultLight())
	if painted(buf) {
		t.Fatal("empty field with no placeholder should draw nothing")
	}
}

// TestTagFieldDrawText covers the in-progress-Text-plus-caret branch.
func TestTagFieldDrawText(t *testing.T) {
	tf := &TagField{}
	tf.Text().Set("go")
	tf.SetBounds(Rect{X: 0, Y: 0, W: 120, H: 16})
	buf := makeSurface(120, 16)
	tf.Draw(newP(buf, 120), DefaultLight())
	if !painted(buf) {
		t.Fatal("text drew nothing")
	}
}

// TestTagFieldCharAppends covers EventChar (append) and its comma swallow.
func TestTagFieldCharAppends(t *testing.T) {
	tf := &TagField{}
	tf.OnEvent(Event{Kind: EventChar, Code: "g"})
	tf.OnEvent(Event{Kind: EventChar, Code: "o"})
	if tf.Text().Get() != "go" {
		t.Fatalf("Text = %q, want %q", tf.Text().Get(), "go")
	}
	tf.OnEvent(Event{Kind: EventChar, Code: ","})
	if tf.Text().Get() != "go" {
		t.Fatalf("comma should be swallowed, Text = %q", tf.Text().Get())
	}
}

// TestTagFieldCommitEnter commits via Enter and notifies Tags() subscribers.
func TestTagFieldCommitEnter(t *testing.T) {
	var got []string
	tf := &TagField{}
	tf.Text().Set("  go  ")
	tf.Tags().Subscribe(func(tags []string) { got = tags })
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"go"}) {
		t.Fatalf("Tags = %v, want [go] (trimmed)", tf.Tags().Get())
	}
	if tf.Text().Get() != "" {
		t.Fatalf("Text not cleared: %q", tf.Text().Get())
	}
	if !reflect.DeepEqual(got, []string{"go"}) {
		t.Fatalf("Tags subscriber got %v", got)
	}
}

// TestTagFieldCommitComma commits via the comma key.
func TestTagFieldCommitComma(t *testing.T) {
	tf := &TagField{}
	tf.Text().Set("rust")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: ","})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"rust"}) {
		t.Fatalf("Tags = %v", tf.Tags().Get())
	}
}

// TestTagFieldCommitBlankAndDuplicate covers the two skip paths in commit.
func TestTagFieldCommitBlankAndDuplicate(t *testing.T) {
	fired := 0
	tf := NewTagField("go")
	tf.Tags().Subscribe(func([]string) { fired++ })
	// blank: cleared, no add, no notify.
	tf.Text().Set("   ")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	// duplicate: cleared, no add, no notify.
	tf.Text().Set("go")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"go"}) {
		t.Fatalf("Tags = %v, want unchanged [go]", tf.Tags().Get())
	}
	if fired != 0 {
		t.Fatalf("Tags subscriber fired %d times, want 0", fired)
	}
}

// TestTagFieldBackspaceRemovesLast covers backspace on empty Text.
func TestTagFieldBackspaceRemovesLast(t *testing.T) {
	var got []string
	tf := NewTagField("a", "b")
	tf.Tags().Subscribe(func(tags []string) { got = tags })
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"a"}) {
		t.Fatalf("Tags = %v, want [a]", tf.Tags().Get())
	}
	if !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("Tags subscriber got %v", got)
	}
}

// TestTagFieldBackspaceNoop covers both false arms of the backspace guard:
// non-empty Text (with tags) and empty Text (no tags).
func TestTagFieldBackspaceNoop(t *testing.T) {
	tf := NewTagField("a")
	tf.Text().Set("x")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"a"}) || tf.Text().Get() != "x" {
		t.Fatalf("non-empty backspace mutated state: %v %q", tf.Tags().Get(), tf.Text().Get())
	}
	empty := &TagField{}
	empty.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	if len(empty.Tags().Get()) != 0 {
		t.Fatalf("backspace on empty field mutated tags: %v", empty.Tags().Get())
	}
}

// TestTagFieldKeyDownUnknown covers a keydown code with no matching case.
func TestTagFieldKeyDownUnknown(t *testing.T) {
	tf := &TagField{}
	tf.Text().Set("go")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "ArrowLeft"})
	if tf.Text().Get() != "go" {
		t.Fatalf("unknown key mutated Text: %q", tf.Text().Get())
	}
}

// TestTagFieldClickRemovesTag clicks a specific token's close slot and
// verifies exactly that tag is removed.
func TestTagFieldClickRemovesTag(t *testing.T) {
	var got []string
	tf := NewTagField("aa", "bb", "cc")
	tf.Tags().Subscribe(func(tags []string) { got = tags })
	tf.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 16})
	rects, _, _ := tf.layout(0, 0)
	// close slot of token index 1 ("bb"): local x in [W-Pad-CloseW, W-Pad).
	rc := rects[1]
	cx := rc.X + rc.W - ChipPadX - ChipCloseW/2
	cy := rc.Y + rc.H/2
	tf.OnEvent(Event{Kind: EventClick, X: cx, Y: cy})
	if !reflect.DeepEqual(tf.Tags().Get(), []string{"aa", "cc"}) {
		t.Fatalf("Tags = %v, want [aa cc]", tf.Tags().Get())
	}
	if !reflect.DeepEqual(got, []string{"aa", "cc"}) {
		t.Fatalf("Tags subscriber got %v", got)
	}
	if !tf.Focused() {
		t.Fatal("click should focus the field")
	}
}

// TestTagFieldClickBodyKeeps clicks a token's label area (not the close
// slot): the tag stays, the field just focuses.
func TestTagFieldClickBodyKeeps(t *testing.T) {
	tf := NewTagField("aa")
	tf.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 16})
	rects, _, _ := tf.layout(0, 0)
	rc := rects[0]
	tf.OnEvent(Event{Kind: EventClick, X: rc.X + ChipPadX, Y: rc.Y + rc.H/2})
	if len(tf.Tags().Get()) != 1 {
		t.Fatalf("body click removed a tag: %v", tf.Tags().Get())
	}
	if !tf.Focused() {
		t.Fatal("click should focus the field")
	}
}

// TestTagFieldClickOutside clicks past every token: nothing removed, but
// the field focuses (loop runs to completion).
func TestTagFieldClickOutside(t *testing.T) {
	tf := NewTagField("aa")
	tf.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 16})
	tf.OnEvent(Event{Kind: EventClick, X: 390, Y: 8})
	if len(tf.Tags().Get()) != 1 {
		t.Fatalf("outside click removed a tag: %v", tf.Tags().Get())
	}
	if !tf.Focused() {
		t.Fatal("click should focus the field")
	}
}

// TestTagFieldNoSubscriberSafe drives every mutating path with no Tags()
// subscriber to prove a Set with no observers is safe.
func TestTagFieldNoSubscriberSafe(t *testing.T) {
	tf := &TagField{}
	tf.SetBounds(Rect{X: 0, Y: 0, W: 400, H: 16})
	tf.Text().Set("go")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"}) // commit + notify
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace"})
	tf.Text().Set("go")
	tf.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	rects, _, _ := tf.layout(0, 0)
	rc := rects[0]
	tf.OnEvent(Event{Kind: EventClick, X: rc.X + rc.W - ChipPadX - ChipCloseW/2, Y: rc.Y + rc.H/2})
	if len(tf.Tags().Get()) != 0 {
		t.Fatalf("expected all tags removed, got %v", tf.Tags().Get())
	}
}

// TestTagFieldIgnoresOtherEvents covers the outer switch's no-match arm.
func TestTagFieldIgnoresOtherEvents(t *testing.T) {
	tf := &TagField{}
	tf.Text().Set("go")
	tf.OnEvent(Event{Kind: EventKeyUp, Code: "g"})
	if tf.Text().Get() != "go" {
		t.Fatalf("EventKeyUp mutated Text: %q", tf.Text().Get())
	}
}

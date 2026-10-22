// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-widgets/painter"
)

// A small slides project: a deck, its configuration, and three folders.
func projectEntries() []FileEntry {
	return []FileEntry{
		{Path: "deck.md", Size: 2048},
		{Path: "slides.yaml", Size: 40},
		{Path: "themes/dark.css", Size: 900},
		{Path: "kinds/quote.yaml", Size: 120},
		{Path: "media", Dir: true},
		{Path: "/media/../media/photo.jpg", Size: 3 << 20},
		{Path: "Zeta.md"},
		{Path: "deck.md"}, // a duplicate is shown once
		{Path: "/"},       // the root is no entry
	}
}

// newTree is a FileTree of the project, 220×400 at the origin, recording its
// requests.
func newTree(t *testing.T) (*FileTree, *[]FileRequest) {
	t.Helper()
	f := NewFileTree(projectEntries())
	f.Label = "Project files"
	var got []FileRequest
	f.OnRequest = func(r FileRequest) { got = append(got, r) }
	f.SetBounds(Rect{W: 220, H: 400})
	return f, &got
}

// labels are the visible rows' names, indented two spaces a level.
func labels(f *FileTree) []string {
	f.ensure()
	f.layout()
	f.tree.flatten()
	var out []string
	for _, r := range f.tree.rows {
		out = append(out, strings.Repeat("  ", r.depth)+r.node.Label)
	}
	return out
}

func ftKey(f *FileTree, code string) { f.OnEvent(Event{Kind: EventKeyDown, Code: code}) }

// same compares two requests, Names included.
func same(a, b FileRequest) bool {
	return a.Action == b.Action && a.Path == b.Path && a.To == b.To && slices.Equal(a.Names, b.Names)
}
func typeText(f *FileTree, s string) {
	for _, r := range s {
		f.OnEvent(Event{Kind: EventChar, Code: string(r)})
	}
}

// rowY is the widget-local y of the middle of the row named label.
func rowY(t *testing.T, f *FileTree, label string) int {
	t.Helper()
	for i, l := range labels(f) {
		if strings.TrimSpace(l) == label {
			return i*f.rowH() + f.rowH()/2
		}
	}
	t.Fatalf("no row %q in %q", label, labels(f))
	return 0
}

func TestFileTreeOrdersFoldersFirstAndKeepsCollapsedState(t *testing.T) {
	f, _ := newTree(t)
	want := []string{"kinds", "  quote.yaml", "media", "  photo.jpg", "themes", "  dark.css", "deck.md", "slides.yaml", "Zeta.md"}
	if got := labels(f); !slices.Equal(got, want) {
		t.Fatalf("rows = %q\nwant  %q", got, want)
	}
	// Collapse media; a new list keeps it collapsed, and new folders open.
	f.nodes["media"].Expanded = false
	f.Entries().Set(append(projectEntries(), FileEntry{Path: "data/table.json"}))
	got := labels(f)
	if !slices.Contains(got, "  table.json") || slices.Contains(got, "  photo.jpg") {
		t.Fatalf("after update rows = %q", got)
	}
	// Equal names differing by case keep a stable order.
	g := NewFileTree([]FileEntry{{Path: "b"}, {Path: "B"}, {Path: "a"}})
	if got := labels(g); !slices.Equal(got, []string{"a", "B", "b"}) {
		t.Fatalf("rows = %q", got)
	}
}

func TestFileTreeClickOpensFilesAndTogglesFolders(t *testing.T) {
	f, got := newTree(t)
	f.OnEvent(Event{Kind: EventClick, X: 60, Y: rowY(t, f, "deck.md")})
	f.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: rowY(t, f, "deck.md")})
	if len(*got) != 1 || !same((*got)[0], FileRequest{Action: FileOpen, Path: "deck.md"}) {
		t.Fatalf("requests = %+v", *got)
	}
	if f.Selected().Get() != "deck.md" {
		t.Fatalf("selected = %q", f.Selected().Get())
	}
	f.OnEvent(Event{Kind: EventClick, X: 60, Y: rowY(t, f, "kinds")})
	if !slices.Contains(labels(f), "  quote.yaml") {
		t.Fatal("a press on a folder should wait for the release")
	}
	f.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: rowY(t, f, "kinds")})
	if slices.Contains(labels(f), "  quote.yaml") {
		t.Fatal("a click on a folder should close it")
	}
	// The host moves the selection; a hidden entry is revealed.
	f.Reveal("kinds/quote.yaml")
	if f.Selected().Get() != "kinds/quote.yaml" || !f.nodes["kinds"].Expanded {
		t.Fatal("Reveal did not open the folder")
	}
	f.Selected().Set("nowhere")
	if f.tree.Selected().Get() != nil {
		t.Fatal("an unknown path selects nothing")
	}
	// A removed entry drops the selection.
	f.Selected().Set("Zeta.md")
	f.Entries().Set([]FileEntry{{Path: "deck.md"}})
	f.ensure()
	if f.Selected().Get() != "" {
		t.Fatalf("selected = %q after its entry went", f.Selected().Get())
	}
	// Without a handler, nothing breaks.
	f.OnRequest = nil
	f.request(FileRequest{})
}

func TestFileTreeKeyboardFollowsTheAPGTreeview(t *testing.T) {
	f, got := newTree(t)
	ftKey(f, "ArrowDown") // first row
	ftKey(f, "ArrowDown")
	if f.Selected().Get() != "kinds/quote.yaml" {
		t.Fatalf("selected = %q", f.Selected().Get())
	}
	ftKey(f, "ArrowLeft") // to the parent
	ftKey(f, "ArrowLeft") // closes it
	if f.Selected().Get() != "kinds" || f.nodes["kinds"].Expanded {
		t.Fatal("Left should go to the parent, then close it")
	}
	ftKey(f, "Enter") // a folder's Enter toggles it
	if !f.nodes["kinds"].Expanded {
		t.Fatal("Enter should open the folder")
	}
	ftKey(f, "End")
	ftKey(f, "Enter")
	if last := (*got)[len(*got)-1]; !same(last, FileRequest{Action: FileOpen, Path: "Zeta.md"}) {
		t.Fatalf("last request = %+v", last)
	}
	// Type-ahead: one character steps through the matches; quick characters
	// match as a string; a pause starts again.
	clock := time.Unix(1000, 0)
	fileTreeNow = func() time.Time { return clock }
	defer func() { fileTreeNow = time.Now }()
	typeText(f, "d")
	if f.Selected().Get() != "themes/dark.css" {
		t.Fatalf("d -> %q", f.Selected().Get())
	}
	clock = clock.Add(2 * time.Second)
	typeText(f, "d")
	if f.Selected().Get() != "deck.md" {
		t.Fatalf("d again -> %q", f.Selected().Get())
	}
	clock = clock.Add(2 * time.Second)
	typeText(f, "sl")
	if f.Selected().Get() != "slides.yaml" {
		t.Fatalf("sl -> %q", f.Selected().Get())
	}
	clock = clock.Add(2 * time.Second)
	typeText(f, "q")
	if f.Selected().Get() != "kinds/quote.yaml" {
		t.Fatalf("q -> %q", f.Selected().Get())
	}
	clock = clock.Add(2 * time.Second)
	typeText(f, "x") // no match: stays
	if f.Selected().Get() != "kinds/quote.yaml" {
		t.Fatalf("x -> %q", f.Selected().Get())
	}
	// * expands every folder beside the selection.
	for _, d := range []string{"kinds", "media", "themes"} {
		f.nodes[d].Expanded = false
	}
	f.Selected().Set("media")
	typeText(f, "*")
	for _, d := range []string{"kinds", "media", "themes"} {
		if !f.nodes[d].Expanded {
			t.Fatalf("%s not expanded by *", d)
		}
	}
	f.Selected().Set("")
	typeText(f, "*") // at the root, the top-level folders
	// Disabled, nothing moves.
	f.Disabled().Set(true)
	ftKey(f, "Home")
	if f.Selected().Get() != "" {
		t.Fatal("a disabled tree took a key")
	}
}

func TestFileTreeRenameInPlace(t *testing.T) {
	f, got := newTree(t)
	f.Selected().Set("themes/dark.css")
	ftKey(f, "F2")
	if !f.Editing() || f.edit.entry.Text().Get() != "dark.css" {
		t.Fatal("F2 should start a rename with the name")
	}
	f.Draw(newP(makeSurface(220, 400), 220), DefaultLight()) // the entry is drawn over the row
	if f.edit.entry.Bounds().W == 0 {
		t.Fatal("the rename entry was not placed")
	}
	cases := []struct{ text, msg string }{
		{"", "A file or folder name must be provided."},
		{"/x", "A file or folder name cannot start with a slash."},
		{"a/../b", "The name 'a/../b' is not valid as a file or folder name. Please choose a different name."},
		{"bad\x01", "The name 'bad\x01' is not valid as a file or folder name. Please choose a different name."},
		{"a:b", "The name 'a:b' is not valid as a file or folder name. Please choose a different name."},
		{"../deck.md", "The name '../deck.md' is not valid as a file or folder name. Please choose a different name."},
		{" light.css ", "Leading or trailing whitespace detected in file or folder name."},
		{"dark.css", ""},
	}
	for _, c := range cases {
		f.edit.entry.SetText(c.text)
		f.validate()
		if f.Message() != c.msg {
			t.Errorf("%q: message %q, want %q", c.text, f.Message(), c.msg)
		}
	}
	// Typing goes to the entry; an unchanged name just ends.
	f.edit.entry.SetText("dark.cs")
	typeText(f, "s")
	ftKey(f, "Enter")
	if f.Editing() || len(*got) != 0 {
		t.Fatalf("unchanged rename: editing=%v requests=%+v", f.Editing(), *got)
	}
	// An existing name is refused; Enter keeps the entry open.
	f.Entries().Set(append(projectEntries(), FileEntry{Path: "themes/light.css"}))
	f.BeginRename("themes/dark.css")
	f.edit.entry.SetText("light.css")
	ftKey(f, "Enter")
	if !f.Editing() || !strings.Contains(f.Message(), "already exists") {
		t.Fatalf("message = %q", f.Message())
	}
	f.Draw(newP(makeSurface(220, 400), 220), DefaultDark()) // the error in the footer
	f.edit.entry.SetText("night.css")
	ftKey(f, "Enter")
	want := FileRequest{Action: FileRename, Path: "themes/dark.css", To: "themes/night.css"}
	if len(*got) != 1 || !same((*got)[0], want) || f.Selected().Get() != "themes/night.css" {
		t.Fatalf("requests = %+v, selected %q", *got, f.Selected().Get())
	}
	// A folder cannot go inside itself; Escape cancels.
	f.BeginRename("kinds")
	f.edit.entry.SetText("kinds/sub")
	f.validate()
	if !f.edit.block {
		t.Fatal("moving a folder into itself must be refused")
	}
	ftKey(f, "Escape")
	if f.Editing() {
		t.Fatal("Escape should cancel")
	}
	// The root and unknown paths cannot be renamed.
	f.BeginRename("")
	f.BeginRename("nope")
	if f.Editing() {
		t.Fatal("rename of nothing started")
	}
	// The entry renamed meanwhile by someone else ends the rename.
	f.BeginRename("deck.md")
	f.Entries().Set([]FileEntry{{Path: "other.md"}})
	f.ensure()
	if f.Editing() {
		t.Fatal("a rename of a vanished entry should end")
	}
}

func TestFileTreeCreateFileAndFolder(t *testing.T) {
	f, got := newTree(t)
	f.Selected().Set("themes/dark.css")
	f.BeginCreate(f.Selected().Get(), false) // a file's folder
	rows := labels(f)
	if rows[5] != "  " || rows[6] != "  dark.css" {
		t.Fatalf("pending row not at the top of the files of themes: %q", rows)
	}
	if f.Message() != "A file or folder name must be provided." {
		t.Fatalf("message = %q", f.Message())
	}
	typeText(f, "light.css")
	ftKey(f, "Enter")
	if len(*got) != 1 || !same((*got)[0], FileRequest{Action: FileCreate, Path: "themes/light.css"}) {
		t.Fatalf("requests = %+v", *got)
	}
	if slices.Contains(labels(f), "  ") {
		t.Fatal("the pending row stayed")
	}
	// A folder goes first among the root's entries; the list may change while
	// the name is typed.
	f.BeginCreate("", true)
	if labels(f)[0] != "" {
		t.Fatalf("pending folder row: %q", labels(f))
	}
	f.Entries().Set(append(projectEntries(), FileEntry{Path: "z.md"}))
	if labels(f)[0] != "" {
		t.Fatal("the pending row should survive an update")
	}
	typeText(f, "data")
	ftKey(f, "Enter")
	if !same((*got)[1], FileRequest{Action: FileCreateDir, Path: "data"}) {
		t.Fatalf("requests = %+v", *got)
	}
	// A folder that vanished is created at the root instead.
	f.BeginCreate("media", false)
	f.Entries().Set([]FileEntry{{Path: "deck.md"}})
	f.ensure()
	if f.edit.path != "" {
		t.Fatalf("pending folder = %q", f.edit.path)
	}
	f.Dismiss()
	// A click outside the entry commits a good name, cancels a bad one.
	f, got = newTree(t)
	f.BeginCreate("", false)
	f.Draw(newP(makeSurface(220, 400), 220), DefaultLight())
	eb := f.edit.entry.Bounds()
	f.OnEvent(Event{Kind: EventClick, X: eb.X + 2, Y: eb.Y + 2}) // inside: stays
	if !f.Editing() {
		t.Fatal("a click in the entry ended the edit")
	}
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 390}) // outside, empty name: cancelled
	if f.Editing() || len(*got) != 0 {
		t.Fatal("an empty name should be cancelled by a click outside")
	}
	f.BeginCreate("", false)
	typeText(f, "notes.md")
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 390})
	if len(*got) != 1 || (*got)[0].Path != "notes.md" {
		t.Fatalf("requests = %+v", *got)
	}
}

func TestFileTreeDeleteAsksFirst(t *testing.T) {
	f, got := newTree(t)
	f.Selected().Set("media")
	ftKey(f, "Delete")
	if f.Confirming() != "media" || f.Message() != "Are you sure you want to delete 'media' and its contents?" {
		t.Fatalf("confirming %q: %q", f.Confirming(), f.Message())
	}
	ftKey(f, "Escape")
	if f.Confirming() != "" || len(*got) != 0 {
		t.Fatal("Escape should cancel the delete")
	}
	f.Selected().Set("deck.md")
	f.OnEvent(Event{Kind: EventKeyDown, Code: "Backspace", Meta: true})
	if f.Message() != "Are you sure you want to delete 'deck.md'?" {
		t.Fatalf("message %q", f.Message())
	}
	ftKey(f, "Enter")
	if len(*got) != 1 || !same((*got)[0], FileRequest{Action: FileDelete, Path: "deck.md"}) {
		t.Fatalf("requests = %+v", *got)
	}
	ftKey(f, "Backspace") // without ⌘, nothing
	if f.Confirming() != "" {
		t.Fatal("plain Backspace asked to delete")
	}
	// The buttons, after a Draw placed them.
	f.AskDelete("slides.yaml")
	buf := makeSurface(220, 400)
	f.Draw(newP(buf, 220), DefaultLight())
	cancel := f.footerButton[1]
	f.OnEvent(Event{Kind: EventClick, X: cancel.X + 2, Y: cancel.Y + 2})
	if f.Confirming() != "" || len(*got) != 1 {
		t.Fatal("Cancel should end the question")
	}
	f.AskDelete("slides.yaml")
	f.Draw(newP(buf, 220), DefaultLight())
	del := f.footerButton[0]
	f.OnEvent(Event{Kind: EventClick, X: del.X + 2, Y: del.Y + 2})
	if len(*got) != 2 || (*got)[1].Path != "slides.yaml" {
		t.Fatalf("requests = %+v", *got)
	}
	// A click in the footer beside the buttons keeps the question; a click on
	// the rows drops it.
	f.AskDelete("slides.yaml")
	f.Draw(newP(buf, 220), DefaultLight())
	f.OnEvent(Event{Kind: EventClick, X: 215, Y: 398})
	if f.Confirming() == "" {
		t.Fatal("a click in the footer dropped the question")
	}
	f.OnEvent(Event{Kind: EventClick, X: 10, Y: 5})
	if f.Confirming() != "" {
		t.Fatal("a click on the rows should drop the question")
	}
	f.AskDelete("")
	f.AskDelete("nope")
	if f.Confirming() != "" {
		t.Fatal("asked to delete nothing")
	}
}

func TestFileTreeContextMenu(t *testing.T) {
	f, _ := newTree(t)
	f.MenuExtra = func(p string, dir bool) []MenuItem {
		if p == "themes" {
			return []MenuItem{{Label: "New theme…"}}
		}
		return nil
	}
	labelsOf := func() []string {
		var out []string
		for _, it := range f.menu.Menu.Items {
			out = append(out, it.Label)
		}
		return out
	}
	f.Context(40, rowY(t, f, "themes"))
	if !f.MenuOpen() || !slices.Equal(labelsOf(), []string{"New File…", "New Folder…", "", "Rename…", "Delete", "", "New theme…"}) {
		t.Fatalf("menu = %q", labelsOf())
	}
	f.Draw(newP(makeSurface(220, 400), 220), DefaultLight())
	ftKey(f, "Escape")
	if f.MenuOpen() {
		t.Fatal("Escape should close the menu")
	}
	f.Context(40, rowY(t, f, "deck.md"))
	if !slices.Equal(labelsOf(), []string{"Rename…", "Delete"}) {
		t.Fatalf("file menu = %q", labelsOf())
	}
	// Activating an item runs it: Rename.
	f.menu.Menu.Items[0].Action()
	if !f.Editing() {
		t.Fatal("Rename… did not start a rename")
	}
	f.Dismiss()
	f.menu.Menu.Items[1].Action()
	if f.Confirming() != "deck.md" {
		t.Fatal("Delete did not ask")
	}
	f.Context(40, 390) // below the rows: the root
	if !slices.Equal(labelsOf(), []string{"New File…", "New Folder…"}) {
		t.Fatalf("root menu = %q", labelsOf())
	}
	f.menu.Menu.Items[0].Action()
	if !f.Editing() || f.edit.action != FileCreate || f.edit.path != "" {
		t.Fatal("New File… at the root")
	}
	f.Dismiss()
	f.Context(40, 390)
	f.menu.Menu.Items[1].Action()
	if f.edit.action != FileCreateDir {
		t.Fatal("New Folder…")
	}
	// The menu takes the events while open (a click outside closes it), and
	// the keyboard opens it on the selection.
	f.Dismiss()
	f.Selected().Set("media")
	ftKey(f, "Shift+F10")
	if !f.MenuOpen() {
		t.Fatal("Shift+F10 should open the menu")
	}
	f.OnEvent(Event{Kind: EventClick, X: 219, Y: 1})
	if f.MenuOpen() {
		t.Fatal("a click outside the menu should close it")
	}
	f.Selected().Set("")
	ftKey(f, "ContextMenu")
	if !f.MenuOpen() {
		t.Fatal("the ContextMenu key should open the menu")
	}
}

func TestFileTreeDragMovesIntoAFolder(t *testing.T) {
	f, got := newTree(t)
	drag := func(from, to string, toY int) {
		y := rowY(t, f, from)
		f.OnEvent(Event{Kind: EventClick, X: 60, Y: y})
		f.OnEvent(Event{Kind: EventMouseDrag, X: 61, Y: y + 1}) // under the threshold
		if to != "" {
			toY = rowY(t, f, to)
		}
		f.OnEvent(Event{Kind: EventMouseDrag, X: 60, Y: toY})
		f.Draw(newP(makeSurface(220, 400), 220), DefaultLight())
		f.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: toY})
	}
	moves := func() []FileRequest {
		var out []FileRequest
		for _, r := range *got {
			if r.Action == FileMove {
				out = append(out, r)
			}
		}
		*got = nil
		return out
	}
	drag("deck.md", "media", 0)
	want := FileRequest{Action: FileMove, Path: "deck.md", To: "media/deck.md"}
	if m := moves(); len(m) != 1 || !same(m[0], want) {
		t.Fatalf("moves = %+v", m)
	}
	drag("dark.css", "photo.jpg", 0) // onto a file: its folder
	if m := moves(); len(m) != 1 || m[0].To != "media/dark.css" {
		t.Fatalf("moves = %+v", m)
	}
	f.Entries().Set(append(projectEntries(), FileEntry{Path: "media/Zeta.md"}))
	drag("slides.yaml", "media", 0) // warm: a move
	moves()
	f.Selected().Set("Zeta.md")
	y := f.tree.rowOf(f.nodes["Zeta.md"])*f.rowH() + f.rowH()/2
	f.OnEvent(Event{Kind: EventClick, X: 60, Y: y})
	f.OnEvent(Event{Kind: EventMouseDrag, X: 60, Y: rowY(t, f, "media")})
	f.OnEvent(Event{Kind: EventMouseUp, X: 60, Y: rowY(t, f, "media")}) // a name already there: nothing
	if m := moves(); len(m) != 0 {
		t.Fatalf("moves = %+v", m)
	}
	drag("dark.css", "", 390) // below the rows: the root
	if m := moves(); len(m) != 1 || m[0].To != "dark.css" {
		t.Fatalf("moves = %+v", m)
	}
	drag("quote.yaml", "kinds", 0)    // its own folder: nothing
	drag("Zeta.md", "slides.yaml", 0) // a name already there: nothing
	drag("media", "photo.jpg", 0)     // into itself: nothing
	if m := moves(); len(m) != 0 {
		t.Fatalf("moves = %+v", m)
	}
	// A plain click and release moves nothing; a release with no press is
	// harmless; the scrollbar's drag stays the tree's.
	f.OnEvent(Event{Kind: EventMouseUp})
	f.OnEvent(Event{Kind: EventMouseDrag, X: 1, Y: 1})
	if _, on := f.DropTarget(); on {
		t.Fatal("no drag is in progress")
	}
}

func TestFileTreeDropFromOutside(t *testing.T) {
	f, got := newTree(t)
	if !f.AcceptsDrop("photo.png") || f.AcceptsDrop("") {
		t.Fatal("AcceptsDrop")
	}
	y := rowY(t, f, "photo.jpg")
	f.OnEvent(Event{Kind: EventDragStart, X: 50, Y: y})
	if dir, on := f.DropTarget(); !on || dir != "media" {
		t.Fatalf("drop target %q %v", dir, on)
	}
	for name, th := range popupThemes() {
		buf := makeSurface(220, 400)
		for i := 0; i+3 < len(buf); i += 4 {
			buf[i], buf[i+1], buf[i+2], buf[i+3] = th.Surface.R, th.Surface.G, th.Surface.B, 255
		}
		f.Draw(newP(buf, 220), th)
		top := rowY(t, f, "media") - f.rowH()/2
		out := pixelAt(buf, 220, 100, top)
		if r := contrastRatio(out, th.Surface); r < 3 {
			t.Errorf("%s: drop outline %v vs surface %.2f:1, want >= 3:1", name, out, r)
		}
	}
	f.OnEvent(Event{Kind: EventDragLeave})
	f.OnEvent(Event{Kind: EventDragMove, X: 50, Y: 395})
	if dir, _ := f.DropTarget(); dir != "" {
		t.Fatalf("below the rows the drop goes to the root, got %q", dir)
	}
	f.Draw(newP(makeSurface(220, 400), 220), DefaultLight())
	f.OnEvent(Event{Kind: EventDrop, X: 50, Y: rowY(t, f, "kinds"), Code: "a.png\nb.mp4\n"})
	want := FileRequest{Action: FileImport, To: "kinds", Names: []string{"a.png", "b.mp4"}}
	if len(*got) != 1 || (*got)[0].To != want.To || !slices.Equal((*got)[0].Names, want.Names) {
		t.Fatalf("requests = %+v", *got)
	}
	f.OnEvent(Event{Kind: EventDrop, Code: ""})
	if len(*got) != 1 {
		t.Fatal("an empty drop asked for something")
	}
	// A folder scrolled out of the window, or collapsed, draws no outline.
	f.SetBounds(Rect{W: 220, H: f.rowH() * 2})
	f.dropping, f.dropDir = true, "themes"
	f.Draw(newP(makeSurface(220, f.rowH()*2), 220), DefaultLight())
	f.nodes["media"].Expanded = false
	f.dropDir = "media/sub"
	f.Draw(newP(makeSurface(220, f.rowH()*2), 220), DefaultLight())
	f.nodes["kinds"].Expanded = false
	f.dropDir = "kinds"
	f.tree.ScrollRow().Set(1)
	f.dropDir = "kinds"
	f.Draw(newP(makeSurface(220, f.rowH()*2), 220), DefaultLight())
	f.Entries().Set([]FileEntry{{Path: "a/b/c.md"}})
	f.ensure()
	f.nodes["a"].Expanded = false
	f.dropDir = "a/b"
	f.Draw(newP(makeSurface(220, f.rowH()*2), 220), DefaultLight())
}

func TestFileTreeDrawsIconsSizesAndElides(t *testing.T) {
	f, _ := newTree(t)
	f.ShowSizes = true
	var asked []string
	f.Icon = func(name string, dir, open bool) func(painter.Painter, Rect, RGBA) {
		asked = append(asked, name)
		if name == "deck.md" {
			return func(p painter.Painter, r Rect, ink RGBA) { fillRect(p, r.X, r.Y, r.W, r.H, ink) }
		}
		return nil // the default
	}
	f.Entries().Set(append(projectEntries(), FileEntry{Path: "a-very-long-file-name-that-will-not-fit-in-the-panel.md"}))
	f.SetFocused(true)
	f.Draw(newP(makeSurface(220, 400), 220), DefaultLight())
	if !slices.Contains(asked, "deck.md") || !slices.Contains(asked, "media") {
		t.Fatalf("Icon asked for %q", asked)
	}
	m := func(s string) int { return len([]rune(s)) * 7 }
	if got := elideToWidth("abcdefghij", 35, m); got != "abcd…" {
		t.Fatalf("elide = %q", got)
	}
	if got := elideToWidth("abc", 35, m); got != "abc" {
		t.Fatalf("elide = %q", got)
	}
	if got := elideToWidth("abc", 1, m); got != "…" {
		t.Fatalf("elide = %q", got)
	}
	for n, want := range map[int64]string{512: "512 B", 12800: "12.5 KiB", 3 << 20: "3.0 MiB"} {
		if got := FileSizeText(n); got != want {
			t.Errorf("FileSizeText(%d) = %q, want %q", n, got, want)
		}
	}
	// A message wraps to the width; a long one is bounded by the height.
	if got := wrapToWidth("one two three", 40, m); !slices.Equal(got, []string{"one", "two", "three"}) {
		t.Fatalf("wrap = %q", got)
	}
	f.SetBounds(Rect{W: 60, H: 40})
	f.AskDelete("deck.md")
	if f.footerH() != 40 {
		t.Fatalf("footer = %d, want the whole height", f.footerH())
	}
	f.Draw(newP(makeSurface(60, 40), 60), DefaultDark())
	// An edited row scrolled away is not drawn over.
	f.SetBounds(Rect{W: 220, H: 400})
	f.Dismiss()
	f.BeginRename("Zeta.md")
	f.SetBounds(Rect{W: 220, H: f.rowH() * 3})
	f.tree.ScrollRow().Set(0)
	f.Draw(newP(makeSurface(220, f.rowH()*3), 220), DefaultLight())
	f.nodes["themes"].Expanded = false
	f.BeginRename("themes/dark.css")
	if _, _, ok := f.rowRect(f.edit.node); ok {
		t.Fatal("a hidden row has no rectangle")
	}
}

func TestFileTreeStrongInks(t *testing.T) {
	weak := &Theme{Background: RGB(0xff, 0xff, 0xff), Surface: RGB(0xff, 0xff, 0xff), SurfaceAlt: RGB(0x80, 0x80, 0x80),
		Accent: RGB(0xee, 0xee, 0xff), Border: RGB(0xf0, 0xf0, 0xf0), OnSurface: RGB(0, 0, 0)}
	if c := strongInk(weak); c == weak.Accent || contrastRatio(c, weak.Surface) < 3 {
		t.Fatalf("a pale accent should give way to the popup border, got %v", c)
	}
	if c := strongError(weak); c != weak.OnSurface {
		t.Fatalf("a mid-grey footer should fall back to the text colour, got %v", c)
	}
	for name, th := range popupThemes() {
		if c := strongError(th); contrastRatio(c, th.SurfaceAlt) < 4.5 {
			t.Errorf("%s: error ink %.2f:1", name, contrastRatio(c, th.SurfaceAlt))
		}
	}
}

func TestFileTreeA11y(t *testing.T) {
	f, _ := newTree(t)
	f.Selected().Set("deck.md")
	if got := f.A11y(); got.Role != RoleTree || got.Name != "Project files" || got.Value != "deck.md" {
		t.Fatalf("A11y = %+v", got)
	}
}

// The cost of a Draw does not grow with the files outside the window.
func BenchmarkFileTreeDraw(b *testing.B) {
	var es []FileEntry
	for i := range 5000 {
		es = append(es, FileEntry{Path: "media/" + strings.Repeat("x", i%7) + string(rune('a'+i%26)) + ".png", Size: int64(i)})
	}
	f := NewFileTree(es)
	f.SetBounds(Rect{W: 240, H: 600})
	buf := makeSurface(240, 600)
	p := newP(buf, 240)
	th := DefaultLight()
	for b.Loop() {
		f.Draw(p, th)
	}
}

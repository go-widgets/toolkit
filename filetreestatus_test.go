package toolkit

import "testing"

// ⛔ THE ZERO VALUE IS THE BORING ONE, and that is the whole compatibility story:
// FileEntry is a struct literal in every consumer's code, so a status nobody sets
// has to mean "no mark". If FileUnchanged were not first, every FileEntry written
// before this existed would silently acquire a state.
func TestFileUnchangedIsTheZeroValue(t *testing.T) {
	var e FileEntry
	if e.Status != FileUnchanged {
		t.Fatalf("the zero FileEntry has status %v, want FileUnchanged", e.Status)
	}
	if got := fileStatusMark(FileUnchanged); got != "" {
		t.Errorf("FileUnchanged draws %q, want no mark", got)
	}
	if got := fileStatusName(FileUnchanged); got != "" {
		t.Errorf("FileUnchanged announces %q, want nothing", got)
	}
}

// Every state that exists must draw something and say something. A state that
// rendered as nothing would be one a consumer could set and never see.
func TestEveryStatusHasAMarkAndAName(t *testing.T) {
	for _, s := range []FileStatus{FileModified, FileAdded, FileDeleted, FileUntracked, FileConflicted} {
		if fileStatusMark(s) == "" {
			t.Errorf("status %v draws no mark", s)
		}
		if fileStatusName(s) == "" {
			t.Errorf("status %v has no spoken name", s)
		}
	}
	// The marks are the ones `git status --short`, `hg status` and `svn status`
	// all use, so a reader who knows one knows this.
	for s, want := range map[FileStatus]string{
		FileModified:   "M",
		FileAdded:      "A",
		FileDeleted:    "D",
		FileUntracked:  "?",
		FileConflicted: "!",
	} {
		if got := fileStatusMark(s); got != want {
			t.Errorf("mark for %v = %q, want %q", s, got, want)
		}
	}
	// ⛔ And an unknown value is not a crash and not a stray glyph: a consumer
	// built against a later version, or a cast from an int, falls back to no mark.
	if got := fileStatusMark(FileStatus(99)); got != "" {
		t.Errorf("an unknown status drew %q", got)
	}
	if got := fileStatusName(FileStatus(99)); got != "" {
		t.Errorf("an unknown status announced %q", got)
	}
}

// ⛔ The ink is chosen for CONTRAST, not for being the expected hue, and the
// contract is that it NEVER invents an unreadable colour: either a candidate
// clears 4.5:1 against the row, or the row's own ink is used — which is readable
// there by construction, being the colour the name is already drawn in.
//
// ⛔ Falling back is therefore correct behaviour, not a gap, and the first draft
// of this test asserted the opposite: it demanded a distinct hue and failed on
// `default-light` for three statuses. Colour is not the channel carrying the
// meaning here — the glyph is (M/A/D/?/!) — which is also why a reader who
// cannot see colour loses nothing.
func TestStatusInkIsNeverUnreadable(t *testing.T) {
	row := RGB(0x12, 0x12, 0x12)
	distinct := 0
	for name, th := range popupThemes() {
		for _, s := range []FileStatus{FileModified, FileAdded, FileDeleted, FileUntracked, FileConflicted} {
			c := fileStatusInk(th, s, row)
			if c == row {
				continue // gave way to the row ink, which is the documented fallback
			}
			distinct++
			if r := contrastRatio(c, th.SurfaceAlt); r < 4.5 {
				t.Errorf("%s: status %v ink is %.2f:1, want ≥ 4.5 or the row ink", name, s, r)
			}
		}
	}
	// And it is not vacuous: at least some themes do get a colour of their own,
	// or the whole function would be an elaborate way to return its argument.
	if distinct == 0 {
		t.Error("no theme got a status colour at all; the ink table never applies")
	}

	// A surface no candidate can clear gives way to the row's ink rather than
	// drawing something illegible.
	impossible := &Theme{SurfaceAlt: RGB(0x80, 0x80, 0x80), OnSurface: RGB(0, 0, 0)}
	if c := fileStatusInk(impossible, FileModified, row); c != row {
		t.Errorf("on a mid-grey row the mark should take the row ink, got %v", c)
	}
	// FileUnchanged has no ink of its own at all.
	if c := fileStatusInk(DefaultLight(), FileUnchanged, row); c != row {
		t.Errorf("FileUnchanged ink = %v, want the row ink", c)
	}
}

// The status reaches the row through Entries, survives a rebuild, and a folder
// keeps one a host set on it explicitly.
func TestStatusTravelsFromEntriesToTheRow(t *testing.T) {
	f := NewFileTree([]FileEntry{
		{Path: "main.tex", Status: FileModified},
		{Path: "new.tex", Status: FileAdded},
		{Path: "chapters/old.tex", Status: FileDeleted},
		{Path: "vendor", Dir: true, Status: FileUntracked},
	})
	f.SetBounds(Rect{W: 240, H: 400})

	for p, want := range map[string]FileStatus{
		"main.tex":         FileModified,
		"new.tex":          FileAdded,
		"chapters/old.tex": FileDeleted,
		"vendor":           FileUntracked,
	} {
		n := f.nodes[p]
		if n == nil {
			t.Fatalf("no node for %q", p)
		}
		if got := n.Data.(*fileNode).status; got != want {
			t.Errorf("%q has status %v, want %v", p, got, want)
		}
	}

	// ⛔ An IMPLIED folder has no status: "chapters" was never an entry, and
	// deriving one from the deleted file inside it would be a policy this widget
	// has no business inventing — git says nothing about folders at all.
	if got := f.nodes["chapters"].Data.(*fileNode).status; got != FileUnchanged {
		t.Errorf("the implied folder \"chapters\" acquired status %v", got)
	}

	// A fresh list replaces the statuses rather than merging them.
	//
	// ⚠ The rebuild is lazy — Entries' subscriber only marks the tree stale, and
	// ensure() runs on draw and on input, not on SetBounds. So the nodes have to
	// be rebuilt before they are read: the first draft of this read f.nodes
	// straight after Set, saw the OLD map, and looked exactly like a status that
	// refused to clear.
	f.Entries().Set([]FileEntry{{Path: "main.tex"}})
	f.Draw(newP(makeSurface(240, 400), 240), DefaultLight())
	if got := f.nodes["main.tex"].Data.(*fileNode).status; got != FileUnchanged {
		t.Errorf("after a clean list main.tex still reads %v", got)
	}

	// And it draws, in both themes, with and without sizes.
	f.Entries().Set([]FileEntry{{Path: "main.tex", Status: FileConflicted}})
	for _, show := range []bool{false, true} {
		f.ShowSizes = show
		f.Draw(newP(makeSurface(240, 400), 240), DefaultLight())
		f.Draw(newP(makeSurface(240, 400), 240), DefaultDark())
	}
}

// ⛔ The mark must reach A11y, because the mark itself is a glyph in a colour and
// both are unavailable to a screen reader. A status that only reached drawRow
// would be information for sighted users and decoration for everyone else.
func TestA11ySpeaksTheSelectedEntrysStatus(t *testing.T) {
	f := NewFileTree([]FileEntry{{Path: "main.tex", Status: FileModified}, {Path: "clean.tex"}})
	f.Label = "Project files"
	f.SetBounds(Rect{W: 240, H: 400})

	f.Selected().Set("main.tex")
	if got := f.A11y().Value; got != "main.tex, modified" {
		t.Errorf("A11y value = %q, want the path and its state", got)
	}
	// An unchanged file says only its name — there is nothing to announce.
	f.Selected().Set("clean.tex")
	if got := f.A11y().Value; got != "clean.tex" {
		t.Errorf("A11y value = %q, want just the path", got)
	}
	// Nothing selected stays empty rather than becoming ", something".
	f.Selected().Set("")
	if got := f.A11y().Value; got != "" {
		t.Errorf("A11y value with no selection = %q", got)
	}
	// A path the tree does not hold is not a crash either.
	f.Selected().Set("gone.tex")
	if got := f.A11y().Value; got != "gone.tex" {
		t.Errorf("A11y value for an unknown path = %q", got)
	}
}

package toolkit

import (
	"strings"
	"testing"
)

// ⛔ THE ZERO VALUE IS THE BORING ONE, the same compatibility story FileStatus
// has: Annotate is a nil func in every FileTree built before it existed, so a
// nil hook must draw nothing, announce nothing, and never be called.
func TestANilAnnotateChangesNothing(t *testing.T) {
	f := NewFileTree([]FileEntry{
		{Path: "a.png", Size: 100},
		{Path: "b.png", Size: 100},
	})
	f.SetBounds(Rect{W: 240, H: 400})
	if f.Annotate != nil {
		t.Fatal("Annotate is not nil on a fresh tree")
	}
	// ⛔ Drawn as well as asked, in both themes and with sizes both ways: the
	// nil check lives inside drawRow, so a nil hook that only ever reached
	// A11y would leave the drawing path untested exactly where it would panic.
	for _, show := range []bool{false, true} {
		f.ShowSizes = show
		f.Draw(newP(makeSurface(240, 400), 240), DefaultLight())
		f.Draw(newP(makeSurface(240, 400), 240), DefaultDark())
	}
	f.Selected().Set("a.png")
	if got := f.A11y().Value; got != "a.png" {
		t.Errorf("A11y value %q, want just the path", got)
	}
}

// TestAnnotateIsAnnouncedNotOnlyDrawn is the one that matters.
//
// ⛔ A mark is a glyph in a colour, and both are unavailable to a screen
// reader. The status marks had exactly this bug once and the fix is recorded
// in A11y's comment; a host's mark is drawn by the very same code, so it would
// have inherited the very same bug — information for sighted users and
// decoration for everyone else.
func TestAnnotateIsAnnouncedNotOnlyDrawn(t *testing.T) {
	f := NewFileTree([]FileEntry{{Path: "holiday.jpg", Size: 2048}})
	f.Annotate = func(p string, dir bool) string {
		if p == "holiday.jpg" {
			return "3 copies"
		}
		return ""
	}
	f.Selected().Set("holiday.jpg")

	got := f.A11y().Value
	if !strings.Contains(got, "holiday.jpg") {
		t.Errorf("A11y value %q lost the path", got)
	}
	if !strings.Contains(got, "3 copies") {
		t.Fatalf("A11y value %q does not announce the annotation", got)
	}
}

// TestAnnotatedRowsDraw exercises the drawing path, which the assertions
// above do not reach.
//
// ⛔ A11y and drawRow are different code. Everything above proves the hook is
// CALLED and SPOKEN; none of it would catch a panic, a negative width or a
// mark drawn off the row. The combinations matter because the row lays itself
// out right to left — size, then status mark, then annotation, then the elided
// name — so each thing present shifts what follows, and an annotation long
// enough to swallow the name is the case where the arithmetic goes negative.
func TestAnnotatedRowsDraw(t *testing.T) {
	for _, mark := range []string{"2", "3 copies", "DUP", strings.Repeat("very long annotation ", 8)} {
		f := NewFileTree([]FileEntry{
			{Path: "src", Dir: true},
			{Path: "src/a.png", Size: 4096, Status: FileModified},
			{Path: "src/b.png", Size: 4096},
			{Path: "c.png", Size: 100, Status: FileConflicted},
		})
		f.SetBounds(Rect{W: 240, H: 400})
		f.Annotate = func(p string, dir bool) string {
			if dir {
				return ""
			}
			return mark
		}
		for _, show := range []bool{false, true} {
			f.ShowSizes = show
			f.Draw(newP(makeSurface(240, 400), 240), DefaultLight())
			f.Draw(newP(makeSurface(240, 400), 240), DefaultDark())
		}
		// And in a pane too narrow for any of it, which is where a width
		// computed by subtraction turns negative.
		f.SetBounds(Rect{W: 48, H: 400})
		f.Draw(newP(makeSurface(48, 400), 48), DefaultLight())
	}
}

// TestAnnotateAndStatusAreBothAnnounced: a host's mark must not displace the
// version-control one, in speech any more than on screen.
func TestAnnotateAndStatusAreBothAnnounced(t *testing.T) {
	f := NewFileTree([]FileEntry{{Path: "draft.tex", Size: 10, Status: FileModified}})
	f.Annotate = func(string, bool) string { return "not backed up" }
	f.Selected().Set("draft.tex")

	got := f.A11y().Value
	for _, want := range []string{"draft.tex", "modified", "not backed up"} {
		if !strings.Contains(got, want) {
			t.Errorf("A11y value %q is missing %q", got, want)
		}
	}
	// The status comes first: it is the widget's own vocabulary and the more
	// urgent fact.
	if strings.Index(got, "modified") > strings.Index(got, "not backed up") {
		t.Errorf("the host's note is announced before the file status: %q", got)
	}
}

// TestAnEmptyAnnotationIsNoAnnotation: returning "" must be indistinguishable
// from having no hook, so a host can annotate some rows and not others without
// the others gaining a stray comma.
func TestAnEmptyAnnotationIsNoAnnotation(t *testing.T) {
	f := NewFileTree([]FileEntry{{Path: "plain.txt", Size: 1}})
	f.Annotate = func(string, bool) string { return "" }
	f.Selected().Set("plain.txt")
	if got := f.A11y().Value; got != "plain.txt" {
		t.Errorf("A11y value %q, want just the path", got)
	}
}

// TestAnnotateSeesThePathNotTheLabel is why this hook exists at all.
//
// ⛔ Icon receives the entry's NAME, so a host cannot look anything up per
// file with it: two folders both holding "cover.jpg" are indistinguishable.
// Annotate receives the PATH, which is what a lookup needs — and a
// deduplicator's whole subject is files whose names tell you nothing.
func TestAnnotateSeesThePathNotTheLabel(t *testing.T) {
	f := NewFileTree([]FileEntry{
		{Path: "one/cover.jpg", Size: 10},
		{Path: "two/cover.jpg", Size: 10},
	})
	var seen []string
	f.Annotate = func(p string, dir bool) string {
		seen = append(seen, p)
		if p == "two/cover.jpg" {
			return "2 copies"
		}
		return ""
	}

	// A11y asks for the selected row only, which is enough to prove the hook
	// is handed a full path rather than a label.
	f.Selected().Set("two/cover.jpg")
	if got := f.A11y().Value; !strings.Contains(got, "2 copies") {
		t.Errorf("A11y value %q: the hook did not distinguish the two covers", got)
	}
	f.Selected().Set("one/cover.jpg")
	if got := f.A11y().Value; strings.Contains(got, "2 copies") {
		t.Errorf("A11y value %q: the hook marked the wrong cover.jpg", got)
	}
	for _, p := range seen {
		if !strings.Contains(p, "/") {
			t.Errorf("the hook was handed %q, which is a label and not a path", p)
		}
	}
}

// TestAnnotateIsHandedTheCleanedRelativePath pins what the doc now says the
// hook receives, which is NOT always the FileEntry.Path the host supplied.
//
// ⛔ Measured by the first consumer (#613). A host walked a directory and passed
// the absolute paths it had. The tree cleaned "/var/…/a.png" to "var/…/a.png",
// the host's map was keyed on the former, and every lookup missed. Nothing
// failed, and the tree drew a folder for every ancestor.
func TestAnnotateIsHandedTheCleanedRelativePath(t *testing.T) {
	f := NewFileTree([]FileEntry{
		{Path: "/abs/./pics/a.png", Size: 1},
		{Path: "rel/b.png", Size: 1},
		{Path: "/top.png", Size: 1},
	})
	f.SetBounds(Rect{W: 300, H: 400})
	got := map[string]bool{}
	f.Annotate = func(p string, dir bool) string {
		got[p] = true
		return ""
	}
	f.MenuExtra = func(p string, dir bool) []MenuItem {
		got["menu:"+p] = true
		return nil
	}
	for _, p := range []string{"abs/pics/a.png", "rel/b.png"} {
		f.Selected().Set(p)
		_ = f.A11y()
	}
	for _, want := range []string{"abs/pics/a.png", "rel/b.png"} {
		if !got[want] {
			t.Errorf("the hook never saw %q; it saw %v", want, got)
		}
	}
	if got["/abs/./pics/a.png"] || got["/abs/pics/a.png"] {
		t.Errorf("the hook was handed the path as supplied, %v; the doc says cleaned and relative", got)
	}
	// MenuExtra, through the context menu of a top-level row supplied as
	// "/top.png".
	f.Context(40, rowY(t, f, "top.png"))
	if !got["menu:top.png"] || got["menu:/top.png"] {
		t.Errorf("MenuExtra saw %v; want \"top.png\", the same form as Annotate", got)
	}
}

// TestTheAnnotationInkIsReadable guards the reason the host does not choose
// the colour.
//
// ⛔ A mark exists to be READ. A host picking its own colour can pick one that
// vanishes into the row, so the widget picks: a muted tone for a light row and
// one for a dark row, first to clear 4.5:1, falling back to the row's own ink,
// which always passes since the name beside it is drawn in it.
func TestTheAnnotationInkIsReadable(t *testing.T) {
	row := RGB(0x11, 0x22, 0x33)
	for _, th := range []*Theme{DefaultLight(), DefaultDark()} {
		c := fileAnnotationInk(th, row)
		if c != row && contrastRatio(c, th.SurfaceAlt) < 4.5 {
			t.Errorf("the annotation ink %v fails contrast against the row", c)
		}
	}

	// A theme no candidate can clear must fall back to the row's ink rather
	// than draw something unreadable. ⛔ The premise is CHECKED: if a
	// candidate happens to pass against this surface, the test would be
	// asserting the fallback while never reaching it.
	impossible := DefaultLight()
	impossible.SurfaceAlt = RGB(0x7f, 0x7f, 0x7f)
	reachable := false
	for _, c := range []RGBA{RGB(0x4a, 0x5a, 0x66), RGB(0x9e, 0xb2, 0xc0)} {
		if contrastRatio(c, impossible.SurfaceAlt) >= 4.5 {
			reachable = true
		}
	}
	if reachable {
		t.Skip("a candidate clears 4.5:1 against this surface, so the fallback cannot be staged here")
	}
	if c := fileAnnotationInk(impossible, row); c != row {
		t.Errorf("with no readable candidate the ink is %v, want the row's own %v", c, row)
	}
}

// TestTheAnnotationDoesNotUseTheStatusHues: the status marks own the
// eye-catching colours, because an unmerged conflict is more urgent than any
// host's note and a second set of bright marks beside them would compete.
func TestTheAnnotationDoesNotUseTheStatusHues(t *testing.T) {
	th := DefaultLight()
	row := RGB(0x11, 0x22, 0x33)
	ann := fileAnnotationInk(th, row)
	for _, s := range []FileStatus{FileModified, FileAdded, FileDeleted, FileUntracked, FileConflicted} {
		if ann == fileStatusInk(th, s, row) && ann != row {
			t.Errorf("the annotation ink is the same as status %v's", s)
		}
	}
}

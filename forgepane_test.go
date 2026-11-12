// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-widgets/mvvm"
)

// fakeForge is a ForgeModel: observables and counting commands.
type fakeForge struct {
	forge, pick                       *mvvm.Observable[int]
	repo, ref, dir, token, note, user *mvvm.Observable[string]
	remember                          *mvvm.Observable[bool]
	mode                              *mvvm.Observable[ForgeMode]
	repos                             *mvvm.Observable[[]string]
	ran                               map[string]int
	cmds                              map[string]*mvvm.Command
	off                               map[string]bool // commands that cannot execute
}

func newFakeForge() *fakeForge {
	m := &fakeForge{
		forge: mvvm.NewObservable(0), pick: mvvm.NewObservable(0),
		repo: mvvm.NewObservable("a/b"), ref: mvvm.NewObservable(""), dir: mvvm.NewObservable(""),
		token: mvvm.NewObservable(""), note: mvvm.NewObservable("hello"), user: mvvm.NewObservable(""),
		remember: mvvm.NewObservable(false),
		mode:     mvvm.NewObservable(ForgeClient),
		repos:    mvvm.NewObservableEq([]string(nil), slices.Equal[[]string]),
		ran:      map[string]int{}, cmds: map[string]*mvvm.Command{}, off: map[string]bool{},
	}
	for _, n := range []string{"connect", "disconnect", "signin", "ssignin", "load", "propose", "refresh", "theirs", "open"} {
		m.cmds[n] = mvvm.NewCommand(func() { m.ran[n]++ }, func() bool { return !m.off[n] })
	}
	return m
}

func (m *fakeForge) Forges() []string                 { return []string{"GitHub", "GitLab"} }
func (m *fakeForge) Forge() *mvvm.Observable[int]     { return m.forge }
func (m *fakeForge) Repo() *mvvm.Observable[string]   { return m.repo }
func (m *fakeForge) Ref() *mvvm.Observable[string]    { return m.ref }
func (m *fakeForge) Dir() *mvvm.Observable[string]    { return m.dir }
func (m *fakeForge) Token() *mvvm.Observable[string]  { return m.token }
func (m *fakeForge) Remember() *mvvm.Observable[bool] { return m.remember }
func (m *fakeForge) Note() *mvvm.Observable[string]   { return m.note }
func (m *fakeForge) Mode() *mvvm.Observable[ForgeMode] {
	return m.mode
}
func (m *fakeForge) User() *mvvm.Observable[string]    { return m.user }
func (m *fakeForge) Repos() *mvvm.Observable[[]string] { return m.repos }
func (m *fakeForge) RepoPick() *mvvm.Observable[int]   { return m.pick }
func (m *fakeForge) Connect() *mvvm.Command            { return m.cmds["connect"] }
func (m *fakeForge) Disconnect() *mvvm.Command         { return m.cmds["disconnect"] }
func (m *fakeForge) SignIn() *mvvm.Command             { return m.cmds["signin"] }
func (m *fakeForge) ServerSignIn() *mvvm.Command       { return m.cmds["ssignin"] }
func (m *fakeForge) Load() *mvvm.Command               { return m.cmds["load"] }
func (m *fakeForge) Propose() *mvvm.Command            { return m.cmds["propose"] }
func (m *fakeForge) Refresh() *mvvm.Command            { return m.cmds["refresh"] }
func (m *fakeForge) TakeTheirs() *mvvm.Command         { return m.cmds["theirs"] }
func (m *fakeForge) OpenLink() *mvvm.Command           { return m.cmds["open"] }

// laidOutForge is a pane of m given a size and drawn once, so every control has
// bounds.
func laidOutForge(m ForgeModel, w, h int) *ForgePane {
	f := NewForgePane(m)
	f.SetBounds(Rect{W: w, H: h})
	f.Draw(newP(makeSurface(w, h), w), DefaultLight())
	return f
}

func TestForgePaneBindsTheFieldsBothWays(t *testing.T) {
	m := newFakeForge()
	m.ref.Set("main")
	f := NewForgePane(m)
	defer f.Close()
	c := f.client
	if c.repo.Text().Get() != "a/b" || c.ref.Text().Get() != "main" {
		t.Fatalf("model -> view: %q %q", c.repo.Text().Get(), c.ref.Text().Get())
	}
	c.repo.Text().Set("c/d") // the person typed
	c.token.Text().Set("secret")
	c.remember.Checked().Set(true)
	c.forge.Selected().Set(1)
	if m.repo.Get() != "c/d" || m.token.Get() != "secret" || !m.remember.Get() || m.forge.Get() != 1 {
		t.Errorf("view -> model: %q %q %v %d", m.repo.Get(), m.token.Get(), m.remember.Get(), m.forge.Get())
	}
	m.token.Set("") // the model empties the field at once
	if c.token.Text().Get() != "" {
		t.Error("the masked field kept its text")
	}
	if c.token.Mask == 0 {
		t.Error("the token field is not masked")
	}
	if got := c.forge.Options; !slices.Equal(got, []string{"GitHub", "GitLab"}) {
		t.Errorf("forges %v", got)
	}
	if f.frame.repo.Text().Get() != "c/d" || f.server.ref.Text().Get() != "main" {
		t.Error("every layout shows the same model")
	}
}

func TestForgePaneButtonsRunTheModelsCommandsAndGreyWhenTheyCannot(t *testing.T) {
	m := newFakeForge()
	m.off["load"] = true
	f := NewForgePane(m)
	defer f.Close()
	c := f.client
	for name, b := range map[string]*Button{
		"connect": c.connect, "disconnect": c.disconnect, "signin": c.signin, "load": c.load, "propose": c.propose,
		"refresh": c.refresh, "theirs": c.theirs, "open": c.open,
	} {
		b.OnClick()
		want := 1
		if name == "load" {
			want = 0 // cannot execute: Execute refuses
		}
		if m.ran[name] != want {
			t.Errorf("%s ran %d times, want %d", name, m.ran[name], want)
		}
	}
	if !c.load.Disabled().Get() || c.propose.Disabled().Get() {
		t.Errorf("greying: load %v propose %v", c.load.Disabled().Get(), c.propose.Disabled().Get())
	}
	m.off["load"] = false
	m.cmds["load"].RaiseCanExecuteChanged()
	if c.load.Disabled().Get() {
		t.Error("load stayed grey after it became possible")
	}
	f.server.signin.OnClick()
	if m.ran["ssignin"] != 1 {
		t.Error("the server's sign-in is its own command")
	}
}

func TestForgePaneShowsTheLayoutOfTheMode(t *testing.T) {
	m := newFakeForge()
	var painted int
	f := NewForgePane(m)
	f.Invalidate = func() { painted++ }
	for mode, page := range map[ForgeMode]string{ForgeClient: "client", ForgeFrame: "frame", ForgeServer: "server"} {
		m.mode.Set(mode)
		if got := f.Visible().Get(); got != page {
			t.Errorf("mode %d shows %q, want %q", mode, got, page)
		}
	}
	if painted == 0 {
		t.Error("a change of mode did not ask for a repaint")
	}
	if kids := f.Children(); len(kids) != 1 {
		t.Errorf("a layout is one child, got %d", len(kids))
	}
}

func TestForgePaneServerLayoutWhoAndRepositories(t *testing.T) {
	m := newFakeForge()
	m.user.Set("ada")
	m.repos.Set([]string{"o/one", "o/two"})
	m.pick.Set(1)
	f := NewForgePane(m)
	defer f.Close()
	s := f.server
	if got := s.user.Text().Get(); got != "Signed in as ada" {
		t.Errorf("who: %q", got)
	}
	if !slices.Equal(s.pick.Options, []string{"o/one", "o/two"}) || s.pick.Selected().Get() != 1 {
		t.Errorf("repos %v pick %d", s.pick.Options, s.pick.Selected().Get())
	}
	m.user.Set("")
	if s.user.Text().Get() != "" {
		t.Error("nobody signed in must show nothing")
	}
	m.repos.Set(nil)
	if len(s.pick.Options) != 0 {
		t.Errorf("no repositories: %v", s.pick.Options)
	}
	s.pick.Selected().Set(0)
	m.repos.Set([]string{"x/y", "x/z"})
	s.pick.Selected().Set(1)
	if m.pick.Get() != 1 {
		t.Error("the person's choice did not reach the model")
	}
}

func TestForgePaneWrapsTheNoteToItsWidth(t *testing.T) {
	m := newFakeForge()
	m.note.Set("one two three four five six seven eight nine ten eleven twelve thirteen fourteen")
	f := NewForgePane(m)
	defer f.Close()
	narrow := f.client.note.Text().Get()
	if strings.Contains(narrow, "\n") != true {
		// a width of 0 wraps at the 20-column floor
		t.Errorf("not wrapped at the floor: %q", narrow)
	}
	f.SetBounds(Rect{W: 4000, H: 600})
	if wide := f.client.note.Text().Get(); strings.Contains(wide, "\n") {
		t.Errorf("a wide pane still wraps: %q", wide)
	}
	f.SetBounds(Rect{W: 4000, H: 600}) // same text, same width: nothing to do
	m.note.Set("short")
	for _, tv := range []*TextView{f.client.note, f.frame.note, f.server.note} {
		if tv.Text().Get() != "short" {
			t.Errorf("a layout missed the note: %q", tv.Text().Get())
		}
	}
	f.NoteWrap = func(s string, cols int) string { return strings.ToUpper(s) }
	m.note.Set("shout")
	f.rewrap(true)
	if f.client.note.Text().Get() != "SHOUT" {
		t.Error("the host's NoteWrap is not used")
	}
}

func TestWrapNoteWords(t *testing.T) {
	for _, tc := range []struct {
		in   string
		cols int
		want string
	}{
		{"", 10, ""},
		{"short", 10, "short"},
		{"aaa bbb ccc", 7, "aaa bbb\nccc"},
		{"aaa\n\nbbb", 10, "aaa\n\nbbb"},
		{"abcdefghij", 4, "abcd\nefgh\nij"},
		{"xx abcdefghij", 4, "xx\nabcd\nefgh\nij"},
		{"abcdefgh", 4, "abcd\nefgh"},
	} {
		if got := wrapNoteWords(tc.in, tc.cols); got != tc.want {
			t.Errorf("wrap(%q,%d) = %q, want %q", tc.in, tc.cols, got, tc.want)
		}
	}
}

func TestForgePaneFrameSlotAndParts(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(m, 300, 900)
	defer f.Close()
	if _, ok := f.FrameRect(); ok {
		t.Error("the client layout has no frame slot")
	}
	client := f.Parts()
	for _, k := range []string{"forge", "repo", "ref", "dir", "token", "remember", "signin", "connect", "disconnect", "load", "propose", "refresh", "theirs", "open"} {
		if r, ok := client[k]; !ok || r.W == 0 {
			t.Errorf("client part %q: %v %v", k, r, ok)
		}
	}
	if _, ok := client["frame"]; ok {
		t.Error("client has no frame")
	}

	m.mode.Set(ForgeFrame)
	f.Draw(newP(makeSurface(300, 900), 300), DefaultLight())
	r, ok := f.FrameRect()
	if !ok || r.H != scaled(forgeSlotH) {
		t.Errorf("frame slot %v %v", r, ok)
	}
	parts := f.Parts()
	if parts["frame"] != r {
		t.Error("Parts and FrameRect disagree")
	}
	for _, k := range []string{"token", "remember", "connect", "disconnect", "forge", "signin"} {
		if _, has := parts[k]; has {
			t.Errorf("the frame layout has no %q", k)
		}
	}
	for _, k := range []string{"repo", "ref", "dir", "load", "propose", "refresh", "theirs", "open"} {
		if _, has := parts[k]; !has {
			t.Errorf("the frame layout lacks %q", k)
		}
	}

	m.mode.Set(ForgeServer)
	f.Draw(newP(makeSurface(300, 900), 300), DefaultLight())
	srv := f.Parts()
	for _, k := range []string{"ref", "dir", "load", "propose", "refresh", "theirs", "open", "pick", "ssignin"} {
		if r, ok := srv[k]; !ok || r.W == 0 {
			t.Errorf("server part %q: %v %v", k, r, ok)
		}
	}
	if _, has := srv["repo"]; has {
		t.Error("the server layout picks a repository from a list")
	}
	if _, ok := f.FrameRect(); ok {
		t.Error("no frame slot in the server layout")
	}

	unsized := NewForgePane(newFakeForge())
	unsized.model.Mode().Set(ForgeFrame)
	if _, ok := unsized.FrameRect(); ok {
		t.Error("a pane with no size has no slot")
	}
	if _, has := unsized.Parts()["frame"]; has {
		t.Error("no frame part before layout")
	}
}

func TestForgePaneTitlesFontAndAccessibility(t *testing.T) {
	m := newFakeForge()
	f := NewForgePane(m)
	if got := f.A11y(); got.Role != RoleGroup || got.Name != "Git" {
		t.Errorf("a11y %+v", got)
	}
	f.SetTitle("GitHub")
	if f.client.title.Text().Get() != "GitHub" || f.frame.title.Text().Get() != "GitHub" || f.A11y().Name != "GitHub" {
		t.Error("SetTitle")
	}
	if f.server.title.Text().Get() != "Git server" {
		t.Errorf("server title %q", f.server.title.Text().Get())
	}
	face := NewBitmapFont(1)
	f.SetTitleFont(face)
	for _, l := range []*Label{f.client.title, f.frame.title, f.server.title} {
		if l.FontSize != 0 || l.Font != face {
			t.Errorf("font %v size %d", l.Font, l.FontSize)
		}
	}
	f.SetTitleFont(nil)
	if f.client.title.FontSize == 0 || f.client.title.Font != nil {
		t.Error("nil restores the default size")
	}
}

func TestForgePaneCloseDetaches(t *testing.T) {
	m := newFakeForge()
	f := NewForgePane(m)
	f.Close()
	f.Close() // idempotent
	m.repo.Set("later/change")
	m.note.Set("later note")
	if f.client.repo.Text().Get() == "later/change" || f.client.note.Text().Get() == "later note" {
		t.Error("a closed pane still follows its model")
	}
}

func TestForgePaneIsWordedThroughTr(t *testing.T) {
	t.Cleanup(func() { SetLocalizer(nil) })
	SetLocalizer(func(key, english string) string {
		if strings.HasPrefix(key, "forge.") {
			return "<" + key + ">"
		}
		return ""
	})
	f := NewForgePane(newFakeForge())
	c, s := f.client, f.server
	for got, want := range map[string]string{
		c.title.Text().Get(): "<forge.title>", c.load.Label().Get(): "<forge.load>",
		c.propose.Label().Get(): "<forge.propose>", c.refresh.Label().Get(): "<forge.refresh>",
		c.theirs.Label().Get(): "<forge.take_theirs>", c.open.Label().Get(): "<forge.open_link>",
		c.connect.Label().Get(): "<forge.connect>", c.disconnect.Label().Get(): "<forge.disconnect>",
		c.signin.Label().Get(): "<forge.signin>", c.repo.Placeholder: "<forge.repo>",
		c.ref.Placeholder: "<forge.branch>", c.dir.Placeholder: "<forge.folder>", c.token.Placeholder: "<forge.token>",
		s.title.Text().Get(): "<forge.server_title>", s.reposLabel.Text().Get(): "<forge.server_repos>",
		s.signin.Label().Get(): "<forge.server_signin>",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if got := f.client.remember.Label; got != "<forge.remember>" {
		t.Errorf("remember %q", got)
	}
	f.model.User().Set("ada")
	if got := s.user.Text().Get(); got != "<forge.server_user>" && !strings.Contains(got, "forge.server_user") {
		t.Errorf("who %q", got)
	}
}

func TestForgeMessageKeysAreEnglish(t *testing.T) {
	for k, en := range MessageKeys {
		if strings.HasPrefix(k, "forge.") && en == "" {
			t.Errorf("%s has no English", k)
		}
	}
	n := 0
	for k := range MessageKeys {
		if strings.HasPrefix(k, "forge.") {
			n++
		}
	}
	if n != 18 {
		t.Errorf("%d forge keys", n)
	}
}

func TestForgePaneDrawsAndTakesEvents(t *testing.T) {
	m := newFakeForge()
	f := laidOutForge(m, 300, 900)
	defer f.Close()
	r := f.Parts()["load"]
	f.OnEvent(Event{Kind: EventClick, X: r.X + r.W/2, Y: r.Y + r.H/2})
	if m.ran["load"] != 1 {
		t.Errorf("a click on Load ran it %d times", m.ran["load"])
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"strings"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// ForgePane is the pane of an editor that works against a code forge (GitHub,
// GitLab, Gitea…): pick the forge, say which repository, branch and folder, then
// Connect (or sign in), Load, Propose changes…, Check for changes, Take theirs
// and Open the pull request, with a note that says what happened.
//
// It is the VIEW only. Everything it shows and every action it takes is behind a
// [ForgeModel], an interface of [mvvm] observables and commands: the toolkit
// links no forge code and no HTTP client, so an application satisfies the
// interface with whatever talks to the forge (a REST adapter, a frame on another
// origin that keeps the token away from the page, a server that holds the
// credentials) and the widget does not know which. The working tree's own state
// (which files differ from the remote) is not here either: that is
// [FileTree.SetRepository] over a vcs.Repository.
//
// # Three layouts
//
// The model's Mode picks one: [ForgeClient] (forge drop-down, repository, branch,
// folder, a masked token field with Remember, a sign-in button, Connect and
// Disconnect, then the actions), [ForgeFrame] (the same without the credential
// controls: a slot, [ForgePane.FrameRect], is left for the host to lay a frame
// over, and that frame has them) and [ForgeServer] (who is signed in, the
// repositories the server grants, branch and folder, then the actions).
//
// # Reactive surface
//
// Every text field, the Remember box, the forge and repository choices are
// bound two-way to the model's observables, and every button to a model command
// (its CanExecute greys it). The widget owns no state of its own beyond what it
// derives for drawing; the toolkit's MVVM gate holds it to that.
//
// # Words
//
// Every string goes through [Tr] under a "forge." key (see [MessageKeys]), so a
// host localises them with [SetLocalizer]. The note is the model's text and is
// wrapped to the pane's width by [ForgePane.NoteWrap].
type ForgePane struct {
	Stack

	// Invalidate is called after a change that comes from the model, so the host
	// repaints. May be nil.
	Invalidate func()
	// NoteWrap breaks the note into lines of at most cols cells. nil: a plain word
	// wrap at spaces that keeps the note's own line breaks. A host with wide
	// scripts or list-shaped notes sets its own.
	NoteWrap func(text string, cols int) string

	model     ForgeModel
	committer ForgeCommitter      // nil: the model has no git transport
	chooser   ForgeChooser        // nil: no forge drop-down
	locatable ForgeLocatable      // nil: no folder field
	hidden    map[Widget]struct{} // controls of a lacking capability (a nil command, no ForgeChooser or ForgeLocatable): not laid out, not in Parts
	client    *forgeClientPage
	frame     *forgeClientPage
	server    *forgeServerPage
	unbind    []func()

	wrapText  string // what the notes were last wrapped from
	wrapWidth int
}

// ForgeMode is which layout the pane shows.
type ForgeMode int

const (
	// ForgeClient: the page talks to the forge itself; it has the credential
	// controls.
	ForgeClient ForgeMode = iota
	// ForgeFrame: the credential lives in a frame the host lays over the slot
	// ([ForgePane.FrameRect]); the page has no credential control.
	ForgeFrame
	// ForgeServer: a server holds the credentials; the page shows who is signed
	// in and the repositories the server grants.
	ForgeServer
)

// ForgeModel is what a [ForgePane] shows and does. A satisfying type is a
// view-model: it holds the observables and the commands, and the pane subscribes.
// All of it runs on the UI goroutine.
//
// One rule decides every optional control: a capability the model lacks is a
// control that is not there; a state that is not yet right greys it. Commands
// that are nil, and the optional interfaces [ForgeChooser], [ForgeLocatable] and
// [ForgeCommitter] a model does not implement, leave no control: no hit target,
// no focus, no announcement, absent from [ForgePane.Parts], the layout closed up.
type ForgeModel interface {
	// Repo and Ref are the repository ("owner/name") and the branch (empty: the
	// default). The pane shows Repo's string as given, so a transport model (one
	// that clones over git's HTTP protocol rather than a forge's REST API) may put
	// a full remote URL in it.
	Repo() *mvvm.Observable[string]
	Ref() *mvvm.Observable[string]
	// Token is the masked field's text. A model empties it as soon as Connect
	// takes it.
	Token() *mvvm.Observable[string]
	// Remember is the "Remember on this device" choice.
	Remember() *mvvm.Observable[bool]
	// Note is what the pane tells the person: state, errors, what to do next.
	Note() *mvvm.Observable[string]
	// Mode is the layout in use.
	Mode() *mvvm.Observable[ForgeMode]
	// User is who the server says is signed in ("" for nobody); Repos are the
	// repositories it grants, and RepoPick the chosen one, an index into Repos.
	User() *mvvm.Observable[string]
	Repos() *mvvm.Observable[[]string]
	RepoPick() *mvvm.Observable[int]

	// The actions. A capability the model never has is a nil *mvvm.Command: the
	// pane does not show that control at all and the layout closes up. A state the
	// model is only in at the moment (not connected yet, nothing to load) is a
	// command whose CanExecute is false: the control stays, greyed. Absent
	// capability removes; absent state greys. The pane reads these commands once,
	// at construction: nil means "never offered", and a command that becomes
	// non-nil later is not picked up.
	Connect() *mvvm.Command
	Disconnect() *mvvm.Command
	// SignIn is the client layout's OAuth sign-in; ServerSignIn the server
	// layout's.
	SignIn() *mvvm.Command
	ServerSignIn() *mvvm.Command
	Load() *mvvm.Command
	// Propose sends the working copy as a new branch and opens a pull request.
	Propose() *mvvm.Command
	// Refresh is Check for changes: has the branch moved since Load?
	Refresh() *mvvm.Command
	TakeTheirs() *mvvm.Command
	// OpenLink opens the pull request Propose made.
	OpenLink() *mvvm.Command
}

// ForgeChooser is the optional forge choice of a [ForgeModel]: a model that
// speaks to several forges implements it and the pane shows the drop-down (in the
// client layout). A model for one generic git remote does not, and gets no
// drop-down. Found by type assertion, read once at construction.
type ForgeChooser interface {
	// Forges are the forge names the drop-down offers, fixed for the model's life.
	Forges() []string
	// Forge is the chosen forge, an index into Forges.
	Forge() *mvvm.Observable[int]
}

// ForgeLocatable is the optional folder of a [ForgeModel]: a model that works in
// a folder of the repository implements it and every layout shows the Folder
// field. A model with no such notion does not, and gets no field. Found by type
// assertion, read once at construction.
type ForgeLocatable interface {
	// Dir is the folder (empty: the root).
	Dir() *mvvm.Observable[string]
}

// ForgeCommitter is the optional git-transport capability of a [ForgeModel]: a
// model that also commits and pushes (over git's own HTTP protocol, say, with no
// forge REST API behind it) implements it, and [NewForgePane] discovers it by
// type assertion. The pane then shows a commit message, the author's name and
// email, and Commit and Push buttons, in all three layouts, between Load and
// Propose. A model that does not implement it gets none of this.
//
// The same rule as [ForgeModel]: a nil Commit or Push is a capability the model
// lacks and its button is not shown; a command that cannot run now is greyed.
// A transport-only model returns nil from Propose and OpenLink and shows Commit
// and Push without a dead Propose button.
type ForgeCommitter interface {
	// Message is the commit message; AuthorName and AuthorEmail the author.
	Message() *mvvm.Observable[string]
	AuthorName() *mvvm.Observable[string]
	AuthorEmail() *mvvm.Observable[string]
	// Commit records the working copy under Message; Push sends the commits.
	Commit() *mvvm.Command
	Push() *mvvm.Command
}

// Pane metrics, LOGICAL units (routed through scaled at use).
const (
	forgeRowH       = 28
	forgeTitleH     = 32
	forgeCheckH     = 24
	forgeSlotH      = 232
	forgeFontPx     = 14
	forgeGap        = 6
	forgeMinCols    = 20
	forgeNoteMargin = 2
)

// forgeClientPage is the client layout's (and the frame layout's) widgets.
type forgeClientPage struct {
	box                                *VBox
	title                              *Label
	note                               *TextView
	repo, ref, dir, token              *Entry
	remember                           *CheckButton
	connect, disconnect, load, propose *Button
	refresh, theirs, open, signin      *Button
	forge                              *DropDown
	slot                               *Label // the frame's place; nil in the client layout
	commit                             *forgeCommitRows
}

// forgeCommitRows is the git-transport controls of one layout.
type forgeCommitRows struct {
	message, name, email *Entry
	commit, push         *Button
}

// forgeServerPage is the server layout's widgets.
type forgeServerPage struct {
	box                     *VBox
	title, user, reposLabel *Label
	note                    *TextView
	pick                    *DropDown
	ref, dir                *Entry
	signin, load, propose   *Button
	refresh, theirs, open   *Button
	commit                  *forgeCommitRows
}

// NewForgePane builds the pane over m. When m also implements [ForgeCommitter]
// the pane has the git-transport controls.
func NewForgePane(m ForgeModel) *ForgePane {
	f := &ForgePane{Stack: Stack{Pages: map[string]Widget{}}, model: m, hidden: map[Widget]struct{}{}}
	f.committer, _ = m.(ForgeCommitter)
	f.chooser, _ = m.(ForgeChooser)
	f.locatable, _ = m.(ForgeLocatable)
	f.client = f.newClientPage(false)
	f.frame = f.newClientPage(true)
	f.server = f.newServerPage()
	f.AddPage("client", f.client.box)
	f.AddPage("frame", f.frame.box)
	f.AddPage("server", f.server.box)
	f.rewrap(true)
	f.unbind = append(f.unbind,
		m.Mode().Subscribe(func(ForgeMode) { f.show() }),
		m.Note().Subscribe(func(string) { f.rewrap(false) }),
	)
	f.show()
	return f
}

// Close detaches the pane from its model: nothing the model does afterwards
// reaches the widgets. Idempotent.
func (f *ForgePane) Close() {
	for _, u := range f.unbind {
		u()
	}
	f.unbind = nil
}

// Children is the layout in use, for the generic walks (accessibility, text
// runs): a layout that is not shown is not on screen.
func (f *ForgePane) Children() []Widget { return f.Stack.Children() }

// A11y names the pane for a screen reader.
func (f *ForgePane) A11y() A11yInfo {
	return A11yInfo{Role: RoleGroup, Name: f.client.title.Text().Get(), Disabled: f.Disabled().Get()}
}

// OnEvent drops every event while the pane is disabled, keys included: Stack
// would hand them straight to a focused control.
func (f *ForgePane) OnEvent(ev Event) {
	if f.Disabled().Get() {
		return
	}
	f.Stack.OnEvent(ev)
}

// SetTitle is the heading of the client and frame layouts, which the host may
// name for its forge ("GitHub"); the default is the word for "Git".
func (f *ForgePane) SetTitle(s string) {
	f.client.title.Text().Set(s)
	f.frame.title.Text().Set(s)
}

// SetTitleFont gives the headings a face of the host's own (a font that has the
// glyphs of the language in use). nil restores the default.
func (f *ForgePane) SetTitleFont(font Font) {
	for _, l := range []*Label{f.client.title, f.frame.title, f.server.title} {
		l.FontSize, l.Font = 0, font
		if font == nil {
			l.FontSize = scaled(forgeFontPx + 2)
		}
	}
}

// SetBounds places the pane and re-wraps the note to its width.
func (f *ForgePane) SetBounds(r Rect) {
	f.Stack.SetBounds(r)
	f.rewrap(false)
}

// Draw paints the layout in use.
func (f *ForgePane) Draw(p painter.Painter, theme *Theme) {
	f.rewrap(false)
	f.Stack.Draw(p, theme)
}

// FrameRect is where the host puts the credential frame: the slot of the frame
// layout, once laid out. It is false in any other layout, and before the pane
// has a size.
func (f *ForgePane) FrameRect() (Rect, bool) {
	if f.model.Mode().Get() != ForgeFrame {
		return Rect{}, false
	}
	r := f.frame.slot.Bounds()
	return r, r.W > 0 && r.H > 0
}

// Parts are the rectangles of the layout in use's controls, by name, for a test
// harness that needs to click them: "forge", "repo", "ref", "dir", "token",
// "remember", "signin", "connect", "disconnect", "load", "propose", "refresh",
// "theirs", "open", and, in the server layout, "pick" and "ssignin". The frame
// layout has no credential controls, so it lacks "forge", "token", "remember",
// "signin", "connect" and "disconnect", and has "frame" once laid out. A control
// whose capability the model lacks (a nil command, or no [ForgeChooser] for
// "forge", or no [ForgeLocatable] for "dir") is not shown and is not in the map.
// When the model is a
// [ForgeCommitter] every layout also has "message", "author_name",
// "author_email", "commit" and "push".
func (f *ForgePane) Parts() map[string]Rect {
	out := map[string]Rect{}
	put := func(m map[string]Widget) {
		for k, w := range m {
			if _, gone := f.hidden[w]; !gone {
				out[k] = w.Bounds()
			}
		}
	}
	var rows *forgeCommitRows
	switch f.model.Mode().Get() {
	case ForgeServer:
		s := f.server
		rows = s.commit
		put(map[string]Widget{
			"ref": s.ref, "dir": s.dir, "load": s.load, "propose": s.propose, "refresh": s.refresh,
			"theirs": s.theirs, "open": s.open, "pick": s.pick, "ssignin": s.signin,
		})
	case ForgeFrame:
		c := f.frame
		rows = c.commit
		put(map[string]Widget{
			"repo": c.repo, "ref": c.ref, "dir": c.dir, "load": c.load, "propose": c.propose,
			"refresh": c.refresh, "theirs": c.theirs, "open": c.open,
		})
		if r, ok := f.FrameRect(); ok {
			out["frame"] = r
		}
	default:
		c := f.client
		rows = c.commit
		put(map[string]Widget{
			"repo": c.repo, "ref": c.ref, "dir": c.dir, "token": c.token, "remember": c.remember,
			"connect": c.connect, "disconnect": c.disconnect, "load": c.load, "propose": c.propose,
			"refresh": c.refresh, "theirs": c.theirs, "open": c.open, "forge": c.forge, "signin": c.signin,
		})
	}
	if rows != nil {
		put(map[string]Widget{
			"message": rows.message, "author_name": rows.name, "author_email": rows.email,
			"commit": rows.commit, "push": rows.push,
		})
	}
	return out
}

// show makes the layout of the model's mode the visible one.
func (f *ForgePane) show() {
	switch f.model.Mode().Get() {
	case ForgeServer:
		f.SetVisible("server")
	case ForgeFrame:
		f.SetVisible("frame")
	default:
		f.SetVisible("client")
	}
	f.Stack.SetBounds(f.Bounds()) // the page that just came forward takes the pane's rectangle
	f.invalidate()
}

func (f *ForgePane) invalidate() {
	if f.Invalidate != nil {
		f.Invalidate()
	}
}

// rewrap sets the notes' text, wrapped to the pane's width, when the note or the
// width changed since (or always, force).
func (f *ForgePane) rewrap(force bool) {
	text, w := f.model.Note().Get(), f.Bounds().W
	if !force && text == f.wrapText && w == f.wrapWidth {
		return
	}
	f.wrapText, f.wrapWidth = text, w
	cols := max(w*100/max(scaled(forgeFontPx)*55, 1)-forgeNoteMargin, forgeMinCols)
	wrap := f.NoteWrap
	if wrap == nil {
		wrap = wrapNoteWords
	}
	s := wrap(text, cols)
	f.client.note.SetText(s)
	f.frame.note.SetText(s)
	f.server.note.SetText(s)
	f.invalidate()
}

// wrapNoteWords is the default NoteWrap: lines of at most cols runes, broken at
// spaces (a word longer than a line is broken mid-word), the note's own line
// breaks kept.
func wrapNoteWords(text string, cols int) string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		cur := ""
		for _, word := range strings.Fields(line) {
			for len([]rune(word)) > cols {
				if cur != "" {
					out = append(out, cur)
					cur = ""
				}
				r := []rune(word)
				out = append(out, string(r[:cols]))
				word = string(r[cols:])
			}
			switch {
			case cur == "":
				cur = word
			case len([]rune(cur))+1+len([]rune(word)) <= cols:
				cur += " " + word
			default:
				out = append(out, cur)
				cur = word
			}
		}
		out = append(out, cur)
	}
	return strings.Join(out, "\n")
}

// --- binding helpers ---------------------------------------------------------

// bindText two-way binds a model string to an entry.
func (f *ForgePane) bindText(src *mvvm.Observable[string], e *Entry) {
	f.unbind = append(f.unbind, mvvm.BindTwoWay(src, e.Text(), f.invalidate))
}

// bindDir binds the Folder entry to the model's Dir, or hides it when the model
// is not a [ForgeLocatable].
func (f *ForgePane) bindDir(e *Entry) {
	if f.locatable == nil {
		f.hidden[e] = struct{}{}
		return
	}
	f.bindText(f.locatable.Dir(), e)
}

// bindCommand wires a button to a model command: pressing runs it, and its
// CanExecute greys the button. A nil command is a capability the model lacks:
// the button is marked hidden and is neither laid out nor in Parts.
func (f *ForgePane) bindCommand(c *mvvm.Command, b *Button) {
	if c == nil {
		f.hidden[b] = struct{}{}
		return
	}
	f.unbind = append(f.unbind, mvvm.BindCommand(c, &b.OnClick, func(on bool) {
		b.Disabled().Set(!on)
		f.invalidate()
	}))
}

// addRow lays out the controls that are not hidden in one fixed-height row: two
// side by side, one alone, none no row at all.
func (f *ForgePane) addRow(box *VBox, ws ...Widget) {
	var shown []Widget
	for _, w := range ws {
		if _, gone := f.hidden[w]; !gone {
			shown = append(shown, w)
		}
	}
	switch len(shown) {
	case 0:
	case 1:
		box.AddFixed(shown[0], scaled(forgeRowH))
	default:
		h := NewHBox()
		h.Spacing = scaled(forgeGap)
		for _, w := range shown {
			h.AddFlex(w, 1)
		}
		box.AddFixed(h, scaled(forgeRowH))
	}
}

// newCommitRows builds the git-transport controls of one layout, bound to the
// model's ForgeCommitter side; nil when the model has none.
func (f *ForgePane) newCommitRows() *forgeCommitRows {
	m := f.committer
	if m == nil {
		return nil
	}
	r := &forgeCommitRows{
		message: NewEntry(""),
		name:    NewEntry(""),
		email:   NewEntry(""),
		commit:  NewButton(Tr("forge.commit", "Commit"), nil),
		push:    NewButton(Tr("forge.push", "Push"), nil),
	}
	r.message.Placeholder = Tr("forge.message", "Commit message")
	r.name.Placeholder = Tr("forge.author_name", "Author name")
	r.email.Placeholder = Tr("forge.author_email", "Author email")
	f.bindText(m.Message(), r.message)
	f.bindText(m.AuthorName(), r.name)
	f.bindText(m.AuthorEmail(), r.email)
	f.bindCommand(m.Commit(), r.commit)
	f.bindCommand(m.Push(), r.push)
	return r
}

// addCommitRows lays the git-transport controls out; nothing without them.
func (f *ForgePane) addCommitRows(box *VBox, r *forgeCommitRows) {
	if r == nil {
		return
	}
	box.AddFixed(r.message, scaled(forgeRowH))
	box.AddFixed(r.name, scaled(forgeRowH))
	box.AddFixed(r.email, scaled(forgeRowH))
	f.addRow(box, r.commit, r.push)
}

func newForgeTitle(key, english string) *Label {
	return NewLabel(Tr(key, english)).SetFontSize(scaled(forgeFontPx + 2))
}

// --- the client and frame layouts ---------------------------------------------

func (f *ForgePane) newClientPage(frame bool) *forgeClientPage {
	m := f.model
	p := &forgeClientPage{
		title:      newForgeTitle("forge.title", "Git"),
		note:       NewTextView(""),
		repo:       NewEntry(""),
		ref:        NewEntry(""),
		dir:        NewEntry(""),
		token:      NewEntry(""),
		remember:   NewCheckButton(Tr("forge.remember", "Remember on this device"), false),
		connect:    NewButton(Tr("forge.connect", "Connect"), nil),
		disconnect: NewButton(Tr("forge.disconnect", "Disconnect"), nil),
		load:       NewButton(Tr("forge.load", "Load"), nil),
		propose:    NewButton(Tr("forge.propose", "Propose changes…"), nil),
		refresh:    NewButton(Tr("forge.refresh", "Check for changes"), nil),
		theirs:     NewButton(Tr("forge.take_theirs", "Take theirs"), nil),
		open:       NewButton(Tr("forge.open_link", "Open the pull request"), nil),
		forge:      NewDropDown(nil, 0),
		signin:     NewButton(Tr("forge.signin", "Sign in with GitLab"), nil),
	}
	p.token.Mask = '•'
	p.repo.Placeholder = Tr("forge.repo", "Repository (owner/name or URL)")
	p.ref.Placeholder = Tr("forge.branch", "Branch (empty: the default)")
	p.dir.Placeholder = Tr("forge.folder", "Folder (empty: the root)")
	p.token.Placeholder = Tr("forge.token", "Token")

	f.bindText(m.Repo(), p.repo)
	f.bindText(m.Ref(), p.ref)
	f.bindDir(p.dir)
	f.bindText(m.Token(), p.token)
	f.unbind = append(f.unbind,
		mvvm.BindTwoWay(m.Remember(), p.remember.Checked(), f.invalidate),
	)
	if f.chooser != nil {
		p.forge.Options = f.chooser.Forges()
		// A chooser with no forge at all (Forges is read once, here) is a capability
		// present in a state not right: the drop-down stays and is greyed, so it
		// cannot open an empty list.
		p.forge.Disabled().Set(len(p.forge.Options) == 0)
		f.unbind = append(f.unbind, mvvm.BindTwoWay(f.chooser.Forge(), p.forge.Selected(), f.invalidate))
	} else {
		f.hidden[p.forge] = struct{}{}
	}
	f.bindCommand(m.Connect(), p.connect)
	f.bindCommand(m.Disconnect(), p.disconnect)
	f.bindCommand(m.SignIn(), p.signin)
	f.bindCommand(m.Load(), p.load)
	f.bindCommand(m.Propose(), p.propose)
	f.bindCommand(m.Refresh(), p.refresh)
	f.bindCommand(m.TakeTheirs(), p.theirs)
	f.bindCommand(m.OpenLink(), p.open)
	p.commit = f.newCommitRows()

	row := scaled(forgeRowH)
	cl := NewVBox()
	cl.Spacing = scaled(forgeGap)
	cl.AddFixed(p.title, scaled(forgeTitleH))
	if frame {
		// no token field, no Remember, no sign-in, no Connect: the frame has them
		p.slot = NewLabel("")
		cl.AddFixed(p.slot, scaled(forgeSlotH))
	} else {
		f.addRow(cl, p.forge)
	}
	cl.AddFixed(p.repo, row)
	cl.AddFixed(p.ref, row)
	f.addRow(cl, p.dir)
	if !frame {
		cl.AddFixed(p.token, row)
		cl.AddFixed(p.remember, scaled(forgeCheckH))
		f.addRow(cl, p.signin)
		f.addRow(cl, p.connect, p.disconnect)
	}
	f.addRow(cl, p.load, p.refresh)
	f.addCommitRows(cl, p.commit)
	f.addRow(cl, p.propose, p.theirs)
	f.addRow(cl, p.open)
	cl.AddFlex(p.note, 1)
	p.box = cl
	return p
}

// --- the server layout --------------------------------------------------------

func (f *ForgePane) newServerPage() *forgeServerPage {
	m := f.model
	p := &forgeServerPage{
		title:      newForgeTitle("forge.server_title", "Git server"),
		user:       NewLabel(""),
		reposLabel: NewLabel(Tr("forge.server_repos", "Repositories granted to you")),
		note:       NewTextView(""),
		pick:       NewDropDown(nil, 0),
		ref:        NewEntry(""),
		dir:        NewEntry(""),
		signin:     NewButton(Tr("forge.server_signin", "Sign in"), nil),
		load:       NewButton(Tr("forge.load", "Load"), nil),
		propose:    NewButton(Tr("forge.propose", "Propose changes…"), nil),
		refresh:    NewButton(Tr("forge.refresh", "Check for changes"), nil),
		theirs:     NewButton(Tr("forge.take_theirs", "Take theirs"), nil),
		open:       NewButton(Tr("forge.open_link", "Open the pull request"), nil),
	}
	p.ref.Placeholder = Tr("forge.branch", "Branch (empty: the default)")
	p.dir.Placeholder = Tr("forge.folder", "Folder (empty: the root)")

	who := func(u string) {
		if u != "" {
			p.user.Text().Set(Trf("forge.server_user", "Signed in as %s", u))
		} else {
			p.user.Text().Set("")
		}
		f.invalidate()
	}
	repos := func(r []string) {
		p.pick.Options = r
		p.pick.Selected().Set(m.RepoPick().Get())
		f.invalidate()
	}
	who(m.User().Get())
	repos(m.Repos().Get())
	f.bindText(m.Ref(), p.ref)
	f.bindDir(p.dir)
	f.unbind = append(f.unbind,
		mvvm.BindTwoWay(m.RepoPick(), p.pick.Selected(), f.invalidate),
		m.User().Subscribe(who),
		m.Repos().Subscribe(repos),
	)
	f.bindCommand(m.ServerSignIn(), p.signin)
	f.bindCommand(m.Load(), p.load)
	f.bindCommand(m.Propose(), p.propose)
	f.bindCommand(m.Refresh(), p.refresh)
	f.bindCommand(m.TakeTheirs(), p.theirs)
	f.bindCommand(m.OpenLink(), p.open)
	p.commit = f.newCommitRows()

	row := scaled(forgeRowH)
	b := NewVBox()
	b.Spacing = scaled(forgeGap)
	b.AddFixed(p.title, scaled(forgeTitleH))
	b.AddFixed(p.user, scaled(forgeCheckH))
	f.addRow(b, p.signin)
	b.AddFixed(p.reposLabel, scaled(forgeCheckH))
	b.AddFixed(p.pick, row)
	b.AddFixed(p.ref, row)
	f.addRow(b, p.dir)
	f.addRow(b, p.load, p.refresh)
	f.addCommitRows(b, p.commit)
	f.addRow(b, p.propose, p.theirs)
	f.addRow(b, p.open)
	b.AddFlex(p.note, 1)
	p.box = b
	return p
}

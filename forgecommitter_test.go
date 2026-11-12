// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-widgets/mvvm"
)

// fakeCommitter is a ForgeModel that is also a ForgeCommitter. Commands named
// in nilCmd are returned as nil (a capability the model lacks).
type fakeCommitter struct {
	*fakeForge
	message, author, email *mvvm.Observable[string]
	nilCmd                 map[string]bool
}

func newFakeCommitter() *fakeCommitter {
	m := &fakeCommitter{
		fakeForge: newFakeForge(),
		message:   mvvm.NewObservable(""), author: mvvm.NewObservable("Ada"), email: mvvm.NewObservable("ada@example.org"),
		nilCmd: map[string]bool{},
	}
	for _, n := range []string{"commit", "push"} {
		m.cmds[n] = mvvm.NewCommand(func() { m.ran[n]++ }, func() bool { return !m.off[n] })
	}
	return m
}

func (m *fakeCommitter) cmd(n string) *mvvm.Command {
	if m.nilCmd[n] {
		return nil
	}
	return m.cmds[n]
}
func (m *fakeCommitter) Message() *mvvm.Observable[string]     { return m.message }
func (m *fakeCommitter) AuthorName() *mvvm.Observable[string]  { return m.author }
func (m *fakeCommitter) AuthorEmail() *mvvm.Observable[string] { return m.email }
func (m *fakeCommitter) Commit() *mvvm.Command                 { return m.cmd("commit") }
func (m *fakeCommitter) Push() *mvvm.Command                   { return m.cmd("push") }
func (m *fakeCommitter) Propose() *mvvm.Command                { return m.cmd("propose") }
func (m *fakeCommitter) OpenLink() *mvvm.Command               { return m.cmd("open") }
func (m *fakeCommitter) Load() *mvvm.Command                   { return m.cmd("load") }
func (m *fakeCommitter) Refresh() *mvvm.Command                { return m.cmd("refresh") }
func (m *fakeCommitter) TakeTheirs() *mvvm.Command             { return m.cmd("theirs") }

var commitParts = []string{"message", "author_name", "author_email", "commit", "push"}

func TestForgeCommitterRowsInEveryLayout(t *testing.T) {
	m := newFakeCommitter()
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		m.mode.Set(mode)
		f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
		parts := f.Parts()
		for _, k := range commitParts {
			if r, ok := parts[k]; !ok || r.W == 0 || r.H == 0 {
				t.Errorf("mode %d: part %q = %v %v", mode, k, r, ok)
			}
		}
		if parts["commit"].Y != parts["push"].Y || parts["commit"].X >= parts["push"].X {
			t.Errorf("mode %d: Commit and Push share a row, Commit first: %v %v", mode, parts["commit"], parts["push"])
		}
		if !(parts["load"].Y < parts["message"].Y && parts["email"].Y < parts["propose"].Y) {
			t.Errorf("mode %d: the commit rows sit between Load and Propose", mode)
		}
	}
	if f.client.commit.message.Placeholder != "Commit message" || f.client.commit.commit.Label().Get() != "Commit" {
		t.Error("English words")
	}
}

func TestForgeWithoutCommitterHasNoRows(t *testing.T) {
	f := laidOutForge(newFakeForge(), 300, 1200)
	defer f.Close()
	if f.committer != nil || f.client.commit != nil || f.frame.commit != nil || f.server.commit != nil {
		t.Fatal("a plain ForgeModel got commit controls")
	}
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		f.model.Mode().Set(mode)
		for _, k := range commitParts {
			if _, ok := f.Parts()[k]; ok {
				t.Errorf("mode %d has %q without the capability", mode, k)
			}
		}
	}
}

func TestForgeCommitterBindsAndRunsCommands(t *testing.T) {
	m := newFakeCommitter()
	m.off["push"] = true
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	c := f.client.commit
	if c.name.Text().Get() != "Ada" || c.email.Text().Get() != "ada@example.org" {
		t.Fatal("model -> view")
	}
	c.message.Text().Set("fix: typo") // the person typed
	c.name.Text().Set("Bob")
	c.email.Text().Set("bob@example.org")
	if m.message.Get() != "fix: typo" || m.author.Get() != "Bob" || m.email.Get() != "bob@example.org" {
		t.Errorf("view -> model: %q %q %q", m.message.Get(), m.author.Get(), m.email.Get())
	}
	m.message.Set("") // the model clears it after a commit
	if c.message.Text().Get() != "" {
		t.Error("the entry kept a cleared message")
	}
	// by click, at the rectangle Parts reports
	r := f.Parts()["commit"]
	f.OnEvent(Event{Kind: EventClick, X: r.X + r.W/2, Y: r.Y + r.H/2})
	// by key, on the focused button
	c.commit.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if m.ran["commit"] != 2 {
		t.Errorf("commit ran %d times", m.ran["commit"])
	}
	// a state, not a capability: Push is shown and greyed, and refuses
	if _, ok := f.Parts()["push"]; !ok || !c.push.Disabled().Get() {
		t.Error("a push that cannot run now must stay, greyed")
	}
	c.push.OnEvent(Event{Kind: EventClick})
	if m.ran["push"] != 0 {
		t.Error("a greyed Push ran")
	}
	m.off["push"] = false
	m.cmds["push"].RaiseCanExecuteChanged()
	c.push.OnEvent(Event{Kind: EventKeyDown, Code: " "})
	if c.push.Disabled().Get() || m.ran["push"] != 1 {
		t.Errorf("push after it became possible: ran %d", m.ran["push"])
	}
	// the other layouts follow the same model
	m.mode.Set(ForgeServer)
	f.server.commit.email.Text().Set("c@example.org")
	if m.email.Get() != "c@example.org" {
		t.Error("the server layout's rows are bound too")
	}
}

func TestForgeNilCommandHidesAndCannotRunGreys(t *testing.T) {
	m := newFakeCommitter()
	m.nilCmd["propose"], m.nilCmd["open"] = true, true
	m.off["load"] = true // a state: greyed, still there
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	for _, mode := range []ForgeMode{ForgeClient, ForgeFrame, ForgeServer} {
		m.mode.Set(mode)
		f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
		parts := f.Parts()
		for _, k := range []string{"propose", "open"} {
			if _, ok := parts[k]; ok {
				t.Errorf("mode %d: nil command still shows %q", mode, k)
			}
		}
		for _, k := range []string{"theirs", "load", "commit", "push"} {
			if _, ok := parts[k]; !ok {
				t.Errorf("mode %d: %q vanished", mode, k)
			}
		}
	}
	if !f.client.load.Disabled().Get() {
		t.Error("a command that cannot run is greyed, not hidden")
	}
	// Theirs takes the row Propose left, alone and full width
	m.mode.Set(ForgeClient)
	f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
	if th, ld := f.Parts()["theirs"], f.Parts()["load"]; th.W <= ld.W {
		t.Errorf("a lone control takes the row: theirs %v load %v", th, ld)
	}
	if m.Propose() != nil || m.OpenLink() != nil {
		t.Fatal("test model")
	}
}

func TestForgeRowWithNothingLeftClosesUp(t *testing.T) {
	m := newFakeCommitter()
	m.nilCmd["propose"], m.nilCmd["theirs"], m.nilCmd["open"] = true, true, true
	m.nilCmd["commit"] = true
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	parts := f.Parts()
	for _, k := range []string{"propose", "theirs", "open", "commit"} {
		if _, ok := parts[k]; ok {
			t.Errorf("%q is shown", k)
		}
	}
	if parts["push"].X != parts["message"].X {
		t.Errorf("a lone Push takes the row: %v vs %v", parts["push"], parts["message"])
	}
}

func TestForgeNilConnectAndSignInHide(t *testing.T) {
	m := &noCredForge{newFakeForge()}
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	for _, k := range []string{"connect", "disconnect", "signin"} {
		if _, ok := f.Parts()[k]; ok {
			t.Errorf("%q shown for a nil command", k)
		}
	}
	m.mode.Set(ForgeServer)
	if _, ok := f.Parts()["ssignin"]; ok {
		t.Error("ssignin shown for a nil command")
	}
}

type noCredForge struct{ *fakeForge }

func (noCredForge) Connect() *mvvm.Command      { return nil }
func (noCredForge) Disconnect() *mvvm.Command   { return nil }
func (noCredForge) SignIn() *mvvm.Command       { return nil }
func (noCredForge) ServerSignIn() *mvvm.Command { return nil }

func TestForgeCommitterIsWordedThroughTr(t *testing.T) {
	t.Cleanup(func() { SetLocalizer(nil) })
	SetLocalizer(func(key, english string) string {
		if strings.HasPrefix(key, "forge.") {
			return "<" + key + ">"
		}
		return ""
	})
	c := NewForgePane(newFakeCommitter()).client.commit
	for got, want := range map[string]string{
		c.message.Placeholder: "<forge.message>", c.name.Placeholder: "<forge.author_name>",
		c.email.Placeholder: "<forge.author_email>", c.commit.Label().Get(): "<forge.commit>",
		c.push.Label().Get(): "<forge.push>",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

// The pane never persists, logs, echoes or copies the token: it is a masked
// entry bound to the model's Token observable, and only the model decides what
// leaves it.
func TestForgePaneNeverExposesTheTokenInClear(t *testing.T) {
	const secret = "ghp_S3cr3t-T0ken"
	defer SetClipboard(nil)
	SetClipboard(nil)
	m := newFakeCommitter()
	f := laidOutForge(m, 300, 1200)
	defer f.Close()
	m.token.Set(secret) // the model's value reaches the masked entry
	f.Draw(newP(makeSurface(300, 1200), 300), DefaultLight())
	if f.client.token.Text().Get() != secret {
		t.Fatal("the token is not bound to the model")
	}

	clear := func(where string, v any) {
		t.Helper()
		if s := fmt.Sprintf("%v %+v %#v", v, v, v); strings.Contains(s, secret) {
			t.Errorf("%s exposes the token", where)
		}
	}
	clear("Parts", f.Parts())
	clear("A11y", f.A11y())
	nodes := WalkA11y(f)
	masked := false
	for _, n := range nodes {
		clear("WalkA11y node", n)
		if n.Role == RoleTextbox && n.Value == strings.Repeat("•", len([]rune(secret))) {
			masked = true
		}
	}
	if !masked {
		t.Error("the token's accessible value is not its bullets")
	}
	clear("CollectRuns", CollectRuns(f))
	clear("Children", f.Children())

	// copy and cut on the focused token field leave the clipboard alone
	f.client.token.SetFocused(true)
	f.client.token.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+C"})
	f.client.token.OnEvent(Event{Kind: EventKeyDown, Code: "Ctrl+X"})
	if got := ClipboardText(); strings.Contains(got, secret) {
		t.Errorf("the clipboard holds the token: %q", got)
	}
	if m.token.Get() != secret {
		t.Error("cut must not empty a masked field")
	}
	// the bindings of the other layouts and a closed pane do not echo it either
	f.Close()
	m.token.Set(secret + "2")
	clear("Parts after Close", f.Parts())
}

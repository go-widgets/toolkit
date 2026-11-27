// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"testing"
)

const testLogoSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="3" y="3" width="18" height="18" fill="red"/></svg>`

func TestLogoDrawsTheDocumentOrTheNeutralMark(t *testing.T) {
	theme := DefaultLight()
	const W, H = 80, 60
	box := Rect{X: 10, Y: 10, W: 50, H: 40}
	doc := ""
	l := NewLogo("Server logo", func() string { return doc })
	l.SetBounds(box)
	if l.HitTest(20, 20) {
		t.Error("a logo is not interactive")
	}
	if a := l.A11y(); a.Role != RoleImg || a.Name != "Server logo" {
		t.Errorf("a11y = %+v", a)
	}
	draw := func() []byte {
		buf := makeSurface(W, H)
		l.Draw(newP(buf, W), theme)
		return buf
	}
	neutral := draw()
	if paintedOutside(neutral, W, H, box) {
		t.Error("neutral mark outside the bounds")
	}
	doc = testLogoSVG
	custom := draw()
	if bytes.Equal(neutral, custom) {
		t.Fatal("the SVG must replace the neutral mark")
	}
	if paintedOutside(custom, W, H, box) {
		t.Error("logo outside the bounds")
	}
	nilDoc := NewLogo("x", nil)
	nilDoc.SetBounds(box)
	buf := makeSurface(W, H)
	nilDoc.Draw(newP(buf, W), theme) // must not panic
	if !bytes.Equal(buf, neutral) {
		t.Error("a nil Doc draws the neutral mark")
	}
}

func TestLogoRowsWrapInsideTheRow(t *testing.T) {
	const W, H = 260, 120
	row := Rect{X: 20, Y: 20, W: 200, H: 60}
	r := LogoRow{Doc: testLogoSVG, Title: "A server with a very long name " + longURL, Subtitle: "Signed in as someone with a long identity name"}
	render := LogoRows(func(int) LogoRow { return r })
	buf := makeSurface(W, H)
	render(newP(buf, W), DefaultLight(), row, 0, "", false, DefaultLight().OnSurface)
	if paintedOutside(buf, W, H, row) {
		t.Fatal("a logo row painted outside its rectangle")
	}
	short := LogoRows(func(int) LogoRow { return LogoRow{Title: "Home", Subtitle: "Connected"} })
	sbuf := makeSurface(W, H)
	short(newP(sbuf, W), DefaultLight(), row, 0, "", false, DefaultLight().OnSurface)
	if bytes.Equal(buf, sbuf) {
		t.Fatal("rows with different text must differ")
	}
}

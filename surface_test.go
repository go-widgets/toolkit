// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func surfaceOf(w, h int, frame func() ([]byte, int, int)) *Surface {
	s := NewSurface(frame)
	s.SetBounds(Rect{X: 0, Y: 0, W: w, H: h})
	return s
}

// The application's pixels land where the widget sits, unscaled: a buffer is
// composed for a size and resampling it would be inventing.
func TestSurfaceBlitsTheApplicationsPixels(t *testing.T) {
	src := gradient(6, 5)
	s := surfaceOf(6, 5, func() ([]byte, int, int) { return src, 6, 5 })
	s.SetBounds(Rect{X: 2, Y: 3, W: 6, H: 5})

	p := surface(12, 12)
	s.Draw(p, DefaultDark())

	if got := pixelAt(p.Buf, p.Width, 2, 3); got != pixelAt(src, 6, 0, 0) {
		t.Errorf("top-left = %v, want the buffer's own 0,0 = %v", got, pixelAt(src, 6, 0, 0))
	}
	if got := pixelAt(p.Buf, p.Width, 7, 7); got != pixelAt(src, 6, 5, 4) {
		t.Errorf("bottom-right = %v, want the buffer's 5,4 = %v", got, pixelAt(src, 6, 5, 4))
	}
	if pixelAt(p.Buf, p.Width, 1, 3) != (RGBA{}) {
		t.Error("painted left of the widget")
	}
}

// A buffer larger than the space it was given is cropped, not allowed over its
// neighbours.
func TestSurfaceClipsToItsBounds(t *testing.T) {
	src := gradient(10, 10)
	s := surfaceOf(4, 4, func() ([]byte, int, int) { return src, 10, 10 })

	p := surface(12, 12)
	s.Draw(p, DefaultDark())

	if pixelAt(p.Buf, p.Width, 3, 3) == (RGBA{}) {
		t.Error("nothing painted inside the bounds")
	}
	if pixelAt(p.Buf, p.Width, 5, 5) != (RGBA{}) {
		t.Error("the buffer painted past the widget's bounds")
	}
}

// Nothing to show, or nothing coherent, paints nothing rather than guessing.
func TestSurfaceDrawsNothingWithoutAUsableFrame(t *testing.T) {
	for _, tc := range []struct {
		name  string
		s     *Surface
		bound Rect
	}{
		{"no Frame", &Surface{}, Rect{W: 4, H: 4}},
		{"empty bounds", surfaceOf(0, 4, func() ([]byte, int, int) { return gradient(4, 4), 4, 4 }), Rect{W: 0, H: 4}},
		{"zero-sized buffer", surfaceOf(4, 4, func() ([]byte, int, int) { return nil, 0, 4 }), Rect{W: 4, H: 4}},
		{"buffer shorter than it claims", surfaceOf(4, 4, func() ([]byte, int, int) { return []byte{1, 2, 3}, 4, 4 }), Rect{W: 4, H: 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.s.SetBounds(tc.bound)
			p := surface(8, 8)
			tc.s.Draw(p, DefaultDark())
			for i := range p.Buf {
				if p.Buf[i] != 0 {
					t.Fatal("something was painted")
				}
			}
		})
	}
}

// Input arrives in the buffer's coordinates, which is the space the application
// composed its pixels in — anything else would make it subtract the widget's
// position itself, every time, in every handler.
func TestSurfaceTranslatesInputIntoBufferCoordinates(t *testing.T) {
	var got []Event
	s := surfaceOf(20, 20, func() ([]byte, int, int) { return gradient(20, 20), 20, 20 })
	s.SetBounds(Rect{X: 7, Y: 11, W: 20, H: 20})
	s.OnInput = func(ev Event) { got = append(got, ev) }

	s.OnEvent(Event{Kind: EventClick, X: 9, Y: 15})
	s.OnEvent(Event{Kind: EventChar, Code: "k"})

	if len(got) != 2 {
		t.Fatalf("forwarded %d events, want 2", len(got))
	}
	if got[0].X != 2 || got[0].Y != 4 {
		t.Errorf("click arrived at %d,%d, want 2,4 in buffer space", got[0].X, got[0].Y)
	}
	if got[1].Code != "k" {
		t.Errorf("character event lost its text: %+v", got[1])
	}

	// No handler is not a crash.
	(&Surface{}).OnEvent(Event{Kind: EventClick})
}

// What the application says it is showing becomes a readable tree, positioned
// on the surface rather than in the buffer.
func TestSurfaceExposesTheApplicationsElements(t *testing.T) {
	s := surfaceOf(40, 40, func() ([]byte, int, int) { return gradient(40, 40), 40, 40 })
	s.SetBounds(Rect{X: 5, Y: 6, W: 40, H: 40})
	s.Elements = func() []SurfaceElement {
		return []SurfaceElement{
			{Role: RoleText, Name: "Today", X: 1, Y: 2, W: 30, H: 10},
			{Role: RoleButton, Name: "An article", Value: "https://example.invalid", X: 1, Y: 14, W: 30, H: 10},
		}
	}

	nodes := WalkA11y(s)
	if len(nodes) != 2 {
		t.Fatalf("the walk found %d nodes, want the two elements (the surface itself is presentation)", len(nodes))
	}
	if nodes[0].Name != "Today" || nodes[0].Role != RoleText {
		t.Errorf("first node = %+v, want the text element", nodes[0])
	}
	if got := nodes[0].Rect; got.X != 6 || got.Y != 8 {
		t.Errorf("first node at %+v, want the element offset onto the surface (6,8)", got)
	}
	if nodes[1].Value != "https://example.invalid" {
		t.Errorf("second node lost its value: %+v", nodes[1])
	}
}

// An application that cannot describe itself is not a broken one: it simply has
// no children, and the walk finds nothing rather than an unnamed group.
func TestSurfaceWithoutElementsHasNoChildren(t *testing.T) {
	s := surfaceOf(10, 10, func() ([]byte, int, int) { return gradient(10, 10), 10, 10 })
	if got := s.Children(); got != nil {
		t.Errorf("Children() = %v, want nil without Elements", got)
	}
	s.Elements = func() []SurfaceElement { return nil }
	if got := s.Children(); got != nil {
		t.Errorf("Children() = %v, want nil for an empty report", got)
	}
	if len(WalkA11y(s)) != 0 {
		t.Error("a surface with nothing to say produced nodes")
	}
}

// The children are rebuilt on every call, because what the application is
// showing changes as it renders and a cached child would describe an older
// frame.
func TestSurfaceChildrenFollowTheApplication(t *testing.T) {
	name := "first"
	s := surfaceOf(10, 10, func() ([]byte, int, int) { return gradient(10, 10), 10, 10 })
	s.Elements = func() []SurfaceElement {
		return []SurfaceElement{{Role: RoleButton, Name: name, W: 5, H: 5}}
	}

	if got := WalkA11y(s)[0].Name; got != "first" {
		t.Fatalf("first walk read %q", got)
	}
	name = "second"
	if got := WalkA11y(s)[0].Name; got != "second" {
		t.Errorf("second walk read %q, want the application's current answer", got)
	}
}

// A proxy paints nothing: the application already painted it.
func TestSurfaceProxyPaintsNothing(t *testing.T) {
	p := surface(4, 4)
	proxy := &surfaceProxy{info: A11yInfo{Role: RoleButton, Name: "x"}}
	proxy.SetBounds(Rect{X: 0, Y: 0, W: 4, H: 4})
	proxy.Draw(p, DefaultDark())
	for i := range p.Buf {
		if p.Buf[i] != 0 {
			t.Fatal("a proxy painted")
		}
	}
}

// The surface reports itself as presentation, so a reader is not made to walk
// through an unnamed group to reach the content.
func TestSurfaceIsPresentation(t *testing.T) {
	if got := (&Surface{}).A11y().Role; got != RolePresentation {
		t.Errorf("Surface role = %v, want RolePresentation", got)
	}
}

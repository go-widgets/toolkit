// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-widgets/painter"
)

// renderRibbon draws r (and its popups) at width w on a surface h tall and
// returns it as an image, in theme th.
func renderRibbon(r *Ribbon, th *Theme, w, h int) *image.RGBA {
	buf := make([]byte, w*h*4)
	p := painter.NewPixelPainter(buf, w, h)
	fillRect(p, 0, 0, w, h, th.Background)
	r.SetBounds(Rect{X: 0, Y: 0, W: w, H: r.Height()})
	r.Draw(p, th)
	r.DrawPopover(p, th)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, buf)
	// the pixel buffer is RGBA straight; nothing to convert
	_ = color.RGBA{}
	return img
}

// TestRibbonGallery writes PNGs of the ribbon to the directory named by
// RIBBON_GALLERY_DIR — light and dark, at several widths, collapsed, with a menu
// open, with keytips — for a person to look at. It writes nothing (and passes)
// when the variable is unset, and refuses a directory inside a git work tree: a
// capture of a rendering belongs outside every repository.
func TestRibbonGallery(t *testing.T) {
	dir := os.Getenv("RIBBON_GALLERY_DIR")
	if dir == "" {
		t.Skip("RIBBON_GALLERY_DIR not set")
	}
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			t.Fatalf("%s is inside the git work tree %s", dir, d)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	themes := map[string]*Theme{"light": DefaultLight(), "dark": DefaultDark()}
	save := func(name string, img *image.RGBA) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	for tn, th := range themes {
		for _, w := range []int{1100, 800, 620, 480, 360} {
			run := &sampleRun{}
			r := run.ribbon()
			save(fmt.Sprintf("%s-width-%d", tn, w), renderRibbon(r, th, w, r.Height()+4))
		}
		run := &sampleRun{}
		r := run.ribbon()
		r.Collapsed().Set(true)
		save(tn+"-collapsed", renderRibbon(r, th, 900, 40))
		r = run.ribbon()
		r.SetBounds(Rect{W: 1100, H: r.Height()})
		r.hover(r.cells()[0].rect.X+4, r.cells()[0].rect.Y+4)
		r.activate(0, partArrow)
		save(tn+"-split-open", renderRibbon(r, th, 1100, 240))
		r = run.ribbon()
		r.SetBounds(Rect{W: 1100, H: r.Height()})
		r.hover(r.cells()[0].rect.X+4, r.cells()[0].rect.Y+4)
		r.Tick(0.6)
		save(tn+"-tooltip", renderRibbon(r, th, 1100, 220))
		r = run.ribbon()
		r.SetBounds(Rect{W: 1100, H: r.Height()})
		r.key(Event{Kind: EventKeyDown, Code: "Alt"})
		r.key(Event{Kind: EventKeyDown, Code: "H"})
		save(tn+"-keytips", renderRibbon(r, th, 1100, 160))
		r = run.ribbon()
		r.SetBounds(Rect{W: 620, H: r.Height()})
		r.SetFocused(true)
		r.Levels()
		if i := firstPopup(r); i >= 0 {
			r.activate(i, partMain)
		}
		save(tn+"-group-popup", renderRibbon(r, th, 620, 240))
	}
	if !strings.HasSuffix(dir, "") {
		t.Fatal("unreachable")
	}
}

// firstPopup is the index of the first folded-group button of the active tab, -1
// when no group is folded.
func firstPopup(r *Ribbon) int {
	for i, c := range r.cells() {
		if c.form == formPopup {
			return i
		}
	}
	return -1
}

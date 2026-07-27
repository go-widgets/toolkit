// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"math"
	"testing"
)

// --- rgbToHSV / hsvToRGB ---------------------------------------------------

func TestRGBToHSVAchromatic(t *testing.T) {
	cases := []struct {
		name         string
		r, g, b      uint8
		wantS, wantV float64
	}{
		{"black", 0, 0, 0, 0, 0},
		{"white", 255, 255, 255, 0, 1},
		{"mid-grey", 128, 128, 128, 0, 128.0 / 255.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, s, v := rgbToHSV(c.r, c.g, c.b)
			if h != 0 {
				t.Errorf("H = %v, want 0 (achromatic)", h)
			}
			if s != c.wantS {
				t.Errorf("S = %v, want %v", s, c.wantS)
			}
			if math.Abs(v-c.wantV) > 1e-9 {
				t.Errorf("V = %v, want %v", v, c.wantV)
			}
		})
	}
}

func TestRGBToHSVDominantChannelBranches(t *testing.T) {
	// max == rf, with (g-b)/delta negative -> exercises the h<0 wrap.
	h, s, v := rgbToHSV(200, 50, 100)
	if math.Abs(h-340) > 0.5 {
		t.Errorf("red-dominant: H = %v, want ~340", h)
	}
	if s <= 0 || v <= 0 {
		t.Errorf("red-dominant: S/V should be positive, got s=%v v=%v", s, v)
	}

	// max == gf.
	h, _, _ = rgbToHSV(50, 200, 100)
	if math.Abs(h-140) > 0.5 {
		t.Errorf("green-dominant: H = %v, want ~140", h)
	}

	// max == bf (default case).
	h, _, _ = rgbToHSV(50, 100, 200)
	if math.Abs(h-220) > 0.5 {
		t.Errorf("blue-dominant: H = %v, want ~220", h)
	}
}

func TestHSVToRGBSextants(t *testing.T) {
	cases := []struct {
		h       float64
		r, g, b uint8
	}{
		{0, 255, 0, 0},
		{90, 128, 255, 0},
		{150, 0, 255, 128},
		{210, 0, 128, 255},
		{270, 128, 0, 255},
		{330, 255, 0, 128},
	}
	for _, c := range cases {
		r, g, b := hsvToRGB(c.h, 1, 1)
		if r != c.r || g != c.g || b != c.b {
			t.Errorf("hsvToRGB(%v,1,1) = (%d,%d,%d), want (%d,%d,%d)", c.h, r, g, b, c.r, c.g, c.b)
		}
	}
}

func TestHSVToRGBWraps(t *testing.T) {
	// Negative hue wraps into [0,360) via the "h<0" adjustment.
	r1, g1, b1 := hsvToRGB(-30, 1, 1)
	r2, g2, b2 := hsvToRGB(330, 1, 1)
	if r1 != r2 || g1 != g2 || b1 != b2 {
		t.Errorf("hsvToRGB(-30,..) = (%d,%d,%d), want match with 330: (%d,%d,%d)", r1, g1, b1, r2, g2, b2)
	}
	// Hue > 360 wraps via math.Mod.
	r3, g3, b3 := hsvToRGB(390, 1, 1)
	r4, g4, b4 := hsvToRGB(30, 1, 1)
	if r3 != r4 || g3 != g4 || b3 != b4 {
		t.Errorf("hsvToRGB(390,..) = (%d,%d,%d), want match with 30: (%d,%d,%d)", r3, g3, b3, r4, g4, b4)
	}
}

func TestHSVRoundTrip(t *testing.T) {
	cases := [][3]uint8{
		{0, 0, 0},
		{255, 255, 255},
		{128, 128, 128}, // achromatic edge, S == 0
		{255, 0, 0},
		{0, 255, 0},
		{0, 0, 255},
		{255, 255, 0},
		{0, 255, 255},
		{255, 0, 255},
		{200, 50, 100},
		{50, 200, 100},
		{50, 100, 200},
		{10, 200, 80},
	}
	for _, c := range cases {
		h, s, v := rgbToHSV(c[0], c[1], c[2])
		r2, g2, b2 := hsvToRGB(h, s, v)
		if absDiff(c[0], r2) > 1 || absDiff(c[1], g2) > 1 || absDiff(c[2], b2) > 1 {
			t.Errorf("round-trip (%d,%d,%d) -> h=%v s=%v v=%v -> (%d,%d,%d)",
				c[0], c[1], c[2], h, s, v, r2, g2, b2)
		}
	}
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// --- helpers: clamp01 / to8 / clampInt ------------------------------------

func TestClamp01(t *testing.T) {
	if v := clamp01(-1); v != 0 {
		t.Errorf("clamp01(-1) = %v, want 0", v)
	}
	if v := clamp01(2); v != 1 {
		t.Errorf("clamp01(2) = %v, want 1", v)
	}
	if v := clamp01(0.5); v != 0.5 {
		t.Errorf("clamp01(0.5) = %v, want 0.5", v)
	}
}

func TestTo8(t *testing.T) {
	if v := to8(-1); v != 0 {
		t.Errorf("to8(-1) = %d, want 0", v)
	}
	if v := to8(2); v != 255 {
		t.Errorf("to8(2) = %d, want 255", v)
	}
	if v := to8(0.5); v != 128 {
		t.Errorf("to8(0.5) = %d, want 128", v)
	}
}

func TestClampInt(t *testing.T) {
	if v := clampInt(-5, 0, 10); v != 0 {
		t.Errorf("clampInt(-5,0,10) = %d, want 0", v)
	}
	if v := clampInt(15, 0, 10); v != 10 {
		t.Errorf("clampInt(15,0,10) = %d, want 10", v)
	}
	if v := clampInt(5, 0, 10); v != 5 {
		t.Errorf("clampInt(5,0,10) = %d, want 5", v)
	}
}

// --- NewColorPicker / Color / SetColor ------------------------------------

func TestNewColorPickerSeedsFromRGBA(t *testing.T) {
	cp := NewColorPicker(RGBA{R: 0, G: 128, B: 255, A: 77})
	wantH, wantS, wantV := rgbToHSV(0, 128, 255)
	if cp.H != wantH || cp.S != wantS || cp.V != wantV {
		t.Errorf("NewColorPicker HSV = (%v,%v,%v), want (%v,%v,%v)", cp.H, cp.S, cp.V, wantH, wantS, wantV)
	}
	if cp.Alpha != 77 {
		t.Errorf("NewColorPicker Alpha = %d, want 77", cp.Alpha)
	}
}

func TestColorPickerColor(t *testing.T) {
	cp := &ColorPicker{H: 210, S: 0.5, V: 0.8, Alpha: 200}
	want := cp.Color()
	r, g, b := hsvToRGB(210, 0.5, 0.8)
	if want.R != r || want.G != g || want.B != b || want.A != 200 {
		t.Errorf("Color() = %+v, want R=%d G=%d B=%d A=200", want, r, g, b)
	}
}

func TestColorPickerSetColor(t *testing.T) {
	cp := &ColorPicker{}
	cp.SetColor(RGBA{R: 10, G: 200, B: 80, A: 55})
	wantH, wantS, wantV := rgbToHSV(10, 200, 80)
	if cp.H != wantH || cp.S != wantS || cp.V != wantV || cp.Alpha != 55 {
		t.Errorf("SetColor -> (%v,%v,%v,%d), want (%v,%v,%v,55)", cp.H, cp.S, cp.V, cp.Alpha, wantH, wantS, wantV)
	}
}

// --- OnEvent: SV square ----------------------------------------------------

func newTestColorPicker() *ColorPicker {
	cp := &ColorPicker{H: 0, S: 0.5, V: 0.5, Alpha: 255}
	cp.SetBounds(Rect{X: 7, Y: 11, W: ColorPickerWidth, H: ColorPickerHeight})
	return cp
}

func TestColorPickerSVSquareClickCorners(t *testing.T) {
	cp := newTestColorPicker()
	var got RGBA
	calls := 0
	cp.OnChange = func(c RGBA) { got = c; calls++ }

	cp.OnEvent(Event{Kind: EventClick, X: 0, Y: 0})
	if cp.S != 0 || cp.V != 1 {
		t.Errorf("top-left click: S=%v V=%v, want S=0 V=1", cp.S, cp.V)
	}
	if calls != 1 {
		t.Fatalf("OnChange calls = %d, want 1", calls)
	}
	if got != cp.Color() {
		t.Errorf("OnChange payload = %+v, want %+v", got, cp.Color())
	}

	cp.OnEvent(Event{Kind: EventClick, X: ColorPickerSquareSize - 1, Y: ColorPickerSquareSize - 1})
	if cp.S != 1 || cp.V != 0 {
		t.Errorf("bottom-right click: S=%v V=%v, want S=1 V=0", cp.S, cp.V)
	}
}

func TestColorPickerSVSquareClickCenter(t *testing.T) {
	cp := newTestColorPicker()
	cp.OnEvent(Event{Kind: EventClick, X: 60, Y: 60})
	wantS := 60.0 / 119.0
	wantV := 1 - 60.0/119.0
	if math.Abs(cp.S-wantS) > 1e-9 || math.Abs(cp.V-wantV) > 1e-9 {
		t.Errorf("centre click: S=%v V=%v, want S=%v V=%v", cp.S, cp.V, wantS, wantV)
	}
}

func TestColorPickerSVSquareDragClamps(t *testing.T) {
	cp := newTestColorPicker()
	// Grab the SV square with an in-bounds click...
	cp.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	// ...then drag far outside the square: setSV must clamp both axes.
	cp.OnEvent(Event{Kind: EventMouseDrag, X: -500, Y: -500})
	if cp.S != 0 || cp.V != 1 {
		t.Errorf("drag clamp (low): S=%v V=%v, want S=0 V=1", cp.S, cp.V)
	}
	cp.OnEvent(Event{Kind: EventMouseDrag, X: 5000, Y: 5000})
	if cp.S != 1 || cp.V != 0 {
		t.Errorf("drag clamp (high): S=%v V=%v, want S=1 V=0", cp.S, cp.V)
	}
}

// --- OnEvent: hue strip -----------------------------------------------------

func TestColorPickerHueStripClick(t *testing.T) {
	cp := newTestColorPicker()
	hueX := ColorPickerSquareSize + ColorPickerGap + 5

	cp.OnEvent(Event{Kind: EventClick, X: hueX, Y: 0})
	if cp.H != 0 {
		t.Errorf("hue strip top: H=%v, want 0", cp.H)
	}

	cp.OnEvent(Event{Kind: EventClick, X: hueX, Y: 30})
	want := 360 * 30.0 / 119.0
	if math.Abs(cp.H-want) > 1e-9 {
		t.Errorf("hue strip mid: H=%v, want %v", cp.H, want)
	}

	// The bottom edge maps to hue 360, which wraps to 0 (360 == 0 on the
	// hue circle -- and matches the rendered pixel, which is fed h=360
	// too and mods down to red).
	cp.OnEvent(Event{Kind: EventClick, X: hueX, Y: ColorPickerSquareSize - 1})
	if cp.H != 0 {
		t.Errorf("hue strip bottom: H=%v, want 0 (wrap)", cp.H)
	}
}

func TestColorPickerHueStripDrag(t *testing.T) {
	cp := newTestColorPicker()
	hueX := ColorPickerSquareSize + ColorPickerGap + 5
	cp.OnEvent(Event{Kind: EventClick, X: hueX, Y: 0})
	cp.OnEvent(Event{Kind: EventMouseDrag, X: hueX, Y: 90})
	want := 360 * 90.0 / 119.0
	if math.Abs(cp.H-want) > 1e-9 {
		t.Errorf("hue strip drag: H=%v, want %v", cp.H, want)
	}
}

// --- OnEvent: alpha slider --------------------------------------------------

func TestColorPickerAlphaSliderClick(t *testing.T) {
	cp := newTestColorPicker()
	alphaY := ColorPickerSquareSize + ColorPickerGap + 1

	cp.OnEvent(Event{Kind: EventClick, X: 0, Y: alphaY})
	if cp.Alpha != 0 {
		t.Errorf("alpha slider left: Alpha=%d, want 0", cp.Alpha)
	}

	cp.OnEvent(Event{Kind: EventClick, X: ColorPickerWidth - 1, Y: alphaY})
	if cp.Alpha != 255 {
		t.Errorf("alpha slider right: Alpha=%d, want 255", cp.Alpha)
	}

	cp.OnEvent(Event{Kind: EventClick, X: 73, Y: alphaY})
	if cp.Alpha != 128 {
		t.Errorf("alpha slider mid: Alpha=%d, want 128", cp.Alpha)
	}
}

func TestColorPickerAlphaSliderDrag(t *testing.T) {
	cp := newTestColorPicker()
	alphaY := ColorPickerSquareSize + ColorPickerGap + 1
	cp.OnEvent(Event{Kind: EventClick, X: 0, Y: alphaY})
	cp.OnEvent(Event{Kind: EventMouseDrag, X: ColorPickerWidth - 1, Y: alphaY})
	if cp.Alpha != 255 {
		t.Errorf("alpha slider drag: Alpha=%d, want 255", cp.Alpha)
	}
}

// --- OnEvent: eyedropper -----------------------------------------------------

func TestColorPickerEyedropClickFiresOnEyedrop(t *testing.T) {
	cp := newTestColorPicker()
	ed := cp.eyedropRectLocal()
	eyedropCalls := 0
	changeCalls := 0
	cp.OnEyedrop = func() { eyedropCalls++ }
	cp.OnChange = func(RGBA) { changeCalls++ }

	cp.OnEvent(Event{Kind: EventClick, X: ed.X + 2, Y: ed.Y + ed.H/2})
	if eyedropCalls != 1 {
		t.Errorf("eyedrop calls = %d, want 1", eyedropCalls)
	}
	if changeCalls != 0 {
		t.Errorf("OnChange should not fire on eyedrop click, got %d calls", changeCalls)
	}
}

// --- OnEvent: misc / edges --------------------------------------------------

func TestColorPickerClickOutsideAllRegions(t *testing.T) {
	cp := newTestColorPicker()
	calls := 0
	cp.OnChange = func(RGBA) { calls++ }
	cp.OnEyedrop = func() { calls++ }

	beforeH, beforeS, beforeV, beforeA := cp.H, cp.S, cp.V, cp.Alpha
	cp.OnEvent(Event{Kind: EventClick, X: 5000, Y: 5000})
	if calls != 0 {
		t.Errorf("click outside all regions fired a callback, calls = %d", calls)
	}
	if cp.H != beforeH || cp.S != beforeS || cp.V != beforeV || cp.Alpha != beforeA {
		t.Error("click outside all regions mutated state")
	}
}

func TestColorPickerDragWithoutPriorClickIsNoop(t *testing.T) {
	cp := &ColorPicker{}
	calls := 0
	cp.OnChange = func(RGBA) { calls++ }
	cp.OnEvent(Event{Kind: EventMouseDrag, X: 60, Y: 60})
	if calls != 0 {
		t.Errorf("drag with no active region fired OnChange, calls = %d", calls)
	}
	if cp.S != 0 || cp.V != 0 {
		t.Errorf("drag with no active region mutated state: S=%v V=%v", cp.S, cp.V)
	}
}

func TestColorPickerMouseUpClearsActive(t *testing.T) {
	cp := newTestColorPicker()
	cp.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	cp.OnEvent(Event{Kind: EventMouseUp})
	sBefore, vBefore := cp.S, cp.V
	// A drag after mouse-up should not move anything: active was cleared.
	cp.OnEvent(Event{Kind: EventMouseDrag, X: 119, Y: 119})
	if cp.S != sBefore || cp.V != vBefore {
		t.Errorf("drag after mouse-up moved state: S=%v V=%v, want S=%v V=%v", cp.S, cp.V, sBefore, vBefore)
	}
}

func TestColorPickerUnhandledEventKindIsNoop(t *testing.T) {
	cp := newTestColorPicker()
	before := *cp
	cp.OnEvent(Event{Kind: EventKeyDown, Code: "Enter"})
	if cp.H != before.H || cp.S != before.S || cp.V != before.V || cp.Alpha != before.Alpha || cp.active != before.active {
		t.Error("unhandled event kind mutated state")
	}
}

// --- nil callback guards -----------------------------------------------------

func TestColorPickerNilCallbacksDontPanic(t *testing.T) {
	cp := &ColorPicker{}
	cp.OnEvent(Event{Kind: EventClick, X: 60, Y: 60}) // SV square, nil OnChange
	cp.OnEvent(Event{Kind: EventMouseDrag, X: 61, Y: 61})
	cp.OnEvent(Event{Kind: EventMouseUp})

	hueX := ColorPickerSquareSize + ColorPickerGap + 5
	cp.OnEvent(Event{Kind: EventClick, X: hueX, Y: 30}) // hue strip, nil OnChange

	alphaY := ColorPickerSquareSize + ColorPickerGap + 1
	cp.OnEvent(Event{Kind: EventClick, X: 73, Y: alphaY}) // alpha slider, nil OnChange

	ed := cp.eyedropRectLocal()
	cp.OnEvent(Event{Kind: EventClick, X: ed.X + 2, Y: ed.Y + 2}) // eyedrop, nil OnEyedrop
}

// --- Draw --------------------------------------------------------------------

func TestColorPickerDraw(t *testing.T) {
	// H=180 keeps the SV-square marker (driven by S/V) and the hue-strip
	// marker (driven by H) away from the corners/edges this test probes.
	cp := &ColorPicker{H: 180, S: 0.5, V: 0.5, Alpha: 255}
	cp.SetBounds(Rect{X: 0, Y: 0, W: ColorPickerWidth, H: ColorPickerHeight})
	surf := makeSurface(ColorPickerWidth, ColorPickerHeight)
	theme := DefaultLight()
	p := newP(surf, ColorPickerWidth)
	cp.Draw(p, theme)

	// SV square corners are governed purely by pixel position (S from x, V
	// from y) at the current hue -- independent of the marker, which sits
	// mid-square at (S=0.5, V=0.5).
	wantHueFull := func(s, v float64) RGBA {
		r, g, b := hsvToRGB(180, s, v)
		return RGBA{R: r, G: g, B: b, A: 0xFF}
	}
	if got := pixelAt(surf, ColorPickerWidth, 0, 0); got != wantHueFull(0, 1) {
		t.Errorf("SV square top-left (S=0,V=1) = %+v, want %+v", got, wantHueFull(0, 1))
	}
	if got := pixelAt(surf, ColorPickerWidth, ColorPickerSquareSize-1, 0); got != wantHueFull(1, 1) {
		t.Errorf("SV square top-right (S=1,V=1) = %+v, want %+v", got, wantHueFull(1, 1))
	}
	if got := pixelAt(surf, ColorPickerWidth, 0, ColorPickerSquareSize-1); got != wantHueFull(0, 0) {
		t.Errorf("SV square bottom-left (S=0,V=0) = %+v, want %+v", got, wantHueFull(0, 0))
	}
	if got := pixelAt(surf, ColorPickerWidth, ColorPickerSquareSize-1, ColorPickerSquareSize-1); got != wantHueFull(1, 0) {
		t.Errorf("SV square bottom-right (S=1,V=0) = %+v, want %+v", got, wantHueFull(1, 0))
	}

	// Hue strip: rows 0 and H-1 both render hue 0 (red) -- the strip's own
	// marker (H=180) sits mid-strip (row ~59) so it doesn't touch either
	// edge. A row well clear of the marker's halo (rows 58-60) confirms the
	// [120,180) sextant, where R is always 0.
	hueX := ColorPickerSquareSize + ColorPickerGap
	if got := pixelAt(surf, ColorPickerWidth, hueX, 0); got != RGB(255, 0, 0) {
		t.Errorf("hue strip top = %+v, want red", got)
	}
	if got := pixelAt(surf, ColorPickerWidth, hueX, ColorPickerSquareSize-1); got != RGB(255, 0, 0) {
		t.Errorf("hue strip bottom = %+v, want red", got)
	}
	mid := pixelAt(surf, ColorPickerWidth, hueX, 50)
	if mid.R != 0 || mid.G != 255 {
		t.Errorf("hue strip row 50 = %+v, want R=0 G=255", mid)
	}

	// Alpha slider: the border stroke owns the rect's exact edges (x=ar.X,
	// x=ar.X+ar.W-1, y=ar.Y, y=ar.Y+ar.H-1), so probe columns just inside
	// it. Low alpha (near the transparent end) reads close to the neutral
	// checkerboard; high alpha (near the opaque end) reads close to the
	// hue's own colour -- for H=180 (cyan) that means R drops sharply.
	ar := cp.alphaRectLocal()
	rowY := ar.Y + 3
	lowAlphaPx := pixelAt(surf, ColorPickerWidth, ar.X+2, rowY)
	highAlphaPx := pixelAt(surf, ColorPickerWidth, ar.X+ar.W-3, rowY)
	if highAlphaPx.R >= lowAlphaPx.R {
		t.Errorf("alpha gradient: high-alpha R=%d should be < low-alpha R=%d (trending toward cyan)",
			highAlphaPx.R, lowAlphaPx.R)
	}
	if got := pixelAt(surf, ColorPickerWidth, ar.X, ar.Y); got != theme.Border {
		t.Errorf("alpha slider border = %+v, want theme.Border %+v", got, theme.Border)
	}

	// Swatch shows the current Color(), bordered.
	sw := cp.swatchRectLocal()
	if got := pixelAt(surf, ColorPickerWidth, sw.X+sw.W/2, sw.Y+sw.H/2); got != cp.Color() {
		t.Errorf("swatch centre = %+v, want %+v", got, cp.Color())
	}
	if got := pixelAt(surf, ColorPickerWidth, sw.X, sw.Y); got != theme.Border {
		t.Errorf("swatch border = %+v, want theme.Border %+v", got, theme.Border)
	}

	// Eyedrop chip: an interior pixel (away from corners + the diagonal
	// glyph) is SurfaceAlt.
	ed := cp.eyedropRectLocal()
	if got := pixelAt(surf, ColorPickerWidth, ed.X+2, ed.Y+ed.H/2); got != theme.SurfaceAlt {
		t.Errorf("eyedrop chip interior = %+v, want theme.SurfaceAlt %+v", got, theme.SurfaceAlt)
	}
}

func TestColorPickerDrawNonZeroOffset(t *testing.T) {
	cp := &ColorPicker{H: 120, S: 1, V: 1, Alpha: 255} // pure green
	const ox, oy = 9, 4
	cp.SetBounds(Rect{X: ox, Y: oy, W: ColorPickerWidth, H: ColorPickerHeight})
	w := ox + ColorPickerWidth + 5
	h := oy + ColorPickerHeight + 5
	surf := makeSurface(w, h)
	p := newP(surf, w)
	cp.Draw(p, DefaultLight())

	sw := cp.swatchRectLocal()
	got := pixelAt(surf, w, ox+sw.X+sw.W/2, oy+sw.Y+sw.H/2)
	if got != cp.Color() {
		t.Errorf("offset swatch centre = %+v, want %+v", got, cp.Color())
	}
}

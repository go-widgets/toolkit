// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-gfx/gfx/vector"
	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/painter"
)

// InkPad is a freehand pad: a box that records pointer strokes (mouse, pen or
// touch), takes the last one back, clears, draws them smooth, and renders them to
// a picture cropped to the ink on a transparent ground -- the widget behind a
// "draw your signature here" box.
//
// A DRAWN SIGNATURE IS A PICTURE, NOT CRYPTOGRAPHY: what [InkPad.PNG] returns is
// ink, and nothing in it proves who drew it.
//
// The state is MVVM-only. [InkPad.Strokes] is the observable list of strokes and
// [InkPad.Empty] an observable flag a host binds to enable its Undo and Clear
// buttons; the points are kept NORMALISED (0..1 across the box), so the ink
// survives the box being laid out at another size.
//
// EVERYTHING IS BOUNDED, because the points come from a pointer a person does not
// control and a script can fabricate: at most [InkPad.MaxStrokes] strokes
// ([InkMaxStrokes] by default) and [InkPad.MaxPoints] points between them
// ([InkMaxPoints]); a point less than [InkMinStep] pixels from the last one is
// dropped, so a slow stroke cannot spend the budget before a quick one begins;
// and a picture is never larger than [InkRenderW] by [InkRenderMaxH] pixels.
// The first press or move the caps refuse calls [InkPad.OnFull] once; [InkPad.Undo]
// and [InkPad.Clear] re-arm it.
//
// A press starts a stroke and the widget then holds the pointer ([PointerGrabber]):
// a [ScrollView] around it forwards the drag and the release to it instead of
// panning, so no host has to route the drag by hand.
type InkPad struct {
	Base

	// MaxStrokes and MaxPoints are the caps; a value <= 0 means the default.
	MaxStrokes, MaxPoints int
	// Ink and Paper are the pen and the ground; a zero colour (A == 0) means the
	// default (dark blue on white). The picture uses the same ink.
	Ink, Paper RGBA
	// OnFull is called, once until the next Undo or Clear, with the cap that
	// stopped the ink. Nil is allowed.
	OnFull func(InkLimit)

	strokes *mvvm.ObservableList[InkStroke]
	empty   *mvvm.Observable[bool]
	points  int
	pen     bool
	warned  bool
	// boxW and boxH are the box's size, in logical pixels, when the ink was
	// drawn: the proportions and the pen scale of the picture.
	boxW, boxH float64
}

// InkPoint is one sample of a stroke, each coordinate 0..1 across the box.
type InkPoint struct{ X, Y float64 }

// InkStroke is one pen-down: the points from the press to the release.
type InkStroke []InkPoint

// InkLimit names the cap that stopped the ink.
type InkLimit int

const (
	// InkStrokesFull: the pad holds MaxStrokes strokes and a press would start one more.
	InkStrokesFull InkLimit = iota + 1
	// InkPointsFull: the strokes hold MaxPoints points between them.
	InkPointsFull
)

// The defaults and the fixed sizes.
const (
	// InkMaxStrokes is the default number of strokes: a signature is a handful.
	InkMaxStrokes = 64
	// InkMaxPoints is the default number of points over all the strokes: 8192
	// points of 16 bytes are 128 KiB at the very most.
	InkMaxPoints = 8192
	// InkMinStep is how far, in pixels, a point must be from the last to be kept.
	InkMinStep = 1.0
	// InkRenderW is how wide the whole pad is drawn into the picture, in pixels,
	// and InkRenderMaxH how tall; the picture is cropped to the ink and scaled
	// uniformly, so with its margin it is never larger than 900 x 600 x 4 bytes
	// = 2 MiB.
	InkRenderW    = 900
	InkRenderMaxH = 600
	// InkPenW is the pen's thickness in the pad's own logical pixels.
	InkPenW = 2.5
)

// ErrInkEmpty is what exporting an empty pad answers: there is nothing to place.
var ErrInkEmpty = errors.New("toolkit: the ink pad is empty")

// inkEncode is the picture encoder, replaceable so a test can watch a failed write.
//
// It is the standard library's PNG writer over the straight-alpha pixels, not
// go-gfx/gfx/codec: the root package must not link the decoder registry (see
// TestRootImportsNoInitHeavyPackage).
var inkEncode = func(w io.Writer, img *raster.Image) error {
	return png.Encode(w, &image.NRGBA{Pix: img.Pix, Stride: 4 * img.W, Rect: image.Rect(0, 0, img.W, img.H)})
}

var (
	defaultInk   = RGB(16, 32, 96)
	defaultPaper = RGB(255, 255, 255)
)

// NewInkPad is an empty pad with the default caps and colours.
func NewInkPad() *InkPad { return &InkPad{boxW: 300, boxH: 120} }

// Strokes is the strokes drawn so far as a shared observable list: a host
// subscribes to it (the list changes on every kept point). The pad's counts and
// Empty are derived from the list on every change, so a host that edits it
// (Clear, RemoveAt) leaves the pad consistent rather than stuck.
func (p *InkPad) Strokes() *mvvm.ObservableList[InkStroke] {
	if p.strokes == nil {
		p.strokes = mvvm.NewObservableList[InkStroke]()
		p.strokes.SubscribeChanged(p.sync)
	}
	return p.strokes
}

// sync re-derives the point count and Empty from the list, and lifts the pen
// when the stroke it was drawing is gone.
func (p *InkPad) sync() {
	n := 0
	for _, s := range p.strokes.Slice() {
		n += len(s)
	}
	p.points = n
	if p.strokes.Len() == 0 {
		p.pen = false
	}
	p.Empty().Set(p.strokes.Len() == 0)
}

// Empty is true while nothing is drawn, as a shared observable.
func (p *InkPad) Empty() *mvvm.Observable[bool] {
	if p.empty == nil {
		p.empty = mvvm.NewObservable(true)
	}
	return p.empty
}

func (p *InkPad) maxStrokes() int {
	if p.MaxStrokes > 0 {
		return p.MaxStrokes
	}
	return InkMaxStrokes
}

func (p *InkPad) maxPoints() int {
	if p.MaxPoints > 0 {
		return p.MaxPoints
	}
	return InkMaxPoints
}

func (p *InkPad) ink() RGBA {
	if p.Ink.A != 0 {
		return p.Ink
	}
	return defaultInk
}

func (p *InkPad) paper() RGBA {
	if p.Paper.A != 0 {
		return p.Paper
	}
	return defaultPaper
}

// size is the box's size, or a nominal one before it has been laid out.
func (p *InkPad) size() (w, h float64) {
	b := p.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return 300, 120
	}
	return float64(b.W), float64(b.H)
}

// norm turns a point in the box's frame into 0..1 across it, clamped: a pointer
// dragged out of the box ends the line at its edge rather than beyond it.
func (p *InkPad) norm(x, y int) InkPoint {
	w, h := p.size()
	return InkPoint{math.Min(math.Max(float64(x)/w, 0), 1), math.Min(math.Max(float64(y)/h, 0), 1)}
}

// refuse tells OnFull why the pad stopped listening, once.
func (p *InkPad) refuse(why InkLimit) {
	if p.warned {
		return
	}
	p.warned = true
	if p.OnFull != nil {
		p.OnFull(why)
	}
}

// begin starts a stroke where the pen came down.
func (p *InkPad) begin(x, y int) {
	p.pen = false
	if p.Strokes().Len() >= p.maxStrokes() {
		p.refuse(InkStrokesFull)
		return
	}
	if p.points >= p.maxPoints() {
		p.refuse(InkPointsFull)
		return
	}
	p.boxW, p.boxH = p.size()
	p.Strokes().Append(InkStroke{p.norm(x, y)})
	p.pen = true
}

// move extends the open stroke; a point under [InkMinStep] from the last is dropped.
func (p *InkPad) move(x, y int) {
	if !p.pen {
		return
	}
	if p.points >= p.maxPoints() {
		p.refuse(InkPointsFull)
		return
	}
	at := p.Strokes().Len() - 1
	if at < 0 || len(p.Strokes().At(at)) == 0 {
		// The host edited the list under the open stroke.
		p.pen = false
		return
	}
	line := p.Strokes().At(at)
	last, now := line[len(line)-1], p.norm(x, y)
	w, h := p.size()
	// The slack absorbs the rounding of normalising then scaling back: one
	// pixel apart must count as one pixel, not 0.99999999999999989.
	if math.Hypot((now.X-last.X)*w, (now.Y-last.Y)*h) < InkMinStep-1e-9 {
		return
	}
	p.Strokes().Set(at, append(line, now))
}

// Undo takes the last stroke back off and says whether there was one: on an empty
// pad it does nothing and answers false.
func (p *InkPad) Undo() bool {
	n := p.Strokes().Len()
	if n == 0 {
		return false
	}
	p.Strokes().RemoveAt(n - 1)
	p.pen, p.warned = false, false
	return true
}

// Clear empties the pad.
func (p *InkPad) Clear() {
	p.Strokes().Clear()
	p.pen, p.warned = false, false
}

// PointerGrabber is a widget that, once a press has been delivered to it, wants
// the rest of the gesture: the drags and the release. A container that would
// otherwise take them for itself (a [ScrollView] pans on a drag) asks, and
// forwards them instead.
type PointerGrabber interface {
	Widget
	// PointerGrabbed reports whether the widget holds the pointer now.
	PointerGrabbed() bool
}

// PointerGrabbed is true between the press that started a stroke and its release.
func (p *InkPad) PointerGrabbed() bool { return p.pen }

// CancelPointer lifts the pen without a release: the host dropped the gesture
// ([PointerRouter.Cancel]).
func (p *InkPad) CancelPointer() { p.pen = false }

// OnEvent takes the pointer: a press (or touch start) begins a stroke, a drag (or
// touch move) extends it and the release lifts the pen. A disabled pad takes no
// ink, but a release still lifts the pen: disabling it mid-stroke must not leave
// the pointer held.
func (p *InkPad) OnEvent(ev Event) {
	if ev.Kind == EventMouseUp || ev.Kind == EventTouchEnd {
		p.pen = false
		return
	}
	if p.Disabled().Get() {
		p.pen = false
		return
	}
	switch ev.Kind {
	case EventClick, EventTouchStart:
		p.begin(ev.X, ev.Y)
	case EventMouseDrag, EventTouchMove:
		p.move(ev.X, ev.Y)
	}
}

// A11y reports the pad as a group whose value says how much ink it holds.
func (p *InkPad) A11y() A11yInfo {
	v := "empty"
	if !p.Empty().Get() {
		v = "drawn"
	}
	return A11yInfo{Role: RoleGroup, Value: v, Disabled: p.Disabled().Get()}
}

// inkPath is one stroke as a smooth path in a frame scaled sx by sy and moved by
// (ox, oy). Three or more points are joined through the midpoints between them as
// quadratic curves, which turns the corners a pointer's sampling puts in a
// handwritten line back into the curve it was. A stroke of one point -- a dot --
// is given a second half a pixel away: a round-capped line of no length is nothing
// to a rasteriser, and a full stop is a mark.
func inkPath(line InkStroke, sx, sy, ox, oy float64) *vector.Path {
	pts := make([][2]float64, 0, len(line)+1)
	for _, q := range line {
		pts = append(pts, [2]float64{q.X*sx - ox, q.Y*sy - oy})
	}
	if len(pts) == 1 {
		pts = append(pts, [2]float64{pts[0][0] + 0.5, pts[0][1]})
	}
	path := vector.NewPath().MoveTo(pts[0][0], pts[0][1])
	for i := 1; i < len(pts)-1; i++ {
		path.QuadTo(pts[i][0], pts[i][1], (pts[i][0]+pts[i+1][0])/2, (pts[i][1]+pts[i+1][1])/2)
	}
	last := pts[len(pts)-1]
	return path.LineTo(last[0], last[1])
}

// Image renders the strokes into a picture cropped to the ink, with a margin of
// a pen width and a little over, on a transparent ground; ErrInkEmpty when
// nothing is drawn. CROPPED, because the pad is mostly empty paper: a picture of
// all of it would be placed as a wide strip with a small signature inside.
func (p *InkPad) Image() (*raster.Image, error) {
	strokes := p.Strokes().Slice()
	if len(strokes) == 0 {
		return nil, ErrInkEmpty
	}
	bw, bh := p.boxW, p.boxH
	if bw <= 0 || bh <= 0 {
		bw, bh = 300, 120
	}
	// The ink's extent in the pad's logical pixels.
	lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
	for _, line := range strokes {
		for _, q := range line {
			lo[0], hi[0] = math.Min(lo[0], q.X*bw), math.Max(hi[0], q.X*bw)
			lo[1], hi[1] = math.Min(lo[1], q.Y*bh), math.Max(hi[1], q.Y*bh)
		}
	}
	// One scale for both axes, so the picture keeps the drawing's proportions:
	// the whole box fits 900 x 600, and so does the ink with its margin (a pen
	// width and 2 px each side), the half pixel absorbing the rounding up.
	f := math.Min(InkRenderW/bw, InkRenderMaxH/bh)
	f = math.Min(f, (InkRenderW-4.5)/(hi[0]-lo[0]+2*InkPenW))
	f = math.Min(f, (InkRenderMaxH-4.5)/(hi[1]-lo[1]+2*InkPenW))
	sx, sy := bw*f, bh*f
	width := InkPenW * f
	margin := width + 2
	ox, oy := lo[0]*f-margin, lo[1]*f-margin
	img := raster.New(int(math.Ceil((hi[0]-lo[0])*f+2*margin)), int(math.Ceil((hi[1]-lo[1])*f+2*margin)))

	var rz vector.Rasterizer
	style := vector.StrokeStyle{Width: width, Cap: vector.RoundCap, Join: vector.RoundJoin}
	ink := color.RGBA(p.ink())
	for _, line := range strokes {
		// ok is not read: a stroke that covers nothing returns an empty grid and
		// a zero box, and compositing a zero box draws nothing.
		cov, x, y, w, h, _ := rz.StrokeWith(inkPath(line, sx, sy, ox, oy), style, img.W, img.H)
		vector.Composite(img, cov, x, y, w, h, vector.SolidPaint{Color: ink})
	}
	return img, nil
}

// PNG is [InkPad.Image] encoded as a PNG.
func (p *InkPad) PNG() ([]byte, error) {
	img, err := p.Image()
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := inkEncode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Draw paints the box: paper, a baseline to sign on, and the ink. On a painter
// without [painter.PathPainter] only the frame is drawn.
func (p *InkPad) Draw(pt painter.Painter, theme *Theme) {
	r := p.Bounds()
	paper, ink := p.paper(), p.ink()
	if p.Disabled().Get() {
		ink = blendRGBA(ink, paper, 0.5)
	}
	pt.FillRect(r, paper)
	pt.StrokeRect(r, theme.Border, 1)
	pt.FillRect(Rect{X: r.X + r.W/20, Y: r.Y + r.H*3/4, W: r.W - r.W/10, H: 1}, theme.Border)
	pp, ok := pt.(painter.PathPainter)
	if !ok {
		return
	}
	for _, line := range p.Strokes().Slice() {
		pp.StrokePath(inkPath(line, float64(r.W), float64(r.H), -float64(r.X), -float64(r.Y)), ink, InkPenW)
	}
}

var _ Accessible = (*InkPad)(nil)
var _ PointerGrabber = (*InkPad)(nil)

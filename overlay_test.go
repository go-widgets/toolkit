// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestOverlayPushPopTop(t *testing.T) {
	o := NewOverlay(&recordingWidget{})
	if o.Top() != nil || o.Pop() != nil {
		t.Fatal("empty overlay Top/Pop should be nil")
	}
	a, b := &recordingWidget{}, &recordingWidget{}
	o.Push(a)
	o.Push(b)
	if o.Top() != b {
		t.Fatal("Top should be the last pushed layer")
	}
	if o.Pop() != b || o.Top() != a {
		t.Fatal("Pop should remove+return the top, exposing the one below")
	}
	o.Clear()
	if len(o.Layers) != 0 || o.Top() != nil {
		t.Fatal("Clear should remove all layers")
	}
}

func TestOverlaySetBoundsFillsContentOnly(t *testing.T) {
	content := &recordingWidget{}
	layer := &recordingWidget{}
	layer.SetBounds(Rect{X: 5, Y: 5, W: 10, H: 10}) // self-positioned
	o := NewOverlay(content)
	o.Push(layer)
	o.SetBounds(Rect{X: 0, Y: 0, W: 100, H: 80})
	if content.Bounds() != (Rect{X: 0, Y: 0, W: 100, H: 80}) {
		t.Errorf("Content not filled: %+v", content.Bounds())
	}
	if layer.Bounds() != (Rect{X: 5, Y: 5, W: 10, H: 10}) {
		t.Errorf("layer bounds should be untouched: %+v", layer.Bounds())
	}
}

func TestOverlayDrawsContentThenLayers(t *testing.T) {
	content := &recordingWidget{}
	l1, l2 := &recordingWidget{}, &recordingWidget{}
	o := NewOverlay(content)
	o.Push(l1)
	o.Push(l2)
	o.Draw(newP(makeSurface(10, 10), 10), DefaultLight())
	if content.draws != 1 || l1.draws != 1 || l2.draws != 1 {
		t.Errorf("draw counts: content=%d l1=%d l2=%d, want 1 each",
			content.draws, l1.draws, l2.draws)
	}
}

func TestOverlayNilContentDrawNoPanic(t *testing.T) {
	o := NewOverlay(nil)
	o.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10}) // Content nil branch
	o.Draw(newP(makeSurface(10, 10), 10), DefaultLight())
	o.OnEvent(Event{Kind: EventClick, X: 1, Y: 1}) // Content nil, no layers
}

func TestOverlayEventTopLayerCaptures(t *testing.T) {
	content := &recordingWidget{}
	bottom := &recordingWidget{}
	top := &recordingWidget{}
	// Both layers cover (10,10); top is pushed last so it wins there.
	bottom.SetBounds(Rect{X: 0, Y: 0, W: 40, H: 40})
	top.SetBounds(Rect{X: 0, Y: 0, W: 20, H: 20})
	o := NewOverlay(content)
	o.Push(bottom)
	o.Push(top)

	// Point inside both → topmost captures.
	o.OnEvent(Event{Kind: EventClick, X: 10, Y: 10})
	if len(top.events) != 1 || len(bottom.events) != 0 || len(content.events) != 0 {
		t.Fatalf("top should capture: top=%d bottom=%d content=%d",
			len(top.events), len(bottom.events), len(content.events))
	}
	// Point inside bottom only (30,30 is outside top's 20x20) → bottom captures.
	o.OnEvent(Event{Kind: EventClick, X: 30, Y: 30})
	if len(bottom.events) != 1 {
		t.Fatalf("bottom should capture the miss-of-top: %d", len(bottom.events))
	}
}

func TestOverlayEventFallsThroughToContent(t *testing.T) {
	content := &recordingWidget{}
	layer := &recordingWidget{}
	layer.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	o := NewOverlay(content)
	o.Push(layer)
	// Point outside every layer → non-modal overlay routes to Content.
	o.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
	if len(content.events) != 1 || len(layer.events) != 0 {
		t.Fatalf("miss should reach content: content=%d layer=%d",
			len(content.events), len(layer.events))
	}
}

func TestOverlayModalSwallowsMiss(t *testing.T) {
	content := &recordingWidget{}
	layer := &recordingWidget{}
	layer.SetBounds(Rect{X: 0, Y: 0, W: 10, H: 10})
	o := NewOverlay(content)
	o.Modal = true
	o.Push(layer)
	// Miss while a layer is up → swallowed, Content untouched.
	o.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
	if len(content.events) != 0 {
		t.Fatalf("modal backdrop should swallow the miss, content got %d", len(content.events))
	}
	// With no layers, even Modal falls through to Content.
	o.Clear()
	o.OnEvent(Event{Kind: EventClick, X: 50, Y: 50})
	if len(content.events) != 1 {
		t.Fatalf("modal w/o layers should reach content: %d", len(content.events))
	}
}

// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

// This file wires native pointer-hover into the chart widgets. Each chart draws
// a hover cue (crosshair, ring, slice outline, spoke, bar column) from its
// Hover/HoverIndex fields but, before EventMouseMove existed, could only have
// those set by the host. Now a container forwards EventMouseMove to the chart,
// which resolves the pointer against its own hit-test helper (ValueAt /
// NearestPoint / SliceAt / AxisAt) and sets — or, when the pointer leaves it,
// clears — its hover state itself. Every other event kind is ignored; charts
// are read-only, so there is no click/drag/scroll behaviour to guard.

// OnEvent tracks the hover crosshair from the pointer: a move over the chart
// sets Hover/HoverIndex to the nearest data point, a move off the chart (a
// container forwards moves to every child) clears Hover.
func (c *LineChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if i, _, ok := c.ValueAt(ev.X); ok {
		c.Hover().Set(true)
		c.HoverIndex().Set(i)
	} else {
		c.Hover().Set(false)
	}
}

// OnEvent tracks the hover crosshair from the pointer (see LineChart.OnEvent).
func (c *AreaChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if i, _, ok := c.ValueAt(ev.X); ok {
		c.Hover().Set(true)
		c.HoverIndex().Set(i)
	} else {
		c.Hover().Set(false)
	}
}

// OnEvent outlines the bar column under the pointer, clearing when it leaves.
func (c *BarChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if i, _, ok := c.ValueAt(ev.X); ok {
		c.Hover().Set(true)
		c.HoverIndex().Set(i)
	} else {
		c.Hover().Set(false)
	}
}

// OnEvent tracks the sparkline crosshair / bar highlight from the pointer.
func (s *Sparkline) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !s.localInBounds(ev.X, ev.Y) {
		s.Hover().Set(false)
		return
	}
	if i, _, ok := s.ValueAt(ev.X); ok {
		s.Hover().Set(true)
		s.HoverIndex().Set(i)
	} else {
		s.Hover().Set(false)
	}
}

// OnEvent rings the scatter point nearest the pointer, clearing when it leaves.
func (c *ScatterChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if si, pi, _, ok := c.NearestPoint(ev.X, ev.Y); ok {
		c.Hover().Set(true)
		c.HoverSeries().Set(si)
		c.HoverPoint().Set(pi)
	} else {
		c.Hover().Set(false)
	}
}

// OnEvent outlines the pie slice under the pointer, clearing when it leaves.
func (c *PieChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if i, _, ok := c.SliceAt(ev.X, ev.Y); ok {
		c.Hover().Set(true)
		c.HoverIndex().Set(i)
	} else {
		c.Hover().Set(false)
	}
}

// OnEvent highlights the radar spoke nearest the pointer, clearing when it
// leaves.
func (c *RadarChart) OnEvent(ev Event) {
	if ev.Kind != EventMouseMove {
		return
	}
	if !c.localInBounds(ev.X, ev.Y) {
		c.Hover().Set(false)
		return
	}
	if a, ok := c.AxisAt(ev.X, ev.Y); ok {
		c.Hover().Set(true)
		c.HoverAxis().Set(a)
	} else {
		c.Hover().Set(false)
	}
}

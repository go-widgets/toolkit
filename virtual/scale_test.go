// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package virtual

import (
	"testing"

	"github.com/go-widgets/mvvm"
	"github.com/go-widgets/toolkit"
)

// TestVirtualDefaultRowHeightScales proves a VirtualList that falls back to
// DefaultRowHeight routes it through the global metric scale, so a HiDPI feed's
// default rows double at 2x (with defer-reset to 1x).
func TestVirtualDefaultRowHeightScales(t *testing.T) {
	defer toolkit.SetMetricScale(1)

	model := mvvm.NewObservableList[int]()
	model.Append(1)
	model.Append(2)

	vl1 := NewVirtualList[int](model, nil, nil)
	if vl1.idx.rowH != DefaultRowHeight {
		t.Fatalf("default rowH at 1x = %d, want %d", vl1.idx.rowH, DefaultRowHeight)
	}

	toolkit.SetMetricScale(2)
	vl2 := NewVirtualList[int](model, nil, nil)
	if vl2.idx.rowH != 2*DefaultRowHeight {
		t.Fatalf("default rowH at 2x = %d, want %d", vl2.idx.rowH, 2*DefaultRowHeight)
	}
}

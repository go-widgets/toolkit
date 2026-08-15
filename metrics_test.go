// Copyright (c) 2026 the wasmdesk/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "testing"

func TestMetricScale(t *testing.T) {
	defer SetMetricScale(1) // restore the default for other tests
	if MetricScale() != 1 {
		t.Fatalf("default MetricScale = %v, want 1", MetricScale())
	}
	if got := scaled(10); got != 10 {
		t.Fatalf("scaled(10) at 1x = %d, want 10 (identity)", got)
	}
	SetMetricScale(2)
	if MetricScale() != 2 {
		t.Fatalf("MetricScale after SetMetricScale(2) = %v, want 2", MetricScale())
	}
	if got := scaled(10); got != 20 {
		t.Fatalf("scaled(10) at 2x = %d, want 20", got)
	}
	if got := scaled(3); got != 6 { // 3*2=6, rounds exact
		t.Fatalf("scaled(3) at 2x = %d, want 6", got)
	}
	SetMetricScale(0)  // ignored (non-positive)
	SetMetricScale(-1) // ignored
	if MetricScale() != 2 {
		t.Fatalf("non-positive SetMetricScale changed scale to %v, want kept 2", MetricScale())
	}
	SetMetricScale(1.5)
	if got := scaled(3); got != 5 { // 3*1.5=4.5 → rounds to 5
		t.Fatalf("scaled(3) at 1.5x = %d, want 5 (round of 4.5)", got)
	}
}

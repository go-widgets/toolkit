// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package vt

import (
	"strings"
	"testing"
)

// BenchmarkWrite measures the parser on coloured log-like output that
// scrolls (the common case: a container's logs, a long listing).
func BenchmarkWrite(b *testing.B) {
	line := "\x1b[32m2026-10-26T17:00:00Z\x1b[0m \x1b[1mINFO\x1b[0m request served in 12ms — état ok\r\n"
	in := []byte(strings.Repeat(line, 1000))
	s := New(120, 40, 5000)
	b.SetBytes(int64(len(in)))
	b.ResetTimer()
	for b.Loop() {
		s.Write(in)
	}
}

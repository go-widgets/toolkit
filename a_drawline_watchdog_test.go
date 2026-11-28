// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// drawLine is reached from a hundred-odd tests (every chart, the graph view, the
// iso widgets, the scale sweeps), each drawing straight onto a painter. A line
// has at most max(|dx|,|dy|)+1 steps, so a drawLine that is still on the same
// goroutine's stack many seconds later has lost its direction: a mutant that
// flips a step sign walks away from the end point for ever, and the test that
// reached it would hang until `go test -timeout`, which mutsweep reads as HUNG
// rather than caught. TestMain watches every goroutine and, instead of waiting,
// ends the run with a failure that names the test.
//
// boundedDrawLine (drawline_spans_test.go) stays the sharper bound for the span
// tests, which fails after 3 s from inside the test; this watchdog is the net
// under every other test and fires later, at drawLineStuck.
const (
	drawLinePoll  = 250 * time.Millisecond
	drawLineStuck = 8 * time.Second
)

// stuckInDrawLine reports the stack of a goroutine that has had drawLine on its
// stack in `need` consecutive samples, or "" while none has. seen carries the
// per-goroutine count between samples and is rebuilt on each call.
func stuckInDrawLine(dump string, seen map[string]int, need int) string {
	next := map[string]int{}
	var stuck string
	for _, g := range strings.Split(dump, "\n\n") {
		if !strings.Contains(g, "toolkit.drawLine(") {
			continue
		}
		id, _, _ := strings.Cut(g, "[")
		next[id] = seen[id] + 1
		if next[id] >= need && stuck == "" {
			stuck = g
		}
	}
	clear(seen)
	for k, v := range next {
		seen[k] = v
	}
	return stuck
}

// testOnStack names the Test function on a goroutine's stack ("" if none).
func testOnStack(g string) string {
	for _, line := range strings.Split(g, "\n") {
		if i := strings.Index(line, "toolkit.Test"); i >= 0 {
			name := line[i+len("toolkit."):]
			name, _, _ = strings.Cut(name, "(")
			name, _, _ = strings.Cut(name, ".func")
			return name
		}
	}
	return ""
}

func allStacks() string {
	for n := 1 << 20; ; n *= 2 {
		buf := make([]byte, n)
		if got := runtime.Stack(buf, true); got < n {
			return string(buf[:got])
		}
	}
}

func TestMain(m *testing.M) {
	done := make(chan struct{})
	go func() {
		seen := map[string]int{}
		tick := time.NewTicker(drawLinePoll)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-tick.C:
			}
			if g := stuckInDrawLine(allStacks(), seen, int(drawLineStuck/drawLinePoll)); g != "" {
				name := testOnStack(g)
				if name == "" {
					name = "TestMain"
				}
				fmt.Fprintf(os.Stdout, "--- FAIL: %s (%.2fs)\n    drawLine did not return for %v: its walk never reaches the end point\n%s\nFAIL\n",
					name, drawLineStuck.Seconds(), drawLineStuck, g)
				os.Exit(1)
			}
		}
	}()
	code := m.Run()
	close(done)
	os.Exit(code)
}

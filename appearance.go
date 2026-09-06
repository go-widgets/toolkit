// Copyright (c) 2026, the go-widgets authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file.

package toolkit

import "sync"

// appearanceMu guards the toolkit's global appearance: the metric scale, the
// touch density, the active font and the OpenType size a host last asked for.
//
// A host SETS these; every widget READS them, on every metric it lays out. One
// goroutine calling SetMetricScale while another builds a widget is therefore a
// plain data race — found by -race from an application whose tests set the
// scale in one test while another was constructing a list. In the wild it is
// not a test artefact: moving a window between displays is exactly this, the
// scale set from whatever thread the display-change event arrives on while the
// UI is drawing at the old one. What it costs is not a crash but silence —
// chrome laid out at one scale around type measured at another.
//
// It is deliberately ONE lock for the four of them. They are one fact, "how big
// is everything"; SetMetricScale re-renders the font, so the scale and the font
// change together; and two locks taken in two orders is how a toolkit deadlocks
// on a display change.
//
// THE INVARIANT, for anyone editing these files: no call holds this lock across
// another call that takes it. SetMetricScale drops it before rescaleText, which
// installs a face through SetFont, which takes it again. sync.RWMutex is not
// reentrant, and RLock is not safely reentrant either — a writer arriving
// between two nested RLocks deadlocks both. Read what you need, unlock, then
// compute.
var appearanceMu sync.RWMutex

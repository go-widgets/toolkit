// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

//go:build js

package toolkit

import "context"

// On js/wasm the tree never calls Status on the UI goroutine: the host sets PostUI
// and the tree asks from a background goroutine.
func (h *vcsHarness) install() { h.f.PostUI = h.post }

// settle waits for the refreshes the tree started on its own (SetRepository, a
// change of Entries) and applies their answers, as the host's UI loop would.
func (h *vcsHarness) settle() { h.drain() }

// refresh is RefreshStatusAsync, applied and read back: the production path. The
// context is the tree's own (a refresh in flight is cancelled by the next one),
// so the caller's is not passed on.
func (h *vcsHarness) refresh(context.Context) error {
	h.f.RefreshStatusAsync()
	h.drain()
	return h.f.StatusError().Get()
}

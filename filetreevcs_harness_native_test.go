// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

//go:build !js

package toolkit

import "context"

// Where Status may run on the UI goroutine the tree refreshes inline: nothing to
// install and nothing to wait for.
func (h *vcsHarness) install() {}

// settle waits for the refreshes the tree started on its own.
func (h *vcsHarness) settle() {}

// refresh is RefreshStatus with the caller's context.
func (h *vcsHarness) refresh(ctx context.Context) error { return h.f.RefreshStatus(ctx) }

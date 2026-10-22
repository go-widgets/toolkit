// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "github.com/go-widgets/mvvm"

// NewCommandAction builds an [Action] that is a view-model [mvvm.Command]: the
// Action runs the command, and its Enabled state follows the command's CanExecute
// — read now, and again each time the command raises CanExecuteChanged — so a
// ribbon button, a menu entry and a palette row all grey out together as the
// state the command's predicate reads changes. A view model owns the Command; the
// toolkit only reflects it.
//
// Visibility is not a command's business, but it is observable too: share the
// view model's own Observable by assigning it to the returned Action's Visible
// field, and every surface hides the control with it.
func NewCommandAction(id, label string, cmd *mvvm.Command) *Action {
	a := NewAction(id, label, cmd.Execute)
	a.SetEnabled(cmd.CanExecute())
	cmd.SubscribeCanExecuteChanged(func() { a.SetEnabled(cmd.CanExecute()) })
	return a
}

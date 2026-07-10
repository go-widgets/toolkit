// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package toolkit

import "strings"

// Drag-and-drop host contract.
//
// The toolkit does not run its own pointer loop, so it cannot originate drags
// on its own — a host (the wasmbox compositor's native HTML5 drag listeners, a
// TTY mouse decoder, ...) tracks the pointer and translates the platform's drag
// gestures into the toolkit's EventDragStart / EventDragMove / EventDragLeave /
// EventDrop events, delivering them to the widget under the pointer. These two
// interfaces let a host discover which widgets participate:
//
//   - a DragSource is asked for its payload when a drag begins on it;
//   - a DropTarget is asked whether it accepts a payload (to drive hover cues)
//     and receives the EventDrop that carries it.
//
// A widget can implement either or both. Payloads are plain strings so the
// contract stays back-end-neutral; multiple items travel newline-separated and
// a target recovers them with SplitDropPayload.

// DragSource is a widget a drag can originate from. DragData returns the payload
// string the host should carry for the drag (e.g. a file path or a row id).
type DragSource interface {
	Widget
	DragData() string
}

// DropTarget is a widget that can receive a drop. AcceptsDrop reports whether
// the given payload is droppable here — a host consults it on EventDragStart to
// decide whether to show an "accepted" cursor and whether to deliver the later
// EventDrop.
type DropTarget interface {
	Widget
	AcceptsDrop(payload string) bool
}

// DropPayloadSep separates individual items within a multi-item drag payload.
const DropPayloadSep = "\n"

// SplitDropPayload splits a drop payload (as carried in Event.Code) into its
// individual items, dropping empty entries so a trailing separator or an empty
// payload yields no phantom items.
func SplitDropPayload(code string) []string {
	if code == "" {
		return nil
	}
	var items []string
	for _, s := range strings.Split(code, DropPayloadSep) {
		if s != "" {
			items = append(items, s)
		}
	}
	return items
}

// JoinDropPayload builds a multi-item payload string from items, the inverse of
// SplitDropPayload — a host uses it to package several dragged paths into one
// Event.Code.
func JoinDropPayload(items []string) string {
	return strings.Join(items, DropPayloadSep)
}

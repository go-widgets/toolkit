// Package a11ydot is parsed, never compiled: a dot import the scan must refuse.
package a11ydot

import . "github.com/go-widgets/toolkit"

// Dotted embeds Base through a dot import.
type Dotted struct{ Base }

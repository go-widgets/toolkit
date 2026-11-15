package a11yscan

import tk "strings"

// Leak uses the alias tk for an unrelated package here, while aliased.go binds
// the same name to the toolkit: an alias belongs to one file and must not leak
// across, so Leak is no widget.
type Leak struct{ tk.Base }

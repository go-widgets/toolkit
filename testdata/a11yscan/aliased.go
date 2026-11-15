// Package a11yscan is parsed, never compiled, by the a11ydisabled audit's own
// tests: it holds one widget per shape the scan must classify.
package a11yscan

import (
	str "strings"

	tk "github.com/go-widgets/toolkit"
	v "github.com/go-widgets/toolkit/virtual"
)

// Aliased embeds Base through an aliased import and answers A11y().
type Aliased struct{ tk.Base }

func (*Aliased) A11y() tk.A11yInfo { return tk.A11yInfo{} }

// Forgetful embeds Base through an aliased import and has no A11y() at all.
type Forgetful struct{ tk.Base }

// Wrapped reaches Base through an aliased sub-package import.
type Wrapped struct{ *v.VirtualList[int] }

// NotAWidget embeds a type of an unrelated package under an alias.
type NotAWidget struct{ str.Builder }

// hidden is unexported and so outside the rule.
type hidden struct{ tk.Base }

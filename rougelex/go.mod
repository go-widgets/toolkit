module github.com/go-widgets/toolkit/rougelex

go 1.26.4

require (
	github.com/go-rouge/rouge v0.1.0
	github.com/go-widgets/toolkit v0.201.0
)

require (
	github.com/go-gfx/gfx v0.6.0 // indirect
	github.com/go-iconoir/iconoir v0.2.0 // indirect
	github.com/go-images/images v0.0.0-20260811115337-bc5d586f8e38 // indirect
	github.com/go-opentype/fonts v0.6.0 // indirect
	github.com/go-opentype/opentype v0.5.0 // indirect
	github.com/go-opentype/shape v0.5.0 // indirect
	github.com/go-regexp/engine v0.1.0 // indirect
	github.com/go-ruby-regexp/regexp v0.0.0-20260807185050-0533785e97b7 // indirect
	github.com/go-typeset/bidi v0.3.0 // indirect
	github.com/go-widgets/mvvm v0.5.0 // indirect
	github.com/go-widgets/painter v0.11.0 // indirect
	golang.org/x/image v0.45.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// rougelex is developed in-tree against the parent toolkit working copy. An
// external consumer must instead require a published toolkit release that
// contains CodeEditor (>= the tag this module is released alongside); this
// relative replace only applies to builds inside this repository and is
// ignored by any module that depends on rougelex.
replace github.com/go-widgets/toolkit => ../

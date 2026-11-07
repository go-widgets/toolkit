module github.com/go-widgets/toolkit/rougelex

go 1.27.2

require (
	github.com/go-rouge/rouge v0.9.0
	github.com/go-widgets/toolkit v0.356.0
)

require (
	github.com/dsnet/compress v0.0.2-0.20230904184137-39efe44ab707 // indirect
	github.com/go-crdt/crdt v0.57.0 // indirect
	github.com/go-gfx/gfx v0.41.0 // indirect
	github.com/go-icons/iconoir v0.3.0 // indirect
	github.com/go-images/images v0.3.0 // indirect
	github.com/go-opentype/fonts v0.14.0 // indirect
	github.com/go-opentype/opentype v0.18.0 // indirect
	github.com/go-opentype/shape v0.7.0 // indirect
	github.com/go-regexp/engine v0.4.2 // indirect
	github.com/go-richdoc/richdoc v0.7.0 // indirect
	github.com/go-ruby-regexp/regexp v0.2.1 // indirect
	github.com/go-typeset/bidi v0.3.1 // indirect
	github.com/go-widgets/mvvm v0.15.0 // indirect
	github.com/go-widgets/painter v0.17.0 // indirect
	github.com/go-widgets/textinput v0.1.1 // indirect
)

// rougelex is developed in-tree against the parent toolkit working copy. An
// external consumer must instead require a published toolkit release that
// contains CodeEditor (>= the tag this module is released alongside); this
// relative replace only applies to builds inside this repository and is
// ignored by any module that depends on rougelex.
replace github.com/go-widgets/toolkit => ../

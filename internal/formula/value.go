// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Package formula is the spreadsheet formula engine behind the toolkit's
// Spreadsheet widget: it lexes, parses and evaluates "=" expressions over an
// A1-addressed grid of cells, and maintains a dependency graph so an edit
// recomputes exactly the cells that (transitively) depend on it, with cycle
// detection that yields a #CIRC! error value instead of looping forever.
//
// The public surface is deliberately small: a Model owns the grid, SetCell
// feeds it raw user input (a literal or a leading-"=" formula), and Display /
// Get read back the computed result. Everything else (the lexer, the parser,
// the AST and the evaluator) is unexported so the engine can evolve without
// widening the widget's contract.
package formula

import (
	"math"
	"strconv"
	"strings"
)

// Kind is the dynamic type of a Value: a number, a text string, an error, or
// the blank a never-written (or cleared) cell holds.
type Kind int

const (
	// KindBlank is an empty cell: it reads as 0 in arithmetic and "" as text.
	KindBlank Kind = iota
	// KindNumber is a floating-point number.
	KindNumber
	// KindText is a string literal (or a non-numeric cell value).
	KindText
	// KindError is one of the ErrKind sentinels (#DIV/0!, #REF!, ...).
	KindError
)

// ErrKind enumerates the spreadsheet error values. Their String forms are the
// exact "#...!" tokens a cell renders and the ones tests assert against.
type ErrKind int

const (
	// ErrNone is the absence of an error; never stored in a KindError Value.
	ErrNone ErrKind = iota
	// ErrDiv0 is #DIV/0!: a division by zero, or an average of no numbers.
	ErrDiv0
	// ErrRef is #REF!: a reference (or a range corner) outside the sheet.
	ErrRef
	// ErrName is #NAME?: an unknown function, an unknown bare identifier, or a
	// formula the engine cannot parse.
	ErrName
	// ErrCirc is #CIRC!: a cell that (transitively) references itself.
	ErrCirc
	// ErrValue is #VALUE!: a type mismatch (text where a number is needed), a
	// bare range in scalar position, or a function called with the wrong arity.
	ErrValue
	// ErrNum is #NUM!: a numeric result that is NaN or infinite.
	ErrNum
)

// String is the "#...!" token an ErrKind renders as.
func (e ErrKind) String() string {
	switch e {
	case ErrDiv0:
		return "#DIV/0!"
	case ErrRef:
		return "#REF!"
	case ErrName:
		return "#NAME?"
	case ErrCirc:
		return "#CIRC!"
	case ErrValue:
		return "#VALUE!"
	case ErrNum:
		return "#NUM!"
	default:
		return "#ERR!"
	}
}

// Value is one evaluated result: a number, a text string, a blank, or an error.
// Only the field named by Kind is meaningful; the others hold their zero value.
type Value struct {
	Kind Kind
	Num  float64
	Text string
	Err  ErrKind
}

// Number builds a numeric Value.
func Number(n float64) Value { return Value{Kind: KindNumber, Num: n} }

// TextValue builds a text Value.
func TextValue(s string) Value { return Value{Kind: KindText, Text: s} }

// Blank builds the empty-cell Value.
func Blank() Value { return Value{Kind: KindBlank} }

// Error builds an error Value carrying e.
func Error(e ErrKind) Value { return Value{Kind: KindError, Err: e} }

// IsError reports whether v is an error value — the check the widget uses to
// tint an errored cell.
func (v Value) IsError() bool { return v.Kind == KindError }

// Display is the string a cell renders for v: the trimmed number, the text
// verbatim, the "#...!" token for an error, or "" for a blank.
func (v Value) Display() string {
	switch v.Kind {
	case KindNumber:
		return formatNumber(v.Num)
	case KindText:
		return v.Text
	case KindError:
		return v.Err.String()
	default:
		return ""
	}
}

// formatNumber renders f the way a cell shows it: the shortest round-tripping
// decimal, with a lone "-0" normalised to "0".
func formatNumber(f float64) string {
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// numResult wraps a computed float, mapping a NaN or an infinity to #NUM! so a
// blown-up calculation surfaces as an error rather than a bogus number.
func numResult(f float64) Value {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return Error(ErrNum)
	}
	return Number(f)
}

// asNumber coerces v to a float64 for arithmetic: a number is itself, a blank
// is 0, and numeric text ("5", " 3.5 ") parses; any other text — or an error —
// fails (ok == false), which the caller turns into #VALUE!.
func asNumber(v Value) (float64, bool) {
	switch v.Kind {
	case KindNumber:
		return v.Num, true
	case KindBlank:
		return 0, true
	case KindText:
		f, err := strconv.ParseFloat(strings.TrimSpace(v.Text), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

// asText coerces v to a string for text comparison: text verbatim, a number in
// its display form, a blank as "", and an error as its token.
func asText(v Value) string {
	switch v.Kind {
	case KindText:
		return v.Text
	case KindNumber:
		return formatNumber(v.Num)
	case KindBlank:
		return ""
	}
	return v.Err.String()
}

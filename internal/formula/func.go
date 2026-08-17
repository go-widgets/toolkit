// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "math"

// builtin is a spreadsheet function: it receives its argument AST nodes (so it
// can expand ranges itself) and the sheet to read them from.
type builtin func(args []*node, env Env) Value

// builtins is the function table, looked up by upper-cased name. AVERAGE is an
// alias for AVG. It is populated in init rather than as a var initializer
// because the functions transitively read builtins (via eval -> evalCall),
// which a static initializer would reject as a cycle.
var builtins map[string]builtin

func init() {
	builtins = map[string]builtin{
		"SUM":     fnSum,
		"AVG":     fnAvg,
		"AVERAGE": fnAvg,
		"MIN":     fnMin,
		"MAX":     fnMax,
		"COUNT":   fnCount,
		"IF":      fnIf,
		"ABS":     fnAbs,
		"ROUND":   fnRound,
	}
}

// evalCall dispatches a call to its builtin, or #NAME? for an unknown function.
func evalCall(n *node, env Env) Value {
	fn, ok := builtins[n.name]
	if !ok {
		return Error(ErrName)
	}
	return fn(n.args, env)
}

// collect flattens the argument list into scalar values, expanding every range
// cell-by-cell. If any operand (or range cell) is an error it stops and returns
// that error with ok=false, so aggregate functions propagate errors.
func collect(args []*node, env Env) ([]Value, Value, bool) {
	var out []Value
	for _, a := range args {
		var vals []Value
		if a.kind == nRange {
			vs, e, ok := expandRange(a, env)
			if !ok {
				return nil, e, false
			}
			vals = vs
		} else {
			vals = []Value{eval(a, env)}
		}
		for _, v := range vals {
			if v.Kind == KindError {
				return nil, v, false
			}
			out = append(out, v)
		}
	}
	return out, Value{}, true
}

// expandRange lists the values of every cell in a range node, normalising the
// corners so A3:A1 works. A corner outside the sheet is #REF! (ok=false).
func expandRange(a *node, env Env) ([]Value, Value, bool) {
	c1, c2 := a.ref.Col, a.ref2.Col
	r1, r2 := a.ref.Row, a.ref2.Row
	if c1 > c2 {
		c1, c2 = c2, c1
	}
	if r1 > r2 {
		r1, r2 = r2, r1
	}
	if !env.InBounds(Ref{Col: c1, Row: r1}) || !env.InBounds(Ref{Col: c2, Row: r2}) {
		return nil, Error(ErrRef), false
	}
	var out []Value
	for rr := r1; rr <= r2; rr++ {
		for cc := c1; cc <= c2; cc++ {
			out = append(out, env.Get(Ref{Col: cc, Row: rr}))
		}
	}
	return out, Value{}, true
}

// numbersOf keeps only the numeric values, so aggregates skip text and blanks
// the way SUM(A1:A9) ignores a stray label in the range.
func numbersOf(vals []Value) []float64 {
	var ns []float64
	for _, v := range vals {
		if v.Kind == KindNumber {
			ns = append(ns, v.Num)
		}
	}
	return ns
}

// fnSum totals the numeric values across its arguments and ranges.
func fnSum(args []*node, env Env) Value {
	vals, err, ok := collect(args, env)
	if !ok {
		return err
	}
	s := 0.0
	for _, f := range numbersOf(vals) {
		s += f
	}
	return numResult(s)
}

// fnAvg is the mean of the numeric values, or #DIV/0! when there are none.
func fnAvg(args []*node, env Env) Value {
	vals, err, ok := collect(args, env)
	if !ok {
		return err
	}
	ns := numbersOf(vals)
	if len(ns) == 0 {
		return Error(ErrDiv0)
	}
	s := 0.0
	for _, f := range ns {
		s += f
	}
	return numResult(s / float64(len(ns)))
}

// fnMin is the smallest numeric value, or 0 when there are none (Excel's MIN of
// an all-text range is 0).
func fnMin(args []*node, env Env) Value {
	vals, err, ok := collect(args, env)
	if !ok {
		return err
	}
	ns := numbersOf(vals)
	if len(ns) == 0 {
		return Number(0)
	}
	m := ns[0]
	for _, f := range ns[1:] {
		if f < m {
			m = f
		}
	}
	return Number(m)
}

// fnMax is the largest numeric value, or 0 when there are none.
func fnMax(args []*node, env Env) Value {
	vals, err, ok := collect(args, env)
	if !ok {
		return err
	}
	ns := numbersOf(vals)
	if len(ns) == 0 {
		return Number(0)
	}
	m := ns[0]
	for _, f := range ns[1:] {
		if f > m {
			m = f
		}
	}
	return Number(m)
}

// fnCount is the count of numeric values across its arguments and ranges.
func fnCount(args []*node, env Env) Value {
	vals, err, ok := collect(args, env)
	if !ok {
		return err
	}
	return Number(float64(len(numbersOf(vals))))
}

// fnIf returns its second argument when the first is truthy (a non-zero
// number), else its third (or 0 when omitted). It takes 2 or 3 arguments; a
// non-numeric condition is #VALUE!.
func fnIf(args []*node, env Env) Value {
	if len(args) < 2 || len(args) > 3 {
		return Error(ErrValue)
	}
	cond := eval(args[0], env)
	if cond.Kind == KindError {
		return cond
	}
	num, ok := asNumber(cond)
	if !ok {
		return Error(ErrValue)
	}
	if num != 0 {
		return eval(args[1], env)
	}
	if len(args) == 3 {
		return eval(args[2], env)
	}
	return Number(0)
}

// fnAbs is the absolute value of its single numeric argument.
func fnAbs(args []*node, env Env) Value {
	if len(args) != 1 {
		return Error(ErrValue)
	}
	v := eval(args[0], env)
	if v.Kind == KindError {
		return v
	}
	num, ok := asNumber(v)
	if !ok {
		return Error(ErrValue)
	}
	return Number(math.Abs(num))
}

// fnRound rounds its first numeric argument to the number of decimal places
// given by its second (ROUND(3.14159, 2) == 3.14; ROUND(2.5, 0) == 3).
func fnRound(args []*node, env Env) Value {
	if len(args) != 2 {
		return Error(ErrValue)
	}
	xv := eval(args[0], env)
	if xv.Kind == KindError {
		return xv
	}
	nv := eval(args[1], env)
	if nv.Kind == KindError {
		return nv
	}
	x, ok1 := asNumber(xv)
	n, ok2 := asNumber(nv)
	if !ok1 || !ok2 {
		return Error(ErrValue)
	}
	p := math.Pow(10, n)
	return numResult(math.Round(x*p) / p)
}

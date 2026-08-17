// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

// nodeKind tags an AST node.
type nodeKind int

const (
	nNum   nodeKind = iota // a numeric literal (num)
	nStr                   // a string literal (str)
	nRef                   // a cell reference (ref)
	nName                  // a bareword that is not a valid ref (name) -> #NAME? at eval
	nRange                 // a range ref:ref2
	nUnary                 // a prefix +/- (op, x)
	nBin                   // a binary op (op, l, r)
	nCall                  // a function call (name, args)
)

// node is one AST node. Only the fields relevant to kind are set; the rest hold
// their zero value. The tree is small and immutable once parsed.
type node struct {
	kind nodeKind
	num  float64
	str  string
	name string
	ref  Ref
	ref2 Ref
	op   tokKind
	x    *node
	l, r *node
	args []*node
}

// binPrec is the binding power of a binary operator: comparisons loosest, then
// +/-, then */. It returns 0 for any token that is not an infix binary operator
// (^ is handled separately, right-associatively, in parsePower), which is how
// the precedence-climbing loop knows to stop.
func binPrec(k tokKind) int {
	switch k {
	case tEq, tLt, tGt, tLe, tGe, tNe:
		return 1
	case tPlus, tMinus:
		return 2
	case tStar, tSlash:
		return 3
	}
	return 0
}

// parser walks a token slice with a single-token lookahead (cur).
type parser struct {
	toks []token
	pos  int
}

func (p *parser) cur() token { return p.toks[p.pos] }
func (p *parser) next()      { p.pos++ }

// Parse lexes and parses a formula body into an AST, or returns errSyntax. The
// whole input must be consumed: trailing tokens ("1 2") are a syntax error.
func Parse(s string) (*node, error) {
	toks, err := lex(s)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if p.cur().kind != tEOF {
		return nil, errSyntax
	}
	return n, nil
}

// parseExpr parses a full expression (the lowest precedence level).
func (p *parser) parseExpr() (*node, error) { return p.parseBinary(1) }

// parseBinary is precedence climbing: it parses a unary operand, then folds in
// any following binary operator of binding power >= minPrec (left-associative,
// so the recursive call raises the floor to prec+1).
func (p *parser) parseBinary(minPrec int) (*node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.cur().kind
		prec := binPrec(op)
		if prec == 0 || prec < minPrec {
			break
		}
		p.next()
		right, err := p.parseBinary(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &node{kind: nBin, op: op, l: left, r: right}
	}
	return left, nil
}

// parseUnary parses a chain of prefix +/- above the ^ level, so -2^2 is -(2^2)
// and 2^-1 is 2^(-1).
func (p *parser) parseUnary() (*node, error) {
	if k := p.cur().kind; k == tPlus || k == tMinus {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &node{kind: nUnary, op: k, x: x}, nil
	}
	return p.parsePower()
}

// parsePower parses a primary optionally raised to a power. ^ is
// right-associative (2^3^2 == 2^(3^2)) and its exponent may itself be unary
// (2^-1), so the right operand is a parseUnary.
func (p *parser) parsePower() (*node, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if p.cur().kind == tCaret {
		p.next()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &node{kind: nBin, op: tCaret, l: base, r: exp}, nil
	}
	return base, nil
}

// parsePrimary parses an atom: a number, a string, a parenthesised expression,
// or an identifier (which resolves to a call, a range, a cell ref or a bare
// name in parseIdent).
func (p *parser) parsePrimary() (*node, error) {
	t := p.cur()
	switch t.kind {
	case tNumber:
		p.next()
		return &node{kind: nNum, num: t.num}, nil
	case tString:
		p.next()
		return &node{kind: nStr, str: t.text}, nil
	case tLParen:
		p.next()
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.cur().kind != tRParen {
			return nil, errSyntax
		}
		p.next()
		return e, nil
	case tIdent:
		return p.parseIdent()
	}
	return nil, errSyntax
}

// parseIdent resolves a bareword: NAME( -> a function call, NAME:NAME -> a
// range (both endpoints must be valid refs), a lone valid ref -> a cell ref,
// and anything else -> a bare name that evaluates to #NAME?.
func (p *parser) parseIdent() (*node, error) {
	name := p.cur().text
	p.next()
	switch p.cur().kind {
	case tLParen:
		return p.parseCall(name)
	case tColon:
		p.next()
		if p.cur().kind != tIdent {
			return nil, errSyntax
		}
		name2 := p.cur().text
		p.next()
		r1, ok1 := ParseRef(name)
		r2, ok2 := ParseRef(name2)
		if !ok1 || !ok2 {
			return nil, errSyntax
		}
		return &node{kind: nRange, ref: r1, ref2: r2}, nil
	}
	if r, ok := ParseRef(name); ok {
		return &node{kind: nRef, ref: r}, nil
	}
	return &node{kind: nName, name: name}, nil
}

// parseCall parses the argument list of a call whose name has been read and
// whose opening "(" is the current token: a possibly-empty, comma-separated
// list of expressions closed by ")".
func (p *parser) parseCall(name string) (*node, error) {
	p.next() // consume '('
	var args []*node
	if p.cur().kind != tRParen {
		for {
			a, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			if p.cur().kind == tComma {
				p.next()
				continue
			}
			break
		}
	}
	if p.cur().kind != tRParen {
		return nil, errSyntax
	}
	p.next()
	return &node{kind: nCall, name: name, args: args}, nil
}

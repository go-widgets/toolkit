// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import (
	"errors"
	"strconv"
	"strings"
)

// errSyntax is the single error every lex/parse failure returns. The Model
// turns it into a #NAME? cell value, so the caller never inspects it beyond
// "did parsing succeed"; it exists so lex and parse can bail cleanly.
var errSyntax = errors.New("formula: syntax error")

// tokKind enumerates the lexical tokens of a formula.
type tokKind int

const (
	tEOF    tokKind = iota // end of input
	tNumber                // a numeric literal (value in token.num)
	tString                // a "double-quoted" string (value in token.text)
	tIdent                 // a bareword: a cell ref, a range endpoint, or a function name
	tPlus                  // +
	tMinus                 // -
	tStar                  // *
	tSlash                 // /
	tCaret                 // ^
	tLParen                // (
	tRParen                // )
	tColon                 // :
	tComma                 // ,
	tEq                    // =
	tLt                    // <
	tGt                    // >
	tLe                    // <=
	tGe                    // >=
	tNe                    // <>
)

// token is one lexed unit. num is meaningful only for tNumber; text only for
// tString and tIdent (upper-cased).
type token struct {
	kind tokKind
	text string
	num  float64
}

// lex turns a formula body (the text after the leading "=") into a token
// stream terminated by a tEOF token. It returns errSyntax on an unterminated
// string, a malformed number, or a character that starts no token.
func lex(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c >= '0' && c <= '9' || c == '.':
			j := i
			for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
				j++
			}
			f, err := strconv.ParseFloat(s[i:j], 64)
			if err != nil {
				return nil, errSyntax
			}
			toks = append(toks, token{kind: tNumber, num: f})
			i = j
		case c == '"':
			j := i + 1
			var b []byte
			for j < len(s) {
				if s[j] == '"' {
					if j+1 < len(s) && s[j+1] == '"' { // "" is an escaped quote
						b = append(b, '"')
						j += 2
						continue
					}
					break
				}
				b = append(b, s[j])
				j++
			}
			if j >= len(s) {
				return nil, errSyntax // ran off the end with no closing quote
			}
			toks = append(toks, token{kind: tString, text: string(b)})
			i = j + 1
		case c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_':
			j := i
			for j < len(s) && (s[j] >= 'A' && s[j] <= 'Z' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= '0' && s[j] <= '9' || s[j] == '_') {
				j++
			}
			toks = append(toks, token{kind: tIdent, text: strings.ToUpper(s[i:j])})
			i = j
		default:
			tok, adv, ok := lexOperator(s, i)
			if !ok {
				return nil, errSyntax
			}
			toks = append(toks, tok)
			i += adv
		}
	}
	return append(toks, token{kind: tEOF}), nil
}

// lexOperator lexes the single- or double-character operator starting at s[i].
// It returns the token, how many bytes it consumed, and ok=false when s[i]
// starts no operator at all.
func lexOperator(s string, i int) (token, int, bool) {
	switch s[i] {
	case '+':
		return token{kind: tPlus}, 1, true
	case '-':
		return token{kind: tMinus}, 1, true
	case '*':
		return token{kind: tStar}, 1, true
	case '/':
		return token{kind: tSlash}, 1, true
	case '^':
		return token{kind: tCaret}, 1, true
	case '(':
		return token{kind: tLParen}, 1, true
	case ')':
		return token{kind: tRParen}, 1, true
	case ':':
		return token{kind: tColon}, 1, true
	case ',':
		return token{kind: tComma}, 1, true
	case '=':
		return token{kind: tEq}, 1, true
	case '<':
		if i+1 < len(s) && s[i+1] == '=' {
			return token{kind: tLe}, 2, true
		}
		if i+1 < len(s) && s[i+1] == '>' {
			return token{kind: tNe}, 2, true
		}
		return token{kind: tLt}, 1, true
	case '>':
		if i+1 < len(s) && s[i+1] == '=' {
			return token{kind: tGe}, 2, true
		}
		return token{kind: tGt}, 1, true
	}
	return token{}, 0, false
}

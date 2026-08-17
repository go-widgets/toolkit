// Copyright (c) 2026 the go-widgets/toolkit authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package formula

import "strconv"

// Ref is a single cell address, held as 0-based column and row indices so it
// indexes the grid directly. A1 is Ref{Col: 0, Row: 0}; B3 is Ref{Col: 1,
// Row: 2}. The A1 spelling (letters then a 1-based row) is only a surface form,
// produced by A1 and consumed by ParseRef.
type Ref struct {
	Col, Row int
}

// A1 is the spreadsheet spelling of the reference: the column letters followed
// by the 1-based row number (Ref{0,0} -> "A1", Ref{27,9} -> "AB10").
func (r Ref) A1() string {
	return ColumnName(r.Col) + strconv.Itoa(r.Row+1)
}

// ColumnName is the bijective base-26 letter label for a 0-based column index:
// 0 -> "A", 25 -> "Z", 26 -> "AA", 27 -> "AB". It is the column-header label the
// Spreadsheet widget paints.
func ColumnName(col int) string {
	name := ""
	col++ // shift to 1-based for the bijective base-26 spelling
	for col > 0 {
		col--
		name = string(rune('A'+col%26)) + name
		col /= 26
	}
	return name
}

// ParseRef parses an A1 spelling ("A1", "AB10") into a Ref. ok is false for any
// string that is not letters-then-digits with a positive row (so "A0", "1",
// "A", "A1B" and "" all fail). The caller has already upper-cased the text.
func ParseRef(s string) (Ref, bool) {
	i := 0
	col := 0
	for i < len(s) && s[i] >= 'A' && s[i] <= 'Z' {
		col = col*26 + int(s[i]-'A'+1)
		i++
	}
	if i == 0 || i >= len(s) {
		return Ref{}, false // no letters, or no digits after the letters
	}
	row := 0
	for i < len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return Ref{}, false // a non-digit in the row part
		}
		row = row*10 + int(c-'0')
		i++
	}
	if row == 0 {
		return Ref{}, false // "A0" has no 0th row
	}
	return Ref{Col: col - 1, Row: row - 1}, true
}

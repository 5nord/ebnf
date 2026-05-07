// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ebnf is a library for EBNF grammars. The input is text ([]byte)
// satisfying the following grammar (represented itself in EBNF):
//
//	Production  = name "=" [ Expression ] "." .
//	Expression  = Alternative { "|" Alternative } .
//	Alternative = Term { Term } .
//	Term        = name | token [ "…" token ] | Group | Option | Repetition .
//	Group       = "(" Expression ")" .
//	Option      = "[" Expression "]" .
//	Repetition  = "{" Expression "}" .
//
// A name is a Go identifier, a token is a Go string, and comments
// and white space follow the same rules as for the Go language.
// Production names starting with an uppercase Unicode letter denote
// non-terminal productions (i.e., productions which allow white-space
// and comments between tokens); all other production names denote
// lexical productions.
package ebnf

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"
)

// ----------------------------------------------------------------------------
// Error handling

type errorList []error

func (list errorList) Err() error {
	if len(list) == 0 {
		return nil
	}
	return list
}

func (list errorList) Error() string {
	switch len(list) {
	case 0:
		return "no errors"
	case 1:
		return list[0].Error()
	}
	return fmt.Sprintf("%s (and %d more errors)", list[0], len(list)-1)
}

func newError(pos scanner.Position, msg string) error {
	return errors.New(fmt.Sprintf("%s: %s", pos, msg))
}

// ----------------------------------------------------------------------------
// Internal representation

type (
	// An Expression node represents a production expression.
	Expression interface {
		// Pos is the position of the first character of the syntactic construct
		Pos() scanner.Position
	}

	// An Alternative node represents a non-empty list of alternative expressions.
	Alternative []Expression // x | y | z

	// A Sequence node represents a non-empty list of sequential expressions.
	Sequence []Expression // x y z

	// A Name node represents a production name.
	Name struct {
		StringPos scanner.Position
		String    string
	}

	// A Token node represents a literal.
	Token struct {
		StringPos scanner.Position
		String    string
	}

	// A List node represents a range of characters.
	Range struct {
		Begin, End *Token // begin ... end
	}

	// A Group node represents a grouped expression.
	Group struct {
		Lparen scanner.Position
		Body   Expression // (body)
	}

	// An Option node represents an optional expression.
	Option struct {
		Lbrack scanner.Position
		Body   Expression // [body]
	}

	// A Repetition node represents a repeated expression.
	Repetition struct {
		Lbrace scanner.Position
		Body   Expression // {body}
	}

	// A Production node represents an EBNF production.
	Production struct {
		Name *Name
		Expr Expression
	}

	// A Bad node stands for pieces of source code that lead to a parse error.
	Bad struct {
		TokPos scanner.Position
		Error  string // parser error message
	}

	// A Grammar is a set of EBNF productions. The map
	// is indexed by production name.
	//
	Grammar map[string]*Production
)

func (x Alternative) Pos() scanner.Position { return x[0].Pos() } // the parser always generates non-empty Alternative
func (x Sequence) Pos() scanner.Position    { return x[0].Pos() } // the parser always generates non-empty Sequences
func (x *Name) Pos() scanner.Position       { return x.StringPos }
func (x *Token) Pos() scanner.Position      { return x.StringPos }
func (x *Range) Pos() scanner.Position      { return x.Begin.Pos() }
func (x *Group) Pos() scanner.Position      { return x.Lparen }
func (x *Option) Pos() scanner.Position     { return x.Lbrack }
func (x *Repetition) Pos() scanner.Position { return x.Lbrace }
func (x *Production) Pos() scanner.Position { return x.Name.Pos() }
func (x *Bad) Pos() scanner.Position        { return x.TokPos }

// ----------------------------------------------------------------------------
// Grammar verification

// IsLexical returns true, when given name is a lexical production.
func IsLexical(name string) bool {
	ch, _ := utf8.DecodeRuneInString(name)
	return !unicode.IsUpper(ch)
}

type verifier struct {
	errors   errorList
	worklist []*Production
	reached  Grammar // set of productions reached from (and including) the root production
	grammar  Grammar
}

func (v *verifier) error(pos scanner.Position, msg string) {
	v.errors = append(v.errors, newError(pos, msg))
}

func (v *verifier) push(prod *Production) {
	name := prod.Name.String
	if _, found := v.reached[name]; !found {
		v.worklist = append(v.worklist, prod)
		v.reached[name] = prod
	}
}

func (v *verifier) verifyChar(x *Token) rune {
	s := x.String
	if utf8.RuneCountInString(s) != 1 {
		v.error(x.Pos(), "single char expected, found "+s)
		return 0
	}
	ch, _ := utf8.DecodeRuneInString(s)
	return ch
}

func (v *verifier) verifyExpr(expr Expression, lexical bool) {
	switch x := expr.(type) {
	case nil:
		// empty expression
	case Alternative:
		for _, e := range x {
			v.verifyExpr(e, lexical)
		}
	case Sequence:
		for _, e := range x {
			v.verifyExpr(e, lexical)
		}
	case *Name:
		// a production with this name must exist;
		// add it to the worklist if not yet processed
		if prod, found := v.grammar[x.String]; found {
			v.push(prod)
		} else {
			v.error(x.Pos(), "missing production "+x.String)
		}
		// within a lexical production references
		// to non-lexical productions are invalid
		if lexical && !IsLexical(x.String) {
			v.error(x.Pos(), "reference to non-lexical production "+x.String)
		}
	case *Token:
		// nothing to do for now
	case *Range:
		i := v.verifyChar(x.Begin)
		j := v.verifyChar(x.End)
		if i >= j {
			v.error(x.Pos(), "decreasing character range")
		}
	case *Group:
		v.verifyExpr(x.Body, lexical)
	case *Option:
		v.verifyExpr(x.Body, lexical)
	case *Repetition:
		v.verifyExpr(x.Body, lexical)
	case *Bad:
		v.error(x.Pos(), x.Error)
	default:
		panic(fmt.Sprintf("internal error: unexpected type %T", expr))
	}
}

func (v *verifier) verify(grammar Grammar, start string) {
	// find root production
	root, found := grammar[start]
	if !found {
		var noPos scanner.Position
		v.error(noPos, "no start production "+start)
		return
	}

	// initialize verifier
	v.worklist = v.worklist[0:0]
	v.reached = make(Grammar)
	v.grammar = grammar

	// work through the worklist
	v.push(root)
	for {
		n := len(v.worklist) - 1
		if n < 0 {
			break
		}
		prod := v.worklist[n]
		v.worklist = v.worklist[0:n]
		v.verifyExpr(prod.Expr, IsLexical(prod.Name.String))
	}

	// check if all productions were reached
	if len(v.reached) < len(v.grammar) {
		for name, prod := range v.grammar {
			if _, found := v.reached[name]; !found {
				v.error(prod.Pos(), name+" is unreachable")
			}
		}
	}
}

// Verify checks that:
//   - all productions used are defined
//   - all productions defined are used when beginning at start
//   - lexical productions refer only to other lexical productions
//
// Position information is interpreted relative to the file set fset.
func Verify(grammar Grammar, start string) error {
	var v verifier
	v.verify(grammar, start)
	return v.errors.Err()
}

// First returns the first token set of a given expression
func First(grammar Grammar, x Expression) []string {
	ret := first(grammar, x, make(map[Expression]bool))
	m := make(map[string]bool)
	for _, tok := range ret {
		m[tok] = true
	}
	ret = make([]string, 0, len(m))
	for tok := range m {
		ret = append(ret, tok)
	}
	return ret
}

func first(grammar Grammar, x Expression, v map[Expression]bool) []string {
	switch x := x.(type) {
	case *Token:
		return []string{x.String}
	case Sequence:
		var ret []string
		if len(x) > 0 {
			ret = append(ret, first(grammar, x[0], v)...)
		}
		if _, ok := x[0].(*Option); ok && len(x) > 1 {
			ret = append(ret, first(grammar, x[1], v)...)
		}
		return ret
	case *Name:
		if !v[x] {
			v[x] = true
			return first(grammar, grammar[x.String], v)
		}
		return nil

	case *Option:
		return first(grammar, x.Body, v)
	case *Repetition:
		return first(grammar, x.Body, v)
	case Alternative:
		var ret []string
		for _, alt := range x {
			ret = append(ret, first(grammar, alt, v)...)
		}
		return ret
	case *Group:
		return first(grammar, x.Body, v)
	case *Production:
		if x.Expr == nil {
			return nil
		}
		return first(grammar, x.Expr, v)
	case *Range:
		return nil
	default:
		log.Printf("first: unhandled expression type: %T\n", x)
		return nil
	}
}

// Text returns the text of the production.
func Text(src []byte, p *Production) string {
	var buf bytes.Buffer
	s := bufio.NewScanner(bytes.NewReader(src[p.Pos().Offset:]))
	for s.Scan() {
		line := s.Text()
		end, ok := productionEnd(line)
		fmt.Fprintf(&buf, "%s", line[:end])
		if ok {
			break
		}
	}

	return buf.String()
}

func productionEnd(text string) (int, bool) {
	var s scanner.Scanner
	s.Init(strings.NewReader(text))
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		if s.TokenText() == "." {
			return s.Pos().Offset, true
		}
	}
	return len(text), false
}

// Productions returns the productions of the grammar in the order they appear
// in the source file.
func Productions(grammar Grammar) []*Production {
	ret := make([]*Production, 0, len(grammar))
	for _, prod := range grammar {
		if !IsLexical(prod.Name.String) {
			ret = append(ret, prod)
		}
	}
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].Pos().Offset < ret[j].Pos().Offset
	})
	return ret
}

// Inspect traverses the given expression and calls the given function for each.
func Inspect(e Expression, fn func(e Expression) bool) bool {
	if e == nil {
		return true
	}

	if !fn(e) {
		return false
	}

	switch e := e.(type) {
	case Alternative:
		for _, alt := range e {
			if !Inspect(alt, fn) {
				return false
			}
		}
	case Sequence:
		for _, seq := range e {
			if !Inspect(seq, fn) {
				return false
			}
		}

	case *Group:
		if e.Body != nil {
			Inspect(e.Body, fn)
		}

	case *Option:
		if e.Body != nil {
			Inspect(e.Body, fn)
		}

	case *Repetition:
		if e.Body != nil {
			Inspect(e.Body, fn)
		}

	}
	return true
}

type printer struct {
	w io.Writer
	q []string
}

func (p *printer) format(e Expression) {
	switch e := e.(type) {
	case Alternative:
		for i, e := range e {
			if i > 0 {
				fmt.Fprintf(p.w, " | ")
			}
			p.format(e)
		}
	case Sequence:
		for i, e := range e {
			if i > 0 {
				fmt.Fprintf(p.w, " ")
			}
			p.format(e)
		}
	case *Name:
		p.q = append(p.q, e.String)
		fmt.Fprintf(p.w, "%s", e.String)
	case *Token:
		fmt.Fprintf(p.w, "%q", e.String)
	case *Range:
		fmt.Fprintf(p.w, "%q…%q", e.Begin.String, e.End.String)
	case *Group:
		fmt.Fprintf(p.w, "(")
		p.format(e.Body)
		fmt.Fprintf(p.w, ")")
	case *Option:
		fmt.Fprintf(p.w, "[")
		p.format(e.Body)
		fmt.Fprintf(p.w, "]")
	case *Repetition:
		fmt.Fprintf(p.w, "{")
		p.format(e.Body)
		fmt.Fprintf(p.w, "}")
	}
}

func (p *printer) Format(prod *Production) {
	fmt.Fprintf(p.w, "%s = ", prod.Name.String)
	p.format(prod.Expr)
	fmt.Fprintf(p.w, ".\n")
}

func Format(g Grammar) string {
	b := strings.Builder{}
	rules := make([]*Production, 0, len(g))
	lexemes := make([]*Production, 0, len(g))

	for name, prod := range g {
		if IsLexical(name) {
			lexemes = append(lexemes, prod)
		} else {
			rules = append(rules, prod)
		}
	}

	sortProductions := func(s []*Production) {
		sort.Slice(s, func(i, j int) bool {
			a := s[i].Pos().Offset
			b := s[j].Pos().Offset
			return a < b
		})
	}
	sortProductions(rules)
	sortProductions(lexemes)

	p := printer{w: &b}
	for _, prod := range append(rules, lexemes...) {
		p.Format(prod)
	}

	return b.String()
}

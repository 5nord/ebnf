// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ebnf

import (
	"bytes"
	"slices"
	"sort"
	"strings"
	"testing"
)

var goodGrammars = []string{
	`Program ::= .`,

	`Program ::= foo .
	foo ::= "foo" .`,

	`Program ::= "a" | "b" "c" .`,

	`Program ::= "a" … "z" .`,

	`Program ::= Song .
	Song ::= { Note } .
	Note ::= Do | (Re | Mi | Fa | So | La) | Ti .
	Do ::= "c" .
	Re ::= "d" .
	Mi ::= "e" .
	Fa ::= "f" .
	So ::= "g" .
	La ::= "a" .
	Ti ::= ti .
	ti ::= "b" .`,

	"Program ::= `\"` .",
}

var badGrammars = []string{
	`Program ::= | .`,
	`Program ::= | b .`,
	`Program ::= a … b .`,
	`Program ::= "a" … .`,
	`Program ::= … "b" .`,
	`Program ::= () .`,
	`Program ::= [] .`,
	`Program ::= {} .`,
}

func checkGood(t *testing.T, src string) {
	grammar, err := Parse("", bytes.NewBuffer([]byte(src)))
	if err != nil {
		t.Errorf("Parse(%s) failed: %v", src, err)
		return
	}
	if err = Verify(grammar, "Program"); err != nil {
		t.Errorf("Verify(%s) failed: %v", src, err)
	}
}

func checkBad(t *testing.T, src string) {
	_, err := Parse("", bytes.NewBuffer([]byte(src)))
	if err == nil {
		t.Errorf("Parse(%s) should have failed", src)
	}
}

func TestGrammars(t *testing.T) {
	for _, src := range goodGrammars {
		checkGood(t, src)
	}
	for _, src := range badGrammars {
		checkBad(t, src)
	}
}

func TestFirst(t *testing.T) {
	g := parse(t, `
	E ::= [ T E ] .
	T ::= "a" | "b" .
	`)
	got := First(g, g["E"])
	sort.Strings(got)
	want := []string{"a", "b"}
	sort.Strings(want)
	if slices.Compare(got, want) != 0 {
		t.Errorf("First: want=%v, got=%v", want, got)
	}
}

func TestFirstSkipsNullablePrefix(t *testing.T) {
	tests := []struct {
		grammar string
		want    []string
	}{
		{
			grammar: `S ::= [ "a" ] [ "b" ] "c" .`,
			want:    []string{"a", "b", "c"},
		},
		{
			grammar: `S ::= P "c" .
			P ::= [ "a" ] .`,
			want: []string{"a", "c"},
		},
		{
			grammar: `S ::= { "a" } "b" .`,
			want:    []string{"a", "b"},
		},
		{
			grammar: `S ::= ( [ "a" ] ) "b" .`,
			want:    []string{"a", "b"},
		},
		{
			grammar: `S ::= P Q "c" .
			P ::= "a" | [ "b" ] .
			Q ::= { "d" } .`,
			want: []string{"a", "b", "c", "d"},
		},
		{
			// A cycle is not evidence of an empty production, so the
			// sequence stops at the recursive name.
			grammar: `S ::= S "a" .`,
			want:    nil,
		},
	}
	for _, test := range tests {
		g := parse(t, test.grammar)
		got := First(g, g["S"])
		sort.Strings(got)
		want := slices.Clone(test.want)
		sort.Strings(want)
		if slices.Compare(got, want) != 0 {
			t.Errorf("First(%q): want=%v, got=%v", test.grammar, want, got)
		}
	}
}

func TestText(t *testing.T) {
	src := `
	E ::= [ T E ] .
	T ::= "a" | "b" .
	`
	g := parse(t, src)
	got := strings.TrimSpace(Text([]byte(src), g["T"]))
	if want := `T ::= "a" | "b" .`; want != got {
		t.Errorf("Text: want=%q, got=%q", want, got)
	}
}

func parse(t *testing.T, input string) Grammar {
	g, err := Parse("test", bytes.NewBuffer([]byte(input)))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

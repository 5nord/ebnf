# EBNF parser and helpers for Go

This package is a fork of https://pkg.go.dev/golang.org/x/exp/ebnf

It parses a slightly different syntax (`=` -> `::=`) and it includes some
additional helpers for working with grammars.

Package ebnf is a library for EBNF grammars. The input is text ([]byte)
satisfying the following grammar (represented itself in EBNF):

```ebnf
 Production  ::= name "::=" [ Expression ] "." .
 Expression  ::= Alternative { "|" Alternative } .
 Alternative ::= Term { Term } .
 Term        ::= name | token [ "…" token ] | Group | Option | Repetition .
 Group       ::= "(" Expression ")" .
 Option      ::= "[" Expression "]" .
 Repetition  ::= "{" Expression "}" .
```

A name is a Go identifier, a token is a Go string, and comments
and white space follow the same rules as for the Go language.
Production names starting with an uppercase Unicode letter denote
non-terminal productions (i.e., productions which allow white-space
and comments between tokens); all other production names denote
lexical productions.

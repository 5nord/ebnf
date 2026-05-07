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

## Examples

### Parsing

```go
package main

import (
	"bytes"
	"fmt"

	"github.com/5nord/ebnf"
)

func main() {
	src := []byte(`
		E ::= [ T E ].
		T ::= "a"|"b" .
	`)

	g, err := ebnf.Parse("", bytes.NewBuffer(src))
	if err != nil {
		panic(err)
	}

	// ...
}
```

## New Functions

### First

`ebnf.First` returns the first-set of the given production.

```go
	fmt.Println(ebnf.First(g, g["E"])) // Output: [a b]
```

### Text

`ebnf.Text` returns the literal text of the given production.

```go
	fmt.Println(ebnf.Text(src, g["E"])) // Output: E := [ T E ].
```

### IsLexical

`ebnf.Islexical` returns `true` if the given string is a terminal name.

### Inspect

`ebnf.Inspect` traverses the given expression and calls the given function for each.

```go
	ebnf.Inspect(g["E"], func(e ebnf.Expression) bool {
		fmt.Printf("%T\n", e)
		return true
	})
```

### Format

`ebnf.Format` formats the grammar.

```go
fmt.Println(ebnf.Format(g))
```

# Package `regex/syntax`

Package `syntax` provides regular expressions parser as well as AST definitions.

## Rationale

The advantages of this package over stdlib [regexp/syntax](https://golang.org/pkg/regexp/syntax/):

1. Does not transformations/optimizations during the parsing.
   The produced parse tree is loseless.

2. Simpler AST representation.

3. Can parse most PCRE operations in addition to [re2](https://github.com/google/re2/wiki/Syntax) syntax.
   It can also handle PHP/Perl style patterns with delimiters.

4. This package is easier to extend than something from the standard library.

This package does almost no assumptions about how generated AST is going to be used
so it preserves as much syntax information as possible.

It's easy to write another intermediate representation on top of it. The main
function of this package is to convert a textual regexp pattern into a more
structured form that can be processed more easily.

## Users

* [go-critic](https://github.com/go-critic/go-critic) - Go static analyzer
* [NoVerify](https://github.com/VKCOM/noverify) - PHP static analyzer

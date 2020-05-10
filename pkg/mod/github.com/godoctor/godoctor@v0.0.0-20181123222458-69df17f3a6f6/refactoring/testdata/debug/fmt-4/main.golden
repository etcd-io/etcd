package main

import "fmt"

func main() {
	i := 0
	j := 1
	k := /* seriously */ j + 1 /* dude */
	l := 3
	// <<<<< debug,1,1,1,1,fmt,pass
	fmt.Printf("%d %d %d %d\n", i, j, k, l)
}

// This produced a result with syntax errors (no semicolon or newline before
// l := 3 after formatting), but that seems to be a bug in in the formatter,
// since gofmt produces the same result.  This test was for documentation.

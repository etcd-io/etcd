package main

// this is a file 'a.go'

import (
	superos "os"
)

func B() superos.Error {
	return nil
}

// notice how changing type of a return function in one file,
// the inferred type of a variable in another file changes also

func (t *Tester) SetC() {
	t.c = 31337
}

func (t *Tester) SetD() {
	t.d = 31337
}

// support for multifile packages, including correct namespace handling

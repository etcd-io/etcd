Fork of [unconvert](https://github.com/mdempsky/unconvert) to be usable as a library.

The specific elements are inside the file `golangci.go`.

The only modification of the file `unconvert.go` is the remove of the global variables for the flags.
The tests will never work because of that, then the CI is disabled.

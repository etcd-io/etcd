# nakedret

nakedret is a Go static analysis tool to find naked returns in functions greater than a specified function length.

## Installation
Install Nakedret via go install:

```cmd
go install github.com/alexkohler/nakedret/v2/cmd/nakedret@latest
```

If you have not already added your `GOPATH/bin` directory to your `PATH` environment variable then you will need to do so.

Windows (cmd):
```cmd
set PATH=%PATH%;C:\your\GOPATH\bin
```

Bash (you can verify a path has been set):
```Bash
# Check if nakedret is on PATH
which nakedret
export PATH=$PATH:/your/GOPATH/bin #to set path if it does not exist
```

## Usage

Similar to other Go static anaylsis tools (such as `golint`, `go vet`), nakedret can be invoked with one or more filenames, directories, or packages named by its import path. Nakedret also supports the `...` wildcard. 

    nakedret [flags] files/directories/packages

Currently, the only flag supported is -l, which is an optional numeric flag to specify the maximum length a function can be (in terms of line length). If not specified, it defaults to 5.

It can also be run using `go vet`:

```shell
go vet -vettool=$(which nakedret) ./...
```

## Purpose

As noted in Go's [Code Review comments](https://github.com/golang/go/wiki/CodeReviewComments#named-result-parameters):

> Naked returns are okay if the function is a handful of lines. Once it's a medium sized function, be explicit with your return 
> values. Corollary: it's not worth it to name result parameters just because it enables you to use naked returns. Clarity of docs is always more important than saving a line or two in your function.

This tool aims to catch naked returns on non-trivial functions.

## Example

Let's take the `types` package in the Go source as an example:

```Bash
$ nakedret -l 25 types/
types/check.go:245 checkFiles naked returns on 26 line function 
types/typexpr.go:443 collectParams naked returns on 53 line function 
types/stmt.go:275 caseTypes naked returns on 27 line function 
types/lookup.go:275 MissingMethod naked returns on 39 line function
```

Below is one of the not so intuitive uses of naked returns in `types/lookup.go` found by nakedret (nakedret will return the line number of the last naked return in the function):


```Go
func MissingMethod(V Type, T *Interface, static bool) (method *Func, wrongType bool) {
	// fast path for common case
	if T.Empty() {
		return
	}

	// TODO(gri) Consider using method sets here. Might be more efficient.

	if ityp, _ := V.Underlying().(*Interface); ityp != nil {
		// TODO(gri) allMethods is sorted - can do this more efficiently
		for _, m := range T.allMethods {
			_, obj := lookupMethod(ityp.allMethods, m.pkg, m.name)
			switch {
			case obj == nil:
				if static {
					return m, false
				}
			case !Identical(obj.Type(), m.typ):
				return m, true
			}
		}
		return
	}

	// A concrete type implements T if it implements all methods of T.
	for _, m := range T.allMethods {
		obj, _, _ := lookupFieldOrMethod(V, false, m.pkg, m.name)

		f, _ := obj.(*Func)
		if f == nil {
			return m, false
		}

		if !Identical(f.typ, m.typ) {
			return m, true
		}
	}

	return
}
```

## TODO

- Unit tests (may require some refactoring to do correctly)
- supporting toggling of `build.Context.UseAllFiles` may be useful for some. 
- Configuration on whether or not to run on test files
- Vim quickfix format?


## Contributing

Pull requests welcome!


## Other static analysis tools

If you've enjoyed nakedret, take a look at my other static anaylsis tools!

- [unimport](https://github.com/alexkohler/unimport) - Finds unnecessary import aliases
- [prealloc](https://github.com/alexkohler/prealloc) - Finds slice declarations that could potentially be preallocated.

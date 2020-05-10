# Go Symbols

A utility for extracting a JSON representation of the package symbols from a go source tree.

If a directory named src is under the directory given that directory will be walked for source code,
otherwise the entire tree will be walked.

# Installing

```
go get -u github.com/newhook/go-symbols
```

# Using

```
> go-symbols /Users/matthew/go foo
```

# Schema

```
go
type symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Package   string `json:"package"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}
```

# structtag [![](https://github.com/alfatraining/structtag/workflows/build/badge.svg)](https://github.com/alfatraining/structtag/actions) [![PkgGoDev](https://pkg.go.dev/badge/github.com/alfatraining/structtag)](https://pkg.go.dev/github.com/alfatraining/structtag)

`structtag` is a library for parsing struct tags in Go during static analysis. 
It is designed to provide a simple API for parsing struct field tags in Go code. 
This fork focuses exclusively on **reading struct tags** and is not intended for modifying or rewriting them.
It also drops support for accessing the options of a struct tag individually.

This project is a fork of [`fatih/structtag`](https://github.com/fatih/structtag), originally created by Fatih Arslan. 
The primary changes in this fork are:

- struct tag values are treated as blobs, options are not treated individually,
- the API surface is minimized, just so that the functionality used by [`4meepo/tagalign`](https://github.com/4meepo/tagalign) is provided.

---

## Install

To install the package:

```bash
go get github.com/alfatraining/structtag
```

---

## Example Usage

The following example demonstrates how to parse and output struct tags using `structtag`:

```go
package main

import (
	"fmt"
	"reflect"

	"github.com/alfatraining/structtag"
)

func main() {
	type Example struct {
		Field string `json:"foo,omitempty" xml:"bar"`
	}

	// Get the struct tag from the field
	tag := reflect.TypeOf(Example{}).Field(0).Tag

	// Parse the tag using structtag
	tags, err := structtag.Parse(string(tag))
	if err != nil {
		panic(err)
	}

	// Iterate over all tags
	for _, t := range tags.Tags() {
		fmt.Printf("Key: %s, Value: %v\n", t.Key, t.Value)
	}
	// Output:
	// Key: json, Value: foo,omitempty
	// Key: xml, Value: bar
}
```

---

## API Overview

### Parsing Struct Tags
Use `Parse` to parse a struct field's tag into a `Tags` object:
```go
tags, err := structtag.Parse(`json:"foo,omitempty" xml:"bar"`)
if err != nil {
    panic(err)
}
```

### Accessing Tags
- **Retrieve all tags**:
  ```go
  allTags := tags.Tags()
  ```

### Inspecting Tags
- **Key**: The key of the tag (e.g., `json` or `xml`).
- **Value**: The value of the tag (e.g., `"foo,omitempty"` for `json:"foo,omitempty"`).

---

## Acknowledgments

This project is based on [`fatih/structtag`](https://github.com/fatih/structtag), created by Fatih Arslan.
Inspiration for the style of this README was taken from [tylergannon/structtag](https://github.com/tylergannon/structtag), created by Tyler Gannon.

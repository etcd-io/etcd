# go-etcd

golang client library for etcd

This etcd client library is under heavy development. Check back soon for more
docs. In the meantime, check out [etcd](https://github.com/coreos/etcd) for
details on the client protocol. 

For usage see example below or look at godoc: [go-etcd/etcd](http://godoc.org/github.com/coreos/go-etcd/etcd)

## Install

```bash
go get github.com/coreos/go-etcd/etcd
```

## Examples

Returning error values are not showed for the sake of simplicity, but you
should check them.

```go
package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
)

func main() {
	c := etcd.NewClient() // default binds to http://0.0.0.0:4001

	// SET the value "bar" to the key "foo" with zero TTL
	// returns a: *store.Response
	res, _ := c.Set("foo", "bar", 0)
	fmt.Printf("set response: %+v\n", res)

	// GET the value that is stored for the key "foo"
	// return a slice: []*store.Response
	values, _ := c.Get("foo")
	for i, res := range values { // .. and print them out
		fmt.Printf("[%d] get response: %+v\n", i, res)
	}

	// DELETE the key "foo"
	// returns a: *store.Response
	res, _ = c.Delete("foo")
	fmt.Printf("delete response: %+v\n", res)
}
```

// Package checkers is a go-critic linter main checkers collection.
package checkers

import (
	"os"

	"github.com/go-critic/go-critic/linter"
)

var collection = &linter.CheckerCollection{
	URL: "https://github.com/go-critic/go-critic/checkers",
}

var debug = func() func() bool {
	v := os.Getenv("DEBUG") != ""
	return func() bool {
		return v
	}
}()

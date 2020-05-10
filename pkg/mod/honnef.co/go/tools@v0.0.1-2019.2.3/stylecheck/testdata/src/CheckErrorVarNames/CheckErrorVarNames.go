// Package pkg ...
package pkg

import (
	"errors"
	"fmt"
)

var (
	foo                   = errors.New("") // want `error var foo should have name of the form errFoo`
	errBar                = errors.New("")
	qux, fisk, errAnother = errors.New(""), errors.New(""), errors.New("") // want `error var qux should have name of the form errFoo` `error var fisk should have name of the form errFoo`
	abc                   = fmt.Errorf("")                                 // want `error var abc should have name of the form errFoo`

	errAbc = fmt.Errorf("")
)

var wrong = errors.New("") // want `error var wrong should have name of the form errFoo`

var result = fn()

func fn() error { return nil }

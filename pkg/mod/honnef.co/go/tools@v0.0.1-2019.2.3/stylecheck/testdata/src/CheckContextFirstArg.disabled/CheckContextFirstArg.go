// Package pkg ...
package pkg

import "context"

type T int

func fn1(int)                                   {}
func fn2(context.Context, int)                  {}
func fn3(context.Context, int, context.Context) {}
func fn4(int, context.Context)                  {} // want `context\.Context should be the first argument of a function`
func (T) FN(int, context.Context)               {} // want `context\.Context should be the first argument of a function`

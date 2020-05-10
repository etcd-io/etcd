package main

import (
	"testing"
)

type t1 struct{}

func TestFoo(t *testing.T) {
	_ = t1{}
}

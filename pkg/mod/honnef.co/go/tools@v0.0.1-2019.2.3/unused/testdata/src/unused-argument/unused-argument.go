package main

type t1 struct{}
type t2 struct{}

func (t1) foo(arg *t2) {}

func init() {
	t1{}.foo(nil)
}

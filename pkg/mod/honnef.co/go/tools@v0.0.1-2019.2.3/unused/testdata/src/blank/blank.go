package pkg

import _ "fmt"

type t1 struct{} // want `t1`
type t2 struct {
	_ int
}
type t3 struct{}
type t4 struct{}
type t5 struct{}

var _ = t2{}

func fn1() { // want `fn1`
	_ = t1{}
	var _ = t1{}
}

func fn2() {
	_ = t3{}
	var _ t4
	var _ *t5 = nil
}

func init() {
	fn2()
}

func _() {}

type _ struct{}

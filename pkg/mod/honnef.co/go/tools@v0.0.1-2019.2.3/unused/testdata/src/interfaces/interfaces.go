package pkg

type I interface {
	fn1()
}

type t struct{}

func (t) fn1() {}
func (t) fn2() {} // want `fn2`

func init() {
	_ = t{}
}

type I1 interface {
	Foo()
}

type I2 interface {
	Foo()
	bar()
}

type t1 struct{}
type t2 struct{}
type t3 struct{}
type t4 struct{ t3 }

func (t1) Foo() {}
func (t2) Foo() {}
func (t2) bar() {}
func (t3) Foo() {}
func (t3) bar() {}

func Fn() {
	var v1 t1
	var v2 t2
	var v3 t3
	var v4 t4
	_ = v1
	_ = v2
	_ = v3
	var x interface{} = v4
	_ = x.(I2)
}

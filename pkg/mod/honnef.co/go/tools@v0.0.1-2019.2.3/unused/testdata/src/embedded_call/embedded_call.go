package pkg

var t1 struct {
	t2
	t3
	t4
}

type t2 struct{}
type t3 struct{}
type t4 struct{ t5 }
type t5 struct{}

func (t2) foo() {}
func (t3) bar() {}
func (t5) baz() {}
func init() {
	t1.foo()
	_ = t1.bar
	t1.baz()
}

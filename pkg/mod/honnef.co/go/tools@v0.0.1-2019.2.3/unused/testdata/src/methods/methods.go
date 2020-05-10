package pkg

type t1 struct{}
type t2 struct{ t3 }
type t3 struct{}

func (t1) Foo() {}
func (t3) Foo() {}
func (t3) foo() {} // want `foo`

func init() {
	_ = t1{}
	_ = t2{}
}

package pkg

type I interface {
	foo()
}

type T struct{}

func (T) foo() {}
func (T) bar() {} // want `bar`

var _ struct{ T }

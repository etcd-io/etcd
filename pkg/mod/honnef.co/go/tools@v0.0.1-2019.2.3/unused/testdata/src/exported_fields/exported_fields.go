package pkg

type t1 struct {
	F1 int
}

type T2 struct {
	F2 int
}

var v struct {
	T3
}

type T3 struct{}

func (T3) Foo() {}

func init() {
	v.Foo()
}

func init() {
	_ = t1{}
}

type codeResponse struct {
	Tree *codeNode `json:"tree"`
}

type codeNode struct {
}

func init() {
	_ = codeResponse{}
}

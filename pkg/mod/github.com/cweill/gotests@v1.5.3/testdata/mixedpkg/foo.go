package foo

type Foo struct {
	Bar string
}

func (*Foo) Foo(s string) error {
	return nil
}

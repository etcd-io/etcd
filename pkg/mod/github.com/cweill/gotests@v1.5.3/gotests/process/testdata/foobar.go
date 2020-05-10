package foobar

type Foo struct {
	Bar string
}

func (*Foo) Foo(s string) error {
	return nil
}

type Bar struct {
	Foo string
}

func (*Bar) bar(s string) error {
	return nil
}

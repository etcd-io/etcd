package bar

type Bar struct {
	Foo string
}

func (*Bar) Bar(s string) error {
	return nil
}

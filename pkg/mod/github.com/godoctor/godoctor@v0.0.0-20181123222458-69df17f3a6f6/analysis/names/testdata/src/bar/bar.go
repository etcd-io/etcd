package bar

func Exported() string { return "bar" }

type I interface {
	Method() int
}

type t int
func (t) Method() float64 { return 1.234 }

func callInterface(i I) {
	i.Method()
}

type i struct {
	Method func() float64 // Not actually a method -- this is a field!
}

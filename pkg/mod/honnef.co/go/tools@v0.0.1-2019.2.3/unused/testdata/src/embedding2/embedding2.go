package main

type AA interface {
	A()
}

type BB interface {
	AA
}

type CC interface {
	BB
	C()
}

func c(cc CC) {
	cc.A()
}

type z struct{}

func (z) A() {}
func (z) B() {}
func (z) C() {}

func main() {
	c(z{})
}

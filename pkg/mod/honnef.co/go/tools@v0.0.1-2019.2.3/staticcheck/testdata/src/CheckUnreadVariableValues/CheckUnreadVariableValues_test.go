package pkg

import "testing"

func TestFoo(t *testing.T) {
	x := fn() // want `never used`
	x = fn()
	println(x)
}

func ExampleFoo() {
	x := fn()
	x = fn()
	println(x)
}

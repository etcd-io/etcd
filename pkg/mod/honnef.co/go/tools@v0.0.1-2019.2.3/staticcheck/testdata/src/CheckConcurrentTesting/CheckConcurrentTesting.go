package pkg

import "testing"

func fn1() {
	var t *testing.T
	t.Fatal()
	go func() { // want `the goroutine calls T\.Fatal, which must be called in the same goroutine as the test`
		t.Fatal()
	}()
	go fn2(t) // want `the goroutine calls T\.Fatal, which must be called in the same goroutine as the test`
	func() {
		t.Fatal()
	}()

	fn := func() {
		t.Fatal()
	}
	fn()
	go fn() // want `the goroutine calls T\.Fatal, which must be called in the same goroutine as the test`
}

func fn2(t *testing.T) {
	t.Fatal()
}

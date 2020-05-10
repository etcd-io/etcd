package pkg

// void foo(void **p) {}
import "C"
import "unsafe"

func Foo() {
	var p unsafe.Pointer

	C.foo(&p)
	if 0 == 0 {
		// We don't currently flag this instance of 0 == 0 because of
		// our cgo-specific exception.
		println()
	}
}

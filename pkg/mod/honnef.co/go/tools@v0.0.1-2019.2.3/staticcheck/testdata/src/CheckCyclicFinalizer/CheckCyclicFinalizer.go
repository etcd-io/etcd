package pkg

import (
	"fmt"
	"runtime"
)

func fn() {
	var x *int
	foo := func(y *int) { fmt.Println(x) }
	runtime.SetFinalizer(x, foo) // want `the finalizer closes over the object, preventing the finalizer from ever running \(at .+:10:9`
	runtime.SetFinalizer(x, nil)
	runtime.SetFinalizer(x, func(_ *int) { // want `the finalizer closes over the object, preventing the finalizer from ever running \(at .+:13:26`
		fmt.Println(x)
	})

	foo = func(y *int) { fmt.Println(y) }
	runtime.SetFinalizer(x, foo)
	runtime.SetFinalizer(x, func(y *int) {
		fmt.Println(y)
	})
}

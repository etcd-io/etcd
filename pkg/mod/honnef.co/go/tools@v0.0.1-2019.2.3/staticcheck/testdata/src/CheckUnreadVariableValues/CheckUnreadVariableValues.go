package pkg

func fn1() {
	var x int
	x = gen() // want `this value of x is never used`
	x = gen()
	println(x)

	var y int
	if true {
		y = gen() // want `this value of y is never used`
	}
	y = gen()
	println(y)
}

func gen() int {
	println() // make it unpure
	return 0
}

func fn2() {
	x, y := gen(), gen() // want `this value of x is never used` `this value of y is never used`
	x, y = gen(), gen()
	println(x, y)
}

func fn3() {
	x := uint32(0)
	if true {
		x = 1
	} else {
		x = 2
	}
	println(x)
}

func gen2() (int, int) {
	println()
	return 0, 0
}

func fn4() {
	x, y := gen2() // want `this value of x is never used`
	println(y)
	x, y = gen2() // want `this value of x is never used` `this value of y is never used`
	x, _ = gen2() // want `this value of x is never used`
	x, y = gen2()
	println(x, y)
}

func fn5(m map[string]string) {
	v, ok := m[""] // want `this value of v is never used` `this value of ok is never used`
	v, ok = m[""]
	println(v, ok)
}

func fn6() {
	x := gen()
	// Do not report variables if they've been assigned to the blank identifier
	_ = x
}

func fn7() {
	func() {
		var x int
		x = gen() // want `this value of x is never used`
		x = gen()
		println(x)
	}()
}

func fn() int { println(); return 0 }

var y = func() {
	v := fn() // want `never used`
	v = fn()
	println(v)
}

func fn8() {
	x := gen()
	switch x {
	}

	y := gen() // want `this value of y is never used`
	y = gen()
	switch y {
	}

	z, _ := gen2()
	switch z {
	}

	_, a := gen2()
	switch a {
	}

	b, c := gen2() // want `this value of b is never used`
	println(c)
	b, c = gen2() // want `this value of c is never used`
	switch b {
	}
}

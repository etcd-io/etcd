package pkg

func fn() {
	var x int // want `should merge variable declaration with assignment on next line`
	x = 1
	_ = x

	var y interface{} // want `should merge variable declaration with assignment on next line`
	y = 1
	_ = y

	if true {
		var x string // want `should merge variable declaration with assignment on next line`
		x = ""
		_ = x
	}

	var z []string
	z = append(z, "")
	_ = z

	var f func()
	f = func() { f() }
	_ = f

	var a int
	a = 1
	a = 2
	_ = a

	var b int
	b = 1
	// do stuff
	b = 2
	_ = b
}

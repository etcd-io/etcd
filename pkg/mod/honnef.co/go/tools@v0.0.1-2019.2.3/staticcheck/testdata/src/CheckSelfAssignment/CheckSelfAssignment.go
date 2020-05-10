package pkg

func fn(x int) {
	var z int
	var y int
	x = x             // want `self-assignment`
	y = y             // want `self-assignment`
	y, x, z = y, x, 1 // want `self-assignment of y to y` `self-assignment of x to x`
	y = x
	_ = y
	_ = x
	_ = z
	func() {
		x := x
		println(x)
	}()
}

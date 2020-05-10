package pkg

var x = func(arg int) { // want `overwritten`
	arg = 1
	println(arg)
}

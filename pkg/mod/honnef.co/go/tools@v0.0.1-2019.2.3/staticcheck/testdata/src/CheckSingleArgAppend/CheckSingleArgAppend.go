package pkg

//lint:file-ignore SA4010,SA4006 Not relevant to this test case

func fn(arg []int) {
	x := append(arg) // want `x = append\(y\) is equivalent to x = y`
	_ = x
	y := append(arg, 1)
	_ = y
	arg = append(arg) // want `x = append\(y\) is equivalent to x = y`
	arg = append(arg, 1, 2, 3)
	var nilly []int
	arg = append(arg, nilly...)
	arg = append(arg, arg...)
}

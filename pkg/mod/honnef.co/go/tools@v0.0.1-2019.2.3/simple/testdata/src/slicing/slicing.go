package pkg

func fn() {
	var s []int
	_ = s[:len(s)] // want `omit second index`

	len := func(s []int) int { return -1 }
	_ = s[:len(s)]
}

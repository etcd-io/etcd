package pkg

func fn(x int) {
	println(x | 0)        // want `x \| 0 always equals x`
	println(x & 0)        // want `x & 0 always equals 0`
	println(x ^ 0)        // want `x \^ 0 always equals x`
	println((x << 5) | 0) // want `x \| 0 always equals x`
	println(x | 1)
	println(x << 0)
}

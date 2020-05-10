package pkg

import "math"

func fn1() {
	var x int
	var y int
	println(x == 0) // MATCH /x == 0 is always true for all possible values/
	println(x == y) // MATCH /x == y is always true for all possible values/
	x = 1
	println(x > 0) // MATCH /x > 0 is always true for all possible values/
	f := math.NaN()
	println(f == f)
}

func fn2(x int) {
	println(x == 0)
}

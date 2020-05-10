package pkg

func fn() {
	const c = 0
	var x, y int
	type s []int
	type ch chan int
	_ = make([]int, 1)
	_ = make([]int, 0)       // length is mandatory for slices, don't suggest removal
	_ = make(s, 0)           // length is mandatory for slices, don't suggest removal
	_ = make(chan int, c)    // constant of 0 may be due to debugging, math or platform-specific code
	_ = make(chan int, 0)    // want `should use make\(chan int\) instead`
	_ = make(ch, 0)          // want `should use make\(ch\) instead`
	_ = make(map[int]int, 0) // want `should use make\(map\[int\]int\) instead`
	_ = make([]int, 1, 1)    // want `should use make\(\[\]int, 1\) instead`
	_ = make([]int, x, x)    // want `should use make\(\[\]int, x\) instead`
	_ = make([]int, 1, 2)
	_ = make([]int, x, y)
}

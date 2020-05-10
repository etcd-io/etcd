package pkg

func fn() bool { return true }
func fn1() bool {
	x := true
	if x { // want `should use 'return <expr>'`
		return true
	}
	return false
}

func fn2() bool {
	x := true
	if !x {
		return true
	}
	if x {
		return true
	}
	return false
}

func fn3() int {
	var x bool
	if x {
		return 1
	}
	return 2
}

func fn4() bool { return true }

func fn5() bool {
	if fn() { // want `should use 'return <expr>'`
		return false
	}
	return true
}

func fn6() bool {
	if fn3() != fn3() { // want `should use 'return <expr>'`
		return true
	}
	return false
}

func fn7() bool {
	if 1 > 2 { // want `should use 'return <expr>'`
		return true
	}
	return false
}

func fn8() bool {
	if fn() || fn() {
		return true
	}
	return false
}

func fn9(x int) bool {
	if x > 0 {
		return true
	}
	return true
}

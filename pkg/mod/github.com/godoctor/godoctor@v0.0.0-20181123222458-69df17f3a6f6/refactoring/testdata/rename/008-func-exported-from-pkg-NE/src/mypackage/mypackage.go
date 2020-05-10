package mypackage

func MyFunction(n int) int { // <<<<< rename,3,6,3,6,xyz,pass
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}

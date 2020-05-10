package mypackage  
func MyFunction(n int) int {             
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}

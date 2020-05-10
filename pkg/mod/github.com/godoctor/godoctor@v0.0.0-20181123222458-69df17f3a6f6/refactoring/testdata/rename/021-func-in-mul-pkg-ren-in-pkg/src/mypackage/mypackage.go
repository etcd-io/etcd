package mypackage 
func MyFunction(n int) int {    // <<<<< rename,2,6,2,6,Xyz,pass         
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}

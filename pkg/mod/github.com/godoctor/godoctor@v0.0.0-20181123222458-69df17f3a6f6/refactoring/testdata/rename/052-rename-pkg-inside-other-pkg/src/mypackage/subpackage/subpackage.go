package subpackage // <<<<< rename,1,10,1,10,Xyz,fail 
func MyFunction(n int) int {             
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-2)
	}
}

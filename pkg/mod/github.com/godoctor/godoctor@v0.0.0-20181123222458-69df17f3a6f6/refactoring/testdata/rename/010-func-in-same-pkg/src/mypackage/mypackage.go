package mypackage

//Test for renaming a function with in same package
 
func MyFunction(n int) int {           // <<<<< rename,5,6,5,6,Xyz,pass  
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}

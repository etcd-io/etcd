package Mypackage

//Test for renaming a function with in same package
 
func Mypackage(n int) int {           // <<<<< rename,5,6,5,6,Xyz,pass  
	if n == 0 {
		return 1
	} else {
		return n + Mypackage(n-1)
	}
}

package mypackage
import "fmt" 
func MyFunction(n int) int {             
	if n == 0 {
		return 1
	} else {
		return n + MyFunction(n-1)
	}
}

func Xyz() {

fmt.Println("this is a dummy function")

}

// <<<<< toggle,7,2,7,40,pass
package main

import "fmt"

func main() {
	var f func() int = func()int {return 7}
	fmt.Println("Function f :",f)
}

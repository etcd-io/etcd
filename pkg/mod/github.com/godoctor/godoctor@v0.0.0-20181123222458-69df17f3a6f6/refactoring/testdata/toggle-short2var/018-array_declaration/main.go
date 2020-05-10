// <<<<< toggle,7,2,7,30,pass
package main

import "fmt"

func main() {
	array := [...]int{1, 2, 3, 4, 5}
	slice1 := array[2:5]
	fmt.Println("slice1 is :", slice1)
}

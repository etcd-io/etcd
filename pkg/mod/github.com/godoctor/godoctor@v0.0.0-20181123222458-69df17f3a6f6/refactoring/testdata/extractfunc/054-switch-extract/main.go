//<<<<< extract,9,2,14,2,newFunc,pass
package main

import "fmt"

func main() {
	s1 := "abcd" + ""
	s2 := "efgh" + ""
	switch s1 {
	case s2:
		fmt.Println("the two strings are equal")
	default:
		fmt.Println("the two strings are NOT equal")
	}
}

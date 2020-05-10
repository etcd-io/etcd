//<<<<<extract,9,4,11,33,foo,pass
package main

import "fmt"

func main() {
	b := 4
	c := 5
	b = 5
	c = 6
	fmt.Println("IN EXTRACT", b, c)
	fmt.Println("IN MAIN", b, c)
}

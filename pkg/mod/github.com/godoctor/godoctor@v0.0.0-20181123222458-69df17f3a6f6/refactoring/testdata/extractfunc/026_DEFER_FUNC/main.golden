//<<<<<extract,12,2,13,25,Foo,fail
package main

import "fmt"

func main() {
	a()
}

func a() {
	i := 0
	defer fmt.Println(i)
	fmt.Println("statement")
	fmt.Println("last Statement")
	i++
	return
}

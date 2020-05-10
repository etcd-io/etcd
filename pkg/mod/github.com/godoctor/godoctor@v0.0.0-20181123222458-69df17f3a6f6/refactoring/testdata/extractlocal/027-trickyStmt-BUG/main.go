package main

import "fmt"

func main() {
	var i int
	var x int
	goto a
b:
	x = i + i // <<<<< var,10,5,10,10,newVar,pass
	goto c
a:
	i = 3
	goto b
c:
	fmt.Println("tricky test", x)
}

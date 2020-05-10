package main

import "fmt"

func main() {
	var i int
	var x int
	goto a
b:
	if x > i {
		x = i + i // <<<<< var,11,7,11,11,newVar,pass
	}
	goto c
a:
	i = 3
	goto b
c:
	fmt.Println("tricky test", x)

}

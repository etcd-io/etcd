package main

import "fmt"

func main() {
	i := 0; j := 1; k := /* seriously */ j + 1 /* dude */
		// <<<<< var,6,39,6,43,newVar,pass
	fmt.Printf("%d %d %d\n", i, j, k)
}

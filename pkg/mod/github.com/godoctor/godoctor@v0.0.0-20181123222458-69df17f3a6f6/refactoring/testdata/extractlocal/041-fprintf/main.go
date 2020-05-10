package main

import (
	"fmt"
	"os"
)

func main() {
	x := 5
	u := "alpha"
	v := "beta"
	i := "gamma"
	if x > 4 {
		fmt.Fprint(os.Stderr, "\tsetphi %s edge %s -> %s (#%d) (alloc=%s) := %s\n",
			u, v, i) // <<<<< var,14,3,15,12,newVar,fail
	}

}

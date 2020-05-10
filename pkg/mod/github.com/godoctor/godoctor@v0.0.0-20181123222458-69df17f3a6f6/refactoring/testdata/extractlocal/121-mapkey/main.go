package main

import "fmt"

func main() {
	// Here, boo *could* be extractable but currently is not
	x := map[string]string{
		"b" + "oo": "foo", //<<<<< var,7,3,7,10,newVar,fail
	}
	fmt.Println(x)

	type S struct {
		boo string
	}
	// Here, boo is definitely not extractable
	y := S{
		boo: "foo", //<<<<< var,15,3,15,5,newVar2,fail
	}
	fmt.Println(y)
}

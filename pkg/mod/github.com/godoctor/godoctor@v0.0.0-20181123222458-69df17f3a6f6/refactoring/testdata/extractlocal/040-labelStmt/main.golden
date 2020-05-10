package main

import (
	"fmt"
	"strconv"
)

func main() {
	x := 5
	if x == 5 {
		goto vars
	}
vars: // <<<<< var,13,1,13,5,newVar,fail
	a := []string{"subdomain", "foo", "category", "technology", "id", "42"}
	fmt.Println(a)
	goto url
url:
	y := "http://foo.domain.com/articles/technology/42"
	fmt.Println(y)

	fmt.Println("works : " + strconv.Itoa(x))
}

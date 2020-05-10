package main

import "fmt"

type s struct {
	name string
}

func main() {
	v := s{name: "foo"} // <<<<< var,10,9,10,12,newVar,fail
	fmt.Println(v.name)
}

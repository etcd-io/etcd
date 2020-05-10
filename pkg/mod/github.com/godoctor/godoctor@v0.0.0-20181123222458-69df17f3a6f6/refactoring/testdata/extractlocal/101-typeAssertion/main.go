package main

import "fmt"

type i interface {
}

type s struct {
	name string
}

func main() {
	var v i = &s{name: "foo"}
	w := v.(*s) // <<<<< var,14,11,14,11,newVar,fail
	fmt.Println(w.name)
}

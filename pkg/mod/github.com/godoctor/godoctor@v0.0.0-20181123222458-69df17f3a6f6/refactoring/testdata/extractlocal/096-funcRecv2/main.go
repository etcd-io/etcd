package main

import "fmt"

func apple() string {
	a := "apple +"
	return a
}

type fruit struct {
	name string
}

func (f *fruit) orange() string { // <<<<< var,14,6,14,15,newVar,fail
	return "helloz worldz"
}

func main() {

	o2 := fruit{"os"}
	s := o2.orange()
	fmt.Println(s)
}

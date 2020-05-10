package main

import "fmt"

func apple() string {
	a := "apple +"
	return a
}

type fruit struct {
	name string
}

func (f *fruit) orange() string {
	return "helloz worldz"
}

func main() {

	o2 := fruit{"os"}
	s := o2.orange()
	s.name = "happy" // <<<<< var,22,2,22,7,newVar,fail
	fmt.Println(s)
}

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
	if f == nil { // <<<<< var,15,5,15,6,newVar,pass
		return "doesn't work buddy"
	}
	return "helloz worldz"
}

func main() {

	o2 := fruit{"os"}
	s := o2.orange()
	fmt.Println(s)
}

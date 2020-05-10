package main

import "fmt"

func main() {
	s := &struct {
		member int
	}{}
	s = &struct{ member int }{4} //<<<<<extract,9,1,10,1,extracted,pass
	fmt.Println(s.member)
}

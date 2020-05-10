package main

import "fmt"

func main() {
	var s struct {
		member int
	}
	s.member = 1
	s.member = 3 //<<<<<extract,10,1,11,1,extracted,pass
	fmt.Println(s.member)
}

package main

import "fmt"

type I interface { String() string }
type S struct {}
func (s *S) String() string { return "" } //<<<<<debug,7,13,7,18,showaffected,pass

func String() { return "" }

func main() {
	fmt.Println(S{})
}

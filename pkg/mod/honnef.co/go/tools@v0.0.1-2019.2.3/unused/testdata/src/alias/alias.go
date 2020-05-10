package main

type t1 struct{}
type t2 struct{} // want `t2`
type t3 struct{}

type alias1 = t1
type alias2 = t2 // want `alias2`
type alias3 = t3
type alias4 = int

func main() {
	var _ alias1
	var _ t3
}

package main

func Fn1() {}
func Fn2() {} // want `Fn2`

const X = 1 // want `X`

var Y = 2 // want `Y`

type Z struct{} // want `Z`

func main() {
	Fn1()
}

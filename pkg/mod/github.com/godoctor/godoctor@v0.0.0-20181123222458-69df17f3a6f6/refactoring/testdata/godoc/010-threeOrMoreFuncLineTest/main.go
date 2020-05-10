package main // <<<<< godoc,1,1,1,1,pass

import "fmt"

func main() {
	Exported()
}

func Exported() {
	fmt.Println("Hello, Go")
}

type Shaper interface {
}

type Rectangle struct {
}

type square struct {
}

type circle interface {
}

type Oval struct {
}

func A() { }; func B() { }; func c() { }; func D() { }

type alpha struct { }; type Beta struct { }

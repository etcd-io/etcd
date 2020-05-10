package main // <<<<< godoc,1,1,1,1,pass

import "fmt"

func main() {
	Exported()
}

func Exported() {
	fmt.Println("Hello, Go")
	
	j := 0
	for i := 0; i < 5; i++ {
		j++
	}
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

func A() { }; func B() { }

type alpha struct { }; type Beta struct { }

func c() { }; func D() { }

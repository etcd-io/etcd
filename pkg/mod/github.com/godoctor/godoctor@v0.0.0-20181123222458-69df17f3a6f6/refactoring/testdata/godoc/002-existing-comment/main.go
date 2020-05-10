package main // <<<<< godoc,1,1,1,1,pass

import "fmt"

func main() {
	fmt.Println("hello")
}

// MyInt is mine
type MyInt int

// MyInterface is my interface
type MyInterface interface {
	SumSides() int
}

// Shape is a struct
type Shape struct {
	NumSides int
	Sides    []int
}

// SumSides adds all of the sides together
func (s *Shape) SumSides() int {
	var sum int
	for _, side := range s.Sides {
		sum += side
	}
	return sum
}

// Hello says hello
func Hello(name string) {
	fmt.Printf("Hello, %s\n", name)
}

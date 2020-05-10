//<<<<<extract,16,2,19,15,foo,pass
package main

import "fmt"

type Pt struct {
	x, y int
}

func main() {
	a := 7 + 0
	b := 2 + 0
	p := Pt{3, 4}
	fmt.Println("Old Pt", p)
	p.x = 5
	p.y = 6
	fmt.Println("New Pt", p)
	c := a * b
	n := p.x * p.y
	fmt.Println("Value of c is ", c)
	fmt.Println("Value of n is ", n)
	fmt.Println("Printing Modified point after returning", p)
}

//<<<<<extract,14,2,15,8,foo,pass
package main

import "fmt"

type Pt struct {
	x, y int
}

func main() {
	p := Pt{3, 4}
	fmt.Println("Old Pt", p)
	p.x = 5
	fmt.Println("This is testing")
	p.y = 6
	fmt.Print("New Pt", p)
}

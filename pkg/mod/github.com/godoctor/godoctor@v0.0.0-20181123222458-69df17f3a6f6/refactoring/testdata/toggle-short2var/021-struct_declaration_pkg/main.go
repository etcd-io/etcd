//<<<<< toggle,10,2,10,24,pass
package main

import (
    "fmt"	
    "shapes"
    )

func main() {
	s:=shapes.InitSquare(4)
	fmt.Println("Side of square is:",s)
	fmt.Println("Area of the square is :",s.Area())
}

// <<<<< toggle,15,2,15,22,pass
package main

import "fmt"

type Rectangle struct {
	length, width float64
}

func area(r Rectangle) float64 {
    	return r.length * r.width
}

func main() {
	r := Rectangle{5, 20}
	fmt.Println(area(r))
}

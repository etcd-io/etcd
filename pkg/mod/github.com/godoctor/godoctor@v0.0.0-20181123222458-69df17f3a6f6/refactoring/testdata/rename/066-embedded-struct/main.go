package main

import "fmt"

func main() {
	type Pt struct { //<<<<<rename,6,7,6,7,XY,pass
		x, y int
	}

	type Pt3 struct {
		Pt
		z int
	}

	pt3 := Pt3{Pt{1, 2}, 3}
	pt3.Pt.x *= 10
	pt3.y *= 20
	fmt.Println(pt3)
}

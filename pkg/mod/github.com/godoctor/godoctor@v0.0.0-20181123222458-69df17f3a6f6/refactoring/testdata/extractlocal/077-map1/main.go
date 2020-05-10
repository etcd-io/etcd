package main

import "fmt"

func main() {
	x := 2
	y := 5
	var m map[string]int     //<<<<< var,8,19,8,22,newVar,fail
	m = make(map[string]int) // initialize the map
	m["route"] = 66
	fmt.Println("please choose: x + x, x * y?")
	//fmt.Printf("map keyvalue is: %s and route", m["route"])
	fmt.Printf("x is %s, y is %s", x, y)
}

package main

import "fmt"

func main() {
	x := 2
	y := 5
	var m map[string]string
	m = map[string]string{
		"apple":   "pie", //<<<<< var,10,3,10,18,newVar,fail
		"potatoe": "tomato",
		"pizza":   "curry",
	}
	fmt.Println("please choose: x + x, x * y?")
	fmt.Printf("map keyvalue is: %s and route", m)
	fmt.Printf("x is %s, y is %s", x, y)
}

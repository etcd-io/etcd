package main

import (
	"fmt"
	"strconv"
)

func main() {
	fmt.Println("works")
	five := 5
	a := Apple{}
	a.printLine(five)
}

type Apple struct {
}

func (a *Apple) printLine(num int) { // <<<<< var,18,27,18,30,newVar,fail
	fmt.Println("number: " + strconv.Itoa(num))
}

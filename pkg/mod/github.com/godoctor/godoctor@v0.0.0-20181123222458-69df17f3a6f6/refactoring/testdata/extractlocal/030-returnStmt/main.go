package main

import "fmt"

func main() {
	num := run()
	fmt.Println(num)
}

func run() int {
	fmt.Println("works")
	return 1 + 2 // <<<<< var,12,8,12,13,newVar,pass
}

package main

import "fmt"

func main() {
	p := make([]string, 1)
	p[0] = "string"
	if p[0] != "/" { // <<<<< var,8,7,8,7,newVar,pass
		fmt.Println("Hello, 世界")
	}
}

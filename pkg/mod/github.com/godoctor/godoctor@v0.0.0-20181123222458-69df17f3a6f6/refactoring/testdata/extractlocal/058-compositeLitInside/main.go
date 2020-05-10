package main

import "fmt"

func main() {
	comp := Class{5}
	pr := &comp
	tree := School{*pr, Books{"/ab", "false", "/ab", "nil"}} // <<<<< var,8,44,8,49,newVar,pass
	fmt.Println("there's a name in there")
	for _, index := range tree.Books {
		fmt.Println(index)
	}
}

type School struct {
	Class
	Books
}
type Class struct {
	classNum int
}

type Books []string

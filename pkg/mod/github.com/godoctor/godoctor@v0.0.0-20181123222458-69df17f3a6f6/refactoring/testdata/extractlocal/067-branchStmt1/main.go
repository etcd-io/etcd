package main

import "fmt"

func main() {
	apple := 10
	orange := 3
	switch apple {
	case 1:
		fmt.Printf("this is a pizza")
		break
	case 3:
		fmt.Printf("this is an orange: %i", orange)
		break
	case 10:
		fmt.Printf("this is an apple: %i ", apple)
		fmt.Printf("this has a branchStmt")
		break //<<<<< var,18,3,18,8,newVar,fail
	}
}

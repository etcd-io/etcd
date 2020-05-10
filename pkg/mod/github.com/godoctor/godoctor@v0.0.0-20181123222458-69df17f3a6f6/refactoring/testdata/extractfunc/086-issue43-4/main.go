package main

import (
	"fmt"
	"strings"
)

func main() {
	inputString := "bob" + ""
	var hasSuffix bool
	// <<<<< extract,12,2,14,2,check,pass
	if strings.Contains(inputString, "Suffix") {
		hasSuffix = true
	}

	fmt.Print("string:", inputString)
	if hasSuffix {
		fmt.Println(" has Suffix")
	} else {
		fmt.Println(" is missing Suffix")
	}
}

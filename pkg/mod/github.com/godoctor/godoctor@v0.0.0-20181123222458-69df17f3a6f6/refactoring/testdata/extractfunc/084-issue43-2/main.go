package main

import (
	"fmt"
	"strings"
)

func main() {
	inputString := "bob"
	missingSuffix := true
	// If you extract this if
	// then it ignores the current value of missingSuffix
	// <<<<< extract,14,2,16,2,check,pass
	if strings.Contains(inputString, "Suffix") {
		missingSuffix = false
	}

	fmt.Print("string:", inputString)
	if missingSuffix {
		fmt.Println(" is missing Suffix")
	} else {
		fmt.Println(" contains Suffix")
	}
}

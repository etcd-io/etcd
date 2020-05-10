//<<<<<extract,11,3,13,3,FOO,fail
package main

import "fmt"

func main() {
	j := 4
	switch j {
	case 0, 2, 4, 6, 8:
		fmt.Println(j, "is even")
		if j == 4 {
			return
		}
		fallthrough
	default:
		fmt.Println("DEFAULT!")
	}
}

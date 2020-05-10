//<<<<<extract,9,3,11,3,FOO,fail
package main

import "fmt"

func main() {
	i := 0
	for i <= 5 {
		if i == 3 {
			break
		}
		fmt.Println(i)
		i++
	}
	fmt.Println("after loop")
}

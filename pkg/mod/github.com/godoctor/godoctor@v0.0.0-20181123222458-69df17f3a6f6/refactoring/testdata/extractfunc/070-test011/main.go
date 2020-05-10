//<<<<<extract,8,2,14,2,FOO,pass
package main

import "fmt"

func main() {
	i := 0 + 0
	for i <= 5 {
		if i == 3 {
			break
		}
		fmt.Println(i)
		i++
	}
	fmt.Println("after loop")
}

//<<<<<extract,8,2,14,2,Foo,pass
package main

import "fmt"

func main() {
	i := 0 + 0
	for {
		if i == 3 {
			break
		}
		fmt.Println("Value of i is:", i)
		i++
	}
}

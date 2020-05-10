//<<<<<extract,9,3,12,34,Foo,fail
package main

import "fmt"

func main() {
	i := 0
	for {
		if i == 3 {
			continue
		}
		fmt.Println("Value of i is:", i)
		i++
	}
	fmt.Println("A statement just after for loop.")
}

//<<<<<extract,8,1,17,2,Foo,pass
package main

import "fmt"

func main() {
	i := 5
ABC:
	fmt.Println("ABC")
	for i < 10 {
		i++
		if i == 8 {
			goto ABC
		} else {
			fmt.Println(i)
		}
	}
	fmt.Println("after for loop")
}

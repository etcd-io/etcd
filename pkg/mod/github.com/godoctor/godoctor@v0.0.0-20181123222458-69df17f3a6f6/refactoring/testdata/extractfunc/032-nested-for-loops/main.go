//<<<<<extract,10,3,20,3,Foo,pass
package main

import "fmt"

func main() {
	i := 1
	for k := 0; k < 3; k++ {
		fmt.Println("k =", k)
		for j := k; j < 5; j++ {
			fmt.Println("j =", j)
			fmt.Println("i =", i)
			if i%2 == 0 {
				fmt.Println("i is ", i)
				if i == 4 {
					break
				}
			}
			i++
		}
		fmt.Println("out of j loop")
	}
	fmt.Println("out of k loop")
}

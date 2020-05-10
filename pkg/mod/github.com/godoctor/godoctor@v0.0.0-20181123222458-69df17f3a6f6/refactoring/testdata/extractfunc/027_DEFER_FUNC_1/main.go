//<<<<<extract,7,2,9,2,Foo,fail
package main

import "fmt"

func main() {
	for i := 0; i < 4; i++ {
		defer fmt.Print(i)
	}
}

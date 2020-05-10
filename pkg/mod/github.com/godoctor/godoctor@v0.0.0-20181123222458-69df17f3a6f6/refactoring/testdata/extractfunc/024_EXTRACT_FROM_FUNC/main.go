//<<<<<extract,11,2,15,2,Foo,pass
package main

import (
	"fmt"
	"math/rand"
	"time"
)

func f(n int) {
	for i := 0; i < 5; i++ {
		fmt.Println(n, ":", i)
		amt := time.Duration(rand.Intn(250))
		time.Sleep(time.Millisecond * amt)
	}
	fmt.Println("Statement after for loop", n)
}

func main() {
	for i := 0; i < 5; i++ {
		go f(i)
	}
	var input string
	fmt.Scanln(&input)
}

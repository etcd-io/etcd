//<<<<<extract,20,1,31,2,Foo,pass
package main

import (
	"fmt"
	"time"
)

func main() {
	c1 := make(chan string)
	c2 := make(chan string)
	go func() {
		time.Sleep(time.Second * 1)
		c1 <- "one"
	}()
	go func() {
		time.Sleep(time.Second * 2)
		c2 <- "two"
	}()
H:
	for i := 0; i < 2; i++ {
		select {
		case msg1 := <-c1:
			fmt.Println("received", msg1)
			if msg1 == "one" {
				break H
			}
		case msg2 := <-c2:
			fmt.Println("received", msg2)
		}
	}
}

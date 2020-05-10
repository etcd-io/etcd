//<<<<< extract,26,3,33,3,newFunc,pass
package main

import (
	"fmt"
	"time"
)

func getMessagesChannel(msg string, delay time.Duration) <-chan string {
	c := make(chan string)
	go func() {
		for i := 1; i <= 10; i++ {
			c <- fmt.Sprintf("%s %d", msg, i)
			time.Sleep(time.Millisecond * delay)
		}
	}()
	return c
}

func main() {
	c1 := getMessagesChannel("first", 20)
	c2 := getMessagesChannel("second", 50)
	c3 := getMessagesChannel("third", 120)

	for i := 1; i <= 9; i++ {
		select {
		case msg := <-c1:
			println(msg)
		case msg := <-c2:
			println(msg)
		case msg := <-c3:
			println(msg)
		}
	}
}

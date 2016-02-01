package main

import "github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/gexpect"
import "fmt"
import "strings"

func main() {
	waitChan := make(chan string)

	fmt.Printf("Starting screen.. \n")

	child, err := gexpect.Spawn("screen")
	if err != nil {
		panic(err)
	}

	sender, reciever := child.AsyncInteractChannels()
	go func() {
		waitString := ""
		count := 0
		for {
			select {
			case waitString = <-waitChan:
				count++
			case msg, open := <-reciever:
				if !open {
					return
				}
				fmt.Printf("Recieved: %s\n", msg)

				if strings.Contains(msg, waitString) {
					if count >= 1 {
						waitChan <- msg
						count -= 1
					}
				}
			}
		}
	}()
	wait := func(str string) {
		waitChan <- str
		<-waitChan
	}
	fmt.Printf("Waiting until started.. \n")
	wait(" ")
	fmt.Printf("Sending Enter.. \n")
	sender <- "\n"
	wait("$")
	fmt.Printf("Sending echo.. \n")
	sender <- "echo Hello World\n"
	wait("Hello World")
	fmt.Printf("Received echo. \n")
}

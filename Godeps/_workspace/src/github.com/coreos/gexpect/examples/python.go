package main

import "github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/gexpect"
import "fmt"

func main() {
	fmt.Printf("Starting python.. \n")
	child, err := gexpect.Spawn("python")
	if err != nil {
		panic(err)
	}
	fmt.Printf("Expecting >>>.. \n")
	child.Expect(">>>")
	fmt.Printf("print 'Hello World'..\n")
	child.SendLine("print 'Hello World'")
	child.Expect(">>>")

	fmt.Printf("Interacting.. \n")
	child.Interact()
	fmt.Printf("Done \n")
	child.Close()
}

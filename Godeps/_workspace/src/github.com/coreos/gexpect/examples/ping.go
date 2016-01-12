package main

import gexpect "github.com/coreos/etcd/Godeps/_workspace/src/github.com/coreos/gexpect"
import "log"

func main() {
	log.Printf("Testing Ping interact... \n")

	child, err := gexpect.Spawn("ping -c8 127.0.0.1")
	if err != nil {
		panic(err)
	}
	child.Interact()
	log.Printf("Success\n")
}

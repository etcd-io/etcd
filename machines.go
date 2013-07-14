package main

import (
	"fmt"
)

type machine struct {
	hostname   string
	raftPort   int
	clientPort int
}

var machinesMap = map[string]machine{}

func addMachine(name string, hostname string, raftPort int, clientPort int) {

	machinesMap[name] = machine{hostname, raftPort, clientPort}

}

func getClientAddr(name string) (string, bool) {
	machine, ok := machinesMap[name]
	if !ok {
		return "", false
	}

	addr := fmt.Sprintf("%s:%v", machine.hostname, machine.clientPort)

	return addr, true
}

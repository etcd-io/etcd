package main

import (
	"fmt"
	"path"
	"strings"
)

func getClientAddr(name string) (string, bool) {
	response, _ := etcdStore.RawGet(path.Join("_etcd/machines", name))

	values := strings.Split(response[0].Value, ",")

	hostname := values[0]
	clientPort := values[2]

	addr := fmt.Sprintf("%s:%s", hostname, clientPort)

	return addr, true
}

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

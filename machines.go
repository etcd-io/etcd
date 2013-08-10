package main

import (
	"path"
	"net/url"
)

func getClientAddr(name string) (string, bool) {
	response, _ := etcdStore.RawGet(path.Join("_etcd/machines", name))

	m, err := url.ParseQuery(response[0].Value)

	if err != nil {
		panic("Failed to parse machines entry")
	}

	addr := m["client"][0]

	return addr, true
}

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

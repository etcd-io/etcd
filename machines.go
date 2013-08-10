package main

import (
	"net/url"
	"path"
)

func getClientAddr(name string) (string, bool) {
	resps, _ := etcdStore.RawGet(path.Join("_etcd/machines", name))

	m, err := url.ParseQuery(resps[0].Value)

	if err != nil {
		panic("Failed to parse machines entry")
	}

	addr := m["etcd"][0]

	return addr, true
}

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

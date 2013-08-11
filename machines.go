package main

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

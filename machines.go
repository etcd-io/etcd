package main

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

// getMachines gets the current machines in the cluster
func getMachines() []string {

	peers := raftServer.Peers()

	machines := make([]string, len(peers)+1)

	leader, ok := nameToEtcdURL(raftServer.Leader())
	self := info.EtcdURL
	i := 1

	if ok {
		machines[0] = leader
		if leader != self {
			machines[1] = self
			i = 2
		}
	} else {
		machines[0] = self
	}

	// Add all peers to the slice
	for peerName, _ := range peers {
		if machine, ok := nameToEtcdURL(peerName); ok {
			// do not add leader twice
			if machine != leader {
				machines[i] = machine
				i++
			}
		}
	}
	return machines
}

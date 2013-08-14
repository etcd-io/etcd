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

	leader, _ := nameToEtcdURL(raftServer.Leader())

	i := 0

	if leader != "" {

		// Add leader at the first of the machines list
		// Add server itself to the machine list
		// Since peer map does not contain the server itself
		if leader == info.EtcdURL {
			machines[i] = info.EtcdURL
			i++
		} else {
			machines[i] = leader
			i++
			machines[i] = info.EtcdURL
			i++
		}
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

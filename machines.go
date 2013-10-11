package main

// getMachines gets the current machines in the cluster
func (r *raftServer) getMachines(toURL func(string) (string, bool)) []string {
	peers := r.Peers()

	machines := make([]string, len(peers)+1)

	leader, ok := toURL(r.Leader())
	self, _ := toURL(r.Name())
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
		if machine, ok := toURL(peerName); ok {
			// do not add leader twice
			if machine != leader {
				machines[i] = machine
				i++
			}
		}
	}
	return machines
}

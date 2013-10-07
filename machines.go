/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

// machineNum returns the number of machines in the cluster
func machineNum() int {
	response, _ := etcdStore.RawGet("_etcd/machines")

	return len(response)
}

// getMachines gets the current machines in the cluster
func getMachines(toURL func(string) (string, bool)) []string {

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

// Copyright 2016 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package mocktikv

import "fmt"

// BootstrapWithSingleStore initializes a Cluster with 1 Region and 1 Store.
func BootstrapWithSingleStore(cluster *Cluster) (storeID, peerID, regionID uint64) {
	ids := cluster.AllocIDs(3)
	storeID, peerID, regionID = ids[0], ids[1], ids[2]
	cluster.AddStore(storeID, fmt.Sprintf("store%d", storeID))
	cluster.Bootstrap(regionID, []uint64{storeID}, []uint64{peerID}, peerID)
	return
}

// BootstrapWithMultiStores initializes a Cluster with 1 Region and n Stores.
func BootstrapWithMultiStores(cluster *Cluster, n int) (storeIDs, peerIDs []uint64, regionID uint64, leaderPeer uint64) {
	storeIDs = cluster.AllocIDs(n)
	peerIDs = cluster.AllocIDs(n)
	leaderPeer = peerIDs[0]
	regionID = cluster.AllocID()
	for _, storeID := range storeIDs {
		cluster.AddStore(storeID, fmt.Sprintf("store%d", storeID))
	}
	cluster.Bootstrap(regionID, storeIDs, peerIDs, leaderPeer)
	return
}

// BootstrapWithMultiRegions initializes a Cluster with multiple Regions and 1
// Store. The number of Regions will be len(splitKeys) + 1.
func BootstrapWithMultiRegions(cluster *Cluster, splitKeys ...[]byte) (storeID uint64, regionIDs, peerIDs []uint64) {
	var firstRegionID, firstPeerID uint64
	storeID, firstPeerID, firstRegionID = BootstrapWithSingleStore(cluster)
	regionIDs = append([]uint64{firstRegionID}, cluster.AllocIDs(len(splitKeys))...)
	peerIDs = append([]uint64{firstPeerID}, cluster.AllocIDs(len(splitKeys))...)
	for i, k := range splitKeys {
		cluster.Split(regionIDs[i], regionIDs[i+1], k, []uint64{peerIDs[i]}, peerIDs[i])
	}
	return
}

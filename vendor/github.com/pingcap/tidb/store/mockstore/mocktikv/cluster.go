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

import (
	"bytes"
	"math"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/tidb/tablecodec"
	"golang.org/x/net/context"
)

// Cluster simulates a TiKV cluster. It focuses on management and the change of
// meta data. A Cluster mainly includes following 3 kinds of meta data:
// 1) Region: A Region is a fragment of TiKV's data whose range is [start, end).
//    The data of a Region is duplicated to multiple Peers and distributed in
//    multiple Stores.
// 2) Peer: A Peer is a replica of a Region's data. All peers of a Region form
//    a group, each group elects a Leader to provide services.
// 3) Store: A Store is a storage/service node. Try to think it as a TiKV server
//    process. Only the store with request's Region's leader Peer could respond
//    to client's request.
type Cluster struct {
	sync.RWMutex
	id      uint64
	stores  map[uint64]*Store
	regions map[uint64]*Region
}

// NewCluster creates an empty cluster. It needs to be bootstrapped before
// providing service.
func NewCluster() *Cluster {
	return &Cluster{
		stores:  make(map[uint64]*Store),
		regions: make(map[uint64]*Region),
	}
}

// AllocID creates an unique ID in cluster. The ID could be used as either
// StoreID, RegionID, or PeerID.
func (c *Cluster) AllocID() uint64 {
	c.Lock()
	defer c.Unlock()

	return c.allocID()
}

// AllocIDs creates multiple IDs.
func (c *Cluster) AllocIDs(n int) []uint64 {
	c.Lock()
	defer c.Unlock()

	var ids []uint64
	for len(ids) < n {
		ids = append(ids, c.allocID())
	}
	return ids
}

func (c *Cluster) allocID() uint64 {
	c.id++
	return c.id
}

// GetAllRegions gets all the regions in the cluster.
func (c *Cluster) GetAllRegions() []*Region {
	regions := make([]*Region, 0, len(c.regions))
	for _, region := range c.regions {
		regions = append(regions, region)
	}
	return regions
}

// GetStore returns a Store's meta.
func (c *Cluster) GetStore(storeID uint64) *metapb.Store {
	c.RLock()
	defer c.RUnlock()

	if store := c.stores[storeID]; store != nil {
		return proto.Clone(store.meta).(*metapb.Store)
	}
	return nil
}

// StopStore stops a store with storeID.
func (c *Cluster) StopStore(storeID uint64) {
	c.Lock()
	defer c.Unlock()

	if store := c.stores[storeID]; store != nil {
		store.meta.State = metapb.StoreState_Offline
	}
}

// StartStore starts a store with storeID.
func (c *Cluster) StartStore(storeID uint64) {
	c.Lock()
	defer c.Unlock()

	if store := c.stores[storeID]; store != nil {
		store.meta.State = metapb.StoreState_Up
	}
}

// CancelStore makes the store with cancel state true.
func (c *Cluster) CancelStore(storeID uint64) {
	c.Lock()
	defer c.Unlock()

	//A store returns context.Cancelled Error when cancel is true.
	if store := c.stores[storeID]; store != nil {
		store.cancel = true
	}
}

// UnCancelStore makes the store with cancel state false.
func (c *Cluster) UnCancelStore(storeID uint64) {
	c.Lock()
	defer c.Unlock()

	if store := c.stores[storeID]; store != nil {
		store.cancel = false
	}
}

// GetStoreByAddr returns a Store's meta by an addr.
func (c *Cluster) GetStoreByAddr(addr string) *metapb.Store {
	c.RLock()
	defer c.RUnlock()

	for _, s := range c.stores {
		if s.meta.GetAddress() == addr {
			return proto.Clone(s.meta).(*metapb.Store)
		}
	}
	return nil
}

// GetAndCheckStoreByAddr checks and returns a Store's meta by an addr
func (c *Cluster) GetAndCheckStoreByAddr(addr string) (*metapb.Store, error) {
	c.RLock()
	defer c.RUnlock()

	for _, s := range c.stores {
		if s.cancel {
			return nil, context.Canceled
		}
		if s.meta.GetAddress() == addr {
			return proto.Clone(s.meta).(*metapb.Store), nil
		}
	}
	return nil, nil
}

// AddStore add a new Store to the cluster.
func (c *Cluster) AddStore(storeID uint64, addr string) {
	c.Lock()
	defer c.Unlock()

	c.stores[storeID] = newStore(storeID, addr)
}

// RemoveStore removes a Store from the cluster.
func (c *Cluster) RemoveStore(storeID uint64) {
	c.Lock()
	defer c.Unlock()

	delete(c.stores, storeID)
}

// UpdateStoreAddr updates store address for cluster.
func (c *Cluster) UpdateStoreAddr(storeID uint64, addr string) {
	c.Lock()
	defer c.Unlock()
	c.stores[storeID] = newStore(storeID, addr)
}

// GetRegion returns a Region's meta and leader ID.
func (c *Cluster) GetRegion(regionID uint64) (*metapb.Region, uint64) {
	c.RLock()
	defer c.RUnlock()

	r := c.regions[regionID]
	if r == nil {
		return nil, 0
	}
	return proto.Clone(r.Meta).(*metapb.Region), r.leader
}

// GetRegionByKey returns the Region and its leader whose range contains the key.
func (c *Cluster) GetRegionByKey(key []byte) (*metapb.Region, *metapb.Peer) {
	c.RLock()
	defer c.RUnlock()

	for _, r := range c.regions {
		if regionContains(r.Meta.StartKey, r.Meta.EndKey, key) {
			return proto.Clone(r.Meta).(*metapb.Region), proto.Clone(r.leaderPeer()).(*metapb.Peer)
		}
	}
	return nil, nil
}

// GetRegionByID returns the Region and its leader whose ID is regionID.
func (c *Cluster) GetRegionByID(regionID uint64) (*metapb.Region, *metapb.Peer) {
	c.RLock()
	defer c.RUnlock()

	for _, r := range c.regions {
		if r.Meta.GetId() == regionID {
			return proto.Clone(r.Meta).(*metapb.Region), proto.Clone(r.leaderPeer()).(*metapb.Peer)
		}
	}
	return nil, nil
}

// Bootstrap creates the first Region. The Stores should be in the Cluster before
// bootstrap.
func (c *Cluster) Bootstrap(regionID uint64, storeIDs, peerIDs []uint64, leaderPeerID uint64) {
	c.Lock()
	defer c.Unlock()

	if len(storeIDs) != len(peerIDs) {
		panic("len(storeIDs) != len(peerIDs)")
	}
	c.regions[regionID] = newRegion(regionID, storeIDs, peerIDs, leaderPeerID)
}

// AddPeer adds a new Peer for the Region on the Store.
func (c *Cluster) AddPeer(regionID, storeID, peerID uint64) {
	c.Lock()
	defer c.Unlock()

	c.regions[regionID].addPeer(peerID, storeID)
}

// RemovePeer removes the Peer from the Region. Note that if the Peer is leader,
// the Region will have no leader before calling ChangeLeader().
func (c *Cluster) RemovePeer(regionID, storeID uint64) {
	c.Lock()
	defer c.Unlock()

	c.regions[regionID].removePeer(storeID)
}

// ChangeLeader sets the Region's leader Peer. Caller should guarantee the Peer
// exists.
func (c *Cluster) ChangeLeader(regionID, leaderPeerID uint64) {
	c.Lock()
	defer c.Unlock()

	c.regions[regionID].changeLeader(leaderPeerID)
}

// GiveUpLeader sets the Region's leader to 0. The Region will have no leader
// before calling ChangeLeader().
func (c *Cluster) GiveUpLeader(regionID uint64) {
	c.ChangeLeader(regionID, 0)
}

// Split splits a Region at the key (encoded) and creates new Region.
func (c *Cluster) Split(regionID, newRegionID uint64, key []byte, peerIDs []uint64, leaderPeerID uint64) {
	c.SplitRaw(regionID, newRegionID, NewMvccKey(key), peerIDs, leaderPeerID)
}

// SplitRaw splits a Region at the key (not encoded) and creates new Region.
func (c *Cluster) SplitRaw(regionID, newRegionID uint64, rawKey []byte, peerIDs []uint64, leaderPeerID uint64) {
	c.Lock()
	defer c.Unlock()

	newRegion := c.regions[regionID].split(newRegionID, rawKey, peerIDs, leaderPeerID)
	c.regions[newRegionID] = newRegion
}

// Merge merges 2 regions, their key ranges should be adjacent.
func (c *Cluster) Merge(regionID1, regionID2 uint64) {
	c.Lock()
	defer c.Unlock()

	c.regions[regionID1].merge(c.regions[regionID2].Meta.GetEndKey())
	delete(c.regions, regionID2)
}

// SplitTable evenly splits the data in table into count regions.
// Only works for single store.
func (c *Cluster) SplitTable(mvccStore MVCCStore, tableID int64, count int) {
	tableStart := tablecodec.GenTableRecordPrefix(tableID)
	tableEnd := tableStart.PrefixNext()
	c.splitRange(mvccStore, NewMvccKey(tableStart), NewMvccKey(tableEnd), count)
}

// SplitIndex evenly splits the data in index into count regions.
// Only works for single store.
func (c *Cluster) SplitIndex(mvccStore MVCCStore, tableID, indexID int64, count int) {
	indexStart := tablecodec.EncodeTableIndexPrefix(tableID, indexID)
	indexEnd := indexStart.PrefixNext()
	c.splitRange(mvccStore, NewMvccKey(indexStart), NewMvccKey(indexEnd), count)
}

func (c *Cluster) splitRange(mvccStore MVCCStore, start, end MvccKey, count int) {
	c.Lock()
	defer c.Unlock()
	c.evacuateOldRegionRanges(start, end)
	regionPairs := c.getEntriesGroupByRegions(mvccStore, start, end, count)
	c.createNewRegions(regionPairs, start, end)
}

// getPairsGroupByRegions groups the key value pairs into splitted regions.
func (c *Cluster) getEntriesGroupByRegions(mvccStore MVCCStore, start, end MvccKey, count int) [][]Pair {
	startTS := uint64(math.MaxUint64)
	limit := int(math.MaxInt32)
	pairs := mvccStore.Scan(start.Raw(), end.Raw(), limit, startTS, kvrpcpb.IsolationLevel_SI)
	regionEntriesSlice := make([][]Pair, 0, count)
	quotient := len(pairs) / count
	remainder := len(pairs) % count
	i := 0
	for i < len(pairs) {
		regionEntryCount := quotient
		if remainder > 0 {
			remainder--
			regionEntryCount++
		}
		regionEntries := pairs[i : i+regionEntryCount]
		regionEntriesSlice = append(regionEntriesSlice, regionEntries)
		i += regionEntryCount
	}
	return regionEntriesSlice
}

func (c *Cluster) createNewRegions(regionPairs [][]Pair, start, end MvccKey) {
	for i := range regionPairs {
		peerID := c.allocID()
		newRegion := newRegion(c.allocID(), []uint64{c.firstStoreID()}, []uint64{peerID}, peerID)
		var regionStartKey, regionEndKey MvccKey
		if i == 0 {
			regionStartKey = start
		} else {
			regionStartKey = NewMvccKey(regionPairs[i][0].Key)
		}
		if i == len(regionPairs)-1 {
			regionEndKey = end
		} else {
			// Use the next region's first key as region end key.
			regionEndKey = NewMvccKey(regionPairs[i+1][0].Key)
		}
		newRegion.updateKeyRange(regionStartKey, regionEndKey)
		c.regions[newRegion.Meta.Id] = newRegion
	}
}

// evacuateOldRegionRanges evacuate the range [start, end].
// Old regions has intersection with [start, end) will be updated or deleted.
func (c *Cluster) evacuateOldRegionRanges(start, end MvccKey) {
	oldRegions := c.getRegionsCoverRange(start, end)
	for _, oldRegion := range oldRegions {
		startCmp := bytes.Compare(oldRegion.Meta.StartKey, start)
		endCmp := bytes.Compare(oldRegion.Meta.EndKey, end)
		if len(oldRegion.Meta.EndKey) == 0 {
			endCmp = 1
		}
		if startCmp >= 0 && endCmp <= 0 {
			// The region is within table data, it will be replaced by new regions.
			delete(c.regions, oldRegion.Meta.Id)
		} else if startCmp < 0 && endCmp > 0 {
			// A single Region covers table data, split into two regions that do not overlap table data.
			oldEnd := oldRegion.Meta.EndKey
			oldRegion.updateKeyRange(oldRegion.Meta.StartKey, start)
			peerID := c.allocID()
			newRegion := newRegion(c.allocID(), []uint64{c.firstStoreID()}, []uint64{peerID}, peerID)
			newRegion.updateKeyRange(end, oldEnd)
			c.regions[newRegion.Meta.Id] = newRegion
		} else if startCmp < 0 {
			oldRegion.updateKeyRange(oldRegion.Meta.StartKey, start)
		} else {
			oldRegion.updateKeyRange(end, oldRegion.Meta.EndKey)
		}
	}
}

func (c *Cluster) firstStoreID() uint64 {
	for id := range c.stores {
		return id
	}
	return 0
}

// getRegionsCoverRange gets regions in the cluster that has intersection with [start, end).
func (c *Cluster) getRegionsCoverRange(start, end MvccKey) []*Region {
	var regions []*Region
	for _, region := range c.regions {
		onRight := bytes.Compare(end, region.Meta.StartKey) <= 0
		onLeft := bytes.Compare(region.Meta.EndKey, start) <= 0
		if len(region.Meta.EndKey) == 0 {
			onLeft = false
		}
		if onLeft || onRight {
			continue
		}
		regions = append(regions, region)
	}
	return regions
}

// Region is the Region meta data.
type Region struct {
	Meta   *metapb.Region
	leader uint64
}

func newPeerMeta(peerID, storeID uint64) *metapb.Peer {
	return &metapb.Peer{
		Id:      peerID,
		StoreId: storeID,
	}
}

func newRegion(regionID uint64, storeIDs, peerIDs []uint64, leaderPeerID uint64) *Region {
	if len(storeIDs) != len(peerIDs) {
		panic("len(storeIDs) != len(peerIds)")
	}
	peers := make([]*metapb.Peer, 0, len(storeIDs))
	for i := range storeIDs {
		peers = append(peers, newPeerMeta(peerIDs[i], storeIDs[i]))
	}
	meta := &metapb.Region{
		Id:    regionID,
		Peers: peers,
	}
	return &Region{
		Meta:   meta,
		leader: leaderPeerID,
	}
}

func (r *Region) addPeer(peerID, storeID uint64) {
	r.Meta.Peers = append(r.Meta.Peers, newPeerMeta(peerID, storeID))
	r.incConfVer()
}

func (r *Region) removePeer(peerID uint64) {
	for i, peer := range r.Meta.Peers {
		if peer.GetId() == peerID {
			r.Meta.Peers = append(r.Meta.Peers[:i], r.Meta.Peers[i+1:]...)
			break
		}
	}
	if r.leader == peerID {
		r.leader = 0
	}
	r.incConfVer()
}

func (r *Region) changeLeader(leaderID uint64) {
	r.leader = leaderID
}

func (r *Region) leaderPeer() *metapb.Peer {
	for _, p := range r.Meta.Peers {
		if p.GetId() == r.leader {
			return p
		}
	}
	return nil
}

func (r *Region) split(newRegionID uint64, key MvccKey, peerIDs []uint64, leaderPeerID uint64) *Region {
	if len(r.Meta.Peers) != len(peerIDs) {
		panic("len(r.meta.Peers) != len(peerIDs)")
	}
	storeIDs := make([]uint64, 0, len(r.Meta.Peers))
	for _, peer := range r.Meta.Peers {
		storeIDs = append(storeIDs, peer.GetStoreId())
	}
	region := newRegion(newRegionID, storeIDs, peerIDs, leaderPeerID)
	region.updateKeyRange(key, r.Meta.EndKey)
	r.updateKeyRange(r.Meta.StartKey, key)
	return region
}

func (r *Region) merge(endKey MvccKey) {
	r.Meta.EndKey = endKey
	r.incVersion()
}

func (r *Region) updateKeyRange(start, end MvccKey) {
	r.Meta.StartKey = start
	r.Meta.EndKey = end
	r.incVersion()
}

func (r *Region) incConfVer() {
	r.Meta.RegionEpoch = &metapb.RegionEpoch{
		ConfVer: r.Meta.GetRegionEpoch().GetConfVer() + 1,
		Version: r.Meta.GetRegionEpoch().GetVersion(),
	}
}

func (r *Region) incVersion() {
	r.Meta.RegionEpoch = &metapb.RegionEpoch{
		ConfVer: r.Meta.GetRegionEpoch().GetConfVer(),
		Version: r.Meta.GetRegionEpoch().GetVersion() + 1,
	}
}

// Store is the Store's meta data.
type Store struct {
	meta   *metapb.Store
	cancel bool // return context.Cancelled error when cancel is true.
}

func newStore(storeID uint64, addr string) *Store {
	return &Store{
		meta: &metapb.Store{
			Id:      storeID,
			Address: addr,
		},
	}
}

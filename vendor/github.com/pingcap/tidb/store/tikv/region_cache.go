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

package tikv

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/btree"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/pd/client"
	"github.com/pingcap/tidb/metrics"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	btreeDegree             = 32
	rcDefaultRegionCacheTTL = time.Minute * 10
)

// CachedRegion encapsulates {Region, TTL}
type CachedRegion struct {
	region     *Region
	lastAccess int64
}

func (c *CachedRegion) isValid() bool {
	lastAccess := atomic.LoadInt64(&c.lastAccess)
	lastAccessTime := time.Unix(lastAccess, 0)
	return time.Since(lastAccessTime) < rcDefaultRegionCacheTTL
}

// RegionCache caches Regions loaded from PD.
type RegionCache struct {
	pdClient pd.Client

	mu struct {
		sync.RWMutex
		regions map[RegionVerID]*CachedRegion
		sorted  *btree.BTree
	}
	storeMu struct {
		sync.RWMutex
		stores map[uint64]*Store
	}
}

// NewRegionCache creates a RegionCache.
func NewRegionCache(pdClient pd.Client) *RegionCache {
	c := &RegionCache{
		pdClient: pdClient,
	}
	c.mu.regions = make(map[RegionVerID]*CachedRegion)
	c.mu.sorted = btree.New(btreeDegree)
	c.storeMu.stores = make(map[uint64]*Store)
	return c
}

// RPCContext contains data that is needed to send RPC to a region.
type RPCContext struct {
	Region RegionVerID
	Meta   *metapb.Region
	Peer   *metapb.Peer
	Addr   string
}

// GetStoreID returns StoreID.
func (c *RPCContext) GetStoreID() uint64 {
	if c.Peer != nil {
		return c.Peer.StoreId
	}
	return 0
}

// GetRPCContext returns RPCContext for a region. If it returns nil, the region
// must be out of date and already dropped from cache.
func (c *RegionCache) GetRPCContext(bo *Backoffer, id RegionVerID) (*RPCContext, error) {
	c.mu.RLock()
	region := c.getCachedRegion(id)
	if region == nil {
		c.mu.RUnlock()
		return nil, nil
	}
	// Note: it is safe to use region.meta and region.peer without clone after
	// unlock, because region cache will never update the content of region's meta
	// or peer. On the contrary, if we want to use `region` after unlock, then we
	// need to clone it to avoid data race.
	meta, peer := region.meta, region.peer
	c.mu.RUnlock()

	addr, err := c.GetStoreAddr(bo, peer.GetStoreId())
	if err != nil {
		return nil, errors.Trace(err)
	}
	if addr == "" {
		// Store not found, region must be out of date.
		c.DropRegion(id)
		return nil, nil
	}
	return &RPCContext{
		Region: id,
		Meta:   meta,
		Peer:   peer,
		Addr:   addr,
	}, nil
}

// KeyLocation is the region and range that a key is located.
type KeyLocation struct {
	Region   RegionVerID
	StartKey []byte
	EndKey   []byte
}

// Contains checks if key is in [StartKey, EndKey).
func (l *KeyLocation) Contains(key []byte) bool {
	return bytes.Compare(l.StartKey, key) <= 0 &&
		(bytes.Compare(key, l.EndKey) < 0 || len(l.EndKey) == 0)
}

// LocateKey searches for the region and range that the key is located.
func (c *RegionCache) LocateKey(bo *Backoffer, key []byte) (*KeyLocation, error) {
	c.mu.RLock()
	r := c.searchCachedRegion(key)
	if r != nil {
		loc := &KeyLocation{
			Region:   r.VerID(),
			StartKey: r.StartKey(),
			EndKey:   r.EndKey(),
		}
		c.mu.RUnlock()
		return loc, nil
	}
	c.mu.RUnlock()

	r, err := c.loadRegion(bo, key)
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.insertRegionToCache(r)

	return &KeyLocation{
		Region:   r.VerID(),
		StartKey: r.StartKey(),
		EndKey:   r.EndKey(),
	}, nil
}

// LocateRegionByID searches for the region with ID.
func (c *RegionCache) LocateRegionByID(bo *Backoffer, regionID uint64) (*KeyLocation, error) {
	c.mu.RLock()
	r := c.getRegionByIDFromCache(regionID)
	if r != nil {
		loc := &KeyLocation{
			Region:   r.VerID(),
			StartKey: r.StartKey(),
			EndKey:   r.EndKey(),
		}
		c.mu.RUnlock()
		return loc, nil
	}
	c.mu.RUnlock()

	r, err := c.loadRegionByID(bo, regionID)
	if err != nil {
		return nil, errors.Trace(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.insertRegionToCache(r)
	return &KeyLocation{
		Region:   r.VerID(),
		StartKey: r.StartKey(),
		EndKey:   r.EndKey(),
	}, nil
}

// GroupKeysByRegion separates keys into groups by their belonging Regions.
// Specially it also returns the first key's region which may be used as the
// 'PrimaryLockKey' and should be committed ahead of others.
func (c *RegionCache) GroupKeysByRegion(bo *Backoffer, keys [][]byte) (map[RegionVerID][][]byte, RegionVerID, error) {
	groups := make(map[RegionVerID][][]byte)
	var first RegionVerID
	var lastLoc *KeyLocation
	for i, k := range keys {
		if lastLoc == nil || !lastLoc.Contains(k) {
			var err error
			lastLoc, err = c.LocateKey(bo, k)
			if err != nil {
				return nil, first, errors.Trace(err)
			}
		}
		id := lastLoc.Region
		if i == 0 {
			first = id
		}
		groups[id] = append(groups[id], k)
	}
	return groups, first, nil
}

// ListRegionIDsInKeyRange lists ids of regions in [start_key,end_key].
func (c *RegionCache) ListRegionIDsInKeyRange(bo *Backoffer, startKey, endKey []byte) (regionIDs []uint64, err error) {
	for {
		curRegion, err := c.LocateKey(bo, startKey)
		if err != nil {
			return nil, errors.Trace(err)
		}
		regionIDs = append(regionIDs, curRegion.Region.id)
		if curRegion.Contains(endKey) {
			break
		}
		startKey = curRegion.EndKey
	}
	return regionIDs, nil
}

// DropRegion removes a cached Region.
func (c *RegionCache) DropRegion(id RegionVerID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dropRegionFromCache(id)
}

// UpdateLeader update some region cache with newer leader info.
func (c *RegionCache) UpdateLeader(regionID RegionVerID, leaderStoreID uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := c.getCachedRegion(regionID)
	if r == nil {
		log.Debugf("regionCache: cannot find region when updating leader %d,%d", regionID, leaderStoreID)
		return
	}

	if !r.SwitchPeer(leaderStoreID) {
		log.Debugf("regionCache: cannot find peer when updating leader %d,%d", regionID, leaderStoreID)
		c.dropRegionFromCache(r.VerID())
	}
}

// insertRegionToCache tries to insert the Region to cache.
func (c *RegionCache) insertRegionToCache(r *Region) {
	old := c.mu.sorted.ReplaceOrInsert(newBtreeItem(r))
	if old != nil {
		delete(c.mu.regions, old.(*btreeItem).region.VerID())
	}
	c.mu.regions[r.VerID()] = &CachedRegion{
		region:     r,
		lastAccess: time.Now().Unix(),
	}
}

// getCachedRegion loads a region from cache. It also checks if the region has
// not been accessed for a long time (maybe out of date). In this case, it
// returns nil so the region will be loaded from PD again.
// Note that it should be called with c.mu.RLock(), and the returned Region
// should not be used after c.mu is RUnlock().
func (c *RegionCache) getCachedRegion(id RegionVerID) *Region {
	cachedRegion, ok := c.mu.regions[id]
	if !ok {
		return nil
	}
	if cachedRegion.isValid() {
		atomic.StoreInt64(&cachedRegion.lastAccess, time.Now().Unix())
		return cachedRegion.region
	}
	return nil
}

// searchCachedRegion finds a region from cache by key. Like `getCachedRegion`,
// it should be called with c.mu.RLock(), and the returned Region should not be
// used after c.mu is RUnlock().
func (c *RegionCache) searchCachedRegion(key []byte) *Region {
	var r *Region
	c.mu.sorted.DescendLessOrEqual(newBtreeSearchItem(key), func(item btree.Item) bool {
		r = item.(*btreeItem).region
		return false
	})
	if r != nil && r.Contains(key) {
		return c.getCachedRegion(r.VerID())
	}
	return nil
}

// getRegionByIDFromCache tries to get region by regionID from cache. Like
// `getCachedRegion`, it should be called with c.mu.RLock(), and the returned
// Region should not be used after c.mu is RUnlock().
func (c *RegionCache) getRegionByIDFromCache(regionID uint64) *Region {
	for v, r := range c.mu.regions {
		if v.id == regionID {
			return r.region
		}
	}
	return nil
}

func (c *RegionCache) dropRegionFromCache(verID RegionVerID) {
	r, ok := c.mu.regions[verID]
	if !ok {
		return
	}
	metrics.TiKVRegionCacheCounter.WithLabelValues("drop_region_from_cache", metrics.RetLabel(nil)).Inc()
	c.mu.sorted.Delete(newBtreeItem(r.region))
	delete(c.mu.regions, verID)
}

// loadRegion loads region from pd client, and picks the first peer as leader.
func (c *RegionCache) loadRegion(bo *Backoffer, key []byte) (*Region, error) {
	var backoffErr error
	for {
		if backoffErr != nil {
			err := bo.Backoff(boPDRPC, backoffErr)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		meta, leader, err := c.pdClient.GetRegion(bo.ctx, key)
		metrics.TiKVRegionCacheCounter.WithLabelValues("get_region", metrics.RetLabel(err)).Inc()
		if err != nil {
			backoffErr = errors.Errorf("loadRegion from PD failed, key: %q, err: %v", key, err)
			continue
		}
		if meta == nil {
			backoffErr = errors.Errorf("region not found for key %q", key)
			continue
		}
		if len(meta.Peers) == 0 {
			return nil, errors.New("receive Region with no peer")
		}
		region := &Region{
			meta: meta,
			peer: meta.Peers[0],
		}
		if leader != nil {
			region.SwitchPeer(leader.GetStoreId())
		}
		return region, nil
	}
}

// loadRegionByID loads region from pd client, and picks the first peer as leader.
func (c *RegionCache) loadRegionByID(bo *Backoffer, regionID uint64) (*Region, error) {
	var backoffErr error
	for {
		if backoffErr != nil {
			err := bo.Backoff(boPDRPC, backoffErr)
			if err != nil {
				return nil, errors.Trace(err)
			}
		}
		meta, leader, err := c.pdClient.GetRegionByID(bo.ctx, regionID)
		metrics.TiKVRegionCacheCounter.WithLabelValues("get_region_by_id", metrics.RetLabel(err)).Inc()
		if err != nil {
			backoffErr = errors.Errorf("loadRegion from PD failed, regionID: %v, err: %v", regionID, err)
			continue
		}
		if meta == nil {
			backoffErr = errors.Errorf("region not found for regionID %q", regionID)
			continue
		}
		if len(meta.Peers) == 0 {
			return nil, errors.New("receive Region with no peer")
		}
		region := &Region{
			meta: meta,
			peer: meta.Peers[0],
		}
		if leader != nil {
			region.SwitchPeer(leader.GetStoreId())
		}
		return region, nil
	}
}

// GetStoreAddr returns a tikv server's address by its storeID. It checks cache
// first, sends request to pd server when necessary.
func (c *RegionCache) GetStoreAddr(bo *Backoffer, id uint64) (string, error) {
	c.storeMu.RLock()
	if store, ok := c.storeMu.stores[id]; ok {
		c.storeMu.RUnlock()
		return store.Addr, nil
	}
	c.storeMu.RUnlock()
	return c.ReloadStoreAddr(bo, id)
}

// ReloadStoreAddr reloads store's address.
func (c *RegionCache) ReloadStoreAddr(bo *Backoffer, id uint64) (string, error) {
	addr, err := c.loadStoreAddr(bo, id)
	if err != nil || addr == "" {
		return "", errors.Trace(err)
	}

	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	c.storeMu.stores[id] = &Store{
		ID:   id,
		Addr: addr,
	}
	return addr, nil
}

// ClearStoreByID clears store from cache with storeID.
func (c *RegionCache) ClearStoreByID(id uint64) {
	c.storeMu.Lock()
	defer c.storeMu.Unlock()
	delete(c.storeMu.stores, id)
}

func (c *RegionCache) loadStoreAddr(bo *Backoffer, id uint64) (string, error) {
	for {
		store, err := c.pdClient.GetStore(bo.ctx, id)
		metrics.TiKVRegionCacheCounter.WithLabelValues("get_store", metrics.RetLabel(err)).Inc()
		if err != nil {
			if errors.Cause(err) == context.Canceled {
				return "", errors.Trace(err)
			}
			err = errors.Errorf("loadStore from PD failed, id: %d, err: %v", id, err)
			if err = bo.Backoff(boPDRPC, err); err != nil {
				return "", errors.Trace(err)
			}
			continue
		}
		if store == nil {
			return "", nil
		}
		return store.GetAddress(), nil
	}
}

// DropStoreOnSendRequestFail is used for clearing cache when a tikv server does not respond.
func (c *RegionCache) DropStoreOnSendRequestFail(ctx *RPCContext, err error) {
	// We need to drop the store only when the request is the first one failed on this store.
	// Because too many concurrently requests trying to drop the store will be blocked on the lock.
	failedRegionID := ctx.Region
	failedStoreID := ctx.Peer.StoreId
	c.mu.Lock()
	_, ok := c.mu.regions[failedRegionID]
	if !ok {
		// The failed region is dropped already by another request, we don't need to iterate the regions
		// and find regions on the failed store to drop.
		c.mu.Unlock()
		return
	}
	for id, r := range c.mu.regions {
		if r.region.peer.GetStoreId() == failedStoreID {
			c.dropRegionFromCache(id)
		}
	}
	c.mu.Unlock()

	// Store's meta may be out of date.
	var failedStoreAddr string
	c.storeMu.Lock()
	store, ok := c.storeMu.stores[failedStoreID]
	if ok {
		failedStoreAddr = store.Addr
		delete(c.storeMu.stores, failedStoreID)
	}
	c.storeMu.Unlock()
	log.Infof("drop regions that on the store %d(%s) due to send request fail, err: %v",
		failedStoreID, failedStoreAddr, err)
}

// OnRegionStale removes the old region and inserts new regions into the cache.
func (c *RegionCache) OnRegionStale(ctx *RPCContext, newRegions []*metapb.Region) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.dropRegionFromCache(ctx.Region)

	for _, meta := range newRegions {
		if _, ok := c.pdClient.(*codecPDClient); ok {
			if err := decodeRegionMetaKey(meta); err != nil {
				return errors.Errorf("newRegion's range key is not encoded: %v, %v", meta, err)
			}
		}
		region := &Region{
			meta: meta,
			peer: meta.Peers[0],
		}
		region.SwitchPeer(ctx.Peer.GetStoreId())
		c.insertRegionToCache(region)
	}
	return nil
}

// PDClient returns the pd.Client in RegionCache.
func (c *RegionCache) PDClient() pd.Client {
	return c.pdClient
}

// btreeItem is BTree's Item that uses []byte to compare.
type btreeItem struct {
	key    []byte
	region *Region
}

func newBtreeItem(r *Region) *btreeItem {
	return &btreeItem{
		key:    r.StartKey(),
		region: r,
	}
}

func newBtreeSearchItem(key []byte) *btreeItem {
	return &btreeItem{
		key: key,
	}
}

func (item *btreeItem) Less(other btree.Item) bool {
	return bytes.Compare(item.key, other.(*btreeItem).key) < 0
}

// Region stores region's meta and its leader peer.
type Region struct {
	meta *metapb.Region
	peer *metapb.Peer
}

// GetID returns id.
func (r *Region) GetID() uint64 {
	return r.meta.GetId()
}

// RegionVerID is a unique ID that can identify a Region at a specific version.
type RegionVerID struct {
	id      uint64
	confVer uint64
	ver     uint64
}

// GetID returns the id of the region
func (r *RegionVerID) GetID() uint64 {
	return r.id
}

// VerID returns the Region's RegionVerID.
func (r *Region) VerID() RegionVerID {
	return RegionVerID{
		id:      r.meta.GetId(),
		confVer: r.meta.GetRegionEpoch().GetConfVer(),
		ver:     r.meta.GetRegionEpoch().GetVersion(),
	}
}

// StartKey returns StartKey.
func (r *Region) StartKey() []byte {
	return r.meta.StartKey
}

// EndKey returns EndKey.
func (r *Region) EndKey() []byte {
	return r.meta.EndKey
}

// GetContext constructs kvprotopb.Context from region info.
func (r *Region) GetContext() *kvrpcpb.Context {
	return &kvrpcpb.Context{
		RegionId:    r.meta.Id,
		RegionEpoch: r.meta.RegionEpoch,
		Peer:        r.peer,
	}
}

// SwitchPeer switches current peer to the one on specific store. It returns
// false if no peer matches the storeID.
func (r *Region) SwitchPeer(storeID uint64) bool {
	for _, p := range r.meta.Peers {
		if p.GetStoreId() == storeID {
			r.peer = p
			return true
		}
	}
	return false
}

// Contains checks whether the key is in the region, for the maximum region endKey is empty.
// startKey <= key < endKey.
func (r *Region) Contains(key []byte) bool {
	return bytes.Compare(r.meta.GetStartKey(), key) <= 0 &&
		(bytes.Compare(key, r.meta.GetEndKey()) < 0 || len(r.meta.GetEndKey()) == 0)
}

// Store contains a tikv server's address.
type Store struct {
	ID   uint64
	Addr string
}

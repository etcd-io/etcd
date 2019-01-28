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
	"fmt"
	"io"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/coprocessor"
	"github.com/pingcap/kvproto/pkg/errorpb"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/parser/terror"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/tikv/tikvrpc"
	"golang.org/x/net/context"
)

// For gofail injection.
var undeterminedErr = terror.ErrResultUndetermined

const requestMaxSize = 8 * 1024 * 1024

func checkGoContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func convertToKeyError(err error) *kvrpcpb.KeyError {
	if locked, ok := errors.Cause(err).(*ErrLocked); ok {
		return &kvrpcpb.KeyError{
			Locked: &kvrpcpb.LockInfo{
				Key:         locked.Key.Raw(),
				PrimaryLock: locked.Primary,
				LockVersion: locked.StartTS,
				LockTtl:     locked.TTL,
			},
		}
	}
	if retryable, ok := errors.Cause(err).(ErrRetryable); ok {
		return &kvrpcpb.KeyError{
			Retryable: retryable.Error(),
		}
	}
	return &kvrpcpb.KeyError{
		Abort: err.Error(),
	}
}

func convertToKeyErrors(errs []error) []*kvrpcpb.KeyError {
	var keyErrors = make([]*kvrpcpb.KeyError, 0)
	for _, err := range errs {
		if err != nil {
			keyErrors = append(keyErrors, convertToKeyError(err))
		}
	}
	return keyErrors
}

func convertToPbPairs(pairs []Pair) []*kvrpcpb.KvPair {
	kvPairs := make([]*kvrpcpb.KvPair, 0, len(pairs))
	for _, p := range pairs {
		var kvPair *kvrpcpb.KvPair
		if p.Err == nil {
			kvPair = &kvrpcpb.KvPair{
				Key:   p.Key,
				Value: p.Value,
			}
		} else {
			kvPair = &kvrpcpb.KvPair{
				Error: convertToKeyError(p.Err),
			}
		}
		kvPairs = append(kvPairs, kvPair)
	}
	return kvPairs
}

// rpcHandler mocks tikv's side handler behavior. In general, you may assume
// TiKV just translate the logic from Go to Rust.
type rpcHandler struct {
	cluster   *Cluster
	mvccStore MVCCStore

	// store id for current request
	storeID uint64
	// Used for handling normal request.
	startKey []byte
	endKey   []byte
	// Used for handling coprocessor request.
	rawStartKey []byte
	rawEndKey   []byte
	// Used for current request.
	isolationLevel kvrpcpb.IsolationLevel
}

func (h *rpcHandler) checkRequestContext(ctx *kvrpcpb.Context) *errorpb.Error {
	ctxPeer := ctx.GetPeer()
	if ctxPeer != nil && ctxPeer.GetStoreId() != h.storeID {
		return &errorpb.Error{
			Message:       *proto.String("store not match"),
			StoreNotMatch: &errorpb.StoreNotMatch{},
		}
	}
	region, leaderID := h.cluster.GetRegion(ctx.GetRegionId())
	// No region found.
	if region == nil {
		return &errorpb.Error{
			Message: *proto.String("region not found"),
			RegionNotFound: &errorpb.RegionNotFound{
				RegionId: *proto.Uint64(ctx.GetRegionId()),
			},
		}
	}
	var storePeer, leaderPeer *metapb.Peer
	for _, p := range region.Peers {
		if p.GetStoreId() == h.storeID {
			storePeer = p
		}
		if p.GetId() == leaderID {
			leaderPeer = p
		}
	}
	// The Store does not contain a Peer of the Region.
	if storePeer == nil {
		return &errorpb.Error{
			Message: *proto.String("region not found"),
			RegionNotFound: &errorpb.RegionNotFound{
				RegionId: *proto.Uint64(ctx.GetRegionId()),
			},
		}
	}
	// No leader.
	if leaderPeer == nil {
		return &errorpb.Error{
			Message: *proto.String("no leader"),
			NotLeader: &errorpb.NotLeader{
				RegionId: *proto.Uint64(ctx.GetRegionId()),
			},
		}
	}
	// The Peer on the Store is not leader.
	if storePeer.GetId() != leaderPeer.GetId() {
		return &errorpb.Error{
			Message: *proto.String("not leader"),
			NotLeader: &errorpb.NotLeader{
				RegionId: *proto.Uint64(ctx.GetRegionId()),
				Leader:   leaderPeer,
			},
		}
	}
	// Region epoch does not match.
	if !proto.Equal(region.GetRegionEpoch(), ctx.GetRegionEpoch()) {
		nextRegion, _ := h.cluster.GetRegionByKey(region.GetEndKey())
		newRegions := []*metapb.Region{region}
		if nextRegion != nil {
			newRegions = append(newRegions, nextRegion)
		}
		return &errorpb.Error{
			Message: *proto.String("stale epoch"),
			StaleEpoch: &errorpb.StaleEpoch{
				NewRegions: newRegions,
			},
		}
	}
	h.startKey, h.endKey = region.StartKey, region.EndKey
	h.isolationLevel = ctx.IsolationLevel
	return nil
}

func (h *rpcHandler) checkRequestSize(size int) *errorpb.Error {
	// TiKV has a limitation on raft log size.
	// mocktikv has no raft inside, so we check the request's size instead.
	if size >= requestMaxSize {
		return &errorpb.Error{
			RaftEntryTooLarge: &errorpb.RaftEntryTooLarge{},
		}
	}
	return nil
}

func (h *rpcHandler) checkRequest(ctx *kvrpcpb.Context, size int) *errorpb.Error {
	if err := h.checkRequestContext(ctx); err != nil {
		return err
	}
	return h.checkRequestSize(size)
}

func (h *rpcHandler) checkKeyInRegion(key []byte) bool {
	return regionContains(h.startKey, h.endKey, []byte(NewMvccKey(key)))
}

func (h *rpcHandler) handleKvGet(req *kvrpcpb.GetRequest) *kvrpcpb.GetResponse {
	if !h.checkKeyInRegion(req.Key) {
		panic("KvGet: key not in region")
	}

	val, err := h.mvccStore.Get(req.Key, req.GetVersion(), h.isolationLevel)
	if err != nil {
		return &kvrpcpb.GetResponse{
			Error: convertToKeyError(err),
		}
	}
	return &kvrpcpb.GetResponse{
		Value: val,
	}
}

func (h *rpcHandler) handleKvScan(req *kvrpcpb.ScanRequest) *kvrpcpb.ScanResponse {
	if !h.checkKeyInRegion(req.GetStartKey()) {
		panic("KvScan: startKey not in region")
	}
	endKey := h.endKey
	if len(req.EndKey) > 0 && (len(endKey) == 0 || bytes.Compare(req.EndKey, endKey) < 0) {
		endKey = req.EndKey
	}
	pairs := h.mvccStore.Scan(req.GetStartKey(), endKey, int(req.GetLimit()), req.GetVersion(), h.isolationLevel)
	return &kvrpcpb.ScanResponse{
		Pairs: convertToPbPairs(pairs),
	}
}

func (h *rpcHandler) handleKvPrewrite(req *kvrpcpb.PrewriteRequest) *kvrpcpb.PrewriteResponse {
	for _, m := range req.Mutations {
		if !h.checkKeyInRegion(m.Key) {
			panic("KvPrewrite: key not in region")
		}
	}
	errs := h.mvccStore.Prewrite(req.Mutations, req.PrimaryLock, req.GetStartVersion(), req.GetLockTtl())
	return &kvrpcpb.PrewriteResponse{
		Errors: convertToKeyErrors(errs),
	}
}

func (h *rpcHandler) handleKvCommit(req *kvrpcpb.CommitRequest) *kvrpcpb.CommitResponse {
	for _, k := range req.Keys {
		if !h.checkKeyInRegion(k) {
			panic("KvCommit: key not in region")
		}
	}
	var resp kvrpcpb.CommitResponse
	err := h.mvccStore.Commit(req.Keys, req.GetStartVersion(), req.GetCommitVersion())
	if err != nil {
		resp.Error = convertToKeyError(err)
	}
	return &resp
}

func (h *rpcHandler) handleKvCleanup(req *kvrpcpb.CleanupRequest) *kvrpcpb.CleanupResponse {
	if !h.checkKeyInRegion(req.Key) {
		panic("KvCleanup: key not in region")
	}
	var resp kvrpcpb.CleanupResponse
	err := h.mvccStore.Cleanup(req.Key, req.GetStartVersion())
	if err != nil {
		if commitTS, ok := errors.Cause(err).(ErrAlreadyCommitted); ok {
			resp.CommitVersion = uint64(commitTS)
		} else {
			resp.Error = convertToKeyError(err)
		}
	}
	return &resp
}

func (h *rpcHandler) handleKvBatchGet(req *kvrpcpb.BatchGetRequest) *kvrpcpb.BatchGetResponse {
	for _, k := range req.Keys {
		if !h.checkKeyInRegion(k) {
			panic("KvBatchGet: key not in region")
		}
	}
	pairs := h.mvccStore.BatchGet(req.Keys, req.GetVersion(), h.isolationLevel)
	return &kvrpcpb.BatchGetResponse{
		Pairs: convertToPbPairs(pairs),
	}
}

func (h *rpcHandler) handleMvccGetByKey(req *kvrpcpb.MvccGetByKeyRequest) *kvrpcpb.MvccGetByKeyResponse {
	debugger, ok := h.mvccStore.(MVCCDebugger)
	if !ok {
		return &kvrpcpb.MvccGetByKeyResponse{
			Error: "not implement",
		}
	}

	if !h.checkKeyInRegion(req.Key) {
		panic("MvccGetByKey: key not in region")
	}
	var resp kvrpcpb.MvccGetByKeyResponse
	resp.Info = debugger.MvccGetByKey(req.Key)
	return &resp
}

func (h *rpcHandler) handleMvccGetByStartTS(req *kvrpcpb.MvccGetByStartTsRequest) *kvrpcpb.MvccGetByStartTsResponse {
	debugger, ok := h.mvccStore.(MVCCDebugger)
	if !ok {
		return &kvrpcpb.MvccGetByStartTsResponse{
			Error: "not implement",
		}
	}
	var resp kvrpcpb.MvccGetByStartTsResponse
	resp.Info, resp.Key = debugger.MvccGetByStartTS(h.startKey, h.endKey, req.StartTs)
	return &resp
}

func (h *rpcHandler) handleKvBatchRollback(req *kvrpcpb.BatchRollbackRequest) *kvrpcpb.BatchRollbackResponse {
	err := h.mvccStore.Rollback(req.Keys, req.StartVersion)
	if err != nil {
		return &kvrpcpb.BatchRollbackResponse{
			Error: convertToKeyError(err),
		}
	}
	return &kvrpcpb.BatchRollbackResponse{}
}

func (h *rpcHandler) handleKvScanLock(req *kvrpcpb.ScanLockRequest) *kvrpcpb.ScanLockResponse {
	startKey := MvccKey(h.startKey).Raw()
	endKey := MvccKey(h.endKey).Raw()
	locks, err := h.mvccStore.ScanLock(startKey, endKey, req.GetMaxVersion())
	if err != nil {
		return &kvrpcpb.ScanLockResponse{
			Error: convertToKeyError(err),
		}
	}
	return &kvrpcpb.ScanLockResponse{
		Locks: locks,
	}
}

func (h *rpcHandler) handleKvResolveLock(req *kvrpcpb.ResolveLockRequest) *kvrpcpb.ResolveLockResponse {
	startKey := MvccKey(h.startKey).Raw()
	endKey := MvccKey(h.endKey).Raw()
	err := h.mvccStore.ResolveLock(startKey, endKey, req.GetStartVersion(), req.GetCommitVersion())
	if err != nil {
		return &kvrpcpb.ResolveLockResponse{
			Error: convertToKeyError(err),
		}
	}
	return &kvrpcpb.ResolveLockResponse{}
}

func (h *rpcHandler) handleKvDeleteRange(req *kvrpcpb.DeleteRangeRequest) *kvrpcpb.DeleteRangeResponse {
	if !h.checkKeyInRegion(req.StartKey) {
		panic("KvDeleteRange: key not in region")
	}
	var resp kvrpcpb.DeleteRangeResponse
	err := h.mvccStore.DeleteRange(req.StartKey, req.EndKey)
	if err != nil {
		resp.Error = err.Error()
	}
	return &resp
}

func (h *rpcHandler) handleKvRawGet(req *kvrpcpb.RawGetRequest) *kvrpcpb.RawGetResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawGetResponse{
			Error: "not implemented",
		}
	}
	return &kvrpcpb.RawGetResponse{
		Value: rawKV.RawGet(req.GetKey()),
	}
}

func (h *rpcHandler) handleKvRawBatchGet(req *kvrpcpb.RawBatchGetRequest) *kvrpcpb.RawBatchGetResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		// TODO should we add error ?
		return &kvrpcpb.RawBatchGetResponse{
			RegionError: &errorpb.Error{
				Message: "not implemented",
			},
		}
	}
	values := rawKV.RawBatchGet(req.Keys)
	kvPairs := make([]*kvrpcpb.KvPair, len(values))
	for i, key := range req.Keys {
		kvPairs[i] = &kvrpcpb.KvPair{
			Key:   key,
			Value: values[i],
		}
	}
	return &kvrpcpb.RawBatchGetResponse{
		Pairs: kvPairs,
	}
}

func (h *rpcHandler) handleKvRawPut(req *kvrpcpb.RawPutRequest) *kvrpcpb.RawPutResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawPutResponse{
			Error: "not implemented",
		}
	}
	rawKV.RawPut(req.GetKey(), req.GetValue())
	return &kvrpcpb.RawPutResponse{}
}

func (h *rpcHandler) handleKvRawBatchPut(req *kvrpcpb.RawBatchPutRequest) *kvrpcpb.RawBatchPutResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawBatchPutResponse{
			Error: "not implemented",
		}
	}
	keys := make([][]byte, 0, len(req.Pairs))
	values := make([][]byte, 0, len(req.Pairs))
	for _, pair := range req.Pairs {
		keys = append(keys, pair.Key)
		values = append(values, pair.Value)
	}
	rawKV.RawBatchPut(keys, values)
	return &kvrpcpb.RawBatchPutResponse{}
}

func (h *rpcHandler) handleKvRawDelete(req *kvrpcpb.RawDeleteRequest) *kvrpcpb.RawDeleteResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawDeleteResponse{
			Error: "not implemented",
		}
	}
	rawKV.RawDelete(req.GetKey())
	return &kvrpcpb.RawDeleteResponse{}
}

func (h *rpcHandler) handleKvRawBatchDelete(req *kvrpcpb.RawBatchDeleteRequest) *kvrpcpb.RawBatchDeleteResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawBatchDeleteResponse{
			Error: "not implemented",
		}
	}
	rawKV.RawBatchDelete(req.Keys)
	return &kvrpcpb.RawBatchDeleteResponse{}
}

func (h *rpcHandler) handleKvRawDeleteRange(req *kvrpcpb.RawDeleteRangeRequest) *kvrpcpb.RawDeleteRangeResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		return &kvrpcpb.RawDeleteRangeResponse{
			Error: "not implemented",
		}
	}
	rawKV.RawDeleteRange(req.GetStartKey(), req.GetEndKey())
	return &kvrpcpb.RawDeleteRangeResponse{}
}

func (h *rpcHandler) handleKvRawScan(req *kvrpcpb.RawScanRequest) *kvrpcpb.RawScanResponse {
	rawKV, ok := h.mvccStore.(RawKV)
	if !ok {
		errStr := "not implemented"
		return &kvrpcpb.RawScanResponse{
			RegionError: &errorpb.Error{
				Message: errStr,
			},
		}
	}
	pairs := rawKV.RawScan(req.GetStartKey(), h.endKey, int(req.GetLimit()))
	return &kvrpcpb.RawScanResponse{
		Kvs: convertToPbPairs(pairs),
	}
}

func (h *rpcHandler) handleSplitRegion(req *kvrpcpb.SplitRegionRequest) *kvrpcpb.SplitRegionResponse {
	key := NewMvccKey(req.GetSplitKey())
	region, _ := h.cluster.GetRegionByKey(key)
	if bytes.Equal(region.GetStartKey(), key) {
		return &kvrpcpb.SplitRegionResponse{}
	}
	newRegionID, newPeerIDs := h.cluster.AllocID(), h.cluster.AllocIDs(len(region.Peers))
	h.cluster.SplitRaw(region.GetId(), newRegionID, key, newPeerIDs, newPeerIDs[0])
	return &kvrpcpb.SplitRegionResponse{}
}

// RPCClient sends kv RPC calls to mock cluster. RPCClient mocks the behavior of
// a rpc client at tikv's side.
type RPCClient struct {
	Cluster       *Cluster
	MvccStore     MVCCStore
	streamTimeout chan *tikvrpc.Lease
}

// NewRPCClient creates an RPCClient.
// Note that close the RPCClient may close the underlying MvccStore.
func NewRPCClient(cluster *Cluster, mvccStore MVCCStore) *RPCClient {
	ch := make(chan *tikvrpc.Lease)
	go tikvrpc.CheckStreamTimeoutLoop(ch)
	return &RPCClient{
		Cluster:       cluster,
		MvccStore:     mvccStore,
		streamTimeout: ch,
	}
}

func (c *RPCClient) getAndCheckStoreByAddr(addr string) (*metapb.Store, error) {
	store, err := c.Cluster.GetAndCheckStoreByAddr(addr)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("connect fail")
	}
	if store.GetState() == metapb.StoreState_Offline ||
		store.GetState() == metapb.StoreState_Tombstone {
		return nil, errors.New("connection refused")
	}
	return store, nil
}

func (c *RPCClient) checkArgs(ctx context.Context, addr string) (*rpcHandler, error) {
	if err := checkGoContext(ctx); err != nil {
		return nil, err
	}

	store, err := c.getAndCheckStoreByAddr(addr)
	if err != nil {
		return nil, err
	}
	handler := &rpcHandler{
		cluster:   c.Cluster,
		mvccStore: c.MvccStore,
		// set store id for current request
		storeID: store.GetId(),
	}
	return handler, nil
}

// SendRequest sends a request to mock cluster.
func (c *RPCClient) SendRequest(ctx context.Context, addr string, req *tikvrpc.Request, timeout time.Duration) (*tikvrpc.Response, error) {
	// gofail: var rpcServerBusy bool
	// if rpcServerBusy {
	//	return tikvrpc.GenRegionErrorResp(req, &errorpb.Error{ServerIsBusy: &errorpb.ServerIsBusy{}})
	// }
	handler, err := c.checkArgs(ctx, addr)
	if err != nil {
		return nil, err
	}
	reqCtx := &req.Context
	resp := &tikvrpc.Response{}
	resp.Type = req.Type
	switch req.Type {
	case tikvrpc.CmdGet:
		r := req.Get
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.Get = &kvrpcpb.GetResponse{RegionError: err}
			return resp, nil
		}
		resp.Get = handler.handleKvGet(r)
	case tikvrpc.CmdScan:
		r := req.Scan
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.Scan = &kvrpcpb.ScanResponse{RegionError: err}
			return resp, nil
		}
		resp.Scan = handler.handleKvScan(r)

	case tikvrpc.CmdPrewrite:
		r := req.Prewrite
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.Prewrite = &kvrpcpb.PrewriteResponse{RegionError: err}
			return resp, nil
		}
		resp.Prewrite = handler.handleKvPrewrite(r)
	case tikvrpc.CmdCommit:
		// gofail: var rpcCommitResult string
		// switch rpcCommitResult {
		// case "timeout":
		// 	return nil, errors.New("timeout")
		// case "notLeader":
		// 	return &tikvrpc.Response{
		// 		Type:   tikvrpc.CmdCommit,
		// 		Commit: &kvrpcpb.CommitResponse{RegionError: &errorpb.Error{NotLeader: &errorpb.NotLeader{}}},
		// 	}, nil
		// case "keyError":
		// 	return &tikvrpc.Response{
		// 		Type:   tikvrpc.CmdCommit,
		// 		Commit: &kvrpcpb.CommitResponse{Error: &kvrpcpb.KeyError{}},
		// 	}, nil
		// }
		r := req.Commit
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.Commit = &kvrpcpb.CommitResponse{RegionError: err}
			return resp, nil
		}
		resp.Commit = handler.handleKvCommit(r)
		// gofail: var rpcCommitTimeout bool
		// if rpcCommitTimeout {
		//	return nil, undeterminedErr
		// }
	case tikvrpc.CmdCleanup:
		r := req.Cleanup
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.Cleanup = &kvrpcpb.CleanupResponse{RegionError: err}
			return resp, nil
		}
		resp.Cleanup = handler.handleKvCleanup(r)
	case tikvrpc.CmdBatchGet:
		r := req.BatchGet
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.BatchGet = &kvrpcpb.BatchGetResponse{RegionError: err}
			return resp, nil
		}
		resp.BatchGet = handler.handleKvBatchGet(r)
	case tikvrpc.CmdBatchRollback:
		r := req.BatchRollback
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.BatchRollback = &kvrpcpb.BatchRollbackResponse{RegionError: err}
			return resp, nil
		}
		resp.BatchRollback = handler.handleKvBatchRollback(r)
	case tikvrpc.CmdScanLock:
		r := req.ScanLock
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.ScanLock = &kvrpcpb.ScanLockResponse{RegionError: err}
			return resp, nil
		}
		resp.ScanLock = handler.handleKvScanLock(r)
	case tikvrpc.CmdResolveLock:
		r := req.ResolveLock
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.ResolveLock = &kvrpcpb.ResolveLockResponse{RegionError: err}
			return resp, nil
		}
		resp.ResolveLock = handler.handleKvResolveLock(r)
	case tikvrpc.CmdGC:
		r := req.GC
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.GC = &kvrpcpb.GCResponse{RegionError: err}
			return resp, nil
		}
		resp.GC = &kvrpcpb.GCResponse{}
	case tikvrpc.CmdDeleteRange:
		r := req.DeleteRange
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.DeleteRange = &kvrpcpb.DeleteRangeResponse{RegionError: err}
			return resp, nil
		}
		resp.DeleteRange = handler.handleKvDeleteRange(r)
	case tikvrpc.CmdRawGet:
		r := req.RawGet
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawGet = &kvrpcpb.RawGetResponse{RegionError: err}
			return resp, nil
		}
		resp.RawGet = handler.handleKvRawGet(r)
	case tikvrpc.CmdRawBatchGet:
		r := req.RawBatchGet
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawBatchGet = &kvrpcpb.RawBatchGetResponse{RegionError: err}
			return resp, nil
		}
		resp.RawBatchGet = handler.handleKvRawBatchGet(r)
	case tikvrpc.CmdRawPut:
		r := req.RawPut
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawPut = &kvrpcpb.RawPutResponse{RegionError: err}
			return resp, nil
		}
		resp.RawPut = handler.handleKvRawPut(r)
	case tikvrpc.CmdRawBatchPut:
		r := req.RawBatchPut
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawBatchPut = &kvrpcpb.RawBatchPutResponse{RegionError: err}
			return resp, nil
		}
		resp.RawBatchPut = handler.handleKvRawBatchPut(r)
	case tikvrpc.CmdRawDelete:
		r := req.RawDelete
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawDelete = &kvrpcpb.RawDeleteResponse{RegionError: err}
			return resp, nil
		}
		resp.RawDelete = handler.handleKvRawDelete(r)
	case tikvrpc.CmdRawBatchDelete:
		r := req.RawBatchDelete
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawBatchDelete = &kvrpcpb.RawBatchDeleteResponse{RegionError: err}
		}
		resp.RawBatchDelete = handler.handleKvRawBatchDelete(r)
	case tikvrpc.CmdRawDeleteRange:
		r := req.RawDeleteRange
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawDeleteRange = &kvrpcpb.RawDeleteRangeResponse{RegionError: err}
			return resp, nil
		}
		resp.RawDeleteRange = handler.handleKvRawDeleteRange(r)
	case tikvrpc.CmdRawScan:
		r := req.RawScan
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.RawScan = &kvrpcpb.RawScanResponse{RegionError: err}
			return resp, nil
		}
		resp.RawScan = handler.handleKvRawScan(r)
	case tikvrpc.CmdUnsafeDestroyRange:
		panic("unimplemented")
	case tikvrpc.CmdCop:
		r := req.Cop
		if err := handler.checkRequestContext(reqCtx); err != nil {
			resp.Cop = &coprocessor.Response{RegionError: err}
			return resp, nil
		}
		handler.rawStartKey = MvccKey(handler.startKey).Raw()
		handler.rawEndKey = MvccKey(handler.endKey).Raw()
		var res *coprocessor.Response
		switch r.GetTp() {
		case kv.ReqTypeDAG:
			res = handler.handleCopDAGRequest(r)
		case kv.ReqTypeAnalyze:
			res = handler.handleCopAnalyzeRequest(r)
		case kv.ReqTypeChecksum:
			res = handler.handleCopChecksumRequest(r)
		default:
			panic(fmt.Sprintf("unknown coprocessor request type: %v", r.GetTp()))
		}
		resp.Cop = res
	case tikvrpc.CmdCopStream:
		r := req.Cop
		if err := handler.checkRequestContext(reqCtx); err != nil {
			resp.CopStream = &tikvrpc.CopStreamResponse{
				Tikv_CoprocessorStreamClient: &mockCopStreamErrClient{Error: err},
				Response: &coprocessor.Response{
					RegionError: err,
				},
			}
			return resp, nil
		}
		handler.rawStartKey = MvccKey(handler.startKey).Raw()
		handler.rawEndKey = MvccKey(handler.endKey).Raw()
		ctx1, cancel := context.WithCancel(ctx)
		copStream, err := handler.handleCopStream(ctx1, r)
		if err != nil {
			return nil, errors.Trace(err)
		}

		streamResp := &tikvrpc.CopStreamResponse{
			Tikv_CoprocessorStreamClient: copStream,
		}
		streamResp.Lease.Cancel = cancel
		streamResp.Timeout = timeout
		c.streamTimeout <- &streamResp.Lease

		first, err := streamResp.Recv()
		if err != nil {
			return nil, errors.Trace(err)
		}
		streamResp.Response = first
		resp.CopStream = streamResp
	case tikvrpc.CmdMvccGetByKey:
		r := req.MvccGetByKey
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.MvccGetByKey = &kvrpcpb.MvccGetByKeyResponse{RegionError: err}
			return resp, nil
		}
		resp.MvccGetByKey = handler.handleMvccGetByKey(r)
	case tikvrpc.CmdMvccGetByStartTs:
		r := req.MvccGetByStartTs
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.MvccGetByStartTS = &kvrpcpb.MvccGetByStartTsResponse{RegionError: err}
			return resp, nil
		}
		resp.MvccGetByStartTS = handler.handleMvccGetByStartTS(r)
	case tikvrpc.CmdSplitRegion:
		r := req.SplitRegion
		if err := handler.checkRequest(reqCtx, r.Size()); err != nil {
			resp.SplitRegion = &kvrpcpb.SplitRegionResponse{RegionError: err}
			return resp, nil
		}
		resp.SplitRegion = handler.handleSplitRegion(r)
	default:
		return nil, errors.Errorf("unsupport this request type %v", req.Type)
	}
	return resp, nil
}

// Close closes the client.
func (c *RPCClient) Close() error {
	close(c.streamTimeout)
	if raw, ok := c.MvccStore.(io.Closer); ok {
		return raw.Close()
	}
	return nil
}

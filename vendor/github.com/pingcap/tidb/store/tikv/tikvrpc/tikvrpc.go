// Copyright 2017 PingCAP, Inc.
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

package tikvrpc

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/coprocessor"
	"github.com/pingcap/kvproto/pkg/errorpb"
	"github.com/pingcap/kvproto/pkg/kvrpcpb"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/kvproto/pkg/tikvpb"
	"golang.org/x/net/context"
)

// CmdType represents the concrete request type in Request or response type in Response.
type CmdType uint16

// CmdType values.
const (
	CmdGet CmdType = 1 + iota
	CmdScan
	CmdPrewrite
	CmdCommit
	CmdCleanup
	CmdBatchGet
	CmdBatchRollback
	CmdScanLock
	CmdResolveLock
	CmdGC
	CmdDeleteRange

	CmdRawGet CmdType = 256 + iota
	CmdRawBatchGet
	CmdRawPut
	CmdRawBatchPut
	CmdRawDelete
	CmdRawBatchDelete
	CmdRawDeleteRange
	CmdRawScan

	CmdUnsafeDestroyRange

	CmdCop CmdType = 512 + iota
	CmdCopStream

	CmdMvccGetByKey CmdType = 1024 + iota
	CmdMvccGetByStartTs
	CmdSplitRegion
)

func (t CmdType) String() string {
	switch t {
	case CmdGet:
		return "Get"
	case CmdScan:
		return "Scan"
	case CmdPrewrite:
		return "Prewrite"
	case CmdCommit:
		return "Commit"
	case CmdCleanup:
		return "Cleanup"
	case CmdBatchGet:
		return "BatchGet"
	case CmdBatchRollback:
		return "BatchRollback"
	case CmdScanLock:
		return "ScanLock"
	case CmdResolveLock:
		return "ResolveLock"
	case CmdGC:
		return "GC"
	case CmdDeleteRange:
		return "DeleteRange"
	case CmdRawGet:
		return "RawGet"
	case CmdRawBatchGet:
		return "RawBatchGet"
	case CmdRawPut:
		return "RawPut"
	case CmdRawBatchPut:
		return "RawBatchPut"
	case CmdRawDelete:
		return "RawDelete"
	case CmdRawBatchDelete:
		return "RawBatchDelete"
	case CmdRawDeleteRange:
		return "RawDeleteRange"
	case CmdRawScan:
		return "RawScan"
	case CmdUnsafeDestroyRange:
		return "UnsafeDestroyRange"
	case CmdCop:
		return "Cop"
	case CmdCopStream:
		return "CopStream"
	case CmdMvccGetByKey:
		return "MvccGetByKey"
	case CmdMvccGetByStartTs:
		return "MvccGetByStartTS"
	case CmdSplitRegion:
		return "SplitRegion"
	}
	return "Unknown"
}

// Request wraps all kv/coprocessor requests.
type Request struct {
	kvrpcpb.Context
	Type               CmdType
	Get                *kvrpcpb.GetRequest
	Scan               *kvrpcpb.ScanRequest
	Prewrite           *kvrpcpb.PrewriteRequest
	Commit             *kvrpcpb.CommitRequest
	Cleanup            *kvrpcpb.CleanupRequest
	BatchGet           *kvrpcpb.BatchGetRequest
	BatchRollback      *kvrpcpb.BatchRollbackRequest
	ScanLock           *kvrpcpb.ScanLockRequest
	ResolveLock        *kvrpcpb.ResolveLockRequest
	GC                 *kvrpcpb.GCRequest
	DeleteRange        *kvrpcpb.DeleteRangeRequest
	RawGet             *kvrpcpb.RawGetRequest
	RawBatchGet        *kvrpcpb.RawBatchGetRequest
	RawPut             *kvrpcpb.RawPutRequest
	RawBatchPut        *kvrpcpb.RawBatchPutRequest
	RawDelete          *kvrpcpb.RawDeleteRequest
	RawBatchDelete     *kvrpcpb.RawBatchDeleteRequest
	RawDeleteRange     *kvrpcpb.RawDeleteRangeRequest
	RawScan            *kvrpcpb.RawScanRequest
	UnsafeDestroyRange *kvrpcpb.UnsafeDestroyRangeRequest
	Cop                *coprocessor.Request
	MvccGetByKey       *kvrpcpb.MvccGetByKeyRequest
	MvccGetByStartTs   *kvrpcpb.MvccGetByStartTsRequest
	SplitRegion        *kvrpcpb.SplitRegionRequest
}

// Response wraps all kv/coprocessor responses.
type Response struct {
	Type               CmdType
	Get                *kvrpcpb.GetResponse
	Scan               *kvrpcpb.ScanResponse
	Prewrite           *kvrpcpb.PrewriteResponse
	Commit             *kvrpcpb.CommitResponse
	Cleanup            *kvrpcpb.CleanupResponse
	BatchGet           *kvrpcpb.BatchGetResponse
	BatchRollback      *kvrpcpb.BatchRollbackResponse
	ScanLock           *kvrpcpb.ScanLockResponse
	ResolveLock        *kvrpcpb.ResolveLockResponse
	GC                 *kvrpcpb.GCResponse
	DeleteRange        *kvrpcpb.DeleteRangeResponse
	RawGet             *kvrpcpb.RawGetResponse
	RawBatchGet        *kvrpcpb.RawBatchGetResponse
	RawPut             *kvrpcpb.RawPutResponse
	RawBatchPut        *kvrpcpb.RawBatchPutResponse
	RawDelete          *kvrpcpb.RawDeleteResponse
	RawBatchDelete     *kvrpcpb.RawBatchDeleteResponse
	RawDeleteRange     *kvrpcpb.RawDeleteRangeResponse
	RawScan            *kvrpcpb.RawScanResponse
	UnsafeDestroyRange *kvrpcpb.UnsafeDestroyRangeResponse
	Cop                *coprocessor.Response
	CopStream          *CopStreamResponse
	MvccGetByKey       *kvrpcpb.MvccGetByKeyResponse
	MvccGetByStartTS   *kvrpcpb.MvccGetByStartTsResponse
	SplitRegion        *kvrpcpb.SplitRegionResponse
}

// CopStreamResponse combinates tikvpb.Tikv_CoprocessorStreamClient and the first Recv() result together.
// In streaming API, get grpc stream client may not involve any network packet, then region error have
// to be handled in Recv() function. This struct facilitates the error handling.
type CopStreamResponse struct {
	tikvpb.Tikv_CoprocessorStreamClient
	*coprocessor.Response // The first result of Recv()
	Timeout               time.Duration
	Lease                 // Shared by this object and a background goroutine.
}

// SetContext set the Context field for the given req to the specified ctx.
func SetContext(req *Request, region *metapb.Region, peer *metapb.Peer) error {
	ctx := &req.Context
	ctx.RegionId = region.Id
	ctx.RegionEpoch = region.RegionEpoch
	ctx.Peer = peer

	switch req.Type {
	case CmdGet:
		req.Get.Context = ctx
	case CmdScan:
		req.Scan.Context = ctx
	case CmdPrewrite:
		req.Prewrite.Context = ctx
	case CmdCommit:
		req.Commit.Context = ctx
	case CmdCleanup:
		req.Cleanup.Context = ctx
	case CmdBatchGet:
		req.BatchGet.Context = ctx
	case CmdBatchRollback:
		req.BatchRollback.Context = ctx
	case CmdScanLock:
		req.ScanLock.Context = ctx
	case CmdResolveLock:
		req.ResolveLock.Context = ctx
	case CmdGC:
		req.GC.Context = ctx
	case CmdDeleteRange:
		req.DeleteRange.Context = ctx
	case CmdRawGet:
		req.RawGet.Context = ctx
	case CmdRawBatchGet:
		req.RawBatchGet.Context = ctx
	case CmdRawPut:
		req.RawPut.Context = ctx
	case CmdRawBatchPut:
		req.RawBatchPut.Context = ctx
	case CmdRawDelete:
		req.RawDelete.Context = ctx
	case CmdRawBatchDelete:
		req.RawBatchDelete.Context = ctx
	case CmdRawDeleteRange:
		req.RawDeleteRange.Context = ctx
	case CmdRawScan:
		req.RawScan.Context = ctx
	case CmdUnsafeDestroyRange:
		req.UnsafeDestroyRange.Context = ctx
	case CmdCop:
		req.Cop.Context = ctx
	case CmdCopStream:
		req.Cop.Context = ctx
	case CmdMvccGetByKey:
		req.MvccGetByKey.Context = ctx
	case CmdMvccGetByStartTs:
		req.MvccGetByStartTs.Context = ctx
	case CmdSplitRegion:
		req.SplitRegion.Context = ctx
	default:
		return fmt.Errorf("invalid request type %v", req.Type)
	}
	return nil
}

// GenRegionErrorResp returns corresponding Response with specified RegionError
// according to the given req.
func GenRegionErrorResp(req *Request, e *errorpb.Error) (*Response, error) {
	resp := &Response{}
	resp.Type = req.Type
	switch req.Type {
	case CmdGet:
		resp.Get = &kvrpcpb.GetResponse{
			RegionError: e,
		}
	case CmdScan:
		resp.Scan = &kvrpcpb.ScanResponse{
			RegionError: e,
		}
	case CmdPrewrite:
		resp.Prewrite = &kvrpcpb.PrewriteResponse{
			RegionError: e,
		}
	case CmdCommit:
		resp.Commit = &kvrpcpb.CommitResponse{
			RegionError: e,
		}
	case CmdCleanup:
		resp.Cleanup = &kvrpcpb.CleanupResponse{
			RegionError: e,
		}
	case CmdBatchGet:
		resp.BatchGet = &kvrpcpb.BatchGetResponse{
			RegionError: e,
		}
	case CmdBatchRollback:
		resp.BatchRollback = &kvrpcpb.BatchRollbackResponse{
			RegionError: e,
		}
	case CmdScanLock:
		resp.ScanLock = &kvrpcpb.ScanLockResponse{
			RegionError: e,
		}
	case CmdResolveLock:
		resp.ResolveLock = &kvrpcpb.ResolveLockResponse{
			RegionError: e,
		}
	case CmdGC:
		resp.GC = &kvrpcpb.GCResponse{
			RegionError: e,
		}
	case CmdDeleteRange:
		resp.DeleteRange = &kvrpcpb.DeleteRangeResponse{
			RegionError: e,
		}
	case CmdRawGet:
		resp.RawGet = &kvrpcpb.RawGetResponse{
			RegionError: e,
		}
	case CmdRawBatchGet:
		resp.RawBatchGet = &kvrpcpb.RawBatchGetResponse{
			RegionError: e,
		}
	case CmdRawPut:
		resp.RawPut = &kvrpcpb.RawPutResponse{
			RegionError: e,
		}
	case CmdRawBatchPut:
		resp.RawBatchPut = &kvrpcpb.RawBatchPutResponse{
			RegionError: e,
		}
	case CmdRawDelete:
		resp.RawDelete = &kvrpcpb.RawDeleteResponse{
			RegionError: e,
		}
	case CmdRawBatchDelete:
		resp.RawBatchDelete = &kvrpcpb.RawBatchDeleteResponse{
			RegionError: e,
		}
	case CmdRawDeleteRange:
		resp.RawDeleteRange = &kvrpcpb.RawDeleteRangeResponse{
			RegionError: e,
		}
	case CmdRawScan:
		resp.RawScan = &kvrpcpb.RawScanResponse{
			RegionError: e,
		}
	case CmdUnsafeDestroyRange:
		resp.UnsafeDestroyRange = &kvrpcpb.UnsafeDestroyRangeResponse{
			RegionError: e,
		}
	case CmdCop:
		resp.Cop = &coprocessor.Response{
			RegionError: e,
		}
	case CmdCopStream:
		resp.CopStream = &CopStreamResponse{
			Response: &coprocessor.Response{
				RegionError: e,
			},
		}
	case CmdMvccGetByKey:
		resp.MvccGetByKey = &kvrpcpb.MvccGetByKeyResponse{
			RegionError: e,
		}
	case CmdMvccGetByStartTs:
		resp.MvccGetByStartTS = &kvrpcpb.MvccGetByStartTsResponse{
			RegionError: e,
		}
	case CmdSplitRegion:
		resp.SplitRegion = &kvrpcpb.SplitRegionResponse{
			RegionError: e,
		}
	default:
		return nil, fmt.Errorf("invalid request type %v", req.Type)
	}
	return resp, nil
}

// GetRegionError returns the RegionError of the underlying concrete response.
func (resp *Response) GetRegionError() (*errorpb.Error, error) {
	var e *errorpb.Error
	switch resp.Type {
	case CmdGet:
		e = resp.Get.GetRegionError()
	case CmdScan:
		e = resp.Scan.GetRegionError()
	case CmdPrewrite:
		e = resp.Prewrite.GetRegionError()
	case CmdCommit:
		e = resp.Commit.GetRegionError()
	case CmdCleanup:
		e = resp.Cleanup.GetRegionError()
	case CmdBatchGet:
		e = resp.BatchGet.GetRegionError()
	case CmdBatchRollback:
		e = resp.BatchRollback.GetRegionError()
	case CmdScanLock:
		e = resp.ScanLock.GetRegionError()
	case CmdResolveLock:
		e = resp.ResolveLock.GetRegionError()
	case CmdGC:
		e = resp.GC.GetRegionError()
	case CmdDeleteRange:
		e = resp.DeleteRange.GetRegionError()
	case CmdRawGet:
		e = resp.RawGet.GetRegionError()
	case CmdRawBatchGet:
		e = resp.RawBatchGet.GetRegionError()
	case CmdRawPut:
		e = resp.RawPut.GetRegionError()
	case CmdRawBatchPut:
		e = resp.RawBatchPut.GetRegionError()
	case CmdRawDelete:
		e = resp.RawDelete.GetRegionError()
	case CmdRawBatchDelete:
		e = resp.RawBatchDelete.GetRegionError()
	case CmdRawDeleteRange:
		e = resp.RawDeleteRange.GetRegionError()
	case CmdRawScan:
		e = resp.RawScan.GetRegionError()
	case CmdUnsafeDestroyRange:
		e = resp.UnsafeDestroyRange.GetRegionError()
	case CmdCop:
		e = resp.Cop.GetRegionError()
	case CmdCopStream:
		e = resp.CopStream.Response.GetRegionError()
	case CmdMvccGetByKey:
		e = resp.MvccGetByKey.GetRegionError()
	case CmdMvccGetByStartTs:
		e = resp.MvccGetByStartTS.GetRegionError()
	case CmdSplitRegion:
		e = resp.SplitRegion.GetRegionError()
	default:
		return nil, fmt.Errorf("invalid response type %v", resp.Type)
	}
	return e, nil
}

// CallRPC launches a rpc call.
// ch is needed to implement timeout for coprocessor streaing, the stream object's
// cancel function will be sent to the channel, together with a lease checked by a background goroutine.
func CallRPC(ctx context.Context, client tikvpb.TikvClient, req *Request) (*Response, error) {
	resp := &Response{}
	resp.Type = req.Type
	var err error
	switch req.Type {
	case CmdGet:
		resp.Get, err = client.KvGet(ctx, req.Get)
	case CmdScan:
		resp.Scan, err = client.KvScan(ctx, req.Scan)
	case CmdPrewrite:
		resp.Prewrite, err = client.KvPrewrite(ctx, req.Prewrite)
	case CmdCommit:
		resp.Commit, err = client.KvCommit(ctx, req.Commit)
	case CmdCleanup:
		resp.Cleanup, err = client.KvCleanup(ctx, req.Cleanup)
	case CmdBatchGet:
		resp.BatchGet, err = client.KvBatchGet(ctx, req.BatchGet)
	case CmdBatchRollback:
		resp.BatchRollback, err = client.KvBatchRollback(ctx, req.BatchRollback)
	case CmdScanLock:
		resp.ScanLock, err = client.KvScanLock(ctx, req.ScanLock)
	case CmdResolveLock:
		resp.ResolveLock, err = client.KvResolveLock(ctx, req.ResolveLock)
	case CmdGC:
		resp.GC, err = client.KvGC(ctx, req.GC)
	case CmdDeleteRange:
		resp.DeleteRange, err = client.KvDeleteRange(ctx, req.DeleteRange)
	case CmdRawGet:
		resp.RawGet, err = client.RawGet(ctx, req.RawGet)
	case CmdRawBatchGet:
		resp.RawBatchGet, err = client.RawBatchGet(ctx, req.RawBatchGet)
	case CmdRawPut:
		resp.RawPut, err = client.RawPut(ctx, req.RawPut)
	case CmdRawBatchPut:
		resp.RawBatchPut, err = client.RawBatchPut(ctx, req.RawBatchPut)
	case CmdRawDelete:
		resp.RawDelete, err = client.RawDelete(ctx, req.RawDelete)
	case CmdRawBatchDelete:
		resp.RawBatchDelete, err = client.RawBatchDelete(ctx, req.RawBatchDelete)
	case CmdRawDeleteRange:
		resp.RawDeleteRange, err = client.RawDeleteRange(ctx, req.RawDeleteRange)
	case CmdRawScan:
		resp.RawScan, err = client.RawScan(ctx, req.RawScan)
	case CmdUnsafeDestroyRange:
		resp.UnsafeDestroyRange, err = client.UnsafeDestroyRange(ctx, req.UnsafeDestroyRange)
	case CmdCop:
		resp.Cop, err = client.Coprocessor(ctx, req.Cop)
	case CmdCopStream:
		var streamClient tikvpb.Tikv_CoprocessorStreamClient
		streamClient, err = client.CoprocessorStream(ctx, req.Cop)
		resp.CopStream = &CopStreamResponse{
			Tikv_CoprocessorStreamClient: streamClient,
		}
	case CmdMvccGetByKey:
		resp.MvccGetByKey, err = client.MvccGetByKey(ctx, req.MvccGetByKey)
	case CmdMvccGetByStartTs:
		resp.MvccGetByStartTS, err = client.MvccGetByStartTs(ctx, req.MvccGetByStartTs)
	case CmdSplitRegion:
		resp.SplitRegion, err = client.SplitRegion(ctx, req.SplitRegion)
	default:
		return nil, errors.Errorf("invalid request type: %v", req.Type)
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	return resp, nil
}

// Lease is used to implement grpc stream timeout.
type Lease struct {
	Cancel   context.CancelFunc
	deadline int64 // A time.UnixNano value, if time.Now().UnixNano() > deadline, cancel() would be called.
}

// Recv overrides the stream client Recv() function.
func (resp *CopStreamResponse) Recv() (*coprocessor.Response, error) {
	deadline := time.Now().Add(resp.Timeout).UnixNano()
	atomic.StoreInt64(&resp.Lease.deadline, deadline)

	ret, err := resp.Tikv_CoprocessorStreamClient.Recv()

	atomic.StoreInt64(&resp.Lease.deadline, 0) // Stop the lease check.
	return ret, errors.Trace(err)
}

// Close closes the CopStreamResponse object.
func (resp *CopStreamResponse) Close() {
	atomic.StoreInt64(&resp.Lease.deadline, 1)
}

// CheckStreamTimeoutLoop runs periodically to check is there any stream request timeouted.
// Lease is an object to track stream requests, call this function with "go CheckStreamTimeoutLoop()"
func CheckStreamTimeoutLoop(ch <-chan *Lease) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	array := make([]*Lease, 0, 1024)

	for {
		select {
		case item, ok := <-ch:
			if !ok {
				// This channel close means goroutine should return.
				return
			}
			array = append(array, item)
		case now := <-ticker.C:
			array = keepOnlyActive(array, now.UnixNano())
		}
	}
}

// keepOnlyActive removes completed items, call cancel function for timeout items.
func keepOnlyActive(array []*Lease, now int64) []*Lease {
	idx := 0
	for i := 0; i < len(array); i++ {
		item := array[i]
		deadline := atomic.LoadInt64(&item.deadline)
		if deadline == 0 || deadline > now {
			array[idx] = array[i]
			idx++
		} else {
			item.Cancel()
		}
	}
	return array[:idx]
}

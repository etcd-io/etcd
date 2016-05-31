// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientv3

import (
	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type (
	PutResponse    pb.PutResponse
	GetResponse    pb.RangeResponse
	DeleteResponse pb.DeleteRangeResponse
	TxnResponse    pb.TxnResponse
)

type KV interface {
	// Put puts a key-value pair into etcd.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte(0x10, 0x20)).
	Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error)

	// Get retrieves keys.
	// By default, Get will return the value for "key", if any.
	// When passed WithRange(end), Get will return the keys in the range [key, end).
	// When passed WithFromKey(), Get returns keys greater than or equal to key.
	// When passed WithRev(rev) with rev > 0, Get retrieves keys at the given revision;
	// if the required revision is compacted, the request will fail with ErrCompacted .
	// When passed WithLimit(limit), the number of returned keys is bounded by limit.
	// When passed WithSort(), the keys will be sorted.
	Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error)

	// Delete deletes a key, or optionally using WithRange(end), [key, end).
	Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error)

	// Compact compacts etcd KV history before the given rev.
	Compact(ctx context.Context, rev int64) error

	// Do applies a single Op on KV without a transaction.
	// Do is useful when declaring operations to be issued at a later time
	// whereas Get/Put/Delete are for better suited for when the operation
	// should be immediately issued at time of declaration.

	// Do applies a single Op on KV without a transaction.
	// Do is useful when creating arbitrary operations to be issued at a
	// later time; the user can range over the operations, calling Do to
	// execute them. Get/Put/Delete, on the other hand, are best suited
	// for when the operation should be issued at the time of declaration.
	Do(ctx context.Context, op Op) (OpResponse, error)

	// Txn creates a transaction.
	Txn(ctx context.Context) Txn
}

type OpResponse struct {
	put *PutResponse
	get *GetResponse
	del *DeleteResponse
}

func (op OpResponse) Put() *PutResponse    { return op.put }
func (op OpResponse) Get() *GetResponse    { return op.get }
func (op OpResponse) Del() *DeleteResponse { return op.del }

type kv struct {
	rc     *remoteClient
	remote pb.KVClient
}

func NewKV(c *Client) KV {
	ret := &kv{}
	f := func(conn *grpc.ClientConn) { ret.remote = pb.NewKVClient(conn) }
	ret.rc = newRemoteClient(c, f)
	return ret
}

func (kv *kv) Put(ctx context.Context, key, val string, opts ...OpOption) (*PutResponse, error) {
	r, err := kv.Do(ctx, OpPut(key, val, opts...))
	return r.put, rpctypes.Error(err)
}

func (kv *kv) Get(ctx context.Context, key string, opts ...OpOption) (*GetResponse, error) {
	r, err := kv.Do(ctx, OpGet(key, opts...))
	return r.get, rpctypes.Error(err)
}

func (kv *kv) Delete(ctx context.Context, key string, opts ...OpOption) (*DeleteResponse, error) {
	r, err := kv.Do(ctx, OpDelete(key, opts...))
	return r.del, rpctypes.Error(err)
}

func (kv *kv) Compact(ctx context.Context, rev int64) error {
	remote, err := kv.getRemote(ctx)
	if err != nil {
		return rpctypes.Error(err)
	}
	defer kv.rc.release()
	_, err = remote.Compact(ctx, &pb.CompactionRequest{Revision: rev})
	if err == nil {
		return nil
	}
	if isHaltErr(ctx, err) {
		return rpctypes.Error(err)
	}
	kv.rc.reconnect(err)
	return rpctypes.Error(err)
}

func (kv *kv) Txn(ctx context.Context) Txn {
	return &txn{
		kv:  kv,
		ctx: ctx,
	}
}

func (kv *kv) Do(ctx context.Context, op Op) (OpResponse, error) {
	for {
		resp, err := kv.do(ctx, op)
		if err == nil {
			return resp, nil
		}
		if isHaltErr(ctx, err) {
			return resp, rpctypes.Error(err)
		}
		// do not retry on modifications
		if op.isWrite() {
			kv.rc.reconnect(err)
			return resp, rpctypes.Error(err)
		}
		if nerr := kv.rc.reconnectWait(ctx, err); nerr != nil {
			return resp, rpctypes.Error(nerr)
		}
	}
}

func (kv *kv) do(ctx context.Context, op Op) (OpResponse, error) {
	remote, err := kv.getRemote(ctx)
	if err != nil {
		return OpResponse{}, err
	}
	defer kv.rc.release()

	switch op.t {
	// TODO: handle other ops
	case tRange:
		var resp *pb.RangeResponse
		r := &pb.RangeRequest{Key: op.key, RangeEnd: op.end, Limit: op.limit, Revision: op.rev, Serializable: op.serializable}
		if op.sort != nil {
			r.SortOrder = pb.RangeRequest_SortOrder(op.sort.Order)
			r.SortTarget = pb.RangeRequest_SortTarget(op.sort.Target)
		}

		resp, err = remote.Range(ctx, r)
		if err == nil {
			return OpResponse{get: (*GetResponse)(resp)}, nil
		}
	case tPut:
		var resp *pb.PutResponse
		r := &pb.PutRequest{Key: op.key, Value: op.val, Lease: int64(op.leaseID)}
		resp, err = remote.Put(ctx, r)
		if err == nil {
			return OpResponse{put: (*PutResponse)(resp)}, nil
		}
	case tDeleteRange:
		var resp *pb.DeleteRangeResponse
		r := &pb.DeleteRangeRequest{Key: op.key, RangeEnd: op.end}
		resp, err = remote.DeleteRange(ctx, r)
		if err == nil {
			return OpResponse{del: (*DeleteResponse)(resp)}, nil
		}
	default:
		panic("Unknown op")
	}
	return OpResponse{}, err
}

// getRemote must be followed by kv.rc.release() call.
func (kv *kv) getRemote(ctx context.Context) (pb.KVClient, error) {
	if err := kv.rc.acquire(ctx); err != nil {
		return nil, err
	}
	return kv.remote, nil
}

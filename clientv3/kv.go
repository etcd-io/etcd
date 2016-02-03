// Copyright 2015 CoreOS, Inc.
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
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
)

type (
	PutResponse         pb.PutResponse
	RangeResponse       pb.RangeResponse
	GetResponse         pb.RangeResponse
	DeleteRangeResponse pb.DeleteRangeResponse
	DeleteResponse      pb.DeleteRangeResponse
	TxnResponse         pb.TxnResponse
)

type KV interface {
	// PUT puts a key-value pair into etcd.
	// Note that key,value can be plain bytes array and string is
	// an immutable representation of that bytes array.
	// To get a string of bytes, do string([]byte(0x10, 0x20)).
	Put(key, val string, leaseID lease.LeaseID) (*PutResponse, error)

	// Range gets the keys [key, end) in the range at rev.
	// If rev <=0, range gets the keys at currentRev.
	// Limit limits the number of keys returned.
	// If the required rev is compacted, ErrCompacted will be returned.
	Range(key, end string, limit, rev int64, sort *SortOption) (*RangeResponse, error)

	// Get is like Range. A shortcut for ranging single key like [key, key+1).
	Get(key string, rev int64) (*GetResponse, error)

	// DeleteRange deletes the given range [key, end).
	DeleteRange(key, end string) (*DeleteRangeResponse, error)

	// Delete is like DeleteRange. A shortcut for deleting single key like [key, key+1).
	Delete(key string) (*DeleteResponse, error)

	// Compact compacts etcd KV history before the given rev.
	Compact(rev int64) error

	// Txn creates a transaction.
	Txn() Txn
}

type kv struct {
	c *Client

	mu     sync.Mutex       // guards all fields
	conn   *grpc.ClientConn // conn in-use
	remote pb.KVClient
}

func NewKV(c *Client) KV {
	conn := c.ActiveConnection()
	remote := pb.NewKVClient(conn)

	return &kv{
		conn:   c.ActiveConnection(),
		remote: remote,

		c: c,
	}
}

func (kv *kv) Put(key, val string, leaseID lease.LeaseID) (*PutResponse, error) {
	r, err := kv.do(OpPut(key, val, leaseID))
	if err != nil {
		return nil, err
	}
	return (*PutResponse)(r.GetResponsePut()), nil
}

func (kv *kv) Range(key, end string, limit, rev int64, sort *SortOption) (*RangeResponse, error) {
	r, err := kv.do(OpRange(key, end, limit, rev, sort))
	if err != nil {
		return nil, err
	}
	return (*RangeResponse)(r.GetResponseRange()), nil
}

func (kv *kv) Get(key string, rev int64) (*GetResponse, error) {
	r, err := kv.do(OpGet(key, rev))
	if err != nil {
		return nil, err
	}
	return (*GetResponse)(r.GetResponseRange()), nil
}

func (kv *kv) DeleteRange(key, end string) (*DeleteRangeResponse, error) {
	r, err := kv.do(OpDeleteRange(key, end))
	if err != nil {
		return nil, err
	}
	return (*DeleteRangeResponse)(r.GetResponseDeleteRange()), nil
}

func (kv *kv) Delete(key string) (*DeleteResponse, error) {
	r, err := kv.do(OpDelete(key))
	if err != nil {
		return nil, err
	}
	return (*DeleteResponse)(r.GetResponseDeleteRange()), nil
}

func (kv *kv) Compact(rev int64) error {
	r := &pb.CompactionRequest{Revision: rev}
	_, err := kv.getRemote().Compact(context.TODO(), r)
	if err == nil {
		return nil
	}

	if isRPCError(err) {
		return err
	}

	go kv.switchRemote(err)
	return nil
}

func (kv *kv) Txn() Txn {
	return &txn{
		kv: kv,
	}
}

func (kv *kv) do(op Op) (*pb.ResponseUnion, error) {
	for {
		var err error
		switch op.t {
		// TODO: handle other ops
		case tRange:
			var resp *pb.RangeResponse
			r := &pb.RangeRequest{Key: op.key, RangeEnd: op.end, Limit: op.limit, Revision: op.rev}
			if op.sort != nil {
				r.SortOrder = pb.RangeRequest_SortOrder(op.sort.Order)
				r.SortTarget = pb.RangeRequest_SortTarget(op.sort.Target)
			}

			resp, err = kv.getRemote().Range(context.TODO(), r)
			if err == nil {
				respu := &pb.ResponseUnion_ResponseRange{ResponseRange: resp}
				return &pb.ResponseUnion{Response: respu}, nil
			}
		case tPut:
			var resp *pb.PutResponse
			r := &pb.PutRequest{Key: op.key, Value: op.val, Lease: int64(op.leaseID)}
			resp, err = kv.getRemote().Put(context.TODO(), r)
			if err == nil {
				respu := &pb.ResponseUnion_ResponsePut{ResponsePut: resp}
				return &pb.ResponseUnion{Response: respu}, nil
			}
		case tDeleteRange:
			var resp *pb.DeleteRangeResponse
			r := &pb.DeleteRangeRequest{Key: op.key, RangeEnd: op.end}
			resp, err = kv.getRemote().DeleteRange(context.TODO(), r)
			if err == nil {
				respu := &pb.ResponseUnion_ResponseDeleteRange{ResponseDeleteRange: resp}
				return &pb.ResponseUnion{Response: respu}, nil
			}
		default:
			panic("Unknown op")
		}

		if isRPCError(err) {
			return nil, err
		}

		// do not retry on modifications
		if op.isWrite() {
			go kv.switchRemote(err)
			return nil, err
		}

		if nerr := kv.switchRemote(err); nerr != nil {
			return nil, nerr
		}
	}
}

func (kv *kv) switchRemote(prevErr error) error {
	newConn, err := kv.c.retryConnection(kv.conn, prevErr)
	if err != nil {
		return err
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.conn = newConn
	kv.remote = pb.NewKVClient(kv.conn)
	return nil
}

func (kv *kv) getRemote() pb.KVClient {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	return kv.remote
}

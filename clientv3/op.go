// Copyright 2016 CoreOS, Inc.
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
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
)

type opType int

const (
	// A default Op has opType 0, which is invalid.
	tRange opType = iota + 1
	tPut
	tDeleteRange
)

// Op represents an Operation that kv can execute.
type Op struct {
	t   opType
	key []byte
	end []byte

	// for range
	limit int64
	rev   int64
	sort  *SortOption

	// for put
	val     []byte
	leaseID lease.LeaseID
}

func (op Op) toRequestUnion() *pb.RequestUnion {
	switch op.t {
	case tRange:
		r := &pb.RangeRequest{Key: op.key, RangeEnd: op.end, Limit: op.limit, Revision: op.rev}
		if op.sort != nil {
			r.SortOrder = pb.RangeRequest_SortOrder(op.sort.Order)
			r.SortTarget = pb.RangeRequest_SortTarget(op.sort.Target)
		}
		return &pb.RequestUnion{Request: &pb.RequestUnion_RequestRange{RequestRange: r}}
	case tPut:
		r := &pb.PutRequest{Key: op.key, Value: op.val, Lease: int64(op.leaseID)}
		return &pb.RequestUnion{Request: &pb.RequestUnion_RequestPut{RequestPut: r}}
	case tDeleteRange:
		r := &pb.DeleteRangeRequest{Key: op.key, RangeEnd: op.end}
		return &pb.RequestUnion{Request: &pb.RequestUnion_RequestDeleteRange{RequestDeleteRange: r}}
	default:
		panic("Unknown Op")
	}
}

func (op Op) isWrite() bool {
	return op.t != tRange
}

func OpGet(key string, opts ...OpOption) Op {
	ret := Op{t: tRange, key: []byte(key)}
	for _, opt := range opts {
		opt(&ret)
	}
	return ret
}

func OpDeleteRange(key, end string) Op {
	return Op{
		t:   tDeleteRange,
		key: []byte(key),
		end: []byte(end),
	}
}

func OpDelete(key string) Op {
	return Op{
		t:   tDeleteRange,
		key: []byte(key),
	}
}

func OpPut(key, val string, leaseID lease.LeaseID) Op {
	return Op{
		t:   tPut,
		key: []byte(key),

		val:     []byte(val),
		leaseID: leaseID,
	}
}

type OpOption func(*Op)

func WithLimit(n int64) OpOption { return func(op *Op) { op.limit = n } }
func WithRev(rev int64) OpOption { return func(op *Op) { op.rev = rev } }
func WithSort(tgt SortTarget, order SortOrder) OpOption {
	return func(op *Op) {
		op.sort = &SortOption{tgt, order}
	}
}
func WithRange(endKey string) OpOption {
	return func(op *Op) { op.end = []byte(endKey) }
}

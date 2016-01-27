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

package recipe

import (
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
)

type Range struct {
	kv     pb.KVClient
	key    []byte
	rev    int64
	keyEnd []byte
}

func NewRange(client *clientv3.Client, key string) *Range {
	return NewRangeRev(client, key, 0)
}

func NewRangeRev(client *clientv3.Client, key string, rev int64) *Range {
	return &Range{client.KV, []byte(key), rev, prefixEnd(key)}
}

// Prefix performs a RangeRequest to get keys matching <key>*
func (r *Range) Prefix() (*pb.RangeResponse, error) {
	return r.kv.Range(
		context.TODO(),
		&pb.RangeRequest{
			Key:      prefixNext(string(r.key)),
			RangeEnd: r.keyEnd,
			Revision: r.rev})
}

// OpenInterval gets the keys in the set <key>* - <key>
func (r *Range) OpenInterval() (*pb.RangeResponse, error) {
	return r.kv.Range(
		context.TODO(),
		&pb.RangeRequest{Key: r.key, RangeEnd: r.keyEnd, Revision: r.rev})
}

func (r *Range) FirstKey() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_ASCEND, pb.RangeRequest_KEY)
}

func (r *Range) LastKey() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_DESCEND, pb.RangeRequest_KEY)
}

func (r *Range) FirstRev() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_ASCEND, pb.RangeRequest_MOD)
}

func (r *Range) LastRev() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_DESCEND, pb.RangeRequest_MOD)
}

func (r *Range) FirstCreate() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_ASCEND, pb.RangeRequest_MOD)
}

func (r *Range) LastCreate() (*pb.RangeResponse, error) {
	return r.topTarget(pb.RangeRequest_DESCEND, pb.RangeRequest_MOD)
}

// topTarget gets the first key for a given sort order and target
func (r *Range) topTarget(order pb.RangeRequest_SortOrder, target pb.RangeRequest_SortTarget) (*pb.RangeResponse, error) {
	return r.kv.Range(
		context.TODO(),
		&pb.RangeRequest{
			Key:        r.key,
			RangeEnd:   r.keyEnd,
			Limit:      1,
			Revision:   r.rev,
			SortOrder:  order,
			SortTarget: target})
}

// prefixNext returns the first key possibly matched by <prefix>* - <prefix>
func prefixNext(prefix string) []byte {
	return append([]byte(prefix), 0)
}

// prefixEnd returns the last key possibly matched by <prefix>*
func prefixEnd(prefix string) []byte {
	keyEnd := []byte(prefix)
	for i := len(keyEnd) - 1; i >= 0; i-- {
		if keyEnd[i] < 0xff {
			keyEnd[i] = keyEnd[i] + 1
			keyEnd = keyEnd[:i+1]
			break
		}
	}
	return keyEnd
}

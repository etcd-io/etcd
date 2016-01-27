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
	"errors"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/Godeps/_workspace/src/google.golang.org/grpc"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	spb "github.com/coreos/etcd/storage/storagepb"
)

var (
	ErrKeyExists    = errors.New("key already exists")
	ErrWaitMismatch = errors.New("unexpected wait result")
)

type EtcdClient struct {
	conn  *grpc.ClientConn
	KV    pb.KVClient
	Lease pb.LeaseClient
	Watch pb.WatchClient
}

func NewEtcdClient(conn *grpc.ClientConn) *EtcdClient {
	kv := pb.NewKVClient(conn)
	lease := pb.NewLeaseClient(conn)
	watch := pb.NewWatchClient(conn)
	return &EtcdClient{conn, kv, lease, watch}
}

// deleteRevKey deletes a key by revision, returning false if key is missing
func (ec *EtcdClient) deleteRevKey(key string, rev int64) (bool, error) {
	cmp := &pb.Compare{
		Result:      pb.Compare_EQUAL,
		Target:      pb.Compare_MOD,
		Key:         []byte(key),
		TargetUnion: &pb.Compare_ModRevision{ModRevision: rev},
	}
	req := &pb.RequestUnion{Request: &pb.RequestUnion_RequestDeleteRange{
		RequestDeleteRange: &pb.DeleteRangeRequest{Key: []byte(key)}}}
	txnresp, err := ec.KV.Txn(
		context.TODO(),
		&pb.TxnRequest{
			Compare: []*pb.Compare{cmp},
			Success: []*pb.RequestUnion{req},
			Failure: nil,
		})
	if err != nil {
		return false, err
	} else if txnresp.Succeeded == false {
		return false, nil
	}
	return true, nil
}

func (ec *EtcdClient) claimFirstKey(kvs []*spb.KeyValue) (*spb.KeyValue, error) {
	for _, kv := range kvs {
		ok, err := ec.deleteRevKey(string(kv.Key), kv.ModRevision)
		if err != nil {
			return nil, err
		} else if ok {
			return kv, nil
		}
	}
	return nil, nil
}

func putEmptyKey(kv pb.KVClient, key string) (*pb.PutResponse, error) {
	return kv.Put(context.TODO(), &pb.PutRequest{Key: []byte(key), Value: []byte{}})
}

// deletePrefix performs a RangeRequest to get keys on a given prefix
func deletePrefix(kv pb.KVClient, prefix string) (*pb.DeleteRangeResponse, error) {
	return kv.DeleteRange(
		context.TODO(),
		&pb.DeleteRangeRequest{
			Key:      []byte(prefix),
			RangeEnd: []byte(prefixEnd(prefix))})
}

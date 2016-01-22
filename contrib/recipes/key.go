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
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
)

// Key is a key/revision pair created by the client and stored on etcd
type RemoteKV struct {
	client *EtcdClient
	key    string
	rev    int64
	val    string
}

func NewKey(client *EtcdClient, key string, leaseID lease.LeaseID) (*RemoteKV, error) {
	return NewKV(client, key, "", leaseID)
}

func NewKV(client *EtcdClient, key, val string, leaseID lease.LeaseID) (*RemoteKV, error) {
	rev, err := putNewKV(client, key, val, leaseID)
	if err != nil {
		return nil, err
	}
	return &RemoteKV{client, key, rev, val}, nil
}

func GetRemoteKV(client *EtcdClient, key string) (*RemoteKV, error) {
	resp, err := client.KV.Range(
		context.TODO(),
		&pb.RangeRequest{Key: []byte(key)},
	)
	if err != nil {
		return nil, err
	}
	rev := resp.Header.Revision
	val := ""
	if len(resp.Kvs) > 0 {
		rev = resp.Kvs[0].ModRevision
		val = string(resp.Kvs[0].Value)
	}
	return &RemoteKV{
		client: client,
		key:    key,
		rev:    rev,
		val:    val}, nil
}

func NewUniqueKey(client *EtcdClient, prefix string) (*RemoteKV, error) {
	return NewUniqueKV(client, prefix, "", 0)
}

func NewUniqueKV(client *EtcdClient, prefix string, val string, leaseID lease.LeaseID) (*RemoteKV, error) {
	for {
		newKey := fmt.Sprintf("%s/%v", prefix, time.Now().UnixNano())
		rev, err := putNewKV(client, newKey, val, 0)
		if err == nil {
			return &RemoteKV{client, newKey, rev, val}, nil
		}
		if err != ErrKeyExists {
			return nil, err
		}
	}
}

// putNewKV attempts to create the given key, only succeeding if the key did
// not yet exist.
func putNewKV(ec *EtcdClient, key, val string, leaseID lease.LeaseID) (int64, error) {
	cmp := &pb.Compare{
		Result: pb.Compare_EQUAL,
		Target: pb.Compare_VERSION,
		Key:    []byte(key)}
	req := &pb.RequestUnion{
		RequestPut: &pb.PutRequest{
			Key:   []byte(key),
			Value: []byte(val),
			Lease: int64(leaseID)}}

	txnresp, err := ec.KV.Txn(
		context.TODO(),
		&pb.TxnRequest{[]*pb.Compare{cmp}, []*pb.RequestUnion{req}, nil})
	if err != nil {
		return 0, err
	}
	if txnresp.Succeeded == false {
		return 0, ErrKeyExists
	}
	return txnresp.Header.Revision, nil
}

// NewSequentialKV allocates a new sequential key-value pair at <prefix>/nnnnn
func NewSequentialKV(client *EtcdClient, prefix, val string) (*RemoteKV, error) {
	return newSequentialKV(client, prefix, val, 0)
}

// newSequentialKV allocates a new sequential key <prefix>/nnnnn with a given
// value and lease.  Note: a bookkeeping node __<prefix> is also allocated.
func newSequentialKV(client *EtcdClient, prefix, val string, leaseID lease.LeaseID) (*RemoteKV, error) {
	resp, err := NewRange(client, prefix).LastKey()
	if err != nil {
		return nil, err
	}

	// add 1 to last key, if any
	newSeqNum := 0
	if len(resp.Kvs) != 0 {
		fields := strings.Split(string(resp.Kvs[0].Key), "/")
		_, err := fmt.Sscanf(fields[len(fields)-1], "%d", &newSeqNum)
		if err != nil {
			return nil, err
		}
		newSeqNum++
	}
	newKey := fmt.Sprintf("%s/%016d", prefix, newSeqNum)

	// base prefix key must be current (i.e., <=) with the server update;
	// the base key is important to avoid the following:
	// N1: LastKey() == 1, start txn.
	// N2: New Key 2, New Key 3, Delete Key 2
	// N1: txn succeeds allocating key 2 when it shouldn't
	baseKey := []byte("__" + prefix)
	cmp := &pb.Compare{
		Result: pb.Compare_LESS,
		Target: pb.Compare_MOD,
		Key:    []byte(baseKey),
		// current revision might contain modification so +1
		ModRevision: resp.Header.Revision + 1,
	}
	prPrefix := &pb.PutRequest{Key: baseKey, Lease: int64(leaseID)}
	reqPrefix := &pb.RequestUnion{RequestPut: prPrefix}

	prNewKey := &pb.PutRequest{
		Key:   []byte(newKey),
		Value: []byte(val),
		Lease: int64(leaseID),
	}
	reqNewKey := &pb.RequestUnion{RequestPut: prNewKey}

	txnresp, err := client.KV.Txn(
		context.TODO(),
		&pb.TxnRequest{
			[]*pb.Compare{cmp},
			[]*pb.RequestUnion{reqPrefix, reqNewKey}, nil})
	if err != nil {
		return nil, err
	}
	if txnresp.Succeeded == false {
		return newSequentialKV(client, prefix, val, leaseID)
	}
	return &RemoteKV{client, newKey, txnresp.Header.Revision, val}, nil
}

func (rk *RemoteKV) Key() string     { return rk.key }
func (rk *RemoteKV) Revision() int64 { return rk.rev }
func (rk *RemoteKV) Value() string   { return rk.val }

func (rk *RemoteKV) Delete() error {
	if rk.client == nil {
		return nil
	}
	req := &pb.DeleteRangeRequest{Key: []byte(rk.key)}
	_, err := rk.client.KV.DeleteRange(context.TODO(), req)
	rk.client = nil
	return err
}

func (rk *RemoteKV) Put(val string) error {
	req := &pb.PutRequest{Key: []byte(rk.key), Value: []byte(val)}
	_, err := rk.client.KV.Put(context.TODO(), req)
	return err
}

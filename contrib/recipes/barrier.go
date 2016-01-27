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
	"github.com/coreos/etcd/storage/storagepb"
)

// Barrier creates a key in etcd to block processes, then deletes the key to
// release all blocked processes.
type Barrier struct {
	client *clientv3.Client
	key    string
}

func NewBarrier(client *clientv3.Client, key string) *Barrier {
	return &Barrier{client, key}
}

// Hold creates the barrier key causing processes to block on Wait.
func (b *Barrier) Hold() error {
	_, err := NewKey(b.client, b.key, 0)
	return err
}

// Release deletes the barrier key to unblock all waiting processes.
func (b *Barrier) Release() error {
	_, err := b.client.KV.DeleteRange(context.TODO(), &pb.DeleteRangeRequest{Key: []byte(b.key)})
	return err
}

// Wait blocks on the barrier key until it is deleted. If there is no key, Wait
// assumes Release has already been called and returns immediately.
func (b *Barrier) Wait() error {
	resp, err := NewRange(b.client, b.key).FirstKey()
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		// key already removed
		return nil
	}
	_, err = WaitEvents(
		b.client,
		b.key,
		resp.Header.Revision,
		[]storagepb.Event_EventType{storagepb.PUT, storagepb.DELETE})
	return err
}

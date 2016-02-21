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
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/storage/storagepb"
)

// Mutex implements the sync Locker interface with etcd
type Mutex struct {
	client *v3.Client
	kv     v3.KV
	ctx    context.Context

	key   string
	myKey *EphemeralKV
}

func NewMutex(client *v3.Client, key string) *Mutex {
	return &Mutex{client, v3.NewKV(client), context.TODO(), key, nil}
}

func (m *Mutex) Lock() (err error) {
	// put self in lock waiters via myKey; oldest waiter holds lock
	m.myKey, err = NewUniqueEphemeralKey(m.client, m.key)
	if err != nil {
		return err
	}
	// find oldest element in waiters via revision of insertion
	resp, err := m.kv.Get(m.ctx, m.key, withFirstRev()...)
	if err != nil {
		return err
	}
	// if myKey is oldest in waiters, then myKey holds the lock
	if m.myKey.Revision() == resp.Kvs[0].CreateRevision {
		return nil
	}
	// otherwise myKey isn't lowest, so there must be a key prior to myKey
	opts := append(withLastRev(), v3.WithRev(m.myKey.Revision()-1))
	lastKey, err := m.kv.Get(m.ctx, m.key, opts...)
	if err != nil {
		return err
	}
	// wait for release on prior key
	_, err = WaitEvents(
		m.client,
		string(lastKey.Kvs[0].Key),
		m.myKey.Revision()-1,
		[]storagepb.Event_EventType{storagepb.DELETE})
	// myKey now oldest
	return err
}

func (m *Mutex) Unlock() error {
	err := m.myKey.Delete()
	m.myKey = nil
	return err
}

type lockerMutex struct{ *Mutex }

func (lm *lockerMutex) Lock() {
	if err := lm.Mutex.Lock(); err != nil {
		panic(err)
	}
}
func (lm *lockerMutex) Unlock() {
	if err := lm.Mutex.Unlock(); err != nil {
		panic(err)
	}
}

func NewLocker(client *v3.Client, key string) sync.Locker {
	return &lockerMutex{NewMutex(client, key)}
}

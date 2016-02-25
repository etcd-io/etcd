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

package concurrency

import (
	"sync"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	v3 "github.com/coreos/etcd/clientv3"
)

// Mutex implements the sync Locker interface with etcd
type Mutex struct {
	client *v3.Client
	kv     v3.KV
	ctx    context.Context

	pfx   string
	myKey string
	myRev int64
}

func NewMutex(ctx context.Context, client *v3.Client, pfx string) *Mutex {
	return &Mutex{client, v3.NewKV(client), ctx, pfx, "", -1}
}

// Lock locks the mutex with a cancellable context. If the context is cancelled
// while trying to acquire the lock, the mutex tries to clean its stale lock entry.
func (m *Mutex) Lock(ctx context.Context) error {
	s, err := NewSession(m.client)
	if err != nil {
		return err
	}
	// put self in lock waiters via myKey; oldest waiter holds lock
	m.myKey, m.myRev, err = NewUniqueKey(ctx, m.kv, m.pfx, v3.WithLease(s.Lease()))
	// wait for lock to become available
	for err == nil {
		// find oldest element in waiters via revision of insertion
		var resp *v3.GetResponse
		resp, err = m.kv.Get(ctx, m.pfx, v3.WithFirstRev()...)
		if err != nil {
			break
		}
		if m.myRev == resp.Kvs[0].CreateRevision {
			// myKey is oldest in waiters; myKey holds the lock now
			return nil
		}
		// otherwise myKey isn't lowest, so there must be a pfx prior to myKey
		opts := append(v3.WithLastRev(), v3.WithRev(m.myRev-1))
		resp, err = m.kv.Get(ctx, m.pfx, opts...)
		if err != nil {
			break
		}
		lastKey := string(resp.Kvs[0].Key)
		// wait for release on prior pfx
		err = waitUpdate(ctx, m.client, lastKey, v3.WithRev(m.myRev))
		// try again in case lastKey left the wait list before acquiring the lock;
		// myKey can only hold the lock if it's the oldest in the list
	}

	// release lock key if cancelled
	select {
	case <-ctx.Done():
		m.Unlock()
	default:
	}
	return err
}

func (m *Mutex) Unlock() error {
	if _, err := m.kv.Delete(m.ctx, m.myKey); err != nil {
		return err
	}
	m.myKey = "\x00"
	m.myRev = -1
	return nil
}

func (m *Mutex) IsOwner() v3.Cmp {
	return v3.Compare(v3.CreatedRevision(m.myKey), "=", m.myRev)
}

func (m *Mutex) Key() string { return m.myKey }

type lockerMutex struct{ *Mutex }

func (lm *lockerMutex) Lock() {
	if err := lm.Mutex.Lock(lm.ctx); err != nil {
		panic(err)
	}
}
func (lm *lockerMutex) Unlock() {
	if err := lm.Mutex.Unlock(); err != nil {
		panic(err)
	}
}

// NewLocker creates a sync.Locker backed by an etcd mutex.
func NewLocker(ctx context.Context, client *v3.Client, pfx string) sync.Locker {
	return &lockerMutex{NewMutex(ctx, client, pfx)}
}

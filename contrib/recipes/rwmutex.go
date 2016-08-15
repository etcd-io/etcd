// Copyright 2016 The etcd Authors
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
	v3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"golang.org/x/net/context"
)

type RWMutex struct {
	s   *concurrency.Session
	ctx context.Context

	key   string
	myKey *EphemeralKV
}

func NewRWMutex(s *concurrency.Session, key string) *RWMutex {
	return &RWMutex{s, context.TODO(), key, nil}
}

func (rwm *RWMutex) RLock() error {
	client := rwm.s.Client()

	rk, err := NewUniqueEphemeralKey(rwm.s, rwm.key+"/read")
	if err != nil {
		return err
	}
	rwm.myKey = rk

	// if there are nodes with "write-" and a lower
	// revision number than us we must wait
	resp, err := client.Get(rwm.ctx, rwm.key+"/write", v3.WithFirstRev()...)
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 || resp.Kvs[0].ModRevision > rk.Revision() {
		// no blocking since no write key
		return nil
	}
	return rwm.waitOnLowest()
}

func (rwm *RWMutex) Lock() error {
	client := rwm.s.Client()

	rk, err := NewUniqueEphemeralKey(rwm.s, rwm.key+"/write")
	if err != nil {
		return err
	}
	rwm.myKey = rk

	for {
		// find any key of lower rev number blocks the write lock
		opts := append(v3.WithLastRev(), v3.WithRev(rk.Revision()-1))
		resp, err := client.Get(rwm.ctx, rwm.key, opts...)
		if err != nil {
			return err
		}
		if len(resp.Kvs) == 0 {
			// no matching for revision before myKey; acquired
			break
		}
		if err := rwm.waitOnLowest(); err != nil {
			return err
		}
		//  get the new lowest, etc until this is the only one left
	}

	return nil
}

func (rwm *RWMutex) waitOnLowest() error {
	client := rwm.s.Client()

	// must block; get key before ek for waiting
	opts := append(v3.WithLastRev(), v3.WithRev(rwm.myKey.Revision()-1))
	lastKey, err := client.Get(rwm.ctx, rwm.key, opts...)
	if err != nil {
		return err
	}
	// wait for release on prior key
	_, err = WaitEvents(
		client,
		string(lastKey.Kvs[0].Key),
		rwm.myKey.Revision(),
		[]mvccpb.Event_EventType{mvccpb.DELETE})
	return err
}

func (rwm *RWMutex) RUnlock() error { return rwm.myKey.Delete() }
func (rwm *RWMutex) Unlock() error  { return rwm.myKey.Delete() }

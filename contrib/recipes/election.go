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
	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/etcdserver"
	"github.com/coreos/etcd/storage/storagepb"
)

type Election struct {
	client    *clientv3.Client
	keyPrefix string
	leaderKey *EphemeralKV
}

// NewElection returns a new election on a given key prefix.
func NewElection(client *clientv3.Client, keyPrefix string) *Election {
	return &Election{client, keyPrefix, nil}
}

// Volunteer puts a value as eligible for the election. It blocks until
// it is elected or an error occurs (cannot withdraw candidacy)
func (e *Election) Volunteer(val string) error {
	if e.leaderKey != nil {
		return e.leaderKey.Put(val)
	}
	myKey, err := NewUniqueEphemeralKV(e.client, e.keyPrefix, val)
	if err != nil {
		return err
	}
	if err = e.waitLeadership(myKey); err != nil {
		return err
	}
	e.leaderKey = myKey
	return nil
}

// Resign lets a leader start a new election.
func (e *Election) Resign() (err error) {
	if e.leaderKey != nil {
		err = e.leaderKey.Delete()
		e.leaderKey = nil
	}
	return err
}

// Leader returns the leader value for the current election.
func (e *Election) Leader() (string, error) {
	resp, err := NewRange(e.client, e.keyPrefix).FirstCreate()
	if err != nil {
		return "", err
	} else if len(resp.Kvs) == 0 {
		// no leader currently elected
		return "", etcdserver.ErrNoLeader
	}
	return string(resp.Kvs[0].Value), nil
}

// Wait waits for a leader to be elected, returning the leader value.
func (e *Election) Wait() (string, error) {
	resp, err := NewRange(e.client, e.keyPrefix).FirstCreate()
	if err != nil {
		return "", err
	} else if len(resp.Kvs) != 0 {
		// leader already exists
		return string(resp.Kvs[0].Value), nil
	}
	_, err = WaitPrefixEvents(
		e.client,
		e.keyPrefix,
		resp.Header.Revision,
		[]storagepb.Event_EventType{storagepb.PUT})
	if err != nil {
		return "", err
	}
	return e.Wait()
}

func (e *Election) waitLeadership(tryKey *EphemeralKV) error {
	resp, err := NewRangeRev(
		e.client,
		e.keyPrefix,
		tryKey.Revision()-1).LastCreate()
	if err != nil {
		return err
	} else if len(resp.Kvs) == 0 {
		// nothing before tryKey => have leadership
		return nil
	}
	_, err = WaitEvents(
		e.client,
		string(resp.Kvs[0].Key),
		tryKey.Revision(),
		[]storagepb.Event_EventType{storagepb.DELETE})
	return err
}

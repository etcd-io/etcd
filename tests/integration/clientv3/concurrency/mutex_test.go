// Copyright 2019 The etcd Authors
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

package concurrency_test

import (
	"context"
	"testing"

	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.etcd.io/etcd/tests/v3/integration"
)

func TestMutexLockSessionExpired(t *testing.T) {
	cli, err := integration.NewClient(t, clientv3.Config{Endpoints: exampleEndpoints()})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	// create two separate sessions for lock competition
	s1, err := concurrency.NewSession(cli)
	if err != nil {
		t.Fatal(err)
	}
	defer s1.Close()
	m1 := concurrency.NewMutex(s1, "/my-lock/")

	s2, err := concurrency.NewSession(cli)
	if err != nil {
		t.Fatal(err)
	}
	m2 := concurrency.NewMutex(s2, "/my-lock/")

	// acquire lock for s1
	if err := m1.Lock(context.TODO()); err != nil {
		t.Fatal(err)
	}

	m2Locked := make(chan struct{})
	var err2 error
	go func() {
		defer close(m2Locked)
		// m2 blocks since m1 already acquired lock /my-lock/
		if err2 = m2.Lock(context.TODO()); err2 == nil {
			t.Error("expect session expired error")
		}
	}()

	// revoke the session of m2 before unlock m1
	err = s2.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := m1.Unlock(context.TODO()); err != nil {
		t.Fatal(err)
	}

	<-m2Locked
}

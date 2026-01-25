// Copyright 2026 The etcd Authors
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

package etcdserver

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/types"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/cindex"
	serverstorage "go.etcd.io/etcd/server/v3/storage"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/raft/v3/raftpb"
)

func TestConfStatePersistenceOnApplyConfChange(t *testing.T) {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)

	cl := membership.NewCluster(lg)
	cl.SetBackend(schema.NewMembershipBackend(lg, be))

	cl.AddMember(&membership.Member{ID: types.ID(1)}, true)

	schema.CreateMetaBucket(be.BatchTx())

	ci := cindex.NewFakeConsistentIndex(0)
	beHooks := serverstorage.NewBackendHooks(lg, ci)

	n := newNodeRecorder()
	r := newRaftNode(raftNodeConfig{
		lg:        lg,
		Node:      n,
		transport: newNopTransporter(),
	})

	srv := &EtcdServer{
		lgMu:         new(sync.RWMutex),
		lg:           lg,
		memberID:     1,
		r:            *r,
		cluster:      cl,
		beHooks:      beHooks,
		be:           be,
		consistIndex: ci,
	}

	now := time.Now()
	urls, err := types.NewURLs([]string{"http://127.0.0.1:2380"})
	require.NoError(t, err)
	m := membership.NewMember("test", urls, "", &now)
	m.ID = types.ID(2)

	memberData, err := json.Marshal(&membership.ConfigChangeContext{Member: *m})
	require.NoError(t, err)

	cc := raftpb.ConfChange{
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  uint64(m.ID),
		Context: memberData,
	}

	confState := raftpb.ConfState{}
	_, err = srv.applyConfChange(cc, &confState, membership.ApplyBoth)
	require.NoError(t, err)

	tx := be.BatchTx()
	tx.Lock()
	persistedConfState := schema.UnsafeConfStateFromBackend(lg, tx)
	tx.Unlock()

	require.NotNil(t, persistedConfState)
}

func TestConfStateConsistencyAfterApplyConfChange(t *testing.T) {
	lg := zaptest.NewLogger(t)
	be, _ := betesting.NewDefaultTmpBackend(t)
	defer betesting.Close(t, be)

	cl := membership.NewCluster(lg)
	cl.SetBackend(schema.NewMembershipBackend(lg, be))

	for i := 1; i <= 3; i++ {
		cl.AddMember(&membership.Member{ID: types.ID(i)}, true)
	}

	schema.CreateMetaBucket(be.BatchTx())

	ci := cindex.NewFakeConsistentIndex(0)
	beHooks := serverstorage.NewBackendHooks(lg, ci)

	n := newNodeRecorder()
	r := newRaftNode(raftNodeConfig{
		lg:        lg,
		Node:      n,
		transport: newNopTransporter(),
	})

	srv := &EtcdServer{
		lgMu:         new(sync.RWMutex),
		lg:           lg,
		memberID:     1,
		r:            *r,
		cluster:      cl,
		beHooks:      beHooks,
		be:           be,
		consistIndex: ci,
	}

	cc := raftpb.ConfChange{
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: 3,
	}

	confState := raftpb.ConfState{Voters: []uint64{1, 2, 3}}
	_, err := srv.applyConfChange(cc, &confState, membership.ApplyBoth)
	require.NoError(t, err)

	tx := be.BatchTx()
	tx.Lock()
	persistedConfState := schema.UnsafeConfStateFromBackend(lg, tx)
	tx.Unlock()

	require.NotNil(t, persistedConfState)
	assert.ElementsMatch(t, confState.Voters, persistedConfState.Voters)
}

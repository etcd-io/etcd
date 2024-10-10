// Copyright 2023 The etcd Authors
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

package options

import (
	"math/rand"
	"testing"

	"go.etcd.io/etcd/server/v3/embed"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func mockRand(source rand.Source) func() {
	tmp := internalRand
	internalRand = rand.New(source)
	return func() {
		internalRand = tmp
	}
}

func TestWithClusterOptionGroups(t *testing.T) {
	restore := mockRand(rand.NewSource(1))
	defer restore()
	tickOptions1 := ClusterOptions{WithTickMs(101), WithElectionMs(1001)}
	tickOptions2 := ClusterOptions{WithTickMs(202), WithElectionMs(2002)}
	tickOptions3 := ClusterOptions{WithTickMs(303), WithElectionMs(3003)}
	opts := ClusterOptions{
		WithSnapshotCount(100, 150, 200),
		WithClusterOptionGroups(tickOptions1, tickOptions2, tickOptions3),
		WithSnapshotCatchUpEntries(100),
	}

	expectedServerConfigs := []embed.Config{
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 101, ElectionMs: 1001},
		{SnapshotCount: 100, SnapshotCatchUpEntries: 100, TickMs: 202, ElectionMs: 2002},
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 202, ElectionMs: 2002},
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 101, ElectionMs: 1001},
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 101, ElectionMs: 1001},
		{SnapshotCount: 150, SnapshotCatchUpEntries: 100, TickMs: 202, ElectionMs: 2002},
	}
	for i, tt := range expectedServerConfigs {
		cluster := *e2e.NewConfig(opts...)
		if cluster.ServerConfig.SnapshotCount != tt.SnapshotCount {
			t.Errorf("Test case %d: SnapshotCount = %v, want %v\n", i, cluster.ServerConfig.SnapshotCount, tt.SnapshotCount)
		}
		if cluster.ServerConfig.SnapshotCatchUpEntries != tt.SnapshotCatchUpEntries {
			t.Errorf("Test case %d: SnapshotCatchUpEntries = %v, want %v\n", i, cluster.ServerConfig.SnapshotCatchUpEntries, tt.SnapshotCatchUpEntries)
		}
		if cluster.ServerConfig.TickMs != tt.TickMs {
			t.Errorf("Test case %d: TickMs = %v, want %v\n", i, cluster.ServerConfig.TickMs, tt.TickMs)
		}
		if cluster.ServerConfig.ElectionMs != tt.ElectionMs {
			t.Errorf("Test case %d: ElectionMs = %v, want %v\n", i, cluster.ServerConfig.ElectionMs, tt.ElectionMs)
		}
	}
}

func TestWithOptionsSubset(t *testing.T) {
	restore := mockRand(rand.NewSource(1))
	defer restore()
	tickOptions := ClusterOptions{WithTickMs(50), WithElectionMs(500)}
	opts := ClusterOptions{
		WithSnapshotCatchUpEntries(100),
		WithSubsetOptions(WithSnapshotCount(100, 150, 200), WithClusterOptionGroups(tickOptions)),
	}

	expectedServerConfigs := []embed.Config{
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 100, ElectionMs: 1000},
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 100, ElectionMs: 1000},
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 100, ElectionMs: 1000},
		// both SnapshotCount and TickMs&ElectionMs are not default values.
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 50, ElectionMs: 500},
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 100, ElectionMs: 1000},
		// only TickMs&ElectionMs are not default values.
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 50, ElectionMs: 500},
		// both SnapshotCount and TickMs&ElectionMs are not default values.
		{SnapshotCount: 200, SnapshotCatchUpEntries: 100, TickMs: 50, ElectionMs: 500},
		// both SnapshotCount and TickMs&ElectionMs are not default values.
		{SnapshotCount: 10000, SnapshotCatchUpEntries: 100, TickMs: 50, ElectionMs: 500},
		// only SnapshotCount is not default value.
		{SnapshotCount: 100, SnapshotCatchUpEntries: 100, TickMs: 100, ElectionMs: 1000},
	}
	for i, tt := range expectedServerConfigs {
		cluster := *e2e.NewConfig(opts...)
		if cluster.ServerConfig.SnapshotCount != tt.SnapshotCount {
			t.Errorf("Test case %d: SnapshotCount = %v, want %v\n", i, cluster.ServerConfig.SnapshotCount, tt.SnapshotCount)
		}
		if cluster.ServerConfig.SnapshotCatchUpEntries != tt.SnapshotCatchUpEntries {
			t.Errorf("Test case %d: SnapshotCatchUpEntries = %v, want %v\n", i, cluster.ServerConfig.SnapshotCatchUpEntries, tt.SnapshotCatchUpEntries)
		}
		if cluster.ServerConfig.TickMs != tt.TickMs {
			t.Errorf("Test case %d: TickMs = %v, want %v\n", i, cluster.ServerConfig.TickMs, tt.TickMs)
		}
		if cluster.ServerConfig.ElectionMs != tt.ElectionMs {
			t.Errorf("Test case %d: ElectionMs = %v, want %v\n", i, cluster.ServerConfig.ElectionMs, tt.ElectionMs)
		}
	}
}

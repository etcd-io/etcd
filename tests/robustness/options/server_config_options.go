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
	"time"

	e2e "go.etcd.io/etcd/tests/v3/framework/e2e"
)

func WithSnapshotCount(input ...uint64) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.SnapshotCount = input[internalRand.Intn(len(input))]
	}
}

func WithExperimentalCompactionBatchLimit(input ...int) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.ExperimentalCompactionBatchLimit = input[internalRand.Intn(len(input))]
	}
}

func WithSnapshotCatchUpEntries(input ...uint64) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.SnapshotCatchUpEntries = input[internalRand.Intn(len(input))]
	}
}

func WithTickMs(input ...uint) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.TickMs = input[internalRand.Intn(len(input))]
	}
}

func WithElectionMs(input ...uint) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.ElectionMs = input[internalRand.Intn(len(input))]
	}
}

func WithExperimentalWatchProgressNotifyInterval(input ...time.Duration) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) {
		c.ServerConfig.ExperimentalWatchProgressNotifyInterval = input[internalRand.Intn(len(input))]
	}
}

func WithVersion(input ...e2e.ClusterVersion) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) { c.Version = input[internalRand.Intn(len(input))] }
}

func WithInitialLeaderIndex(input ...int) e2e.EPClusterOption {
	return func(c *e2e.EtcdProcessClusterConfig) { c.InitialLeaderIndex = input[internalRand.Intn(len(input))] }
}

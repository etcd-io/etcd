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

// +build !cluster_proxy

package integration

import (
	"go.etcd.io/etcd/v3/clientv3"
	"go.etcd.io/etcd/v3/etcdserver/api/v3election/v3electionpb"
	"go.etcd.io/etcd/v3/etcdserver/api/v3lock/v3lockpb"
	pb "go.etcd.io/etcd/v3/etcdserver/etcdserverpb"
)

const ThroughProxy = false

func toGRPC(c *clientv3.Client) grpcAPI {
	return grpcAPI{
		pb.NewClusterClient(c.ActiveConnection()),
		pb.NewKVClient(c.ActiveConnection()),
		pb.NewLeaseClient(c.ActiveConnection()),
		pb.NewWatchClient(c.ActiveConnection()),
		pb.NewMaintenanceClient(c.ActiveConnection()),
		pb.NewAuthClient(c.ActiveConnection()),
		v3lockpb.NewLockClient(c.ActiveConnection()),
		v3electionpb.NewElectionClient(c.ActiveConnection()),
	}
}

func newClientV3(cfg clientv3.Config) (*clientv3.Client, error) {
	return clientv3.New(cfg)
}

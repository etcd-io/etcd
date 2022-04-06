// Copyright 2022 The etcd Authors
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

package framework

import (
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/config"
)

type testRunner interface {
	TestMain(m *testing.M)
	BeforeTest(testing.TB)
	NewCluster(testing.TB, config.ClusterConfig) Cluster
}

type Cluster interface {
	Members() []Member
	Client() Client
	Close() error
}

type Member interface {
	Client() Client
	Start() error
	Stop()
}

type Client interface {
	Put(key, value string, opts config.PutOptions) error
	Get(key string, opts config.GetOptions) (*clientv3.GetResponse, error)
	Delete(key string, opts config.DeleteOptions) (*clientv3.DeleteResponse, error)
	Compact(rev int64, opts config.CompactOption) (*clientv3.CompactResponse, error)
	Status() ([]*clientv3.StatusResponse, error)
	HashKV(rev int64) ([]*clientv3.HashKVResponse, error)
	Health() error
	Defragment(opts config.DefragOption) error
	AlarmList() (*clientv3.AlarmResponse, error)
	AlarmDisarm(alarmMember *clientv3.AlarmMember) (*clientv3.AlarmResponse, error)
	Grant(ttl int64) (*clientv3.LeaseGrantResponse, error)
	TimeToLive(id clientv3.LeaseID, opts config.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error)
	LeaseList() (*clientv3.LeaseLeasesResponse, error)
	LeaseKeepAliveOnce(id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error)
	LeaseRevoke(id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)

	UserAdd(name, password string, opts config.UserAddOptions) (*clientv3.AuthUserAddResponse, error)
	UserList() (*clientv3.AuthUserListResponse, error)
	UserDelete(name string) (*clientv3.AuthUserDeleteResponse, error)
	UserChangePass(user, newPass string) error
}

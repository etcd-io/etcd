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

package command

import (
	"context"

	"github.com/spf13/cobra"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// commandClient captures the subset of clientv3.Client used by unit-testable
// command handlers so tests can inject a fake client without starting a cluster.
type commandClient interface {
	Close() error

	Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error)
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	GetStream(ctx context.Context, key string, opts ...clientv3.OpOption) (clientv3.GetStreamChan, error)
	Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error)
	Txn(ctx context.Context) clientv3.Txn

	Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error)
	Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	TimeToLive(ctx context.Context, id clientv3.LeaseID, opts ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error)
	Leases(ctx context.Context) (*clientv3.LeaseLeasesResponse, error)
	KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	KeepAliveOnce(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error)

	MemberAdd(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)
	MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error)
	MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error)
	MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*clientv3.MemberUpdateResponse, error)
	MemberList(ctx context.Context, opts ...clientv3.OpOption) (*clientv3.MemberListResponse, error)
	MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error)

	AlarmList(ctx context.Context) (*clientv3.AlarmResponse, error)
	AlarmDisarm(ctx context.Context, m *clientv3.AlarmMember) (*clientv3.AlarmResponse, error)
	Downgrade(ctx context.Context, action clientv3.DowngradeAction, version string) (*clientv3.DowngradeResponse, error)

	AuthEnable(ctx context.Context) (*clientv3.AuthEnableResponse, error)
	AuthDisable(ctx context.Context) (*clientv3.AuthDisableResponse, error)
	AuthStatus(ctx context.Context) (*clientv3.AuthStatusResponse, error)

	UserAddWithOptions(ctx context.Context, name string, password string, opt *clientv3.UserAddOptions) (*clientv3.AuthUserAddResponse, error)
	UserDelete(ctx context.Context, name string) (*clientv3.AuthUserDeleteResponse, error)
	UserChangePassword(ctx context.Context, name string, password string) (*clientv3.AuthUserChangePasswordResponse, error)
	UserGrantRole(ctx context.Context, user string, role string) (*clientv3.AuthUserGrantRoleResponse, error)
	UserGet(ctx context.Context, name string) (*clientv3.AuthUserGetResponse, error)
	UserList(ctx context.Context) (*clientv3.AuthUserListResponse, error)
	UserRevokeRole(ctx context.Context, name string, role string) (*clientv3.AuthUserRevokeRoleResponse, error)

	RoleAdd(ctx context.Context, name string) (*clientv3.AuthRoleAddResponse, error)
	RoleGrantPermission(ctx context.Context, name string, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error)
	RoleGet(ctx context.Context, role string) (*clientv3.AuthRoleGetResponse, error)
	RoleList(ctx context.Context) (*clientv3.AuthRoleListResponse, error)
	RoleRevokePermission(ctx context.Context, role string, key, rangeEnd string) (*clientv3.AuthRoleRevokePermissionResponse, error)
	RoleDelete(ctx context.Context, role string) (*clientv3.AuthRoleDeleteResponse, error)
}

var newCommandClientFromCmd = func(cmd *cobra.Command) commandClient {
	return mustClientFromCmd(cmd)
}

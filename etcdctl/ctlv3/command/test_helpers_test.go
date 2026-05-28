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
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func newTestCommand(tb testing.TB) *cobra.Command {
	tb.Helper()

	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Duration("command-timeout", 5*time.Second, "")
	cmd.Flags().Bool("hex", false, "")
	cmd.Flags().String("write-out", "simple", "")
	cmd.Flags().Bool("debug", false, "")
	return cmd
}

func withTestClient(tb testing.TB, c commandClient) {
	tb.Helper()

	prev := newCommandClientFromCmd
	newCommandClientFromCmd = func(*cobra.Command) commandClient { return c }
	tb.Cleanup(func() {
		newCommandClientFromCmd = prev
	})
}

func withTestDisplay(tb testing.TB, p printer) {
	tb.Helper()

	prev := display
	display = p
	tb.Cleanup(func() {
		display = prev
	})
}

func captureStdout(tb testing.TB, fn func()) string {
	tb.Helper()
	return captureOutput(tb, &os.Stdout, fn)
}

func withTestStdin(tb testing.TB, input string) {
	tb.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		tb.Fatalf("pipe: %v", err)
	}
	if _, err = io.WriteString(w, input); err != nil {
		tb.Fatalf("write stdin: %v", err)
	}
	_ = w.Close()

	prev := os.Stdin
	os.Stdin = r
	tb.Cleanup(func() {
		os.Stdin = prev
		_ = r.Close()
	})
}

func captureOutput(tb testing.TB, target **os.File, fn func()) string {
	tb.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		tb.Fatalf("pipe: %v", err)
	}

	prev := *target
	*target = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()

	_ = w.Close()
	*target = prev
	out := <-done
	_ = r.Close()

	return out
}

type fakeCommandClient struct {
	closeFn func() error

	putFn           func(context.Context, string, string, ...clientv3.OpOption) (*clientv3.PutResponse, error)
	getFn           func(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error)
	getStreamFn     func(context.Context, string, ...clientv3.OpOption) (clientv3.GetStreamChan, error)
	deleteFn        func(context.Context, string, ...clientv3.OpOption) (*clientv3.DeleteResponse, error)
	compactFn       func(context.Context, int64, ...clientv3.CompactOption) (*clientv3.CompactResponse, error)
	txnFn           func(context.Context) clientv3.Txn
	grantFn         func(context.Context, int64) (*clientv3.LeaseGrantResponse, error)
	revokeFn        func(context.Context, clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error)
	timeToLiveFn    func(context.Context, clientv3.LeaseID, ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error)
	leasesFn        func(context.Context) (*clientv3.LeaseLeasesResponse, error)
	keepAliveFn     func(context.Context, clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error)
	keepAliveOnceFn func(context.Context, clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error)

	memberAddFn          func(context.Context, []string) (*clientv3.MemberAddResponse, error)
	memberAddAsLearnerFn func(context.Context, []string) (*clientv3.MemberAddResponse, error)
	memberRemoveFn       func(context.Context, uint64) (*clientv3.MemberRemoveResponse, error)
	memberUpdateFn       func(context.Context, uint64, []string) (*clientv3.MemberUpdateResponse, error)
	memberListFn         func(context.Context, ...clientv3.OpOption) (*clientv3.MemberListResponse, error)
	memberPromoteFn      func(context.Context, uint64) (*clientv3.MemberPromoteResponse, error)

	alarmListFn   func(context.Context) (*clientv3.AlarmResponse, error)
	alarmDisarmFn func(context.Context, *clientv3.AlarmMember) (*clientv3.AlarmResponse, error)
	downgradeFn   func(context.Context, clientv3.DowngradeAction, string) (*clientv3.DowngradeResponse, error)

	authEnableFn  func(context.Context) (*clientv3.AuthEnableResponse, error)
	authDisableFn func(context.Context) (*clientv3.AuthDisableResponse, error)
	authStatusFn  func(context.Context) (*clientv3.AuthStatusResponse, error)

	userAddWithOptionsFn func(context.Context, string, string, *clientv3.UserAddOptions) (*clientv3.AuthUserAddResponse, error)
	userDeleteFn         func(context.Context, string) (*clientv3.AuthUserDeleteResponse, error)
	userChangePasswordFn func(context.Context, string, string) (*clientv3.AuthUserChangePasswordResponse, error)
	userGrantRoleFn      func(context.Context, string, string) (*clientv3.AuthUserGrantRoleResponse, error)
	userGetFn            func(context.Context, string) (*clientv3.AuthUserGetResponse, error)
	userListFn           func(context.Context) (*clientv3.AuthUserListResponse, error)
	userRevokeRoleFn     func(context.Context, string, string) (*clientv3.AuthUserRevokeRoleResponse, error)

	roleAddFn              func(context.Context, string) (*clientv3.AuthRoleAddResponse, error)
	roleGrantPermissionFn  func(context.Context, string, string, string, clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error)
	roleGetFn              func(context.Context, string) (*clientv3.AuthRoleGetResponse, error)
	roleListFn             func(context.Context) (*clientv3.AuthRoleListResponse, error)
	roleRevokePermissionFn func(context.Context, string, string, string) (*clientv3.AuthRoleRevokePermissionResponse, error)
	roleDeleteFn           func(context.Context, string) (*clientv3.AuthRoleDeleteResponse, error)
}

type fakeTxn struct {
	ifCmps   []clientv3.Cmp
	thenOps  []clientv3.Op
	elseOps  []clientv3.Op
	commitFn func() (*clientv3.TxnResponse, error)
}

func unexpectedFakeClientCall(name string) {
	panic("unexpected fakeCommandClient call: " + name)
}

func (f *fakeTxn) If(cs ...clientv3.Cmp) clientv3.Txn {
	f.ifCmps = append(f.ifCmps, cs...)
	return f
}

func (f *fakeTxn) Then(ops ...clientv3.Op) clientv3.Txn {
	f.thenOps = append(f.thenOps, ops...)
	return f
}

func (f *fakeTxn) Else(ops ...clientv3.Op) clientv3.Txn {
	f.elseOps = append(f.elseOps, ops...)
	return f
}

func (f *fakeTxn) Commit() (*clientv3.TxnResponse, error) {
	if f.commitFn == nil {
		return &clientv3.TxnResponse{}, nil
	}
	return f.commitFn()
}

func (f *fakeCommandClient) Close() error {
	if f.closeFn == nil {
		return nil
	}
	return f.closeFn()
}

func (f *fakeCommandClient) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if f.putFn == nil {
		unexpectedFakeClientCall("Put")
	}
	return f.putFn(ctx, key, val, opts...)
}

func (f *fakeCommandClient) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	if f.getFn == nil {
		unexpectedFakeClientCall("Get")
	}
	return f.getFn(ctx, key, opts...)
}

func (f *fakeCommandClient) GetStream(ctx context.Context, key string, opts ...clientv3.OpOption) (clientv3.GetStreamChan, error) {
	if f.getStreamFn == nil {
		unexpectedFakeClientCall("GetStream")
	}
	return f.getStreamFn(ctx, key, opts...)
}

func (f *fakeCommandClient) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	if f.deleteFn == nil {
		unexpectedFakeClientCall("Delete")
	}
	return f.deleteFn(ctx, key, opts...)
}

func (f *fakeCommandClient) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	if f.compactFn == nil {
		unexpectedFakeClientCall("Compact")
	}
	return f.compactFn(ctx, rev, opts...)
}

func (f *fakeCommandClient) Txn(ctx context.Context) clientv3.Txn {
	if f.txnFn == nil {
		unexpectedFakeClientCall("Txn")
	}
	return f.txnFn(ctx)
}

func (f *fakeCommandClient) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	if f.grantFn == nil {
		unexpectedFakeClientCall("Grant")
	}
	return f.grantFn(ctx, ttl)
}

func (f *fakeCommandClient) Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	if f.revokeFn == nil {
		unexpectedFakeClientCall("Revoke")
	}
	return f.revokeFn(ctx, id)
}

func (f *fakeCommandClient) TimeToLive(ctx context.Context, id clientv3.LeaseID, opts ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	if f.timeToLiveFn == nil {
		unexpectedFakeClientCall("TimeToLive")
	}
	return f.timeToLiveFn(ctx, id, opts...)
}

func (f *fakeCommandClient) Leases(ctx context.Context) (*clientv3.LeaseLeasesResponse, error) {
	if f.leasesFn == nil {
		unexpectedFakeClientCall("Leases")
	}
	return f.leasesFn(ctx)
}

func (f *fakeCommandClient) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	if f.keepAliveFn == nil {
		unexpectedFakeClientCall("KeepAlive")
	}
	return f.keepAliveFn(ctx, id)
}

func (f *fakeCommandClient) KeepAliveOnce(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	if f.keepAliveOnceFn == nil {
		unexpectedFakeClientCall("KeepAliveOnce")
	}
	return f.keepAliveOnceFn(ctx, id)
}

func (f *fakeCommandClient) MemberAdd(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	if f.memberAddFn == nil {
		unexpectedFakeClientCall("MemberAdd")
	}
	return f.memberAddFn(ctx, peerAddrs)
}

func (f *fakeCommandClient) MemberAddAsLearner(ctx context.Context, peerAddrs []string) (*clientv3.MemberAddResponse, error) {
	if f.memberAddAsLearnerFn == nil {
		unexpectedFakeClientCall("MemberAddAsLearner")
	}
	return f.memberAddAsLearnerFn(ctx, peerAddrs)
}

func (f *fakeCommandClient) MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error) {
	if f.memberRemoveFn == nil {
		unexpectedFakeClientCall("MemberRemove")
	}
	return f.memberRemoveFn(ctx, id)
}

func (f *fakeCommandClient) MemberUpdate(ctx context.Context, id uint64, peerAddrs []string) (*clientv3.MemberUpdateResponse, error) {
	if f.memberUpdateFn == nil {
		unexpectedFakeClientCall("MemberUpdate")
	}
	return f.memberUpdateFn(ctx, id, peerAddrs)
}

func (f *fakeCommandClient) MemberList(ctx context.Context, opts ...clientv3.OpOption) (*clientv3.MemberListResponse, error) {
	if f.memberListFn == nil {
		unexpectedFakeClientCall("MemberList")
	}
	return f.memberListFn(ctx, opts...)
}

func (f *fakeCommandClient) MemberPromote(ctx context.Context, id uint64) (*clientv3.MemberPromoteResponse, error) {
	if f.memberPromoteFn == nil {
		unexpectedFakeClientCall("MemberPromote")
	}
	return f.memberPromoteFn(ctx, id)
}

func (f *fakeCommandClient) AlarmList(ctx context.Context) (*clientv3.AlarmResponse, error) {
	if f.alarmListFn == nil {
		unexpectedFakeClientCall("AlarmList")
	}
	return f.alarmListFn(ctx)
}

func (f *fakeCommandClient) AlarmDisarm(ctx context.Context, m *clientv3.AlarmMember) (*clientv3.AlarmResponse, error) {
	if f.alarmDisarmFn == nil {
		unexpectedFakeClientCall("AlarmDisarm")
	}
	return f.alarmDisarmFn(ctx, m)
}

func (f *fakeCommandClient) Downgrade(ctx context.Context, action clientv3.DowngradeAction, version string) (*clientv3.DowngradeResponse, error) {
	if f.downgradeFn == nil {
		unexpectedFakeClientCall("Downgrade")
	}
	return f.downgradeFn(ctx, action, version)
}

func (f *fakeCommandClient) AuthEnable(ctx context.Context) (*clientv3.AuthEnableResponse, error) {
	if f.authEnableFn == nil {
		unexpectedFakeClientCall("AuthEnable")
	}
	return f.authEnableFn(ctx)
}

func (f *fakeCommandClient) AuthDisable(ctx context.Context) (*clientv3.AuthDisableResponse, error) {
	if f.authDisableFn == nil {
		unexpectedFakeClientCall("AuthDisable")
	}
	return f.authDisableFn(ctx)
}

func (f *fakeCommandClient) AuthStatus(ctx context.Context) (*clientv3.AuthStatusResponse, error) {
	if f.authStatusFn == nil {
		unexpectedFakeClientCall("AuthStatus")
	}
	return f.authStatusFn(ctx)
}

func (f *fakeCommandClient) UserAddWithOptions(ctx context.Context, name string, password string, opt *clientv3.UserAddOptions) (*clientv3.AuthUserAddResponse, error) {
	if f.userAddWithOptionsFn == nil {
		unexpectedFakeClientCall("UserAddWithOptions")
	}
	return f.userAddWithOptionsFn(ctx, name, password, opt)
}

func (f *fakeCommandClient) UserDelete(ctx context.Context, name string) (*clientv3.AuthUserDeleteResponse, error) {
	if f.userDeleteFn == nil {
		unexpectedFakeClientCall("UserDelete")
	}
	return f.userDeleteFn(ctx, name)
}

func (f *fakeCommandClient) UserChangePassword(ctx context.Context, name string, password string) (*clientv3.AuthUserChangePasswordResponse, error) {
	if f.userChangePasswordFn == nil {
		unexpectedFakeClientCall("UserChangePassword")
	}
	return f.userChangePasswordFn(ctx, name, password)
}

func (f *fakeCommandClient) UserGrantRole(ctx context.Context, user string, role string) (*clientv3.AuthUserGrantRoleResponse, error) {
	if f.userGrantRoleFn == nil {
		unexpectedFakeClientCall("UserGrantRole")
	}
	return f.userGrantRoleFn(ctx, user, role)
}

func (f *fakeCommandClient) UserGet(ctx context.Context, name string) (*clientv3.AuthUserGetResponse, error) {
	if f.userGetFn == nil {
		unexpectedFakeClientCall("UserGet")
	}
	return f.userGetFn(ctx, name)
}

func (f *fakeCommandClient) UserList(ctx context.Context) (*clientv3.AuthUserListResponse, error) {
	if f.userListFn == nil {
		unexpectedFakeClientCall("UserList")
	}
	return f.userListFn(ctx)
}

func (f *fakeCommandClient) UserRevokeRole(ctx context.Context, name string, role string) (*clientv3.AuthUserRevokeRoleResponse, error) {
	if f.userRevokeRoleFn == nil {
		unexpectedFakeClientCall("UserRevokeRole")
	}
	return f.userRevokeRoleFn(ctx, name, role)
}

func (f *fakeCommandClient) RoleAdd(ctx context.Context, name string) (*clientv3.AuthRoleAddResponse, error) {
	if f.roleAddFn == nil {
		unexpectedFakeClientCall("RoleAdd")
	}
	return f.roleAddFn(ctx, name)
}

func (f *fakeCommandClient) RoleGrantPermission(ctx context.Context, name string, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error) {
	if f.roleGrantPermissionFn == nil {
		unexpectedFakeClientCall("RoleGrantPermission")
	}
	return f.roleGrantPermissionFn(ctx, name, key, rangeEnd, permType)
}

func (f *fakeCommandClient) RoleGet(ctx context.Context, role string) (*clientv3.AuthRoleGetResponse, error) {
	if f.roleGetFn == nil {
		unexpectedFakeClientCall("RoleGet")
	}
	return f.roleGetFn(ctx, role)
}

func (f *fakeCommandClient) RoleList(ctx context.Context) (*clientv3.AuthRoleListResponse, error) {
	if f.roleListFn == nil {
		unexpectedFakeClientCall("RoleList")
	}
	return f.roleListFn(ctx)
}

func (f *fakeCommandClient) RoleRevokePermission(ctx context.Context, role, key, rangeEnd string) (*clientv3.AuthRoleRevokePermissionResponse, error) {
	if f.roleRevokePermissionFn == nil {
		unexpectedFakeClientCall("RoleRevokePermission")
	}
	return f.roleRevokePermissionFn(ctx, role, key, rangeEnd)
}

func (f *fakeCommandClient) RoleDelete(ctx context.Context, role string) (*clientv3.AuthRoleDeleteResponse, error) {
	if f.roleDeleteFn == nil {
		unexpectedFakeClientCall("RoleDelete")
	}
	return f.roleDeleteFn(ctx, role)
}

type recordingPrinter struct {
	putFn                  func(*clientv3.PutResponse)
	getFn                  func(*clientv3.GetResponse)
	delFn                  func(*clientv3.DeleteResponse)
	txnFn                  func(*clientv3.TxnResponse)
	watchFn                func(*clientv3.WatchResponse)
	grantFn                func(*clientv3.LeaseGrantResponse)
	revokeFn               func(clientv3.LeaseID, *clientv3.LeaseRevokeResponse)
	keepAliveFn            func(*clientv3.LeaseKeepAliveResponse)
	timeToLiveFn           func(*clientv3.LeaseTimeToLiveResponse, bool)
	leasesFn               func(*clientv3.LeaseLeasesResponse)
	memberAddFn            func(*clientv3.MemberAddResponse)
	memberRemoveFn         func(uint64, *clientv3.MemberRemoveResponse)
	memberUpdateFn         func(uint64, *clientv3.MemberUpdateResponse)
	memberPromoteFn        func(uint64, *clientv3.MemberPromoteResponse)
	memberListFn           func(*clientv3.MemberListResponse)
	endpointHealthFn       func([]epHealth)
	endpointStatusFn       func([]epStatus)
	endpointHashKVFn       func([]epHashKV)
	moveLeaderFn           func(uint64, uint64, *clientv3.MoveLeaderResponse)
	downgradeValidateFn    func(*clientv3.DowngradeResponse)
	downgradeEnableFn      func(*clientv3.DowngradeResponse)
	downgradeCancelFn      func(*clientv3.DowngradeResponse)
	alarmFn                func(*clientv3.AlarmResponse)
	roleAddFn              func(string, *clientv3.AuthRoleAddResponse)
	roleGetFn              func(string, *clientv3.AuthRoleGetResponse)
	roleDeleteFn           func(string, *clientv3.AuthRoleDeleteResponse)
	roleListFn             func(*clientv3.AuthRoleListResponse)
	roleGrantPermissionFn  func(string, *clientv3.AuthRoleGrantPermissionResponse)
	roleRevokePermissionFn func(string, string, string, *clientv3.AuthRoleRevokePermissionResponse)
	userAddFn              func(string, *clientv3.AuthUserAddResponse)
	userGetFn              func(string, *clientv3.AuthUserGetResponse)
	userListFn             func(*clientv3.AuthUserListResponse)
	userChangePasswordFn   func(*clientv3.AuthUserChangePasswordResponse)
	userGrantRoleFn        func(string, string, *clientv3.AuthUserGrantRoleResponse)
	userRevokeRoleFn       func(string, string, *clientv3.AuthUserRevokeRoleResponse)
	userDeleteFn           func(string, *clientv3.AuthUserDeleteResponse)
	authStatusFn           func(*clientv3.AuthStatusResponse)
}

func (p *recordingPrinter) Put(r *clientv3.PutResponse) {
	if p.putFn != nil {
		p.putFn(r)
	}
}

func (p *recordingPrinter) Get(r *clientv3.GetResponse) {
	if p.getFn != nil {
		p.getFn(r)
	}
}

func (p *recordingPrinter) Del(r *clientv3.DeleteResponse) {
	if p.delFn != nil {
		p.delFn(r)
	}
}

func (p *recordingPrinter) Txn(r *clientv3.TxnResponse) {
	if p.txnFn != nil {
		p.txnFn(r)
	}
}

func (p *recordingPrinter) Watch(r *clientv3.WatchResponse) {
	if p.watchFn != nil {
		p.watchFn(r)
	}
}

func (p *recordingPrinter) Grant(r *clientv3.LeaseGrantResponse) {
	if p.grantFn != nil {
		p.grantFn(r)
	}
}

func (p *recordingPrinter) Revoke(id clientv3.LeaseID, r *clientv3.LeaseRevokeResponse) {
	if p.revokeFn != nil {
		p.revokeFn(id, r)
	}
}

func (p *recordingPrinter) KeepAlive(r *clientv3.LeaseKeepAliveResponse) {
	if p.keepAliveFn != nil {
		p.keepAliveFn(r)
	}
}

func (p *recordingPrinter) TimeToLive(r *clientv3.LeaseTimeToLiveResponse, keys bool) {
	if p.timeToLiveFn != nil {
		p.timeToLiveFn(r, keys)
	}
}

func (p *recordingPrinter) Leases(r *clientv3.LeaseLeasesResponse) {
	if p.leasesFn != nil {
		p.leasesFn(r)
	}
}

func (p *recordingPrinter) MemberAdd(r *clientv3.MemberAddResponse) {
	if p.memberAddFn != nil {
		p.memberAddFn(r)
	}
}

func (p *recordingPrinter) MemberRemove(id uint64, r *clientv3.MemberRemoveResponse) {
	if p.memberRemoveFn != nil {
		p.memberRemoveFn(id, r)
	}
}

func (p *recordingPrinter) MemberUpdate(id uint64, r *clientv3.MemberUpdateResponse) {
	if p.memberUpdateFn != nil {
		p.memberUpdateFn(id, r)
	}
}

func (p *recordingPrinter) MemberPromote(id uint64, r *clientv3.MemberPromoteResponse) {
	if p.memberPromoteFn != nil {
		p.memberPromoteFn(id, r)
	}
}

func (p *recordingPrinter) MemberList(r *clientv3.MemberListResponse) {
	if p.memberListFn != nil {
		p.memberListFn(r)
	}
}

func (p *recordingPrinter) EndpointHealth(v []epHealth) {
	if p.endpointHealthFn != nil {
		p.endpointHealthFn(v)
	}
}

func (p *recordingPrinter) EndpointStatus(v []epStatus) {
	if p.endpointStatusFn != nil {
		p.endpointStatusFn(v)
	}
}

func (p *recordingPrinter) EndpointHashKV(v []epHashKV) {
	if p.endpointHashKVFn != nil {
		p.endpointHashKVFn(v)
	}
}

func (p *recordingPrinter) MoveLeader(leader, target uint64, r *clientv3.MoveLeaderResponse) {
	if p.moveLeaderFn != nil {
		p.moveLeaderFn(leader, target, r)
	}
}

func (p *recordingPrinter) DowngradeValidate(r *clientv3.DowngradeResponse) {
	if p.downgradeValidateFn != nil {
		p.downgradeValidateFn(r)
	}
}

func (p *recordingPrinter) DowngradeEnable(r *clientv3.DowngradeResponse) {
	if p.downgradeEnableFn != nil {
		p.downgradeEnableFn(r)
	}
}

func (p *recordingPrinter) DowngradeCancel(r *clientv3.DowngradeResponse) {
	if p.downgradeCancelFn != nil {
		p.downgradeCancelFn(r)
	}
}

func (p *recordingPrinter) Alarm(r *clientv3.AlarmResponse) {
	if p.alarmFn != nil {
		p.alarmFn(r)
	}
}

func (p *recordingPrinter) RoleAdd(role string, r *clientv3.AuthRoleAddResponse) {
	if p.roleAddFn != nil {
		p.roleAddFn(role, r)
	}
}

func (p *recordingPrinter) RoleGet(role string, r *clientv3.AuthRoleGetResponse) {
	if p.roleGetFn != nil {
		p.roleGetFn(role, r)
	}
}

func (p *recordingPrinter) RoleDelete(role string, r *clientv3.AuthRoleDeleteResponse) {
	if p.roleDeleteFn != nil {
		p.roleDeleteFn(role, r)
	}
}

func (p *recordingPrinter) RoleList(r *clientv3.AuthRoleListResponse) {
	if p.roleListFn != nil {
		p.roleListFn(r)
	}
}

func (p *recordingPrinter) RoleGrantPermission(role string, r *clientv3.AuthRoleGrantPermissionResponse) {
	if p.roleGrantPermissionFn != nil {
		p.roleGrantPermissionFn(role, r)
	}
}

func (p *recordingPrinter) RoleRevokePermission(role string, key string, end string, r *clientv3.AuthRoleRevokePermissionResponse) {
	if p.roleRevokePermissionFn != nil {
		p.roleRevokePermissionFn(role, key, end, r)
	}
}

func (p *recordingPrinter) UserAdd(user string, r *clientv3.AuthUserAddResponse) {
	if p.userAddFn != nil {
		p.userAddFn(user, r)
	}
}

func (p *recordingPrinter) UserGet(user string, r *clientv3.AuthUserGetResponse) {
	if p.userGetFn != nil {
		p.userGetFn(user, r)
	}
}

func (p *recordingPrinter) UserList(r *clientv3.AuthUserListResponse) {
	if p.userListFn != nil {
		p.userListFn(r)
	}
}

func (p *recordingPrinter) UserChangePassword(r *clientv3.AuthUserChangePasswordResponse) {
	if p.userChangePasswordFn != nil {
		p.userChangePasswordFn(r)
	}
}

func (p *recordingPrinter) UserGrantRole(user string, role string, r *clientv3.AuthUserGrantRoleResponse) {
	if p.userGrantRoleFn != nil {
		p.userGrantRoleFn(user, role, r)
	}
}

func (p *recordingPrinter) UserRevokeRole(user string, role string, r *clientv3.AuthUserRevokeRoleResponse) {
	if p.userRevokeRoleFn != nil {
		p.userRevokeRoleFn(user, role, r)
	}
}

func (p *recordingPrinter) UserDelete(user string, r *clientv3.AuthUserDeleteResponse) {
	if p.userDeleteFn != nil {
		p.userDeleteFn(user, r)
	}
}

func (p *recordingPrinter) AuthStatus(r *clientv3.AuthStatusResponse) {
	if p.authStatusFn != nil {
		p.authStatusFn(r)
	}
}

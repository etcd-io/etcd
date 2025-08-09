package fakeclient

import (
	"context"
	"io"
	"strings"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// New returns *clientv3.Client with fake implementations for KV, Lease, Auth, Maintenance and Cluster.
// It uses clientv3.NewCtxClient so that no real gRPC connection is established.
func New() *clientv3.Client {
	c := clientv3.NewCtxClient(context.TODO())
	fakeKV := &kvStub{}
	fakeLease := &leaseStub{}
	fakeAuth := &authStub{}
	fakeMaint := &maintenanceStub{}
	fakeCluster := &clusterStub{}

	c.KV = fakeKV
	c.Lease = fakeLease
	c.Auth = fakeAuth
	c.Maintenance = fakeMaint
	c.Cluster = fakeCluster
	c.Watcher = &watcherStub{}

	return c
}

// KV

type kvStub struct{}

func (kv *kvStub) Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	return &clientv3.PutResponse{}, nil
}

func (kv *kvStub) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

func (kv *kvStub) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return &clientv3.DeleteResponse{}, nil
}

func (kv *kvStub) Compact(ctx context.Context, rev int64, opts ...clientv3.CompactOption) (*clientv3.CompactResponse, error) {
	return &clientv3.CompactResponse{}, nil
}

func (kv *kvStub) Do(ctx context.Context, op clientv3.Op) (clientv3.OpResponse, error) {
	return clientv3.OpResponse{}, nil
}

func (kv *kvStub) Txn(ctx context.Context) clientv3.Txn {
	return &txnStub{}
}

// Lease

type leaseStub struct{}

func (l *leaseStub) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	return &clientv3.LeaseGrantResponse{ID: clientv3.LeaseID(1), TTL: ttl}, nil
}

func (l *leaseStub) Revoke(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseRevokeResponse, error) {
	return &clientv3.LeaseRevokeResponse{}, nil
}

func (l *leaseStub) TimeToLive(ctx context.Context, id clientv3.LeaseID, opts ...clientv3.LeaseOption) (*clientv3.LeaseTimeToLiveResponse, error) {
	return &clientv3.LeaseTimeToLiveResponse{ID: id, TTL: int64(1), GrantedTTL: int64(1)}, nil
}

func (l *leaseStub) Leases(ctx context.Context) (*clientv3.LeaseLeasesResponse, error) {
	return &clientv3.LeaseLeasesResponse{}, nil
}

func (l *leaseStub) KeepAlive(ctx context.Context, id clientv3.LeaseID) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	ch := make(chan *clientv3.LeaseKeepAliveResponse, 1)
	ch <- &clientv3.LeaseKeepAliveResponse{ID: id, TTL: int64(1)}
	close(ch)
	return ch, nil
}

func (l *leaseStub) KeepAliveOnce(ctx context.Context, id clientv3.LeaseID) (*clientv3.LeaseKeepAliveResponse, error) {
	return &clientv3.LeaseKeepAliveResponse{ID: id, TTL: int64(1)}, nil
}

func (l *leaseStub) Close() error { return nil }

// Auth

type authStub struct{}

func (a *authStub) AuthEnable(ctx context.Context) (*clientv3.AuthEnableResponse, error) {
	return &clientv3.AuthEnableResponse{}, nil
}

func (a *authStub) AuthDisable(ctx context.Context) (*clientv3.AuthDisableResponse, error) {
	return &clientv3.AuthDisableResponse{}, nil
}

func (a *authStub) AuthStatus(ctx context.Context) (*clientv3.AuthStatusResponse, error) {
	return &clientv3.AuthStatusResponse{}, nil
}

func (a *authStub) Authenticate(ctx context.Context, name string, password string) (*clientv3.AuthenticateResponse, error) {
	return &clientv3.AuthenticateResponse{Token: "fake"}, nil
}

func (a *authStub) UserAdd(ctx context.Context, name string, password string) (*clientv3.AuthUserAddResponse, error) {
	return &clientv3.AuthUserAddResponse{}, nil
}

func (a *authStub) UserAddWithOptions(ctx context.Context, name string, password string, options *clientv3.UserAddOptions) (*clientv3.AuthUserAddResponse, error) {
	return &clientv3.AuthUserAddResponse{}, nil
}

func (a *authStub) UserDelete(ctx context.Context, name string) (*clientv3.AuthUserDeleteResponse, error) {
	return &clientv3.AuthUserDeleteResponse{}, nil
}

func (a *authStub) UserChangePassword(ctx context.Context, name string, password string) (*clientv3.AuthUserChangePasswordResponse, error) {
	return &clientv3.AuthUserChangePasswordResponse{}, nil
}

func (a *authStub) UserGrantRole(ctx context.Context, user string, role string) (*clientv3.AuthUserGrantRoleResponse, error) {
	return &clientv3.AuthUserGrantRoleResponse{}, nil
}

func (a *authStub) UserGet(ctx context.Context, name string) (*clientv3.AuthUserGetResponse, error) {
	return &clientv3.AuthUserGetResponse{}, nil
}

func (a *authStub) UserList(ctx context.Context) (*clientv3.AuthUserListResponse, error) {
	return &clientv3.AuthUserListResponse{}, nil
}

func (a *authStub) UserRevokeRole(ctx context.Context, name string, role string) (*clientv3.AuthUserRevokeRoleResponse, error) {
	return &clientv3.AuthUserRevokeRoleResponse{}, nil
}

func (a *authStub) RoleAdd(ctx context.Context, name string) (*clientv3.AuthRoleAddResponse, error) {
	return &clientv3.AuthRoleAddResponse{}, nil
}

func (a *authStub) RoleGrantPermission(ctx context.Context, name string, key, rangeEnd string, permType clientv3.PermissionType) (*clientv3.AuthRoleGrantPermissionResponse, error) {
	return &clientv3.AuthRoleGrantPermissionResponse{}, nil
}

func (a *authStub) RoleGet(ctx context.Context, role string) (*clientv3.AuthRoleGetResponse, error) {
	return &clientv3.AuthRoleGetResponse{}, nil
}

func (a *authStub) RoleList(ctx context.Context) (*clientv3.AuthRoleListResponse, error) {
	return &clientv3.AuthRoleListResponse{}, nil
}

func (a *authStub) RoleRevokePermission(ctx context.Context, role, key, rangeEnd string) (*clientv3.AuthRoleRevokePermissionResponse, error) {
	return &clientv3.AuthRoleRevokePermissionResponse{}, nil
}

func (a *authStub) RoleDelete(ctx context.Context, role string) (*clientv3.AuthRoleDeleteResponse, error) {
	return &clientv3.AuthRoleDeleteResponse{}, nil
}

// Maintenance

type maintenanceStub struct{}

func (m *maintenanceStub) AlarmList(ctx context.Context) (*clientv3.AlarmResponse, error) {
	return &clientv3.AlarmResponse{}, nil
}

func (m *maintenanceStub) AlarmDisarm(ctx context.Context, am *clientv3.AlarmMember) (*clientv3.AlarmResponse, error) {
	return &clientv3.AlarmResponse{}, nil
}

func (m *maintenanceStub) Defragment(ctx context.Context, endpoint string) (*clientv3.DefragmentResponse, error) {
	return &clientv3.DefragmentResponse{}, nil
}

func (m *maintenanceStub) Status(ctx context.Context, endpoint string) (*clientv3.StatusResponse, error) {
	return &clientv3.StatusResponse{}, nil
}

func (m *maintenanceStub) HashKV(ctx context.Context, endpoint string, rev int64) (*clientv3.HashKVResponse, error) {
	return &clientv3.HashKVResponse{}, nil
}

func (m *maintenanceStub) SnapshotWithVersion(ctx context.Context) (*clientv3.SnapshotResponse, error) {
	return &clientv3.SnapshotResponse{}, nil
}

func (m *maintenanceStub) Snapshot(ctx context.Context) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (m *maintenanceStub) MoveLeader(ctx context.Context, transfereeID uint64) (*clientv3.MoveLeaderResponse, error) {
	return &clientv3.MoveLeaderResponse{}, nil
}

func (m *maintenanceStub) Downgrade(ctx context.Context, action clientv3.DowngradeAction, version string) (*clientv3.DowngradeResponse, error) {
	return &clientv3.DowngradeResponse{}, nil
}

// Cluster

type clusterStub struct{}

func (c *clusterStub) MemberAdd(ctx context.Context, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}

func (c *clusterStub) MemberAddAsLearner(ctx context.Context, peerURLs []string) (*clientv3.MemberAddResponse, error) {
	return &clientv3.MemberAddResponse{}, nil
}

func (c *clusterStub) MemberRemove(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error) {
	return &clientv3.MemberRemoveResponse{}, nil
}

func (c *clusterStub) MemberUpdate(ctx context.Context, id uint64, peerURLs []string) (*clientv3.MemberUpdateResponse, error) {
	return &clientv3.MemberUpdateResponse{}, nil
}

func (c *clusterStub) MemberList(ctx context.Context, opts ...clientv3.OpOption) (*clientv3.MemberListResponse, error) {
	return &clientv3.MemberListResponse{}, nil
}

func (c *clusterStub) MemberPromote(ctx context.Context, memberID uint64) (*clientv3.MemberPromoteResponse, error) {
	return &clientv3.MemberPromoteResponse{}, nil
}

// Watcher

type watcherStub struct{}

func (w *watcherStub) Close() error { return nil }

func (w *watcherStub) RequestProgress(ctx context.Context) error { return nil }

func (w *watcherStub) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
}

// Txn

type txnStub struct{}

func (t *txnStub) If(cs ...clientv3.Cmp) clientv3.Txn     { return t }
func (t *txnStub) Then(ops ...clientv3.Op) clientv3.Txn   { return t }
func (t *txnStub) Else(ops ...clientv3.Op) clientv3.Txn   { return t }
func (t *txnStub) Commit() (*clientv3.TxnResponse, error) { return &clientv3.TxnResponse{}, nil }

func init() {
	// Ensure fake client compiles against interfaces when etcd is upgraded.
	_ = &kvStub{}
	_ = &leaseStub{}
	_ = &authStub{}
	_ = &maintenanceStub{}
	_ = &clusterStub{}
	_ = &watcherStub{}
}

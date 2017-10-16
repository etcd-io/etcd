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

package clientv3

import (
	"context"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rpcFunc func(context.Context) error

func (c *Client) do(
	rpcCtx context.Context,
	pinned string,
	write bool,
	f rpcFunc) (unhealthy, switchEp, retryEp bool, err error) {
	err = f(rpcCtx)
	if err == nil {
		unhealthy, switchEp, retryEp = false, false, false
		return unhealthy, switchEp, retryEp, nil
	}

	if logger.V(4) {
		logger.Infof("clientv3/do: error %q on pinned endpoint %q (write %v)", err.Error(), pinned, write)
	}

	rerr := rpctypes.Error(err)
	if ev, ok := rerr.(rpctypes.EtcdError); ok {
		if ev.Code() != codes.Unavailable {
			// error from etcd server with non codes.Unavailable
			// then no endpoint switch and no retry
			// e.g. rpctypes.ErrEmptyKey, rpctypes.ErrNoSpace
			unhealthy, switchEp, retryEp = false, false, false
			return unhealthy, switchEp, retryEp, err
		}
		// error from etcd server with codes.Unavailable
		// then endpoint switch and retry
		// e.g. rpctypes.ErrTimeout
		unhealthy, switchEp, retryEp = true, true, true
		if write { // only retry immutable RPCs ("put at-most-once semantics")
			retryEp = false
		}
		return unhealthy, switchEp, retryEp, err
	}

	// if unknown status error from gRPC, then endpoint switch and no retry
	s, ok := status.FromError(err)
	if !ok {
		unhealthy, switchEp, retryEp = true, true, false
		return unhealthy, switchEp, retryEp, err
	}

	// assume transport.ConnectionError, transport.StreamError, or others from gRPC
	// converts to grpc/status.(*statusError) by grpc/toRPCErr
	// (e.g. transport.ErrConnClosing when server closed transport, failing node)
	// if known status error from gRPC with following codes, then endpoint switch and retry
	if s.Code() == codes.Unavailable ||
		s.Code() == codes.Internal ||
		s.Code() == codes.DeadlineExceeded {
		unhealthy, switchEp, retryEp = true, true, true
		// only retry immutable RPCs ("put at-most-once semantics")
		// mutable RPCs only retried when "there is no address available"
		if s.Message() == "there is no address available" { // pinned == ""
			// if connection is not up yet, then just retry endpoint switch
			// in case of slow gRPC connection creation
			unhealthy, switchEp = false, true
			if logger.V(4) {
				logger.Infof("clientv3/do: empty endpoint will be retried")
			}
		} else if write {
			unhealthy, retryEp = true, false
		}
	}

	return unhealthy, switchEp, retryEp, err
}

type retryRPCFunc func(context.Context, rpcFunc) error

func (c *Client) newRetryWrapper(write bool) retryRPCFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			pinned := c.balancer.pinned()
			staleEp := pinned != "" && c.balancer.endpoint(pinned) == ""

			var unhealthy, switchEp, retryEp bool
			var err error
			if !staleEp { // endpoint is up-to-date
				unhealthy, switchEp, retryEp, err = c.do(rpcCtx, pinned, write, f)
			} else {
				// if stale endpoint, then endpoint switch and retry
				unhealthy, switchEp, retryEp = false, true, true
				if logger.V(4) {
					logger.Infof("clientv3/retry: stale endpoint %q will be switched/retried", pinned)
				}
			}
			if !unhealthy && !switchEp && !retryEp && err == nil {
				return nil
			}

			// mark as unhealthy
			if unhealthy {
				c.balancer.endpointError(pinned, err)
			}
			// trigger endpoint switch in balancer
			if switchEp {
				c.balancer.next()
			}
			// wait for another endpoint to come up
			if retryEp {
				select {
				case <-c.balancer.ConnectNotify():
				case <-rpcCtx.Done():
					return rpcCtx.Err()
				case <-c.ctx.Done():
					return c.ctx.Err()
				}
				continue
			}

			// TODO: remove duplicate error handling inside toErr
			return toErr(rpcCtx, err)
		}
	}
}

func (c *Client) newAuthRetryWrapper() retryRPCFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			pinned := c.balancer.pinned()
			err := f(rpcCtx)
			if err == nil {
				return nil
			}
			// always stop retry on etcd errors other than invalid auth token
			if rpctypes.Error(err) == rpctypes.ErrInvalidAuthToken {
				gterr := c.getToken(rpcCtx)
				if gterr != nil {
					if logger.V(4) {
						logger.Infof("clientv3/auth-retry: error %v(%v) on pinned endpoint %q (returning)", err, gterr, pinned)
					}
					return err // return the original error for simplicity
				}
				if logger.V(4) {
					logger.Infof("clientv3/auth-retry: error %v on pinned endpoint %q (retrying)", err, pinned)
				}
				continue
			}
			return err
		}
	}
}

// RetryKVClient implements a KVClient that uses the client's FailFast retry policy.
func RetryKVClient(c *Client) pb.KVClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	conn := pb.NewKVClient(c.conn)
	retryBasic := &retryKVClient{&retryWriteKVClient{conn, writeRetry}, readRetry}
	retryAuthWrapper := c.newAuthRetryWrapper()
	return &retryKVClient{
		&retryWriteKVClient{retryBasic, retryAuthWrapper},
		retryAuthWrapper}
}

type retryKVClient struct {
	*retryWriteKVClient
	readRetry retryRPCFunc
}

func (rkv *retryKVClient) Range(ctx context.Context, in *pb.RangeRequest, opts ...grpc.CallOption) (resp *pb.RangeResponse, err error) {
	err = rkv.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Range(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryWriteKVClient struct {
	pb.KVClient
	retryf retryRPCFunc
}

func (rkv *retryWriteKVClient) Put(ctx context.Context, in *pb.PutRequest, opts ...grpc.CallOption) (resp *pb.PutResponse, err error) {
	err = rkv.retryf(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Put(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) DeleteRange(ctx context.Context, in *pb.DeleteRangeRequest, opts ...grpc.CallOption) (resp *pb.DeleteRangeResponse, err error) {
	err = rkv.retryf(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.DeleteRange(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) Txn(ctx context.Context, in *pb.TxnRequest, opts ...grpc.CallOption) (resp *pb.TxnResponse, err error) {
	err = rkv.retryf(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Txn(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) Compact(ctx context.Context, in *pb.CompactionRequest, opts ...grpc.CallOption) (resp *pb.CompactionResponse, err error) {
	err = rkv.retryf(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Compact(rctx, in, opts...)
		return err
	})
	return resp, err
}

// RetryLeaseClient implements a LeaseClient.
func RetryLeaseClient(c *Client) pb.LeaseClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	conn := pb.NewLeaseClient(c.conn)
	retryBasic := &retryLeaseClient{&retryWriteLeaseClient{conn, writeRetry}, readRetry}
	retryAuthWrapper := c.newAuthRetryWrapper()
	return &retryLeaseClient{
		&retryWriteLeaseClient{retryBasic, retryAuthWrapper},
		retryAuthWrapper}
}

type retryLeaseClient struct {
	*retryWriteLeaseClient
	readRetry retryRPCFunc
}

func (rlc *retryLeaseClient) LeaseTimeToLive(ctx context.Context, in *pb.LeaseTimeToLiveRequest, opts ...grpc.CallOption) (resp *pb.LeaseTimeToLiveResponse, err error) {
	err = rlc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseTimeToLive(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rlc *retryLeaseClient) LeaseLeases(ctx context.Context, in *pb.LeaseLeasesRequest, opts ...grpc.CallOption) (resp *pb.LeaseLeasesResponse, err error) {
	err = rlc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseLeases(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryWriteLeaseClient struct {
	pb.LeaseClient
	retryf retryRPCFunc
}

func (rlc *retryWriteLeaseClient) LeaseGrant(ctx context.Context, in *pb.LeaseGrantRequest, opts ...grpc.CallOption) (resp *pb.LeaseGrantResponse, err error) {
	err = rlc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseGrant(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rlc *retryWriteLeaseClient) LeaseRevoke(ctx context.Context, in *pb.LeaseRevokeRequest, opts ...grpc.CallOption) (resp *pb.LeaseRevokeResponse, err error) {
	err = rlc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseRevoke(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rlc *retryWriteLeaseClient) LeaseKeepAlive(ctx context.Context, opts ...grpc.CallOption) (stream pb.Lease_LeaseKeepAliveClient, err error) {
	err = rlc.retryf(ctx, func(rctx context.Context) error {
		stream, err = rlc.LeaseClient.LeaseKeepAlive(rctx, opts...)
		return err
	})
	return stream, err
}

// RetryClusterClient implements a ClusterClient.
func RetryClusterClient(c *Client) pb.ClusterClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	conn := pb.NewClusterClient(c.conn)
	return &retryClusterClient{&retryWriteClusterClient{conn, writeRetry}, readRetry}
}

type retryClusterClient struct {
	*retryWriteClusterClient
	readRetry retryRPCFunc
}

func (rcc *retryClusterClient) MemberList(ctx context.Context, in *pb.MemberListRequest, opts ...grpc.CallOption) (resp *pb.MemberListResponse, err error) {
	err = rcc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberList(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryWriteClusterClient struct {
	pb.ClusterClient
	retryf retryRPCFunc
}

func (rcc *retryWriteClusterClient) MemberAdd(ctx context.Context, in *pb.MemberAddRequest, opts ...grpc.CallOption) (resp *pb.MemberAddResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryWriteClusterClient) MemberRemove(ctx context.Context, in *pb.MemberRemoveRequest, opts ...grpc.CallOption) (resp *pb.MemberRemoveResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberRemove(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryWriteClusterClient) MemberUpdate(ctx context.Context, in *pb.MemberUpdateRequest, opts ...grpc.CallOption) (resp *pb.MemberUpdateResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberUpdate(rctx, in, opts...)
		return err
	})
	return resp, err
}

// RetryMaintenanceClient implements a Maintenance.
func RetryMaintenanceClient(c *Client, conn *grpc.ClientConn) pb.MaintenanceClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	return &retryMaintenanceClient{&retryWriteMaintenanceClient{pb.NewMaintenanceClient(conn), writeRetry}, readRetry}
}

type retryMaintenanceClient struct {
	pb.MaintenanceClient
	readRetry retryRPCFunc
}

func (rmc *retryMaintenanceClient) Alarm(ctx context.Context, in *pb.AlarmRequest, opts ...grpc.CallOption) (resp *pb.AlarmResponse, err error) {
	err = rmc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.Alarm(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rmc *retryMaintenanceClient) Status(ctx context.Context, in *pb.StatusRequest, opts ...grpc.CallOption) (resp *pb.StatusResponse, err error) {
	err = rmc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.Status(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rmc *retryMaintenanceClient) Hash(ctx context.Context, in *pb.HashRequest, opts ...grpc.CallOption) (resp *pb.HashResponse, err error) {
	err = rmc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.Hash(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rmc *retryMaintenanceClient) HashKV(ctx context.Context, in *pb.HashKVRequest, opts ...grpc.CallOption) (resp *pb.HashKVResponse, err error) {
	err = rmc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.HashKV(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rmc *retryMaintenanceClient) Snapshot(ctx context.Context, in *pb.SnapshotRequest, opts ...grpc.CallOption) (stream pb.Maintenance_SnapshotClient, err error) {
	err = rmc.readRetry(ctx, func(rctx context.Context) error {
		stream, err = rmc.MaintenanceClient.Snapshot(rctx, in, opts...)
		return err
	})
	return stream, err
}

type retryWriteMaintenanceClient struct {
	pb.MaintenanceClient
	retryf retryRPCFunc
}

func (rmc *retryWriteMaintenanceClient) Defragment(ctx context.Context, in *pb.DefragmentRequest, opts ...grpc.CallOption) (resp *pb.DefragmentResponse, err error) {
	err = rmc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.Defragment(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rmc *retryWriteMaintenanceClient) MoveLeader(ctx context.Context, in *pb.MoveLeaderRequest, opts ...grpc.CallOption) (resp *pb.MoveLeaderResponse, err error) {
	err = rmc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rmc.MaintenanceClient.MoveLeader(rctx, in, opts...)
		return err
	})
	return resp, err
}

// RetryAuthClient implements a AuthClient.
func RetryAuthClient(c *Client) pb.AuthClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	conn := pb.NewAuthClient(c.conn)
	return &retryAuthClient{&retryWriteAuthClient{conn, writeRetry}, readRetry}
}

type retryAuthClient struct {
	*retryWriteAuthClient
	readRetry retryRPCFunc
}

func (rac *retryAuthClient) UserList(ctx context.Context, in *pb.AuthUserListRequest, opts ...grpc.CallOption) (resp *pb.AuthUserListResponse, err error) {
	err = rac.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserList(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleGet(ctx context.Context, in *pb.AuthRoleGetRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleGetResponse, err error) {
	err = rac.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleGet(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleList(ctx context.Context, in *pb.AuthRoleListRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleListResponse, err error) {
	err = rac.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleList(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryWriteAuthClient struct {
	pb.AuthClient
	retryf retryRPCFunc
}

func (rac *retryWriteAuthClient) AuthEnable(ctx context.Context, in *pb.AuthEnableRequest, opts ...grpc.CallOption) (resp *pb.AuthEnableResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthEnable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) AuthDisable(ctx context.Context, in *pb.AuthDisableRequest, opts ...grpc.CallOption) (resp *pb.AuthDisableResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthDisable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) UserAdd(ctx context.Context, in *pb.AuthUserAddRequest, opts ...grpc.CallOption) (resp *pb.AuthUserAddResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) UserDelete(ctx context.Context, in *pb.AuthUserDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthUserDeleteResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) UserChangePassword(ctx context.Context, in *pb.AuthUserChangePasswordRequest, opts ...grpc.CallOption) (resp *pb.AuthUserChangePasswordResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserChangePassword(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) UserGrantRole(ctx context.Context, in *pb.AuthUserGrantRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserGrantRoleResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserGrantRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) UserRevokeRole(ctx context.Context, in *pb.AuthUserRevokeRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserRevokeRoleResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserRevokeRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) RoleAdd(ctx context.Context, in *pb.AuthRoleAddRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleAddResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) RoleDelete(ctx context.Context, in *pb.AuthRoleDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleDeleteResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) RoleGrantPermission(ctx context.Context, in *pb.AuthRoleGrantPermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleGrantPermissionResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleGrantPermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) RoleRevokePermission(ctx context.Context, in *pb.AuthRoleRevokePermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleRevokePermissionResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleRevokePermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryWriteAuthClient) Authenticate(ctx context.Context, in *pb.AuthenticateRequest, opts ...grpc.CallOption) (resp *pb.AuthenticateResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.Authenticate(rctx, in, opts...)
		return err
	})
	return resp, err
}

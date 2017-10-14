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
	"google.golang.org/grpc/transport"
)

type rpcFunc func(ctx context.Context) error
type retryRpcFunc func(context.Context, rpcFunc) error

func (c *Client) newRetryWrapper(isWrite bool) retryRpcFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			select {
			case <-c.balancer.ConnectNotify():
			case <-rpcCtx.Done():
				return rpcCtx.Err()
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
			pinned := c.balancer.pinned()
			err := f(rpcCtx)
			if err == nil {
				return nil
			}

			if logger.V(4) {
				logger.Infof("clientv3/retry: error %v on pinned endpoint %s (write %v)", err, pinned, isWrite)
			}

			// error from etcd server with codes.Unavailable, then endpoint switch and retry
			// e.g. rpctypes.ErrTimeout
			// error from etcd server with non codes.Unavailable, then no endpoint switch and no retry
			// e.g. rpctypes.ErrEmptyKey, rpctypes.ErrNoSpace
			rerr := rpctypes.Error(err)
			if ev, ok := rerr.(rpctypes.EtcdError); ok {
				if ev.Code() != codes.Unavailable {
					return err
				}
				c.balancer.endpointError(pinned, err)
				c.balancer.next()
				if !isWrite { // only retry immutable RPCs ("put at most once semantics")
					continue
				}
				return err
			}

			// if temporary gRPC connection error, then endpoint switch but no retry
			// e.g. transport.ErrConnClosing when server closed transport, failing node
			if tv, ok := err.(transport.ConnectionError); ok && tv.Temporary() {
				// mark as unhealthy and wait for another endpoint to come up
				c.balancer.endpointError(pinned, err)
				c.balancer.next()
				return err
			}

			// TODO: handle transport.StreamError?

			// if unknown status error from gRPC, then endpoint switch but no retry
			s, ok := status.FromError(err)
			if !ok {
				c.balancer.endpointError(pinned, err)
				c.balancer.next()
				return err
			}

			// if known status error from gRPC with following codes,
			// then endpoint switch and retry
			if s.Code() == codes.Unavailable ||
				s.Code() == codes.Internal ||
				s.Code() == codes.DeadlineExceeded {
				c.balancer.endpointError(pinned, err)
				c.balancer.next()
				if !isWrite { // only retry immutable RPCs ("put at most once semantics")
					continue
				}
			}

			// TODO: remove duplicate error handling inside toErr
			return toErr(rpcCtx, err)
		}
	}
}

func (c *Client) newAuthRetryWrapper() retryRpcFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			pinned := c.balancer.pinned()
			err := f(rpcCtx)
			if err == nil {
				return nil
			}
			if logger.V(4) {
				logger.Infof("clientv3/auth-retry: error %v on pinned endpoint %s", err, pinned)
			}
			// always stop retry on etcd errors other than invalid auth token
			if rpctypes.Error(err) == rpctypes.ErrInvalidAuthToken {
				gterr := c.getToken(rpcCtx)
				if gterr != nil {
					return err // return the original error for simplicity
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
	readRetry retryRpcFunc
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
	retryf retryRpcFunc
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
	readRetry retryRpcFunc
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
	retryf retryRpcFunc
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

type retryClusterClient struct {
	pb.ClusterClient
	retryf retryRpcFunc
}

// RetryClusterClient implements a ClusterClient that uses the client's FailFast retry policy.
func RetryClusterClient(c *Client) pb.ClusterClient {
	return &retryClusterClient{pb.NewClusterClient(c.conn), c.newRetryWrapper(true)}
}

func (rcc *retryClusterClient) MemberAdd(ctx context.Context, in *pb.MemberAddRequest, opts ...grpc.CallOption) (resp *pb.MemberAddResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryClusterClient) MemberRemove(ctx context.Context, in *pb.MemberRemoveRequest, opts ...grpc.CallOption) (resp *pb.MemberRemoveResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberRemove(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryClusterClient) MemberUpdate(ctx context.Context, in *pb.MemberUpdateRequest, opts ...grpc.CallOption) (resp *pb.MemberUpdateResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberUpdate(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryClusterClient) MemberList(ctx context.Context, in *pb.MemberListRequest, opts ...grpc.CallOption) (resp *pb.MemberListResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberList(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryMaintenanceClient struct {
	pb.MaintenanceClient
	retryf retryRpcFunc
}

// RetryMaintenanceClient implements a Maintenance that uses the client's FailFast retry policy.
func RetryMaintenanceClient(c *Client, conn *grpc.ClientConn) pb.MaintenanceClient {
	return &retryMaintenanceClient{pb.NewMaintenanceClient(conn), c.newRetryWrapper(false)}
}

func (rcc *retryMaintenanceClient) Alarm(ctx context.Context, in *pb.AlarmRequest, opts ...grpc.CallOption) (resp *pb.AlarmResponse, err error) {
	err = rcc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rcc.MaintenanceClient.Alarm(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryAuthClient struct {
	pb.AuthClient
	retryf retryRpcFunc
}

// RetryAuthClient implements a AuthClient that uses the client's FailFast retry policy.
func RetryAuthClient(c *Client) pb.AuthClient {
	return &retryAuthClient{pb.NewAuthClient(c.conn), c.newRetryWrapper(true)}
}

func (rac *retryAuthClient) AuthEnable(ctx context.Context, in *pb.AuthEnableRequest, opts ...grpc.CallOption) (resp *pb.AuthEnableResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthEnable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) AuthDisable(ctx context.Context, in *pb.AuthDisableRequest, opts ...grpc.CallOption) (resp *pb.AuthDisableResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthDisable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserAdd(ctx context.Context, in *pb.AuthUserAddRequest, opts ...grpc.CallOption) (resp *pb.AuthUserAddResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserDelete(ctx context.Context, in *pb.AuthUserDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthUserDeleteResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserChangePassword(ctx context.Context, in *pb.AuthUserChangePasswordRequest, opts ...grpc.CallOption) (resp *pb.AuthUserChangePasswordResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserChangePassword(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserGrantRole(ctx context.Context, in *pb.AuthUserGrantRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserGrantRoleResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserGrantRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserRevokeRole(ctx context.Context, in *pb.AuthUserRevokeRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserRevokeRoleResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserRevokeRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleAdd(ctx context.Context, in *pb.AuthRoleAddRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleAddResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleDelete(ctx context.Context, in *pb.AuthRoleDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleDeleteResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleGet(ctx context.Context, in *pb.AuthRoleGetRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleGetResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleGet(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleList(ctx context.Context, in *pb.AuthRoleListRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleListResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleList(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleGrantPermission(ctx context.Context, in *pb.AuthRoleGrantPermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleGrantPermissionResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleGrantPermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleRevokePermission(ctx context.Context, in *pb.AuthRoleRevokePermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleRevokePermissionResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleRevokePermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) Authenticate(ctx context.Context, in *pb.AuthenticateRequest, opts ...grpc.CallOption) (resp *pb.AuthenticateResponse, err error) {
	err = rac.retryf(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.Authenticate(rctx, in, opts...)
		return err
	})
	return resp, err
}

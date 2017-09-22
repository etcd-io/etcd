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

type rpcFunc func(ctx context.Context) error
type retryRpcFunc func(context.Context, rpcFunc) error
type retryStopErrFunc func(error) bool

func isReadStopError(err error) bool {
	eErr := rpctypes.Error(err)
	// always stop retry on etcd errors
	if _, ok := eErr.(rpctypes.EtcdError); ok {
		return true
	}
	// only retry if unavailable
	ev, _ := status.FromError(err)
	return ev.Code() != codes.Unavailable
}

func isWriteStopError(err error) bool {
	ev, _ := status.FromError(err)
	if ev.Code() != codes.Unavailable {
		return true
	}
	return rpctypes.ErrorDesc(err) != "there is no address available"
}

func (c *Client) newRetryWrapper(isStop retryStopErrFunc) retryRpcFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			if err := f(rpcCtx); err == nil || isStop(err) {
				return err
			}
			select {
			case <-c.balancer.ConnectNotify():
			case <-rpcCtx.Done():
				return rpcCtx.Err()
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		}
	}
}

func (c *Client) newAuthRetryWrapper() retryRpcFunc {
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			err := f(rpcCtx)
			if err == nil {
				return nil
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
	readRetry := c.newRetryWrapper(isReadStopError)
	writeRetry := c.newRetryWrapper(isWriteStopError)
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

type retryLeaseClient struct {
	pb.LeaseClient
	retryf retryRpcFunc
}

// RetryLeaseClient implements a LeaseClient that uses the client's FailFast retry policy.
func RetryLeaseClient(c *Client) pb.LeaseClient {
	retry := &retryLeaseClient{
		pb.NewLeaseClient(c.conn),
		c.newRetryWrapper(isReadStopError),
	}
	return &retryLeaseClient{retry, c.newAuthRetryWrapper()}
}

func (rlc *retryLeaseClient) LeaseGrant(ctx context.Context, in *pb.LeaseGrantRequest, opts ...grpc.CallOption) (resp *pb.LeaseGrantResponse, err error) {
	err = rlc.retryf(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseGrant(rctx, in, opts...)
		return err
	})
	return resp, err

}

func (rlc *retryLeaseClient) LeaseRevoke(ctx context.Context, in *pb.LeaseRevokeRequest, opts ...grpc.CallOption) (resp *pb.LeaseRevokeResponse, err error) {
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
	return &retryClusterClient{pb.NewClusterClient(c.conn), c.newRetryWrapper(isWriteStopError)}
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

type retryAuthClient struct {
	pb.AuthClient
	retryf retryRpcFunc
}

// RetryAuthClient implements a AuthClient that uses the client's FailFast retry policy.
func RetryAuthClient(c *Client) pb.AuthClient {
	return &retryAuthClient{pb.NewAuthClient(c.conn), c.newRetryWrapper(isWriteStopError)}
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

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
	"time"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/transport"
)

type rpcFunc func(context.Context) error

// TODO: clean up gRPC error handling when "FailFast=false" is fixed.
// See https://github.com/grpc/grpc-go/issues/1532.
func (c *Client) do(
	rpcCtx context.Context,
	pinned string,
	write bool,
	f rpcFunc) (unhealthy, connectWait, switchEp, retryEp bool, err error) {
	err = f(rpcCtx)
	if err == nil {
		unhealthy, connectWait, switchEp, retryEp = false, false, false, false
		return unhealthy, connectWait, switchEp, retryEp, nil
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
			unhealthy, connectWait, switchEp, retryEp = false, false, false, false
			return unhealthy, connectWait, switchEp, retryEp, err
		}
		// error from etcd server with codes.Unavailable
		// then endpoint switch and retry
		// e.g. rpctypes.ErrTimeout
		unhealthy, connectWait, switchEp, retryEp = true, false, true, true
		if write { // only retry immutable RPCs ("put at-most-once semantics")
			retryEp = false
		}
		return unhealthy, connectWait, switchEp, retryEp, err
	}

	// if failed to establish new stream or server is not reachable,
	// then endpoint switch and retry
	// e.g. transport.ErrConnClosing
	if tv, ok := err.(transport.ConnectionError); ok && tv.Temporary() {
		unhealthy, connectWait, switchEp, retryEp = true, false, true, true
		return unhealthy, connectWait, switchEp, retryEp, err
	}

	// if unknown status error from gRPC, then endpoint switch and no retry
	s, ok := status.FromError(err)
	if !ok {
		unhealthy, connectWait, switchEp, retryEp = true, false, true, false
		return unhealthy, connectWait, switchEp, retryEp, err
	}

	// assume transport.ConnectionError, transport.StreamError, or others from gRPC
	// converts to grpc/status.(*statusError) by grpc/toRPCErr
	// (e.g. transport.ErrConnClosing when server closed transport, failing node)
	// if known status error from gRPC with following codes,
	// then endpoint switch and retry
	if s.Code() == codes.Unavailable ||
		s.Code() == codes.Internal ||
		s.Code() == codes.DeadlineExceeded {
		unhealthy, connectWait, switchEp, retryEp = true, false, true, true
		switch s.Message() {
		case "there is no address available":
			// pinned == "" or endpoint unpinned right before/during RPC send
			// no need to mark empty endpoint
			unhealthy = false

			// RPC was sent with empty pinned address:
			//   1. initial connection has not been established (never pinned)
			//   2. an endpoint has just been unpinned from an error
			// Both cases expect to pin a new endpoint when Up is called.
			// Thus, should be retried.
			retryEp = true
			if logger.V(4) {
				logger.Infof("clientv3/do: there was no pinned endpoint (will be retried)")
			}

			// case A.
			// gRPC is being too slow to start transport (e.g. errors, backoff),
			// then endpoint switch. Otherwise, it can block forever to connect wait.
			// This case is safe to reset/drain all current connections.
			switchEp = true

			// case B.
			// gRPC is just about to establish transport within a moment,
			// where endpoint switch would enforce connection drain on the healthy
			// endpoint, which otherwise could be pinned and connected.
			// The healthy endpoint gets pinned but unpinned right after with
			// an error "grpc: the connection is drained", from endpoint switch.
			// Thus, not safe to trigger endpoint switch; new healthy endpoint
			// will be marked as unhealthy and not be pinned for another >3 seconds.
			// Not acceptable when this healthy endpoint was the only available one!
			// Workaround is wait up to dial timeout, and then endpoint switch
			// only when the connection still has not been "Up" (still no pinned endpoint).
			// TODO: remove this when gRPC has better way to track connection status
			connectWait = true

		case "grpc: the connection is closing",
			"grpc: the connection is unavailable":
			// gRPC v1.7 retries on these errors with FailFast=false
			// etcd client sets FailFast=true, thus retry manually
			unhealthy, connectWait, switchEp, retryEp = true, false, true, true

		default:
			// only retry immutable RPCs ("put at-most-once semantics")
			// mutable RPCs should not be retried if unknown errors
			if write {
				unhealthy, connectWait, switchEp, retryEp = true, false, true, false
			}
		}
	}

	return unhealthy, connectWait, switchEp, retryEp, err
}

type retryRPCFunc func(context.Context, rpcFunc) error

const minDialDuration = 3 * time.Second

func (c *Client) newRetryWrapper(write bool) retryRPCFunc {
	dialTimeout := c.cfg.DialTimeout
	if dialTimeout < minDialDuration {
		dialTimeout = minDialDuration
	}
	dialWait := func(rpcCtx context.Context) (up bool, err error) {
		select {
		case <-c.balancer.ConnectNotify():
			if logger.V(4) {
				logger.Infof("clientv3/retry: new healthy endpoint %q is up!", c.balancer.pinned())
			}
			return true, nil
		case <-time.After(dialTimeout):
			return false, nil
		case <-rpcCtx.Done():
			return false, rpcCtx.Err()
		case <-c.ctx.Done():
			return false, c.ctx.Err()
		}
	}
	return func(rpcCtx context.Context, f rpcFunc) error {
		for {
			pinned := c.balancer.pinned()
			staleEp := pinned != "" && c.balancer.endpoint(pinned) == ""
			eps := c.balancer.endpoints()
			singleEp, multiEp := len(eps) == 1, len(eps) > 1

			var unhealthy, connectWait, switchEp, retryEp bool
			var err error
			if pinned == "" {
				// no endpoint has not been up, then wait for connection up and retry
				unhealthy, connectWait, switchEp, retryEp = false, true, true, true
			} else if staleEp {
				// if stale, then endpoint switch and retry
				unhealthy, switchEp, retryEp = false, true, true
				// should wait in case this endpoint is stale from "SetEndpoints"
				// which resets all connections, thus expecting new healthy endpoint "Up"
				connectWait = true
				if logger.V(4) {
					logger.Infof("clientv3/retry: found stale endpoint %q (switching and retrying)", pinned)
				}
			} else { // endpoint is up-to-date
				unhealthy, connectWait, switchEp, retryEp, err = c.do(rpcCtx, pinned, write, f)
			}
			if !unhealthy && !connectWait && !switchEp && !retryEp && err == nil {
				return nil
			}

			// single endpoint with failed gRPC connection, before RPC is sent
			// should neither do endpoint switch nor retry
			// because client might have manually closed the connection
			// and there's no other endpoint to switch
			// e.g. grpc.ErrClientConnClosing
			if singleEp && connectWait {
				ep := eps[0]
				_, host, _ := parseEndpoint(ep)
				ev, ok := c.balancer.isFailed(host)
				if ok {
					// error returned to gRPC balancer "Up" error function
					// before RPC is sent (error is typed "grpc.downErr")
					if ev.err.Error() == grpc.ErrClientConnClosing.Error() {
						if logger.V(4) {
							logger.Infof("clientv3/retry: single endpoint %q with error %q (returning)", host, ev.err.Error())
						}
						return grpc.ErrClientConnClosing
					}
					if logger.V(4) {
						logger.Infof("clientv3/retry: single endpoint %q with error %q", host, ev.err.Error())
					}
				}
			}

			// mark as unhealthy, only when there are multiple endpoints
			if multiEp && unhealthy {
				c.balancer.endpointError(pinned, err)
			}

			// wait for next healthy endpoint up
			// before draining all current connections
			if connectWait {
				if logger.V(4) {
					logger.Infof("clientv3/retry: wait %v for healthy endpoint", dialTimeout)
				}
				up, derr := dialWait(rpcCtx)
				if derr != nil {
					return derr
				}
				if up { // connection is up, no need to switch endpoint
					continue
				}
				if logger.V(4) {
					logger.Infof("clientv3/retry: took too long to reset transport (switching endpoints)")
				}
			}

			// trigger endpoint switch in balancer
			if switchEp {
				c.balancer.next()
			}

			// wait for another endpoint to come up
			if retryEp {
				if logger.V(4) {
					logger.Infof("clientv3/retry: wait %v for connection before retry", dialTimeout)
				}
				up, derr := dialWait(rpcCtx)
				if derr != nil {
					return derr
				}
				if up { // retry with new endpoint
					continue
				}
				if logger.V(4) {
					logger.Infof("clientv3/retry: took too long for connection up (retrying)")
				}
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
						logger.Infof("clientv3/auth-retry: error %q(%q) on pinned endpoint %q (returning)", err.Error(), gterr.Error(), pinned)
					}
					return err // return the original error for simplicity
				}
				if logger.V(4) {
					logger.Infof("clientv3/auth-retry: error %q on pinned endpoint %q (retrying)", err, pinned)
				}
				continue
			}
			return err
		}
	}
}

// RetryKVClient implements a KVClient.
func RetryKVClient(c *Client) (readWrite, readOnly pb.KVClient) {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	retryAuthWrapper := c.newAuthRetryWrapper()
	kvc := pb.NewKVClient(c.conn)

	retryBasic := &retryKVClient{&retryWriteKVClient{kvc, writeRetry}, readRetry}
	readWrite = &retryKVClient{
		&retryWriteKVClient{retryBasic, retryAuthWrapper},
		retryAuthWrapper,
	}

	retryRead := &retryReadKVClient{kvc, readRetry}
	readOnly = &retryReadKVClient{retryRead, retryAuthWrapper}

	return readWrite, readOnly
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
	writeRetry retryRPCFunc
}

func (rkv *retryWriteKVClient) Put(ctx context.Context, in *pb.PutRequest, opts ...grpc.CallOption) (resp *pb.PutResponse, err error) {
	err = rkv.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Put(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) DeleteRange(ctx context.Context, in *pb.DeleteRangeRequest, opts ...grpc.CallOption) (resp *pb.DeleteRangeResponse, err error) {
	err = rkv.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.DeleteRange(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) Txn(ctx context.Context, in *pb.TxnRequest, opts ...grpc.CallOption) (resp *pb.TxnResponse, err error) {
	err = rkv.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Txn(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryReadKVClient struct {
	pb.KVClient
	readRetry retryRPCFunc
}

func (rkv *retryReadKVClient) Txn(ctx context.Context, in *pb.TxnRequest, opts ...grpc.CallOption) (resp *pb.TxnResponse, err error) {
	err = rkv.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Txn(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rkv *retryWriteKVClient) Compact(ctx context.Context, in *pb.CompactionRequest, opts ...grpc.CallOption) (resp *pb.CompactionResponse, err error) {
	err = rkv.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rkv.KVClient.Compact(rctx, in, opts...)
		return err
	})
	return resp, err
}

// RetryWatchClient implements a WatchClient.
func RetryWatchClient(c *Client) pb.WatchClient {
	readRetry := c.newRetryWrapper(false)
	wc := pb.NewWatchClient(c.conn)
	return &retryWatchClient{wc, readRetry}
}

type retryWatchClient struct {
	pb.WatchClient
	readRetry retryRPCFunc
}

func (rwc *retryWatchClient) Watch(ctx context.Context, opts ...grpc.CallOption) (stream pb.Watch_WatchClient, err error) {
	err = rwc.readRetry(ctx, func(rctx context.Context) error {
		stream, err = rwc.WatchClient.Watch(rctx, opts...)
		return err
	})
	return stream, err
}

type retryLeaseClient struct {
	pb.LeaseClient
	readRetry retryRPCFunc
}

// RetryLeaseClient implements a LeaseClient.
func RetryLeaseClient(c *Client) pb.LeaseClient {
	readRetry := c.newRetryWrapper(false)
	lc := pb.NewLeaseClient(c.conn)
	retryBasic := &retryLeaseClient{lc, readRetry}
	retryAuthWrapper := c.newAuthRetryWrapper()
	return &retryLeaseClient{retryBasic, retryAuthWrapper}
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

func (rlc *retryLeaseClient) LeaseGrant(ctx context.Context, in *pb.LeaseGrantRequest, opts ...grpc.CallOption) (resp *pb.LeaseGrantResponse, err error) {
	err = rlc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseGrant(rctx, in, opts...)
		return err
	})
	return resp, err

}

func (rlc *retryLeaseClient) LeaseRevoke(ctx context.Context, in *pb.LeaseRevokeRequest, opts ...grpc.CallOption) (resp *pb.LeaseRevokeResponse, err error) {
	err = rlc.readRetry(ctx, func(rctx context.Context) error {
		resp, err = rlc.LeaseClient.LeaseRevoke(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rlc *retryLeaseClient) LeaseKeepAlive(ctx context.Context, opts ...grpc.CallOption) (stream pb.Lease_LeaseKeepAliveClient, err error) {
	err = rlc.readRetry(ctx, func(rctx context.Context) error {
		stream, err = rlc.LeaseClient.LeaseKeepAlive(rctx, opts...)
		return err
	})
	return stream, err
}

// RetryClusterClient implements a ClusterClient.
func RetryClusterClient(c *Client) pb.ClusterClient {
	readRetry := c.newRetryWrapper(false)
	writeRetry := c.newRetryWrapper(true)
	cc := pb.NewClusterClient(c.conn)
	return &retryClusterClient{&retryWriteClusterClient{cc, writeRetry}, readRetry}
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
	writeRetry retryRPCFunc
}

func (rcc *retryWriteClusterClient) MemberAdd(ctx context.Context, in *pb.MemberAddRequest, opts ...grpc.CallOption) (resp *pb.MemberAddResponse, err error) {
	err = rcc.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryWriteClusterClient) MemberRemove(ctx context.Context, in *pb.MemberRemoveRequest, opts ...grpc.CallOption) (resp *pb.MemberRemoveResponse, err error) {
	err = rcc.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberRemove(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rcc *retryWriteClusterClient) MemberUpdate(ctx context.Context, in *pb.MemberUpdateRequest, opts ...grpc.CallOption) (resp *pb.MemberUpdateResponse, err error) {
	err = rcc.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rcc.ClusterClient.MemberUpdate(rctx, in, opts...)
		return err
	})
	return resp, err
}

type retryAuthClient struct {
	pb.AuthClient
	writeRetry retryRPCFunc
}

// RetryAuthClient implements a AuthClient that uses the client's FailFast retry policy.
func RetryAuthClient(c *Client) pb.AuthClient {
	return &retryAuthClient{pb.NewAuthClient(c.conn), c.newRetryWrapper(true)}
}

func (rac *retryAuthClient) AuthEnable(ctx context.Context, in *pb.AuthEnableRequest, opts ...grpc.CallOption) (resp *pb.AuthEnableResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthEnable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) AuthDisable(ctx context.Context, in *pb.AuthDisableRequest, opts ...grpc.CallOption) (resp *pb.AuthDisableResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.AuthDisable(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserAdd(ctx context.Context, in *pb.AuthUserAddRequest, opts ...grpc.CallOption) (resp *pb.AuthUserAddResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserDelete(ctx context.Context, in *pb.AuthUserDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthUserDeleteResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserChangePassword(ctx context.Context, in *pb.AuthUserChangePasswordRequest, opts ...grpc.CallOption) (resp *pb.AuthUserChangePasswordResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserChangePassword(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserGrantRole(ctx context.Context, in *pb.AuthUserGrantRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserGrantRoleResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserGrantRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) UserRevokeRole(ctx context.Context, in *pb.AuthUserRevokeRoleRequest, opts ...grpc.CallOption) (resp *pb.AuthUserRevokeRoleResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.UserRevokeRole(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleAdd(ctx context.Context, in *pb.AuthRoleAddRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleAddResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleAdd(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleDelete(ctx context.Context, in *pb.AuthRoleDeleteRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleDeleteResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleDelete(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleGrantPermission(ctx context.Context, in *pb.AuthRoleGrantPermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleGrantPermissionResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleGrantPermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

func (rac *retryAuthClient) RoleRevokePermission(ctx context.Context, in *pb.AuthRoleRevokePermissionRequest, opts ...grpc.CallOption) (resp *pb.AuthRoleRevokePermissionResponse, err error) {
	err = rac.writeRetry(ctx, func(rctx context.Context) error {
		resp, err = rac.AuthClient.RoleRevokePermission(rctx, in, opts...)
		return err
	})
	return resp, err
}

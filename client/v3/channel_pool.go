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

package clientv3

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.etcd.io/etcd/client/v3/internal/resolver"
)

type channel struct {
	conn     *grpc.ClientConn
	resolver *resolver.EtcdManualResolver
}

func (ch channel) close(ctx context.Context) error {
	ch.resolver.Close()
	return ContextError(ctx, ch.conn.Close())
}

func (c *Client) initChannelKey(ctx context.Context, key string) error {
	if err := validateChannelKey(key); err != nil {
		return err
	}
	if ctx == nil {
		ctx = c.ctx
	}

	c.epMu.Lock()
	defer c.epMu.Unlock()
	if c.channels == nil {
		return c.connectionPoolClosedError()
	}
	if _, ok := c.channels[key]; ok {
		return nil
	}
	endpoints := make([]string, len(c.endpoints))
	copy(endpoints, c.endpoints)

	r := resolver.New(endpoints...)
	conn, err := c.dialWithBalancerForKey(ctx, key, r, endpoints)
	if err != nil {
		r.Close()
		return err
	}
	if c.cfg.DialTimeout == 0 {
		if _, ok := ctx.Deadline(); ok {
			if err := waitForConnection(ctx, conn); err != nil {
				conn.Close()
				r.Close()
				return err
			}
		}
	}

	c.channels[key] = channel{conn: conn, resolver: r}
	return nil
}

func (c *Client) connectionForContext(ctx context.Context) (*grpc.ClientConn, error) {
	key := channelKeyForRequest(ctx)
	c.epMu.RLock()
	channels := c.channels
	ch := channels[key]
	c.epMu.RUnlock()
	if channels == nil {
		return nil, c.connectionPoolClosedError()
	}
	if ch.conn == nil {
		return nil, fmt.Errorf("%w: %q", ErrChannelKeyNotFound, key)
	}
	return ch.conn, nil
}

func (c *Client) connectionPoolClosedError() error {
	if err := c.ctx.Err(); err != nil {
		return err
	}
	return status.Error(codes.Canceled, "etcdclient: client connection is closing")
}

type routingClientConn struct {
	client *Client
}

func (r *routingClientConn) Invoke(ctx context.Context, method string, args, reply any, opts ...grpc.CallOption) error {
	conn, err := r.client.connectionForContext(ctx)
	if err != nil {
		return err
	}
	return conn.Invoke(ctx, method, args, reply, opts...)
}

func (r *routingClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	conn, err := r.client.connectionForContext(ctx)
	if err != nil {
		return nil, err
	}
	return conn.NewStream(ctx, desc, method, opts...)
}

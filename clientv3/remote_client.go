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
	"sync"

	"github.com/coreos/etcd/etcdserver/api/v3rpc/rpctypes"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type remoteClient struct {
	client     *Client
	conn       *grpc.ClientConn
	updateConn func(*grpc.ClientConn)
	mu         sync.Mutex
}

func newRemoteClient(client *Client, update func(*grpc.ClientConn)) *remoteClient {
	ret := &remoteClient{
		client:     client,
		conn:       client.ActiveConnection(),
		updateConn: update,
	}
	ret.mu.Lock()
	defer ret.mu.Unlock()
	ret.updateConn(ret.conn)
	return ret
}

// reconnectWait reconnects the client, returning when connection establishes/fails.
func (r *remoteClient) reconnectWait(ctx context.Context, prevErr error) error {
	r.mu.Lock()
	updated := r.tryUpdate()
	r.mu.Unlock()
	if updated {
		return nil
	}
	conn, err := r.client.connWait(ctx, prevErr)
	if err == nil {
		r.mu.Lock()
		r.conn = conn
		r.updateConn(conn)
		r.mu.Unlock()
	}
	return err
}

// reconnect will reconnect the client without waiting
func (r *remoteClient) reconnect(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tryUpdate() {
		return
	}
	r.client.connStartRetry(err)
}

func (r *remoteClient) tryUpdate() bool {
	activeConn := r.client.ActiveConnection()
	if activeConn == nil || activeConn == r.conn {
		return false
	}
	r.conn = activeConn
	r.updateConn(activeConn)
	return true
}

// acquire gets the client read lock on an established connection or
// returns an error without holding the lock.
func (r *remoteClient) acquire(ctx context.Context) error {
	for {
		r.mu.Lock()
		r.client.mu.RLock()
		closed := r.client.cancel == nil
		c := r.client.conn
		match := r.conn == c
		r.mu.Unlock()
		if match {
			return nil
		}
		r.client.mu.RUnlock()
		if closed {
			return rpctypes.ErrConnClosed
		}
		if err := r.reconnectWait(ctx, nil); err != nil {
			return err
		}
	}
}

func (r *remoteClient) release() { r.client.mu.RUnlock() }

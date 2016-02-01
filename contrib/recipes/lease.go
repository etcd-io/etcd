// Copyright 2016 CoreOS, Inc.
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
package recipe

import (
	"sync"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/lease"
)

// only keep one ephemeral lease per client
var clientLeases clientLeaseMgr = clientLeaseMgr{leases: make(map[*clientv3.Client]*leaseKeepAlive)}

type clientLeaseMgr struct {
	leases map[*clientv3.Client]*leaseKeepAlive
	mu     sync.Mutex
}

type leaseKeepAlive struct {
	id    lease.LeaseID
	donec chan struct{}
}

func SessionLease(client *clientv3.Client) (lease.LeaseID, error) {
	return clientLeases.sessionLease(client, 120)
}

func SessionLeaseTTL(client *clientv3.Client, ttl int64) (lease.LeaseID, error) {
	return clientLeases.sessionLease(client, ttl)
}

// StopSessionLease ends the refresh for the session lease. This is useful
// in case the state of the client connection is indeterminate (revoke
// would fail) or if transferring lease ownership.
func StopSessionLease(client *clientv3.Client) {
	clientLeases.mu.Lock()
	lka, ok := clientLeases.leases[client]
	if ok {
		delete(clientLeases.leases, client)
	}
	clientLeases.mu.Unlock()
	if lka != nil {
		lka.donec <- struct{}{}
		<-lka.donec
	}
}

// RevokeSessionLease revokes the session lease.
func RevokeSessionLease(client *clientv3.Client) (err error) {
	clientLeases.mu.Lock()
	lka := clientLeases.leases[client]
	clientLeases.mu.Unlock()
	StopSessionLease(client)
	if lka != nil {
		req := &pb.LeaseRevokeRequest{ID: int64(lka.id)}
		_, err = client.Lease.LeaseRevoke(context.TODO(), req)
	}
	return err
}

func (clm *clientLeaseMgr) sessionLease(client *clientv3.Client, ttl int64) (lease.LeaseID, error) {
	clm.mu.Lock()
	defer clm.mu.Unlock()
	if lka, ok := clm.leases[client]; ok {
		return lka.id, nil
	}

	resp, err := client.Lease.LeaseCreate(context.TODO(), &pb.LeaseCreateRequest{TTL: ttl})
	if err != nil {
		return lease.NoLease, err
	}
	id := lease.LeaseID(resp.ID)

	ctx, cancel := context.WithCancel(context.Background())
	keepAlive, err := client.Lease.LeaseKeepAlive(ctx)
	if err != nil || keepAlive == nil {
		return lease.NoLease, err
	}

	lka := &leaseKeepAlive{id: id, donec: make(chan struct{})}
	clm.leases[client] = lka

	// keep the lease alive until client error
	go func() {
		defer func() {
			keepAlive.CloseSend()
			clm.mu.Lock()
			delete(clm.leases, client)
			clm.mu.Unlock()
			cancel()
			close(lka.donec)
		}()

		ttl := resp.TTL
		for {
			lreq := &pb.LeaseKeepAliveRequest{ID: int64(id)}
			select {
			case <-lka.donec:
				return
			case <-time.After(time.Duration(ttl/2) * time.Second):
			}
			if err := keepAlive.Send(lreq); err != nil {
				break
			}
			resp, err := keepAlive.Recv()
			if err != nil {
				break
			}
			ttl = resp.TTL
		}
	}()

	return id, nil
}

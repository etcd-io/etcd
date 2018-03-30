// Copyright 2018 The etcd Authors
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

package rpcpb

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"

	grpc "google.golang.org/grpc"
)

var dialOpts = []grpc.DialOption{
	grpc.WithInsecure(),
	grpc.WithTimeout(5 * time.Second),
	grpc.WithBlock(),
}

// DialEtcdGRPCServer creates a raw gRPC connection to an etcd member.
func (m *Member) DialEtcdGRPCServer() (*grpc.ClientConn, error) {
	if m.EtcdClientTLS {
		// TODO: support TLS
		panic("client TLS not supported yet")
	}
	return grpc.Dial(m.EtcdClientEndpoint, dialOpts...)
}

// CreateEtcdClient creates a client from member.
func (m *Member) CreateEtcdClient() (*clientv3.Client, error) {
	if m.EtcdClientTLS {
		// TODO: support TLS
		panic("client TLS not supported yet")
	}
	return clientv3.New(clientv3.Config{
		Endpoints:   []string{m.EtcdClientEndpoint},
		DialTimeout: 5 * time.Second,
	})
}

// CheckCompact ensures that historical data before given revision has been compacted.
func (m *Member) CheckCompact(rev int64) error {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	wch := cli.Watch(ctx, "\x00", clientv3.WithFromKey(), clientv3.WithRev(rev-1))
	wr, ok := <-wch
	cancel()

	if !ok {
		return fmt.Errorf("watch channel terminated (endpoint %q)", m.EtcdClientEndpoint)
	}
	if wr.CompactRevision != rev {
		return fmt.Errorf("got compact revision %v, wanted %v (endpoint %q)", wr.CompactRevision, rev, m.EtcdClientEndpoint)
	}

	return nil
}

// Defrag runs defragmentation on this member.
func (m *Member) Defrag() error {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	_, err = cli.Defragment(ctx, m.EtcdClientEndpoint)
	cancel()
	return err
}

// RevHash fetches current revision and hash on this member.
func (m *Member) RevHash() (int64, int64, error) {
	conn, err := m.DialEtcdGRPCServer()
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	mt := pb.NewMaintenanceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := mt.Hash(ctx, &pb.HashRequest{}, grpc.FailFast(false))
	cancel()

	if err != nil {
		return 0, 0, err
	}

	return resp.Header.Revision, int64(resp.Hash), nil
}

// Rev fetches current revision on this member.
func (m *Member) Rev(ctx context.Context) (int64, error) {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return 0, fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	resp, err := cli.Status(ctx, m.EtcdClientEndpoint)
	if err != nil {
		return 0, err
	}
	return resp.Header.Revision, nil
}

// IsLeader returns true if this member is the current cluster leader.
func (m *Member) IsLeader() (bool, error) {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return false, fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	resp, err := cli.Status(context.Background(), m.EtcdClientEndpoint)
	if err != nil {
		return false, err
	}
	return resp.Header.MemberId == resp.Leader, nil
}

// WriteHealthKey writes a health key to this member.
func (m *Member) WriteHealthKey() error {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	// give enough time-out in case expensive requests (range/delete) are pending
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err = cli.Put(ctx, "health", "good")
	cancel()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	return nil
}

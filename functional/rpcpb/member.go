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
	"net/url"
	"os"
	"time"

	"github.com/coreos/etcd/clientv3"
	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/coreos/etcd/snapshot"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// DialEtcdGRPCServer creates a raw gRPC connection to an etcd member.
func (m *Member) DialEtcdGRPCServer(opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithTimeout(5 * time.Second),
		grpc.WithBlock(),
	}

	secure := false
	for _, cu := range m.Etcd.AdvertiseClientURLs {
		u, err := url.Parse(cu)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "https" { // TODO: handle unix
			secure = true
		}
	}

	if secure {
		// assume save TLS assets are already stord on disk
		tlsInfo := transport.TLSInfo{
			CertFile:      m.ClientCertPath,
			KeyFile:       m.ClientKeyPath,
			TrustedCAFile: m.ClientTrustedCAPath,

			// TODO: remove this with generated certs
			// only need it for auto TLS
			InsecureSkipVerify: true,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		creds := credentials.NewTLS(tlsConfig)
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	dialOpts = append(dialOpts, opts...)
	return grpc.Dial(m.EtcdClientEndpoint, dialOpts...)
}

// CreateEtcdClient creates a client from member.
func (m *Member) CreateEtcdClient(opts ...grpc.DialOption) (*clientv3.Client, error) {
	secure := false
	for _, cu := range m.Etcd.AdvertiseClientURLs {
		u, err := url.Parse(cu)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "https" { // TODO: handle unix
			secure = true
		}
	}

	cfg := clientv3.Config{
		Endpoints:   []string{m.EtcdClientEndpoint},
		DialTimeout: 5 * time.Second,
		DialOptions: opts,
	}
	if secure {
		// assume save TLS assets are already stord on disk
		tlsInfo := transport.TLSInfo{
			CertFile:      m.ClientCertPath,
			KeyFile:       m.ClientKeyPath,
			TrustedCAFile: m.ClientTrustedCAPath,

			// TODO: remove this with generated certs
			// only need it for auto TLS
			InsecureSkipVerify: true,
		}
		tlsConfig, err := tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		cfg.TLS = tlsConfig
	}
	return clientv3.New(cfg)
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

// Compact compacts member storage with given revision.
// It blocks until it's physically done.
func (m *Member) Compact(rev int64, timeout time.Duration) error {
	cli, err := m.CreateEtcdClient()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_, err = cli.Compact(ctx, rev, clientv3.WithCompactPhysical())
	cancel()
	return err
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

// FetchSnapshot downloads a snapshot file from this member.
func (m *Member) FetchSnapshot(lg *zap.Logger) (err error) {
	// remove existing snapshot first
	if err = os.Remove(m.SnapshotPath); err != nil {
		return err
	}

	var cli *clientv3.Client
	cli, err = m.CreateEtcdClient()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}
	defer cli.Close()

	now := time.Now()
	mgr := snapshot.NewV3(cli, lg)
	if err = mgr.Save(context.Background(), m.SnapshotPath); err != nil {
		return err
	}
	took := time.Since(now)

	var fi os.FileInfo
	fi, err = os.Stat(m.SnapshotPath)
	if err != nil {
		return err
	}

	var st snapshot.Status
	st, err = mgr.Status(m.SnapshotPath)
	if err != nil {
		return err
	}

	lg.Info(
		"snapshot saved",
		zap.String("snapshot-path", m.SnapshotPath),
		zap.String("snapshot-file-size", humanize.Bytes(uint64(fi.Size()))),
		zap.String("snapshot-total-size", humanize.Bytes(uint64(st.TotalSize))),
		zap.Int("snapshot-total-key", st.TotalKey),
		zap.Uint32("snapshot-hash", st.Hash),
		zap.Int64("snapshot-revision", st.Revision),
		zap.Duration("took", took),
	)
	return nil
}

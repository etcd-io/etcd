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
	"crypto/tls"
	"fmt"
	"net/url"
	"os"
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdctl/v3/snapshot"
	"go.etcd.io/etcd/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/transport"

	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ElectionTimeout returns an election timeout duration.
func (m *Member) ElectionTimeout() time.Duration {
	return time.Duration(m.Etcd.ElectionTimeoutMs) * time.Millisecond
}

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

// CreateEtcdClientConfig creates a client configuration from member.
func (m *Member) CreateEtcdClientConfig(opts ...grpc.DialOption) (cfg *clientv3.Config, err error) {
	secure := false
	for _, cu := range m.Etcd.AdvertiseClientURLs {
		var u *url.URL
		u, err = url.Parse(cu)
		if err != nil {
			return nil, err
		}
		if u.Scheme == "https" { // TODO: handle unix
			secure = true
		}
	}

	// TODO: make this configurable
	level := "error"
	if os.Getenv("ETCD_CLIENT_DEBUG") != "" {
		level = "debug"
	}
	lcfg := logutil.DefaultZapLoggerConfig
	lcfg.Level = zap.NewAtomicLevelAt(logutil.ConvertToZapLevel(level))

	cfg = &clientv3.Config{
		Endpoints:   []string{m.EtcdClientEndpoint},
		DialTimeout: 10 * time.Second,
		DialOptions: opts,
		LogConfig:   &lcfg,
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
		var tlsConfig *tls.Config
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			return nil, err
		}
		cfg.TLS = tlsConfig
	}
	return cfg, err
}

// CreateEtcdClient creates a client from member.
func (m *Member) CreateEtcdClient(opts ...grpc.DialOption) (*clientv3.Client, error) {
	cfg, err := m.CreateEtcdClientConfig(opts...)
	if err != nil {
		return nil, err
	}
	return clientv3.New(*cfg)
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
	resp, err := mt.Hash(ctx, &pb.HashRequest{}, grpc.WaitForReady(true))
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

// SaveSnapshot downloads a snapshot file from this member, locally.
// It's meant to requested remotely, so that local member can store
// snapshot file on local disk.
func (m *Member) SaveSnapshot(lg *zap.Logger) (err error) {
	// remove existing snapshot first
	if err = os.RemoveAll(m.SnapshotPath); err != nil {
		return err
	}

	var ccfg *clientv3.Config
	ccfg, err = m.CreateEtcdClientConfig()
	if err != nil {
		return fmt.Errorf("%v (%q)", err, m.EtcdClientEndpoint)
	}

	lg.Info(
		"snapshot save START",
		zap.String("member-name", m.Etcd.Name),
		zap.Strings("member-client-urls", m.Etcd.AdvertiseClientURLs),
		zap.String("snapshot-path", m.SnapshotPath),
	)
	now := time.Now()
	mgr := snapshot.NewV3(lg)
	if err = mgr.Save(context.Background(), *ccfg, m.SnapshotPath); err != nil {
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
	m.SnapshotInfo = &SnapshotInfo{
		MemberName:        m.Etcd.Name,
		MemberClientURLs:  m.Etcd.AdvertiseClientURLs,
		SnapshotPath:      m.SnapshotPath,
		SnapshotFileSize:  humanize.Bytes(uint64(fi.Size())),
		SnapshotTotalSize: humanize.Bytes(uint64(st.TotalSize)),
		SnapshotTotalKey:  int64(st.TotalKey),
		SnapshotHash:      int64(st.Hash),
		SnapshotRevision:  st.Revision,
		Took:              fmt.Sprintf("%v", took),
	}
	lg.Info(
		"snapshot save END",
		zap.String("member-name", m.SnapshotInfo.MemberName),
		zap.Strings("member-client-urls", m.SnapshotInfo.MemberClientURLs),
		zap.String("snapshot-path", m.SnapshotPath),
		zap.String("snapshot-file-size", m.SnapshotInfo.SnapshotFileSize),
		zap.String("snapshot-total-size", m.SnapshotInfo.SnapshotTotalSize),
		zap.Int64("snapshot-total-key", m.SnapshotInfo.SnapshotTotalKey),
		zap.Int64("snapshot-hash", m.SnapshotInfo.SnapshotHash),
		zap.Int64("snapshot-revision", m.SnapshotInfo.SnapshotRevision),
		zap.String("took", m.SnapshotInfo.Took),
	)
	return nil
}

// RestoreSnapshot restores a cluster from a given snapshot file on disk.
// It's meant to requested remotely, so that local member can load the
// snapshot file from local disk.
func (m *Member) RestoreSnapshot(lg *zap.Logger) (err error) {
	if err = os.RemoveAll(m.EtcdOnSnapshotRestore.DataDir); err != nil {
		return err
	}
	if err = os.RemoveAll(m.EtcdOnSnapshotRestore.WALDir); err != nil {
		return err
	}

	lg.Info(
		"snapshot restore START",
		zap.String("member-name", m.Etcd.Name),
		zap.Strings("member-client-urls", m.Etcd.AdvertiseClientURLs),
		zap.String("snapshot-path", m.SnapshotPath),
	)
	now := time.Now()
	mgr := snapshot.NewV3(lg)
	err = mgr.Restore(snapshot.RestoreConfig{
		SnapshotPath:        m.SnapshotInfo.SnapshotPath,
		Name:                m.EtcdOnSnapshotRestore.Name,
		OutputDataDir:       m.EtcdOnSnapshotRestore.DataDir,
		OutputWALDir:        m.EtcdOnSnapshotRestore.WALDir,
		PeerURLs:            m.EtcdOnSnapshotRestore.AdvertisePeerURLs,
		InitialCluster:      m.EtcdOnSnapshotRestore.InitialCluster,
		InitialClusterToken: m.EtcdOnSnapshotRestore.InitialClusterToken,
		SkipHashCheck:       false,
		// TODO: set SkipHashCheck it true, to recover from existing db file
	})
	took := time.Since(now)
	lg.Info(
		"snapshot restore END",
		zap.String("member-name", m.SnapshotInfo.MemberName),
		zap.Strings("member-client-urls", m.SnapshotInfo.MemberClientURLs),
		zap.String("snapshot-path", m.SnapshotPath),
		zap.String("snapshot-file-size", m.SnapshotInfo.SnapshotFileSize),
		zap.String("snapshot-total-size", m.SnapshotInfo.SnapshotTotalSize),
		zap.Int64("snapshot-total-key", m.SnapshotInfo.SnapshotTotalKey),
		zap.Int64("snapshot-hash", m.SnapshotInfo.SnapshotHash),
		zap.Int64("snapshot-revision", m.SnapshotInfo.SnapshotRevision),
		zap.String("took", took.String()),
		zap.Error(err),
	)
	return err
}

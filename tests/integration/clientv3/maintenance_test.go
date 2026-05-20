// Copyright 2017 The etcd Authors
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

package clientv3test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bolt "go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/api/v3/version"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/lease"
	"go.etcd.io/etcd/server/v3/storage"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
	"go.etcd.io/etcd/server/v3/storage/mvcc/testutil"
	"go.etcd.io/etcd/tests/v3/framework/integration"
)

func TestMaintenanceHashKV(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	for i := 0; i < 3; i++ {
		_, err := clus.RandClient().Put(t.Context(), "foo", "bar")
		require.NoError(t, err)
	}

	var hv uint32
	for i := 0; i < 3; i++ {
		cli := clus.Client(i)
		// ensure writes are replicated
		_, err := cli.Get(t.Context(), "foo")
		require.NoError(t, err)
		hresp, err := cli.HashKV(t.Context(), clus.Members[i].GRPCURL, 0)
		require.NoError(t, err)
		if hv == 0 {
			hv = hresp.Hash
			continue
		}
		if hv != hresp.Hash {
			t.Fatalf("#%d: hash expected %d, got %d", i, hv, hresp.Hash)
		}
	}
}

// TestCompactionHash tests compaction hash
// TODO: Change this to fuzz test
func TestCompactionHash(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	cc, err := clus.ClusterClient(t)
	require.NoError(t, err)

	testutil.TestCompactionHash(t.Context(), t, hashTestCase{cc, clus.Members[0].GRPCURL}, 1000)
}

type hashTestCase struct {
	*clientv3.Client
	url string
}

func (tc hashTestCase) Put(ctx context.Context, key, value string) error {
	_, err := tc.Client.Put(ctx, key, value)
	return err
}

func (tc hashTestCase) Delete(ctx context.Context, key string) error {
	_, err := tc.Client.Delete(ctx, key)
	return err
}

func (tc hashTestCase) HashByRev(ctx context.Context, rev int64) (testutil.KeyValueHash, error) {
	resp, err := tc.Client.HashKV(ctx, tc.url, rev)
	return testutil.KeyValueHash{Hash: resp.Hash, CompactRevision: resp.CompactRevision, Revision: resp.Header.Revision}, err
}

func (tc hashTestCase) Defrag(ctx context.Context) error {
	_, err := tc.Client.Defragment(ctx, tc.url)
	return err
}

func (tc hashTestCase) Compact(ctx context.Context, rev int64) error {
	_, err := tc.Client.Compact(ctx, rev)
	// Wait for compaction to be compacted
	time.Sleep(50 * time.Millisecond)
	return err
}

func TestMaintenanceMoveLeader(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3})
	defer clus.Terminate(t)

	oldLeadIdx := clus.WaitLeader(t)
	targetIdx := (oldLeadIdx + 1) % 3
	target := uint64(clus.Members[targetIdx].ID())

	cli := clus.Client(targetIdx)
	_, err := cli.MoveLeader(t.Context(), target)
	if !errors.Is(err, rpctypes.ErrNotLeader) {
		t.Fatalf("error expected %v, got %v", rpctypes.ErrNotLeader, err)
	}

	cli = clus.Client(oldLeadIdx)
	_, err = cli.MoveLeader(t.Context(), target)
	require.NoError(t, err)

	leadIdx := clus.WaitLeader(t)
	lead := uint64(clus.Members[leadIdx].ID())
	if target != lead {
		t.Fatalf("new leader expected %d, got %d", target, lead)
	}
}

// TestMaintenanceSnapshotCancel ensures that context cancel
// before snapshot reading returns corresponding context errors.
func TestMaintenanceSnapshotCancel(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	// reading snapshot with canceled context should error out
	ctx, cancel := context.WithCancel(t.Context())

	// Since http2 spec defines the receive windows's size and max size of
	// frame in the stream, the underlayer - gRPC client can pre-read data
	// from server even if the application layer hasn't read it yet.
	//
	// And the initialized cluster has 20KiB snapshot, which can be
	// pre-read by underlayer. We should increase the snapshot's size here,
	// just in case that io.Copy won't return the canceled error.
	populateDataIntoCluster(t, clus, 3, 1024*1024)

	rc1, err := clus.RandClient().Snapshot(ctx)
	require.NoError(t, err)
	defer rc1.Close()

	// read 16 bytes to ensure that server opens snapshot
	buf := make([]byte, 16)
	n, err := rc1.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 16, n)

	cancel()
	_, err = io.Copy(io.Discard, rc1)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected %v, got %v", context.Canceled, err)
	}
}

// TestMaintenanceSnapshotFromServerClient verifies that snapshot streams created
// by Member.ServerClient (in-process adapter path) complete successfully.
func TestMaintenanceSnapshotFromServerClient(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	srvClient := clus.Members[0].ServerClient
	require.NotNilf(t, srvClient, "Member.ServerClient must be initialized")

	rc, err := srvClient.Snapshot(t.Context())
	require.NoError(t, err)
	defer rc.Close()

	_, err = io.Copy(io.Discard, rc)
	require.NoErrorf(t, err, "snapshot stream should terminate cleanly")
}

// TestMaintenanceSnapshotWithVersionTimeout ensures that SnapshotWithVersion function
// returns corresponding context errors when context timeout happened before snapshot reading
func TestMaintenanceSnapshotWithVersionTimeout(t *testing.T) {
	testMaintenanceSnapshotTimeout(t, func(ctx context.Context, client *clientv3.Client) (io.ReadCloser, error) {
		resp, err := client.SnapshotWithVersion(ctx)
		if err != nil {
			return nil, err
		}
		return resp.Snapshot, nil
	})
}

// TestMaintenanceSnapshotTimeout ensures that Snapshot function
// returns corresponding context errors when context timeout happened before snapshot reading
func TestMaintenanceSnapshotTimeout(t *testing.T) {
	testMaintenanceSnapshotTimeout(t, func(ctx context.Context, client *clientv3.Client) (io.ReadCloser, error) {
		return client.Snapshot(ctx)
	})
}

// testMaintenanceSnapshotTimeout given snapshot function ensures that it
// returns corresponding context errors when context timeout happened before snapshot reading
func testMaintenanceSnapshotTimeout(t *testing.T, snapshot func(context.Context, *clientv3.Client) (io.ReadCloser, error)) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	// reading snapshot with deadline exceeded should error out
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	// Since http2 spec defines the receive windows's size and max size of
	// frame in the stream, the underlayer - gRPC client can pre-read data
	// from server even if the application layer hasn't read it yet.
	//
	// And the initialized cluster has 20KiB snapshot, which can be
	// pre-read by underlayer. We should increase the snapshot's size here,
	// just in case that io.Copy won't return the timeout error.
	populateDataIntoCluster(t, clus, 3, 1024*1024)

	rc2, err := snapshot(ctx, clus.RandClient())
	require.NoError(t, err)
	defer rc2.Close()

	time.Sleep(2 * time.Second)

	_, err = io.Copy(io.Discard, rc2)
	if IsClientTimeout(err) {
		return
	}
	// Assumes the client receives a single message header and then
	// waits for the payload body. If the context is canceled before
	// the payload arrives, the client will read io.EOF. However, the
	// grpc-go client converts this into io.ErrUnexpectedEOF with an
	// internal error code. Ideally, grpc-go might return context.Canceled
	// instead, but it's unclear if that's feasible. Let's explicitly
	// check for this error in the test code.
	//
	// REF: https://github.com/grpc/grpc-go/blob/6821606f351799b026fda1e6ba143315e6c1e620/rpc_util.go#L644
	//
	// Once https://github.com/grpc/grpc-go/issues/8281 is fixed, we should
	// revert this change. See more discussion in https://github.com/etcd-io/etcd/pull/19833.
	assert.ErrorIs(t, status.Error(codes.Internal, io.ErrUnexpectedEOF.Error()), err)
}

// TestMaintenanceSnapshotWithVersionErrorInflight ensures that ReaderCloser returned by SnapshotWithVersion function
// will fail to read with corresponding context errors on inflight context cancel timeout.
func TestMaintenanceSnapshotWithVersionErrorInflight(t *testing.T) {
	testMaintenanceSnapshotErrorInflight(t, func(ctx context.Context, client *clientv3.Client) (io.ReadCloser, error) {
		resp, err := client.SnapshotWithVersion(ctx)
		if err != nil {
			return nil, err
		}
		return resp.Snapshot, nil
	})
}

// TestMaintenanceSnapshotErrorInflight ensures that ReaderCloser returned by Snapshot function
// will fail to read with corresponding context errors on inflight context cancel timeout.
func TestMaintenanceSnapshotErrorInflight(t *testing.T) {
	testMaintenanceSnapshotErrorInflight(t, func(ctx context.Context, client *clientv3.Client) (io.ReadCloser, error) {
		return client.Snapshot(ctx)
	})
}

// testMaintenanceSnapshotErrorInflight given snapshot function ensures that ReaderCloser returned by it
// will fail to read with corresponding context errors on inflight context cancel timeout.
func testMaintenanceSnapshotErrorInflight(t *testing.T, snapshot func(context.Context, *clientv3.Client) (io.ReadCloser, error)) {
	integration.BeforeTest(t)
	lg := zaptest.NewLogger(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, UseBridge: true})
	defer clus.Terminate(t)

	// take about 1-second to read snapshot
	clus.Members[0].Stop(t)
	dpath := filepath.Join(clus.Members[0].DataDir, "member", "snap", "db")
	b := backend.NewDefaultBackend(lg, dpath)
	s := mvcc.NewStore(lg, b, &lease.FakeLessor{}, mvcc.StoreConfig{CompactionBatchLimit: math.MaxInt32})
	rev := 100000
	for i := 2; i <= rev; i++ {
		s.Put([]byte(fmt.Sprintf("%10d", i)), bytes.Repeat([]byte("a"), 1024), lease.NoLease)
	}
	s.Close()
	b.Close()
	clus.Members[0].Restart(t)

	// reading snapshot with canceled context should error out
	ctx, cancel := context.WithCancel(t.Context())
	rc1, err := snapshot(ctx, clus.RandClient())
	require.NoError(t, err)
	defer rc1.Close()

	donec := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
		close(donec)
	}()
	_, err = io.Copy(io.Discard, rc1)
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("expected %v, got %v", context.Canceled, err)
	}
	<-donec

	// reading snapshot with deadline exceeded should error out
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rc2, err := snapshot(ctx, clus.RandClient())
	require.NoError(t, err)
	defer rc2.Close()

	// 300ms left and expect timeout while snapshot reading is in progress
	time.Sleep(700 * time.Millisecond)
	_, err = io.Copy(io.Discard, rc2)
	if err != nil && !IsClientTimeout(err) {
		t.Errorf("expected client timeout, got %v", err)
	}
}

// TestMaintenanceSnapshotWithVersionVersion ensures that SnapshotWithVersion returns correct version value.
func TestMaintenanceSnapshotWithVersionVersion(t *testing.T) {
	integration.BeforeTest(t)

	// Set SnapshotCount to 1 to force raft snapshot to ensure that storage version is set
	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1, SnapshotCount: 1})
	defer clus.Terminate(t)

	// Put some keys to ensure that wal snapshot is triggered
	for i := 0; i < 10; i++ {
		clus.RandClient().Put(t.Context(), fmt.Sprintf("%d", i), "1")
	}

	// reading snapshot with canceled context should error out
	resp, err := clus.RandClient().SnapshotWithVersion(t.Context())
	require.NoError(t, err)
	defer resp.Snapshot.Close()
	if resp.Version != "3.7.0" {
		t.Errorf("unexpected version, expected %q, got %q", version.Version, resp.Version)
	}
}

func TestMaintenanceSnapshotContentDigest(t *testing.T) {
	integration.BeforeTest(t)

	clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 1})
	defer clus.Terminate(t)

	populateDataIntoCluster(t, clus, 3, 1024*1024)

	// reading snapshot with canceled context should error out
	resp, err := clus.RandClient().SnapshotWithVersion(t.Context())
	require.NoError(t, err)
	defer resp.Snapshot.Close()

	tmpDir := t.TempDir()
	snapFile, err := os.Create(filepath.Join(tmpDir, t.Name()))
	require.NoError(t, err)
	defer snapFile.Close()

	snapSize, err := io.Copy(snapFile, resp.Snapshot)
	require.NoError(t, err)

	// read the checksum
	checksumSize := int64(sha256.Size)
	_, err = snapFile.Seek(-checksumSize, io.SeekEnd)
	require.NoError(t, err)

	checksumInBytes, err := io.ReadAll(snapFile)
	require.NoError(t, err)
	require.Len(t, checksumInBytes, int(checksumSize))

	// remove the checksum part and rehash
	err = snapFile.Truncate(snapSize - checksumSize)
	require.NoError(t, err)

	_, err = snapFile.Seek(0, io.SeekStart)
	require.NoError(t, err)

	hashWriter := sha256.New()
	_, err = io.Copy(hashWriter, snapFile)
	require.NoError(t, err)

	// compare the checksum
	actualChecksum := hashWriter.Sum(nil)
	require.Equal(t, checksumInBytes, actualChecksum)
}

// TestMaintenanceSnapshotDefragmented verifies that when the
// DefragmentedSnapshot server feature gate is enabled, the Snapshot RPC
// returns a defragmented bbolt blob that is (a) smaller than the raw
// snapshot taken from a DB with the same content but freed pages and
// (b) still a valid bbolt database.
func TestMaintenanceSnapshotDefragmented(t *testing.T) {
	integration.BeforeTest(t)

	rawSize := snapshotSizeWithFeatureGates(t, "")
	defragSize := snapshotSizeWithFeatureGates(t, "DefragmentedSnapshot=true")

	if defragSize >= rawSize {
		t.Errorf("defragmented snapshot size = %d, want < raw size = %d", defragSize, rawSize)
	}
}

// snapshotSizeWithFeatureGates spins up a single-member cluster with the
// given feature-gate string, populates it and then deletes most keys (to
// create freed pages), takes a Snapshot, writes it to disk, verifies it is
// a valid bbolt DB, and returns the on-wire size of the snapshot body.
func snapshotSizeWithFeatureGates(t *testing.T, featureGates string) int64 {
	t.Helper()
	clus := integration.NewCluster(t, &integration.ClusterConfig{
		Size:               1,
		ServerFeatureGates: featureGates,
	})
	defer clus.Terminate(t)

	cli := clus.RandClient()
	ctx := t.Context()

	populateDataIntoCluster(t, clus, 200, 64*1024)
	for i := 0; i < 180; i++ {
		_, err := cli.Delete(ctx, fmt.Sprintf("%s-%v", t.Name(), i))
		require.NoError(t, err)
	}
	// mvcc-compact (but do NOT physically defragment) so the bbolt file has
	// tombstoned key/value pages that bbolt has freed but not reclaimed —
	// exactly the state the DefragmentedSnapshot feature gate is meant to
	// make smaller.
	getResp, err := cli.Get(ctx, "anykey")
	require.NoError(t, err)
	_, err = cli.Compact(ctx, getResp.Header.Revision)
	require.NoError(t, err)

	resp, err := cli.SnapshotWithVersion(ctx)
	require.NoError(t, err)
	defer resp.Snapshot.Close()

	f, err := os.CreateTemp(t.TempDir(), "snap_*.db")
	require.NoError(t, err)
	defer f.Close()

	n, err := io.Copy(f, resp.Snapshot)
	require.NoError(t, err)

	// Trim the appended sha256 digest so bbolt.Open accepts the file.
	require.NoError(t, f.Truncate(n-int64(sha256.Size)))
	require.NoError(t, f.Sync())

	db, err := bolt.Open(f.Name(), 0o600, &bolt.Options{ReadOnly: true})
	require.NoError(t, err, "snapshot file must be a valid bbolt DB (feature-gates=%q)", featureGates)
	require.NoError(t, db.Close())

	return n - int64(sha256.Size)
}

func TestMaintenanceStatus(t *testing.T) {
	testCases := []struct {
		name          string
		quotaCfg      int64
		expectedQuota int64
	}{
		{
			name:          "0 quota",
			quotaCfg:      0,
			expectedQuota: storage.DefaultQuotaBytes,
		},
		{
			name:          "default quota",
			quotaCfg:      storage.DefaultQuotaBytes,
			expectedQuota: storage.DefaultQuotaBytes,
		},
		{
			name:          "customized quota",
			quotaCfg:      300010002000,
			expectedQuota: 300010002000,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			integration.BeforeTest(t)

			clus := integration.NewCluster(t, &integration.ClusterConfig{Size: 3, QuotaBackendBytes: tc.quotaCfg})
			defer clus.Terminate(t)

			t.Logf("Waiting for leader...")
			clus.WaitLeader(t)
			t.Logf("Leader established.")

			eps := make([]string, 3)
			for i := 0; i < 3; i++ {
				eps[i] = clus.Members[i].GRPCURL
			}

			t.Logf("Creating client...")
			cli, err := integration.NewClient(t, clientv3.Config{Endpoints: eps})
			require.NoError(t, err)
			defer cli.Close()
			t.Logf("Creating client [DONE]")

			prevID, leaderFound := uint64(0), false
			for i := 0; i < 3; i++ {
				resp, err := cli.Status(t.Context(), eps[i])
				require.NoError(t, err)
				t.Logf("Response from %v: %v", i, resp)
				require.Equal(t, tc.expectedQuota, resp.DbSizeQuota)
				if prevID == 0 {
					prevID, leaderFound = resp.Header.MemberId, resp.Header.MemberId == resp.Leader
					continue
				}
				if prevID == resp.Header.MemberId {
					t.Errorf("#%d: status returned duplicate member ID with %016x", i, prevID)
				}
				if leaderFound && resp.Header.MemberId == resp.Leader {
					t.Errorf("#%d: leader already found, but found another %016x", i, resp.Header.MemberId)
				}
				if !leaderFound {
					leaderFound = resp.Header.MemberId == resp.Leader
				}
			}
			if !leaderFound {
				t.Fatal("no leader found")
			}
		})
	}
}

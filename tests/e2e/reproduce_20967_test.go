package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestIssue20967_Upgrade(t *testing.T) {
	e2e.BeforeTest(t)

	snapshotCount := 20

	t.Log("Creating cluster with zombie members...")
	epc := createClustewithZombieMembers(t, snapshotCount)

	t.Log("Upgradeing cluster to the new version...")
	for _, proc := range epc.Procs {
		serverCfg := proc.Config()

		t.Logf("Stopping node: %v", serverCfg.Name)
		require.NoError(t, proc.Stop(), "error closing etcd process (%v)", serverCfg.Name)

		serverCfg.ExecPath = e2e.BinPath
		serverCfg.KeepDataDir = true

		t.Logf("Restarting node in the new version: %v", serverCfg.Name)
		require.NoError(t, proc.Restart(), "error restarting etcd process (%v)", serverCfg.Name)
	}

	t.Log("Cluster upgraded to current version successfully")
	epc.WaitLeader(t)

	t.Log("Verifying zombie members are removed from v3store...")
	require.NoError(t, epc.Stop())
	for _, proc := range epc.Procs {
		members := readMembersFromV3Store(t, proc.Config().DataDirPath)
		require.Lenf(t, members, 3, "expected 3 members in v3store after upgrade, got %v", members)
	}

	t.Log("Restarting cluster")
	require.NoError(t, epc.Start())
	epc.WaitLeader(t)

	t.Log("Writing key/values")
	for i := 0; i < snapshotCount; i++ {
		k, v := fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i)

		require.NoError(t, epc.Procs[i%len(epc.Procs)].Etcdctl(e2e.ClientNonTLS, false, false).Put(k, v))
	}
}

func TestIssue20967_Snapshot(t *testing.T) {
	e2e.BeforeTest(t)

	snapshotCount := 20
	keyCount := snapshotCount

	t.Log("Creating cluster with zombie members...")
	epc := createClustewithZombieMembers(t, snapshotCount)

	lastProc := epc.Procs[2]
	t.Logf("Stopping last member %s", lastProc.Config().Name)
	require.NoError(t, lastProc.Stop())

	epc.WaitLeader(t)

	cli, err := clientv3.New(clientv3.Config{Endpoints: epc.Procs[0].EndpointsGRPC()})
	require.NoError(t, err)
	defer cli.Close()

	t.Log("Writing key/values to trigger snapshot...")
	for i := 0; i < keyCount; i++ {
		k, v := fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i)

		_, err = cli.Put(t.Context(), k, v)
		require.NoErrorf(t, err, "failed to put key %s", k)
	}
	cli.Close()

	t.Log("Restart the first two members to drop pre-snapshot in-memory log entries," +
		"ensuring the last member is forced to recover from the snapshot." +
		"(Note: SnapshotCatchUpEntries in v3.4.x is a fixed value of 5000.)")
	require.NoError(t, epc.Procs[0].Restart())
	require.NoError(t, epc.Procs[1].Restart())
	epc.WaitLeader(t)

	t.Logf("Restarting last member %s with new version", lastProc.Config().Name)
	lastProc.Config().ExecPath = e2e.BinPath
	require.NoError(t, lastProc.Start(), "failed to start member process")

	t.Logf("Verifying last member %s was recovered from snapshot sent by leader", lastProc.Config().Name)
	found := false
	for _, line := range lastProc.Logs().Lines() {
		if strings.Contains(line, "applied snapshot") {
			t.Logf("Found %s", line)
			found = true
			break
		}
	}
	require.True(t, found, "last member did not receive snapshot from leader")

	for i := 0; i < keyCount; i++ {
		k, v := fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i)

		value, err := lastProc.Etcdctl(e2e.ClientNonTLS, false, false).Get(k)
		require.NoErrorf(t, err, "failed to get key %s from rejoined member", k)

		require.Len(t, value.Kvs, 1)
		require.Equal(t, v, string(value.Kvs[0].Value))
	}

	t.Log("Verifying zombie members for last member")
	require.NoError(t, lastProc.Stop())
	members := readMembersFromV3Store(t, lastProc.Config().DataDirPath)
	require.Lenf(t, members, 3, "expected 3 members in v3store after upgrade, got %v", members)
}

// readMembersFromV3Store read all members from the v3store in the given dataDir.
func readMembersFromV3Store(t *testing.T, dataDir string) []membership.Member {
	dbPath := datadir.ToBackendFileName(dataDir)
	db, err := bbolt.Open(dbPath, 0400, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	var members []membership.Member
	_ = db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(buckets.Members.Name()).ForEach(func(_, v []byte) error {
			m := membership.Member{}
			err := json.Unmarshal(v, &m)
			require.NoError(t, err)

			members = append(members, m)
			return nil
		})
	})
	return members
}

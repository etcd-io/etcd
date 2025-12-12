package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/mvcc/buckets"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

// createClustewithZombieMembers creates a cluster with zombie members in v3store.
//
// There are two ways to create zombie members:
//
// 1. Restoring a cluster using a snapshot taken from version <= v3.4.39.
//
//	The `etcdctl snapshot restore` tool in versions <= v3.4.39 contains a bug
//	that fails to clean up old member information during restore. This leaves
//	behind stale (zombie) member entries in the v3store.
//
// 2. Using ForceNewCluster on a cluster running a version < v3.5.22.
//
//	If ETCD commits a ConfChangeAddNode entry to the v3store but crashes before
//	the corresponding hard state is persisted to the WAL, then after restarting
//	with ForceNewCluster, the member entries in the v3store are not cleaned up,
//	resulting in zombie members. This issue was fixed in [v3.5.22][1].
//
//	It may be possible to create zombie member in versions < [v3.4.0][2] through a
//	single-node ForceNewCluster restart. For example, suppose we start with a
//	one-node cluster. We add a second node, but the second node fails to join
//	and the cluster loses quorum. We then restart the first node with ForceNewCluster,
//	which results in a zombie member entry for the second node. However, it
//	is still unlikely for three zombie members (like [issue-20967][3]) to
//	appear at the same time.
//
//	So in theory, this could happen when using ForceNewCluster. However, it is
//	extremely unlikely in practice because multiple members are running, and it
//	would require all of them to panic at the same time. It would also require
//	selecting one of the crashed members to restart with ForceNewCluster, where
//	that member had failed to persist the hard state to the WAL—an improbable
//	combination of events (what a coincidence).
//
// So, in this function, we use the first method to create zombie members for
// testing.
//
// REF:
//
// [1]: https://github.com/etcd-io/etcd/commit/ccf66456e7237ef5251d9fdc96f1624d1aa4daf1
// [2]: https://github.com/etcd-io/etcd/issues/14370
// [3]: https://github.com/etcd-io/etcd/issues/20967
func createClustewithZombieMembers(t *testing.T, snapshotCount int) *e2e.EtcdProcessCluster {
	if !fileutil.Exist(e2e.BinPathLastRelease) {
		t.Skipf("%q does not exist", e2e.BinPathLastRelease)
	}

	t.Log("Creating initial cluster with LastVersion...")
	epc, err := e2e.NewEtcdProcessCluster(t,
		&e2e.EtcdProcessClusterConfig{
			Version:      e2e.LastVersion,
			ClusterSize:  3,
			KeepDataDir:  true,
			RollingStart: true,
			// LogLevel:      "debug",
			SnapshotCount: snapshotCount,
		})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, epc.Close())
	})

	t.Log("Fetching snapshot from cluster")
	snapFile := fetchSnapshotFromMember(t, e2e.CtlBinPathV3439Release, epc.Procs[0])

	t.Log("Stopping all members")
	require.NoError(t, epc.Stop())

	t.Log("Restoring snapshot using etcdctl v3.4.39 to create zombie members...")
	for _, proc := range epc.Procs {
		serverCfg := proc.Config()

		require.NoError(t, restoreSnapshotIntoMemberDir(t, e2e.CtlBinPathV3439Release, snapFile, serverCfg.InitialCluster, proc),
			"failed to restore snapshot for member %s", serverCfg.Name)
	}

	t.Log("Restarting all members from restored data dirs...")
	require.NoError(t, epc.RollingStart())

	t.Log("Ensuring zombie members exist in v3store...")
	require.NoError(t, epc.Stop())
	for _, proc := range epc.Procs {
		ensureZombieMembers(t, proc.Config().DataDirPath)
	}
	require.NoError(t, epc.RollingStart())
	epc.WaitLeader(t)
	return epc
}

// ensureZombieMembers checks that there are 6 members in the v3store.
func ensureZombieMembers(t *testing.T, dataDir string) {
	dbPath := datadir.ToBackendFileName(dataDir)
	db, err := bbolt.Open(dbPath, 0400, &bbolt.Options{ReadOnly: true})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	count := 0
	members := map[string]int{}
	_ = db.View(func(tx *bbolt.Tx) error {
		return tx.Bucket(buckets.Members.Name()).ForEach(func(_, v []byte) error {
			m := membership.Member{}
			err := json.Unmarshal(v, &m)
			require.NoError(t, err)

			key := fmt.Sprintf("name=%s,peerURLs=%v,isLearner=%v", m.Name, m.PeerURLs, m.IsLearner)
			members[key]++

			count++
			return nil
		})
	})
	require.Equal(t, 6, count, "expected 6 members in v3store")
	require.Equal(t, 3, len(members), "expected 3 member names in v3store")
}

// restoreSnapshotIntoMemberDir restores snapshot into the given member's data dir.
func restoreSnapshotIntoMemberDir(t *testing.T, etcdctlBin string, snapshotPath string, initialCluster string, proc e2e.EtcdProcess) error {
	serverCfg := proc.Config()
	require.NoError(t, os.RemoveAll(serverCfg.DataDirPath), "failed to remove data dir %s", serverCfg.DataDirPath)

	serverCfg.DataDirPath = filepath.Join(t.TempDir(), "restored-"+serverCfg.Name)
	updateServerCfgArgs(serverCfg, "--data-dir", serverCfg.DataDirPath)

	serverCfg.InitialCluster = initialCluster
	updateServerCfgArgs(serverCfg, "--initial-cluster", initialCluster)

	// We can use AddMember to create unique member ID with same cluter name.
	// However, it takes long time to wait for the cluster to be healthy.
	// So, we simply update the initial cluster configuration with different
	// cluster name here. This achieves the same effect for testing zombie
	// members.
	//
	// NOTE: If we use same cluster name without using AddMember before,
	// etcd will generate same member ID and it won't create zombie members.
	serverCfg.InitialToken = "restored"
	updateServerCfgArgs(serverCfg, "--initial-cluster-token", serverCfg.InitialToken)

	ctl := proc.Etcdctl(e2e.ClientNonTLS, false, false)
	args := ctl.GenerateCmdArgs(
		"snapshot", "restore", snapshotPath,
		"--data-dir", serverCfg.DataDirPath,
		"--name", serverCfg.Name,
		"--initial-cluster", serverCfg.InitialCluster,
		"--initial-cluster-token", serverCfg.InitialToken,
		"--initial-advertise-peer-urls", serverCfg.Purl.String(),
	)
	args[0] = etcdctlBin

	t.Logf("Restoring command args: %v", args)
	return e2e.SpawnWithExpectWithEnv(args, ctl.Envs(), "restored snapshot")
}

// fetchSnapshotFromMember fetches snapshot from the given member and returnes
// the snapshot file path.
func fetchSnapshotFromMember(t *testing.T, etcdctlBin string, proc e2e.EtcdProcess) string {
	fPath := filepath.Join(t.TempDir(), "snapshot.db")

	ctl := proc.Etcdctl(e2e.ClientNonTLS, false, false)

	args := ctl.GenerateCmdArgs("snapshot", "save", fPath)
	args[0] = etcdctlBin

	require.NoError(t,
		e2e.SpawnWithExpectWithEnv(
			args,
			ctl.Envs(),
			fmt.Sprintf("Snapshot saved at %s", fPath),
		),
	)
	return fPath
}

// updateServerCfgArgs updates the given argKey to argValue in serverCfg.
func updateServerCfgArgs(serverCfg *e2e.EtcdServerProcessConfig, argKey, argValue string) {
	for i := range serverCfg.Args {
		if serverCfg.Args[i] == argKey {
			serverCfg.Args[i+1] = argValue
			return
		}
	}
}

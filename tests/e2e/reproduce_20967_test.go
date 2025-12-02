package e2e

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/framework/e2e"
)

func TestIssue20967_SnapshotRestore(t *testing.T) {
	e2e.BeforeTest(t)

	v3414 := downloadReleaseBinary(t, "v3.4.14", "etcd")
	v3414ctl := downloadReleaseBinary(t, "v3.4.14", "etcdctl")

	snapshotCount := 10

	epc, err := e2e.NewEtcdProcessCluster(t,
		&e2e.EtcdProcessClusterConfig{
			ExecPath:      v3414,
			ClusterSize:   1,
			KeepDataDir:   true,
			RollingStart:  true,
			LogLevel:      "debug",
			IsPeerTLS:     false,
			SnapshotCount: snapshotCount,
		})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	t.Log("Add new member")
	_, err = epc.StartNewProc(nil, false, t)
	require.NoError(t, err)

	t.Log("Waiting 5 seconds for cluster health...")
	time.Sleep(5 * time.Second)

	t.Log("Add new member")
	_, err = epc.StartNewProc(nil, false, t)
	require.NoError(t, err)

	t.Logf("Put some data to trigger snapshot (snapshot count: %d)", snapshotCount)
	for range snapshotCount {
		require.NoError(t, epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false).Put("foo", strings.Repeat("Oops", 1)))
	}

	t.Log("Take snapshot on test-0")
	snapshotPath := filepath.Join(t.TempDir(), "snapshot.db")
	err = epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false).WithBinPath(v3414ctl).SnapshotSave(snapshotPath)
	require.NoError(t, err)

	t.Log("Somehow, all members exit unexpectedly...")
	require.NoError(t, epc.Stop())

	t.Logf("Restoring snapshot from %s for test-1", snapshotPath)
	proc := epc.Procs[1]
	serverCfg := proc.Config()

	restoreDir := filepath.Join(t.TempDir(), "restore-data")
	err = proc.Etcdctl(e2e.ClientNonTLS, false, false).WithBinPath(v3414ctl).
		SnapshotRestore(snapshotPath,
			restoreDir,
			serverCfg.Name,
			fmt.Sprintf("%s=%s", serverCfg.Name, serverCfg.Purl.String()),
			serverCfg.InitialToken,
			serverCfg.Purl.String())
	require.NoError(t, err)

	t.Logf("Restart %s from restored data dir", serverCfg.Name)
	serverCfg.DataDirPath = restoreDir
	serverCfg.Args = append(serverCfg.Args, fmt.Sprintf("--data-dir=%s", restoreDir))
	epc.Cfg.SetInitialCluster(serverCfg, []string{fmt.Sprintf("%s=%s", serverCfg.Name, serverCfg.Purl.String())}, "existing")
	require.NoError(t, proc.Start())

	t.Logf("Wait for %s to be healthy...", serverCfg.Name)
	require.NoError(t, proc.Etcdctl(e2e.ClientNonTLS, false, false).Put("foo", strings.Repeat("Oops", 1)))

	proc0 := epc.Procs[0]
	serverCfgTest0 := proc0.Config()

	t.Logf("Removing member %s data dir %s", serverCfgTest0.Name, serverCfgTest0.DataDirPath)
	if err := os.RemoveAll(serverCfgTest0.DataDirPath); err != nil {
		t.Fatalf("failed to remove data dir %s: %v", serverCfgTest0.DataDirPath, err)
	}

	serverCfgTest0.DataDirPath = t.TempDir()
	serverCfgTest0.Args = append(serverCfgTest0.Args, "--data-dir", serverCfgTest0.DataDirPath)
	initialCluster := []string{
		fmt.Sprintf("%s=%s", serverCfg.Name, serverCfg.Purl.String()),
		fmt.Sprintf("%s=%s", serverCfgTest0.Name, serverCfgTest0.Purl.String()),
	}
	epc.Cfg.SetInitialCluster(serverCfgTest0, initialCluster, "existing")

	t.Logf("Adding member %s back to cluster...", serverCfgTest0.Name)
	_, err = proc.Etcdctl(e2e.ClientNonTLS, false, false).MemberAdd(serverCfgTest0.Name, []string{serverCfgTest0.Purl.String()})
	require.NoError(t, err)

	require.NoError(t, proc0.Start())

	t.Logf("Wait for %s to be healthy...", serverCfgTest0.Name)
	require.NoError(t, proc0.Etcdctl(e2e.ClientNonTLS, false, false).Put("foo", strings.Repeat("Oops", 1)))

	memberList, err := proc0.Etcdctl(e2e.ClientNonTLS, false, false).MemberList()
	require.NoError(t, err)

	require.Equal(t, 2, len(memberList.Members))
	for _, m := range memberList.Members {
		t.Logf("Member ID: %d, Name: %s, PeerURLs: %v, ClientURLs: %v", m.ID, m.Name, m.PeerURLs, m.ClientURLs)
	}

	t.Log("Both members are healthy now.")

	t.Logf("Checking v3store consistency...")
	require.NoError(t, epc.Close())

	data, err := exec.Command(filepath.Join(e2e.BinDir, "tools", "etcd-dump-db"), "iterate-bucket", proc.Config().DataDirPath, "members").CombinedOutput()
	require.NoError(t, err)

	t.Logf("etcd-dump-db output:\n%s", string(data))
	/*
		t.Logf("Collect data dirs into a temp dir for investigation...")
		dir, err := os.MkdirTemp("", "issue20967-")
		require.NoError(t, err)

		for _, proc := range epc.Procs {
			targetName := filepath.Join(dir, proc.Config().Name)
			sourceDir := proc.Config().DataDirPath

			t.Logf("Copy data dir from %s to %s", sourceDir, targetName)
			require.NoError(t, os.Rename(sourceDir, targetName))
		}
	*/
}

func TestIssue20967_ByForceNewCluster(t *testing.T) {
	t.Skip("working on it")

	e2e.BeforeTest(t)

	snapshotCount := 10
	v3414 := downloadReleaseBinary(t, "v3.4.14", "etcd")
	epc, err := e2e.NewEtcdProcessCluster(t,
		&e2e.EtcdProcessClusterConfig{
			ExecPath:     v3414,
			ClusterSize:  3,
			KeepDataDir:  true,
			RollingStart: true,
			// LogLevel:      "debug",
			SnapshotCount: snapshotCount,
			IsPeerTLS:     false,
		})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, epc.Close())
	}()

	/*
				   term         index      type    data
		   1             1      conf    method=ConfChangeAddNode id=439f8dbfa7ae526d
		   1             2      conf    method=ConfChangeAddNode id=496ffb2d732c3aaf
		   1             3      conf    method=ConfChangeAddNode id=7d996272b6b7e23f
		   2             4      norm
		   2             5      norm    method=PUT path="/0/members/439f8dbfa7ae526d/attributes" val="{\"name\":\"test-0\",\"clientURLs\":[\"http://localhost:20000\"]}"
		   2             6      norm    method=PUT path="/0/members/496ffb2d732c3aaf/attributes" val="{\"name\":\"test-1\",\"clientURLs\":[\"http://localhost:20005\"]}"
		   2             7      norm    method=PUT path="/0/version" val="3.0.0"
		   2             8      norm    method=PUT path="/0/members/7d996272b6b7e23f/attributes" val="{\"name\":\"test-2\",\"clientURLs\":[\"http://localhost:20010\"]}"
		   2             9      norm    header:<ID:5939573835183217413 > put:<key:"foo" value:"Oops" >

	*/
	require.NoError(t, epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false).Put("foo", strings.Repeat("Oops", 1)))

	t.Log("Waiting 5 seconds for cluster health...")
	time.Sleep(5 * time.Second)

	t.Log("Cluster will trigger snapshot when we delete member test-0")
	require.NoError(t, epc.RejoinMember(t, 0))

	require.NoError(t, epc.Procs[0].Etcdctl(e2e.ClientNonTLS, false, false).Put("foo", strings.Repeat("Oops", 1)))

	t.Log("Somehow, first two members exits unexpectedly...")
	for i := 0; i < 2; i++ {
		require.NoError(t, epc.Procs[i].Stop())
	}

	t.Log("Unable to rejoin deleted member test-0, so we force new cluster on test-2...")
	require.NoError(t, epc.ForceNewCluster(t, 2))

	t.Logf("Collect data dirs into a temp dir for investigation...")
	dir, err := os.MkdirTemp("", "issue20967-")
	require.NoError(t, err)

	require.NoError(t, epc.Close())

	for _, proc := range epc.Procs {
		targetName := filepath.Join(dir, proc.Config().Name)
		sourceDir := proc.Config().DataDirPath

		t.Logf("Copy data dir from %s to %s", sourceDir, targetName)
		require.NoError(t, os.Rename(sourceDir, targetName))
	}

}

func downloadReleaseBinary(t *testing.T, ver string, binaryName string) string {
	arch := runtime.GOARCH
	if arch == "386" {
		arch = "amd64"
	}
	targetURL := fmt.Sprintf("https://github.com/etcd-io/etcd/releases/download/%s/etcd-%s-%s-%s.tar.gz",
		ver, ver, runtime.GOOS, arch,
	)

	transport := http.DefaultTransport.(*http.Transport).Clone()
	// NOTE: avoid background goroutine and pass leaked goroutine check
	transport.DisableKeepAlives = true
	cli := &http.Client{Transport: transport}

	resp, err := cli.Get(targetURL)
	require.NoError(t, err, "failed to http-get %s", targetURL)

	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	gzReader, err := gzip.NewReader(resp.Body)
	require.NoError(t, err, "%s should be gzip stream", targetURL)
	defer gzReader.Close()

	targetBinaryPath := filepath.Join(t.TempDir(), "etcd")

	tr := tar.NewReader(gzReader)
	for {
		hdrInfo, err := tr.Next()
		if err != nil {
			require.Equal(t, io.EOF, err)
			break
		}

		switch hdrInfo.Typeflag {
		case tar.TypeReg, tar.TypeRegA:
		default:
			continue
		}

		if filepath.Base(hdrInfo.Name) != binaryName {
			continue
		}

		f, err := os.OpenFile(targetBinaryPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdrInfo.Mode))
		require.NoError(t, err, "failed to open file %s", targetBinaryPath)
		defer f.Close()

		err = os.Chmod(targetBinaryPath, os.FileMode(hdrInfo.Mode))
		require.NoError(t, err, "failed to chmod %s", targetBinaryPath)

		_, err = io.Copy(f, io.Reader(tr))
		require.NoError(t, err, "failed to copy data into %s", targetBinaryPath)

		require.NoError(t, f.Sync(), "failed to sync data into %s", targetBinaryPath)

		return targetBinaryPath
	}
	t.Fatalf("failed to get etcd binary from %s", targetURL)
	return "" // unreachable
}

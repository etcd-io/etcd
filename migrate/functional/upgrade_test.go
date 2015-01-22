package functional

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
)

var (
	binDir         = ".versions"
	v1BinPath      = path.Join(binDir, "1")
	v2BinPath      = path.Join(binDir, "2")
	etcdctlBinPath string
)

func init() {
	os.RemoveAll(binDir)
	if err := os.Mkdir(binDir, 0700); err != nil {
		fmt.Printf("unexpected Mkdir error: %v\n", err)
		os.Exit(1)
	}
	if err := os.Symlink(absPathFromEnv("ETCD_V1_BIN"), v1BinPath); err != nil {
		fmt.Printf("unexpected Symlink error: %v\n", err)
		os.Exit(1)
	}
	if err := os.Symlink(absPathFromEnv("ETCD_V2_BIN"), v2BinPath); err != nil {
		fmt.Printf("unexpected Symlink error: %v\n", err)
		os.Exit(1)
	}
	etcdctlBinPath = os.Getenv("ETCDCTL_BIN")

	mustExist(v1BinPath)
	mustExist(v2BinPath)
	mustExist(etcdctlBinPath)
}

func TestStartSingleNewETCDUsingDefaultFlags(t *testing.T) {
	i := NewDefaultInstance(v2BinPath)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "2" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartSingleNewETCDUsingV1Flags(t *testing.T) {
	i := NewDefaultInstance(v2BinPath)
	i.SetV1PeerAddr("127.0.0.1:7002")
	if err := i.Start(); err == nil {
		t.Errorf("Start error = %v, want nil", err)
	}
	defer i.Terminate()
}

func TestStartSingleNewETCDUsingV2Flags(t *testing.T) {
	i := NewDefaultInstance(v2BinPath)
	i.SetV2PeerURL("http://127.0.0.1:7002")
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "2" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartSingleV2ETCDUsingDefaultFlags(t *testing.T) {
	// get v2 data dir
	i := NewDefaultInstance(v2BinPath)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	i.Stop()

	i = i.Clone(v2BinPath)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "2" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartSingleV1ETCDUsingV1Flags(t *testing.T) {
	// get v1 data dir
	i := NewDefaultInstance(v1BinPath)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	i.Stop()

	i = i.Clone(v2BinPath)
	i.SetV1PeerAddr("127.0.0.1:7001")
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "1" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartV2DesiredSingleV1ETCDUsingV1Flags(t *testing.T) {
	// get v2-desired v1 data dir
	i := NewDefaultInstance(v1BinPath)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	cmd := exec.Command(etcdctlBinPath, "upgrade", "--peer-url", i.PeerURL)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	t.Logf("wait until etcd exits...")
	if err := i.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}

	i = i.Clone(v2BinPath)
	i.SetV1PeerAddr("127.0.0.1:7001")
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "2" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartV2DesiredSingleV1ETCDWithSnapshotUsingV1Flags(t *testing.T) {
	// get v2-desired v1 data dir
	i := NewDefaultInstance(v1BinPath)
	i.SetSnapCount(10)
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	cmd := exec.Command(etcdctlBinPath, "upgrade", "--peer-url", i.PeerURL)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	t.Logf("wait until etcd exits...")
	if err := i.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	// check it has taken snapshot
	fis, err := ioutil.ReadDir(path.Join(i.DataDir, "snapshot"))
	if err != nil {
		t.Fatalf("unexpected ReadDir error: %v", err)
	}
	if len(fis) == 0 {
		t.Fatalf("unexpected no-snapshot data dir")
	}

	i = i.Clone(v2BinPath)
	i.SetV1PeerAddr("127.0.0.1:7001")
	if err := i.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer i.Terminate()

	ver, err := checkInternalVersion(i.URL)
	if err != nil {
		t.Fatalf("checkVersion error: %v", err)
	}
	if ver != "2" {
		t.Errorf("internal version = %s, want %s", ver, "2")
	}
}

func TestStartMultiV1ETCDUsingV1Flags(t *testing.T) {
	ins := NewDefaultV1Instances(v1BinPath, 3)
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	ins.Stop()

	ins = ins.Clone(v2BinPath)
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer ins.Terminate()

	for _, i := range ins {
		ver, err := checkInternalVersion(i.URL)
		if err != nil {
			t.Fatalf("checkVersion error: %v", err)
		}
		if ver != "1" {
			t.Errorf("internal version = %s, want %s", ver, "1")
		}
	}
}

func TestStartV2DesiredMultiV1ETCDUsingV1Flags(t *testing.T) {
	ins := NewDefaultV1Instances(v1BinPath, 3)
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	cmd := exec.Command(etcdctlBinPath, "upgrade", "--peer-url", ins[1].PeerURL)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
	t.Logf("wait until etcd exits...")
	if err := ins.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}

	ins = ins.Clone(v2BinPath)
	ins.CleanUnsuppportedV1Flags()
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer ins.Terminate()

	for _, i := range ins {
		ver, err := checkInternalVersion(i.URL)
		if err != nil {
			t.Fatalf("checkVersion error: %v", err)
		}
		if ver != "2" {
			t.Errorf("internal version = %s, want %s", ver, "2")
		}
	}
}

func TestStartV1JoinedNewETCDUsingV1Flags(t *testing.T) {
	ins := NewDefaultV1Instances(v1BinPath, 3)
	ins[1] = ins[1].Clone(v2BinPath)
	ins[2] = ins[2].Clone(v2BinPath)
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer ins.Terminate()

	for _, i := range ins {
		ver, err := checkInternalVersion(i.URL)
		if err != nil {
			t.Fatalf("checkVersion error: %v", err)
		}
		if ver != "1" {
			t.Errorf("internal version = %s, want %s", ver, "1")
		}
	}
}

func TestStartV1JoinedNewETCDUsingV1FlagsThroughDiscovery(t *testing.T) {
	di := NewDefaultInstance(v1BinPath)
	di.SetV1Addr("127.0.0.1:5001")
	di.SetV1PeerAddr("127.0.0.1:8001")
	if err := di.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer di.Terminate()

	ins := NewDefaultV1InstancesThroughDiscovery(v1BinPath, 3, "http://127.0.0.1:5001/v2/keys/cluster/")
	ins[1] = ins[1].Clone(v2BinPath)
	ins[2] = ins[2].Clone(v2BinPath)
	if err := ins.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer ins.Terminate()

	for _, i := range ins {
		ver, err := checkInternalVersion(i.URL)
		if err != nil {
			t.Fatalf("checkVersion error: %v", err)
		}
		if ver != "1" {
			t.Errorf("internal version = %s, want %s", ver, "1")
		}
	}
}

func absPathFromEnv(name string) string {
	path, err := filepath.Abs(os.Getenv(name))
	if err != nil {
		fmt.Printf("unexpected Abs error: %v\n", err)
	}
	return path
}

func mustExist(path string) {
	if _, err := os.Stat(path); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}

func checkInternalVersion(url string) (string, error) {
	resp, err := http.Get(url + "/version")
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var m map[string]string
	err = json.Unmarshal(b, &m)
	return m["internalVersion"], err
}

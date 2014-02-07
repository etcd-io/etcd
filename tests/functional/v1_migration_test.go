package test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensure that we can start a v2 node from the log of a v1 node.
func TestV1SoloMigration(t *testing.T) {
	path, _ := ioutil.TempDir("", "etcd-")
	os.MkdirAll(path, 0777)
	defer os.RemoveAll(path)

	nodepath := filepath.Join(path, "node0")
	fixturepath, _ := filepath.Abs("../fixtures/v1.solo/node0")
	fmt.Println("DATA_DIR =", nodepath)

	// Copy over fixture files.
	c := exec.Command("cp", "-rf", fixturepath, nodepath)
	if out, err := c.CombinedOutput(); err != nil {
		fmt.Println(">>>>>>\n", string(out), "<<<<<<")
		panic("Fixture initialization error:" + err.Error())
	}

	procAttr := new(os.ProcAttr)
	procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

	args := []string{"etcd", fmt.Sprintf("-data-dir=%s", nodepath)}
	args = append(args, "-addr", "127.0.0.1:4001")
	args = append(args, "-peer-addr", "127.0.0.1:7001")
	args = append(args, "-name", "v1")
	process, err := os.StartProcess(EtcdBinPath, args, procAttr)
	if err != nil {
		t.Fatal("start process failed:" + err.Error())
		return
	}
	defer process.Kill()
	time.Sleep(time.Second)

	// Ensure deleted message is removed.
	resp, err := tests.Get("http://localhost:4001/v2/keys/message")
	tests.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 200, "")
}

// Ensure that we can start a v2 cluster from the logs of a v1 cluster.
func TestV1ClusterMigration(t *testing.T) {
	path, _ := ioutil.TempDir("", "etcd-")
	os.RemoveAll(path)
	defer os.RemoveAll(path)

	nodes := []string{"node0", "node2"}
	for i, node := range nodes {
		nodepath := filepath.Join(path, node)
		fixturepath, _ := filepath.Abs(filepath.Join("../fixtures/v1.cluster/", node))
		fmt.Println("FIXPATH  =", fixturepath)
		fmt.Println("NODEPATH =", nodepath)
		os.MkdirAll(filepath.Dir(nodepath), 0777)

		// Copy over fixture files.
		c := exec.Command("cp", "-rf", fixturepath, nodepath)
		if out, err := c.CombinedOutput(); err != nil {
			fmt.Println(">>>>>>\n", string(out), "<<<<<<")
			panic("Fixture initialization error:" + err.Error())
		}

		procAttr := new(os.ProcAttr)
		procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

		args := []string{"etcd", fmt.Sprintf("-data-dir=%s", nodepath)}
		args = append(args, "-addr", fmt.Sprintf("127.0.0.1:%d", 4001+i))
		args = append(args, "-peer-addr", fmt.Sprintf("127.0.0.1:%d", 7001+i))
		args = append(args, "-name", node)
		process, err := os.StartProcess(EtcdBinPath, args, procAttr)
		if err != nil {
			t.Fatal("start process failed:" + err.Error())
			return
		}
		defer process.Kill()
		time.Sleep(time.Second)
	}

	// Ensure deleted message is removed.
	resp, err := tests.Get("http://localhost:4001/v2/keys/message")
	body := tests.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	assert.Equal(t, string(body), `{"errorCode":100,"message":"Key not found","cause":"/message","index":11}`+"\n")

	// Ensure TTL'd message is removed.
	resp, err = tests.Get("http://localhost:4001/v2/keys/foo")
	body = tests.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 200, "")
	assert.Equal(t, string(body), `{"action":"get","node":{"key":"/foo","value":"one","modifiedIndex":9,"createdIndex":9}}`)
}

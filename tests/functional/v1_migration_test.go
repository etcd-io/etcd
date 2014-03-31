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

	etcdtest "github.com/coreos/etcd/tests"
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

	i := etcdtest.NewOldInstance()
	i.Conf.DataDir = nodepath
	i.Conf.Addr = "127.0.0.1:4001"
	i.Conf.Peer.Addr = "127.0.0.1:7001"
	i.Conf.Name = "node0"
	if err := i.Start(); err != nil {
		t.Fatal("cannot start etcd")
	}
	defer i.Stop()
	time.Sleep(time.Second)

	// Ensure deleted message is removed.
	resp, err := etcdtest.Get("http://localhost:4001/v2/keys/message")
	etcdtest.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 200, "")
}

// Ensure that we can start a v2 cluster from the logs of a v1 cluster.
func TestV1ClusterMigration(t *testing.T) {
	path, _ := ioutil.TempDir("", "etcd-")
	os.RemoveAll(path)
	defer os.RemoveAll(path)

	nodes := []string{"node0", "node2"}
	for idx, node := range nodes {
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

		i := etcdtest.NewOldInstance()
		i.Conf.DataDir = nodepath
		i.Conf.Addr = fmt.Sprintf("127.0.0.1:%d", 4001+idx)
		i.Conf.Peer.Addr = fmt.Sprintf("127.0.0.1:%d", 7001+idx)
		i.Conf.Name = node
		if err := i.Start(); err != nil {
			t.Fatal("cannot start etcd")
		}
		defer i.Stop()
		time.Sleep(time.Second)
	}

	// Ensure deleted message is removed.
	resp, err := etcdtest.Get("http://localhost:4001/v2/keys/message")
	body := etcdtest.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
	assert.Equal(t, string(body), `{"errorCode":100,"message":"Key not found","cause":"/message","index":11}`+"\n")

	// Ensure TTL'd message is removed.
	resp, err = etcdtest.Get("http://localhost:4001/v2/keys/foo")
	body = etcdtest.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 200, "")
	assert.Equal(t, string(body), `{"action":"get","node":{"key":"/foo","value":"one","modifiedIndex":9,"createdIndex":9}}`)
}

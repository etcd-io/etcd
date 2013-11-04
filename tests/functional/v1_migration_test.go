package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/etcd/tests"
	"github.com/stretchr/testify/assert"
)

// Ensure that we can start a v2 node from the log of a v1 node.
func TestV1Migration(t *testing.T) {
	path, _ := ioutil.TempDir("", "etcd-")
	os.RemoveAll(path)
	defer os.RemoveAll(path)

	nodes := []string{"node0", "node1"}
	for i, node := range nodes {
		nodepath := filepath.Join(path, node)

		// Copy over fixture files.
		if err := exec.Command("cp", "-r", "../fixtures/v1/" + node, nodepath).Run(); err != nil {
			panic("Fixture initialization error")
		}

		procAttr := new(os.ProcAttr)
		procAttr.Files = []*os.File{nil, os.Stdout, os.Stderr}

		args := []string{"etcd", fmt.Sprintf("-d=%s", nodepath)}
		args = append(args, "-c", fmt.Sprintf("127.0.0.1:%d", 4001 + i))
		args = append(args, "-s", fmt.Sprintf("127.0.0.1:%d", 7001 + i))
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
	tests.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 404, "")

	// Ensure TTL'd message is removed.
	resp, err = tests.Get("http://localhost:4001/v2/keys/foo")
	tests.ReadBody(resp)
	assert.Nil(t, err, "")
	assert.Equal(t, resp.StatusCode, 404, "")
}

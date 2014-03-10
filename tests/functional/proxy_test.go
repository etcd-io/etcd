package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Create a full cluster and then add extra an extra proxy node.
func TestProxy(t *testing.T) {
	clusterSize := 10 // DefaultActiveSize + 1
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	assert.NoError(t, err)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	c := etcd.NewClient(nil)
	c.SyncCluster()

	// Set key.
	time.Sleep(time.Second)
	if _, err := c.Set("foo", "bar", 0); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)

	// Check that all peers and proxies have the value.
	for i, _ := range etcds {
		resp, err := tests.Get(fmt.Sprintf("http://localhost:%d/v2/keys/foo", 4000+(i+1)))
		if assert.NoError(t, err) {
			body := tests.ReadBodyJSON(resp)
			if node, _ := body["node"].(map[string]interface{}); assert.NotNil(t, node) {
				assert.Equal(t, node["value"], "bar")
			}
		}
	}

	// Verify that we have one proxy.
	result, err := c.Get("_etcd/proxies", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 1)

	// Reconfigure with larger active size (10 nodes) and wait for promotion.
	resp, _ := tests.Put("http://localhost:7001/config", "application/json", bytes.NewBufferString(`{"activeSize":10, "promoteDelay":1800}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	time.Sleep(server.ActiveMonitorTimeout + (1 * time.Second))

	// Verify that the proxy node is now a peer.
	result, err = c.Get("_etcd/proxies", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 0)

	// Reconfigure with a smaller active size (8 nodes).
	resp, _ = tests.Put("http://localhost:7001/config", "application/json", bytes.NewBufferString(`{"activeSize":8, "promoteDelay":1800}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Wait for two monitor cycles before checking for demotion.
	time.Sleep((2 * server.ActiveMonitorTimeout) + (1 * time.Second))

	// Verify that we now have eight peers.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 8)

	// Verify that we now have two proxies.
	result, err = c.Get("_etcd/proxies", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 2)
}

// Create a full cluster, disconnect a peer, wait for autodemotion, wait for autopromotion.
func TestProxyAutoPromote(t *testing.T) {
	clusterSize := 10 // DefaultActiveSize + 1
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer func() {
		// Wrap this in a closure so that it picks up the updated version of
		// the "etcds" variable.
		DestroyCluster(etcds)
	}()

	c := etcd.NewClient(nil)
	c.SyncCluster()

	time.Sleep(1 * time.Second)

	// Verify that we have one proxy.
	result, err := c.Get("_etcd/proxies", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 1)

	// Reconfigure with a short promote delay (2 second).
	resp, _ := tests.Put("http://localhost:7001/config", "application/json", bytes.NewBufferString(`{"activeSize":9, "promoteDelay":2}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Remove peer.
	etcd := etcds[1]
	etcds = append(etcds[:1], etcds[2:]...)
	if err := etcd.Kill(); err != nil {
		panic(err.Error())
	}
	etcd.Release()

	// Wait for it to get dropped.
	time.Sleep(server.PeerActivityMonitorTimeout + (2 * time.Second))

	// Wait for the proxy to be promoted.
	time.Sleep(server.ActiveMonitorTimeout + (2 * time.Second))

	// Verify that we have 9 peers.
	result, err = c.Get("_etcd/machines", true, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 9)

	// Verify that node10 is one of those peers.
	result, err = c.Get("_etcd/machines/node10", false, false)
	assert.NoError(t, err)

	// Verify that there are no more proxies.
	result, err = c.Get("_etcd/proxies", false, true)
	assert.NoError(t, err)
	if assert.Equal(t, len(result.Node.Nodes), 1) {
		assert.Equal(t, result.Node.Nodes[0].Key, "/_etcd/proxies/node2")
	}
}

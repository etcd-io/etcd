package test

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/server"
	"github.com/coreos/etcd/store"
	"github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/coreos/go-etcd/etcd"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Create a full cluster and then change the active size.
func TestStandby(t *testing.T) {
	clusterSize := 15
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	if !assert.NoError(t, err) {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"syncInterval":1}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	time.Sleep(time.Second)
	c := etcd.NewClient(nil)
	c.SyncCluster()

	// Verify that we just have default machines.
	result, err := c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 9)

	t.Log("Reconfigure with a smaller active size")
	resp, _ = tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":7, "syncInterval":1}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Wait for two monitor cycles before checking for demotion.
	time.Sleep((2 * server.ActiveMonitorTimeout) + (2 * time.Second))

	// Verify that we now have seven peers.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 7)

	t.Log("Test the functionality of all servers")
	// Set key.
	time.Sleep(time.Second)
	if _, err := c.Set("foo", "bar", 0); err != nil {
		panic(err)
	}
	time.Sleep(time.Second)

	// Check that all peers and standbys have the value.
	for i := range etcds {
		resp, err := tests.Get(fmt.Sprintf("http://localhost:%d/v2/keys/foo", 4000+(i+1)))
		if assert.NoError(t, err) {
			body := tests.ReadBodyJSON(resp)
			if node, _ := body["node"].(map[string]interface{}); assert.NotNil(t, node) {
				assert.Equal(t, node["value"], "bar")
			}
		}
	}

	t.Log("Reconfigure with larger active size and wait for join")
	resp, _ = tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":8, "syncInterval":1}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	time.Sleep((1 * time.Second) + (1 * time.Second))

	// Verify that exactly eight machines are in the cluster.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 8)
}

// Create a full cluster, disconnect a peer, wait for removal, wait for standby join.
func TestStandbyAutoJoin(t *testing.T) {
	clusterSize := 5
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

	// Verify that we have five machines.
	result, err := c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 5)

	// Reconfigure with a short remove delay (2 second).
	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":4, "removeDelay":2, "syncInterval":1}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Wait for a monitor cycle before checking for removal.
	time.Sleep(server.ActiveMonitorTimeout + (1 * time.Second))

	// Verify that we now have four peers.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 4)

	// Remove peer.
	etcd := etcds[1]
	etcds = append(etcds[:1], etcds[2:]...)
	if err := etcd.Kill(); err != nil {
		panic(err.Error())
	}
	etcd.Release()

	// Wait for it to get dropped.
	time.Sleep(server.PeerActivityMonitorTimeout + (1 * time.Second))

	// Wait for the standby to join.
	time.Sleep((1 * time.Second) + (1 * time.Second))

	// Verify that we have 4 peers.
	result, err = c.Get("_etcd/machines", true, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 4)

	// Verify that node2 is not one of those peers.
	_, err = c.Get("_etcd/machines/node2", false, false)
	assert.Error(t, err)
}

// Create a full cluster and then change the active size gradually.
func TestStandbyGradualChange(t *testing.T) {
	clusterSize := 9
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	assert.NoError(t, err)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	time.Sleep(time.Second)
	c := etcd.NewClient(nil)
	c.SyncCluster()

	num := clusterSize
	for inc := 0; inc < 2; inc++ {
		for i := 0; i < 6; i++ {
			// Verify that we just have i machines.
			result, err := c.Get("_etcd/machines", false, true)
			assert.NoError(t, err)
			assert.Equal(t, len(result.Node.Nodes), num)

			if inc == 0 {
				num--
			} else {
				num++
			}

			t.Log("Reconfigure with active size", num)
			resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"activeSize":%d, "syncInterval":1}`, num)))
			if !assert.Equal(t, resp.StatusCode, 200) {
				t.FailNow()
			}

			if inc == 0 {
				// Wait for monitor cycles before checking for demotion.
				time.Sleep(server.ActiveMonitorTimeout + (1 * time.Second))
			} else {
				time.Sleep(time.Second + (1 * time.Second))
			}

			// Verify that we now have peers.
			result, err = c.Get("_etcd/machines", false, true)
			assert.NoError(t, err)
			assert.Equal(t, len(result.Node.Nodes), num)

			t.Log("Test the functionality of all servers")
			// Set key.
			if _, err := c.Set("foo", "bar", 0); err != nil {
				panic(err)
			}
			time.Sleep(100 * time.Millisecond)

			// Check that all peers and standbys have the value.
			for i := range etcds {
				resp, err := tests.Get(fmt.Sprintf("http://localhost:%d/v2/keys/foo", 4000+(i+1)))
				if assert.NoError(t, err) {
					body := tests.ReadBodyJSON(resp)
					if node, _ := body["node"].(map[string]interface{}); assert.NotNil(t, node) {
						assert.Equal(t, node["value"], "bar")
					}
				}
			}
		}
	}
}

// Create a full cluster and then change the active size dramatically.
func TestStandbyDramaticChange(t *testing.T) {
	clusterSize := 9
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	assert.NoError(t, err)
	defer DestroyCluster(etcds)

	if err != nil {
		t.Fatal("cannot create cluster")
	}

	time.Sleep(time.Second)
	c := etcd.NewClient(nil)
	c.SyncCluster()

	num := clusterSize
	for i := 0; i < 3; i++ {
		for inc := 0; inc < 2; inc++ {
			// Verify that we just have i machines.
			result, err := c.Get("_etcd/machines", false, true)
			assert.NoError(t, err)
			assert.Equal(t, len(result.Node.Nodes), num)

			if inc == 0 {
				num -= 6
			} else {
				num += 6
			}

			t.Log("Reconfigure with active size", num)
			resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"activeSize":%d, "syncInterval":1}`, num)))
			if !assert.Equal(t, resp.StatusCode, 200) {
				t.FailNow()
			}

			if inc == 0 {
				// Wait for monitor cycles before checking for demotion.
				time.Sleep(6*server.ActiveMonitorTimeout + (1 * time.Second))
			} else {
				time.Sleep(time.Second + (1 * time.Second))
			}

			// Verify that we now have peers.
			result, err = c.Get("_etcd/machines", false, true)
			assert.NoError(t, err)
			assert.Equal(t, len(result.Node.Nodes), num)

			t.Log("Test the functionality of all servers")
			// Set key.
			if _, err := c.Set("foo", "bar", 0); err != nil {
				panic(err)
			}
			time.Sleep(100 * time.Millisecond)

			// Check that all peers and standbys have the value.
			for i := range etcds {
				resp, err := tests.Get(fmt.Sprintf("http://localhost:%d/v2/keys/foo", 4000+(i+1)))
				if assert.NoError(t, err) {
					body := tests.ReadBodyJSON(resp)
					if node, _ := body["node"].(map[string]interface{}); assert.NotNil(t, node) {
						assert.Equal(t, node["value"], "bar")
					}
				}
			}
		}
	}
}

func TestStandbyJoinMiss(t *testing.T) {
	clusterSize := 2
	_, etcds, err := CreateCluster(clusterSize, &os.ProcAttr{Files: []*os.File{nil, os.Stdout, os.Stderr}}, false)
	if err != nil {
		t.Fatal("cannot create cluster")
	}
	defer DestroyCluster(etcds)

	c := etcd.NewClient(nil)
	c.SyncCluster()

	time.Sleep(1 * time.Second)

	// Verify that we have two machines.
	result, err := c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), clusterSize)

	resp, _ := tests.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"removeDelay":4, "syncInterval":4}`))
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}
	time.Sleep(time.Second)

	resp, _ = tests.Delete("http://localhost:7001/v2/admin/machines/node2", "application/json", nil)
	if !assert.Equal(t, resp.StatusCode, 200) {
		t.FailNow()
	}

	// Wait for a monitor cycle before checking for removal.
	time.Sleep(server.ActiveMonitorTimeout + (1 * time.Second))

	// Verify that we now have one peer.
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 1)

	// Simulate the join failure
	_, err = server.NewClient(nil).AddMachine("http://localhost:7001",
		&server.JoinCommand{
			MinVersion: store.MinVersion(),
			MaxVersion: store.MaxVersion(),
			Name:       "node2",
			RaftURL:    "http://127.0.0.1:7002",
			EtcdURL:    "http://127.0.0.1:4002",
		})
	assert.NoError(t, err)

	time.Sleep(6 * time.Second)

	go tests.Delete("http://localhost:7001/v2/admin/machines/node2", "application/json", nil)

	time.Sleep(time.Second)
	result, err = c.Get("_etcd/machines", false, true)
	assert.NoError(t, err)
	assert.Equal(t, len(result.Node.Nodes), 1)
}

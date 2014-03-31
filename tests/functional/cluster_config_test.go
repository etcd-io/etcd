package test

import (
	"bytes"
	"testing"
	"time"

	etcdtest "github.com/coreos/etcd/tests"
	"github.com/coreos/etcd/third_party/github.com/stretchr/testify/assert"
)

// Ensure that the cluster configuration can be updated.
func TestClusterConfig(t *testing.T) {
	cluster := etcdtest.NewCluster(3, false)
	ok := cluster.Start()
	assert.True(t, ok)
	defer cluster.Stop()

	resp, _ := etcdtest.Put("http://localhost:7001/v2/admin/config", "application/json", bytes.NewBufferString(`{"activeSize":3, "promoteDelay":60}`))
	assert.Equal(t, resp.StatusCode, 200)

	time.Sleep(1 * time.Second)

	resp, _ = etcdtest.Get("http://localhost:7002/v2/admin/config")
	body := etcdtest.ReadBodyJSON(resp)
	assert.Equal(t, resp.StatusCode, 200)
	assert.Equal(t, body["activeSize"], 3)
	assert.Equal(t, body["promoteDelay"], 60)
}

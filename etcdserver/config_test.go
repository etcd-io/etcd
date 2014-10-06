package etcdserver

import (
	"testing"
)

func TestConfigVerify(t *testing.T) {
	cluster := &Cluster{}
	cfg := ServerConfig{
		Name:         "node1",
		Cluster:      cluster,
		ClusterState: ClusterStateValueNew,
	}

	err := cfg.Verify()
	if err == nil {
		t.Error("Did not get error for lacking self in cluster.")
	}

	cluster.Set("node1=http://localhost:7001,node2=http://localhost:7001")
	err = cfg.Verify()
	if err == nil {
		t.Error("Did not get error for double URL in cluster.")
	}

	cluster.Set("node1=http://localhost:7001,node2=http://localhost:7002")
	err = cfg.Verify()
	if err != nil {
		t.Errorf("Got unexpected error %v", err)
	}

}

package etcdserver

import (
	"testing"
)

func TestBootstrapConfigVerify(t *testing.T) {
	tests := []struct {
		clusterSetting string
		clst           ClusterState
		disc           string
		shouldError    bool
	}{
		{"", ClusterStateValueNew, "", true},
		{"", "", "http://discovery", true},
		{
			"node1=http://localhost:7001,node2=http://localhost:7001",
			ClusterStateValueNew, "", true,
		},
		{
			"node1=http://localhost:7001,node2=http://localhost:7002",
			ClusterStateValueNew, "", false,
		},
		{
			"node1=http://localhost:7001",
			"", "http://discovery", false,
		},
	}

	for i, tt := range tests {
		cluster := &Cluster{}
		cluster.Set(tt.clusterSetting)
		cfg := ServerConfig{
			Name:         "node1",
			DiscoveryURL: tt.disc,
			Cluster:      cluster,
			ClusterState: tt.clst,
		}
		err := cfg.VerifyBootstrapConfig()
		if (err == nil) && tt.shouldError {
			t.Errorf("#%d: Got no error where one was expected", i)
		}
		if (err != nil) && !tt.shouldError {
			t.Errorf("#%d: Got unexpected error: %v", i, err)
		}
	}
}

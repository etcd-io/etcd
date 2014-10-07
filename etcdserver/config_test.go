package etcdserver

import (
	"testing"
)

func TestConfigVerify(t *testing.T) {
	tests := []struct {
		clusterSetting string
		shouldError    bool
	}{
		{"", true},
		{"node1=http://localhost:7001,node2=http://localhost:7001", true},
		{"node1=http://localhost:7001,node2=http://localhost:7002", false},
	}

	for i, tt := range tests {
		cluster := &Cluster{}
		cluster.Set(tt.clusterSetting)
		cfg := ServerConfig{
			Name:    "node1",
			Cluster: cluster,
		}
		err := cfg.Verify()
		if (err == nil) && tt.shouldError {
			t.Errorf("#%d: Got no error where one was expected", i)
		}
		if (err != nil) && !tt.shouldError {
			t.Errorf("#%d: Got unexpected error: %v", i, err)
		}
	}
}

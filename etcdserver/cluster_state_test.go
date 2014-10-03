package etcdserver

import (
	"testing"
)

func TestClusterStateSet(t *testing.T) {
	tests := []struct {
		val  string
		pass bool
	}{
		// known values
		{"new", true},

		// unrecognized values
		{"foo", false},
		{"", false},
	}

	for i, tt := range tests {
		pf := new(ClusterState)
		err := pf.Set(tt.val)
		if tt.pass != (err == nil) {
			t.Errorf("#%d: want pass=%t, but got err=%v", i, tt.pass, err)
		}
	}
}

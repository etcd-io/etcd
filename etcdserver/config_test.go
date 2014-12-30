/*
   Copyright 2014 CoreOS, Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package etcdserver

import (
	"net/url"
	"testing"

	"github.com/coreos/etcd/pkg/types"
)

func mustNewURLs(t *testing.T, urls []string) []url.URL {
	u, err := types.NewURLs(urls)
	if err != nil {
		t.Fatalf("error creating new URLs from %q: %v", urls, err)
	}
	return u
}

func TestBootstrapConfigVerify(t *testing.T) {
	tests := []struct {
		clusterSetting string
		newclst        bool
		apurls         []string
		disc           string
		shouldError    bool
	}{
		{
			// Node must exist in cluster
			"",
			true,
			nil,
			"",

			true,
		},
		{
			// Cannot have duplicate URLs in cluster config
			"node1=http://localhost:7001,node2=http://localhost:7001,node2=http://localhost:7002",
			true,
			nil,
			"",

			true,
		},
		{
			// Node defined, ClusterState OK
			"node1=http://localhost:7001,node2=http://localhost:7002",
			true,
			[]string{"http://localhost:7001"},
			"",

			false,
		},
		{
			// Node defined, discovery OK
			"node1=http://localhost:7001",
			false,
			[]string{"http://localhost:7001"},
			"http://discovery",

			false,
		},
		{
			// Cannot have ClusterState!=new && !discovery
			"node1=http://localhost:7001",
			false,
			nil,
			"",

			true,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:7001",
			true,
			[]string{"http://localhost:12345"},
			"",

			true,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:7001,node1=http://localhost:12345",
			true,
			[]string{"http://localhost:12345"},
			"",

			true,
		},
	}

	for i, tt := range tests {
		cluster, err := NewClusterFromString("", tt.clusterSetting)
		if err != nil {
			t.Fatalf("#%d: Got unexpected error: %v", i, err)
		}
		cfg := ServerConfig{
			Name:         "node1",
			DiscoveryURL: tt.disc,
			Cluster:      cluster,
			NewCluster:   tt.newclst,
		}
		if tt.apurls != nil {
			cfg.PeerURLs = mustNewURLs(t, tt.apurls)
		}
		err = cfg.VerifyBootstrapConfig()
		if (err == nil) && tt.shouldError {
			t.Errorf("%#v", *cluster)
			t.Errorf("#%d: Got no error where one was expected", i)
		}
		if (err != nil) && !tt.shouldError {
			t.Errorf("#%d: Got unexpected error: %v", i, err)
		}
	}
}

func TestSnapDir(t *testing.T) {
	tests := map[string]string{
		"/":            "/snap",
		"/var/lib/etc": "/var/lib/etc/snap",
	}
	for dd, w := range tests {
		cfg := ServerConfig{
			DataDir: dd,
		}
		if g := cfg.SnapDir(); g != w {
			t.Errorf("DataDir=%q: SnapDir()=%q, want=%q", dd, g, w)
		}
	}
}

func TestWALDir(t *testing.T) {
	tests := map[string]string{
		"/":            "/wal",
		"/var/lib/etc": "/var/lib/etc/wal",
	}
	for dd, w := range tests {
		cfg := ServerConfig{
			DataDir: dd,
		}
		if g := cfg.WALDir(); g != w {
			t.Errorf("DataDir=%q: WALDir()=%q, want=%q", dd, g, w)
		}
	}
}

func TestShouldDiscover(t *testing.T) {
	tests := map[string]bool{
		"":    false,
		"foo": true,
		"http://discovery.etcd.io/asdf": true,
	}
	for durl, w := range tests {
		cfg := ServerConfig{
			DiscoveryURL: durl,
		}
		if g := cfg.ShouldDiscover(); g != w {
			t.Errorf("durl=%q: ShouldDiscover()=%t, want=%t", durl, g, w)
		}
	}
}

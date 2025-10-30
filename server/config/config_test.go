// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/client/pkg/v3/types"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v3discovery"
)

func mustNewURLs(t *testing.T, urls []string) []url.URL {
	if len(urls) == 0 {
		return nil
	}
	u, err := types.NewURLs(urls)
	require.NoErrorf(t, err, "error creating new URLs from %q: %v", urls, err)
	return u
}

func TestConfigVerifyBootstrapWithoutClusterFail(t *testing.T) {
	c := &ServerConfig{
		Name: "node1",
		DiscoveryCfg: v3discovery.DiscoveryConfig{
			ConfigSpec: clientv3.ConfigSpec{
				Endpoints: []string{},
			},
		},
		InitialPeerURLsMap: types.URLsMap{},
		Logger:             zaptest.NewLogger(t),
	}
	if err := c.VerifyBootstrap(); err == nil {
		t.Errorf("err = nil, want not nil")
	}
}

func TestConfigVerifyExistingWithDiscoveryURLFail(t *testing.T) {
	cluster, err := types.NewURLsMap("node1=http://127.0.0.1:2380")
	require.NoErrorf(t, err, "NewCluster error: %v", err)
	c := &ServerConfig{
		Name: "node1",
		DiscoveryCfg: v3discovery.DiscoveryConfig{
			ConfigSpec: clientv3.ConfigSpec{
				Endpoints: []string{"http://192.168.0.100:2379"},
			},
		},
		PeerURLs:           mustNewURLs(t, []string{"http://127.0.0.1:2380"}),
		InitialPeerURLsMap: cluster,
		NewCluster:         false,
		Logger:             zaptest.NewLogger(t),
	}
	if err := c.VerifyJoinExisting(); err == nil {
		t.Errorf("err = nil, want not nil")
	}
}

func TestConfigVerifyLocalMember(t *testing.T) {
	tests := []struct {
		clusterSetting string
		apurls         []string
		strict         bool
		shouldError    bool
	}{
		{
			// Node must exist in cluster
			"",
			nil,
			true,

			true,
		},
		{
			// Initial cluster set
			"node1=http://localhost:7001,node2=http://localhost:7002",
			[]string{"http://localhost:7001"},
			true,

			false,
		},
		{
			// Default initial cluster
			"node1=http://localhost:2380,node1=http://localhost:7001",
			[]string{"http://localhost:2380", "http://localhost:7001"},
			true,

			false,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:7001",
			[]string{"http://localhost:12345"},
			true,

			true,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:2380,node1=http://localhost:12345",
			[]string{"http://localhost:12345"},
			true,

			true,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:12345",
			[]string{"http://localhost:2380", "http://localhost:12345"},
			true,

			true,
		},
		{
			// Advertised peer URLs must match those in cluster-state
			"node1=http://localhost:2380",
			[]string{},
			true,

			true,
		},
		{
			// do not care about the urls if strict is not set
			"node1=http://localhost:2380",
			[]string{},
			false,

			false,
		},
	}

	for i, tt := range tests {
		cluster, err := types.NewURLsMap(tt.clusterSetting)
		require.NoErrorf(t, err, "#%d: Got unexpected error: %v", i, err)
		cfg := ServerConfig{
			Name:               "node1",
			InitialPeerURLsMap: cluster,
			Logger:             zaptest.NewLogger(t),
		}
		if tt.apurls != nil {
			cfg.PeerURLs = mustNewURLs(t, tt.apurls)
		}
		if err = cfg.hasLocalMember(); err == nil && tt.strict {
			err = cfg.advertiseMatchesCluster()
		}
		if (err == nil) && tt.shouldError {
			t.Errorf("#%d: Got no error where one was expected", i)
		}
		if (err != nil) && !tt.shouldError {
			t.Errorf("#%d: Got unexpected error: %v", i, err)
		}
	}
}

func TestSnapDir(t *testing.T) {
	tests := map[string]string{
		"/":            "/member/snap",
		"/var/lib/etc": "/var/lib/etc/member/snap",
	}
	for dd, w := range tests {
		cfg := ServerConfig{
			DataDir: dd,
			Logger:  zaptest.NewLogger(t),
		}
		if g := cfg.SnapDir(); g != w {
			t.Errorf("DataDir=%q: SnapDir()=%q, want=%q", dd, g, w)
		}
	}
}

func TestWALDir(t *testing.T) {
	tests := map[string]string{
		"/":            "/member/wal",
		"/var/lib/etc": "/var/lib/etc/member/wal",
	}
	for dd, w := range tests {
		cfg := ServerConfig{
			DataDir: dd,
			Logger:  zaptest.NewLogger(t),
		}
		if g := cfg.WALDir(); g != w {
			t.Errorf("DataDir=%q: WALDir()=%q, want=%q", dd, g, w)
		}
	}
}

func TestShouldDiscover(t *testing.T) {
	tests := map[string]bool{
		"":                              false,
		"foo":                           true,
		"http://discovery.etcd.io/asdf": true,
	}
	for durl, w := range tests {
		var eps []string
		if durl != "" {
			eps = append(eps, durl)
		}
		cfg := ServerConfig{
			DiscoveryCfg: v3discovery.DiscoveryConfig{
				ConfigSpec: clientv3.ConfigSpec{
					Endpoints: eps,
				},
			},
			Logger: zaptest.NewLogger(t),
		}
		if g := cfg.ShouldDiscover(); g != w {
			t.Errorf("durl=%q: ShouldDiscover()=%t, want=%t", durl, g, w)
		}
	}
}

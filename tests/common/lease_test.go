// Copyright 2022 The etcd Authors
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

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/tests/v3/framework/config"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func TestLeaseGrantTimeToLive(t *testing.T) {
	testRunner.BeforeTest(t)

	tcs := []struct {
		name   string
		config config.ClusterConfig
	}{
		{
			name:   "NoTLS",
			config: config.ClusterConfig{ClusterSize: 1},
		},
		{
			name:   "PeerTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.ManualTLS},
		},
		{
			name:   "PeerAutoTLS",
			config: config.ClusterConfig{ClusterSize: 3, PeerTLS: config.AutoTLS},
		},
		{
			name:   "ClientTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.ManualTLS},
		},
		{
			name:   "ClientAutoTLS",
			config: config.ClusterConfig{ClusterSize: 1, ClientTLS: config.AutoTLS},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			clus := testRunner.NewCluster(t, tc.config)
			defer clus.Close()
			cc := clus.Client()

			testutils.ExecuteWithTimeout(t, 10*time.Second, func() {
				ttl := int64(10)
				leaseResp, err := cc.Grant(ttl)
				require.NoError(t, err)

				ttlResp, err := cc.TimeToLive(leaseResp.ID, config.LeaseOption{})
				require.NoError(t, err)
				require.Equal(t, ttl, ttlResp.GrantedTTL)
			})
		})
	}
}

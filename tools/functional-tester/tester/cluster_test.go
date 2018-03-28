// Copyright 2018 The etcd Authors
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

package tester

import (
	"reflect"
	"testing"

	"github.com/coreos/etcd/tools/functional-tester/rpcpb"

	"go.uber.org/zap"
)

func Test_newCluster(t *testing.T) {
	exp := &Cluster{
		Members: []*rpcpb.Member{
			{
				EtcdExecPath:       "./bin/etcd",
				AgentAddr:          "127.0.0.1:19027",
				FailpointHTTPAddr:  "http://127.0.0.1:7381",
				BaseDir:            "/tmp/etcd-agent-data-1",
				EtcdLogPath:        "/tmp/etcd-agent-data-1/current-etcd.log",
				EtcdClientTLS:      false,
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:1379",
				Etcd: &rpcpb.Etcd{
					Name:                     "s1",
					DataDir:                  "/tmp/etcd-agent-data-1/etcd.data",
					WALDir:                   "/tmp/etcd-agent-data-1/etcd.data/member/wal",
					ListenClientURLs:         []string{"http://127.0.0.1:1379"},
					AdvertiseClientURLs:      []string{"http://127.0.0.1:1379"},
					ListenPeerURLs:           []string{"http://127.0.0.1:1380"},
					InitialAdvertisePeerURLs: []string{"http://127.0.0.1:13800"},
					InitialCluster:           "s1=http://127.0.0.1:13800,s2=http://127.0.0.1:23800,s3=http://127.0.0.1:33800",
					InitialClusterState:      "new",
					InitialClusterToken:      "tkn",
					SnapshotCount:            10000,
					QuotaBackendBytes:        10740000000,
					PreVote:                  true,
					InitialCorruptCheck:      true,
				},
			},
			{
				EtcdExecPath:       "./bin/etcd",
				AgentAddr:          "127.0.0.1:29027",
				FailpointHTTPAddr:  "http://127.0.0.1:7382",
				BaseDir:            "/tmp/etcd-agent-data-2",
				EtcdLogPath:        "/tmp/etcd-agent-data-2/current-etcd.log",
				EtcdClientTLS:      false,
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:2379",
				Etcd: &rpcpb.Etcd{
					Name:                     "s2",
					DataDir:                  "/tmp/etcd-agent-data-2/etcd.data",
					WALDir:                   "/tmp/etcd-agent-data-2/etcd.data/member/wal",
					ListenClientURLs:         []string{"http://127.0.0.1:2379"},
					AdvertiseClientURLs:      []string{"http://127.0.0.1:2379"},
					ListenPeerURLs:           []string{"http://127.0.0.1:2380"},
					InitialAdvertisePeerURLs: []string{"http://127.0.0.1:23800"},
					InitialCluster:           "s1=http://127.0.0.1:13800,s2=http://127.0.0.1:23800,s3=http://127.0.0.1:33800",
					InitialClusterState:      "new",
					InitialClusterToken:      "tkn",
					SnapshotCount:            10000,
					QuotaBackendBytes:        10740000000,
					PreVote:                  true,
					InitialCorruptCheck:      true,
				},
			},
			{
				EtcdExecPath:       "./bin/etcd",
				AgentAddr:          "127.0.0.1:39027",
				FailpointHTTPAddr:  "http://127.0.0.1:7383",
				BaseDir:            "/tmp/etcd-agent-data-3",
				EtcdLogPath:        "/tmp/etcd-agent-data-3/current-etcd.log",
				EtcdClientTLS:      false,
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:3379",
				Etcd: &rpcpb.Etcd{
					Name:                     "s3",
					DataDir:                  "/tmp/etcd-agent-data-3/etcd.data",
					WALDir:                   "/tmp/etcd-agent-data-3/etcd.data/member/wal",
					ListenClientURLs:         []string{"http://127.0.0.1:3379"},
					AdvertiseClientURLs:      []string{"http://127.0.0.1:3379"},
					ListenPeerURLs:           []string{"http://127.0.0.1:3380"},
					InitialAdvertisePeerURLs: []string{"http://127.0.0.1:33800"},
					InitialCluster:           "s1=http://127.0.0.1:13800,s2=http://127.0.0.1:23800,s3=http://127.0.0.1:33800",
					InitialClusterState:      "new",
					InitialClusterToken:      "tkn",
					SnapshotCount:            10000,
					QuotaBackendBytes:        10740000000,
					PreVote:                  true,
					InitialCorruptCheck:      true,
				},
			},
		},
		Tester: &rpcpb.Tester{
			TesterNetwork:    "tcp",
			TesterAddr:       "127.0.0.1:9028",
			DelayLatencyMs:   500,
			DelayLatencyMsRv: 50,
			RoundLimit:       1,
			ExitOnFailure:    true,
			ConsistencyCheck: true,
			EnablePprof:      true,
			FailureCases: []string{
				"KILL_ONE_FOLLOWER",
				"KILL_LEADER",
				"KILL_ONE_FOLLOWER_FOR_LONG",
				"KILL_LEADER_FOR_LONG",
				"KILL_QUORUM",
				"KILL_ALL",
				"BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER",
				"BLACKHOLE_PEER_PORT_TX_RX_LEADER_ONE",
				"BLACKHOLE_PEER_PORT_TX_RX_ALL",
				"DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER",
				"DELAY_PEER_PORT_TX_RX_LEADER",
				"DELAY_PEER_PORT_TX_RX_ALL",
			},
			FailpointCommands:       []string{`panic("etcd-tester")`},
			RunnerExecPath:          "/etcd-runner",
			ExternalExecPath:        "",
			StressTypes:             []string{"KV", "LEASE"},
			StressKeySize:           100,
			StressKeySizeLarge:      32769,
			StressKeySuffixRange:    250000,
			StressKeySuffixRangeTxn: 100,
			StressKeyTxnOps:         10,
			StressQPS:               1000,
		},
	}

	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	cfg, err := newCluster(logger, "./local-test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.logger = nil

	if !reflect.DeepEqual(exp, cfg) {
		t.Fatalf("expected %+v, got %+v", exp, cfg)
	}
}

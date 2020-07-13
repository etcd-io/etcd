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
	"sort"
	"testing"

	"go.etcd.io/etcd/v3/functional/rpcpb"

	"go.uber.org/zap"
)

func Test_read(t *testing.T) {
	exp := &Cluster{
		Members: []*rpcpb.Member{
			{
				EtcdExec:           "./bin/etcd",
				AgentAddr:          "127.0.0.1:19027",
				FailpointHTTPAddr:  "http://127.0.0.1:7381",
				BaseDir:            "/tmp/etcd-functional-1",
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:1379",
				Etcd: &rpcpb.Etcd{
					Name:                "s1",
					DataDir:             "/tmp/etcd-functional-1/etcd.data",
					WALDir:              "/tmp/etcd-functional-1/etcd.data/member/wal",
					HeartbeatIntervalMs: 100,
					ElectionTimeoutMs:   1000,
					ListenClientURLs:    []string{"https://127.0.0.1:1379"},
					AdvertiseClientURLs: []string{"https://127.0.0.1:1379"},
					ClientAutoTLS:       true,
					ClientCertAuth:      false,
					ClientCertFile:      "",
					ClientKeyFile:       "",
					ClientTrustedCAFile: "",
					ListenPeerURLs:      []string{"https://127.0.0.1:1380"},
					AdvertisePeerURLs:   []string{"https://127.0.0.1:1381"},
					PeerAutoTLS:         true,
					PeerClientCertAuth:  false,
					PeerCertFile:        "",
					PeerKeyFile:         "",
					PeerTrustedCAFile:   "",
					InitialCluster:      "s1=https://127.0.0.1:1381,s2=https://127.0.0.1:2381,s3=https://127.0.0.1:3381",
					InitialClusterState: "new",
					InitialClusterToken: "tkn",
					SnapshotCount:       2000,
					QuotaBackendBytes:   10740000000,
					PreVote:             true,
					InitialCorruptCheck: true,
					Logger:              "zap",
					LogOutputs:          []string{"/tmp/etcd-functional-1/etcd.log"},
					LogLevel:            "info",
				},
				ClientCertData:      "",
				ClientCertPath:      "",
				ClientKeyData:       "",
				ClientKeyPath:       "",
				ClientTrustedCAData: "",
				ClientTrustedCAPath: "",
				PeerCertData:        "",
				PeerCertPath:        "",
				PeerKeyData:         "",
				PeerKeyPath:         "",
				PeerTrustedCAData:   "",
				PeerTrustedCAPath:   "",
				SnapshotPath:        "/tmp/etcd-functional-1.snapshot.db",
			},
			{
				EtcdExec:           "./bin/etcd",
				AgentAddr:          "127.0.0.1:29027",
				FailpointHTTPAddr:  "http://127.0.0.1:7382",
				BaseDir:            "/tmp/etcd-functional-2",
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:2379",
				Etcd: &rpcpb.Etcd{
					Name:                "s2",
					DataDir:             "/tmp/etcd-functional-2/etcd.data",
					WALDir:              "/tmp/etcd-functional-2/etcd.data/member/wal",
					HeartbeatIntervalMs: 100,
					ElectionTimeoutMs:   1000,
					ListenClientURLs:    []string{"https://127.0.0.1:2379"},
					AdvertiseClientURLs: []string{"https://127.0.0.1:2379"},
					ClientAutoTLS:       true,
					ClientCertAuth:      false,
					ClientCertFile:      "",
					ClientKeyFile:       "",
					ClientTrustedCAFile: "",
					ListenPeerURLs:      []string{"https://127.0.0.1:2380"},
					AdvertisePeerURLs:   []string{"https://127.0.0.1:2381"},
					PeerAutoTLS:         true,
					PeerClientCertAuth:  false,
					PeerCertFile:        "",
					PeerKeyFile:         "",
					PeerTrustedCAFile:   "",
					InitialCluster:      "s1=https://127.0.0.1:1381,s2=https://127.0.0.1:2381,s3=https://127.0.0.1:3381",
					InitialClusterState: "new",
					InitialClusterToken: "tkn",
					SnapshotCount:       2000,
					QuotaBackendBytes:   10740000000,
					PreVote:             true,
					InitialCorruptCheck: true,
					Logger:              "zap",
					LogOutputs:          []string{"/tmp/etcd-functional-2/etcd.log"},
					LogLevel:            "info",
				},
				ClientCertData:      "",
				ClientCertPath:      "",
				ClientKeyData:       "",
				ClientKeyPath:       "",
				ClientTrustedCAData: "",
				ClientTrustedCAPath: "",
				PeerCertData:        "",
				PeerCertPath:        "",
				PeerKeyData:         "",
				PeerKeyPath:         "",
				PeerTrustedCAData:   "",
				PeerTrustedCAPath:   "",
				SnapshotPath:        "/tmp/etcd-functional-2.snapshot.db",
			},
			{
				EtcdExec:           "./bin/etcd",
				AgentAddr:          "127.0.0.1:39027",
				FailpointHTTPAddr:  "http://127.0.0.1:7383",
				BaseDir:            "/tmp/etcd-functional-3",
				EtcdClientProxy:    false,
				EtcdPeerProxy:      true,
				EtcdClientEndpoint: "127.0.0.1:3379",
				Etcd: &rpcpb.Etcd{
					Name:                "s3",
					DataDir:             "/tmp/etcd-functional-3/etcd.data",
					WALDir:              "/tmp/etcd-functional-3/etcd.data/member/wal",
					HeartbeatIntervalMs: 100,
					ElectionTimeoutMs:   1000,
					ListenClientURLs:    []string{"https://127.0.0.1:3379"},
					AdvertiseClientURLs: []string{"https://127.0.0.1:3379"},
					ClientAutoTLS:       true,
					ClientCertAuth:      false,
					ClientCertFile:      "",
					ClientKeyFile:       "",
					ClientTrustedCAFile: "",
					ListenPeerURLs:      []string{"https://127.0.0.1:3380"},
					AdvertisePeerURLs:   []string{"https://127.0.0.1:3381"},
					PeerAutoTLS:         true,
					PeerClientCertAuth:  false,
					PeerCertFile:        "",
					PeerKeyFile:         "",
					PeerTrustedCAFile:   "",
					InitialCluster:      "s1=https://127.0.0.1:1381,s2=https://127.0.0.1:2381,s3=https://127.0.0.1:3381",
					InitialClusterState: "new",
					InitialClusterToken: "tkn",
					SnapshotCount:       2000,
					QuotaBackendBytes:   10740000000,
					PreVote:             true,
					InitialCorruptCheck: true,
					Logger:              "zap",
					LogOutputs:          []string{"/tmp/etcd-functional-3/etcd.log"},
					LogLevel:            "info",
				},
				ClientCertData:      "",
				ClientCertPath:      "",
				ClientKeyData:       "",
				ClientKeyPath:       "",
				ClientTrustedCAData: "",
				ClientTrustedCAPath: "",
				PeerCertData:        "",
				PeerCertPath:        "",
				PeerKeyData:         "",
				PeerKeyPath:         "",
				PeerTrustedCAData:   "",
				PeerTrustedCAPath:   "",
				SnapshotPath:        "/tmp/etcd-functional-3.snapshot.db",
			},
		},
		Tester: &rpcpb.Tester{
			DataDir:               "/tmp/etcd-tester-data",
			Network:               "tcp",
			Addr:                  "127.0.0.1:9028",
			DelayLatencyMs:        5000,
			DelayLatencyMsRv:      500,
			UpdatedDelayLatencyMs: 5000,
			RoundLimit:            1,
			ExitOnCaseFail:        true,
			EnablePprof:           true,
			CaseDelayMs:           7000,
			CaseShuffle:           true,
			Cases: []string{
				"SIGTERM_ONE_FOLLOWER",
				"SIGTERM_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT",
				"SIGTERM_LEADER",
				"SIGTERM_LEADER_UNTIL_TRIGGER_SNAPSHOT",
				"SIGTERM_QUORUM",
				"SIGTERM_ALL",
				"SIGQUIT_AND_REMOVE_ONE_FOLLOWER",
				"SIGQUIT_AND_REMOVE_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT",
				// "SIGQUIT_AND_REMOVE_LEADER",
				// "SIGQUIT_AND_REMOVE_LEADER_UNTIL_TRIGGER_SNAPSHOT",
				// "SIGQUIT_AND_REMOVE_QUORUM_AND_RESTORE_LEADER_SNAPSHOT_FROM_SCRATCH",
				// "BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER",
				// "BLACKHOLE_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT",
				"BLACKHOLE_PEER_PORT_TX_RX_LEADER",
				"BLACKHOLE_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT",
				"BLACKHOLE_PEER_PORT_TX_RX_QUORUM",
				"BLACKHOLE_PEER_PORT_TX_RX_ALL",
				// "DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER",
				// "RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER",
				// "DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT",
				// "RANDOM_DELAY_PEER_PORT_TX_RX_ONE_FOLLOWER_UNTIL_TRIGGER_SNAPSHOT",
				"DELAY_PEER_PORT_TX_RX_LEADER",
				"RANDOM_DELAY_PEER_PORT_TX_RX_LEADER",
				"DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT",
				"RANDOM_DELAY_PEER_PORT_TX_RX_LEADER_UNTIL_TRIGGER_SNAPSHOT",
				"DELAY_PEER_PORT_TX_RX_QUORUM",
				"RANDOM_DELAY_PEER_PORT_TX_RX_QUORUM",
				"DELAY_PEER_PORT_TX_RX_ALL",
				"RANDOM_DELAY_PEER_PORT_TX_RX_ALL",
				"NO_FAIL_WITH_STRESS",
				"NO_FAIL_WITH_NO_STRESS_FOR_LIVENESS",
			},
			FailpointCommands: []string{`panic("etcd-tester")`},
			RunnerExecPath:    "./bin/etcd-runner",
			ExternalExecPath:  "",
			Stressers: []*rpcpb.Stresser{
				{Type: "KV_WRITE_SMALL", Weight: 0.35},
				{Type: "KV_WRITE_LARGE", Weight: 0.002},
				{Type: "KV_READ_ONE_KEY", Weight: 0.07},
				{Type: "KV_READ_RANGE", Weight: 0.07},
				{Type: "KV_DELETE_ONE_KEY", Weight: 0.07},
				{Type: "KV_DELETE_RANGE", Weight: 0.07},
				{Type: "KV_TXN_WRITE_DELETE", Weight: 0.35},
				{Type: "LEASE", Weight: 0.0},
			},
			Checkers:                []string{"KV_HASH", "LEASE_EXPIRE"},
			StressKeySize:           100,
			StressKeySizeLarge:      32769,
			StressKeySuffixRange:    250000,
			StressKeySuffixRangeTxn: 100,
			StressKeyTxnOps:         10,
			StressClients:           100,
			StressQPS:               2000,
		},
	}

	logger, err := zap.NewProduction()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Sync()

	cfg, err := read(logger, "../../functional.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg.lg = nil

	if !reflect.DeepEqual(exp, cfg) {
		t.Fatalf(`exp != cfg:
  expected %+v    
       got %+v`, exp, cfg)
	}

	cfg.lg = logger

	cfg.updateCases()
	fs1 := cfg.listCases()

	cfg.shuffleCases()
	fs2 := cfg.listCases()
	if reflect.DeepEqual(fs1, fs2) {
		t.Fatalf("expected shuffled failure cases, got %q", fs2)
	}

	cfg.shuffleCases()
	fs3 := cfg.listCases()
	if reflect.DeepEqual(fs2, fs3) {
		t.Fatalf("expected reshuffled failure cases from %q, got %q", fs2, fs3)
	}

	// shuffle ensures visit all exactly once
	// so when sorted, failure cases must be equal
	sort.Strings(fs1)
	sort.Strings(fs2)
	sort.Strings(fs3)

	if !reflect.DeepEqual(fs1, fs2) {
		t.Fatalf("expected %q, got %q", fs1, fs2)
	}
	if !reflect.DeepEqual(fs2, fs3) {
		t.Fatalf("expected %q, got %q", fs2, fs3)
	}
}

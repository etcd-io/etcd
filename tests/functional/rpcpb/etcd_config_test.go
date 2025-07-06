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

package rpcpb

import (
	"reflect"
	"testing"
)

func TestEtcd(t *testing.T) {
	e := &Etcd{
		Name:    "s1",
		DataDir: "/tmp/etcd-functionl-1/etcd.data",
		WALDir:  "/tmp/etcd-functionl-1/etcd.data/member/wal",

		HeartbeatIntervalMs: 100,
		ElectionTimeoutMs:   1000,

		ListenClientURLs:    []string{"https://127.0.0.1:1379"},
		AdvertiseClientURLs: []string{"https://127.0.0.1:13790"},
		ClientAutoTLS:       true,
		ClientCertAuth:      false,
		ClientCertFile:      "",
		ClientKeyFile:       "",
		ClientTrustedCAFile: "",

		ListenPeerURLs:     []string{"https://127.0.0.1:1380"},
		AdvertisePeerURLs:  []string{"https://127.0.0.1:13800"},
		PeerAutoTLS:        true,
		PeerClientCertAuth: false,
		PeerCertFile:       "",
		PeerKeyFile:        "",
		PeerTrustedCAFile:  "",

		InitialCluster:      "s1=https://127.0.0.1:13800,s2=https://127.0.0.1:23800,s3=https://127.0.0.1:33800",
		InitialClusterState: "new",
		InitialClusterToken: "tkn",

		SnapshotCount:     10000,
		QuotaBackendBytes: 10740000000,

		PreVote:             true,
		InitialCorruptCheck: true,

		Logger:     "zap",
		LogOutputs: []string{"/tmp/etcd-functional-1/etcd.log"},
		LogLevel:   "info",
	}

	exps := []string{
		"--name=s1",
		"--data-dir=/tmp/etcd-functionl-1/etcd.data",
		"--wal-dir=/tmp/etcd-functionl-1/etcd.data/member/wal",
		"--heartbeat-interval=100",
		"--election-timeout=1000",
		"--listen-client-urls=https://127.0.0.1:1379",
		"--advertise-client-urls=https://127.0.0.1:13790",
		"--auto-tls=true",
		"--client-cert-auth=false",
		"--listen-peer-urls=https://127.0.0.1:1380",
		"--initial-advertise-peer-urls=https://127.0.0.1:13800",
		"--peer-auto-tls=true",
		"--peer-client-cert-auth=false",
		"--initial-cluster=s1=https://127.0.0.1:13800,s2=https://127.0.0.1:23800,s3=https://127.0.0.1:33800",
		"--initial-cluster-state=new",
		"--initial-cluster-token=tkn",
		"--snapshot-count=10000",
		"--quota-backend-bytes=10740000000",
		"--pre-vote=true",
		"--experimental-initial-corrupt-check=true",
		"--logger=zap",
		"--log-outputs=/tmp/etcd-functional-1/etcd.log",
		"--log-level=info",
	}
	fs := e.Flags()
	if !reflect.DeepEqual(exps, fs) {
		t.Fatalf("expected %q, got %q", exps, fs)
	}
}

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
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/coreos/etcd/functional/rpcpb"

	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"
)

func read(lg *zap.Logger, fpath string) (*Cluster, error) {
	bts, err := ioutil.ReadFile(fpath)
	if err != nil {
		return nil, err
	}
	lg.Info("opened configuration file", zap.String("path", fpath))

	clus := &Cluster{lg: lg}
	if err = yaml.Unmarshal(bts, clus); err != nil {
		return nil, err
	}

	for i, mem := range clus.Members {
		if mem.BaseDir == "" {
			return nil, fmt.Errorf("BaseDir cannot be empty (got %q)", mem.BaseDir)
		}
		if mem.EtcdLogPath == "" {
			return nil, fmt.Errorf("EtcdLogPath cannot be empty (got %q)", mem.EtcdLogPath)
		}

		if mem.Etcd.Name == "" {
			return nil, fmt.Errorf("'--name' cannot be empty (got %+v)", mem)
		}
		if mem.Etcd.DataDir == "" {
			return nil, fmt.Errorf("'--data-dir' cannot be empty (got %+v)", mem)
		}
		if mem.Etcd.SnapshotCount == 0 {
			return nil, fmt.Errorf("'--snapshot-count' cannot be 0 (got %+v)", mem.Etcd.SnapshotCount)
		}
		if mem.Etcd.DataDir == "" {
			return nil, fmt.Errorf("'--data-dir' cannot be empty (got %q)", mem.Etcd.DataDir)
		}
		if mem.Etcd.WALDir == "" {
			clus.Members[i].Etcd.WALDir = filepath.Join(mem.Etcd.DataDir, "member", "wal")
		}

		if mem.Etcd.HeartbeatIntervalMs == 0 {
			return nil, fmt.Errorf("'--heartbeat-interval' cannot be 0 (got %+v)", mem.Etcd)
		}
		if mem.Etcd.ElectionTimeoutMs == 0 {
			return nil, fmt.Errorf("'--election-timeout' cannot be 0 (got %+v)", mem.Etcd)
		}
		if int64(clus.Tester.DelayLatencyMs) <= mem.Etcd.ElectionTimeoutMs {
			return nil, fmt.Errorf("delay latency %d ms must be greater than election timeout %d ms", clus.Tester.DelayLatencyMs, mem.Etcd.ElectionTimeoutMs)
		}

		port := ""
		listenClientPorts := make([]string, len(clus.Members))
		for i, u := range mem.Etcd.ListenClientURLs {
			if !isValidURL(u) {
				return nil, fmt.Errorf("'--listen-client-urls' has valid URL %q", u)
			}
			listenClientPorts[i], err = getPort(u)
			if err != nil {
				return nil, fmt.Errorf("'--listen-client-urls' has no port %q", u)
			}
		}
		for i, u := range mem.Etcd.AdvertiseClientURLs {
			if !isValidURL(u) {
				return nil, fmt.Errorf("'--advertise-client-urls' has valid URL %q", u)
			}
			port, err = getPort(u)
			if err != nil {
				return nil, fmt.Errorf("'--advertise-client-urls' has no port %q", u)
			}
			if mem.EtcdClientProxy && listenClientPorts[i] == port {
				return nil, fmt.Errorf("clus.Members[%d] requires client port proxy, but advertise port %q conflicts with listener port %q", i, port, listenClientPorts[i])
			}
		}

		listenPeerPorts := make([]string, len(clus.Members))
		for i, u := range mem.Etcd.ListenPeerURLs {
			if !isValidURL(u) {
				return nil, fmt.Errorf("'--listen-peer-urls' has valid URL %q", u)
			}
			listenPeerPorts[i], err = getPort(u)
			if err != nil {
				return nil, fmt.Errorf("'--listen-peer-urls' has no port %q", u)
			}
		}
		for j, u := range mem.Etcd.AdvertisePeerURLs {
			if !isValidURL(u) {
				return nil, fmt.Errorf("'--initial-advertise-peer-urls' has valid URL %q", u)
			}
			port, err = getPort(u)
			if err != nil {
				return nil, fmt.Errorf("'--initial-advertise-peer-urls' has no port %q", u)
			}
			if mem.EtcdPeerProxy && listenPeerPorts[j] == port {
				return nil, fmt.Errorf("clus.Members[%d] requires peer port proxy, but advertise port %q conflicts with listener port %q", i, port, listenPeerPorts[j])
			}
		}

		if !strings.HasPrefix(mem.EtcdLogPath, mem.BaseDir) {
			return nil, fmt.Errorf("EtcdLogPath must be prefixed with BaseDir (got %q)", mem.EtcdLogPath)
		}
		if !strings.HasPrefix(mem.Etcd.DataDir, mem.BaseDir) {
			return nil, fmt.Errorf("Etcd.DataDir must be prefixed with BaseDir (got %q)", mem.Etcd.DataDir)
		}

		// TODO: support separate WALDir that can be handled via failure-archive
		if !strings.HasPrefix(mem.Etcd.WALDir, mem.BaseDir) {
			return nil, fmt.Errorf("Etcd.WALDir must be prefixed with BaseDir (got %q)", mem.Etcd.WALDir)
		}

		// TODO: only support generated certs with TLS generator
		// deprecate auto TLS
		if mem.Etcd.ClientAutoTLS && mem.Etcd.ClientCertAuth {
			return nil, fmt.Errorf("Etcd.ClientAutoTLS and Etcd.ClientCertAuth are both 'true'")
		}
		if mem.Etcd.ClientAutoTLS && mem.Etcd.ClientCertFile != "" {
			return nil, fmt.Errorf("Etcd.ClientAutoTLS 'true', but Etcd.ClientCertFile is %q", mem.Etcd.ClientCertFile)
		}
		if mem.Etcd.ClientCertAuth && mem.Etcd.ClientCertFile == "" {
			return nil, fmt.Errorf("Etcd.ClientCertAuth 'true', but Etcd.ClientCertFile is %q", mem.Etcd.PeerCertFile)
		}
		if mem.Etcd.ClientAutoTLS && mem.Etcd.ClientKeyFile != "" {
			return nil, fmt.Errorf("Etcd.ClientAutoTLS 'true', but Etcd.ClientKeyFile is %q", mem.Etcd.ClientKeyFile)
		}
		if mem.Etcd.ClientAutoTLS && mem.Etcd.ClientTrustedCAFile != "" {
			return nil, fmt.Errorf("Etcd.ClientAutoTLS 'true', but Etcd.ClientTrustedCAFile is %q", mem.Etcd.ClientTrustedCAFile)
		}
		if mem.Etcd.PeerAutoTLS && mem.Etcd.PeerClientCertAuth {
			return nil, fmt.Errorf("Etcd.PeerAutoTLS and Etcd.PeerClientCertAuth are both 'true'")
		}
		if mem.Etcd.PeerAutoTLS && mem.Etcd.PeerCertFile != "" {
			return nil, fmt.Errorf("Etcd.PeerAutoTLS 'true', but Etcd.PeerCertFile is %q", mem.Etcd.PeerCertFile)
		}
		if mem.Etcd.PeerClientCertAuth && mem.Etcd.PeerCertFile == "" {
			return nil, fmt.Errorf("Etcd.PeerClientCertAuth 'true', but Etcd.PeerCertFile is %q", mem.Etcd.PeerCertFile)
		}
		if mem.Etcd.PeerAutoTLS && mem.Etcd.PeerKeyFile != "" {
			return nil, fmt.Errorf("Etcd.PeerAutoTLS 'true', but Etcd.PeerKeyFile is %q", mem.Etcd.PeerKeyFile)
		}
		if mem.Etcd.PeerAutoTLS && mem.Etcd.PeerTrustedCAFile != "" {
			return nil, fmt.Errorf("Etcd.PeerAutoTLS 'true', but Etcd.PeerTrustedCAFile is %q", mem.Etcd.PeerTrustedCAFile)
		}

		if mem.Etcd.ClientAutoTLS || mem.Etcd.ClientCertFile != "" {
			for _, cu := range mem.Etcd.ListenClientURLs {
				var u *url.URL
				u, err = url.Parse(cu)
				if err != nil {
					return nil, err
				}
				if u.Scheme != "https" { // TODO: support unix
					return nil, fmt.Errorf("client TLS is enabled with wrong scheme %q", cu)
				}
			}
			for _, cu := range mem.Etcd.AdvertiseClientURLs {
				var u *url.URL
				u, err = url.Parse(cu)
				if err != nil {
					return nil, err
				}
				if u.Scheme != "https" { // TODO: support unix
					return nil, fmt.Errorf("client TLS is enabled with wrong scheme %q", cu)
				}
			}
		}
		if mem.Etcd.PeerAutoTLS || mem.Etcd.PeerCertFile != "" {
			for _, cu := range mem.Etcd.ListenPeerURLs {
				var u *url.URL
				u, err = url.Parse(cu)
				if err != nil {
					return nil, err
				}
				if u.Scheme != "https" { // TODO: support unix
					return nil, fmt.Errorf("peer TLS is enabled with wrong scheme %q", cu)
				}
			}
			for _, cu := range mem.Etcd.AdvertisePeerURLs {
				var u *url.URL
				u, err = url.Parse(cu)
				if err != nil {
					return nil, err
				}
				if u.Scheme != "https" { // TODO: support unix
					return nil, fmt.Errorf("peer TLS is enabled with wrong scheme %q", cu)
				}
			}
		}
	}

	if len(clus.Tester.FailureCases) == 0 {
		return nil, errors.New("FailureCases not found")
	}
	if clus.Tester.DelayLatencyMs <= clus.Tester.DelayLatencyMsRv*5 {
		return nil, fmt.Errorf("delay latency %d ms must be greater than 5x of delay latency random variable %d ms", clus.Tester.DelayLatencyMs, clus.Tester.DelayLatencyMsRv)
	}
	if clus.Tester.UpdatedDelayLatencyMs == 0 {
		clus.Tester.UpdatedDelayLatencyMs = clus.Tester.DelayLatencyMs
	}

	for _, v := range clus.Tester.FailureCases {
		if _, ok := rpcpb.FailureCase_value[v]; !ok {
			return nil, fmt.Errorf("%q is not defined in 'rpcpb.FailureCase_value'", v)
		}
	}

	for _, v := range clus.Tester.StressTypes {
		if _, ok := rpcpb.StressType_value[v]; !ok {
			return nil, fmt.Errorf("StressType is unknown; got %q", v)
		}
	}
	if clus.Tester.StressKeySuffixRangeTxn > 100 {
		return nil, fmt.Errorf("StressKeySuffixRangeTxn maximum value is 100, got %v", clus.Tester.StressKeySuffixRangeTxn)
	}
	if clus.Tester.StressKeyTxnOps > 64 {
		return nil, fmt.Errorf("StressKeyTxnOps maximum value is 64, got %v", clus.Tester.StressKeyTxnOps)
	}

	return clus, err
}

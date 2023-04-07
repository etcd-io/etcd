// Copyright 2023 The etcd Authors
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

package testclient

import (
	"fmt"
	"time"

	"google.golang.org/grpc"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/tests/v3/framework/testutils"
)

func New(entpoints []string, cfg Config) (*clientv3.Client, error) {
	tlscfg, err := tlsInfo(cfg)
	if err != nil {
		return nil, err
	}
	ccfg := clientv3.Config{
		Endpoints:   entpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
	}
	if tlscfg != nil {
		tls, err := tlscfg.ClientConfig()
		if err != nil {
			return nil, err
		}
		ccfg.TLS = tls
	}
	return clientv3.New(ccfg)
}

func tlsInfo(cfg Config) (*transport.TLSInfo, error) {
	switch cfg.ConnectionType {
	case ConnectionNonTLS, ConnectionTLSAndNonTLS:
		return nil, nil
	case ConnectionTLS:
		if cfg.AutoTLS {
			return nil, nil
		} else {
			return &TestTLSInfo, nil
		}
	default:
		return nil, fmt.Errorf("config %v not supported", cfg)
	}
}

var (
	TestTLSInfo = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("../fixtures/server.key.insecure"),
		CertFile:       testutils.MustAbsPath("../fixtures/server.crt"),
		TrustedCAFile:  testutils.MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoWithSpecificUsage = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("../fixtures/server-serverusage.key.insecure"),
		CertFile:       testutils.MustAbsPath("../fixtures/server-serverusage.crt"),
		ClientKeyFile:  testutils.MustAbsPath("../fixtures/client-clientusage.key.insecure"),
		ClientCertFile: testutils.MustAbsPath("../fixtures/client-clientusage.crt"),
		TrustedCAFile:  testutils.MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoIP = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("../fixtures/server-ip.key.insecure"),
		CertFile:       testutils.MustAbsPath("../fixtures/server-ip.crt"),
		TrustedCAFile:  testutils.MustAbsPath("../fixtures/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoExpired = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("./fixtures-expired/server.key.insecure"),
		CertFile:       testutils.MustAbsPath("./fixtures-expired/server.crt"),
		TrustedCAFile:  testutils.MustAbsPath("./fixtures-expired/ca.crt"),
		ClientCertAuth: true,
	}

	TestTLSInfoExpiredIP = transport.TLSInfo{
		KeyFile:        testutils.MustAbsPath("./fixtures-expired/server-ip.key.insecure"),
		CertFile:       testutils.MustAbsPath("./fixtures-expired/server-ip.crt"),
		TrustedCAFile:  testutils.MustAbsPath("./fixtures-expired/ca.crt"),
		ClientCertAuth: true,
	}
)

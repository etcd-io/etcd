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

package command

import (
	"crypto/tls"

	"go.uber.org/zap"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestNewClientConfig(t *testing.T) {
	cases := []struct {
		name         string
		spec         clientv3.ConfigSpec
		expectedConf clientv3.Config
	}{
		{
			name: "default secure transport",
			spec: clientv3.ConfigSpec{
				Endpoints:        []string{"http://192.168.0.10:2379"},
				DialTimeout:      2 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
				Secure: &clientv3.SecureConfig{
					InsecureTransport: false,
				},
			},
			expectedConf: clientv3.Config{
				Endpoints:            []string{"http://192.168.0.10:2379"},
				DialTimeout:          2 * time.Second,
				DialKeepAliveTime:    3 * time.Second,
				DialKeepAliveTimeout: 5 * time.Second,
				TLS:                  &tls.Config{},
			},
		},
		{
			name: "default secure transport and auth enabled",
			spec: clientv3.ConfigSpec{
				Endpoints:        []string{"http://192.168.0.12:2379"},
				DialTimeout:      1 * time.Second,
				KeepAliveTime:    4 * time.Second,
				KeepAliveTimeout: 6 * time.Second,
				Secure: &clientv3.SecureConfig{
					InsecureTransport: false,
				},
				Auth: &clientv3.AuthConfig{
					Username: "test",
					Password: "changeme",
				},
			},
			expectedConf: clientv3.Config{
				Endpoints:            []string{"http://192.168.0.12:2379"},
				DialTimeout:          1 * time.Second,
				DialKeepAliveTime:    4 * time.Second,
				DialKeepAliveTimeout: 6 * time.Second,
				TLS:                  &tls.Config{},
				Username:             "test",
				Password:             "changeme",
			},
		},
		{
			name: "default secure transport and skip TLS verification",
			spec: clientv3.ConfigSpec{
				Endpoints:        []string{"http://192.168.0.13:2379"},
				DialTimeout:      1 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
				Secure: &clientv3.SecureConfig{
					InsecureTransport:  false,
					InsecureSkipVerify: true,
				},
			},
			expectedConf: clientv3.Config{
				Endpoints:            []string{"http://192.168.0.13:2379"},
				DialTimeout:          1 * time.Second,
				DialKeepAliveTime:    3 * time.Second,
				DialKeepAliveTimeout: 5 * time.Second,
				TLS: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := newClientCfg(tc.spec.Endpoints, tc.spec.DialTimeout, tc.spec.KeepAliveTime, tc.spec.KeepAliveTimeout, tc.spec.Secure, tc.spec.Auth)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assert.Equal(t, tc.expectedConf, *cfg)
		})
	}
}

func TestNewClientConfigWithSecureCfg(t *testing.T) {
	tls, err := transport.SelfCert(zap.NewNop(), t.TempDir(), []string{"localhost"}, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	scfg := &clientv3.SecureConfig{
		Cert:   tls.CertFile,
		Key:    tls.KeyFile,
		Cacert: tls.TrustedCAFile,
	}

	cfg, err := newClientCfg([]string{"http://192.168.0.13:2379"}, 2*time.Second, 3*time.Second, 5*time.Second, scfg, nil)
	if cfg == nil || err != nil {
		t.Fatalf("Unexpected result client config: %v", err)
	}
}

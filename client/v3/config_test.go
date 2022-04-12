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

package clientv3

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.uber.org/zap"
)

func TestNewClientConfig(t *testing.T) {
	cases := []struct {
		name         string
		spec         ConfigSpec
		expectedConf Config
	}{
		{
			name: "only has basic info",
			spec: ConfigSpec{
				Endpoints:        []string{"http://192.168.0.10:2379"},
				DialTimeout:      2 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
			},
			expectedConf: Config{
				Endpoints:            []string{"http://192.168.0.10:2379"},
				DialTimeout:          2 * time.Second,
				DialKeepAliveTime:    3 * time.Second,
				DialKeepAliveTimeout: 5 * time.Second,
			},
		},
		{
			name: "auth enabled",
			spec: ConfigSpec{
				Endpoints:        []string{"http://192.168.0.12:2379"},
				DialTimeout:      1 * time.Second,
				KeepAliveTime:    4 * time.Second,
				KeepAliveTimeout: 6 * time.Second,
				Auth: &AuthConfig{
					Username: "test",
					Password: "changeme",
				},
			},
			expectedConf: Config{
				Endpoints:            []string{"http://192.168.0.12:2379"},
				DialTimeout:          1 * time.Second,
				DialKeepAliveTime:    4 * time.Second,
				DialKeepAliveTimeout: 6 * time.Second,
				Username:             "test",
				Password:             "changeme",
			},
		},
		{
			name: "default secure transport",
			spec: ConfigSpec{
				Endpoints:        []string{"http://192.168.0.10:2379"},
				DialTimeout:      2 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
				Secure: &SecureConfig{
					InsecureTransport: false,
				},
			},
			expectedConf: Config{
				Endpoints:            []string{"http://192.168.0.10:2379"},
				DialTimeout:          2 * time.Second,
				DialKeepAliveTime:    3 * time.Second,
				DialKeepAliveTimeout: 5 * time.Second,
				TLS:                  &tls.Config{},
			},
		},
		{
			name: "default secure transport and skip TLS verification",
			spec: ConfigSpec{
				Endpoints:        []string{"http://192.168.0.13:2379"},
				DialTimeout:      1 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
				Secure: &SecureConfig{
					InsecureTransport:  false,
					InsecureSkipVerify: true,
				},
			},
			expectedConf: Config{
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
			lg, _ := zap.NewProduction()

			cfg, err := NewClientConfig(&tc.spec, lg)
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

	scfg := &SecureConfig{
		Cert:   tls.CertFile,
		Key:    tls.KeyFile,
		Cacert: tls.TrustedCAFile,
	}

	cfg, err := NewClientConfig(&ConfigSpec{
		Endpoints:        []string{"http://192.168.0.13:2379"},
		DialTimeout:      2 * time.Second,
		KeepAliveTime:    3 * time.Second,
		KeepAliveTimeout: 5 * time.Second,
		Secure:           scfg,
	}, nil)
	if err != nil || cfg == nil || cfg.TLS == nil {
		t.Fatalf("Unexpected result client config: %v", err)
	}
}

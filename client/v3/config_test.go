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
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/client/pkg/v3/transport"
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
		{
			name: "insecure transport and skip TLS verification",
			spec: ConfigSpec{
				Endpoints:        []string{"http://192.168.0.13:2379"},
				DialTimeout:      1 * time.Second,
				KeepAliveTime:    3 * time.Second,
				KeepAliveTimeout: 5 * time.Second,
				Secure: &SecureConfig{
					InsecureTransport:  true,
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
			lg, _ := logutil.CreateDefaultZapLogger(zap.InfoLevel)

			cfg, err := NewClientConfig(&tc.spec, lg)
			require.NoError(t, err)

			assert.Equal(t, tc.expectedConf, *cfg)
		})
	}
}

func TestNewClientConfigWithSecureCfg(t *testing.T) {
	tls, err := transport.SelfCert(zap.NewNop(), t.TempDir(), []string{"localhost"}, 1)
	require.NoError(t, err)

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
	require.NoErrorf(t, err, "Unexpected result client config")
	if cfg == nil || cfg.TLS == nil {
		t.Fatalf("Unexpected result client config: %v", err)
	}
}

func TestConfigSpecClone(t *testing.T) {
	cfgSpec := &ConfigSpec{
		Endpoints:        []string{"ep1", "ep2", "ep3"},
		RequestTimeout:   10 * time.Second,
		DialTimeout:      2 * time.Second,
		KeepAliveTime:    5 * time.Second,
		KeepAliveTimeout: 2 * time.Second,

		Secure: &SecureConfig{
			Cert:               "path/2/cert",
			Key:                "path/2/key",
			Cacert:             "path/2/cacert",
			InsecureTransport:  true,
			InsecureSkipVerify: false,
		},

		Auth: &AuthConfig{
			Username: "foo",
			Password: "changeme",
		},
	}

	testCases := []struct {
		name          string
		cs            *ConfigSpec
		newEp         []string
		newSecure     *SecureConfig
		newAuth       *AuthConfig
		expectedEqual bool
	}{
		{
			name:          "normal case",
			cs:            cfgSpec,
			expectedEqual: true,
		},
		{
			name:          "point to a new slice of endpoint, but with the same data",
			cs:            cfgSpec,
			newEp:         []string{"ep1", "ep2", "ep3"},
			expectedEqual: true,
		},
		{
			name:          "update endpoint",
			cs:            cfgSpec,
			newEp:         []string{"ep1", "newep2", "ep3"},
			expectedEqual: false,
		},
		{
			name: "point to a new secureConfig, but with the same data",
			cs:   cfgSpec,
			newSecure: &SecureConfig{
				Cert:               "path/2/cert",
				Key:                "path/2/key",
				Cacert:             "path/2/cacert",
				InsecureTransport:  true,
				InsecureSkipVerify: false,
			},
			expectedEqual: true,
		},
		{
			name: "update key in secureConfig",
			cs:   cfgSpec,
			newSecure: &SecureConfig{
				Cert:               "path/2/cert",
				Key:                "newPath/2/key",
				Cacert:             "path/2/cacert",
				InsecureTransport:  true,
				InsecureSkipVerify: false,
			},
			expectedEqual: false,
		},
		{
			name: "update bool values in secureConfig",
			cs:   cfgSpec,
			newSecure: &SecureConfig{
				Cert:               "path/2/cert",
				Key:                "path/2/key",
				Cacert:             "path/2/cacert",
				InsecureTransport:  false,
				InsecureSkipVerify: true,
			},
			expectedEqual: false,
		},
		{
			name: "point to a new authConfig, but with the same data",
			cs:   cfgSpec,
			newAuth: &AuthConfig{
				Username: "foo",
				Password: "changeme",
			},
			expectedEqual: true,
		},
		{
			name: "update authConfig",
			cs:   cfgSpec,
			newAuth: &AuthConfig{
				Username: "newUser",
				Password: "newPassword",
			},
			expectedEqual: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dataBeforeTest, err := json.Marshal(tc.cs)
			require.NoError(t, err)

			clonedCfgSpec := tc.cs.Clone()
			if len(tc.newEp) > 0 {
				clonedCfgSpec.Endpoints = tc.newEp
			}
			if tc.newSecure != nil {
				clonedCfgSpec.Secure = tc.newSecure
			}
			if tc.newAuth != nil {
				clonedCfgSpec.Auth = tc.newAuth
			}

			actualEqual := reflect.DeepEqual(tc.cs, clonedCfgSpec)
			require.Equal(t, tc.expectedEqual, actualEqual)

			// double-check the original ConfigSpec isn't updated
			dataAfterTest, err := json.Marshal(tc.cs)
			require.NoError(t, err)
			require.True(t, reflect.DeepEqual(dataBeforeTest, dataAfterTest))
		})
	}
}

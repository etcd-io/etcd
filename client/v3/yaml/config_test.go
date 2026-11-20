// Copyright 2016 The etcd Authors
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

package yaml

import (
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

var (
	certPath       = "../../../tests/fixtures/server.crt"
	privateKeyPath = "../../../tests/fixtures/server.key.insecure"
	caPath         = "../../../tests/fixtures/ca.crt"
)

func TestNewConfigFileSizeLimit(t *testing.T) {
	const (
		limit = 1024 * 1024
		small = "endpoints:\n  - http://localhost:2379\n"
	)
	large := "endpoints:\n" + strings.Repeat("  - http://localhost:2379\n", 70000)
	large += "#" + strings.Repeat(" ", 2*limit-len(large)-1)
	tests := []struct {
		name string
		data string
	}{
		{name: "small", data: small},
		{name: "at limit", data: small + "#" + strings.Repeat(" ", limit-len(small)-1)},
		{name: "over limit", data: small + "#" + strings.Repeat(" ", limit-len(small))},
		{name: "oversized", data: large},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fpath := filepath.Join(t.TempDir(), "client.yaml")
			require.NoError(t, os.WriteFile(fpath, []byte(tt.data), 0o600))

			cfg, err := NewConfig(fpath)
			if len(tt.data) > limit {
				require.ErrorContains(t, err, "exceeds 1048576 bytes limit")
				require.ErrorContains(t, err, fpath)
				require.Nil(t, cfg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"http://localhost:2379"}, cfg.Endpoints)
		})
	}
}

func TestNewConfigTrustedCAFileSizeLimit(t *testing.T) {
	caPEM, err := os.ReadFile(caPath)
	require.NoError(t, err)
	for _, size := range []int{len(caPEM), 2 * 1024 * 1024} {
		dir := t.TempDir()
		cafile := filepath.Join(dir, "ca.pem")
		data := string(caPEM) + strings.Repeat("\n", size-len(caPEM))
		require.NoError(t, os.WriteFile(cafile, []byte(data), 0o600))
		configData, err := yaml.Marshal(&yamlConfig{TrustedCAfile: cafile})
		require.NoError(t, err)
		fpath := filepath.Join(dir, "client.yaml")
		require.NoError(t, os.WriteFile(fpath, configData, 0o600))

		cfg, err := NewConfig(fpath)
		if size > 1024*1024 {
			require.ErrorContains(t, err, "exceeds 1048576 bytes limit")
			require.ErrorContains(t, err, cafile)
			require.Nil(t, cfg)
			continue
		}
		require.NoError(t, err)
		require.NotNil(t, cfg.TLS.RootCAs)
	}
}

func TestConfigFromFile(t *testing.T) {
	tests := []struct {
		ym *yamlConfig

		werr bool
	}{
		{
			&yamlConfig{},
			false,
		},
		{
			&yamlConfig{
				InsecureTransport: true,
			},
			false,
		},
		{
			&yamlConfig{
				Keyfile:               privateKeyPath,
				Certfile:              certPath,
				TrustedCAfile:         caPath,
				InsecureSkipTLSVerify: true,
			},
			false,
		},
		{
			&yamlConfig{
				Keyfile:  "bad",
				Certfile: "bad",
			},
			true,
		},
		{
			&yamlConfig{
				Keyfile:       privateKeyPath,
				Certfile:      certPath,
				TrustedCAfile: "bad",
			},
			true,
		},
	}

	for i, tt := range tests {
		tmpfile, err := os.CreateTemp(t.TempDir(), "clientcfg")
		if err != nil {
			log.Fatal(err)
		}

		b, err := yaml.Marshal(tt.ym)
		require.NoError(t, err)

		_, err = tmpfile.Write(b)
		require.NoError(t, err)
		require.NoError(t, tmpfile.Close())

		cfg, cerr := NewConfig(tmpfile.Name())
		if cerr != nil && !tt.werr {
			t.Errorf("#%d: err = %v, want %v", i, cerr, tt.werr)
			continue
		}
		if cerr != nil {
			os.Remove(tmpfile.Name())
			continue
		}

		if !reflect.DeepEqual(cfg.Endpoints, tt.ym.Endpoints) {
			t.Errorf("#%d: endpoint = %v, want %v", i, cfg.Endpoints, tt.ym.Endpoints)
		}

		if tt.ym.InsecureTransport != (cfg.TLS == nil) {
			t.Errorf("#%d: insecureTransport = %v, want %v", i, cfg.TLS == nil, tt.ym.InsecureTransport)
		}

		if !tt.ym.InsecureTransport {
			if tt.ym.Certfile != "" && len(cfg.TLS.Certificates) == 0 {
				t.Errorf("#%d: failed to load in cert", i)
			}
			if tt.ym.TrustedCAfile != "" && cfg.TLS.RootCAs == nil {
				t.Errorf("#%d: failed to load in ca cert", i)
			}
			if cfg.TLS.InsecureSkipVerify != tt.ym.InsecureSkipTLSVerify {
				t.Errorf("#%d: skipTLSVeify = %v, want %v", i, cfg.TLS.InsecureSkipVerify, tt.ym.InsecureSkipTLSVerify)
			}
		}

		os.Remove(tmpfile.Name())
	}
}

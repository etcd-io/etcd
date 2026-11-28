// Copyright 2026 The etcd Authors
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

package tlsutil

import (
	"bytes"
	"crypto/x509"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCertFileSizeLimit(t *testing.T) {
	certPEM, err := os.ReadFile("../../../tests/fixtures/server.crt")
	require.NoError(t, err)
	keyPEM, err := os.ReadFile("../../../tests/fixtures/server.key.insecure")
	require.NoError(t, err)

	const limit = 1024 * 1024
	tests := []struct {
		name     string
		certSize int
		keySize  int
		wantFile string
	}{
		{name: "small", certSize: len(certPEM), keySize: len(keyPEM)},
		{name: "at limit", certSize: limit, keySize: limit},
		{name: "certificate over limit", certSize: limit + 1, keySize: len(keyPEM), wantFile: "cert.pem"},
		{name: "key over limit", certSize: len(certPEM), keySize: limit + 1, wantFile: "key.pem"},
		{name: "oversized pair", certSize: 2 * limit, keySize: 2 * limit, wantFile: "cert.pem"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			certfile := filepath.Join(dir, "cert.pem")
			keyfile := filepath.Join(dir, "key.pem")
			// Trailing whitespace keeps the padded PEM files valid.
			certData := append(bytes.Clone(certPEM), bytes.Repeat([]byte("\n"), tt.certSize-len(certPEM))...)
			keyData := append(bytes.Clone(keyPEM), bytes.Repeat([]byte("\n"), tt.keySize-len(keyPEM))...)
			require.NoError(t, os.WriteFile(certfile, certData, 0o600))
			require.NoError(t, os.WriteFile(keyfile, keyData, 0o600))

			cert, err := NewCert(certfile, keyfile, nil)
			if tt.wantFile != "" {
				require.ErrorContains(t, err, "exceeds 1048576 bytes limit")
				require.ErrorContains(t, err, filepath.Join(dir, tt.wantFile))
				require.Nil(t, cert)
				return
			}
			require.NoError(t, err)
			require.NotEmpty(t, cert.Certificate)
			require.NotNil(t, cert.PrivateKey)
		})
	}
}

func TestNewCertPoolFileSizeLimit(t *testing.T) {
	caPEM, err := os.ReadFile("../../../tests/fixtures/ca.crt")
	require.NoError(t, err)
	wantPool := x509.NewCertPool()
	require.True(t, wantPool.AppendCertsFromPEM(caPEM))

	const limit = 1024 * 1024
	tests := []struct {
		name string
		size int
	}{
		{name: "small", size: len(caPEM)},
		{name: "at limit", size: limit},
		{name: "over limit", size: limit + 1},
		{name: "oversized", size: 2 * limit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cafile := filepath.Join(t.TempDir(), "ca.pem")
			data := append(bytes.Clone(caPEM), bytes.Repeat([]byte("\n"), tt.size-len(caPEM))...)
			require.NoError(t, os.WriteFile(cafile, data, 0o600))

			pool, err := NewCertPool([]string{cafile})
			if tt.size > limit {
				require.ErrorContains(t, err, "exceeds 1048576 bytes limit")
				require.ErrorContains(t, err, cafile)
				require.Nil(t, pool)
				return
			}
			require.NoError(t, err)
			require.True(t, pool.Equal(wantPool))
		})
	}
}

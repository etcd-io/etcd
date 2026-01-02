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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// generateTestCA creates a self-signed CA certificate for testing
func generateTestCA(t *testing.T) []byte {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}

func TestNewCAReloader(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, generateTestCA(t), 0600)
	require.NoError(t, err)

	// Create CAReloader
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	require.NotNil(t, reloader)

	// Check that pool is loaded
	pool := reloader.GetCertPool()
	require.NotNil(t, pool)
}

func TestNewCAReloader_InvalidFile(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Try to create CAReloader with non-existent file
	_, err := NewCAReloader([]string{"/nonexistent/ca.crt"}, logger)
	require.Error(t, err)
}

func TestNewCAReloader_InvalidCert(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and invalid CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, []byte("invalid cert data"), 0600)
	require.NoError(t, err)

	// Create CAReloader - should fail because cert is invalid
	_, err = NewCAReloader([]string{caFile}, logger)
	require.Error(t, err)
}

func TestCAReloader_Reload(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Generate two different CA certs
	ca1 := generateTestCA(t)
	ca2 := generateTestCA(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, ca1, 0600)
	require.NoError(t, err)

	// Create CAReloader with short interval
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	reloader.WithInterval(50 * time.Millisecond)
	reloader.Start()
	t.Cleanup(func() { reloader.Stop() })

	// Get initial pool
	pool1 := reloader.GetCertPool()
	require.NotNil(t, pool1)

	// Update CA file with different cert
	err = os.WriteFile(caFile, ca2, 0600)
	require.NoError(t, err)

	// Wait for reload
	time.Sleep(100 * time.Millisecond)

	// Get updated pool
	pool2 := reloader.GetCertPool()
	require.NotNil(t, pool2)

	// Pools should be different (different CA was loaded)
	require.NotEqual(t, pool1, pool2)
}

func TestCAReloader_GracefulFallback(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, generateTestCA(t), 0600)
	require.NoError(t, err)

	// Create CAReloader with short interval
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	reloader.WithInterval(50 * time.Millisecond)
	reloader.Start()
	t.Cleanup(func() { reloader.Stop() })

	// Get initial pool
	pool1 := reloader.GetCertPool()
	require.NotNil(t, pool1)

	// Write invalid data to CA file
	err = os.WriteFile(caFile, []byte("invalid cert data"), 0600)
	require.NoError(t, err)

	// Wait for reload attempt
	time.Sleep(100 * time.Millisecond)

	// Pool should still be the old one (graceful fallback)
	pool2 := reloader.GetCertPool()
	require.NotNil(t, pool2)
	require.Equal(t, pool1, pool2)
}

func TestCAReloader_Concurrency(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Generate two different CA certs for alternating writes
	ca1 := generateTestCA(t)
	ca2 := generateTestCA(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, ca1, 0600)
	require.NoError(t, err)

	// Create CAReloader with short interval
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	reloader.WithInterval(10 * time.Millisecond)
	reloader.Start()
	t.Cleanup(func() { reloader.Stop() })

	// Run concurrent reads and writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				pool := reloader.GetCertPool()
				assert.NotNil(t, pool)
			}
		}()
	}

	// Also update the file concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			ca := ca1
			if j%2 == 0 {
				ca = ca2
			}
			_ = os.WriteFile(caFile, ca, 0600)
			time.Sleep(5 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func TestCAReloader_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, generateTestCA(t), 0600)
	require.NoError(t, err)

	// Create CAReloader
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	reloader.WithInterval(10 * time.Millisecond)
	reloader.Start()

	// Stop should complete without hanging
	done := make(chan struct{})
	go func() {
		reloader.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() did not complete in time")
	}
}

func TestCAReloader_StopWithoutStart(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, generateTestCA(t), 0600)
	require.NoError(t, err)

	// Create CAReloader but don't call Start()
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)

	// Stop should complete without hanging even though Start was never called
	done := make(chan struct{})
	go func() {
		reloader.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() without Start() should not deadlock")
	}
}

func TestCAReloader_NoChangeNoReload(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create temp dir and CA file
	tmpDir := t.TempDir()
	caFile := filepath.Join(tmpDir, "ca.crt")
	err := os.WriteFile(caFile, generateTestCA(t), 0600)
	require.NoError(t, err)

	// Create CAReloader with short interval
	reloader, err := NewCAReloader([]string{caFile}, logger)
	require.NoError(t, err)
	reloader.WithInterval(50 * time.Millisecond)
	reloader.Start()
	t.Cleanup(func() { reloader.Stop() })

	// Get initial pool
	pool1 := reloader.GetCertPool()
	require.NotNil(t, pool1)

	// Wait for reload cycle without changing file
	time.Sleep(100 * time.Millisecond)

	// Pool should be the same object (no reload occurred)
	pool2 := reloader.GetCertPool()
	require.Equal(t, pool1, pool2)
}

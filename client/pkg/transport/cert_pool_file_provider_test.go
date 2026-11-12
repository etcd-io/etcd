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

package transport

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// poolVerifies reports whether the certificate stored in certFile chains up
// to the given pool. Self-signed test certificates verify against pools that
// contain themselves.
func poolVerifies(pool *x509.CertPool, certFile string) bool {
	cert, err := mustLoadCertificateE(certFile)
	if err != nil {
		return false
	}
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	})
	return err == nil
}

func mustLoadCertificateE(certFile string) (*x509.Certificate, error) {
	pemBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	return x509.ParseCertificate(block.Bytes)
}

func mustWriteCAFile(t *testing.T, dir, name string, info *TLSInfo) string {
	t.Helper()
	content, err := os.ReadFile(info.CertFile)
	require.NoError(t, err)
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func mustOverwriteFile(t *testing.T, dst, src string) {
	t.Helper()
	content, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, content, 0o600))
}

func TestFileCertPoolProvider_RequiresAtLeastOneFile(t *testing.T) {
	_, err := NewFileCertPoolProvider(zaptest.NewLogger(t))
	require.Error(t, err)
}

func TestFileCertPoolProvider_InitialLoadFailsOnInvalidPEM(t *testing.T) {
	f := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(f, []byte("not a pem bundle"), 0o600))
	_, err := NewFileCertPoolProvider(zaptest.NewLogger(t), f)
	require.Error(t, err)
}

func TestFileCertPoolProvider_InitialLoadFailsOnMissingFile(t *testing.T) {
	_, err := NewFileCertPoolProvider(zaptest.NewLogger(t), filepath.Join(t.TempDir(), "absent.pem"))
	require.Error(t, err)
}

func TestFileCertPoolProvider_ReloadPicksUpRotatedCA(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := NewFileCertPoolProvider(zaptest.NewLogger(t), caFile)
	require.NoError(t, err)
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))
	require.False(t, poolVerifies(p.GetCertPool(), ca2.CertFile))

	mustOverwriteFile(t, caFile, ca2.CertFile)
	require.NoError(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
	require.False(t, poolVerifies(p.GetCertPool(), ca1.CertFile))
}

func TestFileCertPoolProvider_UnchangedFilesKeepSamePool(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := NewFileCertPoolProvider(zaptest.NewLogger(t), caFile)
	require.NoError(t, err)

	before := p.GetCertPool()
	require.NoError(t, p.Reload())
	require.Same(t, before, p.GetCertPool())
}

func TestFileCertPoolProvider_ReloadKeepsLastGoodPoolOnMalformedFile(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := NewFileCertPoolProvider(zaptest.NewLogger(t), caFile)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(caFile, []byte("garbage"), 0o600))
	require.Error(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))

	// Rotating back to a valid CA recovers.
	mustOverwriteFile(t, caFile, ca2.CertFile)
	require.NoError(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
}

func TestFileCertPoolProvider_ReloadKeepsLastGoodPoolOnMissingFile(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := NewFileCertPoolProvider(zaptest.NewLogger(t), caFile)
	require.NoError(t, err)

	require.NoError(t, os.Remove(caFile))
	require.Error(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))

	// Recreating the file recovers.
	mustOverwriteFile(t, caFile, ca2.CertFile)
	require.NoError(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
}

func TestFileCertPoolProvider_MultipleFilesReloadedTogether(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)
	ca3, err := createSelfCertEx(t, "ca3")
	require.NoError(t, err)

	dir := t.TempDir()
	f1 := mustWriteCAFile(t, dir, "ca1.pem", ca1)
	f2 := mustWriteCAFile(t, dir, "ca2.pem", ca2)

	p, err := NewFileCertPoolProvider(zaptest.NewLogger(t), f1, f2)
	require.NoError(t, err)
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))

	// Losing one file keeps the last good pool serving both CAs.
	require.NoError(t, os.Remove(f2))
	require.Error(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))

	// Rotating one file refreshes the pool with the union of both files.
	mustOverwriteFile(t, f1, ca3.CertFile)
	ca2Content, err := os.ReadFile(ca2.CertFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(f2, ca2Content, 0o600))
	require.NoError(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca3.CertFile))
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
	require.False(t, poolVerifies(p.GetCertPool(), ca1.CertFile))
}

func TestFileCertPoolProvider_PollerPicksUpRotation(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := newFileCertPoolProvider(zaptest.NewLogger(t), 10*time.Millisecond, caFile)
	require.NoError(t, err)
	p.Start()

	mustOverwriteFile(t, caFile, ca2.CertFile)
	require.Eventually(t, func() bool {
		return poolVerifies(p.GetCertPool(), ca2.CertFile)
	}, 5*time.Second, 10*time.Millisecond)
	require.False(t, poolVerifies(p.GetCertPool(), ca1.CertFile))

	p.Close()
}

func TestFileCertPoolProvider_CloseStopsPoller(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := newFileCertPoolProvider(zaptest.NewLogger(t), 10*time.Millisecond, caFile)
	require.NoError(t, err)
	p.Start()
	p.Close()

	mustOverwriteFile(t, caFile, ca2.CertFile)
	// The poller would have ticked ~10 times within this window if it had
	// not been stopped.
	time.Sleep(100 * time.Millisecond)
	require.False(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
	require.True(t, poolVerifies(p.GetCertPool(), ca1.CertFile))

	// Manual reloads still work after Close.
	require.NoError(t, p.Reload())
	require.True(t, poolVerifies(p.GetCertPool(), ca2.CertFile))
}

func TestFileCertPoolProvider_ConcurrentGetCertPoolDuringReloads(t *testing.T) {
	ca1, err := createSelfCertEx(t, "ca1")
	require.NoError(t, err)
	ca2, err := createSelfCertEx(t, "ca2")
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "ca.pem", ca1)
	p, err := newFileCertPoolProvider(zaptest.NewLogger(t), time.Hour, caFile)
	require.NoError(t, err)

	var wg sync.WaitGroup
	stopc := make(chan struct{})
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stopc:
					return
				default:
					require.NotNil(t, p.GetCertPool())
				}
			}
		}()
	}

	current := ca1
	for i := 0; i < 20; i++ {
		if current == ca1 {
			current = ca2
		} else {
			current = ca1
		}
		mustOverwriteFile(t, caFile, current.CertFile)
		require.NoError(t, p.Reload())
	}

	close(stopc)
	wg.Wait()
	require.True(t, poolVerifies(p.GetCertPool(), current.CertFile))
}

// TestFileCertPoolProvider_ServerConfigPicksUpRotatedClientCA exercises the
// full hook-plus-backend path: a listener built from a TLSInfo configured
// with a FileCertPoolProvider accepts client certificates signed by a CA
// that was rotated on disk, without rebuilding the listener.
func TestFileCertPoolProvider_ServerConfigPicksUpRotatedClientCA(t *testing.T) {
	serverTLSInfo, err := createSelfCert(t)
	require.NoError(t, err)

	clientCA1, err := createSelfCertEx(t, "127.0.0.1", x509.ExtKeyUsageClientAuth)
	require.NoError(t, err)
	clientCA2, err := createSelfCertEx(t, "127.0.0.1", x509.ExtKeyUsageClientAuth)
	require.NoError(t, err)

	caFile := mustWriteCAFile(t, t.TempDir(), "client-ca.pem", clientCA1)
	p, err := newFileCertPoolProvider(zaptest.NewLogger(t), 10*time.Millisecond, caFile)
	require.NoError(t, err)
	p.Start()
	defer p.Close()

	serverTLSInfo.ClientCertAuth = true
	serverTLSInfo.Logger = zaptest.NewLogger(t)
	serverTLSInfo.SetDynamicTrustRoots(p)

	ln := mustNewTLSListener(t, serverTLSInfo)
	defer ln.Close()

	rootCAs := mustCertPoolFromFile(t, serverTLSInfo.CertFile)
	dial := func(clientCA *TLSInfo) {
		t.Helper()
		conn := mustDialTLS(t, ln.Addr().String(), &tls.Config{
			Certificates: []tls.Certificate{mustLoadKeyPair(t, clientCA.CertFile, clientCA.KeyFile)},
			RootCAs:      rootCAs,
		})
		conn.Close()
		serverConn := mustAcceptConn(t, ln)
		serverConn.Close()
	}

	dial(clientCA1)

	mustOverwriteFile(t, caFile, clientCA2.CertFile)
	require.Eventually(t, func() bool {
		return poolVerifies(p.GetCertPool(), clientCA2.CertFile)
	}, 5*time.Second, 10*time.Millisecond)

	dial(clientCA2)
}

// TestFileCertPoolProvider_ClientConfigPicksUpRotatedServerCA exercises the
// client-side path: a client verifying server certificates against a
// FileCertPoolProvider accepts a rotated server identity without changing
// the client config.
func TestFileCertPoolProvider_ClientConfigPicksUpRotatedServerCA(t *testing.T) {
	serverCA1, err := createSelfCertEx(t, "127.0.0.1")
	require.NoError(t, err)
	serverCA2, err := createSelfCertEx(t, "127.0.0.1")
	require.NoError(t, err)

	stableCertFile, stableKeyFile := mustWriteStableTLSFiles(t, serverCA1.CertFile, serverCA1.KeyFile)
	serverInfo := &TLSInfo{
		CertFile: stableCertFile,
		KeyFile:  stableKeyFile,
		Logger:   zaptest.NewLogger(t),
	}

	ln := mustNewTLSListener(t, serverInfo)
	defer ln.Close()

	caFile := mustWriteCAFile(t, t.TempDir(), "server-ca.pem", serverCA1)
	p, err := newFileCertPoolProvider(zaptest.NewLogger(t), 10*time.Millisecond, caFile)
	require.NoError(t, err)
	p.Start()
	defer p.Close()

	clientInfo := TLSInfo{ServerName: "127.0.0.1"}
	clientInfo.SetDynamicTrustRoots(p)

	clientCfg, err := clientInfo.ClientConfig()
	require.NoError(t, err)

	conn1 := mustDialTLS(t, ln.Addr().String(), clientCfg)
	serverConn1 := mustAcceptConn(t, ln)
	conn1.Close()
	serverConn1.Close()

	mustReplaceTLSFiles(t, serverCA2.CertFile, serverCA2.KeyFile, stableCertFile, stableKeyFile)
	mustOverwriteFile(t, caFile, serverCA2.CertFile)
	require.Eventually(t, func() bool {
		return poolVerifies(p.GetCertPool(), serverCA2.CertFile)
	}, 5*time.Second, 10*time.Millisecond)

	conn2 := mustDialTLS(t, ln.Addr().String(), clientCfg)
	serverConn2 := mustAcceptConn(t, ln)
	require.Equal(t, mustLoadCertificate(t, serverCA2.CertFile).Raw, conn2.ConnectionState().PeerCertificates[0].Raw)
	conn2.Close()
	serverConn2.Close()
}

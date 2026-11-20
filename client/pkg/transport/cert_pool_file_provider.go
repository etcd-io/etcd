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
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DefaultCAReloadInterval is the polling interval used by a
// FileCertPoolProvider when no interval is configured.
const DefaultCAReloadInterval = 30 * time.Second

// FileCertPoolProvider is a poll-based, file-backed CertPoolProvider. It
// periodically reloads the x509 CA bundles found in the configured PEM files
// and serves the latest pool to every TLS handshake consulting it through
// TLSInfo.SetDynamicTrustRoots, so trusted CA files can be rotated in place
// without restarting etcd.
//
// A FileCertPoolProvider is safe for concurrent use by multiple handshakes.
// Reload failures never break handshakes: if a file is missing, unreadable,
// or no longer parses, the provider keeps serving the last good pool and
// logs a warning, so operators can rotate bundles atomically (e.g. via
// symlink or rename) without a window in which verification would fail.
type FileCertPoolProvider struct {
	lg       *zap.Logger
	interval time.Duration
	files    []string

	// mu guards pool and state. GetCertPool takes the read lock on every
	// handshake, so it must only protect pointer access, never file IO.
	mu    sync.RWMutex
	pool  *x509.CertPool
	state map[string]string

	stopc chan struct{}
	wg    sync.WaitGroup
}

// NewFileCertPoolProvider builds a provider serving the CA bundles found in
// the given PEM files, polling them every DefaultCAReloadInterval. All files
// must currently contain valid PEM certificates, so configuration mistakes
// surface at startup instead of at the first rotated handshake.
func NewFileCertPoolProvider(lg *zap.Logger, files ...string) (*FileCertPoolProvider, error) {
	return newFileCertPoolProvider(lg, DefaultCAReloadInterval, files...)
}

func newFileCertPoolProvider(lg *zap.Logger, interval time.Duration, files ...string) (*FileCertPoolProvider, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("file cert pool provider requires at least one CA file")
	}
	if interval <= 0 {
		interval = DefaultCAReloadInterval
	}
	if lg == nil {
		lg = zap.NewNop()
	}
	p := &FileCertPoolProvider{
		lg:       lg,
		interval: interval,
		files:    append([]string(nil), files...),
		state:    make(map[string]string, len(files)),
		stopc:    make(chan struct{}),
	}
	if err := p.Reload(); err != nil {
		return nil, err
	}
	return p, nil
}

// GetCertPool implements CertPoolProvider. It returns the latest
// successfully loaded pool; it never returns nil because the constructor
// only succeeds once at least one pool has been loaded.
func (p *FileCertPoolProvider) GetCertPool() *x509.CertPool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pool
}

// Reload re-reads every configured CA file. If all files still hash to their
// last seen content, the current pool is kept untouched. If any file changed,
// a fresh pool is built and atomically swapped in. If a file cannot be read
// or does not contain any valid PEM certificate, the previously loaded pool
// is kept and an error is returned.
func (p *FileCertPoolProvider) Reload() error {
	contents := make(map[string][]byte, len(p.files))
	state := make(map[string]string, len(p.files))
	for _, f := range p.files {
		b, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("failed to read CA file %q: %w", f, err)
		}
		contents[f] = b
		state[f] = hashPEMContent(b)
	}

	p.mu.RLock()
	unchanged := len(p.state) == len(state)
	if unchanged {
		for f, h := range state {
			if p.state[f] != h {
				unchanged = false
				break
			}
		}
	}
	p.mu.RUnlock()
	if unchanged {
		return nil
	}

	pool := x509.NewCertPool()
	for _, f := range p.files {
		if !pool.AppendCertsFromPEM(contents[f]) {
			return fmt.Errorf("CA file %q does not contain any valid PEM certificate", f)
		}
	}

	p.mu.Lock()
	p.pool = pool
	p.state = state
	p.mu.Unlock()
	p.lg.Info("reloaded trusted CA files", zap.Strings("files", p.files))
	return nil
}

// Start launches the background poller. Close stops it again; Start must be
// called at most once.
func (p *FileCertPoolProvider) Start() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stopc:
				return
			case <-ticker.C:
				if err := p.Reload(); err != nil {
					p.lg.Warn(
						"failed to reload trusted CA files, keeping last good pool",
						zap.Strings("files", p.files),
						zap.Error(err),
					)
				}
			}
		}
	}()
}

// Close stops the background poller and waits for it to return. The provider
// keeps serving the last loaded pool afterwards, so in-flight configs stay
// usable until they are discarded.
func (p *FileCertPoolProvider) Close() {
	close(p.stopc)
	p.wg.Wait()
}

func hashPEMContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

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

package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/expect"
)

func newLazyFS(lg *zap.Logger, dataDir string, tmp TempDirProvider) *LazyFS {
	return &LazyFS{
		lg:        lg,
		DataDir:   dataDir,
		LazyFSDir: tmp.TempDir(),
	}
}

type TempDirProvider interface {
	TempDir() string
}

type LazyFS struct {
	lg *zap.Logger

	DataDir   string
	LazyFSDir string

	ep *expect.ExpectProcess
}

func (fs *LazyFS) Start(ctx context.Context) (err error) {
	if fs.ep != nil {
		return nil
	}
	err = os.WriteFile(fs.configPath(), fs.config(), 0666)
	if err != nil {
		return err
	}
	dataPath := filepath.Join(fs.LazyFSDir, "data")
	err = os.Mkdir(dataPath, 0700)
	if err != nil {
		return err
	}
	flags := []string{fs.DataDir, "--config-path", fs.configPath(), "-o", "modules=subdir", "-o", "subdir=" + dataPath, "-f"}
	fs.lg.Info("Started lazyfs", zap.Strings("flags", flags))
	fs.ep, err = expect.NewExpect(BinPath.LazyFS, flags...)
	if err != nil {
		return err
	}
	_, err = fs.ep.ExpectWithContext(ctx, expect.ExpectedResponse{Value: "waiting for fault commands"})
	return err
}

func (fs *LazyFS) configPath() string {
	return filepath.Join(fs.LazyFSDir, "config.toml")
}

func (fs *LazyFS) socketPath() string {
	return filepath.Join(fs.LazyFSDir, "sock.fifo")
}

func (fs *LazyFS) config() []byte {
	return []byte(fmt.Sprintf(`[faults]
fifo_path=%q
[cache]
apply_eviction=false
[cache.simple]
custom_size="1gb"
blocks_per_page=1
[filesystem]
log_all_operations=false
`, fs.socketPath()))
}

func (fs *LazyFS) Stop() error {
	if fs.ep == nil {
		return nil
	}
	defer func() { fs.ep = nil }()
	err := fs.ep.Stop()
	if err != nil {
		return err
	}
	return fs.ep.Close()
}

func (fs *LazyFS) ClearCache(ctx context.Context) error {
	err := os.WriteFile(fs.socketPath(), []byte("lazyfs::clear-cache\n"), 0666)
	if err != nil {
		return err
	}
	// TODO: Wait for response on socket instead of reading logs to get command completion.
	// Set `fifo_path_completed` config for LazyFS to create separate socket to write when it has completed command.
	_, err = fs.ep.ExpectWithContext(ctx, expect.ExpectedResponse{Value: "cache is cleared"})
	return err
}

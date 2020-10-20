// Copyright 2018 The etcd Authors
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

package snapshot

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/dustin/go-humanize"
	"go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/fileutil"
	"go.uber.org/zap"
)

// hasChecksum returns "true" if the file size "n"
// has appended sha256 hash digest.
func hasChecksum(n int64) bool {
	// 512 is chosen because it's a minimum disk sector size
	// smaller than (and multiplies to) OS page size in most systems
	return (n % 512) == sha256.Size
}

// Save fetches snapshot from remote etcd server and saves data
// to target path. If the context "ctx" is canceled or timed out,
// snapshot save stream will error out (e.g. context.Canceled,
// context.DeadlineExceeded). Make sure to specify only one endpoint
// in client configuration. Snapshot API must be requested to a
// selected node, and saved snapshot is the point-in-time state of
// the selected node.
func Save(ctx context.Context, lg *zap.Logger, cfg clientv3.Config, dbPath string) error {
	if lg == nil {
		lg = zap.NewExample()
	}
	if len(cfg.Endpoints) != 1 {
		return fmt.Errorf("snapshot must be requested to one selected node, not multiple %v", cfg.Endpoints)
	}
	cli, err := clientv3.New(cfg)
	if err != nil {
		return err
	}
	defer cli.Close()

	partpath := dbPath + ".part"
	defer os.RemoveAll(partpath)

	var f *os.File
	f, err = os.OpenFile(partpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileutil.PrivateFileMode)
	if err != nil {
		return fmt.Errorf("could not open %s (%v)", partpath, err)
	}
	lg.Info("created temporary db file", zap.String("path", partpath))

	now := time.Now()
	var rd io.ReadCloser
	rd, err = cli.Snapshot(ctx)
	if err != nil {
		return err
	}
	lg.Info("fetching snapshot", zap.String("endpoint", cfg.Endpoints[0]))
	var size int64
	size, err = io.Copy(f, rd)
	if err != nil {
		return err
	}
	if !hasChecksum(size) {
		return fmt.Errorf("sha256 checksum not found [bytes: %d]", size)
	}
	if err = fileutil.Fsync(f); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	lg.Info("fetched snapshot",
		zap.String("endpoint", cfg.Endpoints[0]),
		zap.String("size", humanize.Bytes(uint64(size))),
		zap.String("took", humanize.Time(now)),
	)

	if err = os.Rename(partpath, dbPath); err != nil {
		return fmt.Errorf("could not rename %s to %s (%v)", partpath, dbPath, err)
	}
	lg.Info("saved", zap.String("path", dbPath))
	return nil
}

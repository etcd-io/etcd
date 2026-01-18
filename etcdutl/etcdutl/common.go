// Copyright 2021 The etcd Authors
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

package etcdutl

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/wal"
	"go.etcd.io/etcd/server/v3/storage/wal/walpb"
)

// FlockTimeout is the duration to wait to obtain a file lock on db file.
var FlockTimeout time.Duration

func GetLogger() *zap.Logger {
	config := logutil.DefaultZapLoggerConfig
	config.Encoding = "console"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	lg, err := config.Build()
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
	}
	return lg
}

func getLatestWALSnap(lg *zap.Logger, dataDir string) (walpb.Snapshot, error) {
	walPath := datadir.ToWALDir(dataDir)
	walSnaps, err := wal.ValidSnapshotEntries(lg, walPath)
	if err != nil {
		return walpb.Snapshot{}, err
	}

	if len(walSnaps) > 0 {
		lastIdx := len(walSnaps) - 1
		return walSnaps[lastIdx], nil
	}
	return walpb.Snapshot{}, nil
}

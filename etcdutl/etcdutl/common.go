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
	"os"

	"go.etcd.io/etcd/client/pkg/v3/logutil"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/datadir"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// validateDataDir checks if the data directory's backend database file exists.
func validateDataDir(dataDir string) error {
	dbPath := datadir.ToBackendFileName(dataDir)
	return validateFilePath(dbPath)
}

// validateFilePath checks if the specified file exists.
// It returns an error encountered by os.Stat, such as os.ErrNotExist, os.ErrPermission, etc.
func validateFilePath(filePath string) error {
	_, err := os.Stat(filePath)
	return err
}

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

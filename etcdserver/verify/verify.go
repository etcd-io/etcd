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

package verify

import (
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/mvcc"
	"go.etcd.io/etcd/mvcc/backend"
	"go.etcd.io/etcd/pkg/fileutil"
	"go.etcd.io/etcd/version"
	"go.uber.org/zap"
)

const ENV_VERIFY = "ETCD_VERIFY"
const ENV_VERIFY_ALL_VALUE = "all"

type Config struct {
	// DataDir is a root directory where the data being verified are stored.
	DataDir string

	Logger *zap.Logger
}

// Verify performs consistency checks of given etcd data-directory.
// The errors are reported as the returned error, but for some situations
// the function can also panic.
// The function is expected to work on not-in-use data model, i.e.
// no file-locks should be taken. Verify does not modified the data.
func Verify(cfg Config) error {
	lg := cfg.Logger
	if lg == nil {
		lg = zap.NewNop()
	}

	if !fileutil.Exist(toBackendFileName(cfg.DataDir)) {
		lg.Info("verification skipped due to non exist db file")
		return nil
	}

	var err error
	lg.Info("verification of persisted state", zap.String("data-dir", cfg.DataDir))
	defer func() {
		if err != nil {
			lg.Error("verification of persisted state failed",
				zap.String("data-dir", cfg.DataDir),
				zap.Error(err))
		} else if r := recover(); r != nil {
			lg.Error("verification of persisted state failed",
				zap.String("data-dir", cfg.DataDir))
			panic(r)
		} else {
			lg.Info("verification of persisted state successful", zap.String("data-dir", cfg.DataDir))
		}
	}()

	beConfig := backend.DefaultBackendConfig()
	beConfig.Path = toBackendFileName(cfg.DataDir)
	beConfig.Logger = cfg.Logger

	be := backend.New(beConfig)
	defer be.Close()

	err = validateSchema(lg, be)
	return err
}

// VerifyIfEnabled performs verification according to ETCD_VERIFY env settings.
// See Verify for more information.
func VerifyIfEnabled(cfg Config) error {
	if os.Getenv(ENV_VERIFY) == ENV_VERIFY_ALL_VALUE {
		return Verify(cfg)
	}
	return nil
}

// MustVerifyIfEnabled performs verification according to ETCD_VERIFY env settings
// and exits in case of found problems.
// See Verify for more information.
func MustVerifyIfEnabled(cfg Config) {
	if err := VerifyIfEnabled(cfg); err != nil {
		cfg.Logger.Fatal("Verification failed",
			zap.String("data-dir", cfg.DataDir),
			zap.Error(err))
	}
}

func validateSchema(lg *zap.Logger, be backend.Backend) error {
	be.ReadTx().RLock()
	defer be.ReadTx().RUnlock()
	v := mvcc.UnsafeDetectSchemaVersion(lg, be.ReadTx())
	if !v.Equal(version.V3_4) {
		return fmt.Errorf("detected unsupported data schema: %s", v.String())
	}
	return nil
}

func toBackendFileName(dataDir string) string {
	return filepath.Join(dataDir, "member", "snap", "db")
}

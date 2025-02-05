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
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/api/v3/version"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"
)

// NewMigrateCommand prints out the version of etcd.
func NewMigrateCommand() *cobra.Command {
	o := newMigrateOptions()
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrates schema of etcd data dir files to make them compatible with different etcd version",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := o.Config()
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitBadArgs, err)
			}
			err = migrateCommandFunc(cfg)
			if err != nil {
				cobrautl.ExitWithError(cobrautl.ExitError, err)
			}
		},
	}
	o.AddFlags(cmd)
	return cmd
}

type migrateOptions struct {
	dataDir       string
	targetVersion string
	force         bool
}

func newMigrateOptions() *migrateOptions {
	return &migrateOptions{}
}

func (o *migrateOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.dataDir, "data-dir", o.dataDir, "Path to the etcd data dir")
	cmd.MarkFlagRequired("data-dir")
	cmd.MarkFlagDirname("data-dir")

	cmd.Flags().StringVar(&o.targetVersion, "target-version", o.targetVersion, `Target etcd version to migrate contents of data dir. Minimal value 3.5. Format "X.Y" for example 3.6.`)
	cmd.MarkFlagRequired("target-version")

	cmd.Flags().BoolVar(&o.force, "force", o.force, "Ignore migration failure and forcefully override storage version. Not recommended.")
}

func (o *migrateOptions) Config() (*migrateConfig, error) {
	c := &migrateConfig{
		force:   o.force,
		dataDir: o.dataDir,
		lg:      GetLogger(),
	}
	var err error
	dotCount := strings.Count(o.targetVersion, ".")
	if dotCount != 1 {
		return nil, fmt.Errorf(`wrong target version format, expected "X.Y", got %q`, o.targetVersion)
	}
	c.targetVersion, err = semver.NewVersion(o.targetVersion + ".0")
	if err != nil {
		return nil, fmt.Errorf("failed to parse target version: %w", err)
	}
	if c.targetVersion.LessThan(version.V3_5) {
		return nil, fmt.Errorf(`target version %q not supported. Minimal "3.5"`, storageVersionToString(c.targetVersion))
	}

	return c, nil
}

type migrateConfig struct {
	lg            *zap.Logger
	targetVersion *semver.Version
	walVersion    wal.Version
	dataDir       string
	force         bool
}

func (c *migrateConfig) finalize() error {
	walPath := datadir.ToWALDir(c.dataDir)
	walSnap, err := getLatestWALSnap(c.lg, c.dataDir)
	if err != nil {
		return fmt.Errorf("failed to get the lastest snapshot: %w", err)
	}
	w, err := wal.OpenForRead(c.lg, walPath, walSnap)
	if err != nil {
		return fmt.Errorf(`failed to open wal: %w`, err)
	}
	defer w.Close()
	c.walVersion, err = wal.ReadWALVersion(w)
	if err != nil {
		return fmt.Errorf(`failed to read wal: %w`, err)
	}

	return nil
}

func migrateCommandFunc(c *migrateConfig) error {
	dbPath := datadir.ToBackendFileName(c.dataDir)
	be := backend.NewDefaultBackend(GetLogger(), dbPath)
	defer be.Close()

	tx := be.BatchTx()
	current, err := schema.DetectSchemaVersion(c.lg, be.ReadTx())
	if err != nil {
		c.lg.Error("failed to detect storage version. Please make sure you are using data dir from etcd v3.5 and older")
		return err
	}
	if current == *c.targetVersion {
		c.lg.Info("storage version up-to-date", zap.String("storage-version", storageVersionToString(&current)))
		return nil
	}

	if err = c.finalize(); err != nil {
		c.lg.Error("Failed to finalize config", zap.Error(err))
		return err
	}

	err = schema.Migrate(c.lg, tx, c.walVersion, *c.targetVersion)
	if err != nil {
		if !c.force {
			return err
		}
		c.lg.Info("normal migrate failed, trying with force", zap.Error(err))
		migrateForce(c.lg, tx, c.targetVersion)
	}
	be.ForceCommit()
	return nil
}

func migrateForce(lg *zap.Logger, tx backend.BatchTx, target *semver.Version) {
	tx.LockOutsideApply()
	defer tx.Unlock()
	// Storage version is only supported since v3.6
	if target.LessThan(version.V3_6) {
		schema.UnsafeClearStorageVersion(tx)
		lg.Warn("forcefully cleared storage version")
	} else {
		schema.UnsafeSetStorageVersion(tx, target)
		lg.Warn("forcefully set storage version", zap.String("storage-version", storageVersionToString(target)))
	}
}

func storageVersionToString(ver *semver.Version) string {
	return fmt.Sprintf("%d.%d", ver.Major, ver.Minor)
}

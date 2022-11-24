// Copyright 2022 The etcd Authors
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
	"encoding/binary"
	"fmt"
	"os"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/etcd/server/v3/storage/schema"
	"go.etcd.io/etcd/server/v3/storage/wal"

	"github.com/spf13/cobra"
	bolt "go.etcd.io/bbolt"
)

var (
	checkDataDir           string
	ConsistentIndexKeyName = []byte("consistent_index")
)

// NewCheckCommand returns the cobra command for "check".
func NewCheckCommand() *cobra.Command {
	cc := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "commands for checking properties of the etcd cluster",
	}

	cc.AddCommand(NewCheckBrokenCommand())

	return cc
}

// NewCheckBrokenCommand returns the cobra command for "check broken".
func NewCheckBrokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "broken [options]",
		Short: "Check whether the data file is broken",
		Run:   newCheckBrokenCommand,
	}

	cmd.Flags().StringVar(&checkDataDir, "data-dir", "", "Path to the output data directory")
	cmd.MarkFlagRequired("data-dir")

	return cmd
}

func newCheckBrokenCommand(cmd *cobra.Command, args []string) {
	walSnaps, err := wal.ValidSnapshotEntries(GetLogger(), datadir.ToWalDir(checkDataDir))
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	ss := snap.New(nil, datadir.ToSnapDir(checkDataDir))
	_, err = ss.LoadNewestAvailable(walSnaps)
	if err != nil && err != snap.ErrNoSnapshot {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	dbPath := datadir.ToBackendFileName(checkDataDir)
	if _, err = os.Stat(dbPath); err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	if err = tryOpenDb(dbPath); err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}

	if v := readConsistentIndex(dbPath); v < 0 {
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("unexpected consistent index %s", v))
	}
}

func tryOpenDb(path string) error {
	var err error
	var db *bolt.DB

	defer func() {
		if err == nil && db != nil {
			db.Close()
		}
	}()
	opts := bolt.DefaultOptions
	opts.ReadOnly = true
	db, err = bolt.Open(path, 0600, opts)
	return err
}

func readConsistentIndex(path string) uint64 {
	beConfig := backend.DefaultBackendConfig(GetLogger())
	beConfig.Path = path
	be := backend.New(beConfig)
	defer be.Close()

	tx := be.BatchTx()
	tx.Lock()
	defer tx.Unlock()
	_, vs := tx.UnsafeRange(schema.Meta, ConsistentIndexKeyName, nil, 0)
	if len(vs) == 0 {
		return 0
	}
	v := binary.BigEndian.Uint64(vs[0])
	return v
}

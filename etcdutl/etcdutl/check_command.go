// Copyright 2024 The etcd Authors
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
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/etcdserver"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/wal"
)

// NewCheckCommand returns the cobra command for "check".
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "commands for checking properties",
	}
	cmd.AddCommand(NewCheckV2StoreCommand())
	return cmd
}

var (
	argCheckV2StoreDataDir string
)

// NewCheckV2StoreCommand returns the cobra command for "check v2store".
func NewCheckV2StoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "v2store",
		Short: "Check custom content in v2store",
		Run:   checkV2StoreRunFunc,
	}
	cmd.Flags().StringVar(&argCheckV2StoreDataDir, "data-dir", "", "Required. A data directory not in use by etcd.")
	cmd.MarkFlagRequired("data-dir")
	return cmd
}

func checkV2StoreRunFunc(_ *cobra.Command, _ []string) {
	err := checkV2StoreDataDir(argCheckV2StoreDataDir)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	fmt.Println("No custom content found in v2store.")
}

func checkV2StoreDataDir(dataDir string) error {
	var (
		lg = GetLogger()

		walDir  = filepath.Join(dataDir, "member", "wal")
		snapDir = filepath.Join(dataDir, "member", "snap")
	)

	walSnaps, err := wal.ValidSnapshotEntries(lg, walDir)
	if err != nil {
		if errors.Is(err, wal.ErrFileNotFound) {
			return nil
		}
		return err
	}

	ss := snap.New(lg, snapDir)
	snapshot, err := ss.LoadNewestAvailable(walSnaps)
	if err != nil {
		if errors.Is(err, snap.ErrNoSnapshot) {
			return nil
		}
		return err
	}
	if snapshot == nil {
		return nil
	}

	st := v2store.New(etcdserver.StoreClusterPrefix, etcdserver.StoreKeysPrefix)

	if err := st.Recovery(snapshot.Data); err != nil {
		return fmt.Errorf("failed to recover v2store from snapshot: %w", err)
	}
	return assertNoV2StoreContent(st)
}

func assertNoV2StoreContent(st v2store.Store) error {
	metaOnly, err := membership.IsMetaStoreOnly(st)
	if err != nil {
		return err
	}
	if metaOnly {
		return nil
	}
	return fmt.Errorf("detected custom content in v2store")
}

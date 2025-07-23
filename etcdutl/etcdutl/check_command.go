// Copyright 2025 The etcd Authors
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

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/client/pkg/v3/fileutil"
	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/etcdserver/api/membership"
	"go.etcd.io/etcd/server/v3/etcdserver/api/snap"
	"go.etcd.io/etcd/server/v3/etcdserver/api/v2store"
	"go.etcd.io/etcd/server/v3/storage/datadir"
	"go.etcd.io/raft/v3/raftpb"
)

var checkDataDir string

// NewCheckCommand returns the cobra command for "check".
func NewCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <subcommand>",
		Short: "Commands for checking properties of the etcd data",
	}

	cmd.AddCommand(NewCheckV2StoreCommand())
	cmd.AddCommand(NewCleanupV2StoreCommand())

	return cmd
}

// NewCheckV2StoreCommand returns the cobra command for "check v2store".
func NewCheckV2StoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "v2store",
		Short: "Check whether v2store contains custom content",
		Long: `Check whether the v2store contains any custom content beyond meta-information.
Meta-information includes cluster membership and version data that can be 
recovered from the v3 backend. Custom content includes any user-created keys
or auth data that could block upgrade to etcd v3.6.

Exit codes:
  0: v2store contains only meta content (safe to upgrade)
  1: v2store contains custom content (cleanup required)
  2: error occurred during check`,
		Run: checkV2StoreCommandFunc,
	}

	cmd.Flags().StringVar(&checkDataDir, "data-dir", "", "Path to the etcd data directory")
	cmd.MarkFlagRequired("data-dir")
	cmd.MarkFlagDirname("data-dir")

	return cmd
}

// NewCleanupV2StoreCommand returns the cobra command for "cleanup v2store".
func NewCleanupV2StoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove custom content from v2store",
		Long: `Remove any custom content from the v2store, keeping only meta-information.
This command will delete all user-created keys and auth data from the v2store,
making it safe to upgrade to etcd v3.6. Only meta-information (cluster 
membership and version data) will be preserved.

WARNING: This operation is irreversible. Any custom data in v2store will be 
permanently deleted. Make sure you have migrated all important data to v3 
before running this command.`,
		Run: cleanupV2StoreCommandFunc,
	}

	cmd.Flags().StringVar(&checkDataDir, "data-dir", "", "Path to the etcd data directory")
	cmd.MarkFlagRequired("data-dir")
	cmd.MarkFlagDirname("data-dir")

	return cmd
}

func checkV2StoreCommandFunc(cmd *cobra.Command, args []string) {
	lg := GetLogger()

	if !fileutil.Exist(checkDataDir) {
		lg.Error("data directory does not exist", zap.String("data-dir", checkDataDir))
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("data directory does not exist: %s", checkDataDir))
	}

	snapDir := datadir.ToSnapDir(checkDataDir)
	if !fileutil.Exist(snapDir) {
		lg.Info("no snapshot directory found, v2store is empty", zap.String("snap-dir", snapDir))
		fmt.Println("v2store is empty (no snapshot directory found)")
		return
	}

	store, err := loadV2Store(lg, checkDataDir)
	if err != nil {
		lg.Error("failed to load v2store", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to load v2store: %w", err))
	}

	metaOnly, err := membership.IsMetaStoreOnly(store)
	if err != nil {
		lg.Error("failed to check v2store content", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to check v2store content: %w", err))
	}

	if metaOnly {
		fmt.Println("v2store contains only meta content (safe to upgrade)")
		lg.Info("v2store check passed: contains only meta content")
	} else {
		fmt.Println("v2store contains custom content (cleanup required before upgrade)")
		lg.Warn("v2store check failed: contains custom content")
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("v2store contains custom content"))
	}
}

func cleanupV2StoreCommandFunc(cmd *cobra.Command, args []string) {
	lg := GetLogger()

	if !fileutil.Exist(checkDataDir) {
		lg.Error("data directory does not exist", zap.String("data-dir", checkDataDir))
		cobrautl.ExitWithError(cobrautl.ExitBadArgs, fmt.Errorf("data directory does not exist: %s", checkDataDir))
	}

	snapDir := datadir.ToSnapDir(checkDataDir)
	if !fileutil.Exist(snapDir) {
		lg.Info("no snapshot directory found, v2store is already clean", zap.String("snap-dir", snapDir))
		fmt.Println("v2store is already clean (no snapshot directory found)")
		return
	}

	store, err := loadV2Store(lg, checkDataDir)
	if err != nil {
		lg.Error("failed to load v2store", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to load v2store: %w", err))
	}

	// Check current state
	metaOnly, err := membership.IsMetaStoreOnly(store)
	if err != nil {
		lg.Error("failed to check v2store content", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to check v2store content: %w", err))
	}

	if metaOnly {
		fmt.Println("v2store already contains only meta content (no cleanup needed)")
		lg.Info("v2store cleanup skipped: already contains only meta content")
		return
	}

	// Perform cleanup
	if err := cleanupV2StoreCustomContent(lg, store); err != nil {
		lg.Error("failed to cleanup v2store", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to cleanup v2store: %w", err))
	}

	// Save the cleaned store back to snapshot
	if err := saveV2Store(lg, checkDataDir, store); err != nil {
		lg.Error("failed to save cleaned v2store", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to save cleaned v2store: %w", err))
	}

	// Verify cleanup was successful
	verifyMetaOnly, err := membership.IsMetaStoreOnly(store)
	if err != nil {
		lg.Error("failed to verify cleanup", zap.Error(err))
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("failed to verify cleanup: %w", err))
	}

	if !verifyMetaOnly {
		lg.Error("cleanup verification failed: v2store still contains custom content")
		cobrautl.ExitWithError(cobrautl.ExitError, fmt.Errorf("cleanup verification failed"))
	}

	fmt.Println("v2store cleanup completed successfully")
	lg.Info("v2store cleanup completed successfully")
}

// loadV2Store loads the v2store from the latest snapshot in the data directory
func loadV2Store(lg *zap.Logger, dataDir string) (v2store.Store, error) {
	snapshot, err := getLatestV2Snapshot(lg, dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	if snapshot == nil {
		// No snapshot exists, create empty store
		lg.Info("no snapshot found, creating empty v2store")
		return v2store.New(), nil
	}

	// Create store and recover from snapshot
	store := v2store.New()
	if err := store.Recovery(snapshot.Data); err != nil {
		return nil, fmt.Errorf("failed to recover v2store from snapshot: %w", err)
	}

	lg.Info("successfully loaded v2store from snapshot", 
		zap.Uint64("snapshot-index", snapshot.Metadata.Index))
	return store, nil
}

// cleanupV2StoreCustomContent removes all custom content from v2store, keeping only meta information
func cleanupV2StoreCustomContent(lg *zap.Logger, store v2store.Store) error {
	event, err := store.Get("/", true, false)
	if err != nil {
		return fmt.Errorf("failed to get root node: %w", err)
	}

	storePrefix := "/0"           // membership and version info (keep)
	storePermsPrefix := "/2"      // auth data (remove custom content)
	
	var deletions []string

	for _, n := range event.Node.Nodes {
		switch n.Key {
		case storePrefix:
			// Keep membership and version information - this is meta content
			lg.Debug("keeping meta content", zap.String("key", n.Key))
			continue
			
		case storePermsPrefix:
			// Remove any custom auth content, but preserve empty structure if needed
			if n.Nodes.Len() > 0 {
				for _, child := range n.Nodes {
					if child.Nodes.Len() > 0 {
						// This child has custom content, mark for deletion
						deletions = append(deletions, child.Key)
						lg.Debug("marking auth content for deletion", zap.String("key", child.Key))
					}
				}
			}
			continue
			
		default:
			// Any other top-level content is custom and should be removed
			deletions = append(deletions, n.Key)
			lg.Debug("marking custom content for deletion", zap.String("key", n.Key))
		}
	}

	// Perform deletions
	for _, key := range deletions {
		lg.Info("deleting custom content", zap.String("key", key))
		if _, err := store.Delete(key, true, true); err != nil {
			lg.Warn("failed to delete key (may not exist)", zap.String("key", key), zap.Error(err))
			// Continue with other deletions even if one fails
		}
	}

	lg.Info("completed v2store cleanup", zap.Int("deletions", len(deletions)))
	return nil
}

// saveV2Store saves the v2store back to a snapshot
func saveV2Store(lg *zap.Logger, dataDir string, store v2store.Store) error {
	snapDir := datadir.ToSnapDir(dataDir)
	
	// Ensure snapshot directory exists
	if !fileutil.Exist(snapDir) {
		if err := fileutil.CreateDirAll(lg, snapDir); err != nil {
			return fmt.Errorf("failed to create snapshot directory: %w", err)
		}
	}

	// Save store data
	data, err := store.Save()
	if err != nil {
		return fmt.Errorf("failed to save v2store data: %w", err)
	}

	// Create snapshotter and save snapshot
	ss := snap.New(lg, snapDir)
	
	// Use a dummy snapshot with index 1 to indicate this is a cleaned snapshot
	snapshot := raftpb.Snapshot{
		Data: data,
		Metadata: raftpb.SnapshotMetadata{
			Index: 1,
			Term:  1,
		},
	}

	if err := ss.SaveSnap(snapshot); err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	lg.Info("successfully saved cleaned v2store snapshot", zap.String("snap-dir", snapDir))
	return nil
} 
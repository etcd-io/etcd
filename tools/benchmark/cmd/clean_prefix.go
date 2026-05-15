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

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	cleanPrefixBatchSize int
	cleanPrefixDryRun    bool
)

var cleanPrefixCmd = &cobra.Command{
	Use:   "clean-prefix <prefix>",
	Short: "Delete keys under a prefix in small batches",
	Args:  cobra.MaximumNArgs(1),
	RunE:  cleanPrefixFunc,
}

func init() {
	RootCmd.AddCommand(cleanPrefixCmd)

	cleanPrefixCmd.Flags().IntVar(&cleanPrefixBatchSize, "batch-size", 100, "Number of keys to delete per transaction; keep at or below etcd --max-txn-ops")
	cleanPrefixCmd.Flags().BoolVar(&cleanPrefixDryRun, "dry-run", false, "Count matching keys without deleting them")
}

func cleanPrefixFunc(cmd *cobra.Command, args []string) error {
	prefix := benchmarkKeyPrefix
	if len(args) > 0 {
		prefix = args[0]
	}
	if prefix == "" {
		return fmt.Errorf("expected prefix argument or --key-prefix")
	}
	if cleanPrefixBatchSize <= 0 || cleanPrefixBatchSize > 128 {
		return fmt.Errorf("expected --batch-size between 1 and 128, got %d", cleanPrefixBatchSize)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	client := mustCreateConn()
	defer client.Close()

	start := time.Now()
	var total int
	for batch := 1; ; batch++ {
		resp, err := client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(int64(cleanPrefixBatchSize)))
		if err != nil {
			return err
		}
		if len(resp.Kvs) == 0 {
			break
		}

		if cleanPrefixDryRun {
			total += len(resp.Kvs)
			fmt.Printf("matched %d keys so far under prefix %q\n", total, prefix)
			if !resp.More {
				break
			}
			continue
		}

		ops := make([]clientv3.Op, 0, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			ops = append(ops, clientv3.OpDelete(string(kv.Key)))
		}
		if _, err = client.Txn(ctx).Then(ops...).Commit(); err != nil {
			return err
		}

		total += len(resp.Kvs)
		if batch == 1 || batch%100 == 0 {
			fmt.Printf("deleted %d keys under prefix %q\n", total, prefix)
		}
	}

	action := "Deleted"
	if cleanPrefixDryRun {
		action = "Matched"
	}
	fmt.Printf("%s %d keys under prefix %q in %s\n", action, total, prefix, time.Since(start).Round(time.Second))
	return nil
}

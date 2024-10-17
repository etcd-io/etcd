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
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

var hashKVRevision int64

// NewHashKVCommand returns the cobra command for "hashkv".
func NewHashKVCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hashkv <filename>",
		Short: "Prints the KV history hash of a given file",
		Args:  cobra.ExactArgs(1),
		Run:   hashKVCommandFunc,
	}
	cmd.Flags().Int64Var(&hashKVRevision, "rev", 0, "maximum revision to hash (default: latest revision)")
	return cmd
}

func hashKVCommandFunc(cmd *cobra.Command, args []string) {
	printer := initPrinterFromCmd(cmd)

	ds, err := calculateHashKV(args[0], hashKVRevision)
	if err != nil {
		cobrautl.ExitWithError(cobrautl.ExitError, err)
	}
	printer.DBHashKV(ds)
}

type HashKV struct {
	Hash            uint32 `json:"hash"`
	HashRevision    int64  `json:"hashRevision"`
	CompactRevision int64  `json:"compactRevision"`
}

func calculateHashKV(dbPath string, rev int64) (HashKV, error) {
	cfg := backend.DefaultBackendConfig(zap.NewNop())
	cfg.Path = dbPath
	b := backend.New(cfg)
	st := mvcc.NewStore(zap.NewNop(), b, nil, mvcc.StoreConfig{})
	hst := mvcc.NewHashStorage(zap.NewNop(), st)

	h, _, err := hst.HashByRev(rev)
	if err != nil {
		return HashKV{}, err
	}
	return HashKV{
		Hash:            h.Hash,
		HashRevision:    h.Revision,
		CompactRevision: h.CompactRevision,
	}, nil
}

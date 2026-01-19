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
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"go.etcd.io/etcd/pkg/v3/cobrautl"
	"go.etcd.io/etcd/server/v3/storage/backend"
	"go.etcd.io/etcd/server/v3/storage/mvcc"
)

var (
	hashKVRevision   int64
	hashKVCompactRev int64
	hashKVDetailed   bool
	hashKVOutputFile string
)

// NewHashKVCommand returns the cobra command for "hashkv".
func NewHashKVCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hashkv <filename>",
		Short: "Prints the KV history hash of a given file",
		Args:  cobra.ExactArgs(1),
		Run:   hashKVCommandFunc,
	}
	cmd.Flags().Int64Var(&hashKVRevision, "rev", 0, "maximum revision to hash (default: latest revision)")
	cmd.Flags().Int64Var(&hashKVCompactRev, "compact-rev", 0, "compact revision - revisions less than or equal to this value will be ignored (default: 0, use storage compact revision)")
	cmd.Flags().BoolVar(&hashKVDetailed, "detailed", false, "enable detailed mode to return individual key+revision hashes")
	cmd.Flags().StringVar(&hashKVOutputFile, "output", "", "output file path for detailed JSON results (default: print to stdout)")
	return cmd
}

func hashKVCommandFunc(cmd *cobra.Command, args []string) {
	printer := initPrinterFromCmd(cmd)

	ds, err := calculateHashKV(args[0], hashKVRevision, hashKVCompactRev, hashKVDetailed, hashKVOutputFile)
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

func calculateHashKV(dbPath string, rev int64, compactRev int64, detailed bool, outputFile string) (HashKV, error) {
	b := backend.NewDefaultBackend(zap.NewNop(), dbPath, backend.WithTimeout(FlockTimeout))
	// Since `etcdutl hashkv` only hashes the keyspace and ignores leases, we use a simple lessor to simplify the implementation.
	st := mvcc.NewStore(zap.NewNop(), b, &SimpleLessor{}, mvcc.StoreConfig{})
	hst := mvcc.NewHashStorage(zap.NewNop(), st)

	defer func() {
		st.Close()
		b.Close()
	}()

	var result HashKV

	if detailed {
		detailedResult, _, err := hst.HashByRevDetailed(rev, compactRev)
		if err != nil {
			return HashKV{}, err
		}

		result = HashKV{
			Hash:            detailedResult.TotalHash.Hash,
			HashRevision:    detailedResult.TotalHash.Revision,
			CompactRevision: detailedResult.TotalHash.CompactRevision,
		}

		if err := outputDetailedResult(detailedResult.KeyHashes, outputFile); err != nil {
			return HashKV{}, err
		}
	} else {
		var h mvcc.KeyValueHash
		var err error

		h, _, err = hst.HashByRevWithCompactRev(rev, compactRev)

		if err != nil {
			return HashKV{}, err
		}

		result = HashKV{
			Hash:            h.Hash,
			HashRevision:    h.Revision,
			CompactRevision: h.CompactRevision,
		}
	}

	return result, nil
}

func outputDetailedResult(detail []mvcc.KeyRevisionHash, outputFile string) error {
	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return err
	}

	if outputFile != "" {
		return os.WriteFile(outputFile, data, 0644)
	}

	fmt.Println(string(data))
	return nil
}

// Copyright 2016 Nippon Telegraph and Telephone Corporation.
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
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/lease"
)

// storageCompactionCmd represents a storage compaction performance benchmarking tool
var storageCompactionCmd = &cobra.Command{
	Use:   "compaction",
	Short: "Benchmark compaction performance of storage",

	Run: storageCompactionFunc,
}

var (
	totalPut int
	nrKeys   int
)

func init() {
	storageCmd.AddCommand(storageCompactionCmd)

	// basically variables for parameters are shared with storage-put.go
	storageCompactionCmd.Flags().IntVar(&totalPut, "total", 100, "a total number of put operations")
	storageCompactionCmd.Flags().IntVar(&storageKeySize, "key-size", 64, "a size of key (Byte)")
	storageCompactionCmd.Flags().IntVar(&valueSize, "value-size", 64, "a size of value (Byte)")
	storageCompactionCmd.Flags().IntVar(&nrKeys, "keys", 1, "a number of keys for put operations")

	// TODO: after the PR https://github.com/spf13/cobra/pull/220 is merged, the below pprof related flags should be moved to RootCmd
	storageCompactionCmd.Flags().StringVar(&cpuProfPath, "cpuprofile", "", "the path of file for storing cpu profile result")
	storageCompactionCmd.Flags().StringVar(&memProfPath, "memprofile", "", "the path of file for storing heap profile result")

}

func storageCompactionFunc(cmd *cobra.Command, args []string) {
	if cpuProfPath != "" {
		f, err := os.Create(cpuProfPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create a file for storing cpu profile result: ", err)
			os.Exit(1)
		}

		err = pprof.StartCPUProfile(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to start cpu profile: ", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	if memProfPath != "" {
		f, err := os.Create(memProfPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to create a file for storing heap profile result: ", err)
			os.Exit(1)
		}

		defer func() {
			err := pprof.WriteHeapProfile(f)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Failed to write heap profile result: ", err)
				// can do nothing for handling the error
			}
		}()
	}

	keys := createBytesSlice(storageKeySize, nrKeys)
	vals := createBytesSlice(valueSize, nrKeys)

	var rev int64

	for i := 0; i < totalPut; i++ {
		rev = s.Put(keys[i%nrKeys], vals[i%nrKeys], lease.NoLease)
	}

	begin := time.Now()
	s.SyncCompact(rev)
	end := time.Now()

	fmt.Printf("required time for compaction: %v\n", end.Sub(begin))
}

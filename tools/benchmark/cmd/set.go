// Copyright 2015 CoreOS, Inc.
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
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/cheggaaa/pb"
	"github.com/coreos/etcd/Godeps/_workspace/src/github.com/spf13/cobra"
	"github.com/coreos/etcd/Godeps/_workspace/src/golang.org/x/net/context"
	"github.com/coreos/etcd/client"
)

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "Benchmark set (v2 only, will be deprecated)",

	Run: setFunc,
}

var (
	setEndpoints []string

	setKeySize int
	setValSize int

	setTotal int

	setKeySpaceSize int
	setSeqKeys      bool
)

func init() {
	RootCmd.AddCommand(setCmd)
	setCmd.Flags().StringSliceVar(&setEndpoints, "endpoints", []string{"http://127.0.0.1:12379", "http://127.0.0.1:22379", "http://127.0.0.1:32379"}, "Specify the V2 client endpoints.")
	setCmd.Flags().IntVar(&setKeySize, "key-size", 8, "Key size of set request")
	setCmd.Flags().IntVar(&setValSize, "val-size", 8, "Value size of set request")
	setCmd.Flags().IntVar(&setTotal, "total", 10000, "Total number of set requests")
	setCmd.Flags().IntVar(&setKeySpaceSize, "key-space-size", 1, "Maximum possible keys")
	setCmd.Flags().BoolVar(&setSeqKeys, "sequential-keys", false, "Use sequential keys")
}

func setFunc(cmd *cobra.Command, args []string) {
	if setKeySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", setKeySpaceSize)
		os.Exit(1)
	}

	results = make(chan result)
	bar = pb.New(setTotal)

	k, v := make([]byte, setKeySize), string(mustRandBytes(setValSize))

	bar.Format("Bom !")
	bar.Start()

	cfg := client.Config{
		Endpoints:               setEndpoints,
		Transport:               client.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
		// SelectionMode: client.EndpointSelectionPrioritizeLeader,
	}
	ct, err := client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
	kapi := client.NewKeysAPI(ct)

	wg.Add(setTotal)

	for i := 0; i < setTotal; i++ {
		go func(i int) {
			defer wg.Done()

			if seqKeys {
				binary.PutVarint(k, int64(i%setKeySpaceSize))
			} else {
				binary.PutVarint(k, int64(rand.Intn(setKeySpaceSize)))
			}

			st := time.Now()
			_, err := kapi.Set(context.Background(), string(k), v, nil)

			var errStr string
			if err != nil {
				errStr = err.Error()
			}
			results <- result{errStr: errStr, duration: time.Since(st)}
			bar.Increment()
		}(i)
	}

	pdoneC := printReport(results)

	wg.Wait()

	bar.Finish()

	close(results)
	<-pdoneC
}

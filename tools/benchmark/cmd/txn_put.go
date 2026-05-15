// Copyright 2017 The etcd Authors
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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	v3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/report"
)

// txnPutCmd represents the txnPut command
var txnPutCmd = &cobra.Command{
	Use:   "txn-put",
	Short: "Benchmark txn-put",

	Run: txnPutFunc,
}

var (
	txnPutTotal     int
	txnPutRate      int
	txnPutOpsPerTxn int
)

type txnPutRevisionState struct {
	mu        sync.Mutex
	revisions []int64
}

func init() {
	RootCmd.AddCommand(txnPutCmd)
	txnPutCmd.Flags().IntVar(&keySize, "key-size", 8, "Key size of txn put")
	txnPutCmd.Flags().IntVar(&valSize, "val-size", 8, "Value size of txn put")
	txnPutCmd.Flags().IntVar(&txnPutOpsPerTxn, "txn-ops", 1, "Number of puts per txn")
	txnPutCmd.Flags().IntVar(&txnPutRate, "rate", 0, "Maximum txns per second (0 is no limit)")

	txnPutCmd.Flags().IntVar(&txnPutTotal, "total", 10000, "Total number of txn requests")
	txnPutCmd.Flags().IntVar(&keySpaceSize, "key-space-size", 1, "Maximum possible keys")
}

func txnPutFunc(cmd *cobra.Command, _ []string) {
	if keySpaceSize <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --key-space-size, got (%v)", keySpaceSize)
		os.Exit(1)
	}

	if txnPutOpsPerTxn <= 0 {
		fmt.Fprintf(os.Stderr, "expected positive --txn-ops, got (%v)\n", txnPutOpsPerTxn)
		os.Exit(1)
	}

	if txnPutOpsPerTxn > keySpaceSize {
		fmt.Fprintf(os.Stderr, "expected --txn-ops no larger than --key-space-size, "+
			"got txn-ops(%v) key-space-size(%v)\n", txnPutOpsPerTxn, keySpaceSize)
		os.Exit(1)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	requests := make(chan []int, totalClients)
	if txnPutRate == 0 {
		txnPutRate = math.MaxInt32
	}
	limit := rate.NewLimiter(rate.Limit(txnPutRate), 1)
	clients := mustCreateClients(totalClients, totalConns)
	keys := txnPutKeys(keySize, keySpaceSize)
	v := string(mustRandBytes(valSize))

	revisions, err := seedTxnPutKeys(ctx, clients[0], keys, v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to seed txn-put keyspace: %v\n", err)
		if errors.Is(err, context.Canceled) {
			return
		}
		os.Exit(1)
	}
	revisionState := newTxnPutRevisionState(revisions)

	bar = pb.New(txnPutTotal)
	bar.Start()

	r := newReport(cmd.Name())
	for i := range clients {
		wg.Add(1)
		go func(c *v3.Client) {
			defer wg.Done()
			for keyIndexes := range requests {
				if err := limit.Wait(ctx); err != nil {
					if !errors.Is(err, context.Canceled) {
						r.Results() <- report.Result{Err: err, Start: time.Now(), End: time.Now()}
					}
					return
				}
				st := time.Now()
				err := runTxnPutCAS(ctx, c, keys, v, keyIndexes, revisionState)
				r.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				bar.Increment()
				if errors.Is(err, context.Canceled) {
					return
				}
			}
		}(clients[i])
	}

	go func() {
		defer close(requests)
		for i := 0; i < txnPutTotal; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			keyIndexes := make([]int, txnPutOpsPerTxn)
			for j := 0; j < txnPutOpsPerTxn; j++ {
				keyIndexes[j] = ((i * txnPutOpsPerTxn) + j) % keySpaceSize
			}
			select {
			case requests <- keyIndexes:
			case <-ctx.Done():
				return
			}
		}
	}()

	rc := r.Run()
	wg.Wait()
	close(r.Results())
	bar.Finish()
	fmt.Println(<-rc)
}

func txnPutKeys(keySize, keySpaceSize int) []string {
	keys := make([]string, keySpaceSize)
	for i := range keys {
		key := make([]byte, keySize)
		encoded := make([]byte, binary.MaxVarintLen64)
		n := binary.PutUvarint(encoded, uint64(i))
		copy(key, encoded[:n])
		keys[i] = benchmarkKey(string(key))
	}
	return keys
}

func seedTxnPutKeys(ctx context.Context, c *v3.Client, keys []string, value string) ([]int64, error) {
	revisions := make([]int64, len(keys))
	for i, key := range keys {
		var revision int64
		if err := casPutSingle(ctx, c, key, value, &revision); err != nil {
			return nil, fmt.Errorf("seed key %q: %w", key, err)
		}
		revisions[i] = revision
	}
	return revisions, nil
}

func newTxnPutRevisionState(revisions []int64) *txnPutRevisionState {
	state := &txnPutRevisionState{revisions: make([]int64, len(revisions))}
	copy(state.revisions, revisions)
	return state
}

func (s *txnPutRevisionState) snapshot(keyIndexes []int) []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	revisions := make([]int64, len(keyIndexes))
	for i, keyIndex := range keyIndexes {
		revisions[i] = s.revisions[keyIndex]
	}
	return revisions
}

func (s *txnPutRevisionState) applyRevision(keyIndexes []int, revision int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, keyIndex := range keyIndexes {
		s.revisions[keyIndex] = maxInt64(s.revisions[keyIndex], revision)
	}
}

func (s *txnPutRevisionState) applyObservedRevisions(keyIndexes []int, revisions []int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, keyIndex := range keyIndexes {
		s.revisions[keyIndex] = maxInt64(s.revisions[keyIndex], revisions[i])
	}
}

func runTxnPutCAS(ctx context.Context, c *v3.Client, keys []string, value string, keyIndexes []int, state *txnPutRevisionState) error {
	for {
		expectedRevisions := state.snapshot(keyIndexes)
		cmps := make([]v3.Cmp, len(keyIndexes))
		thenOps := make([]v3.Op, len(keyIndexes))
		elseOps := make([]v3.Op, len(keyIndexes))
		for i, keyIndex := range keyIndexes {
			key := keys[keyIndex]
			cmps[i] = v3.Compare(v3.ModRevision(key), "=", expectedRevisions[i])
			thenOps[i] = v3.OpPut(key, value)
			elseOps[i] = v3.OpGet(key)
		}

		txnResp, err := c.Txn(ctx).If(cmps...).Then(thenOps...).Else(elseOps...).Commit()
		if err != nil {
			return err
		}
		if txnResp.Succeeded {
			state.applyRevision(keyIndexes, txnResp.Header.Revision)
			return nil
		}

		revisions, err := txnPutObservedModRevisions(txnResp, len(keyIndexes))
		if err != nil {
			return err
		}
		state.applyObservedRevisions(keyIndexes, revisions)
	}
}

func txnPutObservedModRevisions(txnResp *v3.TxnResponse, expectedResponses int) ([]int64, error) {
	if len(txnResp.Responses) != expectedResponses {
		return nil, fmt.Errorf("expected %d refresh responses, got %d", expectedResponses, len(txnResp.Responses))
	}

	revisions := make([]int64, len(txnResp.Responses))
	for i, response := range txnResp.Responses {
		rangeResp := response.GetResponseRange()
		if rangeResp == nil {
			return nil, fmt.Errorf("expected range response at index %d", i)
		}
		if len(rangeResp.Kvs) == 0 {
			continue
		}
		revisions[i] = rangeResp.Kvs[0].ModRevision
	}
	return revisions, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

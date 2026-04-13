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

//nolint:unparam
package validate

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/anishathalye/porcupine"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/tests/v3/robustness/model"
)

func TestValidateSerializableOperations(t *testing.T) {
	tcs := []struct {
		name              string
		persistedRequests []model.EtcdRequest
		operations        []porcupine.Operation
		expectError       string
	}{
		{
			name: "Success",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 1, 0),
					Output: rangeResponse(0),
				},
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(1, keyValueRevision("a", "1", 2)),
				},
				{
					Input: rangeRequest("a", "z", 3, 0),
					Output: rangeResponse(2,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 0),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 3),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 4),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("a", "z", 4, 2),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input: rangeRequest("b\x00", "z", 4, 2),
					Output: rangeResponse(1,
						keyValueRevision("c", "3", 4),
					),
				},
				{
					Input: rangeRequest("b", "", 4, 0),
					Output: rangeResponse(1,
						keyValueRevision("b", "2", 3),
					),
				},
				{
					Input:  rangeRequest("b", "", 2, 0),
					Output: rangeResponse(0),
				},
			},
		},
		{
			name: "Invalid order",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 4, 0),
					Output: rangeResponse(3,
						keyValueRevision("c", "3", 4),
						keyValueRevision("b", "2", 3),
						keyValueRevision("a", "1", 2),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid count",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 1, 0),
					Output: rangeResponse(1),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid keys",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(3,
						keyValueRevision("b", "2", 3),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Invalid revision",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input: rangeRequest("a", "z", 2, 0),
					Output: rangeResponse(3,
						keyValueRevision("a", "1", 2),
						keyValueRevision("b", "2", 3),
					),
				},
			},
			expectError: errRespNotMatched.Error(),
		},
		{
			name: "Error",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: errorResponse(model.ErrEtcdFutureRev),
				},
				{
					Input:  rangeRequest("a", "z", 2, 0),
					Output: errorResponse(fmt.Errorf("timeout")),
				},
			},
		},
		{
			name: "Future rev returned",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: errorResponse(model.ErrEtcdFutureRev),
				},
			},
		},
		{
			name: "Future rev success",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: rangeResponse(0),
				},
			},
			expectError: errFutureRevRespRequested.Error(),
		},
		{
			name: "Future rev failure",
			persistedRequests: []model.EtcdRequest{
				putRequest("a", "1"),
				putRequest("b", "2"),
				putRequest("c", "3"),
			},
			operations: []porcupine.Operation{
				{
					Input:  rangeRequest("a", "z", 6, 0),
					Output: errorResponse(fmt.Errorf("timeout")),
				},
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			replay := model.NewReplay(tc.persistedRequests)
			result := validateSerializableOperations(zaptest.NewLogger(t), tc.operations, replay)
			if result.Message != tc.expectError {
				t.Errorf("validateSerializableOperations(...), got: %q, want: %q", result.Message, tc.expectError)
			}
		})
	}
}

func rangeRequest(start, end string, rev, limit int64) model.EtcdRequest {
	return model.EtcdRequest{
		Type: model.Range,
		Range: &model.RangeRequest{
			RangeOptions: model.RangeOptions{
				Start: start,
				End:   end,
				Limit: limit,
			},
			Revision: rev,
		},
	}
}

func rangeResponse(count int64, kvs ...model.KeyValue) model.MaybeEtcdResponse {
	if kvs == nil {
		kvs = []model.KeyValue{}
	}
	return model.MaybeEtcdResponse{
		EtcdResponse: model.EtcdResponse{
			Range: &model.RangeResponse{
				KVs:   kvs,
				Count: count,
			},
		},
	}
}

func errorResponse(err error) model.MaybeEtcdResponse {
	return model.MaybeEtcdResponse{
		Error: err.Error(),
	}
}

func keyValueRevision(key, value string, rev int64) model.KeyValue {
	return model.KeyValue{
		Key: key,
		ValueRevision: model.ValueRevision{
			Value:       model.ToValueOrHash(value),
			ModRevision: rev,
			Version:     1,
		},
	}
}

func TestValidateLinearizableOperationsTimeoutIsRespected(t *testing.T) {
	timeout := time.Second
	history := concurrentFailedPutsWithRead(t, 22)
	keys := model.ModelKeys(history)

	start := time.Now()
	result := validateLinearizableOperationsAndVisualize(zap.NewNop(), keys, history, timeout)
	elapsed := time.Since(start)

	if !result.Timeout {
		t.Fatalf("validateLinearizableOperationsAndVisualize(...) timed out = false, message = %q", result.Message)
	}
	if result.Message != "timed out" {
		t.Fatalf("validateLinearizableOperationsAndVisualize(...) message = %q, want %q", result.Message, "timed out")
	}
	if elapsed > timeout+250*time.Millisecond {
		t.Fatalf("validateLinearizableOperationsAndVisualize(...) does not respect timeout: %v, timeout was %v", elapsed, timeout)
	}
}

func TestValidateLinearizableOperationsIllegalGeneratesVisualization(t *testing.T) {
	history := impossibleReadAfterSuccessfulPut()
	keys := model.ModelKeys(history)

	result := validateLinearizableOperationsAndVisualize(zap.NewNop(), keys, history, 100*time.Millisecond)
	if result.Timeout {
		t.Fatal("validateLinearizableOperationsAndVisualize(...) timed out unexpectedly")
	}
	if result.Message != "illegal" {
		t.Fatalf("validateLinearizableOperationsAndVisualize(...) message = %q, want %q", result.Message, "illegal")
	}
	if result.Status != Failure {
		t.Fatalf("validateLinearizableOperationsAndVisualize(...) status = %q, want %q", result.Status, Failure)
	}
	if len(result.Info.PartialLinearizations()) == 0 {
		t.Fatal("validateLinearizableOperationsAndVisualize(...) produced no linearization info for illegal history")
	}
	if err := result.Visualize(zap.NewNop(), filepath.Join(t.TempDir(), "history.html")); err != nil {
		t.Fatalf("result.Visualize(...): %v", err)
	}
}

func BenchmarkValidateLinearizableOperations(b *testing.B) {
	lg := zap.NewNop()
	b.Run("SequentialSuccessPuts", func(b *testing.B) {
		history := sequentialSuccessPuts(7000, 2)
		shuffles := shuffleHistory(history, b.N)
		b.ResetTimer()
		validateShuffles(b, lg, shuffles, time.Second)
	})
	b.Run("SequentialFailedPuts", func(b *testing.B) {
		history := sequentialFailedPuts(14, 1)
		shuffles := shuffleHistory(history, b.N)
		b.ResetTimer()
		validateShuffles(b, lg, shuffles, time.Second)
	})
	b.Run("ConcurrentFailedPutsWithRead", func(b *testing.B) {
		history := concurrentFailedPutsWithRead(b, 13)
		shuffles := shuffleHistory(history, b.N)
		b.ResetTimer()
		validateShuffles(b, lg, shuffles, time.Second)
	})
	b.Run("BacktrackingHeavy", func(b *testing.B) {
		history := backtrackingHeavy(b)
		shuffles := shuffleHistory(history, b.N)
		keys := model.ModelKeys(history)
		b.ResetTimer()
		for i := 0; i < len(shuffles); i++ {
			validateLinearizableOperationsAndVisualize(lg, keys, shuffles[i], time.Second)
		}
	})
}

func sequentialSuccessPuts(count int, startRevision int64) []porcupine.Operation {
	ops := []porcupine.Operation{}
	for i := 0; i < count; i++ {
		ops = append(ops, porcupine.Operation{
			ClientId: i,
			Input:    putRequest("key", "value"),
			Output:   txnResponse(startRevision+int64(i), model.EtcdOperationResult{}),
			Call:     int64(i * 2),
			Return:   int64(i*2 + 1),
		})
	}
	return ops
}

func concurrentFailedPutsWithRead(tb testing.TB, concurrencyCount int) []porcupine.Operation {
	tb.Helper()
	ops := []porcupine.Operation{}
	for i := 0; i < concurrencyCount; i++ {
		ops = append(ops, porcupine.Operation{
			ClientId: i,
			Input:    putRequest("key", "value"),
			Output:   errorResponse(fmt.Errorf("timeout")),
			Call:     int64(i),
			Return:   int64(i) + int64(concurrencyCount),
		})
	}
	replay := model.NewReplayFromOperations(ops)
	state, err := replay.StateForRevision(int64(concurrencyCount) + 1)
	if err != nil {
		tb.Fatal(err)
	}
	request := rangeRequest("key", "kez", 0, 0)
	_, resp := state.Step(request)
	ops = append(ops, porcupine.Operation{
		ClientId: 0,
		Input:    request,
		Output:   resp,
		Call:     int64(concurrencyCount) + 1,
		Return:   int64(concurrencyCount) + 2,
	})
	return ops
}

func sequentialFailedPuts(count int, keyCount int) []porcupine.Operation {
	ops := []porcupine.Operation{}
	for i := 0; i < count; i++ {
		key := "key0"
		if keyCount > 1 {
			key = fmt.Sprintf("key%d", i%keyCount)
		}
		ops = append(ops, porcupine.Operation{
			ClientId: i,
			Input:    putRequest(key, "value"),
			Output:   errorResponse(fmt.Errorf("timeout")),
			Call:     int64(i * 2),
			Return:   int64(i*2 + 1),
		})
	}
	return ops
}

func impossibleReadAfterSuccessfulPut() []porcupine.Operation {
	return []porcupine.Operation{
		{
			ClientId: 0,
			Input:    putRequest("key", "value"),
			Output:   txnResponse(2, model.EtcdOperationResult{}),
			Call:     0,
			Return:   1,
		},
		{
			ClientId: 1,
			Input:    rangeRequest("key", "", 0, 0),
			Output:   rangeResponse(1, keyValueRevision("key", "wrong", 9999)),
			Call:     2,
			Return:   3,
		},
	}
}

func backtrackingHeavy(b *testing.B) (ops []porcupine.Operation) {
	for i := 0; i < 30; i++ {
		ops = append(ops, porcupine.Operation{
			ClientId: -1,
			Input:    putRequest(fmt.Sprintf("key%d", i+1000), "value"),
			Output:   txnResponse(int64(i+2), model.EtcdOperationResult{}),
			Call:     int64(i),
			Return:   int64(i) + 1,
		})
	}
	startTime := int64(1000)

	failedPuts := 4
	for i := 0; i < failedPuts; i++ {
		ops = append(ops, porcupine.Operation{
			ClientId: i,
			Input:    putRequest(fmt.Sprintf("key%d", i), "value"),
			Output:   errorResponse(fmt.Errorf("timeout")),
			Call:     startTime + int64(i),
			Return:   startTime + 1000 + int64(i),
		})
	}
	replay := model.NewReplayFromOperations(ops)
	state, err := replay.StateForRevision(int64(30 + 1))
	if err != nil {
		b.Fatal(err)
	}

	concurrentReads := 3
	for i := 0; i < concurrentReads; i++ {
		request := rangeRequest(fmt.Sprintf("key%d", i), "", 0, 0)
		_, resp := state.Step(request)
		ops = append(ops, porcupine.Operation{
			ClientId: failedPuts + i,
			Input:    request,
			Output:   resp,
			Call:     startTime + 1100,
			Return:   startTime + 2100,
		})
	}

	ops = append(ops, porcupine.Operation{
		ClientId: 99,
		Input:    rangeRequest("key0", "", 0, 0),
		Output:   rangeResponse(0, keyValueRevision("key0", "wrong", 9999)),
		Call:     startTime + 3000,
		Return:   startTime + 4000,
	})
	return ops
}

func shuffleHistory(history []porcupine.Operation, shuffleCount int) [][]porcupine.Operation {
	shuffles := make([][]porcupine.Operation, shuffleCount)
	for i := 0; i < shuffleCount; i++ {
		historyCopy := make([]porcupine.Operation, len(history))
		copy(historyCopy, history)
		rand.Shuffle(len(historyCopy), func(i, j int) {
			historyCopy[i], historyCopy[j] = historyCopy[j], historyCopy[i]
		})
		shuffles[i] = historyCopy
	}
	return shuffles
}

func validateShuffles(b *testing.B, lg *zap.Logger, shuffles [][]porcupine.Operation, duration time.Duration) {
	keys := model.ModelKeys(shuffles[0])
	for i := 0; i < len(shuffles); i++ {
		result := validateLinearizableOperationsAndVisualize(lg, keys, shuffles[i], duration)
		if err := result.Error(); err != nil {
			b.Fatalf("Not linearizable: %v", err)
		}
	}
}

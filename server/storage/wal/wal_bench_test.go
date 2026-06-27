// Copyright 2015 The etcd Authors
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

package wal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/protobuf/proto"

	"go.etcd.io/raft/v3/raftpb"
)

var k8sBatchDistribution = []int{
	2, 1, 1, 1, 1, 2, 83, 2, 83, 1, 1, 101, 148, 252, 147, 152, 1, 101, 104, 1,
	203, 195, 174, 22, 31, 1, 212, 187, 1, 124, 248, 28, 1, 1, 216, 183, 187, 184, 30, 1,
	30, 190, 180, 191, 204, 1, 25, 1, 1, 179, 221, 1, 398, 1, 1, 2, 170, 230, 168, 229,
	1, 3, 1, 370, 9, 1, 157, 241, 21, 1, 1, 343, 43, 353, 44, 16, 1, 4, 307, 89,
	308, 91, 2, 1, 136, 143, 1, 166, 231, 122, 1, 1, 50, 294, 101, 295, 59, 2, 1,
	327, 27, 176, 193, 1, 76, 1, 1, 29, 300, 38, 356, 10, 67, 1, 320, 13, 178, 114,
	1, 168, 5, 185, 48, 210, 184, 26, 143, 191, 44, 1, 256, 143, 1, 4, 163, 233, 1,
	237, 163, 1, 48, 202, 1, 85,
}

func BenchmarkWrite100EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 100, []int{1}) }
func BenchmarkWrite100EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 100, []int{10}) }
func BenchmarkWrite100EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 100, []int{100}) }
func BenchmarkWrite100EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 100, []int{500}) }
func BenchmarkWrite100EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 100, []int{1000}) }

func BenchmarkWrite1000EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 1000, []int{1}) }
func BenchmarkWrite1000EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 1000, []int{10}) }
func BenchmarkWrite1000EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 1000, []int{100}) }
func BenchmarkWrite1000EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 1000, []int{500}) }
func BenchmarkWrite1000EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 1000, []int{1000}) }

func BenchmarkWrite10KEntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 10145, []int{1}) }
func BenchmarkWrite10KEntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 10145, []int{10}) }
func BenchmarkWrite10KEntryBatch50(b *testing.B)      { benchmarkWriteEntry(b, 10145, []int{50}) }
func BenchmarkWrite10KEntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 10145, []int{100}) }
func BenchmarkWrite10KEntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 10145, []int{500}) }

func BenchmarkWrite10KEntryK8sDistribution(b *testing.B) {
	benchmarkWriteEntry(b, 10145, k8sBatchDistribution)
}

func benchmarkWriteEntry(b *testing.B, size int, batchSamples []int) {
	p := b.TempDir()

	w, err := Create(zaptest.NewLogger(b), p, []byte("somedata"))
	require.NoErrorf(b, err, "err = %v, want nil", err)
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}
	e := &raftpb.Entry{Data: data}

	b.ResetTimer()
	b.SetBytes(int64(proto.Size(e)))

	i := 0
	batchIdx := 0
	for i < b.N {
		batchLimit := 1
		if len(batchSamples) > 0 {
			batchLimit = batchSamples[batchIdx]
			batchIdx = (batchIdx + 1) % len(batchSamples)
		}

		for j := 0; j < batchLimit && i < b.N; j++ {
			err := w.saveEntry(e)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
		w.sync()
	}
}

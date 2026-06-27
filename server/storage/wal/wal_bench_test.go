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

func BenchmarkWrite100EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 100, 0) }
func BenchmarkWrite100EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 100, 10) }
func BenchmarkWrite100EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 100, 100) }
func BenchmarkWrite100EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 100, 500) }
func BenchmarkWrite100EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 100, 1000) }

func BenchmarkWrite1000EntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 1000, 0) }
func BenchmarkWrite1000EntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 1000, 10) }
func BenchmarkWrite1000EntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 1000, 100) }
func BenchmarkWrite1000EntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 1000, 500) }
func BenchmarkWrite1000EntryBatch1000(b *testing.B)    { benchmarkWriteEntry(b, 1000, 1000) }

func BenchmarkWrite10KEntryWithoutBatch(b *testing.B) { benchmarkWriteEntry(b, 10145, 0) }
func BenchmarkWrite10KEntryBatch10(b *testing.B)      { benchmarkWriteEntry(b, 10145, 10) }
func BenchmarkWrite10KEntryBatch50(b *testing.B)      { benchmarkWriteEntry(b, 10145, 50) }
func BenchmarkWrite10KEntryBatch100(b *testing.B)     { benchmarkWriteEntry(b, 10145, 100) }
func BenchmarkWrite10KEntryBatch500(b *testing.B)     { benchmarkWriteEntry(b, 10145, 500) }

func BenchmarkWrite10KEntryK8sDistribution(b *testing.B) {
	p := b.TempDir()

	w, err := Create(zaptest.NewLogger(b), p, []byte("somedata"))
	require.NoErrorf(b, err, "err = %v, want nil", err)
	data := make([]byte, 10145)
	for i := 0; i < len(data); i++ {
		data[i] = byte(i % 256)
	}
	e := &raftpb.Entry{Data: data}

	// Distribution of batch sizes observed during K8s delete-create benchmark run
	batchSizes := []int{
		2, 1, 1, 1, 1, 1, 63, 6, 63, 1, 4, 1, 1, 392, 8, 194, 205, 1, 1, 1,
		388, 11, 123, 277, 1, 3, 1, 133, 176, 1, 110, 204, 7, 1, 1, 369, 31,
		273, 126, 1, 1, 65, 308, 91, 308, 28, 2, 1, 238, 126,
	}
	batchIdx := 0

	b.ResetTimer()
	b.SetBytes(int64(proto.Size(e)))

	i := 0
	for i < b.N {
		batchLimit := batchSizes[batchIdx]
		batchIdx = (batchIdx + 1) % len(batchSizes)

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




func benchmarkWriteEntry(b *testing.B, size int, batch int) {
	p := b.TempDir()

	w, err := Create(zaptest.NewLogger(b), p, []byte("somedata"))
	require.NoErrorf(b, err, "err = %v, want nil", err)
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i)
	}
	e := &raftpb.Entry{Data: data}

	b.ResetTimer()
	n := 0
	b.SetBytes(int64(proto.Size(e)))
	for i := 0; i < b.N; i++ {
		err := w.saveEntry(e)
		if err != nil {
			b.Fatal(err)
		}
		n++
		if n > batch {
			w.sync()
			n = 0
		}
	}
}

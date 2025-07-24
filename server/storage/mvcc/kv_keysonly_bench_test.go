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

package mvcc

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"go.uber.org/zap/zaptest"

	"go.etcd.io/etcd/server/v3/lease"
	betesting "go.etcd.io/etcd/server/v3/storage/backend/testing"
)

// BenchmarkRangeKeysOnly compares memory usage and performance between
// range operations with and without KeysOnly
func BenchmarkRangeKeysOnly(b *testing.B) {
	valueSizes := []int{64, 1024, 4096, 16384} // Different value sizes
	keyCounts := []int{100, 1000, 10000}       // Different number of keys

	for _, valueSize := range valueSizes {
		for _, keyCount := range keyCounts {
			b.Run(fmt.Sprintf("ValueSize%d/Keys%d", valueSize, keyCount), func(b *testing.B) {
				benchmarkRangeKeysOnlyWithParams(b, valueSize, keyCount)
			})
		}
	}
}

func benchmarkRangeKeysOnlyWithParams(b *testing.B, valueSize, keyCount int) {
	// Setup store and data
	be, _ := betesting.NewDefaultTmpBackend(b)
	s := NewStore(zaptest.NewLogger(b), be, &lease.FakeLessor{}, StoreConfig{})
	defer func() {
		s.Close()
		be.Close()
	}()

	// Create test data with specified value size
	value := make([]byte, valueSize)
	for i := range value {
		value[i] = byte(i % 256)
	}

	// Populate store
	for i := 0; i < keyCount; i++ {
		key := []byte(fmt.Sprintf("key%06d", i))
		s.Put(key, value, lease.NoLease)
	}

	// Force commit to ensure data is persisted
	s.Commit()

	b.Run("WithValues", func(b *testing.B) {
		benchmarkRange(b, s, false)
	})

	b.Run("KeysOnly", func(b *testing.B) {
		benchmarkRange(b, s, true)
	})
}

func benchmarkRange(b *testing.B, s KV, keysOnly bool) {
	ro := RangeOptions{KeysOnly: keysOnly}

	// Measure initial memory
	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := s.Range(context.Background(), []byte("key"), []byte("key999999"), ro)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()

	// Measure final memory
	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Report memory usage
	allocBytes := m2.TotalAlloc - m1.TotalAlloc
	b.ReportMetric(float64(allocBytes)/float64(b.N), "alloc-bytes/op")
}

// BenchmarkRangeKeysOnlyMemoryEfficiency measures memory efficiency of KeysOnly
func BenchmarkRangeKeysOnlyMemoryEfficiency(b *testing.B) {
	be, _ := betesting.NewDefaultTmpBackend(b)
	defer be.Close()
	s := NewStore(zaptest.NewLogger(b), be, &lease.FakeLessor{}, StoreConfig{})
	defer s.Close()

	// Setup large values to see memory difference
	valueSize := 8192
	numKeys := 1000

	// Put keys with large values
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%05d", i)
		value := make([]byte, valueSize)
		for j := range value {
			value[j] = byte(i % 256)
		}
		s.Put([]byte(key), value, lease.NoLease)
	}

	b.Run("WithLargeValues", func(b *testing.B) {
		var totalValueBytes, totalKeyBytes int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := s.Range(context.Background(), []byte("key_"), []byte("key_~"), RangeOptions{})
			if err != nil {
				b.Fatal(err)
			}
			for _, kv := range r.KVs {
				totalKeyBytes += int64(len(kv.Key))
				totalValueBytes += int64(len(kv.Value))
			}
		}
		b.ReportMetric(float64(totalKeyBytes/int64(b.N)), "avg-key-bytes/op")
		b.ReportMetric(float64(totalValueBytes/int64(b.N)), "avg-value-bytes/op")
	})

	b.Run("KeysOnlyLargeValues", func(b *testing.B) {
		var totalValueBytes, totalKeyBytes int64
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := s.Range(context.Background(), []byte("key_"), []byte("key_~"), RangeOptions{KeysOnly: true})
			if err != nil {
				b.Fatal(err)
			}
			for _, kv := range r.KVs {
				totalKeyBytes += int64(len(kv.Key))
				totalValueBytes += int64(len(kv.Value))
			}
		}
		b.ReportMetric(float64(totalKeyBytes/int64(b.N)), "avg-key-bytes/op")
		b.ReportMetric(float64(totalValueBytes/int64(b.N)), "avg-value-bytes/op")
	})
}

// BenchmarkRangeKeysOnlyThroughput measures throughput differences
func BenchmarkRangeKeysOnlyThroughput(b *testing.B) {
	be, _ := betesting.NewDefaultTmpBackend(b)
	s := NewStore(zaptest.NewLogger(b), be, &lease.FakeLessor{}, StoreConfig{})
	defer func() {
		s.Close()
		be.Close()
	}()

	// Create test data
	value := make([]byte, 1024) // 1KB values
	for i := range value {
		value[i] = byte(i % 256)
	}

	keyCount := 5000
	for i := 0; i < keyCount; i++ {
		key := []byte(fmt.Sprintf("key%06d", i))
		s.Put(key, value, lease.NoLease)
	}

	s.Commit()

	b.Run("WithValues", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := s.Range(context.Background(), []byte("key"), []byte("key999999"), RangeOptions{KeysOnly: false})
			if err != nil {
				b.Fatal(err)
			}
			if len(r.KVs) == 0 {
				b.Fatal("no keys returned")
			}
		}
	})

	b.Run("KeysOnly", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, err := s.Range(context.Background(), []byte("key"), []byte("key999999"), RangeOptions{KeysOnly: true})
			if err != nil {
				b.Fatal(err)
			}
			if len(r.KVs) == 0 {
				b.Fatal("no keys returned")
			}
		}
	})
}

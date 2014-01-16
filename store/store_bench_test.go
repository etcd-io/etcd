/*
Copyright 2014 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"testing"
)

func BenchmarkStoreSet(b *testing.B) {
	s := newStore()
	b.StopTimer()
	kvs, size := generateNRandomKV(b.N)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, err := s.Set(kvs[i][0], false, kvs[i][1], Permanent)
		if err != nil {
			panic(err)
		}
	}

	memStats := new(runtime.MemStats)
	runtime.GC()
	runtime.ReadMemStats(memStats)
	fmt.Printf("\nAlloc: %vKB; Data: %vKB; Kvs: %v\n",
		memStats.Alloc/1000, size/1000, b.N)
}

func BenchmarkStoreSetWithJson(b *testing.B) {
	s := newStore()
	b.StopTimer()
	kvs, size := generateNRandomKV(b.N)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		resp, err := s.Set(kvs[i][0], false, kvs[i][1], Permanent)
		if err != nil {
			panic(err)
		}
		_, err = json.Marshal(resp)
		if err != nil {
			panic(err)
		}
	}

	memStats := new(runtime.MemStats)
	runtime.GC()
	runtime.ReadMemStats(memStats)
	fmt.Printf("\nAlloc: %vKB; Data: %vKB; Kvs: %v\n",
		memStats.Alloc/1000, size/1000, b.N)
}

func generateNRandomKV(n int) ([][]string, uint64) {
	var size uint64
	kvs := make([][]string, n)
	bytes := make([]byte, 128) //128Byte content
	for i := range bytes {
		bytes[i] = byte(rand.Int())
	}

	for i := 0; i < n; i++ {
		kvs[i] = make([]string, 2)
		kvs[i][0] = fmt.Sprintf("/%d/%d/%d",
			rand.Int()%100, rand.Int()%100, rand.Int()%100)
		kvs[i][1] = string(bytes)
		size = size + uint64(len(kvs[i][0])) + uint64(len(kvs[i][1]))
	}

	return kvs, size
}

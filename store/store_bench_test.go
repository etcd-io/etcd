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
	"testing"
)

func BenchmarkStoreSet(b *testing.B) {
	s := newStore()
	b.StopTimer()
	kvs := generateNRandomKV(b.N)
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, err := s.Set(kvs[i][0], false, kvs[i][1], Permanent)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkStoreSetWithJson(b *testing.B) {
	s := newStore()
	b.StopTimer()
	kvs := generateNRandomKV(b.N)
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
}

func generateNRandomKV(n int) [][]string {
	kvs := make([][]string, n)

	for i := 0; i < n; i++ {
		kvs[i] = make([]string, 2)
		kvs[i][0] = fmt.Sprintf("/%d/%d/%d",
			rand.Int()%100, rand.Int()%100, rand.Int()%100)
		kvs[i][1] = fmt.Sprint(i)
	}

	return kvs
}

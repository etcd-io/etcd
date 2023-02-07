// Copyright 2023 The etcd Authors
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

package ttlcache

import (
	"math/rand"
	"strconv"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Benchmark_Set(b *testing.B) {
	b.ReportAllocs()

	// Benchmark for the case when changes are caused by capacity limit
	loc, _ := time.LoadLocation("UTC")
	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)

	c := NewCache[string, *CacheObj](10, time.Second)

	for i := 0; i < b.N; i++ {
		key := "k" + strconv.Itoa(i)
		obj := &CacheObj{key, i}
		c.Set(key, obj, now)
		assertInCache(b, c, &CacheObj{key, i}, now)
	}
}

func Benchmark_Get(b *testing.B) {
	b.ReportAllocs()

	// Benchmark for the case when changes are caused by ttl limit
	loc, _ := time.LoadLocation("UTC")
	now := time.Date(2023, 01, 01, 00, 00, 00, 0, loc)

	c := NewCache[string, *CacheObj](1000, time.Second)

	for i := 0; i < 1000; i++ {
		key := "k" + strconv.Itoa(i)
		c.Set(key, &CacheObj{key, i}, now)
	}

	expired := now.Add(time.Hour).Add(time.Second)

	for i := 0; i < b.N; i++ {
		ki := rand.Intn(1000)
		key := "k" + strconv.Itoa(ki)
		obj := &CacheObj{key, i}

		assertNotInCache(b, c, obj, expired)

		c.Set(key, obj, now)
	}
}

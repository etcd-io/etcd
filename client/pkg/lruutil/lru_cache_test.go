// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lruutil

import (
	"testing"
	"time"
)

func TestLruCache(t *testing.T) {
	tests := []struct {
		vals     []string
		interval time.Duration
		ttl      time.Duration
		expect   int
	}{
		{
			vals:     []string{"a", "b", "c", "d"},
			interval: time.Millisecond * 50,
			ttl:      time.Second,
			expect:   4,
		},
		{
			vals:     []string{"a", "b", "c", "d"},
			interval: time.Second * 2,
			ttl:      time.Second,
			expect:   0,
		},
	}
	for i, tt := range tests {
		sf := NewTimeEvictLru(tt.ttl)
		for _, v := range tt.vals {
			sf.Set(v, []byte(v))
		}
		time.Sleep(tt.interval)
		for _, v := range tt.vals {
			sf.Get(v)
		}
		if tt.expect != sf.Len() {
			t.Fatalf("#%d: expected %+v, got %+v", i, tt.expect, sf.Len())
		}
	}
}

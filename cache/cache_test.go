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

package cache

import (
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestValidateWatchRange(t *testing.T) {
	type tc struct {
		name        string
		watchKey    string
		opts        []clientv3.OpOption
		cachePrefix string
		wantErr     bool
	}

	tests := []tc{
		{
			name:        "single key",
			watchKey:    "/a",
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "prefix single key",
			watchKey:    "/foo/a",
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "single key outside prefix returns error",
			watchKey:    "/z",
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "explicit range",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/b")},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "exact prefix range",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/b")},
			cachePrefix: "/a",
			wantErr:     false,
		},
		{
			name:        "prefix subrange",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo/a")},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "reverse range returns error",
			watchKey:    "/b",
			opts:        []clientv3.OpOption{clientv3.WithRange("/a")},
			cachePrefix: "",
			wantErr:     true,
		},
		{
			name:        "empty range returns error",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo")},
			cachePrefix: "",
			wantErr:     true,
		},
		{
			name:        "range starting below cache prefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/foo")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "range encompassing cache prefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithRange("/z")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "range crossing prefixEnd returns error",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithRange("/z")},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "empty prefix",
			watchKey:    "",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "empty prefix with cachePrefix returns error",
			watchKey:    "",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "prefix watch matches cachePrefix exactly",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "prefix watch inside cachePrefix",
			watchKey:    "/foo/bar",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     false,
		},
		{
			name:        "prefix starting below cachePrefix returns error",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "prefix starting above shard prefixEnd returns error",
			watchKey:    "/fop",
			opts:        []clientv3.OpOption{clientv3.WithPrefix()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey open‑ended",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "",
			wantErr:     false,
		},
		{
			name:        "fromKey starting at prefix start",
			watchKey:    "/foo",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey starting below prefixEnd",
			watchKey:    "/a",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
		{
			name:        "fromKey starting above prefixEnd returns error",
			watchKey:    "/fop",
			opts:        []clientv3.OpOption{clientv3.WithFromKey()},
			cachePrefix: "/foo",
			wantErr:     true,
		},
	}

	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			dummyCache := &Cache{prefix: c.cachePrefix}
			op := clientv3.OpGet(c.watchKey, c.opts...)
			err := dummyCache.validateWatchRange([]byte(c.watchKey), op.RangeBytes())
			if gotErr := err != nil; gotErr != c.wantErr {
				t.Fatalf("validateWatchRange(%q, %q, %v) err=%v, wantErr=%v",
					c.cachePrefix, c.watchKey, c.opts, err, c.wantErr)
			}
		})
	}
}

// Copyright 2026 The etcd Authors
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

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
)

func TestCacheIsolatesResponsesByAuthToken(t *testing.T) {
	c := NewCache(10)

	req := &pb.RangeRequest{Key: []byte("protected")}
	rootResp := &pb.RangeResponse{
		Kvs: []*mvccpb.KeyValue{{Key: []byte("protected"), Value: []byte("secret")}},
	}

	c.Add(req, rootResp, "root-token")

	// A different caller (or no caller at all) must not receive the cached
	// response produced for an authenticated principal.
	if got, err := c.Get(req, "low-privilege-token"); err == nil {
		t.Fatalf("cache hit across auth tokens: %+v", got)
	}
	if got, err := c.Get(req, ""); err == nil {
		t.Fatalf("cache hit without a token: %+v", got)
	}

	got, err := c.Get(req, "root-token")
	if err != nil {
		t.Fatalf("cache miss for the original token: %v", err)
	}
	if len(got.Kvs) != 1 || string(got.Kvs[0].Value) != "secret" {
		t.Fatalf("unexpected cached response: %+v", got)
	}
}

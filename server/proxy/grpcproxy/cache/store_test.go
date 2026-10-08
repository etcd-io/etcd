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
	"time"

	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestCacheAuthKeyIsolation(t *testing.T) {
	c := NewCache(10)
	defer c.Close()

	req := &pb.RangeRequest{Key: []byte("secret"), Serializable: true}
	resp := &pb.RangeResponse{
		Header: &pb.ResponseHeader{Revision: 1},
		Kvs:    []*mvccpb.KeyValue{{Key: []byte("secret"), Value: []byte("s3cr3t")}},
	}

	// Store under identity "alice"
	c.Add(req, resp, "alice")

	// Same identity must hit
	got, err := c.Get(req, "alice")
	if err != nil {
		t.Fatalf("expected cache hit for same identity, got %v", err)
	}
	if string(got.Kvs[0].Value) != "s3cr3t" {
		t.Fatalf("unexpected value %q", got.Kvs[0].Value)
	}

	// Different identity must miss
	_, err = c.Get(req, "bob")
	if err == nil {
		t.Fatal("expected cache miss for different identity")
	}

	// Empty identity must miss
	_, err = c.Get(req, "")
	if err == nil {
		t.Fatal("expected cache miss for empty identity")
	}

	// Store under empty identity; other identities still miss
	c.Add(req, resp, "")
	if _, err = c.Get(req, ""); err != nil {
		t.Fatalf("expected cache hit for empty identity, got %v", err)
	}
	// "alice" hits because her earlier Add(req,resp,"alice") entry is still
	// resident, not because "" collides with a named identity.
	if _, err = c.Get(req, "alice"); err != nil {
		t.Fatalf("expected cache hit for alice after empty-identity add (same key), got %v", err)
	}
}

func TestCacheClear(t *testing.T) {
	c := NewCache(10)
	defer c.Close()

	req := &pb.RangeRequest{Key: []byte("k"), Serializable: true}
	resp := &pb.RangeResponse{Header: &pb.ResponseHeader{Revision: 1}}

	c.Add(req, resp, "alice")
	c.Add(req, resp, "bob")
	c.Add(req, resp, "")

	if size := c.Size(); size != 3 {
		t.Fatalf("expected 3 entries, got %d", size)
	}

	c.Clear()

	if size := c.Size(); size != 0 {
		t.Fatalf("expected 0 entries after Clear, got %d", size)
	}
	for _, id := range []string{"alice", "bob", ""} {
		if _, err := c.Get(req, id); err == nil {
			t.Fatalf("expected miss after Clear for identity %q", id)
		}
	}

	// Cache must remain usable after Clear
	c.Add(req, resp, "alice")
	if _, err := c.Get(req, "alice"); err != nil {
		t.Fatalf("expected hit after re-Add, got %v", err)
	}
}

func TestKeyFuncDeterministic(t *testing.T) {
	req := &pb.RangeRequest{Key: []byte("a"), Serializable: true}
	k1 := keyFunc(req, "alice")
	k2 := keyFunc(req, "alice")
	k3 := keyFunc(req, "bob")
	k4 := keyFunc(req, "")

	if k1 != k2 {
		t.Fatal("same inputs must produce same key")
	}
	if k1 == k3 {
		t.Fatal("different identities must produce different keys")
	}
	if k1 == k4 {
		t.Fatal("identity vs no-identity must produce different keys")
	}
}

func TestCacheEntryExpiry(t *testing.T) {
	// Create a cache with a very short TTL for testing
	c := NewCacheWithTTL(10, 50*time.Millisecond)
	defer c.Close()

	req := &pb.RangeRequest{Key: []byte("k"), Serializable: true}
	resp := &pb.RangeResponse{Header: &pb.ResponseHeader{Revision: 1}}

	c.Add(req, resp, "alice")

	// Entry should be available immediately
	if _, err := c.Get(req, "alice"); err != nil {
		t.Fatalf("expected cache hit before expiry, got %v", err)
	}

	// Wait for entry to expire
	time.Sleep(60 * time.Millisecond)

	// Entry should now be expired
	_, err := c.Get(req, "alice")
	if err == nil {
		t.Fatal("expected cache miss after expiry")
	}
	if err.Error() != "entry expired" {
		t.Fatalf("expected 'entry expired' error, got %v", err)
	}

	// Cache should still accept new entries
	c.Add(req, resp, "alice")
	if _, err := c.Get(req, "alice"); err != nil {
		t.Fatalf("expected cache hit after re-Add, got %v", err)
	}
}

func TestCacheNoExpiryWhenTTLZero(t *testing.T) {
	// Create a cache with no TTL (0 means no expiry)
	c := NewCacheWithTTL(10, 0)
	defer c.Close()

	req := &pb.RangeRequest{Key: []byte("k"), Serializable: true}
	resp := &pb.RangeResponse{Header: &pb.ResponseHeader{Revision: 1}}

	c.Add(req, resp, "alice")

	// Wait a bit
	time.Sleep(60 * time.Millisecond)

	// Entry should still be available (no expiry)
	if _, err := c.Get(req, "alice"); err != nil {
		t.Fatalf("expected cache hit with no TTL, got %v", err)
	}
}

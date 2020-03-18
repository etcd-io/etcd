// Copyright 2020 The etcd Authors
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

package clientv3

import (
	"context"
	"reflect"
	"testing"

	"go.etcd.io/etcd/etcdserver/api/v3rpc/rpctypes"
	"go.etcd.io/etcd/version"
	"google.golang.org/grpc/metadata"
)

func TestMetadataWithRequireLeader(t *testing.T) {
	ctx := context.TODO()
	md, ok := metadata.FromOutgoingContext(ctx)
	if ok {
		t.Fatal("expected no outgoing metadata ctx key")
	}

	// add a conflicting key with some other value
	md = metadata.Pairs(rpctypes.MetadataRequireLeaderKey, "invalid")
	// add a key, and expect not be overwritten
	metadataSet(md, "hello", "1", "2")
	ctx = metadata.NewOutgoingContext(ctx, md)

	// expect overwrites but still keep other keys
	ctx = WithRequireLeader(ctx)
	md, ok = metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata ctx key")
	}
	if ss := metadataGet(md, rpctypes.MetadataRequireLeaderKey); !reflect.DeepEqual(ss, []string{rpctypes.MetadataHasLeader}) {
		t.Fatalf("unexpected metadata for %q %v", rpctypes.MetadataRequireLeaderKey, ss)
	}
	if ss := metadataGet(md, "hello"); !reflect.DeepEqual(ss, []string{"1", "2"}) {
		t.Fatalf("unexpected metadata for 'hello' %v", ss)
	}
}

func TestMetadataWithClientAPIVersion(t *testing.T) {
	ctx := withVersion(WithRequireLeader(context.TODO()))

	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata ctx key")
	}
	if ss := metadataGet(md, rpctypes.MetadataRequireLeaderKey); !reflect.DeepEqual(ss, []string{rpctypes.MetadataHasLeader}) {
		t.Fatalf("unexpected metadata for %q %v", rpctypes.MetadataRequireLeaderKey, ss)
	}
	if ss := metadataGet(md, rpctypes.MetadataClientAPIVersionKey); !reflect.DeepEqual(ss, []string{version.APIVersion}) {
		t.Fatalf("unexpected metadata for %q %v", rpctypes.MetadataClientAPIVersionKey, ss)
	}
}

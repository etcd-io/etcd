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
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/api/v3/version"
)

func TestMetadataWithRequireLeader(t *testing.T) {
	ctx := t.Context()
	_, ok := metadata.FromOutgoingContext(ctx)
	require.Falsef(t, ok, "expected no outgoing metadata ctx key")

	// add a conflicting key with some other value
	md := metadata.Pairs(rpctypes.MetadataRequireLeaderKey, "invalid")
	// add a key, and expect not be overwritten
	md.Set("hello", "1", "2")
	ctx = metadata.NewOutgoingContext(ctx, md)

	// expect overwrites but still keep other keys
	ctx = WithRequireLeader(ctx)
	md, ok = metadata.FromOutgoingContext(ctx)
	require.Truef(t, ok, "expected outgoing metadata ctx key")
	ss := md.Get(rpctypes.MetadataRequireLeaderKey)
	require.Truef(t, reflect.DeepEqual(ss, []string{rpctypes.MetadataHasLeader}), "unexpected metadata for %q %v", rpctypes.MetadataRequireLeaderKey, ss)
	ss = md.Get("hello")
	require.Truef(t, reflect.DeepEqual(ss, []string{"1", "2"}), "unexpected metadata for 'hello' %v", ss)
}

func TestMetadataWithClientAPIVersion(t *testing.T) {
	ctx := withVersion(WithRequireLeader(t.Context()))

	md, ok := metadata.FromOutgoingContext(ctx)
	require.Truef(t, ok, "expected outgoing metadata ctx key")
	ss := md.Get(rpctypes.MetadataRequireLeaderKey)
	require.Truef(t, reflect.DeepEqual(ss, []string{rpctypes.MetadataHasLeader}), "unexpected metadata for %q %v", rpctypes.MetadataRequireLeaderKey, ss)
	ss = md.Get(rpctypes.MetadataClientAPIVersionKey)
	require.Truef(t, reflect.DeepEqual(ss, []string{version.APIVersion}), "unexpected metadata for %q %v", rpctypes.MetadataClientAPIVersionKey, ss)
}

// Copyright 2016 The etcd Authors
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
	"testing"

	"google.golang.org/grpc/metadata"

	"go.etcd.io/etcd/api/v3/mvccpb"
)

func TestEvent(t *testing.T) {
	tests := []struct {
		ev       *Event
		isCreate bool
		isModify bool
	}{{
		ev: &Event{
			Type: EventTypePut,
			Kv: &mvccpb.KeyValue{
				CreateRevision: 3,
				ModRevision:    3,
			},
		},
		isCreate: true,
	}, {
		ev: &Event{
			Type: EventTypePut,
			Kv: &mvccpb.KeyValue{
				CreateRevision: 3,
				ModRevision:    4,
			},
		},
		isModify: true,
	}}
	for i, tt := range tests {
		if tt.isCreate && !tt.ev.IsCreate() {
			t.Errorf("#%d: event should be Create event", i)
		}
		if tt.isModify && !tt.ev.IsModify() {
			t.Errorf("#%d: event should be Modify event", i)
		}
	}
}

// TestStreamKeyFromCtx tests the streamKeyFromCtx function to ensure it correctly
// formats metadata as a map[string][]string when extracting metadata from the context.
//
// The fmt package in Go guarantees that maps are printed in a consistent order,
// sorted by the keys. This test verifies that the streamKeyFromCtx function
// produces the expected formatted string representation of metadata maps when called with
// various context scenarios.
func TestStreamKeyFromCtx(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name: "multiple keys",
			ctx: metadata.NewOutgoingContext(context.Background(), metadata.MD{
				"key1": []string{"value1"},
				"key2": []string{"value2a", "value2b"},
			}),
			expected: "map[key1:[value1] key2:[value2a value2b]]",
		},
		{
			name:     "no keys",
			ctx:      metadata.NewOutgoingContext(context.Background(), metadata.MD{}),
			expected: "map[]",
		},
		{
			name: "only one key",
			ctx: metadata.NewOutgoingContext(context.Background(), metadata.MD{
				"key1": []string{"value1", "value1a"},
			}),
			expected: "map[key1:[value1 value1a]]",
		},
		{
			name:     "no metadata",
			ctx:      context.Background(),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := streamKeyFromCtx(tt.ctx)
			if actual != tt.expected {
				t.Errorf("streamKeyFromCtx() = %v, expected %v", actual, tt.expected)
			}
		})
	}
}

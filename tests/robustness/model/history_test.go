// Copyright 2025 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"testing"

	"github.com/anishathalye/porcupine"
	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/tests/v3/robustness/identity"
)

func TestHistoryAppendSuccess(t *testing.T) {
	tcs := []struct {
		name       string
		operations []porcupine.Operation
	}{
		{
			name: "append non-overlapping operations for the same clientId",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     100,
					Output:   nil,
					Return:   200,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   2,
				},
			},
		},
		{
			name: "append overlapping operations in different ClientId",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   2,
				},
				{
					ClientId: 3,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   20,
				},
				{
					ClientId: 1,
					Input:    nil,
					Call:     101,
					Output:   nil,
					Return:   200,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     3,
					Output:   nil,
					Return:   4,
				},
				{
					ClientId: 3,
					Input:    nil,
					Call:     30,
					Output:   nil,
					Return:   40,
				},
			},
		},
	}

	for _, tc := range tcs {
		h := NewAppendableHistory(identity.NewIDProvider())

		for _, operation := range tc.operations[:len(tc.operations)-1] {
			h.append(operation)
		}
	}
}

func TestHistoryAppendFailure(t *testing.T) {
	tcs := []struct {
		name        string
		operations  []porcupine.Operation
		expectError string
	}{
		{
			name: "append operation with call time > return time",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     2,
					Output:   nil,
					Return:   1,
				},
			},
			expectError: "Invalid operation, call(2) >= return(1)",
		},
		{
			name: "out of order append in the same ClientId",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 1,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   10,
				},
			},
			expectError: "Out of order append, new.call(1) <= prev.call(10)",
		},
		{
			name: "out of order append in one of the ClientIds",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 1,
					Input:    nil,
					Call:     101,
					Output:   nil,
					Return:   200,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   10,
				},
			},
			expectError: "Out of order append, new.call(1) <= prev.call(10)",
		},
		{
			name: "append overlapping operations in the same ClientId",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 1,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   20,
				},
			},
			expectError: "Overlapping operations, new.call(10) <= prev.return(100)",
		},
		{
			name: "append overlapping operations in one of the ClientIds",
			operations: []porcupine.Operation{
				{
					ClientId: 1,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 1,
					Input:    nil,
					Call:     101,
					Output:   nil,
					Return:   200,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     1,
					Output:   nil,
					Return:   100,
				},
				{
					ClientId: 2,
					Input:    nil,
					Call:     10,
					Output:   nil,
					Return:   20,
				},
			},
			expectError: "Overlapping operations, new.call(10) <= prev.return(100)",
		},
	}

	for _, tc := range tcs {
		h := NewAppendableHistory(identity.NewIDProvider())

		for _, operation := range tc.operations[:len(tc.operations)-1] {
			h.append(operation)
		}
		assert.PanicsWithValue(t, tc.expectError, func() { h.append(tc.operations[len(tc.operations)-1]) })
	}
}

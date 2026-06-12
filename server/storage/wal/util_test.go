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

package wal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestIsValidSeq(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  bool
	}{
		{
			name:  "empty slice",
			names: nil,
			want:  true,
		},
		{
			name:  "single file",
			names: []string{walName(0, 0)},
			want:  true,
		},
		{
			name:  "sequential from zero",
			names: []string{walName(0, 0), walName(1, 0), walName(2, 0)},
			want:  true,
		},
		{
			name:  "sequential non-zero start",
			names: []string{walName(5, 0), walName(6, 0), walName(7, 0)},
			want:  true,
		},
		{
			name:  "gap in sequence",
			names: []string{walName(0, 0), walName(1, 0), walName(3, 0)},
			want:  false,
		},
		{
			name:  "duplicate sequence number",
			names: []string{walName(0, 0), walName(1, 0), walName(1, 0)},
			want:  false,
		},
		{
			name:  "two files sequential",
			names: []string{walName(0, 0), walName(1, 0)},
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSeq(zaptest.NewLogger(t), tt.names)
			assert.Equal(t, tt.want, got)
		})
	}
}

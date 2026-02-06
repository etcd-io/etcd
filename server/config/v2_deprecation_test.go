// Copyright 2021 The etcd Authors
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

package config

import "testing"

func TestV2DeprecationEnum_IsAtLeast(t *testing.T) {
	tests := []struct {
		e    V2DeprecationEnum
		v2d  V2DeprecationEnum
		want bool
	}{
		{V2Depr0NotYet, V2Depr0NotYet, true},
		{V2Depr0NotYet, V2Depr1WriteOnlyDrop, false},
		{V2Depr0NotYet, V2Depr2Gone, false},
		{V2Depr2Gone, V2Depr1WriteOnlyDrop, true},
		{V2Depr2Gone, V2Depr0NotYet, true},
		{V2Depr2Gone, V2Depr2Gone, true},
		{V2Depr1WriteOnly, V2Depr1WriteOnlyDrop, false},
		{V2Depr1WriteOnlyDrop, V2Depr1WriteOnly, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.e)+" >= "+string(tt.v2d), func(t *testing.T) {
			if got := tt.e.IsAtLeast(tt.v2d); got != tt.want {
				t.Errorf("IsAtLeast() = %v, want %v", got, tt.want)
			}
		})
	}
}

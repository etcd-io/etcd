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
		{V2_DEPR_0_NOT_YET, V2_DEPR_0_NOT_YET, true},
		{V2_DEPR_0_NOT_YET, V2_DEPR_1_WRITE_ONLY_DROP, false},
		{V2_DEPR_0_NOT_YET, V2_DEPR_2_GONE, false},
		{V2_DEPR_2_GONE, V2_DEPR_1_WRITE_ONLY_DROP, true},
		{V2_DEPR_2_GONE, V2_DEPR_0_NOT_YET, true},
		{V2_DEPR_2_GONE, V2_DEPR_2_GONE, true},
		{V2_DEPR_1_WRITE_ONLY, V2_DEPR_1_WRITE_ONLY_DROP, false},
		{V2_DEPR_1_WRITE_ONLY_DROP, V2_DEPR_1_WRITE_ONLY, true},
	}
	for _, tt := range tests {
		t.Run(string(tt.e)+" >= "+string(tt.v2d), func(t *testing.T) {
			if got := tt.e.IsAtLeast(tt.v2d); got != tt.want {
				t.Errorf("IsAtLeast() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Copyright 2019 The etcd Authors
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

package logutil

import (
	"testing"
)

func TestLogFormat(t *testing.T) {
	tests := []struct {
		given       string
		want        string
		errExpected bool
	}{
		{"json", JSONLogFormat, false},
		{"console", ConsoleLogFormat, false},
		{"", JSONLogFormat, false},
		{"konsole", "", true},
	}

	for i, tt := range tests {
		got, err := ConvertToZapFormat(tt.given)
		if got != tt.want {
			t.Errorf("#%d: ConvertToZapFormat failure: want=%v, got=%v", i, tt.want, got)
		}

		if err != nil {
			if !tt.errExpected {
				t.Errorf("#%d: ConvertToZapFormat unexpected error: %v", i, err)
			}
		}
	}
}

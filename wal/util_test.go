// Copyright 2018 The etcd Authors
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
	"fmt"
	"testing"

	"go.uber.org/zap"
)

func makeFilesName(seq []int) []string {
	filesName := make([]string, len(seq))
	for i, s := range seq {
		filesName[i] = fmt.Sprintf("%016x-0000000000000000.wal", s)
	}
	return filesName
}

func TestIsValidSeq(t *testing.T) {
	tests := []struct {
		seq  []int
		want bool
	}{
		{[]int{0, 1, 2, 3, 4}, true},
		{[]int{1, 2, 3, 4}, true},
		{[]int{0, 2, 3, 4}, false},
		{[]int{0, 1, 2, 3, 4, 6}, false},
		{[]int{4, 0, 2, 3, 4, 6}, false},
	}
	for i, tt := range tests {
		filesName := makeFilesName(tt.seq)
		got := isValidSeq(zap.NewExample(), filesName)
		if got != tt.want {
			t.Fatalf("#%d: want=%#v, got=%#v", i, tt.want, got)
		}
	}
}

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

package stringutil

import (
	"fmt"
	"testing"
)

const testSamplesLength = 10

func TestUniqueStrings(t *testing.T) {
	ss := UniqueStrings(testSamplesLength, 50)
	for i := 1; i < len(ss); i++ {
		if ss[i-1] == ss[i] {
			t.Fatalf("ss[i-1] %q == ss[i] %q", ss[i-1], ss[i])
		}
	}
	fmt.Println(ss)
}

func TestRandString(t *testing.T) {
	const count = 50
	ss := make([]string, count)
	for i := 0; i < count; i++ {
		ss[i] = RandString(testSamplesLength)
	}

	// search for duplicate strings
	for i := 1; i < count; i++ {
		for n := i+1; n < count; n++ {
			if ss[i] == ss[n] {
				t.Fatalf("ss[%d] %q == ss[%d] %q", i, ss[i], n, ss[n])
			}
		}
	}

	// validate length of each sample
	for i := 0; i < count; i++ {
		if len(ss[i]) != testSamplesLength {
			t.Fatalf("ss[%d] %q length != %d", i, ss[i], testSamplesLength)
		}
	}

	fmt.Println(ss)
}

// Copyright 2017 The etcd Authors
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

import "testing"

func TestSetSeed(t *testing.T) {
	SetSeed(1)
	str1 := RandomStrings(5, 1)[0]
	SetSeed(2)
	str2 := RandomStrings(5, 1)[0]
	SetSeed(1)
	str3 := RandomStrings(5, 1)[0]
	if str1 == str2 {
		t.Fatalf("expect str1 %v and str2 %v be different, but they are the same", str1, str2)
	}
	if str1 != str3 {
		t.Fatalf("expect SetSeed(1) to produce same first string, but got different ones %v %v", str1, str3)
	}
}

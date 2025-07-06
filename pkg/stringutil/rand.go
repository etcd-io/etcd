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
	"math/rand"
	"time"
)

// UniqueStrings returns a slice of randomly generated unique strings.
func UniqueStrings(slen uint, n int) (ss []string) {
	exist := make(map[string]struct{})
	ss = make([]string, 0, n)
	for len(ss) < n {
		s := randString(slen)
		if _, ok := exist[s]; !ok {
			ss = append(ss, s)
			exist[s] = struct{}{}
		}
	}
	return ss
}

// RandomStrings returns a slice of randomly generated strings.
func RandomStrings(slen uint, n int) (ss []string) {
	ss = make([]string, 0, n)
	for i := 0; i < n; i++ {
		ss = append(ss, randString(slen))
	}
	return ss
}

const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randString(l uint) string {
	rand.Seed(time.Now().UnixNano())
	s := make([]byte, l)
	for i := 0; i < int(l); i++ {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

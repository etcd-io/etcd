/*
Copyright 2013 CoreOS Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"math/rand"
	"strconv"
)

// GenKeys randomly generate num of keys with max depth
func GenKeys(num int, depth int) []string {
	keys := make([]string, num)
	for i := 0; i < num; i++ {

		keys[i] = "/foo/"
		depth := rand.Intn(depth) + 1

		for j := 0; j < depth; j++ {
			keys[i] += "/" + strconv.Itoa(rand.Int())
		}
	}
	return keys
}

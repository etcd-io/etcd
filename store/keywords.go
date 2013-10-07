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
	"path"
	"strings"
)

// keywords for internal useage
// Key for string keyword; Value for only checking prefix
var keywords = map[string]bool{
	"/_etcd":          true,
	"/ephemeralNodes": true,
}

// CheckKeyword will check if the key contains the keyword.
// For now, we only check for prefix.
func CheckKeyword(key string) bool {
	key = path.Clean("/" + key)

	// find the second "/"
	i := strings.Index(key[1:], "/")

	var prefix string

	if i == -1 {
		prefix = key
	} else {
		prefix = key[:i+1]
	}
	_, ok := keywords[prefix]

	return ok
}

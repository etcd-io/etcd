// Copyright 2024 The etcd Authors
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

package util

import "strings"

const indentation = "  "

// Normalize normalizes a string:
//  1. trim the leading and trailing space
//  2. add an indentation before each line
func Normalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return normalizer{s}.trim().indent().string
}

type normalizer struct {
	string
}

func (n normalizer) trim() normalizer {
	n.string = strings.TrimSpace(n.string)
	return n
}

func (n normalizer) indent() normalizer {
	indentedLines := []string{}
	for _, line := range strings.Split(n.string, "\n") {
		trimmed := strings.TrimSpace(line)
		indented := indentation + trimmed
		indentedLines = append(indentedLines, indented)
	}
	n.string = strings.Join(indentedLines, "\n")
	return n
}

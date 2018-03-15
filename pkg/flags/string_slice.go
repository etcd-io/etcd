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

package flags

import (
	"sort"
	"strings"
)

// StringSlice wraps "sort.StringSlice".
type StringSlice sort.StringSlice

// Set parses a command line set of strings, separated by comma.
func (ss *StringSlice) Set(s string) error {
	*ss = strings.Split(s, ",")
	return nil
}

func (ss *StringSlice) String() string { return strings.Join(*ss, ",") }

// NewStringSlice implements string slice as flag.Value interface.
// Given value is to be separated by comma.
func NewStringSlice(s string) (ss *StringSlice) {
	if s == "" {
		return &StringSlice{}
	}
	ss = new(StringSlice)
	if err := ss.Set(s); err != nil {
		plog.Panicf("new StringSlice should never fail: %v", err)
	}
	return ss
}

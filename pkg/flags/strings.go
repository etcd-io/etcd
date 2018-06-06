// Copyright 2015 The etcd Authors
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
	"errors"
	"flag"
	"sort"
	"strings"
)

// NewStringsFlag creates a new string flag for which any one of the given
// strings is a valid value, and any other value is an error.
func NewStringsFlag(valids ...string) *StringsFlag {
	return &StringsFlag{Values: valids}
}

// StringsFlag implements the flag.Value interface.
type StringsFlag struct {
	Values []string
	val    string
}

// Set verifies the argument to be a valid member of the allowed values
// before setting the underlying flag value.
func (ss *StringsFlag) Set(s string) error {
	for _, v := range ss.Values {
		if s == v {
			ss.val = s
			return nil
		}
	}
	return errors.New("invalid value")
}

// String returns the set value (if any) of the StringsFlag
func (ss *StringsFlag) String() string {
	return ss.val
}

// StringsValueV2 wraps "sort.StringSlice".
type StringsValueV2 sort.StringSlice

// Set parses a command line set of strings, separated by comma.
// Implements "flag.Value" interface.
func (ss *StringsValueV2) Set(s string) error {
	*ss = strings.Split(s, ",")
	return nil
}

// String implements "flag.Value" interface.
func (ss *StringsValueV2) String() string { return strings.Join(*ss, ",") }

// NewStringsValueV2 implements string slice as "flag.Value" interface.
// Given value is to be separated by comma.
func NewStringsValueV2(s string) (ss *StringsValueV2) {
	if s == "" {
		return &StringsValueV2{}
	}
	ss = new(StringsValueV2)
	if err := ss.Set(s); err != nil {
		plog.Panicf("new StringsValueV2 should never fail: %v", err)
	}
	return ss
}

// StringsFromFlagV2 returns a string slice from the flag.
func StringsFromFlagV2(fs *flag.FlagSet, flagName string) []string {
	return []string(*fs.Lookup(flagName).Value.(*StringsValueV2))
}

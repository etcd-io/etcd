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
	"strings"
)

// NewStringsFlag creates a new string flag for which any one of the given
// strings is a valid value, and any other value is an error.
//
// valids[0] will be default value. Caller must be sure len(valids)!=0 or
// it will panic.
func NewStringsFlag(valids ...string) *StringsFlag {
	return &StringsFlag{Values: valids, val: valids[0]}
}

// StringsFlag implements the flag.Value and pflag.Value interfaces.
type StringsFlag struct {
	Values []string
	val    string
}

// Set verifies the argument to be a valid member of the allowed values
// before setting the underlying flag value.
func (sf *StringsFlag) Set(s string) error {
	for _, v := range sf.Values {
		if s == v {
			sf.val = s
			return nil
		}
	}
	return errors.New("invalid value")
}

// String returns the set value (if any) of the StringsFlag
func (sf *StringsFlag) String() string {
	return sf.val
}

// Type returns the given type as string
func (sf *StringsFlag) Type() string {
	return "string"
}

// StringSliceFlag implements the flag.Value and pflag.Value interfaces.
type StringSliceFlag struct {
	Values []string
	val    []string
}

// NewStringSliceFlag creates a new string slice flag for which any one of the given
// strings is a valid value, and any other value is an error.
func NewStringSliceFlag(valids ...string) *StringSliceFlag {
	return &StringSliceFlag{Values: valids, val: []string{}}
}

// Set verifies the argument to be a valid member of the allowed values
// before setting the underlying flag value.
func (ssf *StringSliceFlag) Set(s string) error {
	sl := strings.Split(s, ",")
	ssf.val = []string{}

	for _, s := range sl {
		if !ssf.has(s) {
			return errors.New("invalid value")
		}

		ssf.val = append(ssf.val, s)
	}

	return nil
}

// String returns the set value (if any) of the StringSliceFlag
func (ssf *StringSliceFlag) String() string {
	return strings.Join(ssf.val, ",")
}

// Slice returns the set value (if any) of the StringSliceFlag as a slice
func (ssf *StringSliceFlag) Slice() []string {
	clone := make([]string, len(ssf.val))
	copy(clone, ssf.val)
	return clone
}

// Type returns the given type as stringSlice
func (ssf *StringSliceFlag) Type() string {
	return "stringSlice"
}

// has checks to see if value in in allowed slice
func (ssf *StringSliceFlag) has(s string) bool {
	for _, v := range ssf.Values {
		if v == s {
			return true
		}
	}

	return false
}

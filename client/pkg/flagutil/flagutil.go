// Copyright 2022 The etcd Authors
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

package flagutil

import (
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

const defaultUnitsDesc = "(default units: seconds)"

// Duration is wrapper over time.Duration
//
// Pointer of time.Duration is required to update value directly,
// specifically when using flag.(*FlagSet).Var
//
// Duration implements flag.Value and (github.com/spf13/pflag).Value interfaces
type Duration struct {
	t *time.Duration
}

// NewDuration creates Duration wrapping over time.Duration.
// The inner time.Duration is stored as reference for direct updates.
//
// If provided, default time duration is assigned to inner time.Duration field.
func NewDuration(duration, defaultDuration *time.Duration) *Duration {
	if defaultDuration != nil {
		*duration = *defaultDuration
	}

	return &Duration{duration}
}

// ToDuration creates Duration instance, similar to NewDuration.
//
// No pointers used as this is used by global vars, etc.,
// where direct update of value isn't needed.
func ToDuration(t time.Duration) Duration {
	return Duration{&t}
}

// String gives the string format of the underlying time.Duration
// 0s in case if time.Duration is not assigned.
//
// Output formats: 0µs, 0ms, 0s, 0m0s, 0h0m0s
func (d Duration) String() string {
	if d.t == nil {
		return time.Duration(0).String()
	}
	return d.t.String()
}

// Set assigns/updates the duration object with input time duration format string.
// Additionally, integer format is accepted which is defaulted to seconds
//
// Input string formats: 0, 0µs (or 0us), 0ms, 0s, 0m, 0m0s, 0h, 0h0m0s
func (d *Duration) Set(s string) error {
	if sec, err := strconv.Atoi(s); err == nil {
		// When input is numeric string (format: 0)
		*d.t = time.Second * time.Duration(sec)
		return nil
	}

	// When input is alphanumeric string (format: 0µs (or 0us), 0ms, 0s, 0m, 0m0s, 0h, 0h0m0s)
	v, err := time.ParseDuration(s)
	*d.t = v
	return err
}

// Type gives the flag type. This is a fixed value which is used by pflag.
func (d Duration) Type() string {
	return "duration"
}

// Dur gives the underlying time.Duration 's value
// 0s in case if time.Duration is not assigned.
func (d Duration) Dur() time.Duration {
	if d.t == nil {
		return time.Duration(0)
	}
	return *d.t
}

// DurationFlag is a convenience function to create pflag.Flag instance.
//
// This follows the pattern in which flags are assigned in flag and pflag packages.
func DurationFlag(p *Duration, name string, value Duration, usage string, descUnits bool) *pflag.Flag {
	if descUnits {
		// When default units string is required in description.
		usage = AddDefaultUnitsDesc(usage)
	}

	if p == nil {
		// When target Duration object is undefined, a new object should be assigned.
		p = new(Duration)
	}

	*p = value
	return &pflag.Flag{
		Name:     name,
		Usage:    usage,
		Value:    p,
		DefValue: value.String(),
	}
}

// AddDefaultUnitsDesc is a convenience function to add default units string to description.
func AddDefaultUnitsDesc(s string) string {
	return strings.Join([]string{s, defaultUnitsDesc}, " ")
}

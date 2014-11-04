/*
   Copyright 2014 CoreOS, Inc.

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

package flags

import (
	"errors"
)

const (
	FallbackExit  = "exit"
	FallbackProxy = "proxy"
)

var (
	FallbackValues = []string{
		FallbackExit,
		FallbackProxy,
	}
)

// FallbackFlag implements the flag.Value interface.
type Fallback string

// Set verifies the argument to be a valid member of FallbackFlagValues
// before setting the underlying flag value.
func (fb *Fallback) Set(s string) error {
	for _, v := range FallbackValues {
		if s == v {
			*fb = Fallback(s)
			return nil
		}
	}

	return errors.New("invalid value")
}

func (fb *Fallback) String() string {
	return string(*fb)
}

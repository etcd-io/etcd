// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import "fmt"

type FormatErrors []*FormatError

func (e FormatErrors) Error() string {
	errStr := "encountered the following formatting errors:\n"
	for _, err := range e {
		errStr += fmt.Sprintf("%s\n", err.Error())
	}
	return errStr
}

type FormatError struct {
	path string
	err  error
}

func wrapFormatError(path string, err error) *FormatError {
	return &FormatError{path: path, err: err}
}

func (e *FormatError) Error() string {
	return fmt.Sprintf("%s: %v", e.path, e.err)
}

// Copyright 2024 Google LLC
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

package yamlfmt

import "fmt"

type LineBreakStyle string

const (
	LineBreakStyleLF   LineBreakStyle = "lf"
	LineBreakStyleCRLF LineBreakStyle = "crlf"
)

type UnsupportedLineBreakError struct {
	style LineBreakStyle
}

func (e UnsupportedLineBreakError) Error() string {
	return fmt.Sprintf("unsupported line break style %s, see package documentation for supported styles", e.style)
}

func (s LineBreakStyle) Separator() (string, error) {
	switch s {
	case LineBreakStyleLF:
		return "\n", nil
	case LineBreakStyleCRLF:
		return "\r\n", nil
	}
	return "", UnsupportedLineBreakError{style: s}
}

// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package sharedbytes

// SharedBytes is a custom type for protobuf, it does not
// allocates memory in Unmarshal.
type SharedBytes []byte

// Marshal implements custom type for gogo/protobuf.
func (sb SharedBytes) Marshal() ([]byte, error) {
	data := make([]byte, len(sb))
	copy(data, sb)
	return data, nil
}

// MarshalTo implements custom type for gogo/protobuf.
func (sb SharedBytes) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, sb)
	return
}

// Unmarshal implements custom type for gogo/protobuf.
func (sb *SharedBytes) Unmarshal(data []byte) error {
	*sb = data
	return nil
}

// Size implements custom type for gogo/protobuf.
func (sb SharedBytes) Size() int {
	return len(sb)
}
